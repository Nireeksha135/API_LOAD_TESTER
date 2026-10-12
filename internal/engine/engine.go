package engine

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

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

// Run executes the load test to completion

func (e *Engine) Run(ctx context.Context) (models.Summary, error) {
	if e == nil {
		return models.Summary{}, fmt.Errorf("engine: Run called on nil Engine")
	}

	if e.cfg.Warmup > 0 {
		e.runWarmup(ctx)
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
	if e.cfg.RPS > 0 {
		countLimit := int64(-1)
		if !e.cfg.UseDuration {
			countLimit = int64(e.cfg.TotalRequests)
		}
		scheduledRequests, droppedRequests = e.runRateWorkers(runCtx, workerCollectors, countLimit)
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
	return summary, nil
}

// Config returns the Engine's underlying Config.

func (e *Engine) Config() *config.Config {
	return e.cfg
}
