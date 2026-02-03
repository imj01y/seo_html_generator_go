// api/internal/service/memory_pool.go
package core

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
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

// UpdateTask represents a status update task
type UpdateTask struct {
	Table string
	ID    int64
}

// MemoryPool is a thread-safe FIFO queue for pool items
type MemoryPool struct {
	items       []PoolItem
	mu          sync.RWMutex
	groupID     int
	poolType    string // "titles" or "contents"
	maxSize     int
	memoryBytes atomic.Int64 // 内存占用追踪
}

// NewMemoryPool creates a new memory pool
func NewMemoryPool(groupID int, poolType string, maxSize int) *MemoryPool {
	return &MemoryPool{
		items:    make([]PoolItem, 0, maxSize),
		groupID:  groupID,
		poolType: poolType,
		maxSize:  maxSize,
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

	return item, true
}

// Push adds items to the end of the pool
func (p *MemoryPool) Push(items []PoolItem) {
	if len(items) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Respect max size
	available := p.maxSize - len(p.items)
	if available <= 0 {
		return
	}
	if len(items) > available {
		items = items[:available]
	}

	// 增加内存计数
	var addedMem int64
	for _, item := range items {
		addedMem += StringMemorySize(item.Text)
	}
	p.memoryBytes.Add(addedMem)

	p.items = append(p.items, items...)
}

// Len returns the current number of items in the pool
func (p *MemoryPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.items)
}

// Clear removes all items from the pool
func (p *MemoryPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = p.items[:0]
	p.memoryBytes.Store(0)
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
