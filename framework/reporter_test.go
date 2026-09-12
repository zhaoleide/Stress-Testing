package framework

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func sampleResults() []*StageResult {
	return []*StageResult{
		{
			StageName: "http",
			Duration:  time.Second,
			Data:      []interface{}{"x"},
			Stats: &StageStats{
				TotalRequests:   4,
				SuccessRequests: 3,
				FailedRequests:  1,
				SuccessRate:     75,
				AvgLatency:      25 * time.Millisecond,
				MinLatency:      10 * time.Millisecond,
				MaxLatency:      40 * time.Millisecond,
				P50:             20 * time.Millisecond,
				P90:             40 * time.Millisecond,
				P95:             40 * time.Millisecond,
				P99:             40 * time.Millisecond,
				QPS:             4,
				StatusCounts:    map[int]int{200: 3, 500: 1},
				ErrorClasses:    map[string]int{"http_5xx": 1},
			},
			Results: []Result{
				{Success: true, Latency: 10 * time.Millisecond, StatusCode: 200},
				{Success: true, Latency: 20 * time.Millisecond, StatusCode: 200},
				{Success: true, Latency: 30 * time.Millisecond, StatusCode: 200},
				{Success: false, Latency: 40 * time.Millisecond, StatusCode: 500, ErrorClass: "http_5xx"},
			},
		},
	}
}

func TestReportsIncludePercentiles(t *testing.T) {
	dir := t.TempDir()
	r := NewDefaultReporter()
	r.SetOutputDir(dir)
	if err := r.GenerateReport(sampleResults(), nil); err != nil {
		t.Fatal(err)
	}

	csvPath := findFile(t, dir, ".csv")
	csv, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(csv)
	for _, col := range []string{"P50(ms)", "P90(ms)", "P95(ms)", "P99(ms)"} {
		if !strings.Contains(text, col) {
			t.Errorf("csv missing %s", col)
		}
	}

	jsonPath := findFile(t, dir, ".json")
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	summary := payload["summary"].(map[string]any)
	overall := summary["overall"].(map[string]any)
	if _, ok := overall["p99"]; !ok {
		t.Errorf("json summary missing p99: %#v", overall)
	}

	htmlPath := findFile(t, dir, ".html")
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	page := string(html)
	if !strings.Contains(page, "P99") {
		t.Error("html missing P99 column")
	}
	if strings.Contains(page, "main.go") {
		t.Error("html should not tell users to edit main.go")
	}
	if strings.Contains(page, "id=\"systemChart\"") {
		t.Error("monitor chart should be omitted when metrics are empty")
	}
}

func TestHTMLIncludesMonitorChartWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	r := NewDefaultReporter()
	r.SetOutputDir(dir)
	metrics := []*SystemMetrics{{
		Timestamp:   time.Now(),
		CPUUsage:    12.5,
		MemoryUsage: 33.3,
	}}
	if err := r.GenerateReport(sampleResults(), metrics); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(findFile(t, dir, ".html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `id="systemChart"`) {
		t.Fatal("expected system monitor chart")
	}
}

func TestReporterHonorsFormats(t *testing.T) {
	dir := t.TempDir()
	r := NewDefaultReporter()
	r.SetOutputDir(dir)
	r.SetFormats([]string{"json"})
	if err := r.GenerateReport(sampleResults(), nil); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".json") {
		t.Fatalf("expected only json report, got %v", names(entries))
	}
}

func findFile(t *testing.T, dir, ext string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ext) {
			return filepath.Join(dir, e.Name())
		}
	}
	t.Fatalf("no %s file in %s (%v)", ext, dir, names(entries))
	return ""
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
