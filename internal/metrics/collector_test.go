package metrics

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

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
	if summary.TotalDuration <= 0 {
		t.Errorf("TotalDuration = %v, want > 0", summary.TotalDuration)
	}
	if summary.RequestsPerSecond <= 0 {
		t.Errorf("RequestsPerSecond = %v, want > 0", summary.RequestsPerSecond)
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
