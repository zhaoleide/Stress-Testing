package framework

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
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
	fmt.Printf("配置: 模式=%s, 请求数=%d, 时长=%v, 并发=%d, 目标RPS=%.1f\n",
		config.LoadMode(), config.RequestCount(), config.Duration, config.Concurrency, config.Rate)

	if err := e.scenario.Validate(config); err != nil {
		return fmt.Errorf("配置验证失败: %w", err)
	}

	if err := e.scenario.BeforeRun(config); err != nil {
		return fmt.Errorf("运行前回调失败: %w", err)
	}
	defer e.scenario.AfterRun(e.results)

	if setter, ok := e.reporter.(interface{ SetOutputDir(string) }); ok && config.ReportDir != "" {
		setter.SetOutputDir(config.ReportDir)
	}
	if setter, ok := e.reporter.(interface{ SetFormats([]string) }); ok && len(config.ReportFormats) > 0 {
		setter.SetFormats(config.ReportFormats)
	}

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

	fmt.Println("\n--- 生成测试报告 ---")

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

func (e *Engine) executeStage(stage Stage, config *Config) (*StageResult, error) {
	if err := stage.Prepare(config); err != nil {
		return nil, fmt.Errorf("阶段准备失败: %w", err)
	}
	defer stage.Cleanup()

	workers := config.Concurrency
	if workers <= 0 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if config.LoadMode() == LoadModeDuration {
		ctx, cancel = context.WithTimeout(context.Background(), config.Duration)
		defer cancel()
	}

	limiter := newRateLimiter(config.Rate)

	var (
		mu        sync.Mutex
		collected []Result
		data      []interface{}
		completed atomic.Int64
		successes atomic.Int64
		wg        sync.WaitGroup
	)

	record := func(res Result) {
		mu.Lock()
		collected = append(collected, res)
		if res.Data != nil {
			data = append(data, res.Data)
		}
		mu.Unlock()
		completed.Add(1)
		if res.Success {
			successes.Add(1)
		}
	}

	start := time.Now()
	stopProgress := e.startProgress(&completed, &successes, start)

	runOne := func(execCtx context.Context, userID int) {
		userCtx := &Context{
			UserID:    userID,
			Timestamp: time.Now(),
			Data:      make(map[string]interface{}),
			Config:    config,
		}
		execResult, err := stage.Execute(execCtx, userCtx)
		if err != nil {
			execResult = &Result{
				Success:    false,
				Error:      err.Error(),
				ErrorClass: "other",
				Timestamp:  time.Now(),
			}
		}
		if execResult == nil {
			execResult = &Result{
				Success:    false,
				Error:      "stage returned nil result",
				ErrorClass: "other",
				Timestamp:  time.Now(),
			}
		}
		record(*execResult)
	}

	if config.LoadMode() == LoadModeDuration {
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				seq := 0
				for {
					if err := limiter.Wait(ctx); err != nil {
						return
					}
					if ctx.Err() != nil {
						return
					}
					runOne(ctx, workerID*1_000_000+seq)
					seq++
				}
			}(i)
		}
	} else {
		jobs := make(chan int)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for userID := range jobs {
					_ = limiter.Wait(context.Background())
					runOne(context.Background(), userID)
				}
			}()
		}
		for i := 0; i < config.RequestCount(); i++ {
			jobs <- i
		}
		close(jobs)
	}

	wg.Wait()
	stopProgress()

	result := &StageResult{
		StageName: stage.Name(),
		Results:   collected,
		Data:      data,
		Duration:  time.Since(start),
	}
	result.Stats = CalculateStats(result.Results, result.Duration)
	return result, nil
}

func (e *Engine) startProgress(completed, successes *atomic.Int64, start time.Time) func() {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				printProgress(completed.Load(), successes.Load(), time.Since(start))
			}
		}
	}()
	return func() {
		once.Do(func() {
			close(done)
			printProgress(completed.Load(), successes.Load(), time.Since(start))
		})
	}
}

func printProgress(done, ok int64, elapsed time.Duration) {
	secs := elapsed.Seconds()
	qps := 0.0
	if secs > 0 {
		qps = float64(done) / secs
	}
	rate := 0.0
	if done > 0 {
		rate = float64(ok) / float64(done) * 100
	}
	fmt.Printf("进度: 已完成=%d 成功率=%.1f%% 近似QPS=%.1f 耗时=%v\n", done, rate, qps, elapsed.Round(time.Millisecond))
}

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
	fmt.Printf("P50: %v  P90: %v  P95: %v  P99: %v\n", stats.P50, stats.P90, stats.P95, stats.P99)
	fmt.Printf("QPS: %.2f\n", stats.QPS)
	fmt.Printf("总耗时: %v\n", result.Duration)
	if len(stats.StatusCounts) > 0 {
		fmt.Printf("状态码: %v\n", stats.StatusCounts)
	}
	if len(stats.ErrorClasses) > 0 {
		fmt.Printf("错误分类: %v\n", stats.ErrorClasses)
	}
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

type rateLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func newRateLimiter(rate float64) *rateLimiter {
	if rate <= 0 {
		return nil
	}
	return &rateLimiter{interval: time.Duration(float64(time.Second) / rate)}
}

func (l *rateLimiter) Wait(ctx context.Context) error {
	if l == nil {
		if ctx != nil {
			return ctx.Err()
		}
		return nil
	}
	l.mu.Lock()
	now := time.Now()
	var wait time.Duration
	if l.next.IsZero() || now.After(l.next) || now.Equal(l.next) {
		l.next = now.Add(l.interval)
		l.mu.Unlock()
		if ctx != nil {
			return ctx.Err()
		}
		return nil
	}
	wait = l.next.Sub(now)
	l.next = l.next.Add(l.interval)
	l.mu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
