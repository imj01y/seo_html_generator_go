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
	stopCh chan struct{}
	wg     sync.WaitGroup

	// 统计
	totalGenerated int64
	totalConsumed  int64
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
	batchPerWorker := int(p.size) / p.numWorkers

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		startIdx := w * batchPerWorker
		go func(start int) {
			defer wg.Done()
			for i := 0; i < batchPerWorker; i++ {
				p.pool[start+i] = p.generator()
			}
		}(startIdx)
	}

	wg.Wait()
	atomic.StoreInt64(&p.tail, p.size)
	atomic.AddInt64(&p.totalGenerated, p.size)
}

// Get 获取对象（无锁，O(1)）
func (p *ObjectPool[T]) Get() T {
	idx := atomic.AddInt64(&p.head, 1) - 1
	atomic.AddInt64(&p.totalConsumed, 1)
	return p.pool[idx%p.size]
}

// Available 当前可用数量
func (p *ObjectPool[T]) Available() int64 {
	tail := atomic.LoadInt64(&p.tail)
	head := atomic.LoadInt64(&p.head)
	avail := tail - head
	if avail < 0 {
		avail = 0
	}
	if avail > p.size {
		avail = p.size
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
	available := p.Available()
	threshold := int64(float64(p.size) * p.lowWatermark)

	if available < threshold {
		p.refillParallel()
	}
}

// refillParallel 多协程并行补充
func (p *ObjectPool[T]) refillParallel() {
	var wg sync.WaitGroup
	batchPerWorker := p.refillBatch / p.numWorkers

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 先生成到本地数组
			items := make([]T, batchPerWorker)
			for i := 0; i < batchPerWorker; i++ {
				items[i] = p.generator()
			}

			// 批量写入池子
			for _, item := range items {
				idx := atomic.AddInt64(&p.tail, 1) - 1
				p.pool[idx%p.size] = item
			}

			atomic.AddInt64(&p.totalGenerated, int64(batchPerWorker))
		}()
	}

	wg.Wait()
}

// Stop 停止池子
func (p *ObjectPool[T]) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	log.Info().Str("pool", p.name).Msg("Object pool stopped")
}

// Stats 返回统计信息
func (p *ObjectPool[T]) Stats() map[string]interface{} {
	return map[string]interface{}{
		"name":            p.name,
		"size":            p.size,
		"available":       p.Available(),
		"total_generated": atomic.LoadInt64(&p.totalGenerated),
		"total_consumed":  atomic.LoadInt64(&p.totalConsumed),
		"utilization":     float64(p.Available()) / float64(p.size) * 100,
	}
}
