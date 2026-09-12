# Claude工作记录 - Go HTTP压力测试框架

## 项目概述

这是一个基于Go语言开发的HTTP压力测试框架，主要用于登录接口的并发压测。项目从简单的压测工具演进为支持系统监控和详细报告生成的完整框架。

## 开发历程

### 初始需求
用户想要创建一个Go语言的HTTP接口压力测试脚本，特别针对登录接口进行压测并收集token数据。

### 架构演进
1. **v1.0** - 基础HTTP压测功能
2. **v1.1** - 添加可插拔的场景架构 
3. **v1.2** - 集成系统监控功能
4. **v1.3** - 完善报告生成和图表展示
5. **v1.4** - 修复关键问题，优化性能

## 核心架构设计

### 主要组件

#### 1. 框架核心 (`framework/`)
- **`types.go`** - 定义核心数据结构
  - `Scenario` / `Stage` / `Monitor` / `Reporter` 接口
  - `Config` / `Result` / `SystemMetrics` 数据结构
  
- **`engine.go`** - 压测引擎实现
  - 并发控制：信号量模式限制并发数
  - 协程管理：每用户一协程的设计
  - 结果收集：channel模式收集执行结果
  - 统计计算：QPS、延迟、成功率等指标

- **`monitor.go`** - 系统监控实现
  - SSH连接：远程服务器监控
  - 指标采集：CPU、内存、磁盘、网络、负载
  - 实时监控：可配置间隔的数据采集

- **`reporter.go`** - 报告生成实现
  - 多格式支持：JSON、HTML、CSV
  - 图表生成：Chart.js集成
  - 数据预处理：后台格式化，前端展示

#### 2. 业务场景 (`scenarios/`)
- **`login_scenario.go`** - 登录压测场景
  - HTTPS支持：TLS证书跳过
  - 超时控制：Context模式的请求超时
  - 错误处理：详细的错误信息收集

#### 3. 主程序 (`main.go`)
- 配置管理：压测参数和监控配置
- 场景实例化：可切换不同的测试场景
- 结果展示：控制台统计信息输出

## 技术实现要点

### 并发控制策略
```go
// 信号量模式控制并发数
semaphore := make(chan struct{}, config.Concurrency)
go func(userID int) {
    semaphore <- struct{}{}        // 获取槽位
    defer func() { <-semaphore }() // 释放槽位
    // 执行压测逻辑
}(i)
```

### 超时控制机制
```go
// Context超时控制
requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()
req, _ := http.NewRequestWithContext(requestCtx, "POST", url, body)
```

### 系统监控实现
```go
// 适配实际Linux系统的top命令输出格式
re := regexp.MustCompile(`(\d+\.?\d*) id`)
if matches := re.FindStringSubmatch(output); len(matches) >= 2 {
    idle, _ := strconv.ParseFloat(matches[1], 64)
    cpuUsage := 100 - idle
}
```

### 报告数据处理
```go
// 后台预处理数据，避免前端格式化错误
func (r *DefaultReporter) processDataForHTML(results []*StageResult, timestamp string) map[string]string {
    data := make(map[string]string)
    data["total_requests"] = fmt.Sprintf("%d", overall["total_requests"].(int))
    data["success_rate"] = fmt.Sprintf("%.1f%%", overall["success_rate"].(float64))
    return data
}
```

## 已解决的关键问题

### 1. 程序挂起问题
**问题描述**：当用户数设置为5000时，程序在前几个请求后挂起
**根本原因**：
- 大量协程创建导致资源耗尽
- 没有proper的并发控制机制
- HTTP请求没有超时控制

**解决方案**：
- 实现信号量模式的并发控制
- 添加Context超时机制
- 优化协程管理和资源释放

### 2. HTML报告格式化错误
**问题描述**：HTML报告显示 `%!s(int=1)` 等格式化错误
**根本原因**：
- `fmt.Sprintf` 格式符与数据类型不匹配
- 直接在HTML模板中进行类型转换

**解决方案**：
- 采用"后台预处理数据，前端直接展示"模式
- 新增 `processDataForHTML` 函数预处理所有数据
- 使用统一的 `%s` 格式符输出字符串

### 3. 系统监控数据丢失
**问题描述**：监控已启用并收集数据，但HTML报告显示"监控未启用"
**根本原因**：
- 时序问题：报告生成在监控数据获取之前
- `defer` 函数执行时机导致的数据传递问题

**解决方案**：
```go
// 在生成报告前主动获取监控数据
if config.MonitorConfig != nil && config.MonitorConfig.Enabled {
    e.mu.Lock()
    e.metrics = e.monitor.GetMetrics()
    e.mu.Unlock()
}
```

### 4. CPU使用率始终为0
**问题描述**：系统监控中CPU使用率显示为0
**根本原因**：
- top命令输出格式与预期不符
- 实际输出：`97.6 id` vs 预期：`98.7%id`

**解决方案**：
- 调试实际的top命令输出格式
- 修正正则表达式：`(\d+\.?\d*) id`
- 验证CPU计算逻辑：`cpuUsage = 100 - idle`

## 性能表现

### 测试配置
- 目标：`https://example.com/apiv2/getVerifyCode`
- 用户数：500
- 并发数：5
- 网络：局域网

### 典型结果
- 成功率：100%
- 平均延迟：~390ms
- QPS：~22
- 总耗时：~23秒

## 架构优势

### 1. 可扩展性
- 接口化设计：`Scenario`、`Stage`、`Monitor`、`Reporter`
- 插件架构：易于添加新的测试场景
- 配置驱动：通过配置文件控制行为

### 2. 稳定性
- 并发控制：防止资源耗尽
- 超时机制：避免无限等待
- 错误处理：详细的错误信息和恢复机制

### 3. 可观测性
- 多维监控：压测指标 + 系统指标
- 可视化报告：图表和统计数据
- 实时反馈：控制台输出 + 文件报告

## 待优化方向

### 1. 功能增强
- 支持更多HTTP方法和认证方式
- 添加数据驱动的测试场景
- 支持分布式压测

### 2. 性能优化
- 连接池复用
- 内存使用优化
- 更细粒度的并发控制

### 3. 监控扩展
- 支持更多监控指标
- 添加告警机制
- 支持多服务器监控

## 使用建议

### 开发环境
- 推荐用于开发和测试环境的接口压测
- 适合中小规模的并发测试（建议1000以下用户数）
- 局域网环境下表现最佳

### 生产使用注意
- 谨慎设置并发数，避免对目标服务器造成过大压力
- 监控服务器资源使用情况
- 建议先小规模测试，再逐步增加压力

### 扩展开发
- 遵循现有的接口设计模式
- 新增场景时考虑可复用性
- 保持测试数据的隔离性

## 文件结构总结

```
Stress-Testing/
├── main.go                    # 入口：场景配置和执行
├── go.mod                     # 依赖管理
├── README.md                  # 用户文档
├── CLAUDE.md                  # 开发记录（本文件）
├── framework/
│   ├── types.go              # 核心接口和数据结构
│   ├── engine.go             # 压测引擎：并发控制+结果收集
│   ├── monitor.go            # 系统监控：SSH连接+指标采集
│   └── reporter.go           # 报告生成：HTML+JSON+CSV
├── scenarios/
│   └── login_scenario.go     # 登录场景：HTTPS请求+超时控制
└── reports/                  # 输出目录：测试报告
    ├── report_*.html         # 可视化报告
    ├── report_*.json         # 详细数据
    └── stats_*.csv           # 统计数据
```

这个项目展示了从简单工具到完整框架的演进过程，在解决实际问题的同时保持了良好的架构设计和代码质量。