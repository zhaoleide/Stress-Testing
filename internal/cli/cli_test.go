package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stress-testing/internal/config"
)

func TestVersionPrintsVersion(t *testing.T) {
	var out bytes.Buffer
	code := Main([]string{"version"}, &out, &out)
	if code != 0 {
		t.Fatalf("exit = %d, out=%s", code, out.String())
	}
	if !strings.Contains(out.String(), Version) {
		t.Fatalf("expected version %q in %q", Version, out.String())
	}
}

func TestValidateRequiresConfig(t *testing.T) {
	var out bytes.Buffer
	code := Main([]string{"validate"}, &out, &out)
	if code == 0 {
		t.Fatal("expected non-zero exit without -c")
	}
}

func TestValidateAcceptsYAML(t *testing.T) {
	path := writeConfig(t, `
name: cli-validate
target:
  base_url: https://example.com
  path: /ping
  method: GET
  timeout: 2s
load:
  mode: requests
  requests: 1
  concurrency: 1
`)
	var out bytes.Buffer
	code := Main([]string{"validate", "-c", path}, &out, &out)
	if code != 0 {
		t.Fatalf("exit = %d, out=%s", code, out.String())
	}
	if !strings.Contains(strings.ToLower(out.String()), "valid") {
		t.Fatalf("expected success message, got %q", out.String())
	}
}

func TestUnknownCommandFails(t *testing.T) {
	var out bytes.Buffer
	code := Main([]string{"explode"}, &out, &out)
	if code == 0 {
		t.Fatal("expected failure for unknown command")
	}
}

func TestApplyOverridesChangesLoadSettings(t *testing.T) {
	cfg := &config.FileConfig{
		Name: "ov",
		Load: config.LoadConfig{
			Mode:        config.ModeRequests,
			Requests:    1,
			Concurrency: 1,
		},
	}
	if err := applyOverrides(cfg, overrides{
		Concurrency: 9,
		Requests:    42,
		Duration:    "5s",
		Out:         "out-dir",
	}); err != nil {
		t.Fatal(err)
	}
	if cfg.Load.Concurrency != 9 || cfg.Load.Requests != 42 {
		t.Fatalf("load overrides not applied: %+v", cfg.Load)
	}
	if cfg.Load.Duration.String() != "5s" {
		t.Fatalf("duration = %v", cfg.Load.Duration)
	}
	if cfg.Report.Dir != "out-dir" {
		t.Fatalf("report dir = %q", cfg.Report.Dir)
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
