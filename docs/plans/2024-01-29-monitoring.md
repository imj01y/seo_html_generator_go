# 系统监控实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现系统资源监控、性能指标采集、告警功能。

**Architecture:** 指标采集 + 阈值告警 + 定期报告

**Tech Stack:** Go, runtime, prometheus metrics (可选)

**依赖:** 阶段7（管理 API）

---

## Task 1: 定义监控指标

**Files:**
- Create: `go-page-server/core/metrics.go`

**Step 1: 创建指标收集器**

```go
package core

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 系统指标
type Metrics struct {
	// 请求指标
	TotalRequests     atomic.Int64
	SuccessRequests   atomic.Int64
	ErrorRequests     atomic.Int64
	TotalLatencyNs    atomic.Int64
	MaxLatencyNs      atomic.Int64

	// 渲染指标
	TotalRenders      atomic.Int64
	RenderErrorCount  atomic.Int64
	TotalRenderTimeNs atomic.Int64

	// 池指标
	PoolHits          atomic.Int64
	PoolMisses        atomic.Int64
	PoolRefills       atomic.Int64

	// 缓存指标
	CacheHits         atomic.Int64
	CacheMisses       atomic.Int64

	// 爬虫检测指标
	SpiderRequests    atomic.Int64
	NormalRequests    atomic.Int64

	// 时间窗口指标
	mu                sync.RWMutex
	windowStart       time.Time
	windowRequests    int64
	windowLatencyNs   int64
}

var globalMetrics = &Metrics{
	windowStart: time.Now(),
}

// GetMetrics 获取全局指标
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordRequest 记录请求
func (m *Metrics) RecordRequest(success bool, latencyNs int64) {
	m.TotalRequests.Add(1)
	m.TotalLatencyNs.Add(latencyNs)

	if success {
		m.SuccessRequests.Add(1)
	} else {
		m.ErrorRequests.Add(1)
	}

	// 更新最大延迟
	for {
		current := m.MaxLatencyNs.Load()
		if latencyNs <= current {
			break
		}
		if m.MaxLatencyNs.CompareAndSwap(current, latencyNs) {
			break
		}
	}

	// 更新时间窗口
	m.mu.Lock()
	m.windowRequests++
	m.windowLatencyNs += latencyNs
	m.mu.Unlock()
}

// RecordRender 记录渲染
func (m *Metrics) RecordRender(success bool, durationNs int64) {
	m.TotalRenders.Add(1)
	m.TotalRenderTimeNs.Add(durationNs)

	if !success {
		m.RenderErrorCount.Add(1)
	}
}

// RecordPoolAccess 记录池访问
func (m *Metrics) RecordPoolAccess(hit bool) {
	if hit {
		m.PoolHits.Add(1)
	} else {
		m.PoolMisses.Add(1)
	}
}

// RecordCacheAccess 记录缓存访问
func (m *Metrics) RecordCacheAccess(hit bool) {
	if hit {
		m.CacheHits.Add(1)
	} else {
		m.CacheMisses.Add(1)
	}
}

// RecordSpiderDetection 记录爬虫检测
func (m *Metrics) RecordSpiderDetection(isSpider bool) {
	if isSpider {
		m.SpiderRequests.Add(1)
	} else {
		m.NormalRequests.Add(1)
	}
}

// GetSnapshot 获取指标快照
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	totalReq := m.TotalRequests.Load()
	successReq := m.SuccessRequests.Load()
	totalLatency := m.TotalLatencyNs.Load()

	var avgLatencyMs float64
	if totalReq > 0 {
		avgLatencyMs = float64(totalLatency) / float64(totalReq) / 1e6
	}

	poolHits := m.PoolHits.Load()
	poolTotal := poolHits + m.PoolMisses.Load()
	var poolHitRate float64
	if poolTotal > 0 {
		poolHitRate = float64(poolHits) / float64(poolTotal) * 100
	}

	cacheHits := m.CacheHits.Load()
	cacheTotal := cacheHits + m.CacheMisses.Load()
	var cacheHitRate float64
	if cacheTotal > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheTotal) * 100
	}

	// 计算 QPS
	m.mu.RLock()
	windowDuration := time.Since(m.windowStart)
	windowReq := m.windowRequests
	m.mu.RUnlock()

	var qps float64
	if windowDuration > 0 {
		qps = float64(windowReq) / windowDuration.Seconds()
	}

	return MetricsSnapshot{
		Timestamp:       time.Now(),

		// 请求指标
		TotalRequests:   totalReq,
		SuccessRequests: successReq,
		ErrorRequests:   m.ErrorRequests.Load(),
		AvgLatencyMs:    avgLatencyMs,
		MaxLatencyMs:    float64(m.MaxLatencyNs.Load()) / 1e6,
		QPS:             qps,

		// 渲染指标
		TotalRenders:    m.TotalRenders.Load(),
		RenderErrors:    m.RenderErrorCount.Load(),

		// 池指标
		PoolHitRate:     poolHitRate,
		PoolRefills:     m.PoolRefills.Load(),

		// 缓存指标
		CacheHitRate:    cacheHitRate,

		// 爬虫指标
		SpiderRequests:  m.SpiderRequests.Load(),
		NormalRequests:  m.NormalRequests.Load(),

		// 系统指标
		NumGoroutine:    runtime.NumGoroutine(),
		HeapAllocBytes:  mem.HeapAlloc,
		HeapSysBytes:    mem.HeapSys,
		GCCycles:        mem.NumGC,
	}
}

// ResetWindow 重置时间窗口
func (m *Metrics) ResetWindow() {
	m.mu.Lock()
	m.windowStart = time.Now()
	m.windowRequests = 0
	m.windowLatencyNs = 0
	m.mu.Unlock()
}

// MetricsSnapshot 指标快照
type MetricsSnapshot struct {
	Timestamp       time.Time `json:"timestamp"`

	// 请求指标
	TotalRequests   int64   `json:"total_requests"`
	SuccessRequests int64   `json:"success_requests"`
	ErrorRequests   int64   `json:"error_requests"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	MaxLatencyMs    float64 `json:"max_latency_ms"`
	QPS             float64 `json:"qps"`

	// 渲染指标
	TotalRenders    int64   `json:"total_renders"`
	RenderErrors    int64   `json:"render_errors"`

	// 池指标
	PoolHitRate     float64 `json:"pool_hit_rate"`
	PoolRefills     int64   `json:"pool_refills"`

	// 缓存指标
	CacheHitRate    float64 `json:"cache_hit_rate"`

	// 爬虫指标
	SpiderRequests  int64   `json:"spider_requests"`
	NormalRequests  int64   `json:"normal_requests"`

	// 系统指标
	NumGoroutine    int     `json:"num_goroutine"`
	HeapAllocBytes  uint64  `json:"heap_alloc_bytes"`
	HeapSysBytes    uint64  `json:"heap_sys_bytes"`
	GCCycles        uint32  `json:"gc_cycles"`
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/metrics.go
git commit -m "feat: add metrics collection"
```

---

## Task 2: 实现告警系统

**Files:**
- Create: `go-page-server/core/alerting.go`

**Step 1: 创建告警系统**

```go
package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo    AlertLevel = "info"
	AlertLevelWarning AlertLevel = "warning"
	AlertLevelError   AlertLevel = "error"
)

// Alert 告警
type Alert struct {
	ID        string     `json:"id"`
	Level     AlertLevel `json:"level"`
	Type      string     `json:"type"`
	Message   string     `json:"message"`
	Value     float64    `json:"value"`
	Threshold float64    `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
	Resolved  bool       `json:"resolved"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name       string
	Type       string
	Condition  func(snapshot MetricsSnapshot) (bool, float64) // 返回是否触发和当前值
	Threshold  float64
	Level      AlertLevel
	Message    string
	Cooldown   time.Duration // 冷却时间
	lastAlert  time.Time
}

// AlertManager 告警管理器
type AlertManager struct {
	rules       []*AlertRule
	alerts      []Alert
	mu          sync.RWMutex
	maxAlerts   int
	handlers    []AlertHandler
}

// AlertHandler 告警处理器接口
type AlertHandler interface {
	Handle(alert Alert)
}

// LogAlertHandler 日志告警处理器
type LogAlertHandler struct{}

func (h *LogAlertHandler) Handle(alert Alert) {
	event := log.Info()
	switch alert.Level {
	case AlertLevelWarning:
		event = log.Warn()
	case AlertLevelError:
		event = log.Error()
	}

	event.
		Str("alert_id", alert.ID).
		Str("type", alert.Type).
		Str("level", string(alert.Level)).
		Float64("value", alert.Value).
		Float64("threshold", alert.Threshold).
		Msg(alert.Message)
}

// NewAlertManager 创建告警管理器
func NewAlertManager(maxAlerts int) *AlertManager {
	return &AlertManager{
		rules:     make([]*AlertRule, 0),
		alerts:    make([]Alert, 0),
		maxAlerts: maxAlerts,
		handlers:  []AlertHandler{&LogAlertHandler{}},
	}
}

// AddHandler 添加告警处理器
func (m *AlertManager) AddHandler(handler AlertHandler) {
	m.handlers = append(m.handlers, handler)
}

// AddRule 添加告警规则
func (m *AlertManager) AddRule(rule *AlertRule) {
	m.rules = append(m.rules, rule)
}

// Check 检查告警
func (m *AlertManager) Check(snapshot MetricsSnapshot) {
	for _, rule := range m.rules {
		triggered, value := rule.Condition(snapshot)

		if triggered {
			// 检查冷却时间
			if time.Since(rule.lastAlert) < rule.Cooldown {
				continue
			}
			rule.lastAlert = time.Now()

			alert := Alert{
				ID:        fmt.Sprintf("%s-%d", rule.Type, time.Now().UnixNano()),
				Level:     rule.Level,
				Type:      rule.Type,
				Message:   rule.Message,
				Value:     value,
				Threshold: rule.Threshold,
				Timestamp: time.Now(),
				Resolved:  false,
			}

			m.addAlert(alert)

			// 触发处理器
			for _, handler := range m.handlers {
				handler.Handle(alert)
			}
		}
	}
}

// addAlert 添加告警
func (m *AlertManager) addAlert(alert Alert) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alerts = append(m.alerts, alert)

	// 保持最大数量
	if len(m.alerts) > m.maxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.maxAlerts:]
	}
}

// GetAlerts 获取告警列表
func (m *AlertManager) GetAlerts(limit int) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	// 返回最新的告警
	result := make([]Alert, limit)
	start := len(m.alerts) - limit
	copy(result, m.alerts[start:])

	// 反转，最新在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetUnresolvedAlerts 获取未解决告警
func (m *AlertManager) GetUnresolvedAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Alert
	for _, alert := range m.alerts {
		if !alert.Resolved {
			result = append(result, alert)
		}
	}

	return result
}

// DefaultAlertRules 默认告警规则
func DefaultAlertRules() []*AlertRule {
	return []*AlertRule{
		{
			Name:      "高错误率",
			Type:      "high_error_rate",
			Level:     AlertLevelError,
			Threshold: 10, // 10%
			Message:   "错误率过高",
			Cooldown:  5 * time.Minute,
			Condition: func(s MetricsSnapshot) (bool, float64) {
				if s.TotalRequests == 0 {
					return false, 0
				}
				rate := float64(s.ErrorRequests) / float64(s.TotalRequests) * 100
				return rate > 10, rate
			},
		},
		{
			Name:      "高延迟",
			Type:      "high_latency",
			Level:     AlertLevelWarning,
			Threshold: 500, // 500ms
			Message:   "平均延迟过高",
			Cooldown:  5 * time.Minute,
			Condition: func(s MetricsSnapshot) (bool, float64) {
				return s.AvgLatencyMs > 500, s.AvgLatencyMs
			},
		},
		{
			Name:      "池命中率低",
			Type:      "low_pool_hit_rate",
			Level:     AlertLevelWarning,
			Threshold: 80, // 80%
			Message:   "对象池命中率过低",
			Cooldown:  10 * time.Minute,
			Condition: func(s MetricsSnapshot) (bool, float64) {
				return s.PoolHitRate < 80 && s.PoolHitRate > 0, s.PoolHitRate
			},
		},
		{
			Name:      "高内存使用",
			Type:      "high_memory",
			Level:     AlertLevelWarning,
			Threshold: 1024 * 1024 * 1024, // 1GB
			Message:   "堆内存使用过高",
			Cooldown:  10 * time.Minute,
			Condition: func(s MetricsSnapshot) (bool, float64) {
				return s.HeapAllocBytes > 1024*1024*1024, float64(s.HeapAllocBytes)
			},
		},
		{
			Name:      "Goroutine泄漏",
			Type:      "goroutine_leak",
			Level:     AlertLevelWarning,
			Threshold: 10000,
			Message:   "Goroutine数量过多",
			Cooldown:  10 * time.Minute,
			Condition: func(s MetricsSnapshot) (bool, float64) {
				return s.NumGoroutine > 10000, float64(s.NumGoroutine)
			},
		},
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/alerting.go
git commit -m "feat: add alerting system"
```

---

## Task 3: 实现监控服务

**Files:**
- Create: `go-page-server/core/monitor.go`

**Step 1: 创建监控服务**

```go
package core

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Monitor 监控服务
type Monitor struct {
	metrics      *Metrics
	alertManager *AlertManager

	// 历史数据
	history     []MetricsSnapshot
	historySize int
	mu          sync.RWMutex

	// 控制
	interval time.Duration
	stopChan chan struct{}
	running  bool
}

// NewMonitor 创建监控服务
func NewMonitor(interval time.Duration, historySize int) *Monitor {
	alertManager := NewAlertManager(1000)
	for _, rule := range DefaultAlertRules() {
		alertManager.AddRule(rule)
	}

	return &Monitor{
		metrics:      GetMetrics(),
		alertManager: alertManager,
		history:      make([]MetricsSnapshot, 0, historySize),
		historySize:  historySize,
		interval:     interval,
		stopChan:     make(chan struct{}),
	}
}

// Start 启动监控
func (m *Monitor) Start() {
	if m.running {
		return
	}
	m.running = true

	go m.collectLoop()

	log.Info().
		Dur("interval", m.interval).
		Msg("Monitor started")
}

// Stop 停止监控
func (m *Monitor) Stop() {
	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false

	log.Info().Msg("Monitor stopped")
}

// collectLoop 采集循环
func (m *Monitor) collectLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

// collect 采集指标
func (m *Monitor) collect() {
	snapshot := m.metrics.GetSnapshot()

	// 保存历史
	m.mu.Lock()
	m.history = append(m.history, snapshot)
	if len(m.history) > m.historySize {
		m.history = m.history[1:]
	}
	m.mu.Unlock()

	// 检查告警
	m.alertManager.Check(snapshot)

	// 重置时间窗口
	m.metrics.ResetWindow()
}

// GetCurrentSnapshot 获取当前快照
func (m *Monitor) GetCurrentSnapshot() MetricsSnapshot {
	return m.metrics.GetSnapshot()
}

// GetHistory 获取历史数据
func (m *Monitor) GetHistory(limit int) []MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	result := make([]MetricsSnapshot, limit)
	start := len(m.history) - limit
	copy(result, m.history[start:])

	return result
}

// GetAlerts 获取告警
func (m *Monitor) GetAlerts(limit int) []Alert {
	return m.alertManager.GetAlerts(limit)
}

// GetUnresolvedAlerts 获取未解决告警
func (m *Monitor) GetUnresolvedAlerts() []Alert {
	return m.alertManager.GetUnresolvedAlerts()
}

// AddAlertHandler 添加告警处理器
func (m *Monitor) AddAlertHandler(handler AlertHandler) {
	m.alertManager.AddHandler(handler)
}

// GetStats 获取监控统计
func (m *Monitor) GetStats() map[string]interface{} {
	m.mu.RLock()
	historyCount := len(m.history)
	m.mu.RUnlock()

	return map[string]interface{}{
		"running":          m.running,
		"interval_seconds": m.interval.Seconds(),
		"history_count":    historyCount,
		"history_size":     m.historySize,
		"unresolved_alerts": len(m.alertManager.GetUnresolvedAlerts()),
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/monitor.go
git commit -m "feat: add monitor service"
```

---

## Task 4: 添加监控 API

**Files:**
- Modify: `go-page-server/api/system_handler.go`

**Step 1: 扩展系统处理器**

在 `system_handler.go` 中添加监控相关方法：

```go
// 在 Dependencies 中添加 Monitor
type Dependencies struct {
	// ... existing fields
	Monitor *core.Monitor
}

// GetMetrics 获取实时指标
func (h *SystemHandler) GetMetrics(c *gin.Context) {
	snapshot := h.deps.Monitor.GetCurrentSnapshot()
	core.Success(c, snapshot)
}

// GetMetricsHistory 获取历史指标
func (h *SystemHandler) GetMetricsHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "60")
	limit, _ := strconv.Atoi(limitStr)

	history := h.deps.Monitor.GetHistory(limit)
	core.Success(c, history)
}

// GetAlerts 获取告警
func (h *SystemHandler) GetAlerts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	unresolvedOnly := c.Query("unresolved") == "true"

	var alerts []core.Alert
	if unresolvedOnly {
		alerts = h.deps.Monitor.GetUnresolvedAlerts()
	} else {
		alerts = h.deps.Monitor.GetAlerts(limit)
	}

	core.Success(c, alerts)
}

// GetMonitorStats 获取监控统计
func (h *SystemHandler) GetMonitorStats(c *gin.Context) {
	stats := h.deps.Monitor.GetStats()
	core.Success(c, stats)
}
```

**Step 2: 更新路由**

在 `router.go` 中添加：

```go
// 在 system 分组中添加
system.GET("/metrics", NewSystemHandler(deps).GetMetrics)
system.GET("/metrics/history", NewSystemHandler(deps).GetMetricsHistory)
system.GET("/alerts", NewSystemHandler(deps).GetAlerts)
system.GET("/monitor", NewSystemHandler(deps).GetMonitorStats)
```

**Step 3: Commit**

```bash
git add go-page-server/api/system_handler.go go-page-server/api/router.go
git commit -m "feat: add monitoring API endpoints"
```

---

## Task 5: 添加请求指标中间件

**Files:**
- Modify: `go-page-server/core/logger.go`

**Step 1: 在请求日志中间件中添加指标记录**

```go
// RequestLogger 请求日志中间件（添加指标记录）
func RequestLogger() func(c *gin.Context) {
	metrics := GetMetrics()

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// ... existing request ID logic ...

		c.Next()

		// 记录指标
		latency := time.Since(start)
		status := c.Writer.Status()
		success := status < 400

		metrics.RecordRequest(success, latency.Nanoseconds())

		// ... existing logging logic ...
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/logger.go
git commit -m "feat: add metrics recording in request middleware"
```

---

## Task 6: 添加测试

**Files:**
- Create: `go-page-server/core/monitor_test.go`

**Step 1: 创建测试文件**

```go
package core

import (
	"testing"
	"time"
)

func TestMetrics_RecordRequest(t *testing.T) {
	metrics := &Metrics{windowStart: time.Now()}

	metrics.RecordRequest(true, 100000000) // 100ms
	metrics.RecordRequest(true, 200000000) // 200ms
	metrics.RecordRequest(false, 50000000) // 50ms

	if metrics.TotalRequests.Load() != 3 {
		t.Errorf("Expected 3 total requests, got %d", metrics.TotalRequests.Load())
	}

	if metrics.SuccessRequests.Load() != 2 {
		t.Errorf("Expected 2 success requests, got %d", metrics.SuccessRequests.Load())
	}

	if metrics.ErrorRequests.Load() != 1 {
		t.Errorf("Expected 1 error request, got %d", metrics.ErrorRequests.Load())
	}
}

func TestMetrics_Snapshot(t *testing.T) {
	metrics := &Metrics{windowStart: time.Now()}

	metrics.RecordRequest(true, 100000000)
	metrics.RecordPoolAccess(true)
	metrics.RecordPoolAccess(false)
	metrics.RecordCacheAccess(true)

	snapshot := metrics.GetSnapshot()

	if snapshot.TotalRequests != 1 {
		t.Errorf("Expected 1 total request, got %d", snapshot.TotalRequests)
	}

	if snapshot.PoolHitRate != 50 {
		t.Errorf("Expected 50%% pool hit rate, got %.2f%%", snapshot.PoolHitRate)
	}

	if snapshot.CacheHitRate != 100 {
		t.Errorf("Expected 100%% cache hit rate, got %.2f%%", snapshot.CacheHitRate)
	}
}

func TestAlertManager_Check(t *testing.T) {
	manager := NewAlertManager(100)

	// 添加测试规则
	manager.AddRule(&AlertRule{
		Name:      "Test",
		Type:      "test",
		Level:     AlertLevelWarning,
		Threshold: 10,
		Message:   "Test alert",
		Cooldown:  0,
		Condition: func(s MetricsSnapshot) (bool, float64) {
			return s.ErrorRequests > 10, float64(s.ErrorRequests)
		},
	})

	// 不触发告警
	snapshot := MetricsSnapshot{ErrorRequests: 5}
	manager.Check(snapshot)

	alerts := manager.GetAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts, got %d", len(alerts))
	}

	// 触发告警
	snapshot.ErrorRequests = 15
	manager.Check(snapshot)

	alerts = manager.GetAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}

	if alerts[0].Type != "test" {
		t.Errorf("Expected type 'test', got '%s'", alerts[0].Type)
	}
}

func TestMonitor_History(t *testing.T) {
	monitor := NewMonitor(time.Second, 10)

	// 手动添加历史
	for i := 0; i < 15; i++ {
		monitor.collect()
	}

	// 应该只保留最后 10 条
	history := monitor.GetHistory(0)
	if len(history) != 10 {
		t.Errorf("Expected 10 history entries, got %d", len(history))
	}

	// 获取最后 5 条
	history = monitor.GetHistory(5)
	if len(history) != 5 {
		t.Errorf("Expected 5 history entries, got %d", len(history))
	}
}
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestMetrics
cd go-page-server && go test -v ./core/... -run TestAlert
cd go-page-server && go test -v ./core/... -run TestMonitor
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/monitor_test.go
git commit -m "test: add monitoring tests"
```

---

## Task 7: 更新 main.go

**Files:**
- Modify: `go-page-server/main.go`

**Step 1: 添加监控服务初始化**

```go
// 初始化监控服务
monitor := core.NewMonitor(10*time.Second, 360) // 10秒采集一次，保留1小时历史
monitor.Start()
defer monitor.Stop()

// 更新 API 依赖
deps := &api.Dependencies{
	// ... existing fields
	Monitor: monitor,
}
```

**Step 2: Commit**

```bash
git add go-page-server/main.go
git commit -m "feat: initialize monitor in main"
```

---

## 监控 API 文档

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/admin/system/metrics | 获取实时指标 |
| GET | /api/admin/system/metrics/history?limit=60 | 获取历史指标 |
| GET | /api/admin/system/alerts?limit=50 | 获取告警列表 |
| GET | /api/admin/system/alerts?unresolved=true | 获取未解决告警 |
| GET | /api/admin/system/monitor | 获取监控统计 |

---

## 完成检查清单

- [ ] Task 1: 指标收集
- [ ] Task 2: 告警系统
- [ ] Task 3: 监控服务
- [ ] Task 4: 监控 API
- [ ] Task 5: 请求指标中间件
- [ ] Task 6: 测试覆盖
- [ ] Task 7: main.go 集成

所有任务完成后运行完整测试：

```bash
cd go-page-server && go test -v ./...
```