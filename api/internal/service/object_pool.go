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
	threshold     float64       // 低于此比例触发补充（0-1）
	numWorkers    int           // 生产者协程数
	checkInterval time.Duration // 检查间隔

	// 生产者
	generator   func() T
	memorySizer func(any) int64 // 计算元素内存占用

	// 控制
	stopCh  chan struct{}
	wg      sync.WaitGroup
	stopped atomic.Bool // 是否已停止

	// 统计
	totalGenerated int64
	totalConsumed  int64
	refillCount    atomic.Int64 // 补充次数统计
	lastRefresh    atomic.Int64 // 最后刷新时间戳（Unix纳秒）

	// 内存追踪（仅用于 string 类型）
	memoryBytes atomic.Int64

	// 用于 Resize 的互斥锁
	mu sync.RWMutex

	// ticker 用于 refillLoop，提升为字段以支持动态更新间隔
	ticker *time.Ticker
}

// PoolConfig 池配置
type PoolConfig struct {
	Name          string
	Size          int
	Threshold     float64 // 低于此比例触发补充（0-1）
	NumWorkers    int
	CheckInterval time.Duration
	MemorySizer   func(any) int64 // 可选：计算单个元素内存占用的函数
}

// NewObjectPool 创建对象池
func NewObjectPool[T any](cfg PoolConfig, generator func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		name:          cfg.Name,
		pool:          make([]T, cfg.Size),
		size:          int64(cfg.Size),
		threshold:     cfg.Threshold,
		numWorkers:    cfg.NumWorkers,
		checkInterval: cfg.CheckInterval,
		generator:     generator,
		memorySizer:   cfg.MemorySizer,
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

	// 用于收集各 worker 的内存统计
	memoryAdded := make([]int64, p.numWorkers)

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		startIdx := w * batchPerWorker
		workerBatch := batchPerWorker
		workerIdx := w
		// 最后一个 worker 处理剩余项
		if w == p.numWorkers-1 {
			workerBatch += remainder
		}
		go func(start, batch, wIdx int) {
			defer wg.Done()
			var localMem int64
			for i := 0; i < batch; i++ {
				item := p.generator()
				p.pool[start+i] = item
				if p.memorySizer != nil {
					localMem += p.memorySizer(item)
				}
			}
			memoryAdded[wIdx] = localMem
		}(startIdx, workerBatch, workerIdx)
	}

	wg.Wait()
	atomic.StoreInt64(&p.tail, p.size)
	atomic.AddInt64(&p.totalGenerated, p.size)
	p.lastRefresh.Store(time.Now().UnixNano())

	// 汇总内存统计
	if p.memorySizer != nil {
		var totalMem int64
		for _, m := range memoryAdded {
			totalMem += m
		}
		p.memoryBytes.Store(totalMem)
	}
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

	p.mu.Lock()
	p.ticker = time.NewTicker(p.checkInterval)
	p.mu.Unlock()
	defer p.ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-p.ticker.C:
			p.checkAndRefill()
		}
	}
}

// checkAndRefill 检查并补充
func (p *ObjectPool[T]) checkAndRefill() {
	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	available := p.Available()
	thresholdCount := int64(float64(size) * p.threshold)

	if available < thresholdCount {
		// 补充到满
		need := size - available
		p.refillToFull(int(need))
		p.refillCount.Add(1)
		p.lastRefresh.Store(time.Now().UnixNano())
	}
}

// refillToFull 多协程并行补充指定数量
func (p *ObjectPool[T]) refillToFull(need int) {
	if need <= 0 {
		return
	}

	var wg sync.WaitGroup
	batchPerWorker := need / p.numWorkers
	remainder := need % p.numWorkers

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		workerBatch := batchPerWorker
		if w == p.numWorkers-1 {
			workerBatch += remainder
		}
		go func(batch int) {
			defer wg.Done()

			items := make([]T, batch)
			for i := 0; i < batch; i++ {
				items[i] = p.generator()
			}

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
		"status":          status,
		"refill_count":    p.refillCount.Load(),
		"num_workers":     p.numWorkers,
		"last_refresh":    lastRefresh,
		"memory_bytes":    p.memoryBytes.Load(),
	}
}

// MemoryBytes 返回内存占用字节数
func (p *ObjectPool[T]) MemoryBytes() int64 {
	return p.memoryBytes.Load()
}

// Clear 清空池（重置 head/tail）
func (p *ObjectPool[T]) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	atomic.StoreInt64(&p.head, 0)
	atomic.StoreInt64(&p.tail, 0)
	p.memoryBytes.Store(0)

	log.Info().Str("pool", p.name).Msg("Object pool cleared")
}

// UpdateConfig 动态更新配置（全部即时生效）
func (p *ObjectPool[T]) UpdateConfig(size int, threshold float64, numWorkers int, checkInterval time.Duration) {
	p.mu.Lock()

	// 1. 更新阈值
	p.threshold = threshold

	// 2. 更新协程数
	p.numWorkers = numWorkers

	// 3. 更新检查间隔（即时生效）
	if checkInterval != p.checkInterval && checkInterval > 0 {
		p.checkInterval = checkInterval
		if p.ticker != nil {
			p.ticker.Reset(checkInterval)
		}
	}

	// 4. 检查是否需要调整大小
	needResize := size > 0 && int64(size) != p.size
	p.mu.Unlock()

	// 5. 调整池大小（Resize 有自己的锁）
	if needResize {
		p.Resize(size)
	}

	log.Info().
		Str("pool", p.name).
		Int("size", size).
		Float64("threshold", threshold).
		Int("workers", numWorkers).
		Dur("interval", checkInterval).
		Msg("Pool config updated")
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
