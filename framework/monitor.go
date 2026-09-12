package framework

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHMonitor SSH系统监控器
type SSHMonitor struct {
	client    *ssh.Client
	config    *MonitorConfig
	metrics   []*SystemMetrics
	cancel    context.CancelFunc
	mu        sync.RWMutex
	isRunning bool
}

// NewSSHMonitor 创建SSH监控器
func NewSSHMonitor() *SSHMonitor {
	return &SSHMonitor{
		metrics: make([]*SystemMetrics, 0),
	}
}

// Start 开始监控
func (m *SSHMonitor) Start(config *MonitorConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.isRunning {
		return fmt.Errorf("监控已在运行中")
	}

	m.config = config

	// 建立SSH连接
	client, err := m.createSSHClient()
	if err != nil {
		return fmt.Errorf("SSH连接失败: %w", err)
	}
	m.client = client

	// 启动监控协程
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.isRunning = true

	go m.monitorLoop(ctx)

	fmt.Printf("系统监控已启动 (间隔: %v)\n", config.Interval)
	return nil
}

// Stop 停止监控
func (m *SSHMonitor) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.isRunning {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	if m.client != nil {
		m.client.Close()
	}

	m.isRunning = false
	fmt.Printf("系统监控已停止，共收集 %d 个数据点\n", len(m.metrics))
	return nil
}

// GetMetrics 获取监控数据
func (m *SSHMonitor) GetMetrics() []*SystemMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	result := make([]*SystemMetrics, len(m.metrics))
	copy(result, m.metrics)
	return result
}

// createSSHClient 创建SSH客户端
func (m *SSHMonitor) createSSHClient() (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            m.config.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	// 优先使用私钥认证
	if m.config.PrivateKey != "" {
		key, err := parsePrivateKey(m.config.PrivateKey)
		if err == nil {
			config.Auth = []ssh.AuthMethod{ssh.PublicKeys(key)}
		}
	}

	// 回退到密码认证
	if len(config.Auth) == 0 && m.config.Password != "" {
		config.Auth = []ssh.AuthMethod{ssh.Password(m.config.Password)}
	}

	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("未配置有效的认证方式")
	}

	return ssh.Dial("tcp", m.config.Host+":22", config)
}

// parsePrivateKey 解析私钥
func parsePrivateKey(keyPath string) (ssh.Signer, error) {
	// 这里简化处理，实际应该读取文件
	// key, err := ioutil.ReadFile(keyPath)
	// if err != nil {
	//     return nil, err
	// }
	// return ssh.ParsePrivateKey(key)
	return nil, fmt.Errorf("private key parsing not implemented")
}

// monitorLoop 监控循环
func (m *SSHMonitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if metrics, err := m.collectMetrics(); err == nil {
				m.mu.Lock()
				m.metrics = append(m.metrics, metrics)
				m.mu.Unlock()
			} else {
				fmt.Printf("采集系统指标失败: %v\n", err)
			}
		}
	}
}

// collectMetrics 收集系统指标
func (m *SSHMonitor) collectMetrics() (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// 收集CPU使用率
	if cpuUsage, err := m.getCPUUsage(); err == nil {
		metrics.CPUUsage = cpuUsage
	}

	// 收集内存使用率
	if memUsage, err := m.getMemoryUsage(); err == nil {
		metrics.MemoryUsage = memUsage
	}

	// 收集磁盘使用率
	if diskUsage, err := m.getDiskUsage(); err == nil {
		metrics.DiskUsage = diskUsage
	}

	// 收集网络IO
	if netIn, netOut, err := m.getNetworkIO(); err == nil {
		metrics.NetworkIn = netIn
		metrics.NetworkOut = netOut
	}

	// 收集系统负载
	if loadAvg, err := m.getLoadAverage(); err == nil {
		metrics.LoadAvg = loadAvg
	}

	return metrics, nil
}

// getCPUUsage 获取CPU使用率
func (m *SSHMonitor) getCPUUsage() (float64, error) {
	output, err := m.executeCommand("top -bn1 | grep 'Cpu(s)' | head -1")
	if err != nil {
		return 0, err
	}

	// 解析 CPU 使用率，实际格式: "%Cpu(s):  1.6 us,  0.8 sy,  0.0 ni, 97.6 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st"
	re := regexp.MustCompile(`(\d+\.?\d*) id`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		idle, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			cpuUsage := 100 - idle
			return cpuUsage, nil
		}
	}

	return 0, fmt.Errorf("无法解析CPU使用率: %s", output)
}

// getMemoryUsage 获取内存使用率
func (m *SSHMonitor) getMemoryUsage() (float64, error) {
	output, err := m.executeCommand("free -m | grep '^Mem:'")
	if err != nil {
		return 0, err
	}

	// 解析内存信息，格式: "Mem: total used free shared buff/cache available"
	fields := strings.Fields(output)
	if len(fields) >= 3 {
		total, err1 := strconv.ParseFloat(fields[1], 64)
		used, err2 := strconv.ParseFloat(fields[2], 64)
		if err1 == nil && err2 == nil && total > 0 {
			return (used / total) * 100, nil
		}
	}

	return 0, fmt.Errorf("无法解析内存使用率: %s", output)
}

// getDiskUsage 获取磁盘使用率
func (m *SSHMonitor) getDiskUsage() (float64, error) {
	output, err := m.executeCommand("df -h | grep '/$' | head -1")
	if err != nil {
		return 0, err
	}

	// 解析磁盘使用率，格式: "device size used avail use% mountpoint"
	fields := strings.Fields(output)
	if len(fields) >= 5 {
		usePercent := strings.TrimSuffix(fields[4], "%")
		usage, err := strconv.ParseFloat(usePercent, 64)
		if err == nil {
			return usage, nil
		}
	}

	return 0, fmt.Errorf("无法解析磁盘使用率: %s", output)
}

// getNetworkIO 获取网络IO
func (m *SSHMonitor) getNetworkIO() (uint64, uint64, error) {
	output, err := m.executeCommand("cat /proc/net/dev | grep eth0")
	if err != nil {
		// 尝试其他网卡
		output, err = m.executeCommand("cat /proc/net/dev | grep -E 'en[ops][0-9]' | head -1")
		if err != nil {
			return 0, 0, err
		}
	}

	// 解析网络IO统计
	fields := strings.Fields(output)
	if len(fields) >= 10 {
		// bytes received (字段1) 和 bytes transmitted (字段9)
		received, err1 := strconv.ParseUint(fields[1], 10, 64)
		transmitted, err2 := strconv.ParseUint(fields[9], 10, 64)
		if err1 == nil && err2 == nil {
			return received, transmitted, nil
		}
	}

	return 0, 0, fmt.Errorf("无法解析网络IO: %s", output)
}

// getLoadAverage 获取系统负载
func (m *SSHMonitor) getLoadAverage() (float64, error) {
	output, err := m.executeCommand("cat /proc/loadavg")
	if err != nil {
		return 0, err
	}

	// 解析负载平均值，格式: "0.08 0.03 0.05 1/180 12345"
	fields := strings.Fields(output)
	if len(fields) >= 1 {
		loadAvg, err := strconv.ParseFloat(fields[0], 64)
		if err == nil {
			return loadAvg, nil
		}
	}

	return 0, fmt.Errorf("无法解析系统负载: %s", output)
}

// executeCommand 执行SSH命令
func (m *SSHMonitor) executeCommand(command string) (string, error) {
	if m.client == nil {
		return "", fmt.Errorf("SSH客户端未连接")
	}

	session, err := m.client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建SSH会话失败: %w", err)
	}
	defer session.Close()

	output, err := session.Output(command)
	if err != nil {
		return "", fmt.Errorf("执行命令失败: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
