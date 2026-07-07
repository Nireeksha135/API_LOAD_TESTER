// Package reporter renders load test progress and results to the
// terminal: a live, in-place-updating dashboard while a run is in
// progress, and a clean static summary report once it completes.
package reporter

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/utils"
)

// defaultRefreshInterval controls how often the live dashboard
// redraws itself. 200ms is frequent enough to feel responsive without
// adding measurable overhead to the load test itself, since it only
// reads cheap O(1) running counters via Collector.Snapshot.
const defaultRefreshInterval = 200 * time.Millisecond

// progressBarWidth is the character width of the dashboard's progress bar.
const progressBarWidth = 30

// Dashboard renders a live-updating view of an in-progress load test
// to a terminal. When the output destination is not an interactive
// TTY (e.g. redirected to a file or piped to another process), it
// automatically falls back to printing simple, non-ANSI status lines
// so log output stays clean and greppable.
type Dashboard struct {
	cfg       *config.Config
	collector *metrics.Collector
	out       *os.File
	interval  time.Duration

	interactive   bool
	lastLineCount int
}

// NewDashboard creates a Dashboard that reads live statistics from
// collector and renders them to out (typically os.Stdout). If out is
// nil, os.Stdout is used.
func NewDashboard(cfg *config.Config, collector *metrics.Collector, out *os.File) *Dashboard {
	if out == nil {
		out = os.Stdout
	}
	return &Dashboard{
		cfg:         cfg,
		collector:   collector,
		out:         out,
		interval:    defaultRefreshInterval,
		interactive: isTerminal(out),
	}
}

// isTerminal reports whether f appears to be an interactive terminal
// (as opposed to a redirected file or pipe), using only the standard
// library so no external dependency is required.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// Run blocks, redrawing the dashboard on a fixed interval, until ctx
// is cancelled. The intended usage is to run the engine's Run() call
// in a background goroutine and run the Dashboard in the foreground,
// with a context that the caller cancels once the engine finishes:
//
//	dashCtx, cancel := context.WithCancel(context.Background())
//	go func() {
//	    summary, _ = engine.Run(runCtx)
//	    cancel()
//	}()
//	dashboard.Run(dashCtx)
func (d *Dashboard) Run(ctx context.Context) {
	if d.collector == nil {
		return
	}

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	d.draw()
	for {
		select {
		case <-ctx.Done():
			// Final redraw so the dashboard reflects the true end
			// state (e.g. 1000/1000 requests) rather than whatever
			// was on screen at the last tick.
			d.draw()
			if d.interactive {
				fmt.Fprintln(d.out)
			}
			return
		case <-ticker.C:
			d.draw()
		}
	}
}

func (d *Dashboard) draw() {
	snap := d.collector.Snapshot()
	lines := d.buildLines(snap)

	if d.interactive {
		d.redrawInPlace(lines)
	} else {
		d.appendLine(snap)
	}
}

// buildLines renders the current Snapshot into a fixed set of
// display lines for the interactive dashboard.
func (d *Dashboard) buildLines(snap metrics.Snapshot) []string {
	lines := make([]string, 0, 6)

	lines = append(lines, fmt.Sprintf("Load Test  %s %s", d.cfg.Method, d.cfg.TargetURL))
	lines = append(lines, d.progressLine(snap))

	lines = append(lines, fmt.Sprintf(
		"Elapsed: %-10s  Concurrency: %-5d  RPS: %s",
		utils.FormatDuration(snap.Elapsed), d.cfg.Concurrency, utils.FormatRate(snap.RequestsPerSecond),
	))

	lines = append(lines, fmt.Sprintf(
		"Success: %-8d  Failed: %-8d  Bytes: %s",
		snap.SuccessRequests, snap.FailedRequests, utils.FormatBytes(snap.TotalBytesRead),
	))

	lines = append(lines, fmt.Sprintf(
		"Latency  min: %-10s max: %-10s mean: %-10s",
		utils.FormatDuration(snap.MinLatency), utils.FormatDuration(snap.MaxLatency), utils.FormatDuration(snap.MeanLatency),
	))

	lines = append(lines, "Status Codes: "+formatStatusCodeCounts(snap.StatusCodeCounts))

	return lines
}

// progressLine renders the progress bar appropriate to the run mode:
// requests-completed-of-total in count mode, elapsed-of-duration in
// duration mode.
func (d *Dashboard) progressLine(snap metrics.Snapshot) string {
	if d.cfg.UseDuration {
		var fraction float64
		if d.cfg.Duration > 0 {
			fraction = snap.Elapsed.Seconds() / d.cfg.Duration.Seconds()
		}
		return fmt.Sprintf(
			"%s %s / %s",
			utils.ProgressBar(fraction, progressBarWidth),
			utils.FormatDuration(snap.Elapsed),
			utils.FormatDuration(d.cfg.Duration),
		)
	}

	total := int64(d.cfg.TotalRequests)
	var fraction float64
	if total > 0 {
		fraction = float64(snap.TotalRequests) / float64(total)
	}
	return fmt.Sprintf(
		"%s %d / %d requests",
		utils.ProgressBar(fraction, progressBarWidth),
		snap.TotalRequests, total,
	)
}

// formatStatusCodeCounts renders a compact, sorted "code:count" list.
// A status code of 0 (transport-level failure, no HTTP response) is
// displayed as "ERR" for readability.
func formatStatusCodeCounts(counts map[int]int64) string {
	if len(counts) == 0 {
		return "(none yet)"
	}

	codes := make([]int, 0, len(counts))
	for code := range counts {
		codes = append(codes, code)
	}
	sort.Ints(codes)

	parts := make([]string, 0, len(codes))
	for _, code := range codes {
		label := fmt.Sprintf("%d", code)
		if code == 0 {
			label = "ERR"
		}
		parts = append(parts, fmt.Sprintf("%s:%d", label, counts[code]))
	}
	return strings.Join(parts, "  ")
}

// redrawInPlace clears and rewrites the previous dashboard block
// using ANSI cursor-movement escape codes ("\x1b[<n>A" to move the
// cursor up n lines, "\x1b[2K" to clear the current line), producing
// a flicker-light, in-place live update in interactive terminals.
func (d *Dashboard) redrawInPlace(lines []string) {
	if d.lastLineCount > 0 {
		fmt.Fprintf(d.out, "\x1b[%dA", d.lastLineCount)
	}
	for _, line := range lines {
		fmt.Fprint(d.out, "\x1b[2K")
		fmt.Fprintln(d.out, line)
	}
	d.lastLineCount = len(lines)
}

// appendLine prints a single compact status line with no ANSI escape
// codes, used when output is not an interactive terminal so that
// redirected/piped output remains clean, append-only, and greppable.
func (d *Dashboard) appendLine(snap metrics.Snapshot) {
	fmt.Fprintf(
		d.out,
		"[%s] sent=%d success=%d failed=%d rps=%.2f\n",
		utils.FormatDuration(snap.Elapsed), snap.TotalRequests, snap.SuccessRequests, snap.FailedRequests, snap.RequestsPerSecond,
	)
}
