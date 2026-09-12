package scenarios

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stress-testing/framework"
	"stress-testing/internal/config"
)

func TestHTTPScenarioImplementsInterfaces(t *testing.T) {
	var _ framework.Scenario = (*HTTPScenario)(nil)
	var _ framework.Stage = (*HTTPStage)(nil)
}

func TestHTTPScenarioConfigAndValidate(t *testing.T) {
	fileCfg := &config.FileConfig{
		Name: "ping",
		Target: config.TargetConfig{
			BaseURL: "https://example.com",
			Path:    "/ping",
			Method:  "GET",
			Timeout: 3 * time.Second,
		},
		Load: config.LoadConfig{
			Mode:        config.ModeRequests,
			Requests:    25,
			Concurrency: 5,
		},
	}
	s := NewHTTPScenario(fileCfg)
	if s.Name() != "ping" {
		t.Errorf("Name() = %q", s.Name())
	}
	cfg := s.Config()
	if cfg.UserCount != 25 || cfg.Concurrency != 5 {
		t.Errorf("mapped load config = %+v", cfg)
	}
	if err := s.Validate(cfg); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestHTTPStagePostsJSONToServer(t *testing.T) {
	var gotMethod, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	t.Cleanup(srv.Close)

	fileCfg := &config.FileConfig{
		Name: "login-like",
		Target: config.TargetConfig{
			URL:     srv.URL + "/apiv2/login",
			Method:  "POST",
			Timeout: time.Second,
			Headers: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test",
			},
			Body: `{"user":"demo"}`,
		},
		Load: config.LoadConfig{
			Mode:        config.ModeRequests,
			Requests:    1,
			Concurrency: 1,
		},
	}
	stage := &HTTPStage{cfg: fileCfg}
	if err := stage.Prepare(fileCfg.ToFrameworkConfig()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	t.Cleanup(func() { _ = stage.Cleanup() })

	result, err := stage.Execute(context.Background(), &framework.Context{
		UserID: 1,
		Config: fileCfg.ToFrameworkConfig(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error %q", result.Error)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("status = %d", result.StatusCode)
	}
	if gotMethod != http.MethodPost || gotAuth != "Bearer test" || gotBody != `{"user":"demo"}` {
		t.Errorf("server saw method=%s auth=%s body=%s", gotMethod, gotAuth, gotBody)
	}
}

func TestHTTPStageClassifiesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	fileCfg := &config.FileConfig{
		Name: "errors",
		Target: config.TargetConfig{
			URL:     srv.URL,
			Method:  "GET",
			Timeout: time.Second,
		},
		Load: config.LoadConfig{Mode: config.ModeRequests, Requests: 1, Concurrency: 1},
	}
	stage := &HTTPStage{cfg: fileCfg}
	_ = stage.Prepare(fileCfg.ToFrameworkConfig())
	result, err := stage.Execute(context.Background(), &framework.Context{
		Config: fileCfg.ToFrameworkConfig(),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for HTTP 500")
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d", result.StatusCode)
	}
	if result.ErrorClass != "http_5xx" {
		t.Errorf("ErrorClass = %q, want http_5xx", result.ErrorClass)
	}
}
