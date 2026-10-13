package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/engine"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/exporter"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/reporter"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/utils"
)

type ThresholdError struct {
	Failures []string
}

func (e *ThresholdError) Error() string {
	return "thresholds failed: " + strings.Join(e.Failures, "; ")
}

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

	var csvWriter *exporter.CSVWriter
	var collectorOpts []metrics.Option
	if cfg.OutputCSV != "" {
		collectorOpts = append(collectorOpts, metrics.WithResultObserver(func(result models.RequestResult) error {
			return csvWriter.Write(result)
		}))
	}
	if cfg.OutputJSON != "" || cfg.OutputTimeSeriesCSV != "" {
		collectorOpts = append(collectorOpts, metrics.WithTimeSeries())
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
	if cfg.OutputCSV != "" {
		csvWriter, err = exporter.NewCSVWriter(cfg.OutputCSV)
		if err != nil {
			return fmt.Errorf("cli: failed to initialize CSV export: %w", err)
		}
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
	var csvCloseErr error
	if csvWriter != nil {
		csvCloseErr = csvWriter.Close()
	}
	if result.err != nil {
		return fmt.Errorf("cli: load test run failed: %w", errors.Join(result.err, csvCloseErr))
	}
	summary := result.summary

	reporter.PrintSummary(stdout, summary)

	if cfg.OutputCSV != "" && csvCloseErr == nil {
		fmt.Fprintf(stdout, "CSV report written to %s\n", cfg.OutputCSV)
	}

	var exportErr error
	if cfg.OutputJSON != "" {
		if err := exporter.ExportJSON(cfg.OutputJSON, summary); err != nil {
			exportErr = errors.Join(exportErr, fmt.Errorf("cli: failed to export JSON: %w", err))
		} else {
			fmt.Fprintf(stdout, "JSON summary written to %s\n", cfg.OutputJSON)
		}
	}
	if cfg.OutputTimeSeriesCSV != "" {
		if err := exporter.ExportTimeSeriesCSV(cfg.OutputTimeSeriesCSV, summary.TimeSeries); err != nil {
			exportErr = errors.Join(exportErr, fmt.Errorf("cli: failed to export time series CSV: %w", err))
		} else {
			fmt.Fprintf(stdout, "Time series CSV written to %s\n", cfg.OutputTimeSeriesCSV)
		}
	}

	failures := checkThresholds(summary, cfg)
	var thresholdErr error
	if len(failures) > 0 {
		fmt.Fprintln(stdout, "Thresholds failed:")
		for _, failure := range failures {
			fmt.Fprintf(stdout, "  - %s\n", failure)
		}
		thresholdErr = &ThresholdError{Failures: failures}
	}
	if cfg.MaxP95 != nil || cfg.MaxErrorRate != nil {
		if len(failures) == 0 {
			fmt.Fprintln(stdout, "Thresholds passed")
		}
	}

	var runErr error
	if csvCloseErr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("cli: failed to finalize CSV export: %w", csvCloseErr))
	}
	if exportErr != nil {
		runErr = errors.Join(runErr, exportErr)
	}

	return errors.Join(runErr, thresholdErr)
}

func checkThresholds(summary models.Summary, cfg *config.Config) []string {
	var failures []string
	if (cfg.MaxP95 != nil || cfg.MaxErrorRate != nil) && summary.TotalRequests == 0 {
		return []string{"no requests completed; thresholds cannot be evaluated"}
	}
	if cfg.MaxP95 != nil && summary.P95 > *cfg.MaxP95 {
		failures = append(failures, fmt.Sprintf("P95 %s exceeds maximum %s", summary.P95, *cfg.MaxP95))
	}
	if cfg.MaxErrorRate != nil {
		var errorRate float64
		denominator := summary.ScheduledRequests
		if denominator == 0 {
			denominator = summary.TotalRequests
		}
		if denominator > 0 {
			errorRate = float64(summary.FailedRequests+summary.DroppedRequests) / float64(denominator)
		}
		if errorRate > *cfg.MaxErrorRate {
			failures = append(failures, fmt.Sprintf("error rate %.2f%% exceeds maximum %.2f%%", errorRate*100, *cfg.MaxErrorRate*100))
		}
	}
	return failures
}
