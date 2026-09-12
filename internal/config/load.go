package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var envPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Load reads a YAML or JSON config file, expands ${ENV} placeholders, and validates it.
func Load(path string) (*FileConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is required")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	expanded := expandEnv(string(raw))
	cfg := &FileConfig{}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal([]byte(expanded), cfg); err != nil {
			return nil, fmt.Errorf("parse json config: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
			return nil, fmt.Errorf("parse yaml config: %w", err)
		}
	default:
		if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
			if jerr := json.Unmarshal([]byte(expanded), cfg); jerr != nil {
				return nil, fmt.Errorf("parse config: yaml: %v; json: %v", err, jerr)
			}
		}
	}

	if err := cfg.parseDurations(); err != nil {
		return nil, err
	}
	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate loads and validates a config file without running a test.
func Validate(path string) error {
	_, err := Load(path)
	return err
}

func expandEnv(s string) string {
	return envPattern.ReplaceAllStringFunc(s, func(match string) string {
		name := envPattern.FindStringSubmatch(match)[1]
		return os.Getenv(name)
	})
}
