package exporter

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

var csvHeader = []string{
	"start_time",
	"worker_id",
	"status_code",
	"latency_ms",
	"bytes_read",
	"success",
	"error",
}

// CSVWriter writes request results to a CSV file as they complete.
type CSVWriter struct {
	mu     sync.Mutex
	file   *os.File
	writer *csv.Writer
	err    error
	closed bool
}

// NewCSVWriter creates a CSV file and writes its header.
func NewCSVWriter(path string) (*CSVWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("exporter: failed to create CSV file %q: %w", path, err)
	}
	writer := &CSVWriter{file: file, writer: csv.NewWriter(file)}
	if err := writer.writer.Write(csvHeader); err != nil {
		writer.err = fmt.Errorf("exporter: failed to write CSV header: %w", err)
		if closeErr := file.Close(); closeErr != nil {
			writer.err = errors.Join(writer.err, fmt.Errorf("exporter: failed to close CSV file: %w", closeErr))
		}
		return nil, writer.err
	}
	return writer, nil
}

// Write appends one request result to the CSV file.
func (w *CSVWriter) Write(result models.RequestResult) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return fmt.Errorf("exporter: write to closed CSV file")
	}
	if w.err != nil {
		return w.err
	}
	errMsg := ""
	if result.Err != nil {
		errMsg = result.Err.Error()
	}
	row := []string{
		result.Timestamp.Format("2006-01-02T15:04:05.000Z07:00"),
		strconv.Itoa(result.WorkerID),
		strconv.Itoa(result.StatusCode),
		strconv.FormatFloat(float64(result.Latency.Microseconds())/1000.0, 'f', 3, 64),
		strconv.FormatInt(result.BytesRead, 10),
		strconv.FormatBool(result.Success),
		errMsg,
	}
	if err := w.writer.Write(row); err != nil {
		w.err = fmt.Errorf("exporter: failed to write CSV row: %w", err)
		return w.err
	}
	return nil
}

// Close flushes buffered rows and closes the CSV file.
func (w *CSVWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return w.err
	}
	w.closed = true
	w.writer.Flush()
	var errs []error
	if err := w.writer.Error(); err != nil {
		errs = append(errs, fmt.Errorf("exporter: error flushing CSV writer: %w", err))
	}
	if err := w.file.Close(); err != nil {
		errs = append(errs, fmt.Errorf("exporter: failed to close CSV file: %w", err))
	}
	w.err = errors.Join(w.err, errors.Join(errs...))
	return w.err
}

// ExportCSV writes one row per individual request result.
func ExportCSV(path string, results []models.RequestResult) error {
	writer, err := NewCSVWriter(path)
	if err != nil {
		return err
	}
	for _, result := range results {
		if err := writer.Write(result); err != nil {
			return errors.Join(err, writer.Close())
		}
	}
	return writer.Close()
}
