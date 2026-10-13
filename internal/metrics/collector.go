// Package metrics provides a thread-safe collector that aggregates
// request results into live snapshots and final summaries. Latency
// percentiles use a bounded-memory HDR histogram.
package metrics

import (
	"sort"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

const maxLatencyNanoseconds int64 = 1<<63 - 1

type Option func(*Collector)

// WithResultObserver registers a callback invoked synchronously for each recorded request.
func WithResultObserver(observer func(models.RequestResult) error) Option {
	return func(c *Collector) {
		c.resultObserver = observer
	}
}

// WithRawResults enables retention of every individual RequestResult in memory.
func WithRawResults() Option {
	return func(c *Collector) {
		c.keepRaw = true
	}
}

// WithTimeSeries enables compact per-second metrics retention.
func WithTimeSeries() Option {
	return func(c *Collector) {
		c.timeSeries = newTimeSeriesStore()
	}
}

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
	latencies        *hdrhistogram.Histogram
	errorCounts      map[string]int64
	resultObserver   func(models.RequestResult) error
	recordingErr     error
	workers          []*Collector

	keepRaw    bool
	rawResults []models.RequestResult
	timeSeries *timeSeriesStore
}

// NewCollector creates a new Collector for a run against targetURL using the given HTTP method and concurrency level.
func NewCollector(targetURL, method string, concurrency int, opts ...Option) *Collector {
	c := &Collector{
		targetURL:        targetURL,
		method:           method,
		concurrency:      concurrency,
		statusCodeCounts: make(map[int]int64),
		latencies:        hdrhistogram.New(1, maxLatencyNanoseconds, 2),
		errorCounts:      make(map[string]int64),
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.keepRaw {
		c.rawResults = make([]models.RequestResult, 0, 1024)
	}
	return c
}

// Start records the wall-clock start time of the load test run.
func (c *Collector) Start() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.startTime = time.Now()
}

// Stop records the wall-clock end time of the load test run.
func (c *Collector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.endTime = time.Now()
}

// NewWorkerCollectors creates independent collectors for request workers.
// The parent aggregates their live snapshots and final summary.
func (c *Collector) NewWorkerCollectors(count int) []*Collector {
	c.mu.Lock()
	defer c.mu.Unlock()

	workers := make([]*Collector, count)
	for i := range workers {
		var opts []Option
		if c.keepRaw {
			opts = append(opts, WithRawResults())
		}
		if c.resultObserver != nil {
			opts = append(opts, WithResultObserver(c.resultObserver))
		}
		worker := NewCollector(c.targetURL, c.method, c.concurrency, opts...)
		worker.startTime = c.startTime
		worker.timeSeries = c.timeSeries
		workers[i] = worker
	}
	c.workers = append(c.workers, workers...)
	return workers
}

func (c *Collector) Record(result models.RequestResult) {
	c.mu.Lock()

	c.totalRequests++
	latencyNanos := result.Latency.Nanoseconds()
	if latencyNanos < 1 {
		latencyNanos = 1
	}
	if err := c.latencies.RecordValue(latencyNanos); err != nil {
		panic("metrics: latency outside HDR histogram range")
	}
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
		c.errorCounts[result.Err.Error()]++
	}

	if c.keepRaw {
		c.rawResults = append(c.rawResults, result)
	}
	if c.timeSeries != nil {
		elapsed := result.Timestamp.Sub(c.startTime)
		if elapsed < 0 {
			elapsed = 0
		}
		second := int64(elapsed / time.Second)
		c.timeSeries.record(second, result)
	}
	observer := c.resultObserver
	c.mu.Unlock()

	if observer != nil {
		if err := observer(result); err != nil {
			c.mu.Lock()
			if c.recordingErr == nil {
				c.recordingErr = err
			}
			c.mu.Unlock()
		}
	}
}

// RecordingError returns the first error produced by a result observer.
func (c *Collector) RecordingError() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.recordingErr != nil {
		return c.recordingErr
	}
	for _, worker := range c.workers {
		worker.mu.Lock()
		err := worker.recordingErr
		worker.mu.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Collector) TotalRequestsSoFar() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.aggregateLocked(false).totalRequests
}

func (c *Collector) RawResults() []models.RequestResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := append([]models.RequestResult(nil), c.rawResults...)
	for _, worker := range c.workers {
		worker.mu.Lock()
		out = append(out, worker.rawResults...)
		worker.mu.Unlock()
	}
	if len(c.workers) > 0 {
		sort.Slice(out, func(i, j int) bool {
			return out[i].Timestamp.Before(out[j].Timestamp)
		})
	}
	return out
}

type aggregate struct {
	totalRequests    int64
	successRequests  int64
	failedRequests   int64
	totalBytesRead   int64
	sumLatency       time.Duration
	minLatency       time.Duration
	maxLatency       time.Duration
	statusCodeCounts map[int]int64
	errorCounts      map[string]int64
	latencies        *hdrhistogram.Histogram
	timeSeries       map[int64]*secondBucket
}

func (c *Collector) aggregateLocked(includeHistogram bool) aggregate {
	a := aggregate{
		statusCodeCounts: make(map[int]int64, len(c.statusCodeCounts)),
		errorCounts:      make(map[string]int64, len(c.errorCounts)),
	}
	if includeHistogram {
		a.latencies = hdrhistogram.New(1, maxLatencyNanoseconds, 2)
	}
	a.addCollectorLocked(c, includeHistogram)
	for _, worker := range c.workers {
		worker.mu.Lock()
		a.addCollectorLocked(worker, includeHistogram)
		worker.mu.Unlock()
	}
	if includeHistogram && c.timeSeries != nil {
		a.timeSeries = c.timeSeries.snapshot()
	}
	return a
}

func (a *aggregate) addCollectorLocked(c *Collector, includeHistogram bool) {
	if c.totalRequests > 0 {
		if a.totalRequests == 0 || c.minLatency < a.minLatency {
			a.minLatency = c.minLatency
		}
		if c.maxLatency > a.maxLatency {
			a.maxLatency = c.maxLatency
		}
	}
	a.totalRequests += c.totalRequests
	a.successRequests += c.successRequests
	a.failedRequests += c.failedRequests
	a.totalBytesRead += c.totalBytesRead
	a.sumLatency += c.sumLatency
	for code, count := range c.statusCodeCounts {
		a.statusCodeCounts[code] += count
	}
	for message, count := range c.errorCounts {
		a.errorCounts[message] += count
	}
	if includeHistogram {
		a.latencies.Merge(c.latencies)
	}
}

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
	data := c.aggregateLocked(false)

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
	if data.totalRequests > 0 {
		mean = time.Duration(int64(data.sumLatency) / data.totalRequests)
	}

	var rps float64
	if seconds := elapsed.Seconds(); seconds > 0 {
		rps = float64(data.totalRequests) / seconds
	}

	return Snapshot{
		Elapsed:           elapsed,
		TotalRequests:     data.totalRequests,
		SuccessRequests:   data.successRequests,
		FailedRequests:    data.failedRequests,
		RequestsPerSecond: rps,
		StatusCodeCounts:  data.statusCodeCounts,
		MinLatency:        data.minLatency,
		MaxLatency:        data.maxLatency,
		MeanLatency:       mean,
		TotalBytesRead:    data.totalBytesRead,
	}
}

func (c *Collector) Summary() models.Summary {
	c.mu.Lock()
	defer c.mu.Unlock()
	data := c.aggregateLocked(true)

	var meanLatency time.Duration
	if data.totalRequests > 0 {
		meanLatency = time.Duration(int64(data.sumLatency) / data.totalRequests)
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
		requestsPerSecond = float64(data.totalRequests) / seconds
		throughputBytesPerSecond = float64(data.totalBytesRead) / seconds
	}
	timeSeries := data.timeSeriesPoints(totalDuration)

	return models.Summary{
		TargetURL:                c.targetURL,
		Method:                   c.method,
		StartTime:                start,
		EndTime:                  end,
		TotalDuration:            totalDuration,
		Concurrency:              c.concurrency,
		TotalRequests:            data.totalRequests,
		SuccessRequests:          data.successRequests,
		FailedRequests:           data.failedRequests,
		StatusCodeCounts:         data.statusCodeCounts,
		MinLatency:               data.minLatency,
		MaxLatency:               data.maxLatency,
		MeanLatency:              meanLatency,
		P50:                      time.Duration(data.latencies.ValueAtQuantile(50)),
		P90:                      time.Duration(data.latencies.ValueAtQuantile(90)),
		P95:                      time.Duration(data.latencies.ValueAtQuantile(95)),
		P99:                      time.Duration(data.latencies.ValueAtQuantile(99)),
		P999:                     time.Duration(data.latencies.ValueAtQuantile(99.9)),
		RequestsPerSecond:        requestsPerSecond,
		TotalBytesRead:           data.totalBytesRead,
		ThroughputBytesPerSecond: throughputBytesPerSecond,
		Errors:                   data.errorCounts,
		TimeSeries:               timeSeries,
	}
}
