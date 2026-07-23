package live

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	defaultReadHeaderTimeout = 5 * time.Second
	defaultHeartbeatInterval = 15 * time.Second
	defaultAddr              = ":0"
	defaultPrefix            = "/"
)

// ErrServerAlreadyRunning is returned when ListenAndServe is called on a
// server that is already serving.
var ErrServerAlreadyRunning = errors.New("live server is already running")

// Config controls the live dashboard server behaviour.
type Config struct {
	// Addr is the TCP address to listen on. Default ":0" (random port).
	Addr string
	// Prefix is the URL path prefix for all dashboard routes.
	// Default "/". Routes: {prefix}/, {prefix}/api/report,
	// {prefix}/api/events, {prefix}/api/health.
	// Trailing slash is stripped.
	Prefix string
	// ReadHeaderTimeout is the maximum duration for reading the request
	// headers. Default 5 seconds. Set to 0 to disable.
	ReadHeaderTimeout time.Duration
	// HeartbeatInterval is how often to send SSE keepalive comments.
	// Default 15 seconds. Set to 0 to disable heartbeats.
	HeartbeatInterval time.Duration
}

// ReportProvider returns the current report as JSON bytes. Called on each
// /api/report request and when building SSE snapshots/completions.
type ReportProvider func() ([]byte, error)

// SnapshotProvider returns the initial SSE snapshot payload as raw JSON.
// The payload should be a JSON object with "report", "events", "metadata",
// "dag", and "complete" fields.
type SnapshotProvider func(isComplete bool) (json.RawMessage, error)

// CompleteProvider returns the final SSE complete payload as raw JSON.
// The payload should be a JSON object with "report" and "dag" fields.
type CompleteProvider func() (json.RawMessage, error)

// DashboardProvider returns the full HTML string for the dashboard page.
type DashboardProvider func() string

// HealthInfo provides dynamic health check data beyond the built-in
// uptime, client count, and completion status.
type HealthInfo struct {
	Events  int   `json:"events"`
	Dropped int64 `json:"dropped"`
}

// HealthProvider returns additional health check information.
type HealthProvider func() HealthInfo

// Server serves the real-time audit dashboard over HTTP.
type Server struct {
	hub    *Hub
	config Config

	serverMu   sync.Mutex
	httpServer *http.Server
	mux        *http.ServeMux

	reportProvider    ReportProvider
	snapshotProvider  SnapshotProvider
	completeProvider  CompleteProvider
	dashboardProvider DashboardProvider
	healthProvider    HealthProvider

	dashboardHTML string
	startTime     time.Time
}

// New creates a Server from an existing Hub with full provider control.
func New(hub *Hub, cfg Config, providers ...ServerOption) *Server {
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}

	if cfg.Prefix == "" {
		cfg.Prefix = defaultPrefix
	}

	cfg.Prefix = normalizePrefix(cfg.Prefix)

	if cfg.ReadHeaderTimeout == 0 {
		cfg.ReadHeaderTimeout = defaultReadHeaderTimeout
	}

	if cfg.HeartbeatInterval == 0 {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}

	s := &Server{
		hub:    hub,
		config: cfg,
		mux:    http.NewServeMux(),
	}

	for _, opt := range providers {
		opt(s)
	}

	s.setupRoutes()

	return s
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithReportProvider sets the report JSON provider.
func WithReportProvider(fn ReportProvider) ServerOption {
	return func(s *Server) { s.reportProvider = fn }
}

// WithSnapshotProvider sets the SSE snapshot provider.
func WithSnapshotProvider(fn SnapshotProvider) ServerOption {
	return func(s *Server) { s.snapshotProvider = fn }
}

// WithCompleteProvider sets the SSE complete provider.
func WithCompleteProvider(fn CompleteProvider) ServerOption {
	return func(s *Server) { s.completeProvider = fn }
}

// WithDashboardProvider sets the dashboard HTML provider.
func WithDashboardProvider(fn DashboardProvider) ServerOption {
	return func(s *Server) {
		s.dashboardProvider = fn
		s.dashboardHTML = fn()
	}
}

// WithHealthProvider sets the health info provider.
func WithHealthProvider(fn HealthProvider) ServerOption {
	return func(s *Server) { s.healthProvider = fn }
}

func (s *Server) setupRoutes() {
	pfx := s.config.Prefix
	if pfx == "/" {
		s.mux.HandleFunc("/", s.handleDashboard)
		s.mux.HandleFunc("/api/report", s.handleReport)
		s.mux.HandleFunc("/api/events", s.handleSSE)
		s.mux.HandleFunc("/api/health", s.handleHealth)
	} else {
		s.mux.HandleFunc(pfx+"/", s.handleDashboard)
		s.mux.HandleFunc(pfx+"/api/report", s.handleReport)
		s.mux.HandleFunc(pfx+"/api/events", s.handleSSE)
		s.mux.HandleFunc(pfx+"/api/health", s.handleHealth)
	}
}

// ListenAndServe starts the HTTP server. It blocks until Shutdown is called
// or the server encounters a fatal error.
func (s *Server) ListenAndServe() error {
	s.serverMu.Lock()

	if s.httpServer != nil {
		s.serverMu.Unlock()

		return ErrServerAlreadyRunning
	}

	s.startTime = time.Now()

	s.httpServer = &http.Server{
		Addr:              s.config.Addr,
		Handler:           s.mux,
		ReadHeaderTimeout: s.config.ReadHeaderTimeout,
	}

	s.serverMu.Unlock()

	err := s.httpServer.ListenAndServe()

	return fmt.Errorf("listen and serve: %w", err)
}

// Addr returns the server's listen address. If the server was started with
// ":0", this returns the actual address after ListenAndServe binds.
func (s *Server) Addr() string {
	s.serverMu.Lock()
	defer s.serverMu.Unlock()

	if s.httpServer == nil {
		return s.config.Addr
	}

	return s.httpServer.Addr
}

// Shutdown gracefully shuts down the server, waiting for in-flight requests
// to complete (up to the context deadline).
func (s *Server) Shutdown(ctx context.Context) error {
	s.serverMu.Lock()
	server := s.httpServer
	s.serverMu.Unlock()

	if server == nil {
		return nil
	}

	err := server.Shutdown(ctx)

	return fmt.Errorf("shutdown: %w", err)
}

// SignalComplete marks the lifecycle as finished. All connected SSE clients
// receive the final report.
func (s *Server) SignalComplete() {
	s.hub.SignalComplete()
}

// OnEvent broadcasts an event to all connected SSE clients.
func (s *Server) OnEvent(data json.RawMessage) {
	s.hub.OnEvent(data)
}

// ClientCount returns the number of currently connected SSE clients.
func (s *Server) ClientCount() int {
	return s.hub.ClientCount()
}

// ServeHTTP implements http.Handler, delegating to the internal mux.
// This allows the Server to be used with httptest.NewServer.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// --- HTTP Handlers ---

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	pfx := s.config.Prefix
	if r.URL.Path != pfx && r.URL.Path != pfx+"/" {
		http.NotFound(w, r)

		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(s.dashboardHTML))
}

func (s *Server) handleReport(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if s.reportProvider == nil {
		http.Error(w, "report provider not configured", http.StatusInternalServerError)

		return
	}

	data, err := s.reportProvider()
	if err != nil {
		http.Error(w, fmt.Sprintf("generate report: %v", err), http.StatusInternalServerError)

		return
	}

	_, _ = w.Write(data)
}

// healthResponse is the JSON payload returned by the health endpoint.
type healthResponse struct {
	Status   string  `json:"status"`
	UptimeS  float64 `json:"uptime_s"`
	Clients  int     `json:"clients"`
	Events   int     `json:"events"`
	Complete bool    `json:"complete"`
	Dropped  int64   `json:"dropped"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	resp := healthResponse{
		Status:   "ok",
		UptimeS:  time.Since(s.startTime).Seconds(),
		Clients:  s.hub.ClientCount(),
		Events:   0,
		Complete: s.hub.IsComplete(),
		Dropped:  0,
	}

	if s.healthProvider != nil {
		info := s.healthProvider()
		resp.Events = info.Events
		resp.Dropped = info.Dropped
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "marshal health response", http.StatusInternalServerError)

		return
	}

	_, _ = w.Write(payload)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	sub := s.hub.Subscribe()
	defer s.hub.Unsubscribe(sub.id)

	err := s.sendSnapshot(w, flusher)
	if err != nil {
		return
	}

	heartbeat := time.NewTicker(s.config.HeartbeatInterval)
	defer heartbeat.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return

		case <-sub.done:
			s.sendComplete(w, flusher)

			return

		case evt := <-sub.ch:
			err = writeSSE(w, "event", evt)
			if err != nil {
				return
			}

			flusher.Flush()

		case <-heartbeat.C:
			_, err = w.Write([]byte(": heartbeat\n\n"))
			if err != nil {
				return
			}

			flusher.Flush()
		}
	}
}

func (s *Server) sendSnapshot(w http.ResponseWriter, flusher http.Flusher) error {
	if s.snapshotProvider == nil {
		return nil
	}

	data, err := s.snapshotProvider(s.hub.IsComplete())
	if err != nil {
		return fmt.Errorf("build snapshot: %w", err)
	}

	err = writeSSE(w, "snapshot", data)
	if err != nil {
		return err
	}

	flusher.Flush()

	return nil
}

func (s *Server) sendComplete(w http.ResponseWriter, flusher http.Flusher) {
	if s.completeProvider == nil {
		return
	}

	data, err := s.completeProvider()
	if err != nil {
		return
	}

	_ = writeSSE(w, "complete", data)

	flusher.Flush()
}

// writeSSE writes a named SSE event with JSON-encoded data.
func writeSSE(w http.ResponseWriter, eventName string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal SSE data: %w", err)
	}

	_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, payload)
	if err != nil {
		return fmt.Errorf("write SSE: %w", err)
	}

	return nil
}

// normalizePrefix ensures the prefix starts with "/" and has no trailing "/".
func normalizePrefix(prefix string) string {
	if prefix == "/" {
		return "/"
	}

	prefix = strings.TrimRight(prefix, "/")

	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	return prefix
}
