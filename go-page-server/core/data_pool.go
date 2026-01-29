package core

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// DataPool 只读数据池（随机选取，不消耗）
type DataPool struct {
	name     string
	items    []string
	mu       sync.RWMutex
	lastLoad time.Time

	// 统计
	totalSelects atomic.Int64
}

// NewDataPool 创建数据池
func NewDataPool(name string) *DataPool {
	return &DataPool{
		name:  name,
		items: make([]string, 0),
	}
}

// Load 加载数据
func (p *DataPool) Load(items []string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 复制数据，避免外部修改影响
	p.items = make([]string, len(items))
	copy(p.items, items)
	p.lastLoad = time.Now()

	log.Info().Str("pool", p.name).Int("count", len(items)).Msg("Data pool loaded")
}

// Get 随机获取一个数据（不消耗）
func (p *DataPool) Get() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.items) == 0 {
		return ""
	}

	p.totalSelects.Add(1)
	idx := rand.IntN(len(p.items))
	return p.items[idx]
}

// GetN 随机获取 N 个数据（可能重复）
func (p *DataPool) GetN(n int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.items) == 0 || n <= 0 {
		return nil
	}

	result := make([]string, n)
	for i := 0; i < n; i++ {
		idx := rand.IntN(len(p.items))
		result[i] = p.items[idx]
	}

	p.totalSelects.Add(int64(n))
	return result
}

// GetUnique 尽量获取不重复的 N 个数据
// 使用 Fisher-Yates 部分洗牌算法
func (p *DataPool) GetUnique(n int) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.items) == 0 || n <= 0 {
		return nil
	}

	// 如果请求数量 > 池大小，只返回池大小数量
	count := n
	if count > len(p.items) {
		count = len(p.items)
	}

	// 复制索引数组用于洗牌
	indices := make([]int, len(p.items))
	for i := range indices {
		indices[i] = i
	}

	// Fisher-Yates 部分洗牌：只洗前 count 个
	for i := 0; i < count; i++ {
		// 从 [i, len) 范围随机选一个
		j := i + rand.IntN(len(indices)-i)
		indices[i], indices[j] = indices[j], indices[i]
	}

	// 取前 count 个索引对应的数据
	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = p.items[indices[i]]
	}

	p.totalSelects.Add(int64(count))
	return result
}

// Count 返回数据量
func (p *DataPool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

// LastLoad 返回最后加载时间
func (p *DataPool) LastLoad() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastLoad
}

// Stats 返回统计信息
func (p *DataPool) Stats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"name":          p.name,
		"count":         len(p.items),
		"last_load":     p.lastLoad,
		"total_selects": p.totalSelects.Load(),
	}
}
