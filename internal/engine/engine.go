package engine

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

type Engine struct {
	cfg            *config.Config
	client         *http.Client
	template       *requestTemplate
	collector      *metrics.Collector
	logger         *log.Logger
	verbose        bool
	requestCounter atomic.Uint64
}

func New(cfg *config.Config, collector *metrics.Collector, logger *log.Logger) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("engine: config must not be nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("engine: invalid config: %w", err)
	}
	if collector == nil {
		return nil, fmt.Errorf("engine: collector must not be nil")
	}
	if logger == nil {
		logger = silentLogger()
	}

	return &Engine{
		cfg:       cfg,
		client:    NewHTTPClient(cfg),
		template:  newRequestTemplate(cfg),
		collector: collector,
		logger:    logger,
		verbose:   cfg.Verbose,
	}, nil
}

// Run executes the load test to completion.
func (e *Engine) Run(ctx context.Context) (models.Summary, error) {
	if e == nil {
		return models.Summary{}, fmt.Errorf("engine: Run called on nil Engine")
	}

	if e.cfg.Warmup > 0 {
		e.runWarmup(ctx)
		e.requestCounter.Store(0)
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if e.cfg.UseDuration {
		runCtx, cancel = context.WithTimeout(ctx, e.cfg.Duration)
		defer cancel()
	}
	e.collector.Start()

	var remainingPtr *int64
	if !e.cfg.UseDuration {
		remaining := int64(e.cfg.TotalRequests)
		remainingPtr = &remaining
	}

	workerCollectors := e.collector.NewWorkerCollectors(e.cfg.Concurrency)
	var scheduledRequests, droppedRequests int64
	var scheduleDuration time.Duration
	expectedScheduledRequests := int64(0)
	if e.cfg.RPS > 0 {
		countLimit := int64(-1)
		if !e.cfg.UseDuration {
			countLimit = int64(e.cfg.TotalRequests)
		}
		scheduledRequests, droppedRequests, scheduleDuration = e.runRateWorkers(runCtx, workerCollectors, countLimit)
		expectedScheduledRequests = expectedArrivals(scheduleDuration, e.cfg.RPS, e.cfg.RampUp, e.cfg.Concurrency)
	} else {
		e.runWorkers(runCtx, remainingPtr, workerCollectors, true)
	}
	e.collector.Stop()

	summary := e.collector.Summary()
	if e.cfg.RPS == 0 {
		scheduledRequests = summary.TotalRequests
	}
	summary.ScheduledRequests = scheduledRequests
	summary.DroppedRequests = droppedRequests
	summary.TargetRPS = e.cfg.RPS
	summary.ExpectedScheduledRequests = expectedScheduledRequests
	summary.ScheduleDuration = scheduleDuration
	if err := e.collector.RecordingError(); err != nil {
		return summary, fmt.Errorf("engine: failed to record request result: %w", err)
	}
	return summary, nil
}

func expectedArrivals(duration time.Duration, targetRPS float64, rampUp time.Duration, concurrency int) int64 {
	if duration <= 0 || targetRPS <= 0 {
		return 0
	}
	elapsed := duration.Seconds()
	if rampUp <= 0 {
		return int64(1 + elapsed*targetRPS)
	}

	rampSeconds := rampUp.Seconds()
	minimumFraction := 1 / float64(concurrency+1)
	minimumRampSeconds := rampSeconds * minimumFraction
	expected := 0.0
	switch {
	case elapsed <= minimumRampSeconds:
		expected = elapsed * targetRPS * minimumFraction
	case elapsed < rampSeconds:
		expected = minimumRampSeconds*targetRPS*minimumFraction +
			targetRPS*(elapsed*elapsed-minimumRampSeconds*minimumRampSeconds)/(2*rampSeconds)
	default:
		expected = minimumRampSeconds*targetRPS*minimumFraction +
			targetRPS*(rampSeconds*rampSeconds-minimumRampSeconds*minimumRampSeconds)/(2*rampSeconds) +
			targetRPS*(elapsed-rampSeconds)
	}
	return int64(expected)
}

// Config returns the Engine's underlying Config.
func (e *Engine) Config() *config.Config {
	return e.cfg
}
