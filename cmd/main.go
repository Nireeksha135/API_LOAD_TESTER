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

var version = "dev"

func main() {
	os.Exit(mainWithExitCode())
}

func mainWithExitCode() int {
	if len(os.Args) > 1 && os.Args[1] == "compare" {
		if err := cli.Compare(os.Args[2:], os.Stdout, os.Stderr); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return 0
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			var regression cli.RegressionError
			if errors.As(err, &regression) {
				return 1
			}
			return 2
		}
		return 0
	}
	opts, err := cli.ParseArgs(os.Args[1:], os.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {

			return 0
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	if opts.ShowVersion {
		fmt.Fprintf(os.Stdout, "api-load-tester %s\n", version)
		return 0
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Run(ctx, opts.Config, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	return 0
}
