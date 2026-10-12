package main

import (
	"os"
	"testing"
)

func TestMainWithExitCodeVersion(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"api-load-tester", "-version"}
	t.Cleanup(func() { os.Args = originalArgs })

	if got := mainWithExitCode(); got != 0 {
		t.Errorf("mainWithExitCode() = %d, want 0", got)
	}
}

func TestMainWithExitCodeHelp(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"api-load-tester", "-help"}
	t.Cleanup(func() { os.Args = originalArgs })

	if got := mainWithExitCode(); got != 0 {
		t.Errorf("mainWithExitCode() = %d, want 0", got)
	}
}

func TestMainWithExitCodeInvalidArguments(t *testing.T) {
	originalArgs := os.Args
	os.Args = []string{"api-load-tester", "-not-a-flag"}
	t.Cleanup(func() { os.Args = originalArgs })

	if got := mainWithExitCode(); got != 1 {
		t.Errorf("mainWithExitCode() = %d, want 1", got)
	}
}
