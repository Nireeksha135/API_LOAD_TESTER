package engine

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
)

// newTestConfig returns a valid default Config pointed at the given
// test server URL, ready for callers to further customize.
func newTestConfig(targetURL string) *config.Config {
	cfg := config.NewDefaultConfig()
	cfg.TargetURL = targetURL
	cfg.Method = config.MethodGET
	cfg.Concurrency = 5
	cfg.TotalRequests = 50
	cfg.Timeout = 5 * time.Second
	return cfg
}

func TestEngineRunCountMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)

	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if summary.TotalRequests != int64(cfg.TotalRequests) {
		t.Errorf("TotalRequests = %d, want %d", summary.TotalRequests, cfg.TotalRequests)
	}
	if summary.SuccessRequests != int64(cfg.TotalRequests) {
		t.Errorf("SuccessRequests = %d, want %d", summary.SuccessRequests, cfg.TotalRequests)
	}
	if summary.FailedRequests != 0 {
		t.Errorf("FailedRequests = %d, want 0", summary.FailedRequests)
	}
	if summary.StatusCodeCounts[200] != int64(cfg.TotalRequests) {
		t.Errorf("StatusCodeCounts[200] = %d, want %d", summary.StatusCodeCounts[200], cfg.TotalRequests)
	}
}

func TestEngineRunDurationMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.UseDuration = true
	cfg.Duration = 300 * time.Millisecond
	cfg.Concurrency = 4

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	start := time.Now()
	summary, err := eng.Run(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.TotalRequests == 0 {
		t.Errorf("TotalRequests = 0, want > 0")
	}
	// Should stop close to the configured duration, with generous
	// slack for scheduler jitter in CI environments.
	if elapsed > 2*time.Second {
		t.Errorf("Run() took %v, want roughly %v", elapsed, cfg.Duration)
	}
}

func TestEngineRunPostWithBody(t *testing.T) {
	var receivedBodies int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) == `{"name":"load-test"}` {
			atomic.AddInt32(&receivedBodies, 1)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("X-Custom-Header") != "test-value" {
			t.Errorf("expected X-Custom-Header test-value, got %s", r.Header.Get("X-Custom-Header"))
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.Method = config.MethodPOST
	cfg.Body = `{"name":"load-test"}`
	cfg.ContentType = "application/json"
	cfg.Headers = map[string]string{"X-Custom-Header": "test-value"}
	cfg.TotalRequests = 10
	cfg.Concurrency = 2

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if summary.StatusCodeCounts[201] != 10 {
		t.Errorf("StatusCodeCounts[201] = %d, want 10", summary.StatusCodeCounts[201])
	}
	if atomic.LoadInt32(&receivedBodies) != 10 {
		t.Errorf("receivedBodies = %d, want 10", receivedBodies)
	}
}

func TestEngineRunHandlesTargetDown(t *testing.T) {
	// A server that is created and immediately closed yields a URL
	// nothing is listening on, guaranteeing connection failures.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	targetURL := server.URL
	server.Close()

	cfg := newTestConfig(targetURL)
	cfg.TotalRequests = 5
	cfg.Concurrency = 2
	cfg.Timeout = 500 * time.Millisecond

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if summary.FailedRequests != 5 {
		t.Errorf("FailedRequests = %d, want 5", summary.FailedRequests)
	}
	if summary.SuccessRequests != 0 {
		t.Errorf("SuccessRequests = %d, want 0", summary.SuccessRequests)
	}
	if len(summary.Errors) == 0 {
		t.Errorf("expected at least one recorded error message")
	}
}

func TestEngineRunGracefulShutdown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.UseDuration = true
	cfg.Duration = 10 * time.Second // long budget; we cancel manually instead
	cfg.Concurrency = 4

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	summary, err := eng.Run(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if elapsed > 1*time.Second {
		t.Errorf("Run() took %v after cancellation, want well under 1s", elapsed)
	}
	if summary.TotalRequests == 0 {
		t.Errorf("TotalRequests = 0, want > 0 partial results after graceful shutdown")
	}
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.TargetURL = "" // invalid: empty target
	collector := metrics.NewCollector("", cfg.Method, cfg.Concurrency)

	if _, err := New(cfg, collector, nil); err == nil {
		t.Error("New() with invalid config: expected error, got nil")
	}
}

func TestNewRejectsNilCollector(t *testing.T) {
	cfg := newTestConfig("http://example.com")
	if _, err := New(cfg, nil, nil); err == nil {
		t.Error("New() with nil collector: expected error, got nil")
	}
}
