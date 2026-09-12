package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"stress-testing/internal/config"
)

// JoinURL builds the request URL from an optional absolute URL or base+path.
func JoinURL(baseURL, path, absolute string) (string, error) {
	if strings.TrimSpace(absolute) != "" {
		u, err := url.Parse(absolute)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return "", fmt.Errorf("invalid url: %s", absolute)
		}
		return u.String(), nil
	}
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("base_url or url is required")
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("invalid base_url: %s", baseURL)
	}
	if strings.TrimSpace(path) == "" {
		return base.String(), nil
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}
	return base.ResolveReference(rel).String(), nil
}

// BuildRequest constructs an HTTP request from target configuration.
func BuildRequest(ctx context.Context, target config.TargetConfig) (*http.Request, error) {
	rawURL, err := JoinURL(target.BaseURL, target.Path, target.URL)
	if err != nil {
		return nil, err
	}

	body, err := requestBody(target)
	if err != nil {
		return nil, err
	}

	method := target.Method
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

func requestBody(target config.TargetConfig) (io.Reader, error) {
	if strings.TrimSpace(target.BodyFile) != "" {
		data, err := os.ReadFile(target.BodyFile)
		if err != nil {
			return nil, fmt.Errorf("read body_file: %w", err)
		}
		return bytes.NewReader(data), nil
	}
	if target.Body == "" {
		return nil, nil
	}
	return bytes.NewReader([]byte(target.Body)), nil
}

// Response is a simplified HTTP response used by scenarios.
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// Do executes the request and fully reads the response body.
func Do(client *http.Client, req *http.Request) (*Response, error) {
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return &Response{
		StatusCode: resp.StatusCode,
		Body:       data,
		Header:     resp.Header.Clone(),
	}, nil
}
