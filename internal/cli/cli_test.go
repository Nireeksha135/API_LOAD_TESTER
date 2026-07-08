package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
)

func TestParseArgsBasicGet(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{"-url", "https://api.example.com", "-c", "20", "-n", "500"}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.TargetURL != "https://api.example.com" {
		t.Errorf("TargetURL = %q, want %q", opts.Config.TargetURL, "https://api.example.com")
	}
	if opts.Config.Concurrency != 20 {
		t.Errorf("Concurrency = %d, want 20", opts.Config.Concurrency)
	}
	if opts.Config.TotalRequests != 500 {
		t.Errorf("TotalRequests = %d, want 500", opts.Config.TotalRequests)
	}
	if opts.Config.Method != config.MethodGET {
		t.Errorf("Method = %q, want %q", opts.Config.Method, config.MethodGET)
	}
}

func TestParseArgsMissingURL(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseArgs([]string{"-c", "10"}, &errOut)
	if err == nil {
		t.Error("ParseArgs() with no -url: expected error, got nil")
	}
}

func TestParseArgsDurationMode(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{"-url", "https://api.example.com", "-d", "15s", "-c", "10"}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !opts.Config.UseDuration {
		t.Error("UseDuration = false, want true when -d is set")
	}
	if opts.Config.Duration != 15*time.Second {
		t.Errorf("Duration = %v, want 15s", opts.Config.Duration)
	}
}

func TestParseArgsHeadersAndBody(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://api.example.com/users",
		"-method", "post",
		"-body", `{"name":"test"}`,
		"-H", "Authorization: Bearer abc123",
		"-H", "X-Trace-Id: xyz",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.Method != config.MethodPOST {
		t.Errorf("Method = %q, want POST (should be uppercased)", opts.Config.Method)
	}
	if opts.Config.Body != `{"name":"test"}` {
		t.Errorf("Body = %q, want %q", opts.Config.Body, `{"name":"test"}`)
	}
	if opts.Config.Headers["Authorization"] != "Bearer abc123" {
		t.Errorf("Headers[Authorization] = %q, want %q", opts.Config.Headers["Authorization"], "Bearer abc123")
	}
	if opts.Config.Headers["X-Trace-Id"] != "xyz" {
		t.Errorf("Headers[X-Trace-Id] = %q, want %q", opts.Config.Headers["X-Trace-Id"], "xyz")
	}
	if opts.Config.ContentType != "application/json" {
		t.Errorf("ContentType = %q, want application/json to be auto-set", opts.Config.ContentType)
	}
}

func TestParseArgsExplicitContentTypeHeaderWins(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-method", "POST",
		"-body", "<xml/>",
		"-H", "Content-Type: application/xml",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.Headers["Content-Type"] != "application/xml" {
		t.Errorf("Headers[Content-Type] = %q, want application/xml", opts.Config.Headers["Content-Type"])
	}
	// ContentType field itself should remain unset since the explicit
	// header already covers it; newRequestTemplate only falls back to
	// cfg.ContentType when no Content-Type header exists.
	if opts.Config.ContentType != "" {
		t.Errorf("ContentType = %q, want empty when Content-Type header explicitly set", opts.Config.ContentType)
	}
}

func TestParseArgsBodyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte(`{"from":"file"}`), 0o644); err != nil {
		t.Fatalf("failed to write test body file: %v", err)
	}

	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-method", "POST",
		"-body", `{"ignored":"inline"}`,
		"-body-file", path,
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.Body != `{"from":"file"}` {
		t.Errorf("Body = %q, want contents of body file (should override -body)", opts.Config.Body)
	}
}

func TestParseArgsBodyFileMissing(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-body-file", "/nonexistent/path/body.json",
	}, &errOut)
	if err == nil {
		t.Error("ParseArgs() with missing body file: expected error, got nil")
	}
}

func TestParseArgsInvalidHeader(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-H", "NoColonHere",
	}, &errOut)
	if err == nil {
		t.Error("ParseArgs() with malformed header: expected error, got nil")
	}
}

func TestParseArgsVersion(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{"-version"}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if !opts.ShowVersion {
		t.Error("ShowVersion = false, want true")
	}
}

func TestParseArgsInvalidConcurrencyPropagatesValidationError(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseArgs([]string{"-url", "https://api.example.com", "-c", "0"}, &errOut)
	if err == nil {
		t.Error("ParseArgs() with -c 0: expected validation error, got nil")
	}
}

func TestRunEndToEndCountMode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.NewDefaultConfig()
	cfg.TargetURL = server.URL
	cfg.Concurrency = 5
	cfg.TotalRequests = 25
	cfg.Timeout = 5 * time.Second

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer stdoutR.Close()

	done := make(chan struct{})
	var output bytes.Buffer
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stdoutR.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
		close(done)
	}()

	runErr := Run(context.Background(), cfg, stdoutW, os.Stderr)
	stdoutW.Close()
	<-done

	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}
	if !strings.Contains(output.String(), "LOAD TEST REPORT") {
		t.Errorf("Run() stdout missing final report, got:\n%s", output.String())
	}
	if !strings.Contains(output.String(), "Total Requests:    25") {
		t.Errorf("Run() stdout missing expected request count, got:\n%s", output.String())
	}
}

func TestRunEndToEndWithExports(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dir := t.TempDir()
	csvPath := filepath.Join(dir, "out.csv")
	jsonPath := filepath.Join(dir, "out.json")

	cfg := config.NewDefaultConfig()
	cfg.TargetURL = server.URL
	cfg.Concurrency = 3
	cfg.TotalRequests = 10
	cfg.Timeout = 5 * time.Second
	cfg.OutputCSV = csvPath
	cfg.OutputJSON = jsonPath

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	defer stdoutR.Close()

	done := make(chan struct{})
	go func() {
		buf := make([]byte, 4096)
		for {
			_, err := stdoutR.Read(buf)
			if err != nil {
				break
			}
		}
		close(done)
	}()

	runErr := Run(context.Background(), cfg, stdoutW, os.Stderr)
	stdoutW.Close()
	<-done

	if runErr != nil {
		t.Fatalf("Run() error = %v", runErr)
	}

	if _, err := os.Stat(csvPath); err != nil {
		t.Errorf("expected CSV file at %s, got error: %v", csvPath, err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected JSON file at %s, got error: %v", jsonPath, err)
	}
}
