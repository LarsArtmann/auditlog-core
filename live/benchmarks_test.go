package live_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/larsartmann/auditlog-core/live"
)

// BenchmarkHub_OnEvent measures event fan-out throughput with varying subscriber counts.
func BenchmarkHub_OnEvent(b *testing.B) {
	for _, subscribers := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("subscribers=%d", subscribers), func(b *testing.B) {
			hub := live.NewHub()

			for range subscribers {
				sub := hub.Subscribe()
				b.Cleanup(func() { hub.Unsubscribe(sub.ID()) })
			}

			event := json.RawMessage(`{"sequence":1,"event_type":"benchmark"}`)

			b.ResetTimer()
			b.ReportAllocs()

			for range b.N {
				hub.OnEvent(event)
			}
		})
	}
}

// BenchmarkHub_SubscribeUnsubscribe measures the cost of adding and removing subscribers.
func BenchmarkHub_SubscribeUnsubscribe(b *testing.B) {
	hub := live.NewHub()

	b.ReportAllocs()

	for range b.N {
		sub := hub.Subscribe()
		hub.Unsubscribe(sub.ID())
	}
}

// BenchmarkServer_ServeHTTP_Dashboard measures the dashboard HTML endpoint throughput.
func BenchmarkServer_ServeHTTP_Dashboard(b *testing.B) {
	hub := live.NewHub()

	server := live.New(
		hub, live.Config{Addr: ":0", Prefix: "/"},
		live.WithDashboardProvider(func() string {
			return "<html><body>Dashboard</body></html>"
		}),
		live.WithReportProvider(func() ([]byte, error) {
			return []byte(`{"status":"ok"}`), nil
		}),
	)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		for pb.Next() {
			server.ServeHTTP(w, req)
		}
	})
}

// BenchmarkServer_ServeHTTP_Report measures the report JSON endpoint throughput.
func BenchmarkServer_ServeHTTP_Report(b *testing.B) {
	hub := live.NewHub()

	server := live.New(
		hub, live.Config{Addr: ":0", Prefix: "/"},
		live.WithReportProvider(func() ([]byte, error) {
			return []byte(`{"version":"1.0","steps":10,"status":"running"}`), nil
		}),
	)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/api/report", nil)
		w := httptest.NewRecorder()

		for pb.Next() {
			server.ServeHTTP(w, req)
		}
	})
}

// BenchmarkServer_ServeHTTP_Health measures the health endpoint throughput.
func BenchmarkServer_ServeHTTP_Health(b *testing.B) {
	hub := live.NewHub()

	server := live.New(
		hub, live.Config{Addr: ":0", Prefix: "/"},
		live.WithHealthProvider(func() live.HealthInfo {
			return live.HealthInfo{Events: 42, Dropped: 0}
		}),
	)

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		w := httptest.NewRecorder()

		for pb.Next() {
			server.ServeHTTP(w, req)
		}
	})
}
