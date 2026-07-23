package live_test

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/larsartmann/auditlog-core/live"
)

const testDashboardHTML = `<!DOCTYPE html><html><head><title>test-auditlog Live</title></head>
<body><span class="live-badge">LIVE</span></body></html>`

// closeBody closes an HTTP response body, discarding the error. Intended for
// test cleanup where leaked connections are harmless and the error is
// unrecoverable.
func closeBody(body io.ReadCloser) { _ = body.Close() }

const testPrefix = "/debug/di"

func newTestReport() json.RawMessage {
	return json.RawMessage(`{"version":"0.1.0","report":{"container_id":"test-container"}}`)
}

func newTestSnapshot(isComplete bool) (json.RawMessage, error) {
	report := newTestReport()
	events := []json.RawMessage{
		json.RawMessage(`{"sequence":1,"event_type":"test"}`),
	}

	return json.Marshal(struct {
		Report   json.RawMessage   `json:"report"`
		Events   []json.RawMessage `json:"events"`
		Complete bool              `json:"complete"`
	}{
		Report:   report,
		Events:   events,
		Complete: isComplete,
	})
}

func newTestComplete() (json.RawMessage, error) {
	return json.Marshal(struct {
		Report json.RawMessage `json:"report"`
	}{
		Report: newTestReport(),
	})
}

func newTestServer(t *testing.T) *live.Server {
	t.Helper()

	hub := live.NewHub()

	server := live.New(hub, live.Config{
		Addr:   ":0",
		Prefix: testPrefix,
	},
		live.WithDashboardProvider(func() string { return testDashboardHTML }),
		live.WithReportProvider(func() ([]byte, error) { return newTestReport(), nil }),
		live.WithSnapshotProvider(newTestSnapshot),
		live.WithCompleteProvider(newTestComplete),
		live.WithHealthProvider(func() live.HealthInfo {
			return live.HealthInfo{Events: 0, Dropped: 0}
		}),
	)

	return server
}

func TestServer_DashboardHTML(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, testPrefix+"/", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}

	body := w.Body.String()
	for _, want := range []string{"<!DOCTYPE html>", "test-auditlog", "LIVE"} {
		if !strings.Contains(body, want) {
			t.Errorf("dashboard HTML missing %q", want)
		}
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, testPrefix+"/api/health", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", resp.Status)
	}
}

func TestServer_ReportEndpoint(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, testPrefix+"/api/report", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected application/json, got %q", ct)
	}

	var resp struct {
		Report struct {
			ContainerID string `json:"container_id"`
		} `json:"report"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if resp.Report.ContainerID != "test-container" {
		t.Fatalf("expected container_id 'test-container', got %q", resp.Report.ContainerID)
	}
}

func TestServer_NotFound(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestServer_CustomPrefix(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	server := live.New(hub, live.Config{
		Addr:   ":0",
		Prefix: "/my/debug",
	},
		live.WithDashboardProvider(func() string { return testDashboardHTML }),
		live.WithReportProvider(func() ([]byte, error) { return newTestReport(), nil }),
		live.WithSnapshotProvider(newTestSnapshot),
		live.WithCompleteProvider(newTestComplete),
	)

	// Should serve at /my/debug/
	req := httptest.NewRequest(http.MethodGet, "/my/debug/", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 at /my/debug/, got %d", w.Code)
	}

	// Root / should 404
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 at /, got %d", w.Code)
	}
}

func TestServer_RootPrefix(t *testing.T) {
	t.Parallel()

	hub := live.NewHub()
	server := live.New(hub, live.Config{
		Addr:   ":0",
		Prefix: "/",
	},
		live.WithDashboardProvider(func() string { return testDashboardHTML }),
		live.WithReportProvider(func() ([]byte, error) { return newTestReport(), nil }),
		live.WithSnapshotProvider(newTestSnapshot),
		live.WithCompleteProvider(newTestComplete),
	)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 at /, got %d", w.Code)
	}
}

func TestServer_ClientCount(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)
	if server.ClientCount() != 0 {
		t.Fatalf("expected 0 clients, got %d", server.ClientCount())
	}
}

func TestServer_SSE_SnapshotOnConnect(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	resp, err := http.Get(ts.URL + testPrefix + "/api/events")
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer closeBody(resp.Body)

	scanner := bufio.NewScanner(resp.Body)

	// Read snapshot event
	if !scanner.Scan() {
		t.Fatal("no SSE data")
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "event: ") {
		t.Fatalf("expected 'event:' prefix, got %q", line)
	}

	eventType := strings.TrimPrefix(line, "event: ")
	if eventType != "snapshot" {
		t.Fatalf("expected 'snapshot' event, got %q", eventType)
	}

	// Read data line
	if !scanner.Scan() {
		t.Fatal("no SSE data line")
	}

	dataLine := scanner.Text()
	if !strings.HasPrefix(dataLine, "data: ") {
		t.Fatalf("expected 'data:' prefix, got %q", dataLine)
	}

	data := strings.TrimPrefix(dataLine, "data: ")

	var snapshot struct {
		Report   json.RawMessage `json:"report"`
		Complete bool            `json:"complete"`
	}

	if err := json.Unmarshal([]byte(data), &snapshot); err != nil {
		t.Fatalf("failed to unmarshal snapshot: %v", err)
	}

	if snapshot.Report == nil {
		t.Fatal("snapshot missing report")
	}
}

func TestServer_SSE_LiveEventDelivery(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	resp, err := http.Get(ts.URL + testPrefix + "/api/events")
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer closeBody(resp.Body)

	scanner := bufio.NewScanner(resp.Body)

	// Skip snapshot
	skipSSESnapshot(t, scanner)

	// Push an event through the hub
	evt := json.RawMessage(`{"sequence":42,"event_type":"test"}`)
	server.OnEvent(evt)

	// Read the event
	if !scanner.Scan() {
		t.Fatal("no SSE event received")
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "event: ") {
		t.Fatalf("expected 'event:' prefix, got %q", line)
	}

	eventType := strings.TrimPrefix(line, "event: ")
	if eventType != "event" {
		t.Fatalf("expected 'event' type, got %q", eventType)
	}

	// Read data
	if !scanner.Scan() {
		t.Fatal("no SSE data line")
	}

	dataLine := scanner.Text()
	if !strings.HasPrefix(dataLine, "data: ") {
		t.Fatalf("expected 'data:' prefix, got %q", dataLine)
	}

	var received struct {
		Sequence int `json:"sequence"`
	}

	data := strings.TrimPrefix(dataLine, "data: ")
	if err := json.Unmarshal([]byte(data), &received); err != nil {
		t.Fatalf("failed to unmarshal event: %v", err)
	}

	if received.Sequence != 42 {
		t.Fatalf("expected sequence 42, got %d", received.Sequence)
	}
}

func TestServer_SSE_CompleteEvent(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	resp, err := http.Get(ts.URL + testPrefix + "/api/events")
	if err != nil {
		t.Fatalf("SSE connect failed: %v", err)
	}
	defer closeBody(resp.Body)

	scanner := bufio.NewScanner(resp.Body)

	// Skip snapshot
	skipSSESnapshot(t, scanner)

	// Signal complete
	server.SignalComplete()

	// Read complete event
	if !scanner.Scan() {
		t.Fatal("no SSE complete received")
	}

	line := scanner.Text()
	if !strings.HasPrefix(line, "event: ") {
		t.Fatalf("expected 'event:' prefix, got %q", line)
	}

	eventType := strings.TrimPrefix(line, "event: ")
	if eventType != "complete" {
		t.Fatalf("expected 'complete' event, got %q", eventType)
	}
}

func TestServer_SSE_FanOut(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)
	defer ts.Close()

	// Connect two clients
	resp1, err := http.Get(ts.URL + testPrefix + "/api/events")
	if err != nil {
		t.Fatalf("client 1 connect failed: %v", err)
	}
	defer closeBody(resp1.Body)

	resp2, err := http.Get(ts.URL + testPrefix + "/api/events")
	if err != nil {
		t.Fatalf("client 2 connect failed: %v", err)
	}
	defer closeBody(resp2.Body)

	scanner1 := bufio.NewScanner(resp1.Body)
	scanner2 := bufio.NewScanner(resp2.Body)

	// Skip snapshots
	skipSSESnapshot(t, scanner1)
	skipSSESnapshot(t, scanner2)

	// Push event
	evt := json.RawMessage(`{"sequence":1,"event_type":"fanout"}`)
	server.OnEvent(evt)

	// Both should receive
	for i, scanner := range []*bufio.Scanner{scanner1, scanner2} {
		if !scanner.Scan() {
			t.Fatalf("client %d: no event received", i+1)
		}

		line := scanner.Text()
		if !strings.HasPrefix(line, "event: event") {
			t.Fatalf("client %d: expected 'event' type, got %q", i+1, line)
		}
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	ts := httptest.NewServer(server)

	// Health should work
	resp, err := http.Get(ts.URL + testPrefix + "/api/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	closeBody(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func skipSSESnapshot(t *testing.T, scanner *bufio.Scanner) {
	t.Helper()

	// Scan until we find "event: snapshot" followed by "data: ..." and the empty line
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: snapshot") {
			// Skip data lines until empty line
			for scanner.Scan() {
				dataLine := scanner.Text()
				if dataLine == "" {
					return
				}
			}

			return
		}
	}
}
