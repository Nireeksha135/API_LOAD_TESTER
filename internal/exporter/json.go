package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

// jsonSummary mirrors models.Summary but expresses every duration as
// an explicit "_ms" float field instead of a raw time.Duration, which
// marshals to JSON as an opaque integer nanosecond count that is easy
// to misread without units.
type jsonSummary struct {
	TargetURL                string           `json:"target_url"`
	Method                   string           `json:"method"`
	StartTime                string           `json:"start_time"`
	EndTime                  string           `json:"end_time"`
	TotalDurationMS          float64          `json:"total_duration_ms"`
	Concurrency              int              `json:"concurrency"`
	TotalRequests            int64            `json:"total_requests"`
	SuccessRequests          int64            `json:"success_requests"`
	FailedRequests           int64            `json:"failed_requests"`
	StatusCodeCounts         map[string]int64 `json:"status_code_counts"`
	MinLatencyMS             float64          `json:"min_latency_ms"`
	MaxLatencyMS             float64          `json:"max_latency_ms"`
	MeanLatencyMS            float64          `json:"mean_latency_ms"`
	P50LatencyMS             float64          `json:"p50_latency_ms"`
	P95LatencyMS             float64          `json:"p95_latency_ms"`
	P99LatencyMS             float64          `json:"p99_latency_ms"`
	RequestsPerSecond        float64          `json:"requests_per_second"`
	TotalBytesRead           int64            `json:"total_bytes_read"`
	ThroughputBytesPerSecond float64          `json:"throughput_bytes_per_second"`
	Errors                   []string         `json:"errors,omitempty"`
}

// ExportJSON marshals the aggregated models.Summary to indented JSON
// and writes it to path (created or truncated, mode 0644).
func ExportJSON(path string, summary models.Summary) error {
	statusCodeCounts := make(map[string]int64, len(summary.StatusCodeCounts))
	for code, count := range summary.StatusCodeCounts {
		key := fmt.Sprintf("%d", code)
		if code == 0 {
			key = "ERR"
		}
		statusCodeCounts[key] = count
	}

	out := jsonSummary{
		TargetURL:                summary.TargetURL,
		Method:                   summary.Method,
		StartTime:                summary.StartTime.Format("2006-01-02T15:04:05.000Z07:00"),
		EndTime:                  summary.EndTime.Format("2006-01-02T15:04:05.000Z07:00"),
		TotalDurationMS:          durationToMS(summary.TotalDuration),
		Concurrency:              summary.Concurrency,
		TotalRequests:            summary.TotalRequests,
		SuccessRequests:          summary.SuccessRequests,
		FailedRequests:           summary.FailedRequests,
		StatusCodeCounts:         statusCodeCounts,
		MinLatencyMS:             durationToMS(summary.MinLatency),
		MaxLatencyMS:             durationToMS(summary.MaxLatency),
		MeanLatencyMS:            durationToMS(summary.MeanLatency),
		P50LatencyMS:             durationToMS(summary.P50),
		P95LatencyMS:             durationToMS(summary.P95),
		P99LatencyMS:             durationToMS(summary.P99),
		RequestsPerSecond:        summary.RequestsPerSecond,
		TotalBytesRead:           summary.TotalBytesRead,
		ThroughputBytesPerSecond: summary.ThroughputBytesPerSecond,
		Errors:                   summary.Errors,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("exporter: failed to marshal summary to JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("exporter: failed to write JSON file %q: %w", path, err)
	}

	return nil
}

// durationToMS converts a time.Duration to a float64 number of
// milliseconds for JSON output.
func durationToMS(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}
