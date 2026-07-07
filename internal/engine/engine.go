package engine

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
)

// Engine ties together a Config, an HTTP client, a request template,
// and a metrics.Collector to execute a full load test run: spinning
// up a worker pool, dispatching requests (either a fixed count or for
// a fixed duration), and producing a final models.Summary.
type Engine struct {
	cfg       *config.Config
	client    *http.Client
	template  *requestTemplate
	collector *metrics.Collector
	logger    *log.Logger
	verbose   bool
}

// New constructs an Engine from a Config and a metrics.Collector. The
// Config is validated before anything else is built; a non-nil error
// means the Engine was not constructed and must not be used.
//
// If logger is nil, a silent logger is used and verbose per-request
// logging (cfg.Verbose) has no effect.
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

// Run executes the load test to completion (or until ctx is
// cancelled, e.g. via a SIGINT-triggered context for graceful
// shutdown) and returns the resulting models.Summary.
//
// In duration mode, Run derives a child context bounded by
// cfg.Duration so that workers stop automatically once the time
// budget is exhausted, in addition to responding to cancellation of
// the parent ctx.
//
// In count mode, Run guarantees that exactly cfg.TotalRequests
// requests are attempted in total (spread across cfg.Concurrency
// workers), unless ctx is cancelled first, in which case whatever
// partial results were collected are still returned alongside a nil
// error -- callers can distinguish a graceful early stop by checking
// ctx.Err() themselves.
func (e *Engine) Run(ctx context.Context) (models.Summary, error) {
	if e == nil {
		return models.Summary{}, fmt.Errorf("engine: Run called on nil Engine")
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

	var wg sync.WaitGroup
	wg.Add(e.cfg.Concurrency)
	for i := 0; i < e.cfg.Concurrency; i++ {
		go e.runWorker(runCtx, i, remainingPtr, &wg)
	}

	wg.Wait()
	e.collector.Stop()

	return e.collector.Summary(), nil
}

// Config returns the Engine's underlying Config. Useful for reporters
// and exporters that need run metadata (target URL, method, etc.)
// alongside the Summary.
func (e *Engine) Config() *config.Config {
	return e.cfg
}
