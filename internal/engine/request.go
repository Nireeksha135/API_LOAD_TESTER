package engine

import (
	"bytes"
	"context"
	"io"
	"net/http"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
)

type requestTemplate struct {
	method      string
	url         string
	headers     map[string]string
	contentType string
	bodyBytes   []byte
}

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
