package framework

import (
	"math"
	"sort"
	"time"
)

// Percentile returns the nearest-rank percentile from latencies.
func Percentile(latencies []time.Duration, p float64) time.Duration {
	n := len(latencies)
	if n == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[n-1]
	}
	rank := int(math.Ceil(p / 100 * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

// CalculateStats aggregates request results into stage statistics.
func CalculateStats(results []Result, duration time.Duration) *StageStats {
	stats := &StageStats{
		TotalRequests: len(results),
		StatusCounts:  map[int]int{},
		ErrorClasses:  map[string]int{},
	}
	if len(results) == 0 {
		return stats
	}

	latencies := make([]time.Duration, 0, len(results))
	var totalLatency time.Duration
	stats.MinLatency = results[0].Latency
	stats.MaxLatency = results[0].Latency

	for _, result := range results {
		if result.Success {
			stats.SuccessRequests++
		} else {
			stats.FailedRequests++
		}
		if result.StatusCode != 0 {
			stats.StatusCounts[result.StatusCode]++
		}
		if result.ErrorClass != "" {
			stats.ErrorClasses[result.ErrorClass]++
		}

		latencies = append(latencies, result.Latency)
		totalLatency += result.Latency
		if result.Latency < stats.MinLatency {
			stats.MinLatency = result.Latency
		}
		if result.Latency > stats.MaxLatency {
			stats.MaxLatency = result.Latency
		}
	}

	stats.SuccessRate = float64(stats.SuccessRequests) / float64(stats.TotalRequests) * 100
	stats.AvgLatency = totalLatency / time.Duration(stats.TotalRequests)
	stats.P50 = Percentile(latencies, 50)
	stats.P90 = Percentile(latencies, 90)
	stats.P95 = Percentile(latencies, 95)
	stats.P99 = Percentile(latencies, 99)
	if duration > 0 {
		stats.QPS = float64(stats.TotalRequests) / duration.Seconds()
	}
	return stats
}
