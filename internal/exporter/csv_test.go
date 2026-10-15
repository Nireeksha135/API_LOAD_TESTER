package exporter

import (
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"sync"
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

	if rows[0][0] != "start_time" || rows[0][2] != "status_code" {
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
	if rows[1][6] != "false" || rows[1][7] != "" {
		t.Errorf("row 1 cancelled/error = %q/%q, want false/empty", rows[1][6], rows[1][7])
	}

	if rows[2][2] != "500" {
		t.Errorf("row 2 status_code = %q, want 500", rows[2][2])
	}
	if rows[2][6] != "false" || rows[2][7] != "server error" {
		t.Errorf("row 2 cancelled/error = %q/%q, want false/server error", rows[2][6], rows[2][7])
	}
}

func TestCSVWriterMarksCancelledRequests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancelled.csv")
	if err := ExportCSV(path, []models.RequestResult{{Cancelled: true}}); err != nil {
		t.Fatalf("ExportCSV() error = %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV: %v", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	if rows[0][6] != "cancelled" || rows[1][6] != "true" {
		t.Errorf("cancelled column = header:%q value:%q, want cancelled/true", rows[0][6], rows[1][6])
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

func TestCSVWriterConcurrentResults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.csv")
	writer, err := NewCSVWriter(path)
	if err != nil {
		t.Fatalf("NewCSVWriter() error = %v", err)
	}

	const count = 100
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- writer.Write(models.RequestResult{
				StatusCode: 200,
				Timestamp:  time.Now(),
				Success:    true,
				WorkerID:   i,
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("CSVWriter.Write() error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("CSVWriter.Close() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV: %v", err)
	}
	defer file.Close()
	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	if len(rows) != count+1 {
		t.Errorf("CSV rows = %d, want %d", len(rows), count+1)
	}
}

func TestExportCSVInvalidPath(t *testing.T) {
	err := ExportCSV("/nonexistent-dir-xyz/results.csv", nil)
	if err == nil {
		t.Error("ExportCSV() with invalid path: expected error, got nil")
	}
}
