// Package cli parses command-line arguments into a validated
// config.Config, using only the standard library "flag" package so
// the tool has zero external dependencies.
package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
)

// headerFlags implements flag.Value so repeated -H flags can be
// collected into a slice, e.g.:
//
//	-H "Authorization: Bearer xyz" -H "X-Request-ID: abc"
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

// Options holds everything parsed directly from the command line,
// including values that are not part of config.Config itself (such
// as output destinations and the help/version flags).
type Options struct {
	Config *config.Config

	ShowHelp    bool
	ShowVersion bool
}

// ParseArgs parses args (typically os.Args[1:]) into an Options
// struct. errOut receives usage/help text when -h/--help is passed.
// A non-nil error means parsing or validation failed and the CLI
// should exit with a non-zero status after printing the error.
func ParseArgs(args []string, errOut io.Writer) (*Options, error) {
	fs := flag.NewFlagSet("api-load-tester", flag.ContinueOnError)
	fs.SetOutput(errOut)

	defaults := config.NewDefaultConfig()

	var (
		targetURL   = fs.String("url", "", "Target URL to load test (required)")
		method      = fs.String("method", defaults.Method, "HTTP method: GET, POST, PUT, DELETE, PATCH, HEAD")
		body        = fs.String("body", "", "Request body, typically JSON (sent with POST/PUT/PATCH/DELETE)")
		bodyFile    = fs.String("body-file", "", "Path to a file containing the request body; overrides -body if set")
		contentType = fs.String("content-type", "", "Content-Type header; defaults to application/json when -body is set and no Content-Type header is given via -H")

		concurrency = fs.Int("c", defaults.Concurrency, "Number of concurrent workers")
		requests    = fs.Int("n", defaults.TotalRequests, "Total number of requests to send (ignored if -d is set)")
		duration    = fs.Duration("d", 0, "Run for a fixed duration instead of a fixed request count, e.g. 30s, 2m")

		timeout             = fs.Duration("timeout", defaults.Timeout, "Per-request timeout")
		insecureSkipVerify  = fs.Bool("insecure", false, "Skip TLS certificate verification")
		disableKeepAlives   = fs.Bool("disable-keep-alives", false, "Disable HTTP keep-alives (force a new connection per request)")
		maxIdleConnsPerHost = fs.Int("max-idle-conns", defaults.MaxIdleConnsPerHost, "Max idle connections kept open per host")

		outputCSV  = fs.String("csv", "", "Path to write a per-request CSV report")
		outputJSON = fs.String("json", "", "Path to write the aggregated JSON summary report")

		verbose     = fs.Bool("v", false, "Verbose per-request logging to stderr")
		showVersion = fs.Bool("version", false, "Print version and exit")
	)

	var headers headerFlags
	fs.Var(&headers, "H", "Custom header \"Key: Value\" (repeatable)")

	fs.Usage = func() {
		fmt.Fprintln(errOut, usageText)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	opts := &Options{}

	if *showVersion {
		opts.ShowVersion = true
		return opts, nil
	}

	cfg := config.NewDefaultConfig()
	cfg.TargetURL = strings.TrimSpace(*targetURL)
	cfg.Method = strings.ToUpper(strings.TrimSpace(*method))
	cfg.Body = *body
	cfg.ContentType = strings.TrimSpace(*contentType)
	cfg.Concurrency = *concurrency
	cfg.TotalRequests = *requests
	cfg.Timeout = *timeout
	cfg.InsecureSkipVerify = *insecureSkipVerify
	cfg.DisableKeepAlives = *disableKeepAlives
	cfg.MaxIdleConnsPerHost = *maxIdleConnsPerHost
	cfg.OutputCSV = strings.TrimSpace(*outputCSV)
	cfg.OutputJSON = strings.TrimSpace(*outputJSON)
	cfg.Verbose = *verbose

	if *duration > 0 {
		cfg.UseDuration = true
		cfg.Duration = *duration
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
	cfg.Headers = parsedHeaders

	if cfg.ContentType == "" && cfg.Body != "" && cfg.Headers["Content-Type"] == "" {
		cfg.ContentType = "application/json"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	opts.Config = cfg
	return opts, nil
}

// usageText is printed above the auto-generated flag list when -h,
// --help, or an invalid flag is supplied.
const usageText = `api-load-tester - a concurrent HTTP API load testing tool

USAGE:
  api-load-tester -url <target> [flags]

EXAMPLES:
  # 1000 GET requests, 50 concurrent workers
  api-load-tester -url https://api.example.com/health -c 50 -n 1000

  # POST with a JSON body and custom headers, run for 30 seconds
  api-load-tester -url https://api.example.com/users -method POST \
      -body '{"name":"load-test"}' -H "Authorization: Bearer TOKEN" \
      -d 30s -c 100

  # Export both a per-request CSV and an aggregated JSON summary
  api-load-tester -url https://api.example.com/health -n 5000 -c 200 \
      -csv results.csv -json summary.json

FLAGS:`
