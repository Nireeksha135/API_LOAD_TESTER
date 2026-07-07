package reporter

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func TestPrintSummaryContainsKeyFields(t *testing.T) {
	summary := models.Summary{
		TargetURL:                "https://api.example.com/users",
		Method:                   "GET",
		StartTime:                time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC),
		EndTime:                  time.Date(2026, 7, 7, 10, 0, 5, 0, time.UTC),
		TotalDuration:            5 * time.Second,
		Concurrency:              20,
		TotalRequests:            1000,
		SuccessRequests:          980,
		FailedRequests:           20,
		StatusCodeCounts:         map[int]int64{200: 980, 500: 15, 0: 5},
		MinLatency:               5 * time.Millisecond,
		MaxLatency:               300 * time.Millisecond,
		MeanLatency:              45 * time.Millisecond,
		P50:                      40 * time.Millisecond,
		P95:                      120 * time.Millisecond,
		P99:                      250 * time.Millisecond,
		RequestsPerSecond:        200.0,
		TotalBytesRead:           1024 * 1024,
		ThroughputBytesPerSecond: 204800,
		Errors:                   []string{"connection refused", "context deadline exceeded"},
	}

	var buf bytes.Buffer
	PrintSummary(&buf, summary)
	output := buf.String()

	mustContain := []string{
		"LOAD TEST REPORT",
		"https://api.example.com/users",
		"GET",
		"Total Requests:    1000",
		"Successful:        980",
		"Failed:            20",
		"P50:",
		"P95:",
		"P99:",
		"ERR (no response)",
		"connection refused",
		"context deadline exceeded",
	}
	for _, want := range mustContain {
		if !strings.Contains(output, want) {
			t.Errorf("PrintSummary() output missing %q\nfull output:\n%s", want, output)
		}
	}
}

func TestPrintSummaryNoErrorsSection(t *testing.T) {
	summary := models.Summary{
		TargetURL:         "http://localhost:8080",
		Method:             "GET",
		StatusCodeCounts:   map[int]int64{200: 10},
		TotalRequests:      10,
		SuccessRequests:    10,
	}

	var buf bytes.Buffer
	PrintSummary(&buf, summary)
	output := buf.String()

	if strings.Contains(output, "Errors observed:") {
		t.Errorf("PrintSummary() should omit the errors section when there are none, got:\n%s", output)
	}
}

func TestCenterText(t *testing.T) {
	got := centerText("HELLO", 11)
	want := "   HELLO"
	if got != want {
		t.Errorf("centerText(%q, 11) = %q, want %q", "HELLO", got, want)
	}

	longText := "this text is already wider than the width"
	if got := centerText(longText, 5); got != longText {
		t.Errorf("centerText() with text wider than width should return text unchanged, got %q", got)
	}
}
