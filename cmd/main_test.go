package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestCompareCommandExitCodes(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()
	os.Args = []string{"api-load-tester", "compare", "-h"}
	if got := mainWithExitCode(); got != 0 {
		t.Errorf("compare help exit = %d, want 0", got)
	}
	os.Args = []string{"api-load-tester", "compare", "-bad"}
	if got := mainWithExitCode(); got != 2 {
		t.Errorf("compare usage error exit = %d, want 2", got)
	}
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	current := filepath.Join(dir, "current.json")
	if err := os.WriteFile(base, []byte(`{"p95_latency_ms":100,"p99_latency_ms":100,"requests_per_second":100,"total_requests":10,"failed_requests":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(current, []byte(`{"p95_latency_ms":200,"p99_latency_ms":100,"requests_per_second":100,"total_requests":10,"failed_requests":0}`), 0o644); err != nil {
		t.Fatal(err)
	}
	os.Args = []string{"api-load-tester", "compare", "-baseline", base, "-current", current, "-max-regression", "10%"}
	if got := mainWithExitCode(); got != 1 {
		t.Errorf("compare regression exit = %d, want 1", got)
	}
}
