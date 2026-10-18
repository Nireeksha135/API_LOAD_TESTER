package engine

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
	if summary.TotalBytesRead != int64(cfg.TotalRequests*len(`{"ok":true}`)) {
		t.Errorf("TotalBytesRead = %d, want %d", summary.TotalBytesRead, cfg.TotalRequests*len(`{"ok":true}`))
	}
	if summary.ThroughputBytesPerSecond <= 0 {
		t.Errorf("ThroughputBytesPerSecond = %f, want > 0", summary.ThroughputBytesPerSecond)
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

func TestEngineRateModeSchedulesIndependentlyOfResponseTime(t *testing.T) {
	arrivals := make(chan time.Time, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivals <- time.Now()
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.TotalRequests = 4
	cfg.Concurrency = 4
	cfg.RPS = 10
	cfg.Timeout = 2 * time.Second
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.ScheduledRequests != 4 || summary.TotalRequests != 4 || summary.DroppedRequests != 0 {
		t.Errorf("scheduled=%d actual=%d dropped=%d, want 4/4/0", summary.ScheduledRequests, summary.TotalRequests, summary.DroppedRequests)
	}
	first := <-arrivals
	last := first
	for i := 1; i < 4; i++ {
		arrival := <-arrivals
		if arrival.Before(first) {
			first = arrival
		}
		if arrival.After(last) {
			last = arrival
		}
	}
	span := last.Sub(first)
	if span < 150*time.Millisecond || span > 800*time.Millisecond {
		t.Errorf("request start span = %v, want approximately 300ms despite 500ms responses", span)
	}
}

func TestExpectedArrivalsAccountsForRampUp(t *testing.T) {
	const (
		targetRPS   = 100.0
		rampUp      = 10 * time.Second
		concurrency = 9
	)
	tests := []struct {
		name     string
		duration time.Duration
		want     int64
	}{
		{name: "before ramp starts", duration: 500 * time.Millisecond, want: 5},
		{name: "during ramp", duration: 5 * time.Second, want: 130},
		{name: "after ramp", duration: 30 * time.Second, want: 2505},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectedArrivals(tt.duration, targetRPS, rampUp, concurrency); got != tt.want {
				t.Errorf("expectedArrivals() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestArrivalOffsetFollowsRampIntegral(t *testing.T) {
	for ordinal := int64(1); ordinal <= 10; ordinal++ {
		offset := arrivalOffset(ordinal, 100, 10*time.Second, 9)
		got := expectedArrivals(offset, 100, 10*time.Second, 9)
		if got != ordinal {
			t.Errorf("expected arrivals by offset %s = %d for ordinal %d", offset, got, ordinal)
		}
	}
}

func TestEngineRateModeCountsDroppedArrivals(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.TotalRequests = 10
	cfg.Concurrency = 1
	cfg.RPS = 20
	cfg.Timeout = time.Second
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.ScheduledRequests != 10 {
		t.Errorf("ScheduledRequests = %d, want 10", summary.ScheduledRequests)
	}
	if summary.ExpectedScheduledRequests < summary.ScheduledRequests {
		t.Errorf("ExpectedScheduledRequests = %d, want at least scheduled arrivals %d", summary.ExpectedScheduledRequests, summary.ScheduledRequests)
	}
	if summary.ScheduleDuration <= 0 || summary.ScheduleDuration >= summary.TotalDuration {
		t.Errorf("ScheduleDuration = %v, want positive duration shorter than request-drain-inclusive TotalDuration %v", summary.ScheduleDuration, summary.TotalDuration)
	}
	if summary.DroppedRequests == 0 {
		t.Error("DroppedRequests = 0, want saturation to be visible")
	}
	if summary.TotalRequests+summary.DroppedRequests != summary.ScheduledRequests {
		t.Errorf("actual + dropped = %d, want scheduled %d", summary.TotalRequests+summary.DroppedRequests, summary.ScheduledRequests)
	}
}

func TestEngineWarmupRequestsAreNotMeasured(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.TotalRequests = 1
	cfg.Concurrency = 1
	cfg.Warmup = 50 * time.Millisecond
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.TotalRequests != 1 {
		t.Errorf("measured TotalRequests = %d, want 1", summary.TotalRequests)
	}
	if got := atomic.LoadInt32(&requests); got <= 1 {
		t.Errorf("server saw %d requests, want warm-up requests plus one measured request", got)
	}
}

func TestEngineWarmupCounterRestartsAtOne(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL + "/{{counter}}")
	cfg.TotalRequests = 3
	cfg.Concurrency = 1
	cfg.Warmup = 30 * time.Millisecond
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := eng.requestCounter.Load(); got != 3 {
		t.Errorf("request counter after run = %d, want 3 measured request IDs", got)
	}
	if got := requests.Load(); got <= 3 {
		t.Errorf("server received %d requests, want warm-up requests plus 3 measured requests", got)
	}
}

func TestEngineDurationStartsAfterWarmup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.UseDuration = true
	cfg.Duration = 100 * time.Millisecond
	cfg.Warmup = 100 * time.Millisecond
	cfg.Concurrency = 1
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	start := time.Now()
	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if elapsed := time.Since(start); elapsed < 180*time.Millisecond {
		t.Errorf("Run() elapsed = %v, want warmup plus measured duration", elapsed)
	}
	if summary.TotalDuration < 80*time.Millisecond {
		t.Errorf("measured duration = %v, want the full duration after warmup", summary.TotalDuration)
	}
}

func TestEngineRampUpStaggersClosedLoopWorkers(t *testing.T) {
	arrivals := make(chan time.Time, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivals <- time.Now()
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.TotalRequests = 3
	cfg.Concurrency = 3
	cfg.RampUp = 120 * time.Millisecond
	cfg.Timeout = time.Second
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	first := <-arrivals
	last := first
	for i := 1; i < 3; i++ {
		arrival := <-arrivals
		if arrival.Before(first) {
			first = arrival
		}
		if arrival.After(last) {
			last = arrival
		}
	}
	span := last.Sub(first)
	if span < 80*time.Millisecond || span > 500*time.Millisecond {
		t.Errorf("request start span = %v, want staggered starts across ramp-up", span)
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

func TestEngineRunUsesHostHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != "virtual.example.com" {
			t.Errorf("Host = %q, want %q", r.Host, "virtual.example.com")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.TotalRequests = 1
	cfg.Concurrency = 1
	cfg.Headers = map[string]string{"hOsT": "virtual.example.com"}
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.SuccessRequests != 1 {
		t.Errorf("SuccessRequests = %d, want 1", summary.SuccessRequests)
	}
}

func TestEngineSubstitutesRequestTokensAndRotatesURLs(t *testing.T) {
	var mu sync.Mutex
	paths := make(map[string]int)
	randomValues := make(map[string]struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) != 3 {
			t.Errorf("request path = %q, want /<target>/<counter>", r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		counter := parts[2]
		randomValue := r.Header.Get("X-Nonce")
		if r.Header.Get("X-Counter") != counter {
			t.Errorf("X-Counter = %q, want %q", r.Header.Get("X-Counter"), counter)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if !strings.Contains(string(body), "id="+counter) || !strings.Contains(string(body), "nonce="+randomValue) {
			t.Errorf("request body = %q, want counter and random token substitutions", body)
		}
		if len(randomValue) != 32 {
			t.Errorf("X-Nonce length = %d, want 32 hex characters", len(randomValue))
		}
		mu.Lock()
		paths[parts[1]]++
		randomValues[randomValue] = struct{}{}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL + "/a/{{counter}}")
	cfg.TargetURLs = []string{server.URL + "/a/{{counter}}", server.URL + "/b/{{counter}}"}
	cfg.TotalRequests = 4
	cfg.Concurrency = 2
	cfg.Body = "id={{counter}}&nonce={{random}}"
	cfg.Headers = map[string]string{"X-Counter": "{{counter}}", "X-Nonce": "{{random}}"}
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := eng.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if paths["a"] != 2 || paths["b"] != 2 {
		t.Errorf("target distribution = %v, want two requests to each target", paths)
	}
	if len(randomValues) != 4 {
		t.Errorf("generated %d distinct random values, want 4", len(randomValues))
	}
}

func TestEngineRunCountsRunCancellationSeparately(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case requestStarted <- struct{}{}:
		default:
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.UseDuration = true
	cfg.Duration = 100 * time.Millisecond
	cfg.Concurrency = 1
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	select {
	case <-requestStarted:
	default:
		t.Fatal("request did not start before run cancellation")
	}
	if summary.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1 for cancelled in-flight request", summary.TotalRequests)
	}
	if summary.FailedRequests != 0 || summary.CancelledRequests != 1 || summary.StatusCodeCounts[0] != 1 {
		t.Errorf("failed=%d cancelled=%d status-0=%d, want 0/1/1", summary.FailedRequests, summary.CancelledRequests, summary.StatusCodeCounts[0])
	}
	if len(summary.Errors) != 0 {
		t.Errorf("Errors = %v, want cancellation excluded from errors", summary.Errors)
	}
	if summary.P50 < 50*time.Millisecond {
		t.Errorf("P50 = %s, want cancelled request latency included", summary.P50)
	}
}

func TestHTTP2NegotiationCanBeConfigured(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled=%t", enabled), func(t *testing.T) {
			protocol := make(chan int, 1)
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				protocol <- r.ProtoMajor
				w.WriteHeader(http.StatusOK)
			}))
			server.EnableHTTP2 = true
			server.StartTLS()
			defer server.Close()

			cfg := newTestConfig(server.URL)
			cfg.EnableHTTP2 = enabled
			cfg.InsecureSkipVerify = true
			client := NewHTTPClient(cfg)
			response, err := client.Get(server.URL)
			if err != nil {
				t.Fatalf("GET() error = %v", err)
			}
			_ = response.Body.Close()

			wantProtocol := 1
			if enabled {
				wantProtocol = 2
			}
			if got := <-protocol; got != wantProtocol {
				t.Errorf("negotiated HTTP major version = %d, want %d", got, wantProtocol)
			}
		})
	}
}

func TestHTTPClientIdlePoolScalesWithConcurrency(t *testing.T) {
	cfg := newTestConfig("http://example.com")
	cfg.Concurrency = 250
	transport := NewHTTPClient(cfg).Transport.(*http.Transport)
	if transport.MaxIdleConnsPerHost != 250 {
		t.Errorf("MaxIdleConnsPerHost = %d, want concurrency-sized pool 250", transport.MaxIdleConnsPerHost)
	}
	if transport.MaxIdleConns != 1000 {
		t.Errorf("MaxIdleConns = %d, want 1000", transport.MaxIdleConns)
	}
}

func TestEngineRunCountsPerRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL)
	cfg.TotalRequests = 1
	cfg.Concurrency = 1
	cfg.Timeout = 20 * time.Millisecond
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.FailedRequests != 1 {
		t.Errorf("FailedRequests = %d, want 1 for per-request timeout", summary.FailedRequests)
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

func TestRequestTemplateMapsRequestIDsToEndpointLabels(t *testing.T) {
	cfg := newTestConfig("http://example.com/login")
	cfg.TargetURLs = []string{
		"http://example.com/login",
		"http://example.com/orders/{{counter}}?expand=items",
		"http://example.com/orders/42",
	}
	template := newRequestTemplate(cfg)

	want := map[uint64]string{
		0: "/login", // no ID allocated: first URL
		1: "/login",
		2: "/orders/{id}",
		3: "/orders/{id}",
		4: "/login", // wraps around
	}
	for id, label := range want {
		if got := template.endpointFor(id); got != label {
			t.Errorf("endpointFor(%d) = %q, want %q", id, got, label)
		}
	}
}

func TestEngineReportsPerEndpointStatsSlowestFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/checkout"):
			time.Sleep(40 * time.Millisecond)
			w.WriteHeader(http.StatusInternalServerError)
		case strings.HasPrefix(r.URL.Path, "/products"):
			time.Sleep(15 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := newTestConfig(server.URL + "/login")
	cfg.TargetURLs = []string{
		server.URL + "/login",
		server.URL + "/login?next=/home", // same endpoint as /login
		server.URL + "/products/{{counter}}",
		server.URL + "/checkout",
	}
	cfg.TotalRequests = 80
	cfg.Concurrency = 4
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency, metrics.WithEndpointStats())
	eng, err := New(cfg, collector, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	summary, err := eng.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if len(summary.Endpoints) != 3 {
		t.Fatalf("got %d endpoints, want 3 (query strings and ids must not create new endpoints): %+v", len(summary.Endpoints), summary.Endpoints)
	}
	wantOrder := []string{"/checkout", "/products/{id}", "/login"}
	wantRequests := map[string]int64{"/checkout": 20, "/products/{id}": 20, "/login": 40}
	for i, label := range wantOrder {
		got := summary.Endpoints[i]
		if got.Endpoint != label {
			t.Fatalf("endpoint[%d] = %q, want %q (slowest first): %+v", i, got.Endpoint, label, summary.Endpoints)
		}
		if got.Requests != wantRequests[label] {
			t.Errorf("%s requests = %d, want %d", label, got.Requests, wantRequests[label])
		}
	}
	if checkout := summary.Endpoints[0]; checkout.Failed != 20 || checkout.ErrorRate() != 1 {
		t.Errorf("checkout failed = %d (error rate %v), want all 20 requests failed", checkout.Failed, checkout.ErrorRate())
	}
	if login := summary.Endpoints[2]; login.Failed != 0 {
		t.Errorf("login failed = %d, want 0", login.Failed)
	}
	if summary.Endpoints[0].P95 < 35*time.Millisecond {
		t.Errorf("checkout P95 = %v, want at least the 40ms server delay (minus precision)", summary.Endpoints[0].P95)
	}
}
