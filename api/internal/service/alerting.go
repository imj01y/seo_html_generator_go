// Package core provides alerting system for monitoring and notifications
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

// Alert 告警记录
type Alert struct {
	ID        string     `json:"id"`         // 告警唯一标识
	Level     AlertLevel `json:"level"`      // 告警级别
	Type      string     `json:"type"`       // 告警类型
	Message   string     `json:"message"`    // 告警消息
	Value     float64    `json:"value"`      // 当前值
	Threshold float64    `json:"threshold"`  // 阈值
	Timestamp time.Time  `json:"timestamp"`  // 告警时间
	Resolved  bool       `json:"resolved"`   // 是否已解决
}

// AlertRule 告警规则
type AlertRule struct {
	Name      string                                  // 规则名称
	Type      string                                  // 规则类型
	Condition func(MetricsSnapshot) (bool, float64)  // 条件函数，返回是否触发和当前值
	Threshold float64                                 // 阈值
	Level     AlertLevel                              // 告警级别
	Message   string                                  // 告警消息模板
	Cooldown  time.Duration                           // 冷却时间
	lastAlert time.Time                               // 上次告警时间
}

// AlertHandler 告警处理器接口
type AlertHandler interface {
	Handle(alert Alert)
}

// LogAlertHandler 日志告警处理器（使用 zerolog）
type LogAlertHandler struct{}

// NewLogAlertHandler 创建日志告警处理器
func NewLogAlertHandler() *LogAlertHandler {
	return &LogAlertHandler{}
}

// Handle 处理告警（写入日志）
func (h *LogAlertHandler) Handle(alert Alert) {
	logEvent := log.With().
		Str("alert_id", alert.ID).
		Str("type", alert.Type).
		Float64("value", alert.Value).
		Float64("threshold", alert.Threshold).
		Time("timestamp", alert.Timestamp).
		Bool("resolved", alert.Resolved).
		Logger()

	switch alert.Level {
	case AlertLevelError:
		logEvent.Error().Msg(alert.Message)
	case AlertLevelWarning:
		logEvent.Warn().Msg(alert.Message)
	default:
		logEvent.Info().Msg(alert.Message)
	}
}

// AlertManager 告警管理器
type AlertManager struct {
	mu        sync.RWMutex
	rules     []*AlertRule
	alerts    []Alert
	maxAlerts int
	handlers  []AlertHandler
	alertSeq  int64 // 告警序号，用于生成唯一ID
}

// NewAlertManager 创建告警管理器
func NewAlertManager(maxAlerts int) *AlertManager {
	if maxAlerts <= 0 {
		maxAlerts = 1000
	}
	return &AlertManager{
		rules:     make([]*AlertRule, 0),
		alerts:    make([]Alert, 0),
		maxAlerts: maxAlerts,
		handlers:  make([]AlertHandler, 0),
	}
}

// AddHandler 添加告警处理器
func (m *AlertManager) AddHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
}

// AddRule 添加告警规则
func (m *AlertManager) AddRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules = append(m.rules, rule)
}

// Check 检查所有规则，触发告警
func (m *AlertManager) Check(snapshot MetricsSnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for _, rule := range m.rules {
		// 检查冷却时间
		if rule.Cooldown > 0 && now.Sub(rule.lastAlert) < rule.Cooldown {
			continue
		}

		// 执行条件检查
		triggered, value := rule.Condition(snapshot)
		if !triggered {
			// 检查是否有未解决的同类型告警需要标记为已解决
			m.resolveAlertsByType(rule.Type)
			continue
		}

		// 生成告警ID
		m.alertSeq++
		alertID := fmt.Sprintf("alert-%d-%d", now.UnixNano(), m.alertSeq)

		// 创建告警
		alert := Alert{
			ID:        alertID,
			Level:     rule.Level,
			Type:      rule.Type,
			Message:   fmt.Sprintf("%s: 当前值 %.2f, 阈值 %.2f", rule.Message, value, rule.Threshold),
			Value:     value,
			Threshold: rule.Threshold,
			Timestamp: now,
			Resolved:  false,
		}

		// 添加告警
		m.alerts = append(m.alerts, alert)

		// 保持告警数量在限制内
		if len(m.alerts) > m.maxAlerts {
			m.alerts = m.alerts[len(m.alerts)-m.maxAlerts:]
		}

		// 更新上次告警时间
		rule.lastAlert = now

		// 通知所有处理器
		for _, handler := range m.handlers {
			handler.Handle(alert)
		}
	}
}

// resolveAlertsByType 将指定类型的未解决告警标记为已解决
func (m *AlertManager) resolveAlertsByType(alertType string) {
	for i := range m.alerts {
		if m.alerts[i].Type == alertType && !m.alerts[i].Resolved {
			m.alerts[i].Resolved = true
		}
	}
}

// GetAlerts 获取最近告警
func (m *AlertManager) GetAlerts(limit int) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}

	// 返回最近的告警（从末尾开始）
	result := make([]Alert, limit)
	start := len(m.alerts) - limit
	copy(result, m.alerts[start:])

	// 反转顺序，最新的在前
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// GetUnresolvedAlerts 获取未解决告警
func (m *AlertManager) GetUnresolvedAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Alert, 0)
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
		// 高错误率 (>10%)
		{
			Name: "high_error_rate",
			Type: "error_rate",
			Condition: func(s MetricsSnapshot) (bool, float64) {
				if s.TotalRequests == 0 {
					return false, 0
				}
				errorRate := float64(s.ErrorRequests) / float64(s.TotalRequests) * 100
				return errorRate > 10, errorRate
			},
			Threshold: 10,
			Level:     AlertLevelError,
			Message:   "高错误率",
			Cooldown:  5 * time.Minute,
		},
		// 高延迟 (>500ms)
		{
			Name: "high_latency",
			Type: "latency",
			Condition: func(s MetricsSnapshot) (bool, float64) {
				return s.AvgLatencyMs > 500, s.AvgLatencyMs
			},
			Threshold: 500,
			Level:     AlertLevelWarning,
			Message:   "高延迟",
			Cooldown:  5 * time.Minute,
		},
		// 池命中率低 (<80%)
		{
			Name: "low_pool_hit_rate",
			Type: "pool_hit_rate",
			Condition: func(s MetricsSnapshot) (bool, float64) {
				// 只有在有足够的池访问时才检查
				totalPoolAccess := s.PoolHits + s.PoolMisses
				if totalPoolAccess < 100 {
					return false, s.PoolHitRate
				}
				return s.PoolHitRate < 80, s.PoolHitRate
			},
			Threshold: 80,
			Level:     AlertLevelWarning,
			Message:   "池命中率低",
			Cooldown:  10 * time.Minute,
		},
		// 高内存使用 (>1GB)
		{
			Name: "high_memory_usage",
			Type: "memory",
			Condition: func(s MetricsSnapshot) (bool, float64) {
				memoryMB := float64(s.HeapAllocBytes) / (1024 * 1024)
				return memoryMB > 1024, memoryMB
			},
			Threshold: 1024,
			Level:     AlertLevelError,
			Message:   "高内存使用(MB)",
			Cooldown:  5 * time.Minute,
		},
		// Goroutine泄漏 (>10000)
		{
			Name: "goroutine_leak",
			Type: "goroutine",
			Condition: func(s MetricsSnapshot) (bool, float64) {
				return s.NumGoroutine > 10000, float64(s.NumGoroutine)
			},
			Threshold: 10000,
			Level:     AlertLevelError,
			Message:   "可能存在Goroutine泄漏",
			Cooldown:  5 * time.Minute,
		},
	}
}
