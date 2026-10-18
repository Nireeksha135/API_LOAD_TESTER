package cli

import (
	"flag"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/exporter"
)

type RegressionError struct{}

func (RegressionError) Error() string { return "performance regression limit exceeded" }

// Compare runs the CLI comparison subcommand and returns an error when a regression exceeds the limit.
func Compare(args []string, out, errOut io.Writer) error {
	fs := flag.NewFlagSet("compare", flag.ContinueOnError)
	fs.SetOutput(errOut)
	baseline := fs.String("baseline", "", "Previous JSON report")
	current := fs.String("current", "", "Current JSON report")
	limit := fs.String("max-regression", "", "Fail when any worsening metric exceeds this percentage (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baseline == "" || *current == "" {
		return fmt.Errorf("compare requires -baseline and -current JSON report paths")
	}
	threshold := -1.0
	if strings.TrimSpace(*limit) != "" {
		raw := strings.TrimSuffix(strings.TrimSpace(*limit), "%")
		parsed, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
			return fmt.Errorf("invalid -max-regression %q: expected a finite non-negative percentage", *limit)
		}
		threshold = parsed
	}
	results, warnings, err := exporter.CompareJSONDetailed(*baseline, *current, threshold)
	if err != nil {
		return err
	}
	if threshold >= 0 {
		fmt.Fprintf(out, "Performance comparison (limit %.2f%%)\n", threshold)
	} else {
		fmt.Fprintln(out, "Performance comparison (no failure limit configured)")
	}
	for _, warning := range warnings {
		fmt.Fprintf(out, "WARNING: %s\n", warning)
	}
	failed := false
	for _, r := range results {
		change := fmt.Sprintf("%+0.2f%s", r.ChangeValue, r.ChangeUnit)
		if math.IsInf(r.ChangeValue, 1) {
			change = "new from 0"
		}
		fmt.Fprintf(out, "%-28s %s", r.Name, change)
		if r.ExceedsLimit {
			fmt.Fprintf(out, "  REGRESSION (limit exceeded)")
			failed = true
		} else if r.Worse {
			fmt.Fprint(out, "  worse")
		}
		fmt.Fprintln(out)
	}
	if failed {
		return RegressionError{}
	}
	if threshold >= 0 {
		fmt.Fprintln(out, "No performance regression exceeded the configured limit")
	}
	return nil
}
