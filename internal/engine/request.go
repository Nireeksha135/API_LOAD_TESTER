package engine

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
)

// requestTemplate holds the immutable, pre-computed pieces needed to
// build a new *http.Request on every worker iteration. Building this
// once up front (instead of re-parsing the URL or re-copying headers
// every request) keeps per-request overhead to a minimum so the tool
// measures the target's performance, not its own.
type requestTemplate struct {
	method      string
	url         string
	headers     map[string]string
	contentType string
	bodyBytes   []byte
}

// newRequestTemplate constructs a requestTemplate from a validated
// Config. cfg.Validate() must have been called successfully before
// this is invoked.
func newRequestTemplate(cfg *config.Config) *requestTemplate {
	headers := make(map[string]string, len(cfg.Headers))
	for k, v := range cfg.Headers {
		headers[k] = v
	}

	var bodyBytes []byte
	if cfg.Body != "" {
		bodyBytes = []byte(cfg.Body)
	}

	return &requestTemplate{
		method:      cfg.Method,
		url:         cfg.TargetURL,
		headers:     headers,
		contentType: cfg.ContentType,
		bodyBytes:   bodyBytes,
	}
}

// newRequest builds a fresh *http.Request bound to ctx from the
// template. A new body reader is created on every call since
// http.Request bodies are single-use (io.Reader is consumed after
// one request), which is essential for correctness under concurrent
// reuse of the same template across many worker goroutines.
func (t *requestTemplate) newRequest(ctx context.Context) (*http.Request, error) {
	var bodyReader io.Reader
	if len(t.bodyBytes) > 0 {
		bodyReader = bytes.NewReader(t.bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, t.method, t.url, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	if t.contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", t.contentType)
	}

	if len(t.bodyBytes) > 0 {
		bodyLen := len(t.bodyBytes)
		req.ContentLength = int64(bodyLen)
	}

	return req, nil
}
