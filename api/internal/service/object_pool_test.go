package core

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 1. TestObjectPool_BasicOperations - 基本操作测试
// - 创建池、启动、预热、获取对象、停止
func TestObjectPool_BasicOperations(t *testing.T) {
	var counter int64

	cfg := PoolConfig{
		Name:          "test-basic",
		Size:          100,
		LowWatermark:  0.3,
		RefillBatch:   20,
		NumWorkers:    4,
		CheckInterval: 100 * time.Millisecond,
	}

	pool := NewObjectPool[string](cfg, func() string {
		n := atomic.AddInt64(&counter, 1)
		return fmt.Sprintf("item-%d", n)
	})

	// 启动池
	pool.Start()

	// 验证启动后有对象可用
	if pool.Available() <= 0 {
		t.Errorf("池启动后应该有可用对象, 但 Available() = %d", pool.Available())
	}

	// 预热到 80%
	pool.Warmup(0.8)

	// 等待预热完成
	time.Sleep(50 * time.Millisecond)

	// 验证预热后可用数量
	if pool.Available() < int64(float64(cfg.Size)*0.8) {
		t.Errorf("预热后应该有至少 80%% 的对象, 但 Available() = %d", pool.Available())
	}

	// 获取对象
	obj := pool.Get()
	if obj == "" {
		t.Error("获取的对象不应该为空")
	}

	// 停止池
	pool.Stop()

	// 验证可以安全地重复调用 Stop
	pool.Stop()
}

// 2. TestObjectPool_ConcurrentAccess - 并发访问测试
// - 100 个 goroutine 各获取 100 个对象
// - 验证所有获取都成功
func TestObjectPool_ConcurrentAccess(t *testing.T) {
	var counter int64

	cfg := PoolConfig{
		Name:          "test-concurrent",
		Size:          10000,
		LowWatermark:  0.3,
		RefillBatch:   1000,
		NumWorkers:    4,
		CheckInterval: 50 * time.Millisecond,
	}

	pool := NewObjectPool[string](cfg, func() string {
		n := atomic.AddInt64(&counter, 1)
		return fmt.Sprintf("item-%d", n)
	})

	pool.Start()
	defer pool.Stop()

	// 等待预填充完成
	time.Sleep(100 * time.Millisecond)

	var wg sync.WaitGroup
	var successCount int64
	numGoroutines := 100
	numGetsPerGoroutine := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numGetsPerGoroutine; j++ {
				obj := pool.Get()
				if obj != "" {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	expectedTotal := int64(numGoroutines * numGetsPerGoroutine)
	if successCount != expectedTotal {
		t.Errorf("并发获取应该全部成功, 期望 %d, 实际 %d", expectedTotal, successCount)
	}

	// 验证统计信息
	stats := pool.Stats()
	consumed := stats["total_consumed"].(int64)
	if consumed != expectedTotal {
		t.Errorf("统计的消费数应该为 %d, 实际 %d", expectedTotal, consumed)
	}
}

// 3. TestObjectPool_Resize - 扩缩容测试
// - 扩容到 200，验证容量
// - 缩容到 50，验证容量
func TestObjectPool_Resize(t *testing.T) {
	var counter int64

	cfg := PoolConfig{
		Name:          "test-resize",
		Size:          100,
		LowWatermark:  0.3,
		RefillBatch:   20,
		NumWorkers:    4,
		CheckInterval: 100 * time.Millisecond,
	}

	pool := NewObjectPool[string](cfg, func() string {
		n := atomic.AddInt64(&counter, 1)
		return fmt.Sprintf("item-%d", n)
	})

	pool.Start()
	defer pool.Stop()

	// 验证初始容量
	if pool.Capacity() != 100 {
		t.Errorf("初始容量应该为 100, 实际 %d", pool.Capacity())
	}

	// 扩容到 200
	pool.Resize(200)
	if pool.Capacity() != 200 {
		t.Errorf("扩容后容量应该为 200, 实际 %d", pool.Capacity())
	}

	// 缩容到 50
	pool.Resize(50)
	if pool.Capacity() != 50 {
		t.Errorf("缩容后容量应该为 50, 实际 %d", pool.Capacity())
	}
}

// 4. TestObjectPool_PauseResume - 暂停恢复测试
// - 暂停后 Stats().paused 应为 true
// - 恢复后 Stats().paused 应为 false
func TestObjectPool_PauseResume(t *testing.T) {
	var counter int64

	cfg := PoolConfig{
		Name:          "test-pause-resume",
		Size:          100,
		LowWatermark:  0.3,
		RefillBatch:   20,
		NumWorkers:    4,
		CheckInterval: 100 * time.Millisecond,
	}

	pool := NewObjectPool[string](cfg, func() string {
		n := atomic.AddInt64(&counter, 1)
		return fmt.Sprintf("item-%d", n)
	})

	pool.Start()
	defer pool.Stop()

	// 初始状态应该未暂停
	stats := pool.Stats()
	if stats["paused"].(bool) {
		t.Error("初始状态不应该是暂停的")
	}

	// 暂停
	pool.Pause()
	stats = pool.Stats()
	if !stats["paused"].(bool) {
		t.Error("暂停后 paused 应该为 true")
	}

	// 恢复
	pool.Resume()
	stats = pool.Stats()
	if stats["paused"].(bool) {
		t.Error("恢复后 paused 应该为 false")
	}
}

// 5. TestObjectPool_Clear - 清空测试
// - 预热后清空，验证 Count() == 0
func TestObjectPool_Clear(t *testing.T) {
	var counter int64

	cfg := PoolConfig{
		Name:          "test-clear",
		Size:          100,
		LowWatermark:  0.3,
		RefillBatch:   20,
		NumWorkers:    4,
		CheckInterval: 100 * time.Millisecond,
	}

	pool := NewObjectPool[string](cfg, func() string {
		n := atomic.AddInt64(&counter, 1)
		return fmt.Sprintf("item-%d", n)
	})

	pool.Start()
	defer pool.Stop()

	// 预热
	pool.Warmup(0.5)
	time.Sleep(50 * time.Millisecond)

	// 验证预热后有对象
	if pool.Count() <= 0 {
		t.Error("预热后应该有对象")
	}

	// 清空
	pool.Clear()

	// 验证清空后 Count() == 0
	if pool.Count() != 0 {
		t.Errorf("清空后 Count() 应该为 0, 实际 %d", pool.Count())
	}
}

// 6. TestPoolPresets - 预设配置测试
// - 验证有 4 种预设
// - 验证 medium 预设的 QPS = 500
func TestPoolPresets(t *testing.T) {
	presets := GetAllPoolPresets()

	// 验证有 4 种预设
	if len(presets) != 4 {
		t.Errorf("应该有 4 种预设, 实际 %d", len(presets))
	}

	// 验证预设名称
	expectedPresets := []string{"low", "medium", "high", "extreme"}
	for _, name := range expectedPresets {
		if _, ok := presets[name]; !ok {
			t.Errorf("缺少预设: %s", name)
		}
	}

	// 验证 medium 预设的 Concurrency = 200
	mediumPreset, ok := GetPoolPreset("medium")
	if !ok {
		t.Error("获取 medium 预设失败")
	}
	if mediumPreset.Concurrency != 200 {
		t.Errorf("medium 预设的 Concurrency 应该为 200, 实际 %d", mediumPreset.Concurrency)
	}
}

// 7. TestCalculatePoolSizes - 池大小计算测试
// - 使用 preset Concurrency=200
// - maxStats: Cls=100, RandomURL=50, KeywordEmoji=20
// - 公式: size = maxStat * Concurrency * DefaultBufferSeconds (3)
// - 验证 ClsPoolSize = 100 * 200 * 3 = 60000
// - 验证 URLPoolSize = 50 * 200 * 3 = 30000
// - 验证 KeywordEmojiPoolSize = 20 * 200 * 3 = 12000
func TestCalculatePoolSizes(t *testing.T) {
	preset := PoolPreset{
		Name:        "测试预设",
		Description: "用于测试的预设",
		Concurrency: 200,
	}

	maxStats := TemplateFuncStats{
		Cls:              100,
		RandomURL:        50,
		KeywordWithEmoji: 20,
	}

	config := CalculatePoolSizes(preset, maxStats)

	// 验证 ClsPoolSize = 100 * 200 * 3 = 60000
	expectedClsSize := 60000
	if config.ClsPoolSize != expectedClsSize {
		t.Errorf("ClsPoolSize 应该为 %d, 实际 %d", expectedClsSize, config.ClsPoolSize)
	}

	// 验证 URLPoolSize = 50 * 200 * 3 = 30000
	expectedURLSize := 30000
	if config.URLPoolSize != expectedURLSize {
		t.Errorf("URLPoolSize 应该为 %d, 实际 %d", expectedURLSize, config.URLPoolSize)
	}

	// 验证 KeywordEmojiPoolSize = 20 * 200 * 3 = 12000
	expectedKeywordEmojiSize := 12000
	if config.KeywordEmojiPoolSize != expectedKeywordEmojiSize {
		t.Errorf("KeywordEmojiPoolSize 应该为 %d, 实际 %d", expectedKeywordEmojiSize, config.KeywordEmojiPoolSize)
	}
}
