package examples_test

import (
	"os"
	"path/filepath"
	"testing"

	"stress-testing/internal/cli"
	"stress-testing/internal/config"
)

func TestExampleConfigsValidate(t *testing.T) {
	t.Setenv("TOKEN", "example-token")
	t.Setenv("LOGIN_PASSWORD", "example-password")
	t.Setenv("SSH_PASSWORD", "")

	matches, err := filepath.Glob("*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	jsons, err := filepath.Glob("*.json")
	if err != nil {
		t.Fatal(err)
	}
	matches = append(matches, jsons...)
	if len(matches) == 0 {
		t.Fatal("no example configs found")
	}
	for _, name := range matches {
		name := name
		t.Run(name, func(t *testing.T) {
			if _, err := config.Load(name); err != nil {
				t.Fatalf("Load(%s): %v", name, err)
			}
			if code := cli.Main([]string{"validate", "-c", name}, os.Stdout, os.Stderr); code != 0 {
				t.Fatalf("validate %s exit %d", name, code)
			}
		})
	}
}
