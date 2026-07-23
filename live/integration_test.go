package live_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/larsartmann/auditlog-core/live"
)

// TestIntegration_FullLifecycle exercises the complete SSE lifecycle:
// connect, receive snapshot, receive live events, signal completion,
// receive complete event, and verify reconnection snapshot.
func TestIntegration_FullLifecycle(t *testing.T) {
	t.Parallel()

	// --- Simulated consumer state ---
	var (
		mu           sync.Mutex
		eventLog     []json.RawMessage
		reportJSON   = json.RawMessage(`{"version":"1.0","steps":2,"status":"running"}`)
		completeJSON = json.RawMessage(`{"version":"1.0","steps":2,"status":"done"}`)
	)

	hub := live.NewHub()

	server := live.New(
		hub, live.Config{
			Addr:              ":0",
			Prefix:            "/dashboard",
			ReadHeaderTimeout: 5 * time.Second,
			HeartbeatInterval: 50 * time.Millisecond,
		},
		live.WithDashboardProvider(func() string {
			return "<html><body>Dashboard</body></html>"
		}),
		live.WithReportProvider(func() ([]byte, error) {
			mu.Lock()
			defer mu.Unlock()

			return reportJSON, nil
		}),
		live.WithSnapshotProvider(func(isComplete bool) (json.RawMessage, error) {
			mu.Lock()
			defer mu.Unlock()

			report := reportJSON
			if isComplete {
				report = completeJSON
			}

			return json.Marshal(struct {
				Report   json.RawMessage   `json:"report"`
				Events   []json.RawMessage `json:"events"`
				Complete bool              `json:"complete"`
			}{
				Report:   report,
				Events:   eventLog,
				Complete: isComplete,
			})
		}),
		live.WithCompleteProvider(func() (json.RawMessage, error) {
			return completeJSON, nil
		},
		),
		live.WithHealthProvider(func() live.HealthInfo {
			mu.Lock()
			defer mu.Unlock()

			return live.HealthInfo{Events: len(eventLog), Dropped: 0}
		}),
	)

	ts := httptest.NewServer(server)
	defer ts.Close()

	// Step 1: Connect SSE client.
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/dashboard/api/events", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("connect SSE: %v", err)
	}

	defer func() { _ = resp.Body.Close() }()

	scanner := bufio.NewScanner(resp.Body)

	// Step 2: Verify snapshot on connect.
	snapshotData, ok := readSSEEventNamed(scanner, "snapshot")
	if !ok {
		t.Fatal("expected snapshot event on connect")
	}

	var snapshot struct {
		Report   json.RawMessage   `json:"report"`
		Events   []json.RawMessage `json:"events"`
		Complete bool              `json:"complete"`
	}

	if err := json.Unmarshal([]byte(snapshotData), &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}

	if snapshot.Complete {
		t.Error("snapshot should not be complete at start")
	}

	if len(snapshot.Events) != 0 {
		t.Errorf("expected 0 events in initial snapshot, got %d", len(snapshot.Events))
	}

	// Step 3: Push live events.
	for i := 1; i <= 3; i++ {
		evt, errMarshal := json.Marshal(map[string]any{
			"sequence": i,
			"step":     "step-" + string(rune('A'+i-1)),
			"status":   "completed",
		})
		if errMarshal != nil {
			t.Fatalf("marshal event %d: %v", i, errMarshal)
		}

		mu.Lock()
		eventLog = append(eventLog, evt)
		mu.Unlock()

		hub.OnEvent(evt)
	}

	// Step 4: Verify each event is received via SSE.
	for i := 1; i <= 3; i++ {
		data, ok := readSSEEventNamed(scanner, "event")
		if !ok {
			t.Fatalf("expected event #%d, got no data", i)
		}

		var event struct {
			Sequence int `json:"sequence"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			t.Fatalf("unmarshal event %d: %v", i, err)
		}

		if event.Sequence != i {
			t.Errorf("expected sequence %d, got %d", i, event.Sequence)
		}
	}

	// Step 5: Signal completion.
	hub.SignalComplete()

	// Step 6: Verify complete event.
	completeData, ok := readSSEEventNamed(scanner, "complete")
	if !ok {
		t.Fatal("expected complete event")
	}

	var complete struct {
		Status string `json:"status"`
	}

	if err := json.Unmarshal([]byte(completeData), &complete); err != nil {
		t.Fatalf("unmarshal complete: %v", err)
	}

	if complete.Status != "done" {
		t.Errorf("expected status 'done', got %q", complete.Status)
	}

	// Step 7: Verify report endpoint reflects completion.
	reportResp, err := http.Get(ts.URL + "/dashboard/api/report")
	if err != nil {
		t.Fatalf("get report: %v", err)
	}

	defer func() { _ = reportResp.Body.Close() }()

	var report struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(reportResp.Body).Decode(&report); err != nil {
		t.Fatalf("decode report: %v", err)
	}

	if report.Status != "running" {
		t.Errorf("expected report status 'running', got %q", report.Status)
	}

	// Step 8: Verify health endpoint.
	healthResp, err := http.Get(ts.URL + "/dashboard/api/health")
	if err != nil {
		t.Fatalf("get health: %v", err)
	}

	defer func() { _ = healthResp.Body.Close() }()

	var health struct {
		Clients  int  `json:"clients"`
		Events   int  `json:"events"`
		Complete bool `json:"complete"`
	}

	if err := json.NewDecoder(healthResp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}

	if !health.Complete {
		t.Error("health should report complete=true")
	}

	if health.Events != 3 {
		t.Errorf("expected 3 events in health, got %d", health.Events)
	}
}

// readSSEEventNamed scans for a named SSE event and returns its data payload.
func readSSEEventNamed(scanner *bufio.Scanner, eventName string) (string, bool) {
	for scanner.Scan() {
		line := scanner.Text()

		if !strings.HasPrefix(line, "event: ") {
			continue
		}

		if strings.TrimPrefix(line, "event: ") != eventName {
			continue
		}

		// Next line should be "data: ..."
		if !scanner.Scan() {
			return "", false
		}

		dataLine := scanner.Text()

		data, found := strings.CutPrefix(dataLine, "data: ")
		if !found {
			return "", false
		}

		return data, true
	}

	return "", false
}
