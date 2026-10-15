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
	"cancelled",
	"error",
}

// CSVWriter writes request results to a CSV file as they complete.
type CSVWriter struct {
	mu     sync.RWMutex
	file   *os.File
	writer *csv.Writer
	rows   chan models.RequestResult
	done   chan struct{}
	err    error
	closed bool
}

// NewCSVWriter creates a CSV file and writes its header.
func NewCSVWriter(path string) (*CSVWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("exporter: failed to create CSV file %q: %w", path, err)
	}
	writer := &CSVWriter{
		file:   file,
		writer: csv.NewWriter(file),
		rows:   make(chan models.RequestResult, 4096),
		done:   make(chan struct{}),
	}
	if err := writer.writer.Write(csvHeader); err != nil {
		writeErr := fmt.Errorf("exporter: failed to write CSV header: %w", err)
		if closeErr := file.Close(); closeErr != nil {
			writeErr = errors.Join(writeErr, fmt.Errorf("exporter: failed to close CSV file: %w", closeErr))
		}
		return nil, writeErr
	}
	go writer.writeRows()
	return writer, nil
}

// Write appends one request result to the CSV file.
func (w *CSVWriter) Write(result models.RequestResult) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.closed {
		return fmt.Errorf("exporter: write to closed CSV file")
	}
	w.rows <- result
	return nil
}

func (w *CSVWriter) writeRows() {
	defer close(w.done)
	for result := range w.rows {
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
			strconv.FormatBool(result.Cancelled),
			errMsg,
		}
		if err := w.writer.Write(row); err != nil && w.err == nil {
			w.err = fmt.Errorf("exporter: failed to write CSV row: %w", err)
		}
	}
	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		w.err = errors.Join(w.err, fmt.Errorf("exporter: error flushing CSV writer: %w", err))
	}
	if err := w.file.Close(); err != nil {
		w.err = errors.Join(w.err, fmt.Errorf("exporter: failed to close CSV file: %w", err))
	}
}

// Close flushes buffered rows and closes the CSV file.
func (w *CSVWriter) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		<-w.done
		return w.err
	}
	w.closed = true
	close(w.rows)
	w.mu.Unlock()
	<-w.done
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
