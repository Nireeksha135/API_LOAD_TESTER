package engine

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
	"github.com/Nireeksha135/API_LOAD_TESTER/internal/endpoint"
)

type requestTemplate struct {
	method             string
	urls               []string
	endpoints          []string // normalized endpoint label per URL, same order as urls
	headers            map[string]string
	contentType        string
	bodyBytes          []byte
	dynamicURL         bool
	dynamicHeaders     bool
	dynamicContentType bool
	dynamicBody        bool
	needsCounter       bool
	needsRandom        bool
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

	template := &requestTemplate{
		method:      cfg.Method,
		urls:        append([]string(nil), cfg.TargetURLs...),
		headers:     headers,
		contentType: cfg.ContentType,
		bodyBytes:   bodyBytes,
	}
	if len(template.urls) == 0 {
		template.urls = []string{cfg.TargetURL}
	}
	template.endpoints = endpoint.Labels(template.urls)
	for _, targetURL := range template.urls {
		template.dynamicURL = template.dynamicURL || strings.Contains(targetURL, "{{")
		template.needsCounter = template.needsCounter || strings.Contains(targetURL, "{{counter}}")
		template.needsRandom = template.needsRandom || strings.Contains(targetURL, "{{random}}")
	}
	template.dynamicBody = strings.Contains(cfg.Body, "{{")
	template.needsCounter = template.needsCounter || strings.Contains(cfg.Body, "{{counter}}")
	template.needsRandom = template.needsRandom || strings.Contains(cfg.Body, "{{random}}")
	template.dynamicContentType = strings.Contains(cfg.ContentType, "{{")
	template.needsCounter = template.needsCounter || strings.Contains(cfg.ContentType, "{{counter}}")
	template.needsRandom = template.needsRandom || strings.Contains(cfg.ContentType, "{{random}}")
	for _, value := range template.headers {
		template.dynamicHeaders = template.dynamicHeaders || strings.Contains(value, "{{")
		template.needsCounter = template.needsCounter || strings.Contains(value, "{{counter}}")
		template.needsRandom = template.needsRandom || strings.Contains(value, "{{random}}")
	}
	return template
}

func (t *requestTemplate) needsRequestID() bool {
	return len(t.urls) > 1 || t.needsCounter
}

// urlIndex returns which configured URL a request with the given ID targets.
// IDs start at 1; 0 means no ID was allocated and selects the first URL.
func (t *requestTemplate) urlIndex(counter uint64) int {
	if counter == 0 {
		counter = 1
	}
	return int((counter - 1) % uint64(len(t.urls)))
}

// endpointFor returns the normalized endpoint label for the request with the given ID.
func (t *requestTemplate) endpointFor(counter uint64) string {
	return t.endpoints[t.urlIndex(counter)]
}

func (t *requestTemplate) newRequest(ctx context.Context, counter uint64) (*http.Request, error) {
	if counter == 0 {
		counter = 1
	}
	counterValue := ""
	if t.needsCounter {
		counterValue = strconv.FormatUint(counter, 10)
	}
	targetURL := t.urls[t.urlIndex(counter)]
	bodyBytes := t.bodyBytes
	contentType := t.contentType
	var randomValue string
	if t.needsRandom {
		var randomBytes [16]byte
		binary.LittleEndian.PutUint64(randomBytes[:8], rand.Uint64())
		binary.LittleEndian.PutUint64(randomBytes[8:], rand.Uint64())
		randomValue = hex.EncodeToString(randomBytes[:])
	}
	replace := func(value string) string {
		value = strings.ReplaceAll(value, "{{counter}}", counterValue)
		return strings.ReplaceAll(value, "{{random}}", randomValue)
	}
	if t.dynamicURL {
		targetURL = replace(targetURL)
	}
	if t.dynamicBody {
		bodyBytes = []byte(replace(string(bodyBytes)))
	}
	if t.dynamicContentType {
		contentType = replace(contentType)
	}

	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, t.method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}

	for k, v := range t.headers {
		if t.dynamicHeaders {
			v = replace(v)
		}
		if strings.EqualFold(k, "Host") {
			req.Host = v
		} else {
			req.Header.Set(k, v)
		}
	}

	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	if len(bodyBytes) > 0 {
		bodyLen := len(bodyBytes)
		req.ContentLength = int64(bodyLen)
	}

	return req, nil
}
