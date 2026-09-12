package scenarios

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"stress-testing/framework"
	"stress-testing/internal/config"
	"stress-testing/internal/httpx"
)

// HTTPScenario is a config-driven generic HTTP load scenario.
type HTTPScenario struct {
	file   *config.FileConfig
	stages []framework.Stage
}

// NewHTTPScenario creates a scenario from a loaded file config.
func NewHTTPScenario(file *config.FileConfig) *HTTPScenario {
	if file == nil {
		file = &config.FileConfig{Name: "http"}
	}
	return &HTTPScenario{
		file: file,
		stages: []framework.Stage{
			&HTTPStage{cfg: file},
		},
	}
}

func (s *HTTPScenario) Name() string {
	if s.file != nil && s.file.Name != "" {
		return s.file.Name
	}
	return "http"
}

func (s *HTTPScenario) Description() string {
	target := ""
	if s.file != nil {
		if s.file.Target.URL != "" {
			target = s.file.Target.URL
		} else {
			target = strings.TrimRight(s.file.Target.BaseURL, "/") + "/" + strings.TrimLeft(s.file.Target.Path, "/")
		}
	}
	return fmt.Sprintf("Generic HTTP load test against %s", target)
}

func (s *HTTPScenario) Stages() []framework.Stage {
	return s.stages
}

func (s *HTTPScenario) Config() *framework.Config {
	return s.file.ToFrameworkConfig()
}

func (s *HTTPScenario) Validate(cfg *framework.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if cfg.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be greater than 0")
	}
	switch cfg.LoadMode() {
	case framework.LoadModeRequests:
		if cfg.RequestCount() <= 0 {
			return fmt.Errorf("requests must be greater than 0")
		}
	case framework.LoadModeDuration:
		if cfg.Duration <= 0 {
			return fmt.Errorf("duration must be greater than 0")
		}
	default:
		return fmt.Errorf("unsupported load mode %q", cfg.Mode)
	}
	if _, err := httpx.JoinURL(s.file.Target.BaseURL, s.file.Target.Path, s.file.Target.URL); err != nil {
		return err
	}
	return nil
}

func (s *HTTPScenario) BeforeRun(cfg *framework.Config) error {
	fmt.Printf("准备 HTTP 压测: %s %s\n", s.file.Target.Method, s.requestURL())
	_ = cfg
	return nil
}

func (s *HTTPScenario) AfterRun(results []*framework.StageResult) error {
	fmt.Printf("HTTP 压测完成，共执行了 %d 个阶段\n", len(results))
	return nil
}

func (s *HTTPScenario) requestURL() string {
	u, err := httpx.JoinURL(s.file.Target.BaseURL, s.file.Target.Path, s.file.Target.URL)
	if err != nil {
		return s.file.Target.BaseURL + s.file.Target.Path
	}
	return u
}

// HTTPStage executes one generic HTTP request per call.
type HTTPStage struct {
	cfg    *config.FileConfig
	client *http.Client
}

func (s *HTTPStage) Name() string {
	if s.cfg != nil && s.cfg.Name != "" {
		return s.cfg.Name
	}
	return "http"
}

func (s *HTTPStage) Prepare(cfg *framework.Config) error {
	if s.cfg == nil {
		return fmt.Errorf("http stage missing file config")
	}
	s.client = httpx.NewClient(s.cfg.Target)
	_ = cfg
	return nil
}

func (s *HTTPStage) Cleanup() error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}

func (s *HTTPStage) Execute(ctx context.Context, userCtx *framework.Context) (*framework.Result, error) {
	start := time.Now()
	result := &framework.Result{Timestamp: time.Now()}

	req, err := httpx.BuildRequest(ctx, s.cfg.Target)
	if err != nil {
		result.Latency = time.Since(start)
		result.Error = err.Error()
		result.ErrorClass = "other"
		return result, nil
	}

	resp, err := httpx.Do(s.client, req)
	result.Latency = time.Since(start)
	if err != nil {
		result.Error = err.Error()
		result.ErrorClass = classifyNetError(err)
		return result, nil
	}

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	if !result.Success {
		result.Error = fmt.Sprintf("status %d", resp.StatusCode)
		result.ErrorClass = classifyStatus(resp.StatusCode)
	}
	result.Data = map[string]interface{}{
		"status_code": resp.StatusCode,
		"bytes":       len(resp.Body),
		"user_id":     userID(userCtx),
	}
	return result, nil
}

func userID(userCtx *framework.Context) int {
	if userCtx == nil {
		return 0
	}
	return userCtx.UserID
}

func classifyStatus(code int) string {
	switch {
	case code >= 400 && code < 500:
		return "http_4xx"
	case code >= 500:
		return "http_5xx"
	default:
		return "http_other"
	}
}

func classifyNetError(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return "timeout"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "connection"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timeout"):
		return "timeout"
	case strings.Contains(msg, "connection refused"), strings.Contains(msg, "no such host"), strings.Contains(msg, "connect"):
		return "connection"
	default:
		return "other"
	}
}
