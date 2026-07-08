package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/engine"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/exporter"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
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
// path, and Run still returns cleanly with whatever partial results
// were collected up to that point.
//
// stdout is used for the live dashboard and final report; stderr is
// used only for verbose per-request logging (via utils.NewLogger).
// Both are *os.File (rather than io.Writer) because the live
// dashboard needs to detect whether stdout is an interactive TTY to
// decide between in-place ANSI redraws and plain append-only lines.
func Run(ctx context.Context, cfg *config.Config, stdout *os.File, stderr *os.File) error {
	if cfg == nil {
		return fmt.Errorf("cli: config must not be nil")
	}
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	// Raw per-request results are only needed (and only kept in
	// memory) if a CSV export was requested.
	var collectorOpts []metrics.Option
	if cfg.OutputCSV != "" {
		collectorOpts = append(collectorOpts, metrics.WithRawResults())
	}
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency, collectorOpts...)

	logger := utils.NewLogger(cfg.Verbose)
	if cfg.Verbose {
		logger.SetOutput(stderr)
	}

	eng, err := engine.New(cfg, collector, logger)
	if err != nil {
		return fmt.Errorf("cli: failed to initialize engine: %w", err)
	}

	// runResult carries the engine's outcome from the background
	// goroutine back to the main goroutine. It is only read after
	// <-done, so no additional synchronization is required beyond
	// the channel close itself (which establishes happens-before).
	type runResult struct {
		summary models.Summary
		err     error
	}
	resultCh := make(chan runResult, 1)

	dashCtx, cancelDash := context.WithCancel(ctx)
	defer cancelDash()

	go func() {
		summary, err := eng.Run(ctx)
		resultCh <- runResult{summary: summary, err: err}
		cancelDash()
	}()

	// Render the live dashboard in the foreground until the engine
	// goroutine signals completion via cancelDash().
	dash := reporter.NewDashboard(cfg, collector, stdout)
	dash.Run(dashCtx)

	result := <-resultCh
	if result.err != nil {
		return fmt.Errorf("cli: load test run failed: %w", result.err)
	}

	summary := result.summary

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
