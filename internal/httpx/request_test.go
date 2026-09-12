package httpx

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stress-testing/internal/config"
)

func TestJoinURLPrefersAbsoluteURL(t *testing.T) {
	got, err := JoinURL("https://example.com/api", "/v1/ping", "https://other.test/full")
	if err != nil {
		t.Fatalf("JoinURL: %v", err)
	}
	if got != "https://other.test/full" {
		t.Errorf("got %q, want absolute url", got)
	}
}

func TestJoinURLCombinesBaseAndPath(t *testing.T) {
	got, err := JoinURL("https://example.com/api/", "v1/ping", "")
	if err != nil {
		t.Fatalf("JoinURL: %v", err)
	}
	if got != "https://example.com/api/v1/ping" {
		t.Errorf("got %q, want https://example.com/api/v1/ping", got)
	}
}

func TestJoinURLRequiresTarget(t *testing.T) {
	if _, err := JoinURL("", "", ""); err == nil {
		t.Fatal("expected error when no URL parts are provided")
	}
}

func TestNewClientHonorsTimeoutAndInsecureFlag(t *testing.T) {
	client := NewClient(config.TargetConfig{
		Timeout:            1500 * time.Millisecond,
		InsecureSkipVerify: true,
	})
	if client.Timeout != 1500*time.Millisecond {
		t.Errorf("timeout = %v", client.Timeout)
	}
	tr, ok := client.Transport.(*http.Transport)
	if !ok || tr.TLSClientConfig == nil || !tr.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("expected TLS InsecureSkipVerify to be enabled")
	}
}

func TestBuildRequestSetsMethodHeadersAndBody(t *testing.T) {
	req, err := BuildRequest(context.Background(), config.TargetConfig{
		BaseURL: "https://example.com",
		Path:    "/echo",
		Method:  "POST",
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Test":       "yes",
		},
		Body: `{"hello":"world"}`,
	})
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	if req.Method != http.MethodPost {
		t.Errorf("method = %q", req.Method)
	}
	if req.URL.String() != "https://example.com/echo" {
		t.Errorf("url = %q", req.URL)
	}
	if req.Header.Get("X-Test") != "yes" {
		t.Errorf("missing header")
	}
	body, _ := io.ReadAll(req.Body)
	if string(body) != `{"hello":"world"}` {
		t.Errorf("body = %q", body)
	}
}

func TestDoReturnsStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "1" {
			http.Error(w, "missing header", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("created"))
	}))
	t.Cleanup(srv.Close)

	client := NewClient(config.TargetConfig{Timeout: time.Second})
	req, err := BuildRequest(context.Background(), config.TargetConfig{
		URL:     srv.URL + "/items",
		Method:  "POST",
		Headers: map[string]string{"X-Test": "1"},
		Body:    "payload",
	})
	if err != nil {
		t.Fatalf("BuildRequest: %v", err)
	}
	resp, err := Do(client, req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if string(resp.Body) != "created" {
		t.Errorf("body = %q", resp.Body)
	}
}
