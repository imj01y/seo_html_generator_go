// Package core provides metrics collection for system monitoring
package core

import (
	"runtime"
	"sync/atomic"
	"time"
)

// Metrics 系统指标收集器（使用 atomic 计数器确保并发安全）
type Metrics struct {
	// 请求指标
	TotalRequests   int64 // 总请求数
	SuccessRequests int64 // 成功请求数
	ErrorRequests   int64 // 错误请求数
	TotalLatencyNs  int64 // 总延迟（纳秒）
	MaxLatencyNs    int64 // 最大延迟（纳秒）

	// 渲染指标
	TotalRenders      int64 // 总渲染次数
	RenderErrorCount  int64 // 渲染错误次数
	TotalRenderTimeNs int64 // 总渲染时间（纳秒）

	// 池指标
	PoolHits    int64 // 池命中次数
	PoolMisses  int64 // 池未命中次数
	PoolRefills int64 // 池补充次数

	// 缓存指标
	CacheHits   int64 // 缓存命中次数
	CacheMisses int64 // 缓存未命中次数

	// 爬虫检测指标
	SpiderRequests int64 // 爬虫请求数
	NormalRequests int64 // 普通请求数

	// 时间窗口指标（用于计算 QPS）
	windowStart     time.Time // 窗口开始时间
	windowRequests  int64     // 窗口内请求数
	windowLatencyNs int64     // 窗口内总延迟
}

// 全局指标实例
var globalMetrics = &Metrics{windowStart: time.Now()}

// GetMetrics 获取全局指标实例
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordRequest 记录请求指标
// success: 请求是否成功
// latencyNs: 请求延迟（纳秒）
func (m *Metrics) RecordRequest(success bool, latencyNs int64) {
	atomic.AddInt64(&m.TotalRequests, 1)
	atomic.AddInt64(&m.TotalLatencyNs, latencyNs)
	atomic.AddInt64(&m.windowRequests, 1)
	atomic.AddInt64(&m.windowLatencyNs, latencyNs)

	if success {
		atomic.AddInt64(&m.SuccessRequests, 1)
	} else {
		atomic.AddInt64(&m.ErrorRequests, 1)
	}

	// 使用 CAS 更新最大延迟
	for {
		currentMax := atomic.LoadInt64(&m.MaxLatencyNs)
		if latencyNs <= currentMax {
			break
		}
		if atomic.CompareAndSwapInt64(&m.MaxLatencyNs, currentMax, latencyNs) {
			break
		}
	}
}

// RecordRender 记录渲染指标
// success: 渲染是否成功
// durationNs: 渲染耗时（纳秒）
func (m *Metrics) RecordRender(success bool, durationNs int64) {
	atomic.AddInt64(&m.TotalRenders, 1)
	atomic.AddInt64(&m.TotalRenderTimeNs, durationNs)

	if !success {
		atomic.AddInt64(&m.RenderErrorCount, 1)
	}
}

// RecordPoolAccess 记录池访问指标
// hit: 是否命中
func (m *Metrics) RecordPoolAccess(hit bool) {
	if hit {
		atomic.AddInt64(&m.PoolHits, 1)
	} else {
		atomic.AddInt64(&m.PoolMisses, 1)
	}
}

// RecordPoolRefill 记录池补充
func (m *Metrics) RecordPoolRefill() {
	atomic.AddInt64(&m.PoolRefills, 1)
}

// RecordCacheAccess 记录缓存访问指标
// hit: 是否命中
func (m *Metrics) RecordCacheAccess(hit bool) {
	if hit {
		atomic.AddInt64(&m.CacheHits, 1)
	} else {
		atomic.AddInt64(&m.CacheMisses, 1)
	}
}

// RecordSpiderDetection 记录爬虫检测指标
// isSpider: 是否为爬虫
func (m *Metrics) RecordSpiderDetection(isSpider bool) {
	if isSpider {
		atomic.AddInt64(&m.SpiderRequests, 1)
	} else {
		atomic.AddInt64(&m.NormalRequests, 1)
	}
}

// MetricsSnapshot 指标快照（用于 JSON 序列化）
type MetricsSnapshot struct {
	// 请求指标
	TotalRequests   int64 `json:"total_requests"`
	SuccessRequests int64 `json:"success_requests"`
	ErrorRequests   int64 `json:"error_requests"`
	TotalLatencyNs  int64 `json:"total_latency_ns"`
	MaxLatencyNs    int64 `json:"max_latency_ns"`

	// 渲染指标
	TotalRenders      int64 `json:"total_renders"`
	RenderErrorCount  int64 `json:"render_error_count"`
	TotalRenderTimeNs int64 `json:"total_render_time_ns"`

	// 池指标
	PoolHits    int64 `json:"pool_hits"`
	PoolMisses  int64 `json:"pool_misses"`
	PoolRefills int64 `json:"pool_refills"`

	// 缓存指标
	CacheHits   int64 `json:"cache_hits"`
	CacheMisses int64 `json:"cache_misses"`

	// 爬虫检测指标
	SpiderRequests int64 `json:"spider_requests"`
	NormalRequests int64 `json:"normal_requests"`

	// 派生值
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	MaxLatencyMs float64 `json:"max_latency_ms"`
	QPS          float64 `json:"qps"`
	PoolHitRate  float64 `json:"pool_hit_rate"`
	CacheHitRate float64 `json:"cache_hit_rate"`

	// 系统指标
	NumGoroutine   int    `json:"num_goroutine"`
	HeapAllocBytes uint64 `json:"heap_alloc_bytes"`
	HeapSysBytes   uint64 `json:"heap_sys_bytes"`
	GCCycles       uint32 `json:"gc_cycles"`

	// 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// GetSnapshot 获取指标快照
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	// 读取所有原子值
	totalRequests := atomic.LoadInt64(&m.TotalRequests)
	successRequests := atomic.LoadInt64(&m.SuccessRequests)
	errorRequests := atomic.LoadInt64(&m.ErrorRequests)
	totalLatencyNs := atomic.LoadInt64(&m.TotalLatencyNs)
	maxLatencyNs := atomic.LoadInt64(&m.MaxLatencyNs)

	totalRenders := atomic.LoadInt64(&m.TotalRenders)
	renderErrorCount := atomic.LoadInt64(&m.RenderErrorCount)
	totalRenderTimeNs := atomic.LoadInt64(&m.TotalRenderTimeNs)

	poolHits := atomic.LoadInt64(&m.PoolHits)
	poolMisses := atomic.LoadInt64(&m.PoolMisses)
	poolRefills := atomic.LoadInt64(&m.PoolRefills)

	cacheHits := atomic.LoadInt64(&m.CacheHits)
	cacheMisses := atomic.LoadInt64(&m.CacheMisses)

	spiderRequests := atomic.LoadInt64(&m.SpiderRequests)
	normalRequests := atomic.LoadInt64(&m.NormalRequests)

	windowRequests := atomic.LoadInt64(&m.windowRequests)

	// 计算派生值
	var avgLatencyMs float64
	if totalRequests > 0 {
		avgLatencyMs = float64(totalLatencyNs) / float64(totalRequests) / 1e6
	}

	maxLatencyMs := float64(maxLatencyNs) / 1e6

	// 计算 QPS（基于时间窗口）
	var qps float64
	windowDuration := time.Since(m.windowStart).Seconds()
	if windowDuration > 0 {
		qps = float64(windowRequests) / windowDuration
	}

	// 计算池命中率
	var poolHitRate float64
	poolTotal := poolHits + poolMisses
	if poolTotal > 0 {
		poolHitRate = float64(poolHits) / float64(poolTotal) * 100
	}

	// 计算缓存命中率
	var cacheHitRate float64
	cacheTotal := cacheHits + cacheMisses
	if cacheTotal > 0 {
		cacheHitRate = float64(cacheHits) / float64(cacheTotal) * 100
	}

	// 获取运行时统计
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return MetricsSnapshot{
		// 请求指标
		TotalRequests:   totalRequests,
		SuccessRequests: successRequests,
		ErrorRequests:   errorRequests,
		TotalLatencyNs:  totalLatencyNs,
		MaxLatencyNs:    maxLatencyNs,

		// 渲染指标
		TotalRenders:      totalRenders,
		RenderErrorCount:  renderErrorCount,
		TotalRenderTimeNs: totalRenderTimeNs,

		// 池指标
		PoolHits:    poolHits,
		PoolMisses:  poolMisses,
		PoolRefills: poolRefills,

		// 缓存指标
		CacheHits:   cacheHits,
		CacheMisses: cacheMisses,

		// 爬虫检测指标
		SpiderRequests: spiderRequests,
		NormalRequests: normalRequests,

		// 派生值
		AvgLatencyMs: avgLatencyMs,
		MaxLatencyMs: maxLatencyMs,
		QPS:          qps,
		PoolHitRate:  poolHitRate,
		CacheHitRate: cacheHitRate,

		// 系统指标
		NumGoroutine:   runtime.NumGoroutine(),
		HeapAllocBytes: memStats.HeapAlloc,
		HeapSysBytes:   memStats.HeapSys,
		GCCycles:       memStats.NumGC,

		// 时间戳
		Timestamp: time.Now(),
	}
}

// ResetWindow 重置时间窗口
func (m *Metrics) ResetWindow() {
	m.windowStart = time.Now()
	atomic.StoreInt64(&m.windowRequests, 0)
	atomic.StoreInt64(&m.windowLatencyNs, 0)
}

// Reset 重置所有指标
func (m *Metrics) Reset() {
	atomic.StoreInt64(&m.TotalRequests, 0)
	atomic.StoreInt64(&m.SuccessRequests, 0)
	atomic.StoreInt64(&m.ErrorRequests, 0)
	atomic.StoreInt64(&m.TotalLatencyNs, 0)
	atomic.StoreInt64(&m.MaxLatencyNs, 0)

	atomic.StoreInt64(&m.TotalRenders, 0)
	atomic.StoreInt64(&m.RenderErrorCount, 0)
	atomic.StoreInt64(&m.TotalRenderTimeNs, 0)

	atomic.StoreInt64(&m.PoolHits, 0)
	atomic.StoreInt64(&m.PoolMisses, 0)
	atomic.StoreInt64(&m.PoolRefills, 0)

	atomic.StoreInt64(&m.CacheHits, 0)
	atomic.StoreInt64(&m.CacheMisses, 0)

	atomic.StoreInt64(&m.SpiderRequests, 0)
	atomic.StoreInt64(&m.NormalRequests, 0)

	m.ResetWindow()
}
