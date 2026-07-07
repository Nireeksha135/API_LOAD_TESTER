package reporter

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Nireeksha/API_LOAD_TESTER/internal/models"
	"github.com/Nireeksha/API_LOAD_TESTER/internal/utils"
)

// reportWidth is the fixed character width of the static final report.
const reportWidth = 60

// PrintSummary renders a clean, human-readable final report for a
// completed (or partially completed, e.g. after graceful shutdown)
// models.Summary to out. This is the "final static report" shown
// after the live dashboard finishes.
func PrintSummary(out io.Writer, summary models.Summary) {
	divider := strings.Repeat("=", reportWidth)
	subDivider := strings.Repeat("-", reportWidth)

	fmt.Fprintln(out, divider)
	fmt.Fprintln(out, centerText("LOAD TEST REPORT", reportWidth))
	fmt.Fprintln(out, divider)

	fmt.Fprintf(out, "Target:            %s\n", summary.TargetURL)
	fmt.Fprintf(out, "Method:            %s\n", summary.Method)
	fmt.Fprintf(out, "Concurrency:       %d\n", summary.Concurrency)
	fmt.Fprintf(out, "Start Time:        %s\n", summary.StartTime.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(out, "End Time:          %s\n", summary.EndTime.Format("2006-01-02 15:04:05 MST"))
	fmt.Fprintf(out, "Total Duration:    %s\n", utils.FormatDuration(summary.TotalDuration))
	fmt.Fprintln(out, subDivider)

	fmt.Fprintf(out, "Total Requests:    %d\n", summary.TotalRequests)
	fmt.Fprintf(out, "Successful:        %d\n", summary.SuccessRequests)
	fmt.Fprintf(out, "Failed:            %d\n", summary.FailedRequests)
	fmt.Fprintf(out, "Requests/sec:      %.2f\n", summary.RequestsPerSecond)
	fmt.Fprintf(out, "Total Bytes Read:  %s\n", utils.FormatBytes(summary.TotalBytesRead))
	fmt.Fprintf(out, "Throughput:        %s/s\n", utils.FormatBytes(int64(summary.ThroughputBytesPerSecond)))
	fmt.Fprintln(out, subDivider)

	fmt.Fprintln(out, "Latency:")
	fmt.Fprintf(out, "  Min:             %s\n", utils.FormatDuration(summary.MinLatency))
	fmt.Fprintf(out, "  Mean:            %s\n", utils.FormatDuration(summary.MeanLatency))
	fmt.Fprintf(out, "  P50:             %s\n", utils.FormatDuration(summary.P50))
	fmt.Fprintf(out, "  P95:             %s\n", utils.FormatDuration(summary.P95))
	fmt.Fprintf(out, "  P99:             %s\n", utils.FormatDuration(summary.P99))
	fmt.Fprintf(out, "  Max:             %s\n", utils.FormatDuration(summary.MaxLatency))
	fmt.Fprintln(out, subDivider)

	fmt.Fprintln(out, "Status Codes:")
	codes := make([]int, 0, len(summary.StatusCodeCounts))
	for code := range summary.StatusCodeCounts {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	for _, code := range codes {
		label := fmt.Sprintf("%d", code)
		if code == 0 {
			label = "ERR (no response)"
		}
		var pct float64
		if summary.TotalRequests > 0 {
			pct = float64(summary.StatusCodeCounts[code]) / float64(summary.TotalRequests) * 100
		}
		fmt.Fprintf(out, "  %-18s %-8d (%.1f%%)\n", label, summary.StatusCodeCounts[code], pct)
	}

	if len(summary.Errors) > 0 {
		fmt.Fprintln(out, subDivider)
		fmt.Fprintln(out, "Errors observed:")
		for _, e := range summary.Errors {
			fmt.Fprintf(out, "  - %s\n", e)
		}
	}

	fmt.Fprintln(out, divider)
}

// centerText pads text with leading spaces to roughly center it
// within width. If text is already as wide as (or wider than) width,
// it is returned unchanged.
func centerText(text string, width int) string {
	if len(text) >= width {
		return text
	}
	padding := (width - len(text)) / 2
	return strings.Repeat(" ", padding) + text
}
