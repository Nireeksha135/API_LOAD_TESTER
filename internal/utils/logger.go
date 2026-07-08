package utils

import (
	"io"
	"log"
	"os"
)

// NewLogger returns a standard library *log.Logger configured for
// the CLI's verbose/debug output. When verbose is false, the logger
// writes to io.Discard so call sites can log unconditionally without
// scattering "if verbose" checks through business logic.
//
// The logger writes to stderr (not stdout) so that verbose diagnostic
// output never interleaves with or corrupts stdout, which is reserved
// for the live dashboard and final report.
func NewLogger(verbose bool) *log.Logger {
	if !verbose {
		return log.New(io.Discard, "", 0)
	}
	return log.New(os.Stderr, "[api-load-tester] ", log.LstdFlags|log.Lmicroseconds)
}
