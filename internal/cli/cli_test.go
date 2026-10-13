package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/models"
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

func TestParseArgsMultipleURLs(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://one.example.com",
		"-url", "https://two.example.com",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if len(opts.Config.TargetURLs) != 2 || opts.Config.TargetURL != "https://one.example.com" {
		t.Errorf("TargetURL=%q TargetURLs=%v, want first target and both URLs", opts.Config.TargetURL, opts.Config.TargetURLs)
	}
}

func TestParseArgsYAMLDefaultsAndCLIOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "load-test.yaml")
	data := []byte("url: https://config.example.com\nconcurrency: 4\nrequests: 25\nmax-error-rate: 2%\nheaders:\n  X-Config: from-file\n")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-config", path,
		"-url", "https://one.example.com",
		"-url", "https://two.example.com",
		"-c", "2",
		"-max-error-rate", "1%",
		"-H", "X-CLI: from-command-line",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.Concurrency != 2 || opts.Config.TotalRequests != 25 {
		t.Errorf("concurrency=%d requests=%d, want CLI concurrency 2 and YAML requests 25", opts.Config.Concurrency, opts.Config.TotalRequests)
	}
	if len(opts.Config.TargetURLs) != 2 || opts.Config.TargetURLs[0] != "https://one.example.com" {
		t.Errorf("TargetURLs = %v, want command-line targets", opts.Config.TargetURLs)
	}
	if opts.Config.MaxErrorRate == nil || *opts.Config.MaxErrorRate != 0.01 {
		t.Errorf("MaxErrorRate = %v, want command-line override 0.01", opts.Config.MaxErrorRate)
	}
	if opts.Config.Headers["X-Config"] != "from-file" || opts.Config.Headers["X-CLI"] != "from-command-line" {
		t.Errorf("Headers = %v, want YAML and CLI headers", opts.Config.Headers)
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

func TestParseArgsRPS(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{"-url", "https://api.example.com", "-rps", "25.5"}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.RPS != 25.5 {
		t.Errorf("RPS = %v, want 25.5", opts.Config.RPS)
	}
}

func TestParseArgsTimeSeriesCSV(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-time-series-csv", "series.csv",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.OutputTimeSeriesCSV != "series.csv" {
		t.Errorf("OutputTimeSeriesCSV = %q, want series.csv", opts.Config.OutputTimeSeriesCSV)
	}
}

func TestParseArgsWarmupAndRampUp(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-warmup", "2s",
		"-ramp-up", "5s",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.Warmup != 2*time.Second || opts.Config.RampUp != 5*time.Second {
		t.Errorf("Warmup=%s RampUp=%s, want 2s/5s", opts.Config.Warmup, opts.Config.RampUp)
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

func TestParseArgsClampsConcurrencyAndAllowsGetBody(t *testing.T) {
	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-c", "10",
		"-n", "2",
		"-body", "payload",
	}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}
	if opts.Config.Concurrency != 2 {
		t.Errorf("Concurrency = %d, want clamped value 2", opts.Config.Concurrency)
	}
}

func TestParseArgsRejectsHeadBody(t *testing.T) {
	var errOut bytes.Buffer
	_, err := ParseArgs([]string{
		"-url", "https://api.example.com",
		"-method", "HEAD",
		"-body", "payload",
	}, &errOut)
	if err == nil {
		t.Error("ParseArgs() with a HEAD body: expected validation error, got nil")
	}
}

func TestParseArgsThresholds(t *testing.T) {
	for _, rawRate := range []string{"1%", "0.01"} {
		t.Run(rawRate, func(t *testing.T) {
			var errOut bytes.Buffer
			opts, err := ParseArgs([]string{
				"-url", "https://api.example.com",
				"-max-p95", "200ms",
				"-max-error-rate", rawRate,
			}, &errOut)
			if err != nil {
				t.Fatalf("ParseArgs() error = %v", err)
			}
			if opts.Config.MaxP95 == nil || *opts.Config.MaxP95 != 200*time.Millisecond {
				t.Errorf("MaxP95 = %v, want 200ms", opts.Config.MaxP95)
			}
			if opts.Config.MaxErrorRate == nil || *opts.Config.MaxErrorRate != 0.01 {
				t.Errorf("MaxErrorRate = %v, want 0.01", opts.Config.MaxErrorRate)
			}
		})
	}
}

func TestParseArgsRejectsInvalidThresholds(t *testing.T) {
	for _, args := range [][]string{
		{"-max-p95", "not-a-duration"},
		{"-max-error-rate", "101%"},
		{"-max-error-rate", "not-a-rate"},
	} {
		t.Run(strings.Join(args, "="), func(t *testing.T) {
			var errOut bytes.Buffer
			fullArgs := append([]string{"-url", "https://api.example.com"}, args...)
			if _, err := ParseArgs(fullArgs, &errOut); err == nil {
				t.Error("ParseArgs() error = nil, want invalid-threshold error")
			}
		})
	}
}

func TestCheckThresholdsUsesStrictLimits(t *testing.T) {
	maxP95 := 200 * time.Millisecond
	maxErrorRate := 0.01
	cfg := &config.Config{MaxP95: &maxP95, MaxErrorRate: &maxErrorRate}
	summary := models.Summary{TotalRequests: 100, FailedRequests: 1, P95: maxP95}
	if failures := checkThresholds(summary, cfg); len(failures) != 0 {
		t.Errorf("checkThresholds() = %v, want no failures at the limit", failures)
	}

	summary.P95++
	summary.FailedRequests++
	if failures := checkThresholds(summary, cfg); len(failures) != 2 {
		t.Errorf("checkThresholds() = %v, want both thresholds to fail", failures)
	}
}

func TestCheckThresholdsFailWhenNoRequestsWereMeasured(t *testing.T) {
	maxP95 := time.Second
	cfg := &config.Config{MaxP95: &maxP95}
	summary := models.Summary{}

	failures := checkThresholds(summary, cfg)
	if len(failures) != 1 || !strings.Contains(failures[0], "no requests completed") {
		t.Errorf("checkThresholds() = %v, want an explicit no-measurements failure", failures)
	}

	if failures := checkThresholds(summary, &config.Config{}); len(failures) != 0 {
		t.Errorf("checkThresholds() without thresholds = %v, want no failure", failures)
	}
}

func TestParseArgsAndRunEndToEndCountMode(t *testing.T) {
	var requests int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requests, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var errOut bytes.Buffer
	opts, err := ParseArgs([]string{"-url", server.URL, "-c", "5", "-n", "25"}, &errOut)
	if err != nil {
		t.Fatalf("ParseArgs() error = %v", err)
	}

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

	runErr := Run(context.Background(), opts.Config, stdoutW, os.Stderr)
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
	if got := atomic.LoadInt64(&requests); got != 25 {
		t.Errorf("server received %d requests, want 25", got)
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
	timeSeriesCSVPath := filepath.Join(dir, "series.csv")

	cfg := config.NewDefaultConfig()
	cfg.TargetURL = server.URL
	cfg.Concurrency = 3
	cfg.TotalRequests = 10
	cfg.Timeout = 5 * time.Second
	cfg.OutputCSV = csvPath
	cfg.OutputJSON = jsonPath
	cfg.OutputTimeSeriesCSV = timeSeriesCSVPath

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
	if _, err := os.Stat(timeSeriesCSVPath); err != nil {
		t.Errorf("expected time-series CSV at %s, got error: %v", timeSeriesCSVPath, err)
	}
}
