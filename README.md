# Go HTTP 压力测试框架

基于Go语言开发的高性能HTTP接口压力测试工具，支持可插拔业务场景和系统监控。

## 🚀 功能特性

### 核心功能
- ✅ **纯Go实现**：无外部依赖，简单高效
- ✅ **并发压测**：支持自定义用户数和并发数
- ✅ **登录场景**：专门针对登录接口的压测场景
- ✅ **HTTPS支持**：自动跳过TLS证书验证
- ✅ **超时控制**：10秒请求超时，避免程序挂起

### 系统监控
- ✅ **SSH监控**：通过SSH连接远程服务器监控
- ✅ **多维指标**：CPU、内存、磁盘、网络IO、系统负载
- ✅ **实时采集**：可配置采集间隔
- ✅ **图表展示**：监控数据自动生成时序图表

### 报告生成
- ✅ **多格式输出**：JSON、HTML、CSV三种格式
- ✅ **可视化图表**：响应时间分布、成功率饼图、响应时间直方图
- ✅ **详细统计**：成功率、平均延迟、QPS、最大最小延迟等
- ✅ **系统监控图表**：CPU、内存使用率时序图

## 📁 项目结构

```
Stress-Testing/
├── main.go                 # 主程序入口
├── framework/              # 核心框架
│   ├── engine.go           # 压测引擎
│   ├── monitor.go          # 系统监控
│   ├── reporter.go         # 报告生成
│   └── types.go           # 类型定义
├── scenarios/              # 业务场景
│   └── login_scenario.go   # 登录压测场景
└── reports/               # 测试报告输出
```

## 🛠️ 快速开始

### 1. 运行压测
```bash
go run main.go
```

### 2. 配置说明
在 `main.go` 中可以自定义以下配置：

```go
// 压测配置
config.UserCount = 500              // 总用户数
config.Concurrency = 5              // 并发数
config.Params["login_url"] = "https://example.com/apiv2/getVerifyCode"
config.Params["password"] = "password123"

// 系统监控配置
config.MonitorConfig = &framework.MonitorConfig{
    Enabled:  true,                  // 启用监控
    Host:     "example.com",       // 目标服务器
    Username: "root",                // SSH用户名
    Password: "" // use env / do not commit secrets,       // SSH密码
    Interval: 2 * time.Second,       // 采集间隔
    Metrics:  []string{"cpu", "memory", "disk", "network"},
}
```

### 3. 查看报告
压测完成后，在 `reports/` 目录下查看：
- `report_*.html` - 可视化HTML报告（推荐）
- `report_*.json` - 详细JSON数据
- `stats_*.csv` - CSV统计数据

## ⚙️ 技术实现

### 并发控制
```go
// 使用信号量控制并发数
semaphore := make(chan struct{}, config.Concurrency)

// 每个用户协程
go func(userID int) {
    semaphore <- struct{}{}        // 获取并发槽位
    defer func() { <-semaphore }() // 释放槽位
    
    // 执行压测逻辑
    result := stage.Execute(ctx, userCtx)
}(i)
```

### 超时控制
```go
// 10秒请求超时
requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

// 带上下文的HTTP请求
req, _ := http.NewRequestWithContext(requestCtx, "POST", loginURL, body)
resp, err := client.Do(req)
```

### 系统监控
```go
// CPU使用率解析（适配实际top命令输出格式）
re := regexp.MustCompile(`(\d+\.?\d*) id`)
if matches := re.FindStringSubmatch(output); len(matches) >= 2 {
    idle, _ := strconv.ParseFloat(matches[1], 64)
    cpuUsage := 100 - idle  // CPU使用率 = 100 - idle
}
```

## 📊 性能数据

### 测试环境
- 目标服务器：example.com
- 测试接口：POST /apiv2/getVerifyCode
- 网络：局域网环境

### 典型测试结果
- **用户数**：500
- **并发数**：5  
- **成功率**：100%
- **平均延迟**：~390ms
- **QPS**：~22
- **总耗时**：~23秒

## 🔧 扩展开发

### 添加新场景
1. 在 `scenarios/` 目录创建新场景文件
2. 实现 `Scenario` 和 `Stage` 接口
3. 在 `main.go` 中使用新场景

```go
type CustomScenario struct {
    stages []framework.Stage
}

func (s *CustomScenario) Name() string {
    return "自定义场景"
}

func (s *CustomScenario) Stages() []framework.Stage {
    return s.stages
}
```

### 自定义监控指标
在 `monitor.go` 中添加新的指标采集函数：

```go
func (m *SSHMonitor) getCustomMetric() (float64, error) {
    output, err := m.executeCommand("your-command")
    // 解析输出并返回指标值
    return value, err
}
```

## ✅ 已解决问题

### 1. 程序挂起问题 
- **问题**：大量用户协程导致程序挂起
- **解决**：添加context超时控制和信号量并发限制

### 2. 报告格式化错误
- **问题**：HTML报告显示`%!s(int=1)`等格式错误
- **解决**：采用"后台预处理数据，前端直接展示"的模式

### 3. 系统监控不显示
- **问题**：监控已启用但HTML显示"监控未启用"
- **解决**：修复时序问题，在生成报告前先获取监控数据

### 4. CPU使用率为0
- **问题**：top命令输出格式与预期不符
- **解决**：修正正则表达式以匹配实际输出格式

## 🎯 核心架构

### 1. 压测引擎 (Engine)
- 并发控制：使用信号量限制并发数
- 协程管理：每个用户一个协程
- 结果收集：通过channel收集执行结果
- 统计计算：自动计算QPS、延迟等指标

### 2. 系统监控 (Monitor)
- SSH连接：远程服务器监控
- 指标采集：CPU、内存、磁盘、网络、负载
- 实时监控：可配置采集间隔
- 数据持久化：监控数据保存到报告中

### 3. 报告生成 (Reporter)
- 多格式支持：JSON、HTML、CSV
- 图表生成：使用Chart.js生成可视化图表
- 数据预处理：后台处理数据格式，前端直接展示
- 响应式设计：HTML报告支持不同屏幕尺寸

## 📈 监控指标

### 压测指标
- 总请求数、成功率、失败率
- 平均/最大/最小响应时间
- QPS（每秒请求数）
- 并发性能表现

### 系统指标
- CPU使用率
- 内存使用率
- 磁盘使用率
- 网络IO流量
- 系统负载

## ⚠️ 注意事项

1. **SSH权限**：确保SSH用户有权限执行系统命令
2. **网络延迟**：监控间隔应考虑网络延迟
3. **资源消耗**：大量并发会消耗较多内存和CPU
4. **目标服务器**：注意不要对生产环境造成过大压力

## 📋 依赖要求

- Go 1.19+
- `golang.org/x/crypto/ssh` (系统监控功能)

## 📄 版本历史

- **v1.0.0** - 基础压测功能，登录场景支持
- **v1.1.0** - 添加系统监控功能
- **v1.2.0** - 完善报告生成，添加图表支持  
- **v1.3.0** - 修复格式化问题和监控显示问题
- **v1.4.0** - 修复CPU监控，完善错误处理

## 📝 许可证

MIT License