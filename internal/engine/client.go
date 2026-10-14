package engine

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/Nireeksha135/API_LOAD_TESTER/internal/config"
)

func NewHTTPClient(cfg *config.Config) *http.Client {
	maxIdleConnsPerHost := cfg.MaxIdleConnsPerHost
	if cfg.Concurrency > maxIdleConnsPerHost {
		maxIdleConnsPerHost = cfg.Concurrency
	}
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   cfg.Timeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          maxIdleConnsPerHost * 4,
		MaxIdleConnsPerHost:   maxIdleConnsPerHost,
		MaxConnsPerHost:       0, // no cap; let concurrency setting govern load
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     cfg.DisableKeepAlives,
		ForceAttemptHTTP2:     cfg.EnableHTTP2,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify, // #nosec G402 -- opt-in via CLI flag for test targets
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,

		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
