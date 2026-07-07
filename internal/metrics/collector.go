// Package metrics provides a thread-safe collector that ingests
// individual request results produced by concurrent workers and
// aggregates them into both a live-updating Snapshot (cheap, O(1)
// running counters for the terminal dashboard) and a final
// statistical Summary (latency percentiles, status code breakdown,
// throughput, etc.) once a run completes.
package metrics

import (
	"sort"
	"sync"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

// maxTrackedErrors caps how many distinct error messages the
// collector retains, preventing unbounded memory growth if a target
// is returning a huge variety of transport errors.
const maxTrackedErrors = 25

// Option configures optional Collector behavior at construction time.
type Option func(*Collector)

// WithRawResults enables retention of every individual RequestResult
// in memory (in addition to the aggregated running counters), which
// is required for exporting a full per-request CSV report. It is
// off by default to keep memory usage flat for very large runs.
func WithRawResults() Option {
	return func(c *Collector) {
		c.keepRaw = true
	}
}

// Collector accumulates RequestResult values from many concurrent
// worker goroutines and produces both cheap point-in-time Snapshots
// and a final aggregated models.Summary. All exported methods are
// safe for concurrent use.
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

	minLatency time.Duration
	maxLatency time.Duration
	sumLatency time.Duration

	statusCodeCounts map[int]int64
	latencies        []time.Duration

	errorSet   map[string]struct{}
	errorOrder []string

	keepRaw    bool
	rawResults []models.RequestResult
}

// NewCollector creates a new Collector for a run against targetURL
// using the given HTTP method and concurrency level. The method and
// concurrency values are purely descriptive and are copied verbatim
// into the final Summary. Optional behavior (such as raw per-request
// retention for CSV export) is enabled via Option values.
func NewCollector(targetURL, method string, concurrency int, opts ...Option) *Collector {
	c := &Collector{
		targetURL:        targetURL,
		method:           method,
		concurrency:      concurrency,
		statusCodeCounts: make(map[int]int64),
		latencies:        make([]time.Duration, 0, 1024),
		errorSet:         make(map[string]struct{}, maxTrackedErrors),
		errorOrder:       make([]string, 0, maxTrackedErrors),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.keepRaw {
		c.rawResults = make([]models.RequestResult, 0, 1024)
	}

	return c
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
	c.sumLatency += result.Latency

	if c.totalRequests == 1 {
		c.minLatency = result.Latency
		c.maxLatency = result.Latency
	} else {
		if result.Latency < c.minLatency {
			c.minLatency = result.Latency
		}
		if result.Latency > c.maxLatency {
			c.maxLatency = result.Latency
		}
	}

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

	if c.keepRaw {
		c.rawResults = append(c.rawResults, result)
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

// RawResults returns a copy of every individual RequestResult
// recorded so far. It only returns data if the Collector was
// constructed with WithRawResults(); otherwise it returns an empty
// slice. Used by the CSV exporter to produce a per-request report.
func (c *Collector) RawResults() []models.RequestResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]models.RequestResult, len(c.rawResults))
	copy(out, c.rawResults)
	return out
}

// Snapshot is a cheap, point-in-time view of the collector's running
// counters, computed in O(1) with respect to the number of requests
// recorded so far (it does not sort latencies or compute
// percentiles). It is intended to be polled frequently (e.g. every
// 200ms) by a live terminal dashboard without adding meaningful
// overhead to a run in progress.
type Snapshot struct {
	// Elapsed is the time since the run started (or the full run
	// duration, if the run has already stopped).
	Elapsed time.Duration

	TotalRequests     int64
	SuccessRequests   int64
	FailedRequests    int64
	RequestsPerSecond float64

	StatusCodeCounts map[int]int64

	MinLatency  time.Duration
	MaxLatency  time.Duration
	MeanLatency time.Duration

	TotalBytesRead int64
}

// Snapshot returns the current running statistics. Safe to call at
// any time, including before Start() (in which case Elapsed is 0)
// and after Stop() (in which case Elapsed reflects the final run
// duration).
func (c *Collector) Snapshot() Snapshot {
	c.mu.Lock()
	defer c.mu.Unlock()

	var elapsed time.Duration
	switch {
	case c.startTime.IsZero():
		elapsed = 0
	case !c.endTime.IsZero():
		elapsed = c.endTime.Sub(c.startTime)
	default:
		elapsed = time.Since(c.startTime)
	}

	var mean time.Duration
	if c.totalRequests > 0 {
		mean = time.Duration(int64(c.sumLatency) / c.totalRequests)
	}

	var rps float64
	if seconds := elapsed.Seconds(); seconds > 0 {
		rps = float64(c.totalRequests) / seconds
	}

	statusCodeCounts := make(map[int]int64, len(c.statusCodeCounts))
	for code, count := range c.statusCodeCounts {
		statusCodeCounts[code] = count
	}

	return Snapshot{
		Elapsed:           elapsed,
		TotalRequests:     c.totalRequests,
		SuccessRequests:   c.successRequests,
		FailedRequests:    c.failedRequests,
		RequestsPerSecond: rps,
		StatusCodeCounts:  statusCodeCounts,
		MinLatency:        c.minLatency,
		MaxLatency:        c.maxLatency,
		MeanLatency:       mean,
		TotalBytesRead:    c.totalBytesRead,
	}
}

// Summary computes and returns the final aggregated models.Summary
// from all results recorded so far. It may be called after Stop() to
// produce the definitive end-of-run report, or mid-run for a partial
// snapshot (e.g. on graceful shutdown via Ctrl+C). Unlike Snapshot,
// Summary sorts all recorded latencies to compute accurate
// percentiles, so it is more expensive and intended to be called once
// (or a handful of times), not polled in a tight loop.
func (c *Collector) Summary() models.Summary {
	c.mu.Lock()
	defer c.mu.Unlock()

	latencies := make([]time.Duration, len(c.latencies))
	copy(latencies, c.latencies)
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var meanLatency time.Duration
	if c.totalRequests > 0 {
		meanLatency = time.Duration(int64(c.sumLatency) / c.totalRequests)
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
		MinLatency:               c.minLatency,
		MaxLatency:               c.maxLatency,
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
