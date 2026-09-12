package framework

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultReporter 默认报告器
type DefaultReporter struct {
	outputDir string
	formats   []string
}

// NewDefaultReporter 创建默认报告器
func NewDefaultReporter() *DefaultReporter {
	return &DefaultReporter{
		outputDir: "reports",
		formats:   []string{"html", "json", "csv"},
	}
}

// SetOutputDir 设置输出目录
func (r *DefaultReporter) SetOutputDir(dir string) {
	r.outputDir = dir
}

// SetFormats limits which report files are written.
func (r *DefaultReporter) SetFormats(formats []string) {
	r.formats = append([]string{}, formats...)
}

func (r *DefaultReporter) wants(format string) bool {
	if len(r.formats) == 0 {
		return true
	}
	for _, f := range r.formats {
		if strings.EqualFold(f, format) {
			return true
		}
	}
	return false
}

// GenerateReport 生成测试报告
func (r *DefaultReporter) GenerateReport(results []*StageResult, metrics []*SystemMetrics) error {
	// 确保输出目录存在
	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return fmt.Errorf("创建输出目录失败: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")

	if r.wants("json") {
		if err := r.generateJSONReport(results, metrics, timestamp); err != nil {
			return fmt.Errorf("生成JSON报告失败: %w", err)
		}
		fmt.Printf("- 详细数据: report_%s.json\n", timestamp)
	}
	if r.wants("html") {
		if err := r.generateHTMLReport(results, metrics, timestamp); err != nil {
			return fmt.Errorf("生成HTML报告失败: %w", err)
		}
		fmt.Printf("- HTML报告: report_%s.html\n", timestamp)
	}
	if r.wants("csv") {
		if err := r.generateCSVReport(results, timestamp); err != nil {
			return fmt.Errorf("生成CSV报告失败: %w", err)
		}
		fmt.Printf("- 统计数据: stats_%s.csv\n", timestamp)
	}

	fmt.Printf("测试报告已生成到目录: %s\n", r.outputDir)

	return nil
}

// generateJSONReport 生成JSON报告
func (r *DefaultReporter) generateJSONReport(results []*StageResult, metrics []*SystemMetrics, timestamp string) error {
	report := map[string]interface{}{
		"timestamp":      timestamp,
		"stage_results":  results,
		"system_metrics": metrics,
		"summary":        r.generateSummary(results),
	}

	filename := filepath.Join(r.outputDir, fmt.Sprintf("report_%s.json", timestamp))
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// generateHTMLReport 生成HTML报告
func (r *DefaultReporter) generateHTMLReport(results []*StageResult, metrics []*SystemMetrics, timestamp string) error {
	html := r.buildHTMLContent(results, metrics, timestamp)

	filename := filepath.Join(r.outputDir, fmt.Sprintf("report_%s.html", timestamp))
	return os.WriteFile(filename, []byte(html), 0644)
}

// generateCSVReport 生成CSV统计报告
func (r *DefaultReporter) generateCSVReport(results []*StageResult, timestamp string) error {
	var csv strings.Builder

	csv.WriteString("Stage,TotalRequests,SuccessRequests,FailedRequests,SuccessRate,AvgLatency(ms),MinLatency(ms),MaxLatency(ms),P50(ms),P90(ms),P95(ms),P99(ms),QPS,Duration(s)\n")

	for _, result := range results {
		stats := result.Stats
		csv.WriteString(fmt.Sprintf("%s,%d,%d,%d,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f,%.2f\n",
			result.StageName,
			stats.TotalRequests,
			stats.SuccessRequests,
			stats.FailedRequests,
			stats.SuccessRate,
			ms(stats.AvgLatency),
			ms(stats.MinLatency),
			ms(stats.MaxLatency),
			ms(stats.P50),
			ms(stats.P90),
			ms(stats.P95),
			ms(stats.P99),
			stats.QPS,
			result.Duration.Seconds(),
		))
	}

	filename := filepath.Join(r.outputDir, fmt.Sprintf("stats_%s.csv", timestamp))
	return os.WriteFile(filename, []byte(csv.String()), 0644)
}

// generateSummary 生成测试摘要
func (r *DefaultReporter) generateSummary(results []*StageResult) map[string]interface{} {
	summary := map[string]interface{}{
		"total_stages": len(results),
		"stages":       make([]map[string]interface{}, 0),
	}

	var totalRequests, totalSuccess, totalFailed int
	var totalDuration time.Duration

	for _, result := range results {
		stats := result.Stats
		totalRequests += stats.TotalRequests
		totalSuccess += stats.SuccessRequests
		totalFailed += stats.FailedRequests
		totalDuration += result.Duration

		stageInfo := map[string]interface{}{
			"name":          result.StageName,
			"success_rate":  stats.SuccessRate,
			"qps":           stats.QPS,
			"avg_latency":   stats.AvgLatency.String(),
			"p50":           stats.P50.String(),
			"p90":           stats.P90.String(),
			"p95":           stats.P95.String(),
			"p99":           stats.P99.String(),
			"status_counts": stats.StatusCounts,
			"error_classes": stats.ErrorClasses,
			"data_count":    len(result.Data),
		}
		summary["stages"] = append(summary["stages"].([]map[string]interface{}), stageInfo)
	}

	// 整体统计
	overallSuccessRate := float64(0)
	if totalRequests > 0 {
		overallSuccessRate = float64(totalSuccess) / float64(totalRequests) * 100
	}

	avgQPS := 0.0
	if totalDuration > 0 {
		avgQPS = float64(totalRequests) / totalDuration.Seconds()
	}
	var allResults []Result
	for _, result := range results {
		allResults = append(allResults, result.Results...)
	}
	overallStats := CalculateStats(allResults, totalDuration)

	summary["overall"] = map[string]interface{}{
		"total_requests":   totalRequests,
		"success_requests": totalSuccess,
		"failed_requests":  totalFailed,
		"success_rate":     overallSuccessRate,
		"total_duration":   totalDuration,
		"avg_qps":          avgQPS,
		"p50":              overallStats.P50.String(),
		"p90":              overallStats.P90.String(),
		"p95":              overallStats.P95.String(),
		"p99":              overallStats.P99.String(),
		"status_counts":    overallStats.StatusCounts,
		"error_classes":    overallStats.ErrorClasses,
	}

	return summary
}

// processDataForHTML 预处理数据为HTML显示格式
func (r *DefaultReporter) processDataForHTML(results []*StageResult, timestamp string) map[string]string {
	summary := r.generateSummary(results)
	overall := summary["overall"].(map[string]interface{})

	// 安全地提取数据并格式化为字符串
	data := make(map[string]string)

	data["timestamp"] = timestamp
	data["total_stages"] = fmt.Sprintf("%d", summary["total_stages"].(int))
	data["total_requests"] = fmt.Sprintf("%d", overall["total_requests"].(int))
	data["success_rate"] = fmt.Sprintf("%.1f%%", overall["success_rate"].(float64))
	data["avg_qps"] = fmt.Sprintf("%.1f", overall["avg_qps"].(float64))
	data["total_duration"] = overall["total_duration"].(time.Duration).String()
	data["p50"] = fmt.Sprint(overall["p50"])
	data["p90"] = fmt.Sprint(overall["p90"])
	data["p95"] = fmt.Sprint(overall["p95"])
	data["p99"] = fmt.Sprint(overall["p99"])

	return data
}

func ms(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1e6
}

// buildHTMLContent 构建HTML内容
func (r *DefaultReporter) buildHTMLContent(results []*StageResult, metrics []*SystemMetrics, timestamp string) string {
	// 后台预处理数据，避免前端格式化错误
	processedData := r.processDataForHTML(results, timestamp)

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>压力测试报告 - %s</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .header { background: #f5f5f5; padding: 20px; border-radius: 5px; }
        .summary { display: flex; gap: 20px; margin: 20px 0; }
        .metric { background: #e8f4fd; padding: 15px; border-radius: 5px; flex: 1; text-align: center; }
        .metric h3 { margin: 0 0 10px 0; color: #333; }
        .metric .value { font-size: 24px; font-weight: bold; color: #2196f3; }
        table { width: 100%%; border-collapse: collapse; margin: 20px 0; }
        th, td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        th { background: #f5f5f5; }
        .success { color: #4caf50; }
        .failed { color: #f44336; }
        .chart-container { margin: 20px 0; padding: 20px; background: #f9f9f9; border-radius: 5px; }
        .chart-container h2, .chart-container h3 { margin-top: 0; color: #333; }
        .chart-row { display: flex; gap: 20px; align-items: flex-start; }
        .chart-large { flex: 2; }
        .chart-small { flex: 1; max-width: 400px; }
        canvas { max-width: 100%%; height: auto; }
    </style>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="header">
        <h1>压力测试报告</h1>
        <p>生成时间: %s</p>
        <p>总阶段数: %s</p>
    </div>

    <div class="summary">
        <div class="metric">
            <h3>总请求数</h3>
            <div class="value">%s</div>
        </div>
        <div class="metric">
            <h3>成功率</h3>
            <div class="value">%s</div>
        </div>
        <div class="metric">
            <h3>平均QPS</h3>
            <div class="value">%s</div>
        </div>
        <div class="metric">
            <h3>总耗时</h3>
            <div class="value">%s</div>
        </div>
        <div class="metric">
            <h3>P99</h3>
            <div class="value">%s</div>
        </div>
    </div>

    <h2>阶段详情</h2>
    <table>
        <thead>
            <tr>
                <th>阶段</th>
                <th>总请求</th>
                <th>成功请求</th>
                <th>失败请求</th>
                <th>成功率</th>
                <th>平均延迟</th>
                <th>P50</th>
                <th>P90</th>
                <th>P95</th>
                <th>P99</th>
                <th>QPS</th>
                <th>数据产出</th>
            </tr>
        </thead>
        <tbody>
`, timestamp, processedData["timestamp"], processedData["total_stages"],
		processedData["total_requests"], processedData["success_rate"],
		processedData["avg_qps"], processedData["total_duration"], processedData["p99"])

	// 添加阶段数据行
	for _, result := range results {
		stats := result.Stats
		html += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td>%d</td>
                <td class="success">%d</td>
                <td class="failed">%d</td>
                <td>%.2f%%</td>
                <td>%v</td>
                <td>%v</td>
                <td>%v</td>
                <td>%v</td>
                <td>%v</td>
                <td>%.2f</td>
                <td>%d</td>
            </tr>
`, result.StageName, stats.TotalRequests, stats.SuccessRequests, stats.FailedRequests,
			stats.SuccessRate, stats.AvgLatency, stats.P50, stats.P90, stats.P95, stats.P99,
			stats.QPS, len(result.Data))
	}

	html += `
        </tbody>
    </table>
`
	html += r.buildClassificationTables(results)

	// 添加压测数据图表
	html += r.buildStressTestCharts(results)

	// 添加系统监控图表
	if len(metrics) > 0 {
		html += r.buildSystemMetricsChart(metrics)
	}

	html += `
</body>
</html>`

	return html
}

func (r *DefaultReporter) buildClassificationTables(results []*StageResult) string {
	var b strings.Builder
	b.WriteString(`
    <h2>状态码与错误分类</h2>
    <table>
        <thead>
            <tr><th>阶段</th><th>状态码分布</th><th>错误分类</th></tr>
        </thead>
        <tbody>
`)
	for _, result := range results {
		if result.Stats == nil {
			continue
		}
		b.WriteString(fmt.Sprintf("            <tr><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			result.StageName,
			formatIntMap(result.Stats.StatusCounts),
			formatStringMap(result.Stats.ErrorClasses),
		))
	}
	b.WriteString(`        </tbody>
    </table>
`)
	return b.String()
}

func formatIntMap(m map[int]int) string {
	if len(m) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%d:%d", k, v))
	}
	return strings.Join(parts, ", ")
}

func formatStringMap(m map[string]int) string {
	if len(m) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, fmt.Sprintf("%s:%d", k, v))
	}
	return strings.Join(parts, ", ")
}

// buildSystemMetricsChart 构建系统监控图表
func (r *DefaultReporter) buildSystemMetricsChart(metrics []*SystemMetrics) string {
	if len(metrics) == 0 {
		return ""
	}

	// 构建时间序列数据
	var timestamps, cpuData, memData []string
	for _, metric := range metrics {
		timestamps = append(timestamps, fmt.Sprintf("'%s'", metric.Timestamp.Format("15:04:05")))
		cpuData = append(cpuData, fmt.Sprintf("%.2f", metric.CPUUsage))
		memData = append(memData, fmt.Sprintf("%.2f", metric.MemoryUsage))
	}

	return fmt.Sprintf(`
    <h2>系统性能监控</h2>
    <div class="chart-container">
        <canvas id="systemChart" width="800" height="400"></canvas>
    </div>
    
    <script>
        const ctx = document.getElementById('systemChart').getContext('2d');
        new Chart(ctx, {
            type: 'line',
            data: {
                labels: [%s],
                datasets: [{
                    label: 'CPU使用率(%%)',
                    data: [%s],
                    borderColor: 'rgb(255, 99, 132)',
                    backgroundColor: 'rgba(255, 99, 132, 0.1)',
                    tension: 0.1
                }, {
                    label: '内存使用率(%%)',
                    data: [%s],
                    borderColor: 'rgb(54, 162, 235)',
                    backgroundColor: 'rgba(54, 162, 235, 0.1)',
                    tension: 0.1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        max: 100
                    }
                }
            }
        });
    </script>
`, strings.Join(timestamps, ","), strings.Join(cpuData, ","), strings.Join(memData, ","))
}

// buildStressTestCharts 构建压测数据图表
func (r *DefaultReporter) buildStressTestCharts(results []*StageResult) string {
	if len(results) == 0 {
		return ""
	}

	var chartHTML strings.Builder

	chartHTML.WriteString(`
    <h2>压测数据可视化</h2>
    `)

	// 为每个阶段生成图表
	for i, result := range results {
		if len(result.Results) == 0 || result.Stats == nil {
			continue
		}

		// 构建响应时间数据
		var latencies []float64
		var timestamps []string
		successCount := 0

		for j, res := range result.Results {
			latencies = append(latencies, float64(res.Latency.Nanoseconds())/1e6) // 转换为毫秒
			timestamps = append(timestamps, fmt.Sprintf("'请求%d'", j+1))
			if res.Success {
				successCount++
			}
		}

		// 生成响应时间和成功率并排图表
		chartHTML.WriteString(fmt.Sprintf(`
    <div class="chart-container">
        <div class="chart-row">
            <div class="chart-large">
                <h3>%s - 响应时间分布</h3>
                <canvas id="latencyChart%d" width="800" height="300"></canvas>
            </div>
            <div class="chart-small">
                <h3>成功率分布</h3>
                <canvas id="successChart%d" width="300" height="300"></canvas>
            </div>
        </div>
    </div>
    `, result.StageName, i, i))

		// 生成响应时间直方图
		chartHTML.WriteString(fmt.Sprintf(`
    <div class="chart-container">
        <h3>%s - 响应时间直方图</h3>
        <canvas id="histogramChart%d" width="800" height="300"></canvas>
    </div>
    <div class="chart-container">
        <h3>%s - 延迟分位数</h3>
        <canvas id="percentileChart%d" width="800" height="240"></canvas>
    </div>
    `, result.StageName, i, result.StageName, i))

		// JavaScript代码
		latencyData := strings.Join(func() []string {
			var strs []string
			for _, lat := range latencies {
				strs = append(strs, fmt.Sprintf("%.2f", lat))
			}
			return strs
		}(), ",")

		// 生成JavaScript代码
		timestampLabels := func() string {
			if len(timestamps) > 50 { // 如果数据点太多，只显示部分标签
				var labels []string
				step := len(timestamps) / 50
				if step < 1 {
					step = 1
				}
				for j := 0; j < len(timestamps); j += step {
					labels = append(labels, timestamps[j])
				}
				return strings.Join(labels, ",")
			}
			return strings.Join(timestamps, ",")
		}()

		chartHTML.WriteString(fmt.Sprintf(`
    <script>
        // 响应时间分布图
        const latencyCtx%d = document.getElementById('latencyChart%d').getContext('2d');
        new Chart(latencyCtx%d, {
            type: 'line',
            data: {
                labels: [%s],
                datasets: [{
                    label: '响应时间(ms)',
                    data: [%s],
                    borderColor: 'rgb(75, 192, 192)',
                    backgroundColor: 'rgba(75, 192, 192, 0.1)',
                    tension: 0.1,
                    pointRadius: 2
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: '响应时间 (ms)'
                        }
                    },
                    x: {
                        title: {
                            display: true,
                            text: '请求序号'
                        }
                    }
                }
            }
        });

        // 成功率饼图
        const successCtx%d = document.getElementById('successChart%d').getContext('2d');
        new Chart(successCtx%d, {
            type: 'pie',
            data: {
                labels: ['成功', '失败'],
                datasets: [{
                    data: [%d, %d],
                    backgroundColor: ['#4CAF50', '#F44336']
                }]
            },
            options: {
                responsive: true,
                maintainAspectRatio: true,
                plugins: {
                    legend: {
                        position: 'bottom',
                        labels: {
                            boxWidth: 12,
                            font: {
                                size: 12
                            }
                        }
                    }
                }
            }
        });

        // 响应时间直方图
        const histogramCtx%d = document.getElementById('histogramChart%d').getContext('2d');
        const latencyArray = [%s];
        const minLatency = Math.min(...latencyArray);
        const maxLatency = Math.max(...latencyArray);
        const binCount = 10;
        const binWidth = (maxLatency - minLatency) / binCount;
        
        const histogramData = new Array(binCount).fill(0);
        const histogramLabels = [];
        
        for (let i = 0; i < binCount; i++) {
            const binStart = minLatency + i * binWidth;
            const binEnd = minLatency + (i + 1) * binWidth;
            histogramLabels.push(binStart.toFixed(1) + '-' + binEnd.toFixed(1) + 'ms');
        }
        
        latencyArray.forEach(latency => {
            const binIndex = Math.min(Math.floor((latency - minLatency) / binWidth), binCount - 1);
            histogramData[binIndex]++;
        });
        
        new Chart(histogramCtx%d, {
            type: 'bar',
            data: {
                labels: histogramLabels,
                datasets: [{
                    label: '请求数量',
                    data: histogramData,
                    backgroundColor: 'rgba(54, 162, 235, 0.6)',
                    borderColor: 'rgba(54, 162, 235, 1)',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: '请求数量'
                        }
                    },
                    x: {
                        title: {
                            display: true,
                            text: '响应时间区间 (ms)'
                        }
                    }
                }
            }
        });

        const percentileCtx%d = document.getElementById('percentileChart%d').getContext('2d');
        new Chart(percentileCtx%d, {
            type: 'bar',
            data: {
                labels: ['P50', 'P90', 'P95', 'P99'],
                datasets: [{
                    label: '延迟 (ms)',
                    data: [%.2f, %.2f, %.2f, %.2f],
                    backgroundColor: 'rgba(255, 159, 64, 0.6)',
                    borderColor: 'rgba(255, 159, 64, 1)',
                    borderWidth: 1
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true,
                        title: { display: true, text: '延迟 (ms)' }
                    }
                }
            }
        });
    </script>
    `, i, i, i, timestampLabels, latencyData,
			i, i, i, successCount, len(result.Results)-successCount,
			i, i, latencyData, i,
			i, i, i, ms(result.Stats.P50), ms(result.Stats.P90), ms(result.Stats.P95), ms(result.Stats.P99)))
	}

	return chartHTML.String()
}

// ExportVegetaConfig 导出vegeta配置
func (r *DefaultReporter) ExportVegetaConfig(data []interface{}, url string) error {
	filename := "vegeta-targets.txt"
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, item := range data {
		// 根据数据类型生成不同的请求配置
		switch v := item.(type) {
		case map[string]interface{}:
			if cookie, ok := v["cookie"].(string); ok {
				line := fmt.Sprintf("GET %s\nCookie: %s\n\n", url, cookie)
				file.WriteString(line)
			}
		case string:
			// 假设是cookie字符串
			line := fmt.Sprintf("GET %s\nCookie: %s\n\n", url, v)
			file.WriteString(line)
		}
	}

	fmt.Printf("Vegeta配置文件已生成: %s\n", filename)
	return nil
}
