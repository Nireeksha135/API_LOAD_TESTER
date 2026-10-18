package metrics

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func withinPrecision(got, want time.Duration) bool {
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	return float64(diff) <= float64(want)*0.02
}

func TestEndpointStatsGroupAndSortSlowestFirst(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 4, WithEndpointStats())
	c.Start()
	workers := c.NewWorkerCollectors(4)

	record := func(w int, endpoint string, latency time.Duration, success bool) {
		status := 200
		if !success {
			status = 500
		}
		workers[w].Record(models.RequestResult{
			StatusCode: status,
			Latency:    latency,
			Success:    success,
			WorkerID:   w,
			Endpoint:   endpoint,
		})
	}
	for i := 0; i < 100; i++ {
		record(i%4, "/login", 10*time.Millisecond, true)
		record(i%4, "/products", 50*time.Millisecond, true)
		record(i%4, "/checkout", 200*time.Millisecond, i%10 != 0) // 10% failures
	}
	// A cancelled request must not count as a failure.
	workers[0].Record(models.RequestResult{StatusCode: 0, Latency: 300 * time.Millisecond, Cancelled: true, WorkerID: 0, Endpoint: "/checkout"})
	c.Stop()

	endpoints := c.Summary().Endpoints
	if len(endpoints) != 3 {
		t.Fatalf("got %d endpoints, want 3: %+v", len(endpoints), endpoints)
	}
	wantOrder := []string{"/checkout", "/products", "/login"}
	for i, want := range wantOrder {
		if endpoints[i].Endpoint != want {
			t.Fatalf("endpoint[%d] = %q, want %q (slowest first); got %+v", i, endpoints[i].Endpoint, want, endpoints)
		}
	}

	checkout := endpoints[0]
	if checkout.Requests != 101 || checkout.Failed != 10 || checkout.Cancelled != 1 {
		t.Errorf("checkout counts = %d requests/%d failed/%d cancelled, want 101/10/1", checkout.Requests, checkout.Failed, checkout.Cancelled)
	}
	if got, want := checkout.ErrorRate(), 10.0/101.0; got < want-1e-9 || got > want+1e-9 {
		t.Errorf("checkout ErrorRate = %v, want %v", got, want)
	}
	if !withinPrecision(endpoints[1].P95, 50*time.Millisecond) || !withinPrecision(endpoints[1].P99, 50*time.Millisecond) {
		t.Errorf("products P95/P99 = %v/%v, want about 50ms", endpoints[1].P95, endpoints[1].P99)
	}
	if !withinPrecision(endpoints[2].MeanLatency, 10*time.Millisecond) {
		t.Errorf("login mean = %v, want about 10ms", endpoints[2].MeanLatency)
	}
	if endpoints[2].Failed != 0 || endpoints[2].ErrorRate() != 0 {
		t.Errorf("login should have no failures, got %+v", endpoints[2])
	}
}

func TestEndpointStatsDisabledByDefault(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1)
	c.Record(models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true, Endpoint: "/a"})
	if got := c.Summary().Endpoints; got != nil {
		t.Errorf("Endpoints = %+v, want nil when WithEndpointStats is not set", got)
	}
}

func TestEndpointStatsIgnoresResultsWithoutEndpoint(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1, WithEndpointStats())
	c.Record(models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true})
	if got := c.Summary().Endpoints; len(got) != 0 {
		t.Errorf("Endpoints = %+v, want none for results without an endpoint", got)
	}
}

func TestEndpointStatsConcurrentWorkers(t *testing.T) {
	const workerCount, perWorker = 32, 500
	c := NewCollector("http://example.com", "GET", workerCount, WithEndpointStats())
	c.Start()
	workers := c.NewWorkerCollectors(workerCount)

	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				endpoint := "/a"
				if i%2 == 1 {
					endpoint = "/b"
				}
				workers[w].Record(models.RequestResult{StatusCode: 200, Latency: time.Duration(i+1) * time.Microsecond, Success: true, WorkerID: w, Endpoint: endpoint})
			}
		}(w)
	}
	wg.Wait()
	c.Stop()

	endpoints := c.Summary().Endpoints
	if len(endpoints) != 2 {
		t.Fatalf("got %d endpoints, want 2", len(endpoints))
	}
	var total int64
	for _, e := range endpoints {
		total += e.Requests
		if e.Requests != workerCount*perWorker/2 {
			t.Errorf("%s requests = %d, want %d", e.Endpoint, e.Requests, workerCount*perWorker/2)
		}
	}
	if total != workerCount*perWorker {
		t.Errorf("total endpoint requests = %d, want %d", total, workerCount*perWorker)
	}
}

func TestEndpointStatsFoldsExcessEndpointsIntoOther(t *testing.T) {
	c := NewCollector("http://example.com", "GET", 1, WithEndpointStats())
	for i := 0; i < maxTrackedEndpoints+10; i++ {
		c.Record(models.RequestResult{StatusCode: 200, Latency: time.Millisecond, Success: true, Endpoint: fmt.Sprintf("/e%d", i)})
	}

	endpoints := c.Summary().Endpoints
	if len(endpoints) != maxTrackedEndpoints+1 {
		t.Fatalf("got %d endpoints, want %d tracked plus %q", len(endpoints), maxTrackedEndpoints, OtherEndpoint)
	}
	var other *models.EndpointSummary
	for i := range endpoints {
		if endpoints[i].Endpoint == OtherEndpoint {
			other = &endpoints[i]
		}
	}
	if other == nil || other.Requests != 10 {
		t.Fatalf("%q = %+v, want 10 folded requests", OtherEndpoint, other)
	}
}
