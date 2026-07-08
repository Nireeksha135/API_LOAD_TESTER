package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/engine"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/exporter"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/reporter"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/utils"
)

// Run executes a full load test end-to-end from a parsed Config: it
// builds the collector and engine, runs the engine in the background
// while a live Dashboard renders progress in the foreground, prints
// the final static report, and writes any requested CSV/JSON exports.
//
// ctx governs the entire operation; cancelling it (e.g. from a
// SIGINT handler in main) triggers the engine's graceful shutdown
// path and Run still returns cleanly with whatever partial results
// were collected.
//
// stdout is used for the live dashboard and final report; stderr is
// used only for verbose per-request logging (via internal/utils.NewLogger).
func Run(ctx context.Context, cfg *config.Config, stdout io.Writer, stderr io.Writer) error {
	if cfg == nil {
		return fmt.Errorf("cli: config must not be nil")
	}

	// Raw per-request results are only needed (and only kept in
	// memory) if a CSV export was requested.
	var collectorOpts []metrics.Option
	if cfg.OutputCSV != "" {
		collectorOpts = append(collectorOpts, metrics.WithRawResults())
	}
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency, collectorOpts...)

	logger := utils.NewLogger(cfg.Verbose)
	// Route the verbose logger to stderr explicitly, in case a
	// caller-supplied stderr differs from os.Stderr (e.g. in tests).
	if cfg.Verbose {
		logger.SetOutput(stderr)
	}

	eng, err := engine.New(cfg, collector, logger)
	if err != nil {
		return fmt.Errorf("cli: failed to initialize engine: %w", err)
	}

	stdoutFile, dashboardEnabled := stdout.(interface {
		io.Writer
		Fd() uintptr
	})

	var (
		summaryResult modelsSummaryHolder
		runErr        error
	)

	dashCtx, cancelDash := context.WithCancel(ctx)
	defer cancelDash()

	done := make(chan struct{})
	go func() {
		defer close(done)
		summary, err := eng.Run(ctx)
		summaryResult.set(summary)
		runErr = err
		cancelDash()
	}()

	if dashboardEnabled {
		dash := reporter.NewDashboardFromWriter(cfg, collector, stdoutFile)
		dash.Run(dashCtx)
	} else {
		<-dashCtx.Done()
	}

	<-done

	if runErr != nil {
		return fmt.Errorf("cli: load test run failed: %w", runErr)
	}

	summary := summaryResult.get()

	reporter.PrintSummary(stdout, summary)

	if cfg.OutputCSV != "" {
		if err := exporter.ExportCSV(cfg.OutputCSV, collector.RawResults()); err != nil {
			return fmt.Errorf("cli: failed to export CSV: %w", err)
		}
		fmt.Fprintf(stdout, "CSV report written to %s\n", cfg.OutputCSV)
	}

	if cfg.OutputJSON != "" {
		if err := exporter.ExportJSON(cfg.OutputJSON, summary); err != nil {
			return fmt.Errorf("cli: failed to export JSON: %w", err)
		}
		fmt.Fprintf(stdout, "JSON summary written to %s\n", cfg.OutputJSON)
	}

	return nil
}
