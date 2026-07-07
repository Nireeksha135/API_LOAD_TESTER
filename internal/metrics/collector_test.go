package metrics

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Nireeksha/API_LOAD_TESTER/internal/models"
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
