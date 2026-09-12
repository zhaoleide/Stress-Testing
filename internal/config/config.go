package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	ModeRequests = "requests"
	ModeDuration = "duration"
)

// FileConfig is the YAML/JSON configuration for a load test run.
type FileConfig struct {
	Name    string        `yaml:"name" json:"name"`
	Target  TargetConfig  `yaml:"target" json:"target"`
	Load    LoadConfig    `yaml:"load" json:"load"`
	Monitor MonitorConfig `yaml:"monitor" json:"monitor"`
	Report  ReportConfig  `yaml:"report" json:"report"`
}

type TargetConfig struct {
	BaseURL            string            `yaml:"base_url" json:"base_url"`
	Path               string            `yaml:"path" json:"path"`
	URL                string            `yaml:"url" json:"url"`
	Method             string            `yaml:"method" json:"method"`
	Headers            map[string]string `yaml:"headers" json:"headers"`
	Body               string            `yaml:"body" json:"body"`
	BodyFile           string            `yaml:"body_file" json:"body_file"`
	Timeout            time.Duration     `yaml:"-" json:"-"`
	TimeoutRaw         string            `yaml:"timeout" json:"timeout"`
	InsecureSkipVerify bool              `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
}

type LoadConfig struct {
	Mode        string        `yaml:"mode" json:"mode"`
	Requests    int           `yaml:"requests" json:"requests"`
	Duration    time.Duration `yaml:"-" json:"-"`
	DurationRaw string        `yaml:"duration" json:"duration"`
	Concurrency int           `yaml:"concurrency" json:"concurrency"`
	Rate        float64       `yaml:"rate" json:"rate"`
}

type MonitorConfig struct {
	Enabled     bool          `yaml:"enabled" json:"enabled"`
	Host        string        `yaml:"host" json:"host"`
	Username    string        `yaml:"username" json:"username"`
	Password    string        `yaml:"password" json:"password"`
	PrivateKey  string        `yaml:"private_key" json:"private_key"`
	Interval    time.Duration `yaml:"-" json:"-"`
	IntervalRaw string        `yaml:"interval" json:"interval"`
	Metrics     []string      `yaml:"metrics" json:"metrics"`
}

type ReportConfig struct {
	Formats []string `yaml:"formats" json:"formats"`
	Dir     string   `yaml:"dir" json:"dir"`
}

func (c *FileConfig) applyDefaults() {
	if c.Target.Method == "" {
		c.Target.Method = "GET"
	} else {
		c.Target.Method = strings.ToUpper(c.Target.Method)
	}
	if c.Target.Headers == nil {
		c.Target.Headers = map[string]string{}
	}
	if c.Load.Mode == "" {
		c.Load.Mode = ModeRequests
	}
	if c.Report.Dir == "" {
		c.Report.Dir = "reports"
	}
	if len(c.Report.Formats) == 0 {
		c.Report.Formats = []string{"html", "json", "csv"}
	}
	if c.Monitor.Interval == 0 {
		c.Monitor.Interval = 2 * time.Second
	}
	if len(c.Monitor.Metrics) == 0 {
		c.Monitor.Metrics = []string{"cpu", "memory", "disk", "network"}
	}
}

func (c *FileConfig) parseDurations() error {
	var err error
	if c.Target.Timeout, err = parseDurationDefault(c.Target.TimeoutRaw, 10*time.Second); err != nil {
		return fmt.Errorf("target.timeout: %w", err)
	}
	if c.Load.Duration, err = parseDurationDefault(c.Load.DurationRaw, 0); err != nil {
		return fmt.Errorf("load.duration: %w", err)
	}
	if c.Monitor.Interval, err = parseDurationDefault(c.Monitor.IntervalRaw, 2*time.Second); err != nil {
		return fmt.Errorf("monitor.interval: %w", err)
	}
	return nil
}

func parseDurationDefault(raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("duration must be non-negative")
	}
	return d, nil
}

func (c *FileConfig) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if err := c.Target.validate(); err != nil {
		return err
	}
	if err := c.Load.validate(); err != nil {
		return err
	}
	if err := c.Monitor.validate(); err != nil {
		return err
	}
	return nil
}

func (t TargetConfig) validate() error {
	if strings.TrimSpace(t.BaseURL) == "" && strings.TrimSpace(t.URL) == "" {
		return fmt.Errorf("target.base_url or target.url is required")
	}
	if t.BaseURL != "" {
		u, err := url.Parse(t.BaseURL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("target.base_url is not a valid URL")
		}
	}
	if t.URL != "" {
		u, err := url.Parse(t.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("target.url is not a valid URL")
		}
	}
	switch t.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
	default:
		return fmt.Errorf("target.method %q is not supported", t.Method)
	}
	if t.Timeout <= 0 {
		return fmt.Errorf("target.timeout must be greater than 0")
	}
	return nil
}

func (l LoadConfig) validate() error {
	switch l.Mode {
	case ModeRequests:
		if l.Requests <= 0 {
			return fmt.Errorf("load.requests must be greater than 0 when mode is %s", ModeRequests)
		}
	case ModeDuration:
		if l.Duration <= 0 {
			return fmt.Errorf("load.duration must be greater than 0 when mode is %s", ModeDuration)
		}
	default:
		return fmt.Errorf("load.mode must be %q or %q", ModeRequests, ModeDuration)
	}
	if l.Concurrency <= 0 {
		return fmt.Errorf("load.concurrency must be greater than 0")
	}
	if l.Rate < 0 {
		return fmt.Errorf("load.rate must be non-negative")
	}
	return nil
}

func (m MonitorConfig) validate() error {
	if !m.Enabled {
		return nil
	}
	if strings.TrimSpace(m.Host) == "" {
		return fmt.Errorf("monitor.host is required when monitor.enabled is true")
	}
	if strings.TrimSpace(m.Username) == "" {
		return fmt.Errorf("monitor.username is required when monitor.enabled is true")
	}
	if strings.TrimSpace(m.Password) == "" && strings.TrimSpace(m.PrivateKey) == "" {
		return fmt.Errorf("monitor.password or monitor.private_key is required when monitor.enabled is true")
	}
	if m.Interval <= 0 {
		return fmt.Errorf("monitor.interval must be greater than 0")
	}
	return nil
}
