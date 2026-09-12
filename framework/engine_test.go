package framework

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingStage struct {
	name  string
	calls atomic.Int64
	delay time.Duration
}

func (s *countingStage) Name() string          { return s.name }
func (s *countingStage) Prepare(*Config) error { return nil }
func (s *countingStage) Cleanup() error        { return nil }
func (s *countingStage) Execute(ctx context.Context, userCtx *Context) (*Result, error) {
	s.calls.Add(1)
	if s.delay > 0 {
		select {
		case <-ctx.Done():
		case <-time.After(s.delay):
		}
	}
	_ = userCtx
	return &Result{Success: true, Latency: time.Millisecond, Timestamp: time.Now(), StatusCode: 200}, nil
}

type stubScenario struct {
	stages         []Stage
	cfg            *Config
	afterRunStages int
}

func (s *stubScenario) Name() string        { return "stub" }
func (s *stubScenario) Description() string { return "test scenario" }
func (s *stubScenario) Stages() []Stage     { return s.stages }
func (s *stubScenario) Config() *Config     { return s.cfg }
func (s *stubScenario) Validate(*Config) error {
	return nil
}
func (s *stubScenario) BeforeRun(*Config) error { return nil }
func (s *stubScenario) AfterRun(results []*StageResult) error {
	s.afterRunStages = len(results)
	return nil
}

func TestEngineRequestsModeStopsAtCount(t *testing.T) {
	stage := &countingStage{name: "count"}
	sc := &stubScenario{stages: []Stage{stage}}
	engine := NewEngine(sc)
	engine.SetReporter(&noopReporter{})
	cfg := &Config{
		Mode:        LoadModeRequests,
		Requests:    8,
		Concurrency: 3,
	}
	if err := engine.Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := stage.calls.Load(); got != 8 {
		t.Fatalf("calls = %d, want 8", got)
	}
	results := engine.GetResults()
	if len(results) != 1 || results[0].Stats.TotalRequests != 8 {
		t.Fatalf("results = %#v", results)
	}
	if sc.afterRunStages != 1 {
		t.Fatalf("AfterRun should see completed stages, got %d", sc.afterRunStages)
	}
}

func TestEngineDurationModeStopsAcceptingWork(t *testing.T) {
	stage := &countingStage{name: "dur", delay: 20 * time.Millisecond}
	engine := NewEngine(&stubScenario{stages: []Stage{stage}})
	engine.SetReporter(&noopReporter{})
	cfg := &Config{
		Mode:        LoadModeDuration,
		Duration:    80 * time.Millisecond,
		Concurrency: 2,
	}
	start := time.Now()
	if err := engine.Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	elapsed := time.Since(start)
	calls := stage.calls.Load()
	if calls < 2 {
		t.Fatalf("expected several duration-mode calls, got %d", calls)
	}
	if elapsed > 600*time.Millisecond {
		t.Fatalf("duration mode ran too long: %v (%d calls)", elapsed, calls)
	}
}

func TestEngineRateLimitSpacesRequests(t *testing.T) {
	stage := &countingStage{name: "rate"}
	engine := NewEngine(&stubScenario{stages: []Stage{stage}})
	engine.SetReporter(&noopReporter{})
	cfg := &Config{
		Mode:        LoadModeRequests,
		Requests:    4,
		Concurrency: 4,
		Rate:        20, // 20 RPS => ~50ms between starts, 4 req ~150ms+
	}
	start := time.Now()
	if err := engine.Run(cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("rate limiter did not space requests: elapsed %v", elapsed)
	}
}

type noopReporter struct{}

func (n *noopReporter) GenerateReport([]*StageResult, []*SystemMetrics) error { return nil }
func (n *noopReporter) ExportVegetaConfig([]interface{}, string) error        { return nil }

type spyMonitor struct {
	started atomic.Bool
}

func (m *spyMonitor) Start(*MonitorConfig) error { m.started.Store(true); return nil }
func (m *spyMonitor) Stop() error                { return nil }
func (m *spyMonitor) GetMetrics() []*SystemMetrics {
	return nil
}

func TestEngineDoesNotStartMonitorWhenDisabled(t *testing.T) {
	stage := &countingStage{name: "mon"}
	engine := NewEngine(&stubScenario{stages: []Stage{stage}})
	mon := &spyMonitor{}
	engine.SetMonitor(mon)
	engine.SetReporter(&noopReporter{})
	cfg := &Config{
		Mode:        LoadModeRequests,
		Requests:    1,
		Concurrency: 1,
		MonitorConfig: &MonitorConfig{
			Enabled: false,
			Host:    "should-not-connect.example.com",
		},
	}
	if err := engine.Run(cfg); err != nil {
		t.Fatal(err)
	}
	if mon.started.Load() {
		t.Fatal("monitor should stay off when enabled=false")
	}
}
