// Package engine implements the core load-generation engine: HTTP
// client construction, request building, the worker pool, and the
// count-based / duration-based dispatch loops that drive requests
// against the target under test.
package engine

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/example/api-load-tester/internal/config"
)

// NewHTTPClient builds an *http.Client tuned for load testing based
// on the supplied Config. Connection pooling is sized so that up to
// cfg.MaxIdleConnsPerHost idle connections can be kept alive per
// host, which avoids artificially throttling throughput via repeated
// TCP/TLS handshakes when DisableKeepAlives is false.
func NewHTTPClient(cfg *config.Config) *http.Client {
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          cfg.MaxIdleConnsPerHost * 4,
		MaxIdleConnsPerHost:   cfg.MaxIdleConnsPerHost,
		MaxConnsPerHost:       0, // no cap; let concurrency setting govern load
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     cfg.DisableKeepAlives,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, // #nosec G402 -- opt-in via CLI flag for test targets
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
		// Load testing tools should not silently follow redirects by
		// default, since that would measure the redirect target's
		// performance instead of the configured TargetURL. Redirects
		// are reported as their own status codes (3xx).
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
