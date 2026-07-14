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
