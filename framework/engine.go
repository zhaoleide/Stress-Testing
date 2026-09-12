package framework

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Engine 压测引擎
type Engine struct {
	scenario Scenario
	monitor  Monitor
	reporter Reporter
	results  []*StageResult
	metrics  []*SystemMetrics
	mu       sync.RWMutex
}

// NewEngine 创建压测引擎
func NewEngine(scenario Scenario) *Engine {
	return &Engine{
		scenario: scenario,
		monitor:  NewSSHMonitor(),
		reporter: NewDefaultReporter(),
		results:  make([]*StageResult, 0),
		metrics:  make([]*SystemMetrics, 0),
	}
}

// SetMonitor 设置监控器
func (e *Engine) SetMonitor(monitor Monitor) {
	e.monitor = monitor
}

// SetReporter 设置报告器
func (e *Engine) SetReporter(reporter Reporter) {
	e.reporter = reporter
}

// Run 运行压测
func (e *Engine) Run(config *Config) error {
	fmt.Printf("=== 开始压测场景: %s ===\n", e.scenario.Name())
	fmt.Printf("场景描述: %s\n", e.scenario.Description())
	fmt.Printf("配置: 用户数=%d, 并发数=%d\n", config.UserCount, config.Concurrency)

	// 验证配置
	if err := e.scenario.Validate(config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	// 运行前回调
	if err := e.scenario.BeforeRun(config); err != nil {
		return fmt.Errorf("运行前回调失败: %w", err)
	}
	defer e.scenario.AfterRun(e.results)

	// 启动系统监控
	if config.MonitorConfig != nil && config.MonitorConfig.Enabled {
		fmt.Println("启动系统监控...")
		if err := e.monitor.Start(config.MonitorConfig); err != nil {
			fmt.Printf("监控启动失败: %v\n", err)
		} else {
			defer func() {
				e.monitor.Stop()
				e.mu.Lock()
				e.metrics = e.monitor.GetMetrics()
				e.mu.Unlock()
			}()
		}
	}

	// 执行各个阶段
	stages := e.scenario.Stages()
	for _, stage := range stages {
		fmt.Printf("\n--- 执行阶段: %s ---\n", stage.Name())
		
		result, err := e.executeStage(stage, config)
		if err != nil {
			return fmt.Errorf("阶段 %s 执行失败: %w", stage.Name(), err)
		}

		e.mu.Lock()
		e.results = append(e.results, result)
		e.mu.Unlock()

		e.printStageStats(result)
	}

	// 生成报告
	fmt.Println("\n--- 生成测试报告 ---")
	
	// 在生成报告前获取监控数据
	if config.MonitorConfig != nil && config.MonitorConfig.Enabled {
		e.mu.Lock()
		e.metrics = e.monitor.GetMetrics()
		e.mu.Unlock()
	}
	
	if err := e.reporter.GenerateReport(e.results, e.metrics); err != nil {
		return fmt.Errorf("报告生成失败: %w", err)
	}

	fmt.Println("=== 压测完成 ===")
	return nil
}

// executeStage 执行单个阶段
func (e *Engine) executeStage(stage Stage, config *Config) (*StageResult, error) {
	// 准备阶段
	if err := stage.Prepare(config); err != nil {
		return nil, fmt.Errorf("阶段准备失败: %w", err)
	}
	defer stage.Cleanup()

	result := &StageResult{
		StageName: stage.Name(),
		Results:   make([]Result, 0, config.UserCount),
		Data:      make([]interface{}, 0),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	
	resultChan := make(chan Result, config.UserCount)
	dataChan := make(chan interface{}, config.UserCount)
	semaphore := make(chan struct{}, config.Concurrency)

	start := time.Now()

	// 启动工作协程
	for i := 0; i < config.UserCount; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			ctx := &Context{
				UserID:    userID,
				Timestamp: time.Now(),
				Data:      make(map[string]interface{}),
				Config:    config,
			}

			execResult, err := stage.Execute(context.Background(), ctx)
			if err != nil {
				execResult = &Result{
					Success:   false,
					Error:     err.Error(),
					Timestamp: time.Now(),
				}
			}

			if execResult == nil {
				execResult = &Result{
					Success:   false,
					Error:     "stage returned nil result",
					Timestamp: time.Now(),
				}
			}

			resultChan <- *execResult
			if execResult.Data != nil {
				dataChan <- execResult.Data
			}
		}(i)
	}

	wg.Wait()
	close(resultChan)
	close(dataChan)

	// 收集结果
	for res := range resultChan {
		mu.Lock()
		result.Results = append(result.Results, res)
		mu.Unlock()
	}

	// 收集数据
	for data := range dataChan {
		mu.Lock()
		result.Data = append(result.Data, data)
		mu.Unlock()
	}

	result.Duration = time.Since(start)
	result.Stats = e.calculateStats(result.Results, result.Duration)

	return result, nil
}

// calculateStats 计算统计信息
func (e *Engine) calculateStats(results []Result, duration time.Duration) *StageStats {
	stats := &StageStats{
		TotalRequests: len(results),
	}

	if len(results) == 0 {
		return stats
	}

	var totalLatency time.Duration
	stats.MinLatency = results[0].Latency
	stats.MaxLatency = results[0].Latency

	for _, result := range results {
		if result.Success {
			stats.SuccessRequests++
		} else {
			stats.FailedRequests++
		}

		totalLatency += result.Latency
		if result.Latency < stats.MinLatency {
			stats.MinLatency = result.Latency
		}
		if result.Latency > stats.MaxLatency {
			stats.MaxLatency = result.Latency
		}
	}

	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
		stats.AvgLatency = totalLatency / time.Duration(stats.TotalRequests)
	}

	if duration > 0 {
		stats.QPS = float64(stats.SuccessRequests) / duration.Seconds()
	}

	return stats
}

// printStageStats 打印阶段统计信息
func (e *Engine) printStageStats(result *StageResult) {
	stats := result.Stats
	fmt.Printf("\n=== %s 统计结果 ===\n", result.StageName)
	fmt.Printf("总请求数: %d\n", stats.TotalRequests)
	fmt.Printf("成功请求: %d\n", stats.SuccessRequests)
	fmt.Printf("失败请求: %d\n", stats.FailedRequests)
	fmt.Printf("成功率: %.2f%%\n", stats.SuccessRate)
	fmt.Printf("平均延迟: %v\n", stats.AvgLatency)
	fmt.Printf("最小延迟: %v\n", stats.MinLatency)
	fmt.Printf("最大延迟: %v\n", stats.MaxLatency)
	fmt.Printf("QPS: %.2f\n", stats.QPS)
	fmt.Printf("总耗时: %v\n", result.Duration)
	fmt.Printf("数据产出: %d 条\n", len(result.Data))
}

// GetResults 获取测试结果
func (e *Engine) GetResults() []*StageResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.results
}

// GetMetrics 获取监控指标
func (e *Engine) GetMetrics() []*SystemMetrics {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.metrics
}