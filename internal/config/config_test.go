package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validTestConfig() *Config {
	cfg := NewDefaultConfig()
	cfg.TargetURL = "https://example.com/api"
	return cfg
}

func TestValidateNormalizesMethodAndClampsConcurrency(t *testing.T) {
	cfg := validTestConfig()
	cfg.Method = " get "
	cfg.Concurrency = 10
	cfg.TotalRequests = 2

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Method != MethodGET {
		t.Errorf("Method = %q, want %q", cfg.Method, MethodGET)
	}
	if cfg.Concurrency != 2 {
		t.Errorf("Concurrency = %d, want 2", cfg.Concurrency)
	}
}

func TestValidateAllowsGetBodyAndDurationConcurrency(t *testing.T) {
	cfg := validTestConfig()
	cfg.Body = "payload"
	cfg.UseDuration = true
	cfg.Duration = time.Second
	cfg.Concurrency = 200

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.Concurrency != 200 {
		t.Errorf("duration-mode concurrency = %d, want 200", cfg.Concurrency)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"empty target", func(c *Config) { c.TargetURL = " " }},
		{"invalid URL", func(c *Config) { c.TargetURL = "://bad" }},
		{"unsupported scheme", func(c *Config) { c.TargetURL = "ftp://example.com" }},
		{"missing host", func(c *Config) { c.TargetURL = "http:///path" }},
		{"empty method", func(c *Config) { c.Method = " " }},
		{"unsupported method", func(c *Config) { c.Method = "OPTIONS" }},
		{"invalid concurrency", func(c *Config) { c.Concurrency = 0 }},
		{"negative RPS", func(c *Config) { c.RPS = -1 }},
		{"RPS above scheduler cap", func(c *Config) { c.RPS = 100_001 }},
		{"excessive RPS", func(c *Config) { c.RPS = 1e10 }},
		{"negative warmup", func(c *Config) { c.Warmup = -time.Second }},
		{"negative ramp-up", func(c *Config) { c.RampUp = -time.Second }},
		{"invalid request count", func(c *Config) { c.TotalRequests = 0 }},
		{"invalid duration", func(c *Config) { c.UseDuration = true }},
		{"invalid timeout", func(c *Config) { c.Timeout = 0 }},
		{"invalid idle connections", func(c *Config) { c.MaxIdleConnsPerHost = 0 }},
		{"invalid expected status", func(c *Config) { c.ExpectedStatusCodes = []int{600} }},
		{"HEAD body", func(c *Config) { c.Method = MethodHEAD; c.Body = "payload" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTestConfig()
			tt.mutate(cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want an error")
			}
		})
	}
	if err := (*Config)(nil).Validate(); err == nil {
		t.Fatal("Validate() on nil config error = nil, want an error")
	}
}

func TestParseExpectedStatusCodes(t *testing.T) {
	got, err := ParseExpectedStatusCodes("200, 201,200")
	if err != nil {
		t.Fatalf("ParseExpectedStatusCodes() error = %v", err)
	}
	if len(got) != 2 || got[0] != 200 || got[1] != 201 {
		t.Errorf("ParseExpectedStatusCodes() = %v, want [200 201]", got)
	}
	for _, raw := range []string{"", "200,", "99", "600", "abc"} {
		if _, err := ParseExpectedStatusCodes(raw); err == nil {
			t.Errorf("ParseExpectedStatusCodes(%q) error = nil, want invalid-code error", raw)
		}
	}
}

func TestSuccessStatusCodeRules(t *testing.T) {
	cfg := NewDefaultConfig()
	for _, code := range []int{200, 302, 399} {
		if !cfg.IsSuccessStatusCode(code) {
			t.Errorf("default status %d should be successful", code)
		}
	}
	if cfg.IsSuccessStatusCode(400) {
		t.Error("default status 400 should not be successful")
	}
	cfg.ExpectedStatusCodes = []int{200, 201}
	if !cfg.IsSuccessStatusCode(201) || cfg.IsSuccessStatusCode(302) {
		t.Error("configured status rule should accept only 200 and 201")
	}
}

func TestParseHeaders(t *testing.T) {
	got, err := ParseHeaders([]string{"X-Test: value", "X-Other: left:right"})
	if err != nil {
		t.Fatalf("ParseHeaders() error = %v", err)
	}
	if got["X-Test"] != "value" || got["X-Other"] != "left:right" {
		t.Errorf("ParseHeaders() = %v, want parsed header values", got)
	}
	for _, raw := range []string{"missing colon", ": value"} {
		t.Run(raw, func(t *testing.T) {
			if _, err := ParseHeaders([]string{raw}); err == nil || !strings.Contains(err.Error(), "invalid header") {
				t.Errorf("ParseHeaders(%q) error = %v, want invalid-header error", raw, err)
			}
		})
	}
}

func TestLoadYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "load-test.yaml")
	data := []byte(strings.Join([]string{
		"url: https://example.com",
		"urls:",
		"  - https://one.example.com",
		"  - https://two.example.com",
		"method: POST",
		"headers:",
		"  X-Test: from-file",
		"body: payload",
		"concurrency: 4",
		"requests: 30",
		"duration: 5s",
		"rps: 12.5",
		"warmup: 2s",
		"ramp-up: 3s",
		"timeout: 1s",
		"max-p95: 200ms",
		"max-error-rate: 1%",
		"expect-status: 200,201",
		"csv: requests.csv",
		"json: summary.json",
		"time-series-csv: series.csv",
		"verbose: true",
		"insecure: true",
		"http2: false",
		"disable-keep-alives: true",
		"max-idle-conns: 20",
	}, "\n"))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	cfg, err := LoadYAML(path)
	if err != nil {
		t.Fatalf("LoadYAML() error = %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("loaded config validation error = %v", err)
	}
	if cfg.TargetURL != "https://one.example.com" || len(cfg.TargetURLs) != 2 {
		t.Errorf("target URLs = %q / %v, want first target and both URLs", cfg.TargetURL, cfg.TargetURLs)
	}
	if cfg.Method != MethodPOST || cfg.Body != "payload" || cfg.Concurrency != 4 || cfg.TotalRequests != 30 {
		t.Errorf("loaded request settings = %+v", cfg)
	}
	if !cfg.UseDuration || cfg.Duration != 5*time.Second || cfg.RPS != 12.5 || cfg.Warmup != 2*time.Second || cfg.RampUp != 3*time.Second {
		t.Errorf("loaded rate/timing settings = %+v", cfg)
	}
	if cfg.MaxP95 == nil || *cfg.MaxP95 != 200*time.Millisecond || cfg.MaxErrorRate == nil || *cfg.MaxErrorRate != 0.01 {
		t.Errorf("loaded thresholds = P95:%v error:%v", cfg.MaxP95, cfg.MaxErrorRate)
	}
	if len(cfg.ExpectedStatusCodes) != 2 || cfg.ExpectedStatusCodes[0] != 200 || cfg.ExpectedStatusCodes[1] != 201 || cfg.EnableHTTP2 {
		t.Errorf("loaded status/HTTP2 settings = statuses:%v http2:%t", cfg.ExpectedStatusCodes, cfg.EnableHTTP2)
	}
	if cfg.OutputCSV != "requests.csv" || cfg.OutputJSON != "summary.json" || cfg.OutputTimeSeriesCSV != "series.csv" {
		t.Errorf("loaded output paths = %+v", cfg)
	}
}

func TestLoadYAMLRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.yaml")
	if err := os.WriteFile(path, []byte("url: https://example.com\nconcurency: 2\n"), 0o644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	if _, err := LoadYAML(path); err == nil {
		t.Error("LoadYAML() error = nil, want unknown-field error")
	}
}
