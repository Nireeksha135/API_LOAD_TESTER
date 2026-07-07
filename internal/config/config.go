// Package config defines the Config type that drives a load test run,
// along with validation and small helper utilities (such as parsing
// "Key: Value" header strings) used by the CLI layer to build it.
package config

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Supported HTTP methods for the load tester.
const (
	MethodGET    = http.MethodGet
	MethodPOST   = http.MethodPost
	MethodPUT    = http.MethodPut
	MethodDELETE = http.MethodDelete
	MethodPATCH  = http.MethodPatch
	MethodHEAD   = http.MethodHead
)

// validMethods is the set of HTTP methods this tool accepts.
var validMethods = map[string]bool{
	MethodGET:    true,
	MethodPOST:   true,
	MethodPUT:    true,
	MethodDELETE: true,
	MethodPATCH:  true,
	MethodHEAD:   true,
}

// Config holds all the parameters that control a single load test run.
// It is built by the cli package from command-line flags and then
// validated before being handed to the engine.
type Config struct {
	// TargetURL is the fully-qualified URL to send requests to.
	TargetURL string

	// Method is the HTTP method to use (GET, POST, PUT, DELETE, ...).
	Method string

	// Headers is a map of extra HTTP headers to send with every request.
	Headers map[string]string

	// Body is the raw request body (typically JSON) sent with requests
	// that support a body (POST, PUT, PATCH, DELETE).
	Body string

	// ContentType, when non-empty and no explicit Content-Type header
	// was provided in Headers, is set as the Content-Type header.
	ContentType string

	// Concurrency is the number of concurrent worker goroutines that
	// issue requests simultaneously.
	Concurrency int

	// TotalRequests is the total number of requests to issue across
	// all workers. Ignored when UseDuration is true.
	TotalRequests int

	// Duration is how long to run the load test for when UseDuration
	// is true. Ignored otherwise.
	Duration time.Duration

	// UseDuration selects duration-based load testing (run for a fixed
	// amount of time) instead of a fixed request count.
	UseDuration bool

	// Timeout is the per-request HTTP client timeout.
	Timeout time.Duration

	// OutputCSV, if non-empty, is the file path to export a CSV report
	// of every individual request result.
	OutputCSV string

	// OutputJSON, if non-empty, is the file path to export the final
	// aggregated JSON summary report.
	OutputJSON string

	// Verbose enables per-request debug logging.
	Verbose bool

	// InsecureSkipVerify disables TLS certificate verification, useful
	// for testing against local/self-signed HTTPS endpoints.
	InsecureSkipVerify bool

	// DisableKeepAlives, if true, forces a new TCP connection per
	// request instead of reusing pooled connections.
	DisableKeepAlives bool

	// MaxIdleConnsPerHost controls HTTP client connection pooling and
	// should generally be set to at least Concurrency for accurate,
	// non-connection-starved benchmarking.
	MaxIdleConnsPerHost int
}

// NewDefaultConfig returns a Config populated with sane defaults. The
// CLI layer overrides these defaults with user-provided flag values.
func NewDefaultConfig() *Config {
	return &Config{
		Method:              MethodGET,
		Headers:             make(map[string]string),
		Concurrency:         10,
		TotalRequests:       100,
		Duration:            0,
		UseDuration:         false,
		Timeout:             30 * time.Second,
		Verbose:             false,
		InsecureSkipVerify:  false,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 100,
	}
}

// Validate checks that the Config is internally consistent and safe
// to execute against, returning a descriptive error if not.
func (c *Config) Validate() error {
	if c == nil {
		return errors.New("config: config is nil")
	}

	if strings.TrimSpace(c.TargetURL) == "" {
		return errors.New("config: target URL must not be empty")
	}

	parsed, err := url.ParseRequestURI(c.TargetURL)
	if err != nil {
		return fmt.Errorf("config: invalid target URL %q: %w", c.TargetURL, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("config: target URL scheme must be http or https, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return fmt.Errorf("config: target URL %q must include a host", c.TargetURL)
	}

	method := strings.ToUpper(strings.TrimSpace(c.Method))
	if method == "" {
		return errors.New("config: HTTP method must not be empty")
	}
	if !validMethods[method] {
		return fmt.Errorf("config: unsupported HTTP method %q", c.Method)
	}
	c.Method = method

	if c.Concurrency <= 0 {
		return fmt.Errorf("config: concurrency must be greater than 0, got %d", c.Concurrency)
	}

	if c.UseDuration {
		if c.Duration <= 0 {
			return fmt.Errorf("config: duration must be greater than 0 when duration mode is enabled, got %s", c.Duration)
		}
	} else {
		if c.TotalRequests <= 0 {
			return fmt.Errorf("config: total requests must be greater than 0, got %d", c.TotalRequests)
		}
		if c.Concurrency > c.TotalRequests {
			return fmt.Errorf("config: concurrency (%d) must not exceed total requests (%d)", c.Concurrency, c.TotalRequests)
		}
	}

	if c.Timeout <= 0 {
		return fmt.Errorf("config: timeout must be greater than 0, got %s", c.Timeout)
	}

	if c.MaxIdleConnsPerHost <= 0 {
		return fmt.Errorf("config: max idle conns per host must be greater than 0, got %d", c.MaxIdleConnsPerHost)
	}

	if c.Body != "" && (method == MethodGET || method == MethodHEAD) {
		// Not a fatal error: some APIs do accept bodies on GET, so we
		// only guard against the truly invalid HEAD + body combination.
		if method == MethodHEAD {
			return errors.New("config: HTTP HEAD requests must not include a body")
		}
	}

	return nil
}

// ParseHeaders converts a slice of "Key: Value" strings (as typically
// supplied via repeated -H flags on the command line) into a header
// map. It trims whitespace around both key and value and returns an
// error if any entry is malformed (missing the ':' separator or has
// an empty key).
func ParseHeaders(raw []string) (map[string]string, error) {
	headers := make(map[string]string, len(raw))
	for _, entry := range raw {
		idx := strings.Index(entry, ":")
		if idx < 0 {
			return nil, fmt.Errorf("config: invalid header %q, expected format \"Key: Value\"", entry)
		}
		key := strings.TrimSpace(entry[:idx])
		value := strings.TrimSpace(entry[idx+1:])
		if key == "" {
			return nil, fmt.Errorf("config: invalid header %q, key must not be empty", entry)
		}
		headers[key] = value
	}
	return headers, nil
}

// String returns a compact human-readable description of the Config,
// suitable for printing at the start of a run in verbose mode.
func (c *Config) String() string {
	mode := fmt.Sprintf("%d requests", c.TotalRequests)
	if c.UseDuration {
		mode = fmt.Sprintf("%s duration", c.Duration)
	}
	return fmt.Sprintf(
		"target=%s method=%s concurrency=%d mode=%s timeout=%s headers=%d bodyBytes=%d",
		c.TargetURL, c.Method, c.Concurrency, mode, c.Timeout, len(c.Headers), len(c.Body),
	)
}
