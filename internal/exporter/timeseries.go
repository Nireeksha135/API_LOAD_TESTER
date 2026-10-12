package exporter

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

// ExportTimeSeriesCSV writes one row per second of aggregate metrics.
func ExportTimeSeriesCSV(path string, points []models.TimeSeriesPoint) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("exporter: failed to create time-series CSV file %q: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"second", "requests", "rps", "p95_latency_ms", "errors"}); err != nil {
		return fmt.Errorf("exporter: failed to write time-series CSV header: %w", err)
	}
	for _, point := range points {
		row := []string{
			strconv.FormatInt(point.Second, 10),
			strconv.FormatInt(point.Requests, 10),
			strconv.FormatFloat(point.RPS, 'f', 3, 64),
			strconv.FormatFloat(durationToMS(point.P95), 'f', 3, 64),
			strconv.FormatInt(point.Errors, 10),
		}
		if err := w.Write(row); err != nil {
			return fmt.Errorf("exporter: failed to write time-series CSV row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("exporter: error flushing time-series CSV writer for %q: %w", path, err)
	}
	return nil
}
