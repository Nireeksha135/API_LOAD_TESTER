package exporter

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func TestExportTimeSeriesCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "series.csv")
	points := []models.TimeSeriesPoint{{
		Second: 2, Requests: 25, RPS: 25, P95: 150 * time.Millisecond, Errors: 1,
	}}
	if err := ExportTimeSeriesCSV(path, points); err != nil {
		t.Fatalf("ExportTimeSeriesCSV() error = %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV: %v", err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("read CSV: %v", err)
	}
	if len(rows) != 2 || rows[0][0] != "second" || rows[1][0] != "2" || rows[1][3] != "150.000" || rows[1][4] != "1" {
		t.Errorf("CSV rows = %v, want per-second metrics row", rows)
	}
}
