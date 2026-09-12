package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"stress-testing/framework"
	"stress-testing/internal/config"
	"stress-testing/scenarios"
)

// Version is printed by the version command.
const Version = "1.5.0"

type overrides struct {
	Concurrency int
	Requests    int
	Duration    string
	Out         string
}

// Main is the CLI entry used by cmd/stress-testing and tests.
func Main(args []string, stdout, stderr io.Writer) int {
	if stdout == nil {
		stdout = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, Version)
		return 0
	case "validate":
		return cmdValidate(args[1:], stdout, stderr)
	case "run":
		return cmdRun(args[1:], stdout, stderr)
	case "-h", "-help", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `stress-testing — config-driven HTTP load testing

Usage:
  stress-testing run -c config.yaml [flags]
  stress-testing validate -c config.yaml
  stress-testing version

Flags:
  -c, -config string     Path to YAML or JSON config (required for run/validate)
  -concurrency int       Override load.concurrency
  -requests int          Override load.requests
  -duration string       Override load.duration (e.g. 30s)
  -out string            Override report output directory
`)
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	path, _, err := parseCommonFlags("validate", args, stderr)
	if err != nil {
		return 2
	}
	if err := config.Validate(path); err != nil {
		fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "config %s is valid\n", path)
	return 0
}

func cmdRun(args []string, stdout, stderr io.Writer) int {
	path, ov, err := parseCommonFlags("run", args, stderr)
	if err != nil {
		return 2
	}
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := applyOverrides(cfg, ov); err != nil {
		fmt.Fprintf(stderr, "apply overrides: %v\n", err)
		return 2
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return 1
	}

	scenario := scenarios.NewHTTPScenario(cfg)
	engine := framework.NewEngine(scenario)
	fw := cfg.ToFrameworkConfig()
	if err := engine.Run(fw); err != nil {
		fmt.Fprintf(stderr, "run failed: %v\n", err)
		return 1
	}
	_ = stdout
	return 0
}

func parseCommonFlags(name string, args []string, stderr io.Writer) (string, overrides, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	short := fs.String("c", "", "config file")
	long := fs.String("config", "", "config file")
	ov := overrides{}
	fs.IntVar(&ov.Concurrency, "concurrency", 0, "override concurrency")
	fs.IntVar(&ov.Requests, "requests", 0, "override request count")
	fs.StringVar(&ov.Duration, "duration", "", "override duration")
	fs.StringVar(&ov.Out, "out", "", "report output directory")
	if err := fs.Parse(args); err != nil {
		return "", ov, err
	}
	path := strings.TrimSpace(*short)
	if path == "" {
		path = strings.TrimSpace(*long)
	}
	if path == "" {
		return "", ov, fmt.Errorf("config path is required (-c)")
	}
	return path, ov, nil
}

func applyOverrides(cfg *config.FileConfig, ov overrides) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if ov.Concurrency > 0 {
		cfg.Load.Concurrency = ov.Concurrency
	}
	if ov.Requests > 0 {
		cfg.Load.Requests = ov.Requests
	}
	if strings.TrimSpace(ov.Duration) != "" {
		d, err := time.ParseDuration(ov.Duration)
		if err != nil {
			return fmt.Errorf("duration: %w", err)
		}
		cfg.Load.Duration = d
		cfg.Load.DurationRaw = ov.Duration
		if cfg.Load.Mode == config.ModeRequests && cfg.Load.Requests == 0 {
			cfg.Load.Mode = config.ModeDuration
		}
	}
	if strings.TrimSpace(ov.Out) != "" {
		cfg.Report.Dir = ov.Out
	}
	return nil
}
