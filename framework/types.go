package framework

import (
	"context"
	"time"
)

const (
	LoadModeRequests = "requests"
	LoadModeDuration = "duration"
)

// Config 压测配置
type Config struct {
	UserCount     int                    `json:"user_count"`     // 兼容旧字段：请求数
	Requests      int                    `json:"requests"`       // 固定请求量（mode=requests）
	Concurrency   int                    `json:"concurrency"`    // 并发数
	Duration      time.Duration          `json:"duration"`       // 持续时间（mode=duration）
	Mode          string                 `json:"mode"`           // requests | duration
	Rate          float64                `json:"rate"`           // 可选目标 RPS
	MonitorConfig *MonitorConfig         `json:"monitor_config"` // 监控配置
	ReportDir     string                 `json:"report_dir"`     // 报告输出目录
	ReportFormats []string               `json:"report_formats"` // html, json, csv
	Params        map[string]interface{} `json:"params"`         // 自定义参数
}

// RequestCount returns the planned request count for requests mode.
func (c *Config) RequestCount() int {
	if c == nil {
		return 0
	}
	if c.Requests > 0 {
		return c.Requests
	}
	return c.UserCount
}

// LoadMode returns the effective load mode, defaulting to requests.
func (c *Config) LoadMode() string {
	if c == nil || c.Mode == "" {
		return LoadModeRequests
	}
	return c.Mode
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	Enabled    bool          `json:"enabled"`     // 是否启用监控
	Host       string        `json:"host"`        // 目标主机
	Username   string        `json:"username"`    // SSH用户名
	Password   string        `json:"password"`    // SSH密码
	PrivateKey string        `json:"private_key"` // SSH私钥路径
	Interval   time.Duration `json:"interval"`    // 采集间隔
	Metrics    []string      `json:"metrics"`     // 监控指标
}

// Context 执行上下文
type Context struct {
	UserID    int                    `json:"user_id"`   // 用户ID
	Timestamp time.Time              `json:"timestamp"` // 时间戳
	Data      map[string]interface{} `json:"data"`      // 共享数据
	Config    *Config                `json:"config"`    // 配置信息
}

// Result 执行结果
type Result struct {
	Success    bool          `json:"success"`     // 是否成功
	Latency    time.Duration `json:"latency"`     // 响应时间
	Error      string        `json:"error"`       // 错误信息
	ErrorClass string        `json:"error_class"` // timeout / connection / http_4xx / http_5xx / other
	StatusCode int           `json:"status_code"` // HTTP 状态码
	Data       interface{}   `json:"data"`        // 返回数据
	Timestamp  time.Time     `json:"timestamp"`   // 时间戳
}

// StageResult 阶段结果
type StageResult struct {
	StageName string        `json:"stage_name"` // 阶段名称
	Results   []Result      `json:"results"`    // 所有结果
	Stats     *StageStats   `json:"stats"`      // 统计信息
	Duration  time.Duration `json:"duration"`   // 总耗时
	Data      []interface{} `json:"data"`       // 阶段产出数据
}

// StageStats 阶段统计
type StageStats struct {
	TotalRequests   int            `json:"total_requests"`   // 总请求数
	SuccessRequests int            `json:"success_requests"` // 成功请求数
	FailedRequests  int            `json:"failed_requests"`  // 失败请求数
	SuccessRate     float64        `json:"success_rate"`     // 成功率
	AvgLatency      time.Duration  `json:"avg_latency"`      // 平均延迟
	MaxLatency      time.Duration  `json:"max_latency"`      // 最大延迟
	MinLatency      time.Duration  `json:"min_latency"`      // 最小延迟
	P50             time.Duration  `json:"p50"`              // 50 分位延迟
	P90             time.Duration  `json:"p90"`              // 90 分位延迟
	P95             time.Duration  `json:"p95"`              // 95 分位延迟
	P99             time.Duration  `json:"p99"`              // 99 分位延迟
	QPS             float64        `json:"qps"`              // 每秒请求数
	StatusCounts    map[int]int    `json:"status_counts"`    // HTTP 状态码分布
	ErrorClasses    map[string]int `json:"error_classes"`    // 错误分类计数
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	Timestamp   time.Time `json:"timestamp"`    // 时间戳
	CPUUsage    float64   `json:"cpu_usage"`    // CPU使用率
	MemoryUsage float64   `json:"memory_usage"` // 内存使用率
	DiskUsage   float64   `json:"disk_usage"`   // 磁盘使用率
	NetworkIn   uint64    `json:"network_in"`   // 网络入流量
	NetworkOut  uint64    `json:"network_out"`  // 网络出流量
	LoadAvg     float64   `json:"load_avg"`     // 系统负载
}

// Stage 压测阶段接口
type Stage interface {
	Name() string                                                   // 阶段名称
	Execute(ctx context.Context, userCtx *Context) (*Result, error) // 执行逻辑
	Prepare(config *Config) error                                   // 准备阶段
	Cleanup() error                                                 // 清理阶段
}

// Scenario 压测场景接口
type Scenario interface {
	Name() string                          // 场景名称
	Description() string                   // 场景描述
	Stages() []Stage                       // 返回所有阶段
	Config() *Config                       // 默认配置
	Validate(config *Config) error         // 配置验证
	BeforeRun(config *Config) error        // 运行前回调
	AfterRun(results []*StageResult) error // 运行后回调
}

// Monitor 系统监控接口
type Monitor interface {
	Start(config *MonitorConfig) error // 开始监控
	Stop() error                       // 停止监控
	GetMetrics() []*SystemMetrics      // 获取监控数据
}

// Reporter 报告生成接口
type Reporter interface {
	GenerateReport(results []*StageResult, metrics []*SystemMetrics) error // 生成报告
	ExportVegetaConfig(data []interface{}, url string) error               // 导出vegeta配置
}
