package main

import (
	"net/http"
	"net/http/httptest"
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

func TestMainWithExitCodeThresholdFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	originalArgs := os.Args
	os.Args = []string{
		"api-load-tester",
		"-url", server.URL,
		"-n", "1",
		"-c", "1",
		"-max-error-rate", "0%",
	}
	t.Cleanup(func() { os.Args = originalArgs })

	if got := mainWithExitCode(); got != 1 {
		t.Errorf("mainWithExitCode() = %d, want 1 when a threshold fails", got)
	}
}
