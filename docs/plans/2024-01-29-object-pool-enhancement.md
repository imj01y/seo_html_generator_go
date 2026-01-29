# 对象池增强实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 基于模板分析结果自动计算和动态调整对象池大小，支持预设配置方案和管理操作。

**Architecture:** 环形缓冲区 + 后台补充 goroutine + 动态扩缩容 + 预设配置

**Tech Stack:** Go sync/atomic, sync.Pool

**依赖:** 阶段2（模板分析器）

---

## Task 1: 增强对象池结构

**Files:**
- Modify: `go-page-server/core/object_pool.go`

**Step 1: 添加动态扩缩容支持**

```go
package core

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ObjectPool 高性能对象池
type ObjectPool struct {
	name       string
	items      []string
	capacity   int64
	head       atomic.Int64 // 读取位置
	tail       atomic.Int64 // 写入位置
	count      atomic.Int64 // 当前数量

	lowMark    int64             // 低水位
	generator  func() string     // 生成器
	refillChan chan struct{}     // 补充信号
	stopChan   chan struct{}     // 停止信号
	running    atomic.Bool       // 是否运行中
	paused     atomic.Bool       // 是否暂停补充

	// 统计
	totalGenerated atomic.Int64
	totalConsumed  atomic.Int64
	refillCount    atomic.Int64

	mu sync.RWMutex
}

// NewObjectPool 创建对象池
func NewObjectPool(name string, capacity int, lowMarkPercent float64, generator func() string) *ObjectPool {
	if capacity <= 0 {
		capacity = 10000
	}
	if lowMarkPercent <= 0 || lowMarkPercent >= 1 {
		lowMarkPercent = 0.2
	}

	pool := &ObjectPool{
		name:       name,
		items:      make([]string, capacity),
		capacity:   int64(capacity),
		lowMark:    int64(float64(capacity) * lowMarkPercent),
		generator:  generator,
		refillChan: make(chan struct{}, 1),
		stopChan:   make(chan struct{}),
	}

	return pool
}

// Start 启动后台补充
func (p *ObjectPool) Start() {
	if p.running.Swap(true) {
		return // 已经在运行
	}

	go p.refillLoop()
	log.Info().Str("pool", p.name).Int64("capacity", p.capacity).Msg("Object pool started")
}

// Stop 停止后台补充
func (p *ObjectPool) Stop() {
	if !p.running.Swap(false) {
		return
	}
	close(p.stopChan)
	log.Info().Str("pool", p.name).Msg("Object pool stopped")
}

// Pause 暂停补充
func (p *ObjectPool) Pause() {
	p.paused.Store(true)
	log.Info().Str("pool", p.name).Msg("Object pool paused")
}

// Resume 恢复补充
func (p *ObjectPool) Resume() {
	p.paused.Store(false)
	p.triggerRefill()
	log.Info().Str("pool", p.name).Msg("Object pool resumed")
}

// Get 获取一个对象
func (p *ObjectPool) Get() string {
	for {
		count := p.count.Load()
		if count <= 0 {
			// 池为空，同步生成
			p.triggerRefill()
			return p.generator()
		}

		head := p.head.Load()
		newHead := (head + 1) % p.capacity

		if p.head.CompareAndSwap(head, newHead) {
			p.count.Add(-1)
			p.totalConsumed.Add(1)

			// 检查是否需要补充
			if p.count.Load() < p.lowMark {
				p.triggerRefill()
			}

			return p.items[head]
		}
	}
}

// triggerRefill 触发补充
func (p *ObjectPool) triggerRefill() {
	select {
	case p.refillChan <- struct{}{}:
	default:
	}
}

// refillLoop 补充循环
func (p *ObjectPool) refillLoop() {
	for {
		select {
		case <-p.stopChan:
			return
		case <-p.refillChan:
			if p.paused.Load() {
				continue
			}
			p.doRefill()
		}
	}
}

// doRefill 执行补充
func (p *ObjectPool) doRefill() {
	p.refillCount.Add(1)

	for p.count.Load() < p.capacity && p.running.Load() && !p.paused.Load() {
		item := p.generator()

		tail := p.tail.Load()
		newTail := (tail + 1) % p.capacity

		if p.tail.CompareAndSwap(tail, newTail) {
			p.items[tail] = item
			p.count.Add(1)
			p.totalGenerated.Add(1)
		}

		// 批量生成后短暂让出 CPU
		if p.totalGenerated.Load()%1000 == 0 {
			time.Sleep(time.Microsecond)
		}
	}
}

// Warmup 预热池
func (p *ObjectPool) Warmup(targetPercent float64) {
	if targetPercent <= 0 || targetPercent > 1 {
		targetPercent = 0.5
	}

	target := int64(float64(p.capacity) * targetPercent)
	current := p.count.Load()

	if current >= target {
		return
	}

	log.Info().
		Str("pool", p.name).
		Int64("target", target).
		Int64("current", current).
		Msg("Warming up pool")

	for p.count.Load() < target {
		item := p.generator()

		tail := p.tail.Load()
		newTail := (tail + 1) % p.capacity

		if p.tail.CompareAndSwap(tail, newTail) {
			p.items[tail] = item
			p.count.Add(1)
			p.totalGenerated.Add(1)
		}
	}

	log.Info().
		Str("pool", p.name).
		Int64("count", p.count.Load()).
		Msg("Pool warmup completed")
}

// Clear 清空池
func (p *ObjectPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.head.Store(0)
	p.tail.Store(0)
	p.count.Store(0)

	log.Info().Str("pool", p.name).Msg("Pool cleared")
}

// Resize 调整池大小
func (p *ObjectPool) Resize(newCapacity int) {
	if newCapacity <= 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	oldCapacity := p.capacity

	// 创建新数组
	newItems := make([]string, newCapacity)

	// 复制现有数据
	count := p.count.Load()
	if count > int64(newCapacity) {
		count = int64(newCapacity)
	}

	head := p.head.Load()
	for i := int64(0); i < count; i++ {
		idx := (head + i) % oldCapacity
		newItems[i] = p.items[idx]
	}

	// 替换
	p.items = newItems
	p.capacity = int64(newCapacity)
	p.head.Store(0)
	p.tail.Store(count)
	p.count.Store(count)
	p.lowMark = int64(float64(newCapacity) * 0.2)

	log.Info().
		Str("pool", p.name).
		Int64("old_capacity", oldCapacity).
		Int("new_capacity", newCapacity).
		Int64("preserved", count).
		Msg("Pool resized")

	// 触发补充
	p.triggerRefill()
}

// Stats 获取统计信息
func (p *ObjectPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"name":            p.name,
		"capacity":        p.capacity,
		"count":           p.count.Load(),
		"usage_percent":   float64(p.count.Load()) / float64(p.capacity) * 100,
		"low_mark":        p.lowMark,
		"total_generated": p.totalGenerated.Load(),
		"total_consumed":  p.totalConsumed.Load(),
		"refill_count":    p.refillCount.Load(),
		"running":         p.running.Load(),
		"paused":          p.paused.Load(),
	}
}

// Capacity 返回容量
func (p *ObjectPool) Capacity() int64 {
	return p.capacity
}

// Count 返回当前数量
func (p *ObjectPool) Count() int64 {
	return p.count.Load()
}

// UsagePercent 返回使用率
func (p *ObjectPool) UsagePercent() float64 {
	return float64(p.count.Load()) / float64(p.capacity) * 100
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/object_pool.go
git commit -m "feat: enhance object pool with resize and pause support"
```

---

## Task 2: 添加预设配置

**Files:**
- Create: `go-page-server/core/pool_presets.go`

**Step 1: 创建预设配置**

```go
package core

// PoolPreset 池预设配置
type PoolPreset struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	TargetQPS     int     `json:"target_qps"`
	SafetyFactor  float64 `json:"safety_factor"`
	BufferSeconds float64 `json:"buffer_seconds"`
}

// 预定义的预设配置
var PoolPresets = map[string]PoolPreset{
	"low": {
		Name:          "低并发",
		Description:   "适用于 100 QPS 以下的场景",
		TargetQPS:     100,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
	"medium": {
		Name:          "中并发",
		Description:   "适用于 500 QPS 左右的场景",
		TargetQPS:     500,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
	"high": {
		Name:          "高并发",
		Description:   "适用于 1000 QPS 左右的场景",
		TargetQPS:     1000,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
	"extreme": {
		Name:          "超高并发",
		Description:   "适用于 2000+ QPS 的场景",
		TargetQPS:     2000,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
}

// GetPoolPreset 获取预设配置
func GetPoolPreset(name string) (PoolPreset, bool) {
	preset, ok := PoolPresets[name]
	return preset, ok
}

// GetAllPoolPresets 获取所有预设配置
func GetAllPoolPresets() map[string]PoolPreset {
	return PoolPresets
}

// CalculatePoolSizes 根据预设和模板分析计算池大小
func CalculatePoolSizes(preset PoolPreset, maxStats TemplateFuncStats) *PoolSizeConfig {
	multiplier := float64(preset.TargetQPS) * preset.SafetyFactor * preset.BufferSeconds

	return &PoolSizeConfig{
		ClsPoolSize:          int(float64(maxStats.Cls) * multiplier),
		URLPoolSize:          int(float64(maxStats.RandomURL) * multiplier),
		KeywordEmojiPoolSize: int(float64(maxStats.KeywordEmoji) * multiplier),
		TargetQPS:            preset.TargetQPS,
		SafetyFactor:         preset.SafetyFactor,
		BufferSeconds:        preset.BufferSeconds,
		MaxStats:             maxStats,
	}
}

// EstimateMemoryUsage 估算内存使用量
func EstimateMemoryUsage(config *PoolSizeConfig) int64 {
	// 假设每个字符串平均 20 字节
	avgStringSize := int64(20)

	total := int64(config.ClsPoolSize) * avgStringSize
	total += int64(config.URLPoolSize) * avgStringSize
	total += int64(config.KeywordEmojiPoolSize) * avgStringSize * 2 // emoji 可能更大

	return total
}

// FormatMemorySize 格式化内存大小
func FormatMemorySize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
```

需要添加 fmt 导入：

```go
import "fmt"
```

**Step 2: Commit**

```bash
git add go-page-server/core/pool_presets.go
git commit -m "feat: add pool preset configurations"
```

---

## Task 3: 更新 TemplateFuncsManager

**Files:**
- Modify: `go-page-server/core/template_funcs.go`

**Step 1: 添加动态调整方法**

在 `template_funcs.go` 中添加：

```go
// ResizePools 根据配置调整所有池大小
func (m *TemplateFuncsManager) ResizePools(config *PoolSizeConfig) {
	if config.ClsPoolSize > 0 && m.clsPool != nil {
		m.clsPool.Resize(config.ClsPoolSize)
	}
	if config.URLPoolSize > 0 && m.urlPool != nil {
		m.urlPool.Resize(config.URLPoolSize)
	}
	if config.KeywordEmojiPoolSize > 0 && m.keywordEmojiPool != nil {
		m.keywordEmojiPool.Resize(config.KeywordEmojiPoolSize)
	}

	log.Info().
		Int("cls", config.ClsPoolSize).
		Int("url", config.URLPoolSize).
		Int("keyword_emoji", config.KeywordEmojiPoolSize).
		Msg("Pools resized")
}

// WarmupPools 预热所有池
func (m *TemplateFuncsManager) WarmupPools(targetPercent float64) {
	if m.clsPool != nil {
		m.clsPool.Warmup(targetPercent)
	}
	if m.urlPool != nil {
		m.urlPool.Warmup(targetPercent)
	}
	if m.keywordEmojiPool != nil {
		m.keywordEmojiPool.Warmup(targetPercent)
	}
}

// ClearPools 清空所有池
func (m *TemplateFuncsManager) ClearPools() {
	if m.clsPool != nil {
		m.clsPool.Clear()
	}
	if m.urlPool != nil {
		m.urlPool.Clear()
	}
	if m.keywordEmojiPool != nil {
		m.keywordEmojiPool.Clear()
	}
	log.Info().Msg("All pools cleared")
}

// PausePools 暂停所有池的补充
func (m *TemplateFuncsManager) PausePools() {
	if m.clsPool != nil {
		m.clsPool.Pause()
	}
	if m.urlPool != nil {
		m.urlPool.Pause()
	}
	if m.keywordEmojiPool != nil {
		m.keywordEmojiPool.Pause()
	}
}

// ResumePools 恢复所有池的补充
func (m *TemplateFuncsManager) ResumePools() {
	if m.clsPool != nil {
		m.clsPool.Resume()
	}
	if m.urlPool != nil {
		m.urlPool.Resume()
	}
	if m.keywordEmojiPool != nil {
		m.keywordEmojiPool.Resume()
	}
}

// GetPoolStats 获取所有池的统计信息
func (m *TemplateFuncsManager) GetPoolStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if m.clsPool != nil {
		stats["cls"] = m.clsPool.Stats()
	}
	if m.urlPool != nil {
		stats["url"] = m.urlPool.Stats()
	}
	if m.keywordEmojiPool != nil {
		stats["keyword_emoji"] = m.keywordEmojiPool.Stats()
	}

	return stats
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/template_funcs.go
git commit -m "feat: add pool management methods to TemplateFuncsManager"
```

---

## Task 4: 添加测试

**Files:**
- Create: `go-page-server/core/object_pool_test.go`

**Step 1: 创建测试文件**

```go
package core

import (
	"sync"
	"testing"
	"time"
)

func TestObjectPool_BasicOperations(t *testing.T) {
	counter := 0
	pool := NewObjectPool("test", 100, 0.2, func() string {
		counter++
		return fmt.Sprintf("item_%d", counter)
	})

	pool.Start()
	defer pool.Stop()

	// 预热
	pool.Warmup(0.5)

	if pool.Count() < 50 {
		t.Errorf("Expected at least 50 items after warmup, got %d", pool.Count())
	}

	// 获取
	item := pool.Get()
	if item == "" {
		t.Error("Expected non-empty item")
	}
}

func TestObjectPool_ConcurrentAccess(t *testing.T) {
	counter := int64(0)
	pool := NewObjectPool("test", 1000, 0.2, func() string {
		return fmt.Sprintf("item_%d", atomic.AddInt64(&counter, 1))
	})

	pool.Start()
	defer pool.Stop()

	pool.Warmup(0.8)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// 100 个并发获取
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				item := pool.Get()
				if item == "" {
					errors <- fmt.Errorf("got empty item")
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestObjectPool_Resize(t *testing.T) {
	pool := NewObjectPool("test", 100, 0.2, func() string {
		return "item"
	})

	pool.Start()
	defer pool.Stop()

	pool.Warmup(0.5)

	initialCount := pool.Count()
	if initialCount < 50 {
		t.Errorf("Expected at least 50 items, got %d", initialCount)
	}

	// 扩容
	pool.Resize(200)

	if pool.Capacity() != 200 {
		t.Errorf("Expected capacity 200, got %d", pool.Capacity())
	}

	// 等待补充
	time.Sleep(100 * time.Millisecond)

	// 缩容
	pool.Resize(50)

	if pool.Capacity() != 50 {
		t.Errorf("Expected capacity 50, got %d", pool.Capacity())
	}
}

func TestObjectPool_PauseResume(t *testing.T) {
	pool := NewObjectPool("test", 100, 0.2, func() string {
		return "item"
	})

	pool.Start()
	defer pool.Stop()

	pool.Warmup(0.5)

	// 暂停
	pool.Pause()

	stats := pool.Stats()
	if !stats["paused"].(bool) {
		t.Error("Expected pool to be paused")
	}

	// 恢复
	pool.Resume()

	stats = pool.Stats()
	if stats["paused"].(bool) {
		t.Error("Expected pool to be resumed")
	}
}

func TestObjectPool_Clear(t *testing.T) {
	pool := NewObjectPool("test", 100, 0.2, func() string {
		return "item"
	})

	pool.Start()
	defer pool.Stop()

	pool.Warmup(0.5)

	if pool.Count() == 0 {
		t.Error("Expected non-zero count after warmup")
	}

	pool.Clear()

	if pool.Count() != 0 {
		t.Errorf("Expected zero count after clear, got %d", pool.Count())
	}
}

func TestPoolPresets(t *testing.T) {
	presets := GetAllPoolPresets()

	if len(presets) != 4 {
		t.Errorf("Expected 4 presets, got %d", len(presets))
	}

	preset, ok := GetPoolPreset("medium")
	if !ok {
		t.Error("Expected medium preset to exist")
	}
	if preset.TargetQPS != 500 {
		t.Errorf("Expected medium preset QPS=500, got %d", preset.TargetQPS)
	}
}

func TestCalculatePoolSizes(t *testing.T) {
	preset := PoolPreset{
		TargetQPS:     500,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	}

	maxStats := TemplateFuncStats{
		Cls:          100,
		RandomURL:    50,
		KeywordEmoji: 20,
	}

	config := CalculatePoolSizes(preset, maxStats)

	// 100 * 500 * 1.5 * 1 = 75000
	if config.ClsPoolSize != 75000 {
		t.Errorf("Expected ClsPoolSize=75000, got %d", config.ClsPoolSize)
	}

	// 50 * 500 * 1.5 * 1 = 37500
	if config.URLPoolSize != 37500 {
		t.Errorf("Expected URLPoolSize=37500, got %d", config.URLPoolSize)
	}

	// 20 * 500 * 1.5 * 1 = 15000
	if config.KeywordEmojiPoolSize != 15000 {
		t.Errorf("Expected KeywordEmojiPoolSize=15000, got %d", config.KeywordEmojiPoolSize)
	}
}
```

需要添加导入：

```go
import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestObjectPool
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/object_pool_test.go
git commit -m "test: add object pool tests"
```

---

## 完成检查清单

- [ ] Task 1: 增强对象池结构
- [ ] Task 2: 添加预设配置
- [ ] Task 3: 更新 TemplateFuncsManager
- [ ] Task 4: 添加测试

所有任务完成后运行完整测试：

```bash
cd go-page-server && go test -v ./...
```
