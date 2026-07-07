package reporter

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/metrics"
)

func TestFormatStatusCodeCounts(t *testing.T) {
	counts := map[int]int64{200: 10, 404: 2, 0: 1, 500: 3}
	got := formatStatusCodeCounts(counts)

	// Codes must appear sorted ascending, with 0 rendered as ERR.
	wantOrder := []string{"ERR:1", "200:10", "404:2", "500:3"}
	lastIdx := -1
	for _, want := range wantOrder {
		idx := strings.Index(got, want)
		if idx == -1 {
			t.Fatalf("formatStatusCodeCounts() = %q, missing %q", got, want)
		}
		if idx < lastIdx {
			t.Errorf("formatStatusCodeCounts() = %q, expected %q before position %d", got, want, lastIdx)
		}
		lastIdx = idx
	}
}

func TestFormatStatusCodeCountsEmpty(t *testing.T) {
	got := formatStatusCodeCounts(map[int]int64{})
	if got != "(none yet)" {
		t.Errorf("formatStatusCodeCounts(empty) = %q, want %q", got, "(none yet)")
	}
}

func TestDashboardBuildLinesCountMode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.TargetURL = "http://example.com"
	cfg.Method = config.MethodGET
	cfg.Concurrency = 10
	cfg.TotalRequests = 100

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	d := NewDashboard(cfg, collector, nil)

	snap := metrics.Snapshot{TotalRequests: 50, SuccessRequests: 48, FailedRequests: 2}
	lines := d.buildLines(snap)

	if len(lines) == 0 {
		t.Fatal("buildLines() returned no lines")
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "50 / 100 requests") {
			found = true
		}
	}
	if !found {
		t.Errorf("buildLines() progress line missing '50 / 100 requests', got: %v", lines)
	}
}

func TestDashboardBuildLinesDurationMode(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.TargetURL = "http://example.com"
	cfg.Method = config.MethodGET
	cfg.UseDuration = true
	cfg.Duration = 10 * time.Second

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	d := NewDashboard(cfg, collector, nil)

	snap := metrics.Snapshot{Elapsed: 5 * time.Second, TotalRequests: 20}
	lines := d.buildLines(snap)

	found := false
	for _, l := range lines {
		if strings.Contains(l, "5.00s / 10.00s") {
			found = true
		}
	}
	if !found {
		t.Errorf("buildLines() duration progress line missing '5.00s / 10.00s', got: %v", lines)
	}
}

func TestDashboardRunStopsOnContextCancel(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.TargetURL = "http://example.com"
	cfg.TotalRequests = 10
	cfg.Concurrency = 2

	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)
	collector.Start()

	// Redirect to a pipe so isTerminal() is false and Run() takes the
	// non-interactive, plain-line code path deterministically in CI.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer r.Close()
	defer w.Close()

	d := NewDashboard(cfg, collector, w)
	if d.interactive {
		t.Fatal("expected dashboard writing to a pipe to be non-interactive")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		d.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// success: Run() returned after context cancellation
	case <-time.After(2 * time.Second):
		t.Fatal("Dashboard.Run() did not return after context cancellation")
	}
}

func TestNewDashboardDefaultsToStdoutWhenNil(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.TargetURL = "http://example.com"
	collector := metrics.NewCollector(cfg.TargetURL, cfg.Method, cfg.Concurrency)

	d := NewDashboard(cfg, collector, nil)
	if d.out != os.Stdout {
		t.Error("NewDashboard(..., nil) should default out to os.Stdout")
	}
}
