// api/internal/service/memory_pool.go
package core

import (
	"sync"
)

// Note: PoolItem and UpdateTask are defined in pool_consumer.go
// They will remain there until pool_consumer.go is deleted

// MemoryPool is a thread-safe FIFO queue for pool items
type MemoryPool struct {
	items    []PoolItem
	mu       sync.RWMutex
	groupID  int
	poolType string // "titles" or "contents"
	maxSize  int
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
}

// Resize changes the max size of the pool
func (p *MemoryPool) Resize(newMaxSize int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.maxSize = newMaxSize

	// Truncate if current items exceed new max size
	if len(p.items) > newMaxSize {
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
