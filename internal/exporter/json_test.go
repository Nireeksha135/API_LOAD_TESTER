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
		TargetURL:                "https://api.example.com",
		Method:                   "POST",
		StartTime:                time.Date(2026, 7, 7, 9, 0, 0, 0, time.UTC),
		EndTime:                  time.Date(2026, 7, 7, 9, 0, 10, 0, time.UTC),
		TotalDuration:            10 * time.Second,
		Concurrency:              25,
		TotalRequests:            2000,
		SuccessRequests:          1950,
		FailedRequests:           50,
		StatusCodeCounts:         map[int]int64{201: 1950, 429: 45, 0: 5},
		MinLatency:               2 * time.Millisecond,
		MaxLatency:               500 * time.Millisecond,
		MeanLatency:              60 * time.Millisecond,
		P50:                      50 * time.Millisecond,
		P95:                      180 * time.Millisecond,
		P99:                      400 * time.Millisecond,
		RequestsPerSecond:        200,
		TotalBytesRead:           2048000,
		ThroughputBytesPerSecond: 204800,
		Errors:                   []string{"timeout"},
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
	if parsed["p99_latency_ms"].(float64) != 400 {
		t.Errorf("p99_latency_ms = %v, want 400", parsed["p99_latency_ms"])
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

	errs, ok := parsed["errors"].([]interface{})
	if !ok || len(errs) != 1 || errs[0] != "timeout" {
		t.Errorf("errors = %v, want [\"timeout\"]", parsed["errors"])
	}
}

func TestExportJSONOmitsEmptyErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "summary_no_errors.json")

	summary := models.Summary{
		TargetURL:         "http://localhost",
		Method:             "GET",
		StatusCodeCounts:   map[int]int64{200: 10},
		TotalRequests:      10,
		SuccessRequests:    10,
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
