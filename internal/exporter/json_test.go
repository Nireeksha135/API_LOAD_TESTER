package exporter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func TestExportJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "summary.json")

	summary := models.Summary{
		TargetURL:                 "https://api.example.com",
		Method:                    "POST",
		StartTime:                 time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC),
		EndTime:                   time.Date(2026, 7, 7, 9, 0, 10, 0, time.UTC),
		TotalDuration:             10 * time.Second,
		Concurrency:               25,
		TotalRequests:             2000,
		ScheduledRequests:         2100,
		ExpectedScheduledRequests: 2200,
		ScheduleDuration:          9 * time.Second,
		SuccessRequests:           1950,
		FailedRequests:            45,
		CancelledRequests:         5,
		StatusCodeCounts:          map[int]int64{201: 1950, 429: 45, 0: 5},
		MinLatency:                2 * time.Millisecond,
		MaxLatency:                500 * time.Millisecond,
		MeanLatency:               60 * time.Millisecond,
		P50:                       50 * time.Millisecond,
		P90:                       120 * time.Millisecond,
		P95:                       180 * time.Millisecond,
		P99:                       400 * time.Millisecond,
		P999:                      450 * time.Millisecond,
		RequestsPerSecond:         200,
		TotalBytesRead:            2048000,
		ThroughputBytesPerSecond:  204800,
		Errors:                    map[string]int64{"timeout": 4},
		Endpoints:                 []models.EndpointSummary{{Endpoint: "https://api.example.com/users/:id", Requests: 10, Errors: 1, CancelledRequests: 2, ErrorRate: 0.125, MeanLatency: time.Millisecond, P95: 2 * time.Millisecond, P99: 3 * time.Millisecond}},
		TimeSeries: []models.TimeSeriesPoint{{
			Second: 1, Requests: 200, RPS: 200, P95: 120 * time.Millisecond, Errors: 3,
		}},
	}

	if err := ExportJSON(path, summary); err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exported JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse exported JSON: %v", err)
	}

	if parsed["target_url"] != "https://api.example.com" {
		t.Errorf("target_url = %v, want %q", parsed["target_url"], "https://api.example.com")
	}
	if parsed["total_requests"].(float64) != 2000 {
		t.Errorf("total_requests = %v, want 2000", parsed["total_requests"])
	}
	if parsed["cancelled_requests"].(float64) != 5 {
		t.Errorf("cancelled_requests = %v, want 5", parsed["cancelled_requests"])
	}
	if parsed["expected_scheduled_requests"].(float64) != 2200 {
		t.Errorf("expected_scheduled_requests = %v, want 2200", parsed["expected_scheduled_requests"])
	}
	if parsed["schedule_duration_ms"].(float64) != 9000 {
		t.Errorf("schedule_duration_ms = %v, want 9000", parsed["schedule_duration_ms"])
	}
	if parsed["total_duration_ns"].(float64) != float64(10*time.Second) || parsed["total_duration_human"] != "10s" {
		t.Errorf("total duration JSON fields = %v / %v, want 10000000000 / 10s", parsed["total_duration_ns"], parsed["total_duration_human"])
	}
	if parsed["p90_latency_ms"].(float64) != 120 || parsed["p999_latency_ms"].(float64) != 450 {
		t.Errorf("additional percentiles = P90:%v P99.9:%v, want 120/450ms", parsed["p90_latency_ms"], parsed["p999_latency_ms"])
	}
	if parsed["p99_latency_ms"].(float64) != 400 {
		t.Errorf("p99_latency_ms = %v, want 400", parsed["p99_latency_ms"])
	}
	endpoints, ok := parsed["endpoints"].([]interface{})
	if !ok || len(endpoints) != 1 {
		t.Fatalf("endpoints = %v", parsed["endpoints"])
	}
	endpoint := endpoints[0].(map[string]interface{})
	if endpoint["cancelled_requests"] != float64(2) || endpoint["error_rate"] != 0.125 {
		t.Errorf("endpoint JSON metrics = %v", endpoint)
	}
	series, ok := parsed["time_series"].([]interface{})
	if !ok || len(series) != 1 || series[0].(map[string]interface{})["p95_latency_ms"] != float64(120) {
		t.Errorf("time_series = %v, want per-second P95 of 120ms", parsed["time_series"])
	}

	statusCodes, ok := parsed["status_code_counts"].(map[string]interface{})
	if !ok {
		t.Fatalf("status_code_counts is not an object: %v", parsed["status_code_counts"])
	}
	if statusCodes["ERR"].(float64) != 5 {
		t.Errorf("status_code_counts[ERR] = %v, want 5", statusCodes["ERR"])
	}
	if statusCodes["201"].(float64) != 1950 {
		t.Errorf("status_code_counts[201] = %v, want 1950", statusCodes["201"])
	}

	errs, ok := parsed["errors"].(map[string]interface{})
	if !ok || len(errs) != 1 || errs["timeout"] != float64(4) {
		t.Errorf("errors = %v, want {\"timeout\": 4}", parsed["errors"])
	}
}

func TestExportJSONOmitsEmptyErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "summary_no_errors.json")

	summary := models.Summary{
		TargetURL:        "http://localhost",
		Method:           "GET",
		StatusCodeCounts: map[int]int64{200: 10},
		TotalRequests:    10,
		SuccessRequests:  10,
	}

	if err := ExportJSON(path, summary); err != nil {
		t.Fatalf("ExportJSON() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read exported JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to parse exported JSON: %v", err)
	}

	if _, exists := parsed["errors"]; exists {
		t.Errorf("expected 'errors' field to be omitted when empty, got: %v", parsed["errors"])
	}
}

func TestExportJSONInvalidPath(t *testing.T) {
	err := ExportJSON("/nonexistent-dir-xyz/summary.json", models.Summary{})
	if err == nil {
		t.Error("ExportJSON() with invalid path: expected error, got nil")
	}
}

func TestCompareJSONReportsRegressionDirectionAndThreshold(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	current := filepath.Join(dir, "current.json")
	if err := os.WriteFile(base, []byte(`{"p95_latency_ms":100,"p99_latency_ms":200,"requests_per_second":100,"total_requests":100,"failed_requests":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte(`{"p95_latency_ms":124,"p99_latency_ms":220,"requests_per_second":80,"total_requests":100,"failed_requests":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	results, err := CompareJSON(base, current, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 || !results[0].Worse || !results[0].ExceedsLimit || results[0].ChangeValue < 23.9 || results[0].ChangeValue > 24.1 || !results[2].Worse || !results[2].ExceedsLimit || !results[3].Worse || !results[3].ExceedsLimit {
		t.Fatalf("comparison = %#v", results)
	}
}

func TestCompareJSONZeroBaselineImprovementAndThroughputOnlyRegression(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	current := filepath.Join(dir, "current.json")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(base, `{"target_url":"https://old.example","target_rps":100,"concurrency":10,"p95_latency_ms":0,"p99_latency_ms":100,"requests_per_second":100,"total_requests":10,"failed_requests":0}`)
	write(current, `{"target_url":"https://new.example","target_rps":80,"concurrency":20,"p95_latency_ms":4,"p99_latency_ms":50,"requests_per_second":75,"total_requests":10,"failed_requests":1}`)
	results, warnings, err := CompareJSONDetailed(base, current, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 3 {
		t.Fatalf("warnings = %v, want target, target RPS and concurrency warnings", warnings)
	}
	if !results[0].Worse || !results[0].ExceedsLimit {
		t.Errorf("zero baseline latency should be a limit-breaking regression: %#v", results[0])
	}
	if results[1].Worse || results[1].ExceedsLimit {
		t.Errorf("latency improvement marked as regression: %#v", results[1])
	}
	if !results[2].ExceedsLimit {
		t.Errorf("throughput-only regression missed: %#v", results[2])
	}
	if results[3].ChangeUnit != "pp" || results[3].ChangeValue != 10 || results[3].ExceedsLimit {
		t.Errorf("zero-baseline error rate should use percentage-point delta and respect limit: %#v", results[3])
	}
}

func TestCompareJSONAllImprovements(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	current := filepath.Join(dir, "current.json")
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(base, `{"p95_latency_ms":100,"p99_latency_ms":200,"requests_per_second":100,"total_requests":100,"failed_requests":10}`)
	write(current, `{"p95_latency_ms":80,"p99_latency_ms":150,"requests_per_second":120,"total_requests":100,"failed_requests":5}`)
	results, err := CompareJSON(base, current, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Worse || result.ExceedsLimit {
			t.Errorf("improvement reported as regression: %#v", result)
		}
	}
}
