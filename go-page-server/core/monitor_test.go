// Package core provides tests for monitoring, metrics, and alerting
package core

import (
	"testing"
	"time"
)

// TestMetrics_RecordRequest 测试请求记录
func TestMetrics_RecordRequest(t *testing.T) {
	// 创建新的 Metrics 实例用于测试
	m := &Metrics{windowStart: time.Now()}

	// 记录成功请求
	m.RecordRequest(true, 100*1e6)  // 100ms
	m.RecordRequest(true, 150*1e6)  // 150ms
	m.RecordRequest(true, 200*1e6)  // 200ms

	// 记录失败请求
	m.RecordRequest(false, 50*1e6)   // 50ms
	m.RecordRequest(false, 300*1e6)  // 300ms

	// 验证总请求数
	if m.TotalRequests != 5 {
		t.Errorf("TotalRequests: expected 5, got %d", m.TotalRequests)
	}

	// 验证成功请求数
	if m.SuccessRequests != 3 {
		t.Errorf("SuccessRequests: expected 3, got %d", m.SuccessRequests)
	}

	// 验证错误请求数
	if m.ErrorRequests != 2 {
		t.Errorf("ErrorRequests: expected 2, got %d", m.ErrorRequests)
	}

	// 验证最大延迟
	expectedMaxLatency := int64(300 * 1e6)
	if m.MaxLatencyNs != expectedMaxLatency {
		t.Errorf("MaxLatencyNs: expected %d, got %d", expectedMaxLatency, m.MaxLatencyNs)
	}

	// 验证总延迟
	expectedTotalLatency := int64((100 + 150 + 200 + 50 + 300) * 1e6)
	if m.TotalLatencyNs != expectedTotalLatency {
		t.Errorf("TotalLatencyNs: expected %d, got %d", expectedTotalLatency, m.TotalLatencyNs)
	}
}

// TestMetrics_Snapshot 测试快照
func TestMetrics_Snapshot(t *testing.T) {
	// 创建新的 Metrics 实例用于测试
	m := &Metrics{windowStart: time.Now()}

	// 记录请求
	m.RecordRequest(true, 100*1e6)  // 100ms - 成功
	m.RecordRequest(true, 200*1e6)  // 200ms - 成功
	m.RecordRequest(false, 50*1e6)  // 50ms - 失败

	// 记录池访问
	m.RecordPoolAccess(true)  // 命中
	m.RecordPoolAccess(true)  // 命中
	m.RecordPoolAccess(true)  // 命中
	m.RecordPoolAccess(false) // 未命中

	// 记录缓存访问
	m.RecordCacheAccess(true)  // 命中
	m.RecordCacheAccess(false) // 未命中
	m.RecordCacheAccess(false) // 未命中

	// 获取快照
	snapshot := m.GetSnapshot()

	// 验证请求指标
	if snapshot.TotalRequests != 3 {
		t.Errorf("Snapshot.TotalRequests: expected 3, got %d", snapshot.TotalRequests)
	}
	if snapshot.SuccessRequests != 2 {
		t.Errorf("Snapshot.SuccessRequests: expected 2, got %d", snapshot.SuccessRequests)
	}
	if snapshot.ErrorRequests != 1 {
		t.Errorf("Snapshot.ErrorRequests: expected 1, got %d", snapshot.ErrorRequests)
	}

	// 验证池指标
	if snapshot.PoolHits != 3 {
		t.Errorf("Snapshot.PoolHits: expected 3, got %d", snapshot.PoolHits)
	}
	if snapshot.PoolMisses != 1 {
		t.Errorf("Snapshot.PoolMisses: expected 1, got %d", snapshot.PoolMisses)
	}

	// 验证池命中率计算 (3/(3+1) * 100 = 75%)
	expectedPoolHitRate := 75.0
	if snapshot.PoolHitRate != expectedPoolHitRate {
		t.Errorf("Snapshot.PoolHitRate: expected %.2f, got %.2f", expectedPoolHitRate, snapshot.PoolHitRate)
	}

	// 验证缓存指标
	if snapshot.CacheHits != 1 {
		t.Errorf("Snapshot.CacheHits: expected 1, got %d", snapshot.CacheHits)
	}
	if snapshot.CacheMisses != 2 {
		t.Errorf("Snapshot.CacheMisses: expected 2, got %d", snapshot.CacheMisses)
	}

	// 验证缓存命中率计算 (1/(1+2) * 100 = 33.33%)
	expectedCacheHitRate := 100.0 / 3.0 // 约 33.33%
	if snapshot.CacheHitRate < 33.3 || snapshot.CacheHitRate > 33.4 {
		t.Errorf("Snapshot.CacheHitRate: expected ~%.2f, got %.2f", expectedCacheHitRate, snapshot.CacheHitRate)
	}

	// 验证时间戳存在
	if snapshot.Timestamp.IsZero() {
		t.Error("Snapshot.Timestamp should not be zero")
	}
}

// TestAlertManager_Check 测试告警检查
func TestAlertManager_Check(t *testing.T) {
	// 创建告警管理器
	am := NewAlertManager(100)

	// 添加测试规则：当 ErrorRequests > 10 时触发
	testRule := &AlertRule{
		Name: "test_error_rule",
		Type: "test_error",
		Condition: func(s MetricsSnapshot) (bool, float64) {
			return s.ErrorRequests > 10, float64(s.ErrorRequests)
		},
		Threshold: 10,
		Level:     AlertLevelError,
		Message:   "错误请求过多",
		Cooldown:  0, // 无冷却时间，方便测试
	}
	am.AddRule(testRule)

	// 测试不触发告警的情况
	snapshotNotTriggered := MetricsSnapshot{
		ErrorRequests: 5, // 小于阈值
	}
	am.Check(snapshotNotTriggered)

	alerts := am.GetAlerts(10)
	if len(alerts) != 0 {
		t.Errorf("Expected 0 alerts when below threshold, got %d", len(alerts))
	}

	// 测试触发告警的情况
	snapshotTriggered := MetricsSnapshot{
		ErrorRequests: 15, // 大于阈值
	}
	am.Check(snapshotTriggered)

	alerts = am.GetAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert when above threshold, got %d", len(alerts))
	}

	if len(alerts) > 0 {
		alert := alerts[0]
		if alert.Type != "test_error" {
			t.Errorf("Alert type: expected 'test_error', got '%s'", alert.Type)
		}
		if alert.Level != AlertLevelError {
			t.Errorf("Alert level: expected '%s', got '%s'", AlertLevelError, alert.Level)
		}
		if alert.Value != 15 {
			t.Errorf("Alert value: expected 15, got %.2f", alert.Value)
		}
		if alert.Threshold != 10 {
			t.Errorf("Alert threshold: expected 10, got %.2f", alert.Threshold)
		}
		if alert.Resolved {
			t.Error("Alert should not be resolved")
		}
	}

	// 再次触发告警（因为没有冷却时间）
	am.Check(snapshotTriggered)
	alerts = am.GetAlerts(10)
	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts after second trigger, got %d", len(alerts))
	}
}

// TestAlertManager_Cooldown 测试告警冷却时间
func TestAlertManager_Cooldown(t *testing.T) {
	am := NewAlertManager(100)

	// 添加带冷却时间的规则
	testRule := &AlertRule{
		Name: "test_cooldown_rule",
		Type: "test_cooldown",
		Condition: func(s MetricsSnapshot) (bool, float64) {
			return s.ErrorRequests > 10, float64(s.ErrorRequests)
		},
		Threshold: 10,
		Level:     AlertLevelWarning,
		Message:   "测试冷却时间",
		Cooldown:  1 * time.Hour, // 1小时冷却
	}
	am.AddRule(testRule)

	snapshot := MetricsSnapshot{
		ErrorRequests: 15,
	}

	// 第一次触发
	am.Check(snapshot)
	alerts := am.GetAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert on first trigger, got %d", len(alerts))
	}

	// 第二次应该被冷却时间阻止
	am.Check(snapshot)
	alerts = am.GetAlerts(10)
	if len(alerts) != 1 {
		t.Errorf("Expected still 1 alert due to cooldown, got %d", len(alerts))
	}
}

// TestMonitor_History 测试历史数据
func TestMonitor_History(t *testing.T) {
	// 创建 Monitor，historySize=10
	monitor := NewMonitor(1*time.Second, 10)

	// 重置全局指标以获得干净的测试环境
	monitor.metrics.Reset()

	// 手动调用 collect() 多次
	for i := 0; i < 15; i++ {
		// 记录一些请求以便每次快照有不同的数据
		monitor.metrics.RecordRequest(true, int64(i)*1e6)
		monitor.collect()
	}

	// 获取历史数据
	history := monitor.GetHistory(0) // 获取全部

	// 验证历史数据保持最大数量
	if len(history) != 10 {
		t.Errorf("History size: expected 10, got %d", len(history))
	}

	// 验证数据是按最新到最旧排序的
	for i := 0; i < len(history)-1; i++ {
		if history[i].Timestamp.Before(history[i+1].Timestamp) {
			t.Error("History should be sorted with newest first")
			break
		}
	}

	// 测试限制数量
	limitedHistory := monitor.GetHistory(5)
	if len(limitedHistory) != 5 {
		t.Errorf("Limited history size: expected 5, got %d", len(limitedHistory))
	}
}

// TestMonitor_StartStop 测试监控服务启动和停止
func TestMonitor_StartStop(t *testing.T) {
	monitor := NewMonitor(100*time.Millisecond, 10)

	// 验证初始状态
	if monitor.IsRunning() {
		t.Error("Monitor should not be running initially")
	}

	// 启动监控
	monitor.Start()
	if !monitor.IsRunning() {
		t.Error("Monitor should be running after Start()")
	}

	// 等待一小段时间让采集运行
	time.Sleep(250 * time.Millisecond)

	// 停止监控
	monitor.Stop()

	// 等待停止完成
	time.Sleep(50 * time.Millisecond)

	if monitor.IsRunning() {
		t.Error("Monitor should not be running after Stop()")
	}
}

// TestMetrics_Reset 测试指标重置
func TestMetrics_Reset(t *testing.T) {
	m := &Metrics{windowStart: time.Now()}

	// 记录一些数据
	m.RecordRequest(true, 100*1e6)
	m.RecordRequest(false, 50*1e6)
	m.RecordPoolAccess(true)
	m.RecordCacheAccess(false)
	m.RecordRender(true, 10*1e6)

	// 验证数据已记录
	if m.TotalRequests == 0 {
		t.Error("TotalRequests should not be 0 before reset")
	}

	// 重置
	m.Reset()

	// 验证所有指标已重置
	if m.TotalRequests != 0 {
		t.Errorf("TotalRequests should be 0 after reset, got %d", m.TotalRequests)
	}
	if m.SuccessRequests != 0 {
		t.Errorf("SuccessRequests should be 0 after reset, got %d", m.SuccessRequests)
	}
	if m.ErrorRequests != 0 {
		t.Errorf("ErrorRequests should be 0 after reset, got %d", m.ErrorRequests)
	}
	if m.PoolHits != 0 {
		t.Errorf("PoolHits should be 0 after reset, got %d", m.PoolHits)
	}
	if m.CacheMisses != 0 {
		t.Errorf("CacheMisses should be 0 after reset, got %d", m.CacheMisses)
	}
	if m.TotalRenders != 0 {
		t.Errorf("TotalRenders should be 0 after reset, got %d", m.TotalRenders)
	}
}

// TestAlertManager_GetUnresolvedAlerts 测试获取未解决告警
func TestAlertManager_GetUnresolvedAlerts(t *testing.T) {
	am := NewAlertManager(100)

	// 添加测试规则
	testRule := &AlertRule{
		Name: "test_unresolved",
		Type: "test_unresolved",
		Condition: func(s MetricsSnapshot) (bool, float64) {
			return s.ErrorRequests > 10, float64(s.ErrorRequests)
		},
		Threshold: 10,
		Level:     AlertLevelError,
		Message:   "测试未解决告警",
		Cooldown:  0,
	}
	am.AddRule(testRule)

	// 触发告警
	snapshotTriggered := MetricsSnapshot{ErrorRequests: 15}
	am.Check(snapshotTriggered)

	// 验证有未解决告警
	unresolvedAlerts := am.GetUnresolvedAlerts()
	if len(unresolvedAlerts) != 1 {
		t.Errorf("Expected 1 unresolved alert, got %d", len(unresolvedAlerts))
	}

	// 传入不触发的快照，应该解决告警
	snapshotNotTriggered := MetricsSnapshot{ErrorRequests: 5}
	am.Check(snapshotNotTriggered)

	// 验证告警已解决
	unresolvedAlerts = am.GetUnresolvedAlerts()
	if len(unresolvedAlerts) != 0 {
		t.Errorf("Expected 0 unresolved alerts after resolution, got %d", len(unresolvedAlerts))
	}
}

// TestMetrics_RecordRender 测试渲染记录
func TestMetrics_RecordRender(t *testing.T) {
	m := &Metrics{windowStart: time.Now()}

	// 记录成功渲染
	m.RecordRender(true, 50*1e6)
	m.RecordRender(true, 100*1e6)

	// 记录失败渲染
	m.RecordRender(false, 200*1e6)

	// 验证总渲染次数
	if m.TotalRenders != 3 {
		t.Errorf("TotalRenders: expected 3, got %d", m.TotalRenders)
	}

	// 验证渲染错误次数
	if m.RenderErrorCount != 1 {
		t.Errorf("RenderErrorCount: expected 1, got %d", m.RenderErrorCount)
	}

	// 验证总渲染时间
	expectedRenderTime := int64((50 + 100 + 200) * 1e6)
	if m.TotalRenderTimeNs != expectedRenderTime {
		t.Errorf("TotalRenderTimeNs: expected %d, got %d", expectedRenderTime, m.TotalRenderTimeNs)
	}
}

// TestMetrics_SpiderDetection 测试爬虫检测记录
func TestMetrics_SpiderDetection(t *testing.T) {
	m := &Metrics{windowStart: time.Now()}

	// 记录爬虫请求
	m.RecordSpiderDetection(true)
	m.RecordSpiderDetection(true)

	// 记录普通请求
	m.RecordSpiderDetection(false)
	m.RecordSpiderDetection(false)
	m.RecordSpiderDetection(false)

	// 验证爬虫请求数
	if m.SpiderRequests != 2 {
		t.Errorf("SpiderRequests: expected 2, got %d", m.SpiderRequests)
	}

	// 验证普通请求数
	if m.NormalRequests != 3 {
		t.Errorf("NormalRequests: expected 3, got %d", m.NormalRequests)
	}
}
