// api/internal/service/memory_pool.go
package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ErrPoolEmpty is returned when the pool is empty
var ErrPoolEmpty = errors.New("pool is empty")

// validTables is a whitelist of allowed table names for SQL queries
var validTables = map[string]bool{
	"contents": true,
}

// validatePoolType validates that the pool type is in the whitelist
func validatePoolType(poolType string) error {
	if !validTables[poolType] {
		return fmt.Errorf("invalid pool type: %s", poolType)
	}
	return nil
}

// PoolItem represents an item in the pool
type PoolItem struct {
	ID   int64  `db:"id" json:"id"`
	Text string `db:"text" json:"text"`
}

// MemoryPool is a thread-safe FIFO queue for pool items
type MemoryPool struct {
	items         []PoolItem
	mu            sync.RWMutex
	groupID       int
	poolType      string // "titles" or "contents"
	maxSize       int
	memoryBytes   atomic.Int64          // 内存占用追踪
	consumedCount atomic.Int64          // 被消费的数量（Pop 计数）
	loadedIDs     map[int64]struct{}    // 已加载的 ID 集合，用于去重
	exhaustedUntil time.Time            // 数据耗尽时的冷却截止时间，避免空转查询
}

// NewMemoryPool creates a new memory pool
func NewMemoryPool(groupID int, poolType string, maxSize int) *MemoryPool {
	return &MemoryPool{
		items:     make([]PoolItem, 0, maxSize),
		groupID:   groupID,
		poolType:  poolType,
		maxSize:   maxSize,
		loadedIDs: make(map[int64]struct{}),
	}
}

// Pop removes and returns the first item from the pool
func (p *MemoryPool) Pop() (PoolItem, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.items) == 0 {
		return PoolItem{}, false
	}

	item := p.items[0]
	p.items = p.items[1:]

	// 减少内存计数
	p.memoryBytes.Add(-StringMemorySize(item.Text))
	// 增加消费计数
	p.consumedCount.Add(1)

	return item, true
}

// Push adds items to the end of the pool, skipping items with duplicate IDs.
// Returns the number of items actually added.
func (p *MemoryPool) Push(items []PoolItem) int {
	if len(items) == 0 {
		return 0
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	available := p.maxSize - len(p.items)
	if available <= 0 {
		return 0
	}

	var addedMem int64
	added := 0
	for _, item := range items {
		if added >= available {
			break
		}
		if _, exists := p.loadedIDs[item.ID]; exists {
			continue
		}
		p.loadedIDs[item.ID] = struct{}{}
		p.items = append(p.items, item)
		addedMem += StringMemorySize(item.Text)
		added++
	}
	p.memoryBytes.Add(addedMem)
	return added
}

// Len returns the current number of items in the pool
func (p *MemoryPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

// Clear removes all items from the pool and resets loaded ID tracking
func (p *MemoryPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = p.items[:0]
	p.loadedIDs = make(map[int64]struct{})
	p.memoryBytes.Store(0)
	p.exhaustedUntil = time.Time{} // 重置冷却，允许立即重新加载
}

// Resize changes the max size of the pool
func (p *MemoryPool) Resize(newMaxSize int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxSize = newMaxSize

	// Truncate if current items exceed new max size
	if len(p.items) > newMaxSize {
		// 计算被移除项的内存
		var removedMem int64
		for i := newMaxSize; i < len(p.items); i++ {
			removedMem += StringMemorySize(p.items[i].Text)
		}
		p.memoryBytes.Add(-removedMem)
		p.items = p.items[:newMaxSize]
	}
}

// GetGroupID returns the group ID
func (p *MemoryPool) GetGroupID() int {
	return p.groupID
}

// GetPoolType returns the pool type
func (p *MemoryPool) GetPoolType() string {
	return p.poolType
}

// GetMaxSize returns the max size
func (p *MemoryPool) GetMaxSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.maxSize
}

// MemoryBytes returns the memory usage in bytes
func (p *MemoryPool) MemoryBytes() int64 {
	return p.memoryBytes.Load()
}

// ConsumedCount returns the number of items consumed (popped) from the pool
func (p *MemoryPool) ConsumedCount() int64 {
	return p.consumedCount.Load()
}

// MarkExhausted sets a cooldown period to avoid repeated useless DB queries
// when there's no new data available
func (p *MemoryPool) MarkExhausted(cooldown time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.exhaustedUntil = time.Now().Add(cooldown)
}

// IsExhausted returns true if the pool is in cooldown (no new data expected)
func (p *MemoryPool) IsExhausted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return time.Now().Before(p.exhaustedUntil)
}
