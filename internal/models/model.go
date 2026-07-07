// Package models defines the core data structures shared across the
// API Load Testing Framework: individual request results and the
// aggregated statistical summary produced at the end of a run.
package models

import (
	"time"
)

// RequestResult represents the outcome of a single HTTP request made
// during a load test run. It is produced by a worker goroutine and
// sent over a results channel to the metrics collector.
type RequestResult struct {
	// StatusCode is the HTTP status code returned by the server.
	// It is 0 if the request failed before receiving a response
	// (e.g. connection refused, timeout, DNS failure).
	StatusCode int

	// Latency is the wall-clock time taken to complete the request,
	// measured from just before the request was sent to just after
	// the response body was fully read (or the request failed).
	Latency time.Duration

	// Err holds the error encountered while performing the request,
	// if any. A nil Err with StatusCode == 0 should not occur; a
	// non-nil Err always implies the request did not succeed.
	Err error

	// Timestamp is the time at which the request was initiated.
	Timestamp time.Time

	// BytesRead is the number of response body bytes read for this
	// request. It is 0 for failed requests.
	BytesRead int64

	// Success indicates whether the request is considered successful.
	// A request is successful if it completed without a transport
	// error and returned an HTTP status code less than 400.
	Success bool

	// WorkerID identifies which worker goroutine produced this result.
	// Useful for debugging and verbose logging.
	WorkerID int
}

// Summary is the final aggregated set of statistics computed from all
// RequestResult values collected during a load test run. It is the
// primary data structure consumed by the reporter and exporters.
type Summary struct {
	// TargetURL is the URL that was load tested.
	TargetURL string `json:"target_url"`

	// Method is the HTTP method used for the load test.
	Method string `json:"method"`

	// StartTime is when the load test began.
	StartTime time.Time `json:"start_time"`

	// EndTime is when the load test finished.
	EndTime time.Time `json:"end_time"`

	// TotalDuration is the wall-clock time the entire run took.
	TotalDuration time.Duration `json:"total_duration_ns"`

	// Concurrency is the number of concurrent workers used.
	Concurrency int `json:"concurrency"`

	// TotalRequests is the total number of requests attempted.
	TotalRequests int64 `json:"total_requests"`

	// SuccessRequests is the number of requests that completed with
	// a non-error transport result and an HTTP status code < 400.
	SuccessRequests int64 `json:"success_requests"`

	// FailedRequests is the number of requests that either failed at
	// the transport level or returned an HTTP status code >= 400.
	FailedRequests int64 `json:"failed_requests"`

	// StatusCodeCounts maps HTTP status codes to the number of times
	// they were observed. A status code of 0 represents transport-
	// level failures (no HTTP response received).
	StatusCodeCounts map[int]int64 `json:"status_code_counts"`

	// MinLatency is the smallest observed request latency.
	MinLatency time.Duration `json:"min_latency_ns"`

	// MaxLatency is the largest observed request latency.
	MaxLatency time.Duration `json:"max_latency_ns"`

	// MeanLatency is the arithmetic mean of all observed latencies.
	MeanLatency time.Duration `json:"mean_latency_ns"`

	// P50 is the 50th percentile (median) latency.
	P50 time.Duration `json:"p50_latency_ns"`

	// P95 is the 95th percentile latency.
	P95 time.Duration `json:"p95_latency_ns"`

	// P99 is the 99th percentile latency.
	P99 time.Duration `json:"p99_latency_ns"`

	// RequestsPerSecond is the average throughput in requests/sec
	// computed as TotalRequests / TotalDuration.Seconds().
	RequestsPerSecond float64 `json:"requests_per_second"`

	// TotalBytesRead is the sum of all response body bytes read
	// across all requests.
	TotalBytesRead int64 `json:"total_bytes_read"`

	// ThroughputBytesPerSecond is TotalBytesRead / TotalDuration.Seconds().
	ThroughputBytesPerSecond float64 `json:"throughput_bytes_per_second"`

	// Errors is a de-duplicated, capped list of distinct error
	// messages encountered during the run, useful for diagnostics.
	Errors []string `json:"errors,omitempty"`
}

// LatencySample is a lightweight structure used internally by the
// metrics collector to keep track of individual latencies before
// percentiles are computed. Kept separate from RequestResult so the
// metrics package can store only what it needs for percentile math.
type LatencySample struct {
	Latency    time.Duration
	StatusCode int
	Success    bool
}
