package engine

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

func (e *Engine) doRequest(ctx context.Context, workerID int) models.RequestResult {
	start := time.Now()

	var requestID uint64
	if e.template.needsRequestID() {
		requestID = e.requestCounter.Add(1)
	}
	req, err := e.template.newRequest(ctx, requestID)
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

func (e *Engine) runWorker(ctx context.Context, workerID int, remaining *int64, wg *sync.WaitGroup, collector *metrics.Collector) {
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

		if !e.recordRequest(ctx, workerID, collector) {
			return
		}
	}
}

func (e *Engine) runWorkers(ctx context.Context, remaining *int64, collectors []*metrics.Collector, ramp bool) {
	var wg sync.WaitGroup
	wg.Add(len(collectors))
	for workerID, collector := range collectors {
		workerID, collector := workerID, collector
		go func() {
			if ramp && !e.waitForRamp(ctx, workerID, len(collectors)) {
				wg.Done()
				return
			}
			e.runWorker(ctx, workerID, remaining, &wg, collector)
		}()
	}
	wg.Wait()
}

func (e *Engine) waitForRamp(ctx context.Context, workerID, workerCount int) bool {
	if workerID == 0 || workerCount < 2 || e.cfg.RampUp <= 0 {
		return ctx.Err() == nil
	}
	delay := e.cfg.RampUp * time.Duration(workerID) / time.Duration(workerCount-1)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return ctx.Err() == nil
	}
}

func (e *Engine) runWarmup(ctx context.Context) {
	warmupCtx, cancel := context.WithTimeout(ctx, e.cfg.Warmup)
	defer cancel()
	collector := metrics.NewCollector(e.cfg.TargetURL, e.cfg.Method, e.cfg.Concurrency)
	workerCollectors := collector.NewWorkerCollectors(e.cfg.Concurrency)
	if e.cfg.RPS > 0 {
		e.runRateWorkers(warmupCtx, workerCollectors, -1)
		return
	}
	e.runWorkers(warmupCtx, nil, workerCollectors, false)
}

func (e *Engine) runRateWorkers(ctx context.Context, collectors []*metrics.Collector, countLimit int64) (scheduled, dropped int64) {
	jobs := make(chan int, len(collectors))
	for workerID := range collectors {
		jobs <- workerID
	}

	started := time.Now()
	intervalFor := func(elapsed time.Duration) time.Duration {
		rate := e.cfg.RPS
		if e.cfg.RampUp > 0 && elapsed < e.cfg.RampUp {
			fraction := float64(elapsed) / float64(e.cfg.RampUp)
			minimumFraction := 1 / float64(e.cfg.Concurrency+1)
			if fraction < minimumFraction {
				fraction = minimumFraction
			}
			rate *= fraction
		}
		return rateInterval(rate)
	}

	var wg sync.WaitGroup
	startRequest := func() {
		scheduled++
		select {
		case workerID := <-jobs:
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { jobs <- workerID }()
				e.recordRequest(ctx, workerID, collectors[workerID])
			}()
		default:
			dropped++
		}
	}

	if e.cfg.RampUp == 0 && ctx.Err() == nil && (countLimit < 0 || countLimit > 0) {
		startRequest()
	}
	nextArrival := started.Add(intervalFor(0))
	if e.cfg.RampUp == 0 && scheduled > 0 {
		nextArrival = started.Add(rateInterval(e.cfg.RPS))
	}
	timer := time.NewTimer(time.Until(nextArrival))
	defer timer.Stop()
	for (countLimit < 0 || scheduled < countLimit) && ctx.Err() == nil {
		select {
		case <-ctx.Done():
			goto done
		case <-timer.C:
			now := time.Now()
			for !nextArrival.After(now) && (countLimit < 0 || scheduled < countLimit) && ctx.Err() == nil {
				startRequest()
				elapsed := nextArrival.Sub(started)
				nextArrival = nextArrival.Add(intervalFor(elapsed))
			}
			if countLimit < 0 || scheduled < countLimit {
				delay := time.Until(nextArrival)
				if delay < 0 {
					delay = 0
				}
				timer.Reset(delay)
			}
		}
	}
done:
	wg.Wait()
	return scheduled, dropped
}

func rateInterval(rate float64) time.Duration {
	const maxDuration = time.Duration(1<<63 - 1)
	intervalNanos := float64(time.Second) / rate
	if intervalNanos >= float64(maxDuration) {
		return maxDuration
	}
	interval := time.Duration(intervalNanos)
	if interval < time.Nanosecond {
		return time.Nanosecond
	}
	return interval
}

func (e *Engine) recordRequest(ctx context.Context, workerID int, collector *metrics.Collector) bool {
	result := e.doRequest(ctx, workerID)
	if ctx.Err() != nil && (errors.Is(result.Err, context.Canceled) || errors.Is(result.Err, context.DeadlineExceeded)) {
		return false
	}
	collector.Record(result)
	if e.logger != nil && e.verbose {
		e.logger.Printf(
			"worker=%d status=%d latency=%s bytes=%d err=%v",
			workerID, result.StatusCode, result.Latency, result.BytesRead, result.Err,
		)
	}
	return true
}

func silentLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}
