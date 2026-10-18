package reporter

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/utils"
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
	if summary.TargetRPS > 0 {
		fmt.Fprintf(out, "Target RPS:        %.2f\n", summary.TargetRPS)
		fmt.Fprintf(out, "Scheduled:         %d  Dropped: %d\n", summary.ScheduledRequests, summary.DroppedRequests)
		if summary.DroppedRequests > 0 {
			fmt.Fprintln(out, "WARNING: arrivals were dropped; latency percentiles exclude requests that never started.")
		}
	}
	fmt.Fprintf(out, "Successful:        %d\n", summary.SuccessRequests)
	fmt.Fprintf(out, "Failed:            %d\n", summary.FailedRequests)
	fmt.Fprintf(out, "Cancelled:         %d\n", summary.CancelledRequests)
	fmt.Fprintf(out, "Requests/sec:      %.2f\n", summary.RequestsPerSecond)
	fmt.Fprintf(out, "Total Bytes Read:  %s\n", utils.FormatBytes(summary.TotalBytesRead))
	fmt.Fprintf(out, "Throughput:        %s/s\n", utils.FormatBytes(int64(summary.ThroughputBytesPerSecond)))
	if summary.TargetRPS > 0 {
		var scheduledRPS, targetRatio float64
		if summary.ScheduleDuration > 0 {
			scheduledRPS = float64(summary.ScheduledRequests) / summary.ScheduleDuration.Seconds()
		}
		if summary.ExpectedScheduledRequests > 0 {
			targetRatio = float64(summary.ScheduledRequests) / float64(summary.ExpectedScheduledRequests) * 100
			if targetRatio > 100 {
				targetRatio = 100
			}
		}
		fmt.Fprintf(out, "Scheduled RPS:     %.2f (%.1f%% of expected arrivals)\n", scheduledRPS, targetRatio)
	}
	fmt.Fprintln(out, subDivider)

	fmt.Fprintln(out, "Latency:")
	fmt.Fprintf(out, "  Min:             %s\n", utils.FormatDuration(summary.MinLatency))
	fmt.Fprintf(out, "  Mean:            %s\n", utils.FormatDuration(summary.MeanLatency))
	fmt.Fprintf(out, "  P50:             %s\n", utils.FormatDuration(summary.P50))
	fmt.Fprintf(out, "  P90:             %s\n", utils.FormatDuration(summary.P90))
	fmt.Fprintf(out, "  P95:             %s\n", utils.FormatDuration(summary.P95))
	fmt.Fprintf(out, "  P99:             %s\n", utils.FormatDuration(summary.P99))
	fmt.Fprintf(out, "  P99.9:           %s\n", utils.FormatDuration(summary.P999))
	fmt.Fprintf(out, "  Max:             %s\n", utils.FormatDuration(summary.MaxLatency))
	fmt.Fprintln(out, subDivider)
	if len(summary.Endpoints) > 0 {
		fmt.Fprintln(out, "Endpoint Performance (slowest P95 first):")
		for _, endpoint := range summary.Endpoints {
			fmt.Fprintf(out, "  %s\n    Requests: %d  Errors: %d (%.2f%%)  Cancelled: %d  Mean: %s  P95: %s  P99: %s\n", endpoint.Endpoint, endpoint.Requests, endpoint.Errors, endpoint.ErrorRate*100, endpoint.CancelledRequests, utils.FormatDuration(endpoint.MeanLatency), utils.FormatDuration(endpoint.P95), utils.FormatDuration(endpoint.P99))
		}
		fmt.Fprintln(out, subDivider)
	}

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
		errorMessages := make([]string, 0, len(summary.Errors))
		for message := range summary.Errors {
			errorMessages = append(errorMessages, message)
		}
		sort.Strings(errorMessages)
		for _, message := range errorMessages {
			fmt.Fprintf(out, "  - %s: %d\n", message, summary.Errors[message])
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
