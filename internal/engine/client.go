
package engine

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/Nireeksha/API_LOAD_TESTER/internal/config"
)

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

		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
