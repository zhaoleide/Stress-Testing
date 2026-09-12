package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func TestLoadYAMLExpandsEnvAndParsesDurations(t *testing.T) {
	t.Setenv("TOKEN", "secret-token")
	path := writeTemp(t, "ok.yaml", `
name: api-smoke
target:
  base_url: https://example.com
  path: /api/v1/ping
  method: GET
  headers:
    Authorization: "Bearer ${TOKEN}"
  timeout: 10s
  insecure_skip_verify: false
load:
  mode: requests
  requests: 1000
  duration: 30s
  concurrency: 20
  rate: 50
monitor:
  enabled: false
  interval: 2s
report:
  formats: [html, json, csv]
  dir: reports
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Name != "api-smoke" {
		t.Errorf("Name = %q, want api-smoke", cfg.Name)
	}
	if got := cfg.Target.Headers["Authorization"]; got != "Bearer secret-token" {
		t.Errorf("Authorization header = %q, want Bearer secret-token", got)
	}
	if cfg.Target.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", cfg.Target.Timeout)
	}
	if cfg.Load.Mode != "requests" || cfg.Load.Requests != 1000 || cfg.Load.Concurrency != 20 {
		t.Errorf("load = %+v", cfg.Load)
	}
	if cfg.Load.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", cfg.Load.Duration)
	}
	if cfg.Load.Rate != 50 {
		t.Errorf("Rate = %v, want 50", cfg.Load.Rate)
	}
	if cfg.Monitor.Enabled {
		t.Error("monitor should default/stay disabled")
	}
}

func TestLoadJSONDurationMode(t *testing.T) {
	path := writeTemp(t, "ok.json", `{
  "name": "json-duration",
  "target": {
    "base_url": "https://example.com",
    "path": "/status",
    "method": "POST",
    "body": "{\"ok\":true}",
    "timeout": "5s"
  },
  "load": {
    "mode": "duration",
    "duration": "15s",
    "concurrency": 4
  }
}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.Load.Mode != "duration" {
		t.Errorf("mode = %q, want duration", cfg.Load.Mode)
	}
	if cfg.Load.Duration != 15*time.Second {
		t.Errorf("duration = %v, want 15s", cfg.Load.Duration)
	}
	if cfg.Target.Method != "POST" {
		t.Errorf("method = %q, want POST", cfg.Target.Method)
	}
	if cfg.Target.Body != `{"ok":true}` {
		t.Errorf("body = %q", cfg.Target.Body)
	}
}

func TestLoadRejectsMissingRequiredFields(t *testing.T) {
	path := writeTemp(t, "bad.yaml", `
name: missing-target
load:
  mode: requests
  requests: 10
  concurrency: 1
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for missing base_url")
	}
}

func TestLoadRejectsInvalidMode(t *testing.T) {
	path := writeTemp(t, "bad.yaml", `
name: bad-mode
target:
  base_url: https://example.com
load:
  mode: forever
  concurrency: 1
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error for invalid mode")
	}
}

func TestLoadRejectsRequestsModeWithoutCount(t *testing.T) {
	path := writeTemp(t, "bad.yaml", `
name: no-requests
target:
  base_url: https://example.com
load:
  mode: requests
  concurrency: 2
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error when requests mode has no request count")
	}
}

func TestLoadRejectsDurationModeWithoutDuration(t *testing.T) {
	path := writeTemp(t, "bad.yaml", `
name: no-duration
target:
  base_url: https://example.com
load:
  mode: duration
  concurrency: 2
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error when duration mode has no duration")
	}
}

func TestLoadRejectsEnabledMonitorWithoutHost(t *testing.T) {
	path := writeTemp(t, "bad.yaml", `
name: monitor-incomplete
target:
  base_url: https://example.com
load:
  mode: requests
  requests: 1
  concurrency: 1
monitor:
  enabled: true
  username: monitor
  password: "${SSH_PASSWORD}"
`)
	t.Setenv("SSH_PASSWORD", "placeholder")
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error when monitor is enabled without host")
	}
}

func TestValidateRequiresConfigPath(t *testing.T) {
	if err := Validate(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}
