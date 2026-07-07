// Package metrics provides a thread-safe collector that ingests
// individual request results produced by concurrent workers and
// aggregates them into a final statistical Summary (latency
// percentiles, status code breakdown, throughput, etc.).
package metrics

import (
	"sort"
	"sync"
	"time"

	"github.com/example/api-load-tester/internal/models"
)

// maxTrackedErrors caps how many distinct error messages the
// collector retains, preventing unbounded memory growth if a target
// is returning a huge variety of transport errors.
const maxTrackedErrors = 25

// Collector accumulates RequestResult values from many concurrent
// worker goroutines and produces an aggregated models.Summary. All
// exported methods are safe for concurrent use.
type Collector struct {
	mu sync.Mutex

	targetURL   string
	method      string
	concurrency int

	startTime time.Time
	endTime   time.Time

	totalRequests   int64
	successRequests int64
	failedRequests  int64
	totalBytesRead  int64

	statusCodeCounts map[int]int64
	latencies        []time.Duration

	errorSet   map[string]struct{}
	errorOrder []string
}

// NewCollector creates a new Collector for a run against targetURL
// using the given HTTP method and concurrency level. The method and
// concurrency values are purely descriptive and are copied verbatim
// into the final Summary.
func NewCollector(targetURL, method string, concurrency int) *Collector {
	return &Collector{
		targetURL:        targetURL,
		method:           method,
		concurrency:      concurrency,
		statusCodeCounts: make(map[int]int64),
		latencies:        make([]time.Duration, 0, 1024),
		errorSet:         make(map[string]struct{}, maxTrackedErrors),
		errorOrder:       make([]string, 0, maxTrackedErrors),
	}
}

// Start records the wall-clock start time of the load test run. It
// should be called exactly once, immediately before the first request
// is dispatched.
func (c *Collector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startTime = time.Now()
}

// Stop records the wall-clock end time of the load test run. It
// should be called exactly once, immediately after the last result
// has been recorded.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.endTime = time.Now()
}

// Record ingests a single RequestResult, updating all running totals
// under the collector's mutex. It is safe to call concurrently from
// any number of worker goroutines.
func (c *Collector) Record(result models.RequestResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalRequests++
	c.latencies = append(c.latencies, result.Latency)
	c.statusCodeCounts[result.StatusCode]++
	c.totalBytesRead += result.BytesRead

	if result.Success {
		c.successRequests++
	} else {
		c.failedRequests++
	}

	if result.Err != nil {
		msg := result.Err.Error()
		if _, exists := c.errorSet[msg]; !exists && len(c.errorOrder) < maxTrackedErrors {
			c.errorSet[msg] = struct{}{}
			c.errorOrder = append(c.errorOrder, msg)
		}
	}
}

// TotalRequestsSoFar returns the number of results recorded so far.
// It is intended for use by progress reporting (e.g. a live counter)
// while a run is still in progress.
func (c *Collector) TotalRequestsSoFar() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.totalRequests
}

// Summary computes and returns the final aggregated models.Summary
// from all results recorded so far. It may be called after Stop() to
// produce the definitive end-of-run report, or mid-run for a partial
// snapshot (e.g. on graceful shutdown via Ctrl+C).
func (c *Collector) Summary() models.Summary {
	c.mu.Lock()
	defer c.mu.Unlock()

	latencies := make([]time.Duration, len(c.latencies))
	copy(latencies, c.latencies)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var minLatency, maxLatency, sumLatency time.Duration
	if len(latencies) > 0 {
		minLatency = latencies[0]
		maxLatency = latencies[len(latencies)-1]
		for _, l := range latencies {
			sumLatency += l
		}
	}

	var meanLatency time.Duration
	if len(latencies) > 0 {
		meanLatency = time.Duration(int64(sumLatency) / int64(len(latencies)))
	}

	end := c.endTime
	if end.IsZero() {
		end = time.Now()
	}
	start := c.startTime
	if start.IsZero() {
		start = end
	}
	totalDuration := end.Sub(start)

	var requestsPerSecond, throughputBytesPerSecond float64
	if seconds := totalDuration.Seconds(); seconds > 0 {
		requestsPerSecond = float64(c.totalRequests) / seconds
		throughputBytesPerSecond = float64(c.totalBytesRead) / seconds
	}

	statusCodeCounts := make(map[int]int64, len(c.statusCodeCounts))
	for code, count := range c.statusCodeCounts {
		statusCodeCounts[code] = count
	}

	errs := make([]string, len(c.errorOrder))
	copy(errs, c.errorOrder)

	return models.Summary{
		TargetURL:                c.targetURL,
		Method:                   c.method,
		StartTime:                start,
		EndTime:                  end,
		TotalDuration:            totalDuration,
		Concurrency:              c.concurrency,
		TotalRequests:            c.totalRequests,
		SuccessRequests:          c.successRequests,
		FailedRequests:           c.failedRequests,
		StatusCodeCounts:         statusCodeCounts,
		MinLatency:               minLatency,
		MaxLatency:               maxLatency,
		MeanLatency:              meanLatency,
		P50:                      Percentile(latencies, 50),
		P95:                      Percentile(latencies, 95),
		P99:                      Percentile(latencies, 99),
		RequestsPerSecond:        requestsPerSecond,
		TotalBytesRead:           c.totalBytesRead,
		ThroughputBytesPerSecond: throughputBytesPerSecond,
		Errors:                   errs,
	}
}
