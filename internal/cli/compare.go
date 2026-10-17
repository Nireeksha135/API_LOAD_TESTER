package cli

import (
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/exporter"
)

// Compare runs the CLI comparison subcommand and returns an error when a regression exceeds the limit.
func Compare(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(errOut)
	baseline := fs.String("baseline", "", "Previous JSON report")
	current := fs.String("current", "", "Current JSON report")
	limit := fs.String("max-regression", "10%", "Fail when any worsening metric exceeds this percentage")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baseline == "" || *current == "" {
		return fmt.Errorf("compare requires -baseline and -current JSON report paths")
	}
	raw := strings.TrimSuffix(strings.TrimSpace(*limit), "%")
	threshold, err := strconv.ParseFloat(raw, 64)
	if err != nil || threshold < 0 {
		return fmt.Errorf("invalid -max-regression %q: expected a non-negative percentage", *limit)
	}
	results, err := exporter.CompareJSON(*baseline, *current, threshold)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Performance comparison (limit %.2f%%)\n", threshold)
	failed := false
	for _, r := range results {
		fmt.Fprintf(out, "%-28s %+0.2f%%", r.Name, r.ChangePercent)
		if r.Worse {
			fmt.Fprintf(out, "  REGRESSION (limit exceeded)")
			failed = true
		}
		fmt.Fprintln(out)
	}
	if failed {
		return fmt.Errorf("performance regression limit exceeded")
	}
	fmt.Fprintln(out, "No performance regression exceeded the configured limit")
	return nil
}
