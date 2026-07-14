package engine

import (
	"context"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func (e *Engine) doRequest(ctx context.Context, workerID int) models.RequestResult {
	start := time.Now()

	req, err := e.template.newRequest(ctx)
	if err != nil {
		return models.RequestResult{
			Latency:   time.Since(start),
			Timestamp: start,
			Err:       err,
			Success:   false,
			WorkerID:  workerID,
		}
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return models.RequestResult{
			Latency:   time.Since(start),
			Timestamp: start,
			Err:       err,
			Success:   false,
			WorkerID:  workerID,
		}
	}
	defer resp.Body.Close()

	n, readErr := io.Copy(io.Discard, resp.Body)
	latency := time.Since(start)

	success := readErr == nil && resp.StatusCode < 400
	resultErr := readErr

	return models.RequestResult{
		StatusCode: resp.StatusCode,
		Latency:    latency,
		Timestamp:  start,
		BytesRead:  n,
		Success:    success,
		Err:        resultErr,
		WorkerID:   workerID,
	}
}

func (e *Engine) runWorker(ctx context.Context, workerID int, remaining *int64, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if remaining != nil {
			if atomic.AddInt64(remaining, -1) < 0 {
				return
			}
		}

		result := e.doRequest(ctx, workerID)
		e.collector.Record(result)

		if e.logger != nil && e.verbose {
			e.logger.Printf(
				"worker=%d status=%d latency=%s bytes=%d err=%v",
				workerID, result.StatusCode, result.Latency, result.BytesRead, result.Err,
			)
		}
	}
}

func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
