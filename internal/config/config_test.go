package config

import (
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
		{"invalid request count", func(c *Config) { c.TotalRequests = 0 }},
		{"invalid duration", func(c *Config) { c.UseDuration = true }},
		{"invalid timeout", func(c *Config) { c.Timeout = 0 }},
		{"invalid idle connections", func(c *Config) { c.MaxIdleConnsPerHost = 0 }},
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
