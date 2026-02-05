// Package core provides monitoring service for system metrics and alerting
package core

import (
	"sync"
	"time"
)

// Monitor 监控服务，整合指标采集和告警管理
type Monitor struct {
	metrics      *Metrics          // 指标收集器
	alertManager *AlertManager     // 告警管理器
	history      []MetricsSnapshot // 历史数据
	historySize  int               // 历史数据最大数量
	mu           sync.RWMutex      // 读写锁
	interval     time.Duration     // 采集间隔
	stopChan     chan struct{}     // 停止信号
	running      bool              // 运行状态
}

// NewMonitor 创建监控服务
// interval: 采集间隔
// historySize: 历史数据最大保存数量
func NewMonitor(interval time.Duration, historySize int) *Monitor {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if historySize <= 0 {
		historySize = 360 // 默认保存1小时数据（10秒间隔）
	}

	// 创建告警管理器
	alertManager := NewAlertManager(1000)

	// 添加默认告警规则
	for _, rule := range DefaultAlertRules() {
		alertManager.AddRule(rule)
	}

	// 添加默认日志处理器
	alertManager.AddHandler(NewLogAlertHandler())

	return &Monitor{
		metrics:      GetMetrics(),
		alertManager: alertManager,
		history:      make([]MetricsSnapshot, 0, historySize),
		historySize:  historySize,
		interval:     interval,
		stopChan:     make(chan struct{}),
		running:      false,
	}
}

// Start 启动监控服务
func (m *Monitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopChan = make(chan struct{})
	m.mu.Unlock()

	go m.collectLoop()
}

// Stop 停止监控服务
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)
}

// IsRunning 检查监控服务是否正在运行
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// collectLoop 定时采集循环
func (m *Monitor) collectLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// 立即采集一次
	m.collect()

	for {
		select {
		case <-ticker.C:
			m.collect()
		case <-m.stopChan:
			return
		}
	}
}

// collect 采集指标
func (m *Monitor) collect() {
	// 获取当前快照
	snapshot := m.metrics.GetSnapshot()

	m.mu.Lock()
	// 保存到历史
	m.history = append(m.history, snapshot)

	// 保持最大数量
	if len(m.history) > m.historySize {
		// 移除最旧的数据
		m.history = m.history[len(m.history)-m.historySize:]
	}
	m.mu.Unlock()

	// 检查告警（不需要持有锁，AlertManager 有自己的锁）
	m.alertManager.Check(snapshot)

	// 重置时间窗口
	m.metrics.ResetWindow()
}

// GetCurrentSnapshot 获取当前指标快照
func (m *Monitor) GetCurrentSnapshot() MetricsSnapshot {
	return m.metrics.GetSnapshot()
}

// GetHistory 获取历史数据
// limit: 返回的最大数量，0 表示返回全部
func (m *Monitor) GetHistory(limit int) []MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	// 返回最近的数据（从末尾开始）
	result := make([]MetricsSnapshot, limit)
	start := len(m.history) - limit
	copy(result, m.history[start:])

	// 反转顺序，最新的在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetAlerts 获取告警列表
// limit: 返回的最大数量，0 表示返回全部
func (m *Monitor) GetAlerts(limit int) []Alert {
	return m.alertManager.GetAlerts(limit)
}

// GetUnresolvedAlerts 获取未解决的告警
func (m *Monitor) GetUnresolvedAlerts() []Alert {
	return m.alertManager.GetUnresolvedAlerts()
}

// AddAlertHandler 添加告警处理器
func (m *Monitor) AddAlertHandler(handler AlertHandler) {
	m.alertManager.AddHandler(handler)
}

// AddAlertRule 添加告警规则
func (m *Monitor) AddAlertRule(rule *AlertRule) {
	m.alertManager.AddRule(rule)
}

// GetStats 获取监控统计信息
func (m *Monitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	historyLen := len(m.history)
	running := m.running
	m.mu.RUnlock()

	snapshot := m.metrics.GetSnapshot()
	unresolvedAlerts := m.alertManager.GetUnresolvedAlerts()

	return map[string]interface{}{
		// 监控服务状态
		"running":       running,
		"interval_ms":   m.interval.Milliseconds(),
		"history_size":  m.historySize,
		"history_count": historyLen,

		// 当前指标摘要
		"total_requests": snapshot.TotalRequests,
		"error_requests": snapshot.ErrorRequests,
		"qps":            snapshot.QPS,
		"avg_latency_ms": snapshot.AvgLatencyMs,
		"max_latency_ms": snapshot.MaxLatencyMs,
		"pool_hit_rate":  snapshot.PoolHitRate,
		"cache_hit_rate": snapshot.CacheHitRate,

		// 系统指标
		"num_goroutine": snapshot.NumGoroutine,
		"heap_alloc_mb": float64(snapshot.HeapAllocBytes) / (1024 * 1024),

		// 告警状态
		"unresolved_alerts": len(unresolvedAlerts),

		// 时间戳
		"timestamp": snapshot.Timestamp,
	}
}

// GetMetrics 获取指标收集器
func (m *Monitor) GetMetrics() *Metrics {
	return m.metrics
}

// GetAlertManager 获取告警管理器
func (m *Monitor) GetAlertManager() *AlertManager {
	return m.alertManager
}
