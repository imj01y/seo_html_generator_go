package core

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ObjectPool 高性能对象池（生产者消费者模式）
type ObjectPool[T any] struct {
	name string // 池名称（用于日志）
	pool []T    // 环形缓冲区
	size int64  // 池子容量

	// 原子计数器
	head int64 // 消费位置
	tail int64 // 生产位置

	// 配置
	lowWatermark  float64       // 低水位线比例
	refillBatch   int           // 每次补充数量
	numWorkers    int           // 生产者协程数
	checkInterval time.Duration // 检查间隔

	// 生产者
	generator func() T

	// 控制
	stopCh  chan struct{}
	wg      sync.WaitGroup
	paused  atomic.Bool // 是否暂停补充
	stopped atomic.Bool // 是否已停止

	// 统计
	totalGenerated int64
	totalConsumed  int64
	refillCount    atomic.Int64 // 补充次数统计
	lastRefresh    atomic.Int64 // 最后刷新时间戳（Unix纳秒）

	// 用于 Resize 的互斥锁
	mu sync.RWMutex
}

// PoolConfig 池配置
type PoolConfig struct {
	Name          string
	Size          int
	LowWatermark  float64
	RefillBatch   int
	NumWorkers    int
	CheckInterval time.Duration
}

// NewObjectPool 创建对象池
func NewObjectPool[T any](cfg PoolConfig, generator func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		name:          cfg.Name,
		pool:          make([]T, cfg.Size),
		size:          int64(cfg.Size),
		lowWatermark:  cfg.LowWatermark,
		refillBatch:   cfg.RefillBatch,
		numWorkers:    cfg.NumWorkers,
		checkInterval: cfg.CheckInterval,
		generator:     generator,
		stopCh:        make(chan struct{}),
	}
}

// Start 启动池子
func (p *ObjectPool[T]) Start() {
	log.Info().Str("pool", p.name).Int64("size", p.size).Msg("Starting object pool")

	// 多协程并行预填充
	p.prefillParallel()

	// 启动后台生产者
	p.wg.Add(1)
	go p.refillLoop()

	log.Info().Str("pool", p.name).Msg("Object pool started")
}

// prefillParallel 并行预填充
func (p *ObjectPool[T]) prefillParallel() {
	var wg sync.WaitGroup
	size := int(p.size)
	batchPerWorker := size / p.numWorkers
	remainder := size % p.numWorkers

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		startIdx := w * batchPerWorker
		workerBatch := batchPerWorker
		// 最后一个 worker 处理剩余项
		if w == p.numWorkers-1 {
			workerBatch += remainder
		}
		go func(start, batch int) {
			defer wg.Done()
			for i := 0; i < batch; i++ {
				p.pool[start+i] = p.generator()
			}
		}(startIdx, workerBatch)
	}

	wg.Wait()
	atomic.StoreInt64(&p.tail, p.size)
	atomic.AddInt64(&p.totalGenerated, p.size)
	p.lastRefresh.Store(time.Now().UnixNano())
}

// Get 获取对象（加读锁保护，防止 Resize 期间数据竞争）
func (p *ObjectPool[T]) Get() T {
	p.mu.RLock()
	pool := p.pool
	size := p.size
	p.mu.RUnlock()

	idx := atomic.AddInt64(&p.head, 1) - 1
	atomic.AddInt64(&p.totalConsumed, 1)
	return pool[idx%size]
}

// Available 当前可用数量
func (p *ObjectPool[T]) Available() int64 {
	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	tail := atomic.LoadInt64(&p.tail)
	head := atomic.LoadInt64(&p.head)
	avail := tail - head
	if avail < 0 {
		avail = 0
	}
	if avail > size {
		avail = size
	}
	return avail
}

// refillLoop 后台补充循环
func (p *ObjectPool[T]) refillLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(p.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.checkAndRefill()
		}
	}
}

// checkAndRefill 检查并补充
func (p *ObjectPool[T]) checkAndRefill() {
	// 如果暂停则跳过补充
	if p.paused.Load() {
		return
	}

	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	available := p.Available()
	threshold := int64(float64(size) * p.lowWatermark)

	if available < threshold {
		p.refillParallel()
		p.refillCount.Add(1)
		p.lastRefresh.Store(time.Now().UnixNano())
	}
}

// refillParallel 多协程并行补充
func (p *ObjectPool[T]) refillParallel() {
	var wg sync.WaitGroup
	batchPerWorker := p.refillBatch / p.numWorkers
	remainder := p.refillBatch % p.numWorkers

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		workerBatch := batchPerWorker
		// 最后一个 worker 处理剩余项
		if w == p.numWorkers-1 {
			workerBatch += remainder
		}
		go func(batch int) {
			defer wg.Done()

			// 先生成到本地数组
			items := make([]T, batch)
			for i := 0; i < batch; i++ {
				items[i] = p.generator()
			}

			// 获取快照保护 Resize 期间的安全
			p.mu.RLock()
			pool := p.pool
			size := p.size
			p.mu.RUnlock()

			// 批量写入池子
			for _, item := range items {
				idx := atomic.AddInt64(&p.tail, 1) - 1
				pool[idx%size] = item
			}

			atomic.AddInt64(&p.totalGenerated, int64(batch))
		}(workerBatch)
	}

	wg.Wait()
}

// Stop 停止池子（安全支持重复调用）
func (p *ObjectPool[T]) Stop() {
	// 使用 CAS 确保只关闭一次
	if !p.stopped.CompareAndSwap(false, true) {
		return
	}
	close(p.stopCh)
	p.wg.Wait()
	log.Info().Str("pool", p.name).Msg("Object pool stopped")
}

// Stats 返回统计信息
func (p *ObjectPool[T]) Stats() map[string]interface{} {
	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	available := p.Available()
	used := size - available

	// 计算状态
	status := "running"
	if p.stopped.Load() {
		status = "stopped"
	} else if p.paused.Load() {
		status = "paused"
	}

	// 转换 lastRefresh 时间戳
	lastRefreshNano := p.lastRefresh.Load()
	var lastRefresh *time.Time
	if lastRefreshNano > 0 {
		t := time.Unix(0, lastRefreshNano)
		lastRefresh = &t
	}

	return map[string]interface{}{
		"name":            p.name,
		"size":            size,
		"available":       available,
		"used":            used,
		"total_generated": atomic.LoadInt64(&p.totalGenerated),
		"total_consumed":  atomic.LoadInt64(&p.totalConsumed),
		"utilization":     float64(available) / float64(size) * 100,
		"paused":          p.paused.Load(),
		"status":          status,
		"refill_count":    p.refillCount.Load(),
		"num_workers":     p.numWorkers,
		"last_refresh":    lastRefresh,
	}
}

// Pause 暂停后台补充
func (p *ObjectPool[T]) Pause() {
	p.paused.Store(true)
	log.Info().Str("pool", p.name).Msg("Object pool refill paused")
}

// Resume 恢复后台补充
func (p *ObjectPool[T]) Resume() {
	p.paused.Store(false)
	log.Info().Str("pool", p.name).Msg("Object pool refill resumed")
}

// Warmup 预热到指定比例
func (p *ObjectPool[T]) Warmup(targetPercent float64) {
	if targetPercent <= 0 || targetPercent > 1 {
		log.Warn().Str("pool", p.name).Float64("targetPercent", targetPercent).Msg("Invalid warmup target percent, should be between 0 and 1")
		return
	}

	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	targetCount := int64(float64(size) * targetPercent)
	currentAvailable := p.Available()

	if currentAvailable >= targetCount {
		log.Info().Str("pool", p.name).Int64("current", currentAvailable).Int64("target", targetCount).Msg("Pool already warmed up")
		return
	}

	needToGenerate := targetCount - currentAvailable
	log.Info().Str("pool", p.name).Int64("need", needToGenerate).Float64("targetPercent", targetPercent).Msg("Warming up pool")

	// 多协程并行预热
	var wg sync.WaitGroup
	batchPerWorker := int(needToGenerate) / p.numWorkers
	remainder := int(needToGenerate) % p.numWorkers
	if batchPerWorker < 1 {
		batchPerWorker = 1
		remainder = 0
	}

	for w := 0; w < p.numWorkers && int64(w*batchPerWorker) < needToGenerate; w++ {
		wg.Add(1)
		workerBatch := batchPerWorker
		// 最后一个 worker 处理剩余项
		if w == p.numWorkers-1 {
			workerBatch += remainder
		}
		if int64((w+1)*batchPerWorker) > needToGenerate && w != p.numWorkers-1 {
			workerBatch = int(needToGenerate) - w*batchPerWorker
		}
		go func(batch int) {
			defer wg.Done()

			items := make([]T, batch)
			for i := 0; i < batch; i++ {
				items[i] = p.generator()
			}

			// 获取快照保护 Resize 期间的安全
			p.mu.RLock()
			pool := p.pool
			currentSize := p.size
			p.mu.RUnlock()

			for _, item := range items {
				idx := atomic.AddInt64(&p.tail, 1) - 1
				pool[idx%currentSize] = item
			}

			atomic.AddInt64(&p.totalGenerated, int64(batch))
		}(workerBatch)
	}

	wg.Wait()
	log.Info().Str("pool", p.name).Int64("available", p.Available()).Msg("Pool warmup completed")
}

// Clear 清空池（重置 head/tail）
func (p *ObjectPool[T]) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	atomic.StoreInt64(&p.head, 0)
	atomic.StoreInt64(&p.tail, 0)

	log.Info().Str("pool", p.name).Msg("Object pool cleared")
}

// Resize 调整池大小，复制现有数据
func (p *ObjectPool[T]) Resize(newSize int) {
	if newSize <= 0 {
		log.Warn().Str("pool", p.name).Int("newSize", newSize).Msg("Invalid resize size, must be positive")
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	oldSize := p.size
	if int64(newSize) == oldSize {
		return
	}

	log.Info().Str("pool", p.name).Int64("oldSize", oldSize).Int("newSize", newSize).Msg("Resizing pool")

	// 创建新的池
	newPool := make([]T, newSize)

	// 获取当前有效数据
	head := atomic.LoadInt64(&p.head)
	tail := atomic.LoadInt64(&p.tail)
	available := tail - head
	if available < 0 {
		available = 0
	}
	if available > oldSize {
		available = oldSize
	}

	// 复制现有数据到新池
	copyCount := available
	if copyCount > int64(newSize) {
		copyCount = int64(newSize)
	}

	for i := int64(0); i < copyCount; i++ {
		srcIdx := (head + i) % oldSize
		newPool[i] = p.pool[srcIdx]
	}

	// 更新池
	p.pool = newPool
	p.size = int64(newSize)
	atomic.StoreInt64(&p.head, 0)
	atomic.StoreInt64(&p.tail, copyCount)

	log.Info().Str("pool", p.name).Int64("copied", copyCount).Int("newSize", newSize).Msg("Pool resize completed")
}

// Capacity 返回容量
func (p *ObjectPool[T]) Capacity() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.size
}

// Count 返回当前数量（同 Available）
func (p *ObjectPool[T]) Count() int64 {
	return p.Available()
}

// UsagePercent 返回使用率百分比
func (p *ObjectPool[T]) UsagePercent() float64 {
	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	available := p.Available()
	if size == 0 {
		return 0
	}
	return float64(available) / float64(size) * 100
}
