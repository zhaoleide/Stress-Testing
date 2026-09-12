package scenarios

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"stress-testing/framework"
)

// LoginScenario 登录压测场景
type LoginScenario struct {
	stages []framework.Stage
}

// NewLoginScenario 创建登录场景
func NewLoginScenario() *LoginScenario {
	return &LoginScenario{
		stages: []framework.Stage{
			&LoginStage{},
		},
	}
}

func (s *LoginScenario) Name() string {
	return "登录压测场景"
}

func (s *LoginScenario) Description() string {
	return "并发登录接口压测：测试登录接口性能，收集响应数据，同步系统监控"
}

func (s *LoginScenario) Stages() []framework.Stage {
	return s.stages
}

func (s *LoginScenario) Config() *framework.Config {
	return &framework.Config{
		UserCount:   1000,
		Concurrency: 50,
		Duration:    30 * time.Second,
		MonitorConfig: &framework.MonitorConfig{
			Enabled:  false, // 默认关闭系统监控
			Interval: 5 * time.Second,
			Metrics:  []string{"cpu", "memory", "disk", "network"},
		},
		Params: map[string]interface{}{
			"login_url": "http://localhost:8080/login",
			"password":  "password123",
		},
	}
}

func (s *LoginScenario) Validate(config *framework.Config) error {
	if config.UserCount <= 0 {
		return fmt.Errorf("用户数必须大于0")
	}
	if config.Concurrency <= 0 {
		return fmt.Errorf("并发数必须大于0")
	}
	if _, ok := config.Params["login_url"]; !ok {
		return fmt.Errorf("缺少登录URL配置")
	}
	return nil
}

func (s *LoginScenario) BeforeRun(config *framework.Config) error {
	fmt.Println("准备登录压测环境...")
	return nil
}

func (s *LoginScenario) AfterRun(results []*framework.StageResult) error {
	fmt.Println("清理登录压测环境...")
	fmt.Printf("登录压测完成，共执行了 %d 个阶段\n", len(results))

	if len(results) >= 1 {
		loginStats := results[0].Stats
		fmt.Printf("登录压测汇总 - 成功率: %.2f%%, 平均延迟: %v, QPS: %.2f\n",
			loginStats.SuccessRate, loginStats.AvgLatency, loginStats.QPS)
	}

	return nil
}

// LoginStage 登录阶段
type LoginStage struct{}

func (s *LoginStage) Name() string {
	return "并发登录压测"
}

func (s *LoginStage) Prepare(config *framework.Config) error {
	fmt.Printf("准备登录压测，目标并发: %d\n", config.Concurrency)
	return nil
}

func (s *LoginStage) Cleanup() error {
	fmt.Println("登录压测阶段完成")
	return nil
}

func (s *LoginStage) Execute(ctx context.Context, userCtx *framework.Context) (*framework.Result, error) {
	start := time.Now()

	// 构造用户名
	username := fmt.Sprintf("user_%d", userCtx.UserID)
	password := userCtx.Config.Params["password"].(string)
	loginURL := userCtx.Config.Params["login_url"].(string)

	// 添加调试日志
	if userCtx.UserID < 3 {
		fmt.Printf("[DEBUG] User %d 开始请求: %s\n", userCtx.UserID, loginURL)
	}

	// 创建带超时的上下文
	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 执行登录请求
	token, err := s.performLoginWithContext(requestCtx, loginURL, username, password)

	result := &framework.Result{
		Success:   err == nil,
		Latency:   time.Since(start),
		Timestamp: time.Now(),
	}

	// 添加调试日志
	if userCtx.UserID < 3 {
		if err != nil {
			fmt.Printf("[DEBUG] User %d 请求失败: %v (耗时: %v)\n", userCtx.UserID, err, result.Latency)
		} else {
			fmt.Printf("[DEBUG] User %d 请求成功 (耗时: %v)\n", userCtx.UserID, result.Latency)
		}
	}

	if err != nil {
		result.Error = err.Error()
		return result, nil
	}

	// 返回登录成功的数据
	result.Data = map[string]interface{}{
		"username": username,
		"token":    token,
	}

	return result, nil
}

func (s *LoginStage) performLogin(loginURL, username, password string) (string, error) {
	// 发送HTTP请求 - 跳过证书验证
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 空请求体
	var reqBody []byte

	resp, err := client.Post(loginURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容（避免连接泄露）
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 只要是HTTP 200就算成功
	if resp.StatusCode == 200 {
		return fmt.Sprintf("SUCCESS_%s_%d", username, time.Now().UnixNano()%1000), nil
	}

	return "", fmt.Errorf("状态码: %d, 响应: %s", resp.StatusCode, string(body))
}

// performLoginWithContext 带上下文的登录请求
func (s *LoginStage) performLoginWithContext(ctx context.Context, loginURL, username, password string) (string, error) {
	// 发送HTTP请求 - 跳过证书验证
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", loginURL, bytes.NewReader([]byte{}))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		// 检查是否是上下文取消
		if ctx.Err() != nil {
			return "", fmt.Errorf("请求超时: %w", ctx.Err())
		}
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应内容（避免连接泄露）
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 只要是HTTP 200就算成功
	if resp.StatusCode == 200 {
		return fmt.Sprintf("SUCCESS_%s_%d", username, time.Now().UnixNano()%1000), nil
	}

	return "", fmt.Errorf("状态码: %d, 响应: %s", resp.StatusCode, string(body))
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
