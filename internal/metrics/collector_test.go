package metrics

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func TestEndpointNormalization(t *testing.T) {
	cases := map[string]string{
		"https://api.example.com/products/123?token=secret":      "https://api.example.com/products/:id",
		"https://api.example.com/users/alice":                    "https://api.example.com/users/:id",
		"https://api.example.com/o/12ab34":                       "https://api.example.com/o/:id",
		"https://api.example.com/users/5f1d00000000000000007a7b": "https://api.example.com/users/:id",
		"https://api.example.com/orders/{{random}}/":             "https://api.example.com/orders/:id",
		"https://api.example.com/a/":                             "https://api.example.com/a",
		"/status/health":                                         "/status/health",
	}
	for input, want := range cases {
		if got := NormalizeEndpoint(input); got != want {
			t.Errorf("NormalizeEndpoint(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestEndpointCountIsBounded(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	for i := 0; i < 100; i++ {
		c.Record(models.RequestResult{Endpoint: fmt.Sprintf("http://example.com/endpoint-%c%c", 'a'+i/26, 'a'+i%26), Latency: time.Millisecond, Success: true})
	}
	got := c.Summary().Endpoints
	if len(got) > maxEndpointCount+1 {
		t.Fatalf("got %d endpoint buckets, maximum is %d", len(got), maxEndpointCount+1)
	}
	foundOther := false
	for _, endpoint := range got {
		foundOther = foundOther || endpoint.Endpoint == endpointOther
	}
	if !foundOther {
		t.Fatalf("overflow bucket missing: %#v", got)
	}
}

func TestEndpointCancelledRequestsAreShownAndExcludedFromErrorRate(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Record(models.RequestResult{Endpoint: "http://example.com/orders/1", Latency: time.Millisecond, Success: true})
	c.Record(models.RequestResult{Endpoint: "http://example.com/orders/2", Latency: time.Millisecond, Cancelled: true})
	endpoint := c.Summary().Endpoints[0]
	if endpoint.Requests != 2 || endpoint.CancelledRequests != 1 || endpoint.Errors != 0 || endpoint.ErrorRate != 0 {
		t.Fatalf("endpoint metrics = %#v", endpoint)
	}
}

func TestSnapshotSkipsEndpointHistogramSummary(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Record(models.RequestResult{Endpoint: "http://example.com/a", Latency: time.Millisecond, Success: true})
	c.mu.Lock()
	data := c.aggregateLocked(false)
	c.mu.Unlock()
	if data.endpoints != nil {
		t.Fatalf("snapshot aggregation retained endpoint summaries: %#v", data.endpoints)
	}
}

func TestEndpointMetricsAreSortedByP95(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Start()
	c.Record(models.RequestResult{Endpoint: "http://example.com/fast", Latency: time.Millisecond, Success: true})
	c.Record(models.RequestResult{Endpoint: "http://example.com/slow", Latency: 20 * time.Millisecond, Success: false})
	c.Stop()
	summary := c.Summary()
	if len(summary.Endpoints) != 2 || summary.Endpoints[0].Endpoint != "http://example.com/slow" {
		t.Fatalf("endpoint order = %#v", summary.Endpoints)
	}
	if summary.Endpoints[0].Requests != 1 || summary.Endpoints[0].Errors != 1 || summary.Endpoints[0].ErrorRate != 1 {
		t.Fatalf("endpoint error metrics = %#v", summary.Endpoints[0])
	}
}

func TestEndpointMetricsAggregateAcrossWorkers(t *testing.T) {
	parent := NewCollector("http://example.com", "GET", 2)
	parent.Start()
	workers := parent.NewWorkerCollectors(2)
	workers[0].Record(models.RequestResult{Endpoint: "http://example.com/users/alice", Latency: time.Millisecond, Success: true})
	workers[1].Record(models.RequestResult{Endpoint: "http://example.com/users/bob", Latency: 3 * time.Millisecond, Success: false})
	parent.Stop()
	summary := parent.Summary()
	if len(summary.Endpoints) != 1 || summary.Endpoints[0].Endpoint != "http://example.com/users/:id" || summary.Endpoints[0].Requests != 2 || summary.Endpoints[0].Errors != 1 {
		t.Fatalf("worker endpoint aggregation = %#v", summary.Endpoints)
	}
}

func TestPercentileBasic(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
	}

	tests := []struct {
		name string
		p    float64
		want time.Duration
	}{
		{"p0", 0, 10 * time.Millisecond},
		{"p50", 50, 30 * time.Millisecond},
		{"p100", 100, 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Percentile(latencies, tt.p)
			if got != tt.want {
				t.Errorf("Percentile(latencies, %v) = %v, want %v", tt.p, got, tt.want)
			}
		})
	}
}

func TestPercentileInterpolation(t *testing.T) {
	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
	}
	// rank = (95/100) * 3 = 2.85 -> interpolate between index 2 (30ms) and 3 (40ms)
	got := Percentile(latencies, 95)
	want := 30*time.Millisecond + time.Duration(0.85*float64(10*time.Millisecond))
	if got != want {
		t.Errorf("Percentile interpolation = %v, want %v", got, want)
	}
}

func TestPercentileEmpty(t *testing.T) {
	if got := Percentile(nil, 50); got != 0 {
		t.Errorf("Percentile(nil, 50) = %v, want 0", got)
	}
	if got := Percentile([]time.Duration{}, 99); got != 0 {
		t.Errorf("Percentile(empty, 99) = %v, want 0", got)
	}
}

func TestCollectorRecordAndSummary(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 4)
	c.Start()

	c.Record(models.RequestResult{StatusCode: 200, Latency: 100 * time.Millisecond, Success: true, BytesRead: 512})
	c.Record(models.RequestResult{StatusCode: 200, Latency: 200 * time.Millisecond, Success: true, BytesRead: 512})
	c.Record(models.RequestResult{StatusCode: 500, Latency: 50 * time.Millisecond, Success: false, BytesRead: 0, Err: errors.New("server error")})
	c.Record(models.RequestResult{StatusCode: 0, Latency: 10 * time.Millisecond, Success: false, Err: errors.New("connection refused")})

	time.Sleep(5 * time.Millisecond)
	c.Stop()

	summary := c.Summary()

	if summary.TotalRequests != 4 {
		t.Errorf("TotalRequests = %d, want 4", summary.TotalRequests)
	}
	if summary.SuccessRequests != 2 {
		t.Errorf("SuccessRequests = %d, want 2", summary.SuccessRequests)
	}
	if summary.FailedRequests != 2 {
		t.Errorf("FailedRequests = %d, want 2", summary.FailedRequests)
	}
	if summary.StatusCodeCounts[200] != 2 {
		t.Errorf("StatusCodeCounts[200] = %d, want 2", summary.StatusCodeCounts[200])
	}
	if summary.StatusCodeCounts[500] != 1 {
		t.Errorf("StatusCodeCounts[500] = %d, want 1", summary.StatusCodeCounts[500])
	}
	if summary.StatusCodeCounts[0] != 1 {
		t.Errorf("StatusCodeCounts[0] = %d, want 1", summary.StatusCodeCounts[0])
	}
	if summary.MinLatency != 10*time.Millisecond {
		t.Errorf("MinLatency = %v, want 10ms", summary.MinLatency)
	}
	if summary.MaxLatency != 200*time.Millisecond {
		t.Errorf("MaxLatency = %v, want 200ms", summary.MaxLatency)
	}
	if summary.TotalBytesRead != 1024 {
		t.Errorf("TotalBytesRead = %d, want 1024", summary.TotalBytesRead)
	}
	if len(summary.Errors) != 2 {
		t.Errorf("len(Errors) = %d, want 2", len(summary.Errors))
	}
	if summary.Errors["server error"] != 1 || summary.Errors["connection refused"] != 1 {
		t.Errorf("Errors = %v, want one occurrence of each error", summary.Errors)
	}
	if summary.TotalDuration <= 0 {
		t.Errorf("TotalDuration = %v, want > 0", summary.TotalDuration)
	}
	if summary.RequestsPerSecond <= 0 {
		t.Errorf("RequestsPerSecond = %v, want > 0", summary.RequestsPerSecond)
	}
}

func TestCollectorCountsCancelledRequestsWithoutTreatingThemAsFailures(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1, WithTimeSeries())
	c.Start()
	timestamp := time.Now()
	c.Record(models.RequestResult{
		StatusCode: 200,
		Latency:    5 * time.Millisecond,
		Timestamp:  timestamp,
		Success:    true,
	})
	c.Record(models.RequestResult{
		Latency:   100 * time.Millisecond,
		Timestamp: timestamp,
		Err:       errors.New("context deadline exceeded"),
		Cancelled: true,
	})
	c.Stop()

	summary := c.Summary()
	if summary.TotalRequests != 2 || summary.SuccessRequests != 1 || summary.FailedRequests != 0 || summary.CancelledRequests != 1 {
		t.Errorf("request counts = total:%d success:%d failed:%d cancelled:%d, want 2/1/0/1",
			summary.TotalRequests, summary.SuccessRequests, summary.FailedRequests, summary.CancelledRequests)
	}
	if summary.P95 < 100*time.Millisecond {
		t.Errorf("P95 = %s, want cancelled latency included", summary.P95)
	}
	if len(summary.Errors) != 0 {
		t.Errorf("Errors = %v, want cancelled request excluded", summary.Errors)
	}
	if len(summary.TimeSeries) == 0 || summary.TimeSeries[0].Requests != 2 || summary.TimeSeries[0].Errors != 0 {
		t.Errorf("TimeSeries = %+v, want 2 requests and 0 errors", summary.TimeSeries)
	}
	if snapshot := c.Snapshot(); snapshot.CancelledRequests != 1 {
		t.Errorf("Snapshot.CancelledRequests = %d, want 1", snapshot.CancelledRequests)
	}
}

func TestCollectorHistogramIsBoundedAndTracksPercentiles(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	initialSize := c.latencies.ByteSize()
	for i := 0; i < 10_000; i++ {
		latency := time.Duration(i%3+1) * time.Millisecond
		c.Record(models.RequestResult{StatusCode: 200, Latency: latency, Success: true})
	}

	summary := c.Summary()
	if c.latencies.ByteSize() != initialSize {
		t.Errorf("histogram size grew from %d to %d bytes", initialSize, c.latencies.ByteSize())
	}
	if c.latencies.TotalCount() != 10_000 {
		t.Errorf("histogram count = %d, want 10000", c.latencies.TotalCount())
	}
	if summary.P50 < time.Millisecond || summary.P50 > 3*time.Millisecond {
		t.Errorf("P50 = %v, want within observed range", summary.P50)
	}
	if summary.P95 < 2*time.Millisecond || summary.P95 > 3100*time.Microsecond {
		t.Errorf("P95 = %v, want near upper observed values", summary.P95)
	}
}

func TestCollectorHistogramRetainsLongLatencies(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Record(models.RequestResult{StatusCode: 200, Latency: 2 * time.Hour, Success: true})

	summary := c.Summary()
	if summary.MaxLatency != 2*time.Hour {
		t.Errorf("MaxLatency = %v, want actual 2h latency", summary.MaxLatency)
	}
	if summary.P95 < 2*time.Hour || summary.P95 > 2*time.Hour+2*time.Minute {
		t.Errorf("P95 = %v, want the 2h latency within histogram precision", summary.P95)
	}
	if size := c.latencies.ByteSize(); size > 64*1024 {
		t.Errorf("histogram size = %d bytes, want bounded size below 64 KiB", size)
	}
}

func TestCollectorCountsEveryError(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	for i := 0; i < 30; i++ {
		c.Record(models.RequestResult{Err: fmt.Errorf("failure %d", i)})
	}
	for i := 0; i < 4; i++ {
		c.Record(models.RequestResult{Err: errors.New("repeated failure")})
	}
	c.Record(models.RequestResult{Err: fmt.Errorf("Get \"http://example.com/items/1\": %w", errors.New("connection reset"))})
	c.Record(models.RequestResult{Err: fmt.Errorf("Get \"http://example.com/items/2\": %w", errors.New("connection reset"))})

	errors := c.Summary().Errors
	if len(errors) != 32 {
		t.Errorf("len(Errors) = %d, want 30 unique failures, one repeated failure, and one unwrapped cause", len(errors))
	}
	if errors["repeated failure"] != 4 {
		t.Errorf("Errors[repeated failure] = %d, want 4", errors["repeated failure"])
	}
	if errors["connection reset"] != 2 {
		t.Errorf("Errors[connection reset] = %d, want 2 URL-independent failures", errors["connection reset"])
	}
}

func TestCollectorReportsResultObserverErrors(t *testing.T) {
	wantErr := errors.New("CSV write failed")
	c := NewCollector("http://example.com", "GET", 1, WithResultObserver(func(models.RequestResult) error {
		return wantErr
	}))
	workers := c.NewWorkerCollectors(1)
	workers[0].Record(models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true})

	if err := c.RecordingError(); !errors.Is(err, wantErr) {
		t.Errorf("RecordingError() = %v, want %v", err, wantErr)
	}
}

func TestCollectorMergesPerSecondTimeSeries(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 2, WithTimeSeries())
	c.Start()
	workers := c.NewWorkerCollectors(2)
	if workers[0].timeSeries != workers[1].timeSeries {
		t.Fatal("worker collectors do not share the time-series store")
	}
	start := time.Now()
	workers[0].Record(models.RequestResult{
		StatusCode: 200,
		Latency:    25 * time.Millisecond,
		Timestamp:  start.Add(10 * time.Millisecond),
		Success:    true,
	})
	workers[1].Record(models.RequestResult{
		StatusCode: 500,
		Latency:    100 * time.Millisecond,
		Timestamp:  start.Add(20 * time.Millisecond),
		Success:    false,
	})
	c.Stop()

	series := c.Summary().TimeSeries
	if len(series) == 0 {
		t.Fatal("TimeSeries is empty, want at least the first-second bucket")
	}
	if series[0].Second != 0 || series[0].Requests != 2 || series[0].Errors != 1 {
		t.Errorf("first time-series point = %+v, want second 0 with 2 requests and 1 error", series[0])
	}
	if series[0].P95 < 100*time.Millisecond || series[0].P95 > 130*time.Millisecond {
		t.Errorf("per-second P95 = %v, want approximately 100ms", series[0].P95)
	}
}

func TestCollectorConcurrentRecord(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 10)
	c.Start()

	const n = 500
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			c.Record(models.RequestResult{
				StatusCode: 200,
				Latency:    time.Duration(i) * time.Microsecond,
				Success:    true,
				BytesRead:  10,
			})
		}(i)
	}
	wg.Wait()
	c.Stop()

	summary := c.Summary()
	if summary.TotalRequests != n {
		t.Errorf("TotalRequests = %d, want %d", summary.TotalRequests, n)
	}
	if summary.SuccessRequests != n {
		t.Errorf("SuccessRequests = %d, want %d", summary.SuccessRequests, n)
	}
	if summary.TotalBytesRead != n*10 {
		t.Errorf("TotalBytesRead = %d, want %d", summary.TotalBytesRead, n*10)
	}
}

func BenchmarkCollectorRecordParallel(b *testing.B) {
	c := NewCollector("http://example.com", "GET", 50)
	result := models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true}
	b.ReportAllocs()
	b.SetParallelism(4)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Record(result)
		}
	})
}

func BenchmarkCollectorRecordWorkerLocalParallel(b *testing.B) {
	const workersCount = 64
	parent := NewCollector("http://example.com", "GET", workersCount)
	workers := parent.NewWorkerCollectors(workersCount)
	result := models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true}
	var nextWorker atomic.Uint64
	b.ReportAllocs()
	b.SetParallelism(4)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		workerID := int(nextWorker.Add(1)-1) % workersCount
		worker := workers[workerID]
		for pb.Next() {
			worker.Record(result)
		}
	})
}

func TestCollectorEmptySummary(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Start()
	c.Stop()

	summary := c.Summary()
	if summary.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", summary.TotalRequests)
	}
	if summary.MinLatency != 0 || summary.MaxLatency != 0 {
		t.Errorf("expected zero min/max latency for empty summary, got min=%v max=%v", summary.MinLatency, summary.MaxLatency)
	}
}

func TestCollectorSnapshotBeforeStart(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	snap := c.Snapshot()
	if snap.Elapsed != 0 {
		t.Errorf("Elapsed = %v, want 0 before Start()", snap.Elapsed)
	}
	if snap.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d, want 0", snap.TotalRequests)
	}
}

func TestCollectorSnapshotDuringRun(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 2)
	c.Start()

	c.Record(models.RequestResult{StatusCode: 200, Latency: 10 * time.Millisecond, Success: true, BytesRead: 100})
	c.Record(models.RequestResult{StatusCode: 200, Latency: 30 * time.Millisecond, Success: true, BytesRead: 100})
	c.Record(models.RequestResult{StatusCode: 404, Latency: 20 * time.Millisecond, Success: false, BytesRead: 50})

	snap := c.Snapshot()

	if snap.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", snap.TotalRequests)
	}
	if snap.SuccessRequests != 2 {
		t.Errorf("SuccessRequests = %d, want 2", snap.SuccessRequests)
	}
	if snap.FailedRequests != 1 {
		t.Errorf("FailedRequests = %d, want 1", snap.FailedRequests)
	}
	if snap.MinLatency != 10*time.Millisecond {
		t.Errorf("MinLatency = %v, want 10ms", snap.MinLatency)
	}
	if snap.MaxLatency != 30*time.Millisecond {
		t.Errorf("MaxLatency = %v, want 30ms", snap.MaxLatency)
	}
	if snap.MeanLatency != 20*time.Millisecond {
		t.Errorf("MeanLatency = %v, want 20ms", snap.MeanLatency)
	}
	if snap.TotalBytesRead != 250 {
		t.Errorf("TotalBytesRead = %d, want 250", snap.TotalBytesRead)
	}
	if snap.StatusCodeCounts[200] != 2 {
		t.Errorf("StatusCodeCounts[200] = %d, want 2", snap.StatusCodeCounts[200])
	}
	if snap.Elapsed <= 0 {
		t.Errorf("Elapsed = %v, want > 0 while run is active", snap.Elapsed)
	}
}

func TestCollectorSnapshotMatchesSummaryAfterStop(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Start()
	c.Record(models.RequestResult{StatusCode: 200, Latency: 15 * time.Millisecond, Success: true, BytesRead: 20})
	c.Stop()

	snap := c.Snapshot()
	summary := c.Summary()

	if snap.TotalRequests != summary.TotalRequests {
		t.Errorf("Snapshot.TotalRequests = %d, Summary.TotalRequests = %d, want equal", snap.TotalRequests, summary.TotalRequests)
	}
	if snap.MinLatency != summary.MinLatency {
		t.Errorf("Snapshot.MinLatency = %v, Summary.MinLatency = %v, want equal", snap.MinLatency, summary.MinLatency)
	}
	if snap.Elapsed != summary.TotalDuration {
		t.Errorf("Snapshot.Elapsed = %v, Summary.TotalDuration = %v, want equal after Stop()", snap.Elapsed, summary.TotalDuration)
	}
}

func TestCollectorRawResultsDisabledByDefault(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Record(models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true})

	if raw := c.RawResults(); len(raw) != 0 {
		t.Errorf("RawResults() len = %d, want 0 when WithRawResults() not used", len(raw))
	}
}

func TestCollectorRawResultsEnabled(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1, WithRawResults())

	c.Record(models.RequestResult{StatusCode: 200, Latency: 5 * time.Millisecond, Success: true, WorkerID: 0})
	c.Record(models.RequestResult{StatusCode: 500, Latency: 8 * time.Millisecond, Success: false, WorkerID: 1, Err: errors.New("boom")})

	raw := c.RawResults()
	if len(raw) != 2 {
		t.Fatalf("RawResults() len = %d, want 2", len(raw))
	}
	if raw[0].StatusCode != 200 || raw[1].StatusCode != 500 {
		t.Errorf("RawResults() did not preserve insertion order/content: %+v", raw)
	}

	// Mutating the returned slice must not affect the collector's
	// internal state, since RawResults() returns a copy.
	raw[0].StatusCode = 999
	rawAgain := c.RawResults()
	if rawAgain[0].StatusCode != 200 {
		t.Errorf("RawResults() leaked internal slice: got %d, want 200", rawAgain[0].StatusCode)
	}
}
