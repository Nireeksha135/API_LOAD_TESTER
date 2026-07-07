// Package exporter writes final load test results to disk in CSV
// (per-request detail) and JSON (aggregated summary) formats.
package exporter

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

// csvHeader defines the column order for the exported CSV file.
var csvHeader = []string{
	"timestamp",
	"worker_id",
	"status_code",
	"latency_ms",
	"bytes_read",
	"success",
	"error",
}

// ExportCSV writes one row per individual models.RequestResult to
// path, including a header row, in the order the results are
// supplied. Callers typically pass the output of
// (*metrics.Collector).RawResults(), which is only populated when the
// collector was constructed with metrics.WithRawResults().
func ExportCSV(path string, results []models.RequestResult) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("exporter: failed to create CSV file %q: %w", path, err)
	}
	defer f.Close()

	w := csv.NewWriter(f)

	if err := w.Write(csvHeader); err != nil {
		return fmt.Errorf("exporter: failed to write CSV header: %w", err)
	}

	for _, r := range results {
		errMsg := ""
		if r.Err != nil {
			errMsg = r.Err.Error()
		}

		row := []string{
			r.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
			strconv.Itoa(r.WorkerID),
			strconv.Itoa(r.StatusCode),
			strconv.FormatFloat(float64(r.Latency.Microseconds())/1000.0, 'f', 3, 64),
			strconv.FormatInt(r.BytesRead, 10),
			strconv.FormatBool(r.Success),
			errMsg,
		}

		if err := w.Write(row); err != nil {
			return fmt.Errorf("exporter: failed to write CSV row: %w", err)
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("exporter: error flushing CSV writer for %q: %w", path, err)
	}

	return nil
}
