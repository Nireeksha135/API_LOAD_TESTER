// Command api-load-tester is a concurrent CLI tool for load testing
// HTTP APIs. It fires a configurable number of concurrent requests
// (or runs for a fixed duration) against a target URL, tracks latency
// and status-code metrics in a thread-safe collector, renders a live
// terminal dashboard while the run is in progress, and prints a
// clean final report with optional CSV/JSON export.
//
// This file is intentionally thin: it owns only process-level
// concerns (flag parsing entrypoint, signal handling for graceful
// shutdown, and exit codes) and delegates all real work to the
// internal packages. Business logic must never live in main().
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/cli"
)

// version is the current release version of the CLI. It is a plain
// variable (not a constant) so it can be overridden at build time,
// e.g.:
//
//	go build -ldflags "-X main.version=1.2.3" ./cmd
var version = "dev"

func main() {
	os.Exit(mainWithExitCode())
}

// mainWithExitCode contains the actual entrypoint logic and returns
// an exit code instead of calling os.Exit directly. Keeping main()
// trivial and separating out this function makes the top-level
// control flow (including signal handling) straightforward to reason
// about and, if needed, exercise in tests.
func mainWithExitCode() int {
	opts, err := cli.ParseArgs(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// Usage text was already printed to stderr by the flag
			// package / our custom fs.Usage function.
			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if opts.ShowVersion {
		fmt.Fprintf(os.Stdout, "api-load-tester %s\n", version)
		return 0
	}

	// ctx is cancelled on the first SIGINT or SIGTERM, which
	// propagates down into the engine's worker pool via context
	// cancellation, allowing every in-flight request to finish (or be
	// abandoned at its own timeout) and the collector to produce a
	// partial summary from whatever was completed so far, rather than
	// the process dying mid-request with no report at all.
	//
	// A second SIGINT/SIGTERM (signal.NotifyContext's default stop
	// behavior only intercepts the first one; the Go runtime's normal
	// signal handling applies to subsequent signals) forces an
	// immediate exit if graceful shutdown is taking too long.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Run(ctx, opts.Config, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
