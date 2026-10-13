package engine

import (
	"context"
	"errors"
	"io"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

const maxCatchUpArrivalsPerWake = 10_000

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

func (e *Engine) runRateWorkers(ctx context.Context, collectors []*metrics.Collector, countLimit int64) (scheduled, dropped int64, scheduleDuration time.Duration) {
	jobs := make(chan int, len(collectors))
	for workerID := range collectors {
		jobs <- workerID
	}

	started := time.Now()

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
	nextArrival := started.Add(arrivalOffset(scheduled+1, e.cfg.RPS, e.cfg.RampUp, e.cfg.Concurrency))
	timer := time.NewTimer(time.Until(nextArrival))
	defer timer.Stop()
	for (countLimit < 0 || scheduled < countLimit) && ctx.Err() == nil {
		select {
		case <-ctx.Done():
			goto done
		case <-timer.C:
			catchUp := 0
			for catchUp < maxCatchUpArrivalsPerWake &&
				!nextArrival.After(time.Now()) &&
				(countLimit < 0 || scheduled < countLimit) &&
				ctx.Err() == nil {
				startRequest()
				catchUp++
				nextArrival = started.Add(arrivalOffset(scheduled+1, e.cfg.RPS, e.cfg.RampUp, e.cfg.Concurrency))
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
	scheduleDuration = time.Since(started)
	wg.Wait()
	return scheduled, dropped, scheduleDuration
}

func arrivalOffset(ordinal int64, targetRPS float64, rampUp time.Duration, concurrency int) time.Duration {
	if ordinal <= 0 {
		return 0
	}
	area := float64(ordinal)
	if rampUp <= 0 {
		area = float64(ordinal - 1)
		return durationFromSeconds(area / targetRPS)
	}

	rampSeconds := rampUp.Seconds()
	minimumFraction := 1 / float64(concurrency+1)
	minimumRampSeconds := rampSeconds * minimumFraction
	minimumArea := targetRPS * rampSeconds * minimumFraction * minimumFraction
	rampArea := targetRPS * rampSeconds * (1 + minimumFraction*minimumFraction) / 2

	var seconds float64
	switch {
	case area <= minimumArea:
		seconds = area / (targetRPS * minimumFraction)
	case area <= rampArea:
		seconds = math.Sqrt(minimumRampSeconds*minimumRampSeconds +
			2*rampSeconds*(area-minimumArea)/targetRPS)
	default:
		seconds = rampSeconds + (area-rampArea)/targetRPS
	}
	return durationFromSeconds(seconds)
}

func durationFromSeconds(seconds float64) time.Duration {
	const maxDuration = time.Duration(1<<63 - 1)
	nanoseconds := seconds * float64(time.Second)
	if nanoseconds >= float64(maxDuration) {
		return maxDuration
	}
	duration := time.Duration(nanoseconds)
	if duration < 0 {
		return maxDuration
	}
	return duration
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
