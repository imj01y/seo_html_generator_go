package core

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// poolSnapshot 不可变的池数据快照（用于无锁读取）
type poolSnapshot[T any] struct {
	data []T
	size int64
}

// ObjectPool 高性能对象池（生产者消费者模式）
// Get() 完全无锁，通过 atomic.Pointer 读取池快照
type ObjectPool[T any] struct {
	name     string                          // 池名称（用于日志）
	snapshot atomic.Pointer[poolSnapshot[T]] // 无锁快照（替代 pool+size+RLock）

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

	// 仅用于 Resize/Clear/UpdateConfig 等低频操作的互斥锁
	mu sync.Mutex

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
	p := &ObjectPool[T]{
		name:          cfg.Name,
		threshold:     cfg.Threshold,
		numWorkers:    cfg.NumWorkers,
		checkInterval: cfg.CheckInterval,
		generator:     generator,
		memorySizer:   cfg.MemorySizer,
		stopCh:        make(chan struct{}),
	}
	// 初始化快照
	snap := &poolSnapshot[T]{
		data: make([]T, cfg.Size),
		size: int64(cfg.Size),
	}
	p.snapshot.Store(snap)
	return p
}

// Start 启动池子
func (p *ObjectPool[T]) Start() {
	snap := p.snapshot.Load()
	log.Info().Str("pool", p.name).Int64("size", snap.size).Msg("Starting object pool")

	// 多协程并行预填充
	p.prefillParallel()

	// 启动后台生产者
	p.wg.Add(1)
	go p.refillLoop()

	log.Info().Str("pool", p.name).Msg("Object pool started")
}

// prefillParallel 并行预填充
func (p *ObjectPool[T]) prefillParallel() {
	snap := p.snapshot.Load()
	size := int(snap.size)
	var wg sync.WaitGroup
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
				snap.data[start+i] = item
				if p.memorySizer != nil {
					localMem += p.memorySizer(item)
				}
			}
			memoryAdded[wIdx] = localMem
		}(startIdx, workerBatch, workerIdx)
	}

	wg.Wait()
	atomic.StoreInt64(&p.tail, snap.size)
	atomic.AddInt64(&p.totalGenerated, snap.size)
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

// Get 获取对象（完全无锁）
func (p *ObjectPool[T]) Get() T {
	snap := p.snapshot.Load() // atomic load, 无锁

	idx := atomic.AddInt64(&p.head, 1) - 1
	atomic.AddInt64(&p.totalConsumed, 1)
	return snap.data[idx%snap.size]
}

// Available 当前可用数量
func (p *ObjectPool[T]) Available() int64 {
	snap := p.snapshot.Load()

	tail := atomic.LoadInt64(&p.tail)
	head := atomic.LoadInt64(&p.head)
	avail := tail - head
	if avail < 0 {
		avail = 0
	}
	if avail > snap.size {
		avail = snap.size
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
	snap := p.snapshot.Load()

	available := p.Available()
	thresholdCount := int64(float64(snap.size) * p.threshold)

	if available < thresholdCount {
		// 补充到满
		need := snap.size - available
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

			snap := p.snapshot.Load()

			for _, item := range items {
				idx := atomic.AddInt64(&p.tail, 1) - 1
				snap.data[idx%snap.size] = item
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
	snap := p.snapshot.Load()

	available := p.Available()
	used := snap.size - available

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
		"size":            snap.size,
		"available":       available,
		"used":            used,
		"total_generated": atomic.LoadInt64(&p.totalGenerated),
		"total_consumed":  atomic.LoadInt64(&p.totalConsumed),
		"utilization":     float64(available) / float64(snap.size) * 100,
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
	snap := p.snapshot.Load()
	needResize := size > 0 && int64(size) != snap.size
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

	oldSnap := p.snapshot.Load()
	if int64(newSize) == oldSnap.size {
		return
	}

	log.Info().Str("pool", p.name).Int64("oldSize", oldSnap.size).Int("newSize", newSize).Msg("Resizing pool")

	// 创建新的池
	newPool := make([]T, newSize)

	// 获取当前有效数据
	head := atomic.LoadInt64(&p.head)
	tail := atomic.LoadInt64(&p.tail)
	available := tail - head
	if available < 0 {
		available = 0
	}
	if available > oldSnap.size {
		available = oldSnap.size
	}

	// 复制现有数据到新池
	copyCount := available
	if copyCount > int64(newSize) {
		copyCount = int64(newSize)
	}

	for i := int64(0); i < copyCount; i++ {
		srcIdx := (head + i) % oldSnap.size
		newPool[i] = oldSnap.data[srcIdx]
	}

	// 原子交换快照
	newSnap := &poolSnapshot[T]{
		data: newPool,
		size: int64(newSize),
	}
	p.snapshot.Store(newSnap)

	atomic.StoreInt64(&p.head, 0)
	atomic.StoreInt64(&p.tail, copyCount)

	log.Info().Str("pool", p.name).Int64("copied", copyCount).Int("newSize", newSize).Msg("Pool resize completed")
}

// Capacity 返回容量
func (p *ObjectPool[T]) Capacity() int64 {
	return p.snapshot.Load().size
}

// Count 返回当前数量（同 Available）
func (p *ObjectPool[T]) Count() int64 {
	return p.Available()
}

// UsagePercent 返回使用率百分比
func (p *ObjectPool[T]) UsagePercent() float64 {
	snap := p.snapshot.Load()
	available := p.Available()
	if snap.size == 0 {
		return 0
	}
	return float64(available) / float64(snap.size) * 100
}
