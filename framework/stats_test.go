package framework

import (
	"testing"
	"time"
)

func TestPercentileNearestRank(t *testing.T) {
	latencies := make([]time.Duration, 100)
	for i := 0; i < 100; i++ {
		latencies[i] = time.Duration(i+1) * time.Millisecond
	}
	cases := []struct {
		p    float64
		want time.Duration
	}{
		{50, 50 * time.Millisecond},
		{90, 90 * time.Millisecond},
		{95, 95 * time.Millisecond},
		{99, 99 * time.Millisecond},
	}
	for _, tc := range cases {
		got := Percentile(latencies, tc.p)
		if got != tc.want {
			t.Errorf("P%.0f = %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestPercentileEmptyAndSingle(t *testing.T) {
	if got := Percentile(nil, 99); got != 0 {
		t.Errorf("empty percentile = %v", got)
	}
	if got := Percentile([]time.Duration{7 * time.Millisecond}, 99); got != 7*time.Millisecond {
		t.Errorf("single percentile = %v", got)
	}
}

func TestCalculateStatsIncludesPercentilesAndClasses(t *testing.T) {
	results := []Result{
		{Success: true, Latency: 10 * time.Millisecond, StatusCode: 200},
		{Success: true, Latency: 20 * time.Millisecond, StatusCode: 200},
		{Success: true, Latency: 30 * time.Millisecond, StatusCode: 200},
		{Success: false, Latency: 40 * time.Millisecond, StatusCode: 500, ErrorClass: "http_5xx"},
	}
	stats := CalculateStats(results, time.Second)
	if stats.TotalRequests != 4 || stats.SuccessRequests != 3 || stats.FailedRequests != 1 {
		t.Fatalf("counts = %+v", stats)
	}
	if stats.SuccessRate != 75 {
		t.Errorf("success rate = %v", stats.SuccessRate)
	}
	if stats.MinLatency != 10*time.Millisecond || stats.MaxLatency != 40*time.Millisecond {
		t.Errorf("min/max = %v/%v", stats.MinLatency, stats.MaxLatency)
	}
	if stats.P50 == 0 || stats.P99 == 0 {
		t.Errorf("percentiles not populated: %+v", stats)
	}
	if stats.StatusCounts[200] != 3 || stats.StatusCounts[500] != 1 {
		t.Errorf("status counts = %#v", stats.StatusCounts)
	}
	if stats.ErrorClasses["http_5xx"] != 1 {
		t.Errorf("error classes = %#v", stats.ErrorClasses)
	}
	if stats.QPS != 4 {
		t.Errorf("QPS = %v, want 4 (total/duration)", stats.QPS)
	}
}
