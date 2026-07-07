package exporter

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func TestExportCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "results.csv")

	results := []models.RequestResult{
		{
			StatusCode: 200,
			Latency:    15500 * time.Microsecond,
			Timestamp:  time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC),
			BytesRead:  1024,
			Success:    true,
			WorkerID:   0,
		},
		{
			StatusCode: 500,
			Latency:    42 * time.Millisecond,
			Timestamp:  time.Date(2026, 7, 7, 12, 0, 1, 0, time.UTC),
			BytesRead:  0,
			Success:    false,
			WorkerID:   1,
			Err:        errors.New("server error"),
		},
	}

	if err := ExportCSV(path, results); err != nil {
		t.Fatalf("ExportCSV() error = %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open exported CSV: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("failed to parse exported CSV: %v", err)
	}

	if len(rows) != 3 { // header + 2 data rows
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}

	if rows[0][0] != "timestamp" || rows[0][2] != "status_code" {
		t.Errorf("unexpected header row: %v", rows[0])
	}

	if rows[1][2] != "200" {
		t.Errorf("row 1 status_code = %q, want 200", rows[1][2])
	}
	if rows[1][3] != "15.500" {
		t.Errorf("row 1 latency_ms = %q, want 15.500", rows[1][3])
	}
	if rows[1][5] != "true" {
		t.Errorf("row 1 success = %q, want true", rows[1][5])
	}
	if rows[1][6] != "" {
		t.Errorf("row 1 error = %q, want empty", rows[1][6])
	}

	if rows[2][2] != "500" {
		t.Errorf("row 2 status_code = %q, want 500", rows[2][2])
	}
	if rows[2][6] != "server error" {
		t.Errorf("row 2 error = %q, want %q", rows[2][6], "server error")
	}
}

func TestExportCSVEmptyResults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.csv")

	if err := ExportCSV(path, nil); err != nil {
		t.Fatalf("ExportCSV() error = %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open exported CSV: %v", err)
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("failed to parse exported CSV: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("len(rows) = %d, want 1 (header only)", len(rows))
	}
}

func TestExportCSVInvalidPath(t *testing.T) {
	err := ExportCSV("/nonexistent-dir-xyz/results.csv", nil)
	if err == nil {
		t.Error("ExportCSV() with invalid path: expected error, got nil")
	}
}
