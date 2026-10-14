// Package cli parses command-line arguments into validated load-test configuration
// and coordinates execution, reporting, and threshold evaluation.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
)

type headerFlags []string

func (h *headerFlags) String() string {
	if h == nil {
		return ""
	}
	return strings.Join(*h, ", ")
}

func (h *headerFlags) Set(value string) error {
	*h = append(*h, value)
	return nil
}

type stringFlags []string

func (s *stringFlags) String() string {
	return strings.Join(*s, ",")
}

func (s *stringFlags) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type Options struct {
	Config *config.Config

	ShowHelp    bool
	ShowVersion bool
}

func ParseArgs(args []string, errOut io.Writer) (*Options, error) {
	configPath, args, err := extractConfigPath(args)
	if err != nil {
		return nil, err
	}
	defaults := config.NewDefaultConfig()
	if configPath != "" {
		defaults, err = config.LoadYAML(configPath)
		if err != nil {
			return nil, err
		}
	}

	fs := flag.NewFlagSet("api-load-tester", flag.ContinueOnError)
	fs.SetOutput(errOut)
	fs.String("config", configPath, "Path to YAML configuration file")

	maxP95Default := ""
	if defaults.MaxP95 != nil {
		maxP95Default = defaults.MaxP95.String()
	}
	maxErrorRateDefault := ""
	if defaults.MaxErrorRate != nil {
		maxErrorRateDefault = fmt.Sprintf("%g", *defaults.MaxErrorRate)
	}
	expectedStatusDefault := ""
	if len(defaults.ExpectedStatusCodes) > 0 {
		statuses := make([]string, len(defaults.ExpectedStatusCodes))
		for i, status := range defaults.ExpectedStatusCodes {
			statuses[i] = fmt.Sprint(status)
		}
		expectedStatusDefault = strings.Join(statuses, ",")
	}

	var (
		method      = fs.String("method", defaults.Method, "HTTP method: GET, POST, PUT, DELETE, PATCH, HEAD")
		body        = fs.String("body", defaults.Body, "Request body, typically JSON (sent with POST/PUT/PATCH/DELETE)")
		bodyFile    = fs.String("body-file", "", "Path to a file containing the request body; overrides -body if set")
		contentType = fs.String("content-type", defaults.ContentType, "Content-Type header; defaults to application/json when -body is set and no Content-Type header is given via -H")

		concurrency = fs.Int("c", defaults.Concurrency, "Number of concurrent workers")
		requests    = fs.Int("n", defaults.TotalRequests, "Total number of requests to send (ignored if -d is set)")
		duration    = fs.Duration("d", defaults.Duration, "Run for a fixed duration instead of a fixed request count, e.g. 30s, 2m")
		rps         = fs.Float64("rps", defaults.RPS, "Open-loop target arrival rate, max 100000/s (0 disables rate scheduling)")
		warmup      = fs.Duration("warmup", defaults.Warmup, "Unmeasured warm-up duration before the test")
		rampUp      = fs.Duration("ramp-up", defaults.RampUp, "Time to ramp workers or arrival rate up to target")

		timeout             = fs.Duration("timeout", defaults.Timeout, "Per-request timeout")
		maxP95              = fs.String("max-p95", maxP95Default, "Fail if P95 latency exceeds this duration, e.g. 200ms")
		maxErrorRate        = fs.String("max-error-rate", maxErrorRateDefault, "Fail if error rate exceeds this fraction or percentage, e.g. 0.01 or 1%")
		expectStatus        = fs.String("expect-status", expectedStatusDefault, "Comma-separated response status codes considered successful, e.g. 200,201")
		insecureSkipVerify  = fs.Bool("insecure", defaults.InsecureSkipVerify, "Skip TLS certificate verification")
		enableHTTP2         = fs.Bool("http2", defaults.EnableHTTP2, "Enable HTTP/2 negotiation for HTTPS targets")
		disableKeepAlives   = fs.Bool("disable-keep-alives", defaults.DisableKeepAlives, "Disable HTTP keep-alives (force a new connection per request)")
		maxIdleConnsPerHost = fs.Int("max-idle-conns", defaults.MaxIdleConnsPerHost, "Max idle connections kept open per host")

		outputCSV     = fs.String("csv", defaults.OutputCSV, "Path to write a per-request CSV report")
		outputJSON    = fs.String("json", defaults.OutputJSON, "Path to write the aggregated JSON summary report")
		timeSeriesCSV = fs.String("time-series-csv", defaults.OutputTimeSeriesCSV, "Path to write per-second RPS, P95, and error counts")

		verbose     = fs.Bool("v", defaults.Verbose, "Verbose per-request logging to stderr")
		showVersion = fs.Bool("version", false, "Print version and exit")
	)

	var targetURLs stringFlags
	fs.Var(&targetURLs, "url", "Target URL to load test; repeat to distribute requests round-robin")
	var headers headerFlags
	fs.Var(&headers, "H", "Custom header \"Key: Value\" (repeatable)")

	fs.Usage = func() {
		fmt.Fprintln(errOut, usageText)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	var requestsFlagSet, durationFlagSet bool
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "n":
			requestsFlagSet = true
		case "d":
			durationFlagSet = true
		}
	})

	opts := &Options{}

	if *showVersion {
		opts.ShowVersion = true
		return opts, nil
	}

	cfg := defaults
	if len(targetURLs) > 0 {
		cfg.TargetURLs = append([]string(nil), targetURLs...)
		for i := range cfg.TargetURLs {
			cfg.TargetURLs[i] = strings.TrimSpace(cfg.TargetURLs[i])
		}
		cfg.TargetURL = cfg.TargetURLs[0]
	}
	cfg.Method = strings.ToUpper(strings.TrimSpace(*method))
	cfg.Body = *body
	cfg.ContentType = strings.TrimSpace(*contentType)
	cfg.Concurrency = *concurrency
	cfg.TotalRequests = *requests
	cfg.RPS = *rps
	cfg.Warmup = *warmup
	cfg.RampUp = *rampUp
	cfg.Timeout = *timeout
	if raw := strings.TrimSpace(*maxP95); raw != "" {
		limit, err := time.ParseDuration(raw)
		if err != nil {
			return nil, fmt.Errorf("cli: invalid -max-p95 %q: %w", raw, err)
		}
		cfg.MaxP95 = &limit
	}
	if raw := strings.TrimSpace(*maxErrorRate); raw != "" {
		limit, err := config.ParseErrorRate(raw)
		if err != nil {
			return nil, fmt.Errorf("cli: invalid -max-error-rate %q: %w", raw, err)
		}
		cfg.MaxErrorRate = &limit
	}
	if raw := strings.TrimSpace(*expectStatus); raw != "" {
		statuses, err := config.ParseExpectedStatusCodes(raw)
		if err != nil {
			return nil, fmt.Errorf("cli: invalid -expect-status %q: %w", raw, err)
		}
		cfg.ExpectedStatusCodes = statuses
	}
	cfg.InsecureSkipVerify = *insecureSkipVerify
	cfg.EnableHTTP2 = *enableHTTP2
	cfg.DisableKeepAlives = *disableKeepAlives
	cfg.MaxIdleConnsPerHost = *maxIdleConnsPerHost
	cfg.OutputCSV = strings.TrimSpace(*outputCSV)
	cfg.OutputJSON = strings.TrimSpace(*outputJSON)
	cfg.OutputTimeSeriesCSV = strings.TrimSpace(*timeSeriesCSV)
	cfg.Verbose = *verbose

	if durationFlagSet && *duration > 0 {
		cfg.UseDuration = true
		cfg.Duration = *duration
	} else if requestsFlagSet || durationFlagSet {
		cfg.UseDuration = false
	}

	if strings.TrimSpace(*bodyFile) != "" {
		data, err := readBodyFile(*bodyFile)
		if err != nil {
			return nil, err
		}
		cfg.Body = data
	}

	parsedHeaders, err := config.ParseHeaders(headers)
	if err != nil {
		return nil, err
	}
	if cfg.Headers == nil {
		cfg.Headers = make(map[string]string)
	}
	for key, value := range parsedHeaders {
		cfg.Headers[key] = value
	}

	if cfg.ContentType == "" && cfg.Body != "" && cfg.Headers["Content-Type"] == "" {
		cfg.ContentType = "application/json"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts.Config = cfg
	return opts, nil
}

func extractConfigPath(args []string) (string, []string, error) {
	var configPath string
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-config" || arg == "--config" {
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return "", nil, fmt.Errorf("cli: -config requires a file path")
			}
			if configPath != "" {
				return "", nil, fmt.Errorf("cli: -config may only be specified once")
			}
			configPath = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "-config=") || strings.HasPrefix(arg, "--config=") {
			if configPath != "" {
				return "", nil, fmt.Errorf("cli: -config may only be specified once")
			}
			configPath = strings.SplitN(arg, "=", 2)[1]
			if configPath == "" {
				return "", nil, fmt.Errorf("cli: -config requires a file path")
			}
			continue
		}
		remaining = append(remaining, arg)
	}
	return configPath, remaining, nil
}

const usageText = `api-load-tester - a concurrent HTTP API load testing tool

USAGE:
  api-load-tester -url <target> [flags]

EXAMPLES:
  # 1000 GET requests, 50 concurrent workers
  api-load-tester -url https://api.example.com/health -c 50 -n 1000

  # Run at a target arrival rate; excess arrivals are counted as dropped
  api-load-tester -url https://api.example.com/health -d 30s -rps 200 -c 100

  # POST with a JSON body and custom headers, run for 30 seconds
  api-load-tester -url https://api.example.com/users -method POST \
      -body '{"name":"load-test"}' -H "Authorization: Bearer TOKEN" \
      -d 30s -c 100

  # Export both a per-request CSV and an aggregated JSON summary
  api-load-tester -url https://api.example.com/health -n 5000 -c 200 \
      -csv results.csv -json summary.json

  # Key options: -rps, -warmup, -ramp-up, -max-p95, -max-error-rate,
  #              -csv, -json, -time-series-csv, -config, -version

FLAGS:`
