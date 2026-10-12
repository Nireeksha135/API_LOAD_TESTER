package config

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type yamlConfig struct {
	URL                 string            `yaml:"url"`
	URLs                []string          `yaml:"urls"`
	Method              string            `yaml:"method"`
	Headers             map[string]string `yaml:"headers"`
	Body                string            `yaml:"body"`
	ContentType         string            `yaml:"content-type"`
	Concurrency         *int              `yaml:"concurrency"`
	Requests            *int              `yaml:"requests"`
	Duration            string            `yaml:"duration"`
	RPS                 *float64          `yaml:"rps"`
	Warmup              string            `yaml:"warmup"`
	RampUp              string            `yaml:"ramp-up"`
	Timeout             string            `yaml:"timeout"`
	MaxP95              string            `yaml:"max-p95"`
	MaxErrorRate        string            `yaml:"max-error-rate"`
	CSV                 string            `yaml:"csv"`
	JSON                string            `yaml:"json"`
	TimeSeriesCSV       string            `yaml:"time-series-csv"`
	Verbose             *bool             `yaml:"verbose"`
	Insecure            *bool             `yaml:"insecure"`
	DisableKeepAlives   *bool             `yaml:"disable-keep-alives"`
	MaxIdleConnsPerHost *int              `yaml:"max-idle-conns"`
}

// LoadYAML loads a strict YAML configuration file over the standard defaults.
func LoadYAML(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read YAML file %q: %w", path, err)
	}

	var file yamlConfig
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("config: decode YAML file %q: %w", path, err)
	}

	cfg := NewDefaultConfig()
	if file.URL != "" {
		cfg.TargetURL = file.URL
	}
	if len(file.URLs) > 0 {
		cfg.TargetURLs = file.URLs
		cfg.TargetURL = file.URLs[0]
	}
	if file.Method != "" {
		cfg.Method = file.Method
	}
	if file.Headers != nil {
		cfg.Headers = file.Headers
	}
	cfg.Body = file.Body
	cfg.ContentType = file.ContentType
	if file.Concurrency != nil {
		cfg.Concurrency = *file.Concurrency
	}
	if file.Requests != nil {
		cfg.TotalRequests = *file.Requests
	}
	if file.RPS != nil {
		cfg.RPS = *file.RPS
	}
	if file.Duration != "" {
		cfg.Duration, err = time.ParseDuration(file.Duration)
		if err != nil {
			return nil, fmt.Errorf("config: invalid YAML duration %q: %w", file.Duration, err)
		}
		cfg.UseDuration = true
	}
	if cfg.Warmup, err = parseOptionalDuration("warmup", file.Warmup); err != nil {
		return nil, err
	}
	if cfg.RampUp, err = parseOptionalDuration("ramp-up", file.RampUp); err != nil {
		return nil, err
	}
	if file.Timeout != "" {
		cfg.Timeout, err = time.ParseDuration(file.Timeout)
		if err != nil {
			return nil, fmt.Errorf("config: invalid YAML timeout %q: %w", file.Timeout, err)
		}
	}
	if file.MaxP95 != "" {
		limit, parseErr := time.ParseDuration(file.MaxP95)
		if parseErr != nil {
			return nil, fmt.Errorf("config: invalid YAML max-p95 %q: %w", file.MaxP95, parseErr)
		}
		cfg.MaxP95 = &limit
	}
	if file.MaxErrorRate != "" {
		limit, parseErr := ParseErrorRate(file.MaxErrorRate)
		if parseErr != nil {
			return nil, fmt.Errorf("config: invalid YAML max-error-rate %q: %w", file.MaxErrorRate, parseErr)
		}
		cfg.MaxErrorRate = &limit
	}
	cfg.OutputCSV = file.CSV
	cfg.OutputJSON = file.JSON
	cfg.OutputTimeSeriesCSV = file.TimeSeriesCSV
	if file.Verbose != nil {
		cfg.Verbose = *file.Verbose
	}
	if file.Insecure != nil {
		cfg.InsecureSkipVerify = *file.Insecure
	}
	if file.DisableKeepAlives != nil {
		cfg.DisableKeepAlives = *file.DisableKeepAlives
	}
	if file.MaxIdleConnsPerHost != nil {
		cfg.MaxIdleConnsPerHost = *file.MaxIdleConnsPerHost
	}
	return cfg, nil
}

func parseOptionalDuration(name, raw string) (time.Duration, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("config: invalid YAML %s %q: %w", name, raw, err)
	}
	return duration, nil
}

// ParseErrorRate accepts either a fraction such as 0.01 or a percentage such as 1%.
func ParseErrorRate(raw string) (float64, error) {
	percent := strings.HasSuffix(strings.TrimSpace(raw), "%")
	value := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(raw), "%"))
	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("expected a fraction or percentage")
	}
	if percent {
		rate /= 100
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 || rate > 1 {
		return 0, fmt.Errorf("must be between 0 and 1 (or 0%% and 100%%)")
	}
	return rate, nil
}
