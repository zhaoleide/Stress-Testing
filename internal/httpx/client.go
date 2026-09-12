package httpx

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"stress-testing/internal/config"
)

// NewClient builds an HTTP client from target settings.
func NewClient(target config.TargetConfig) *http.Client {
	timeout := target.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: target.InsecureSkipVerify,
		},
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}
