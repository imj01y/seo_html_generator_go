// api/internal/service/pool_manager.go
package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// ErrCachePoolEmpty is returned when the cache pool is empty
var ErrCachePoolEmpty = errors.New("cache pool is empty")

// PoolManager manages memory pools for titles and contents
type PoolManager struct {
	titles   map[int]*MemoryPool // groupID -> pool
	contents map[int]*MemoryPool // groupID -> pool
	config   *CachePoolConfig
	db       *sqlx.DB
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	updateCh chan UpdateTask
	wg       sync.WaitGroup
	stopped  atomic.Bool
}

// NewPoolManager creates a new pool manager
func NewPoolManager(db *sqlx.DB) *PoolManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolManager{
		titles:   make(map[int]*MemoryPool),
		contents: make(map[int]*MemoryPool),
		config:   DefaultCachePoolConfig(),
		db:       db,
		ctx:      ctx,
		cancel:   cancel,
		updateCh: make(chan UpdateTask, 1000),
	}
}

// Start starts the pool manager
func (m *PoolManager) Start(ctx context.Context) error {
	// Load config from DB
	config, err := LoadCachePoolConfig(ctx, m.db)
	if err != nil {
		return fmt.Errorf("failed to load pool config: %w", err)
	}
	m.config = config

	// Discover and initialize pools for all groups
	groupIDs, err := m.discoverGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover groups: %w", err)
	}

	for _, gid := range groupIDs {
		m.getOrCreatePool("titles", gid)
		m.getOrCreatePool("contents", gid)
	}

	// Initial fill
	m.checkAndRefillAll()

	// Start background workers
	m.wg.Add(2)
	go m.refillLoop()
	go m.updateWorker()

	log.Info().
		Int("groups", len(groupIDs)).
		Int("titles_size", m.config.TitlesSize).
		Int("contents_size", m.config.ContentsSize).
		Msg("PoolManager started")

	return nil
}

// Stop stops the pool manager gracefully
func (m *PoolManager) Stop() {
	m.stopped.Store(true)
	m.cancel()
	close(m.updateCh)
	m.wg.Wait()
	log.Info().Msg("PoolManager stopped")
}

// discoverGroups finds all active group IDs
func (m *PoolManager) discoverGroups(ctx context.Context) ([]int, error) {
	query := `
		SELECT DISTINCT group_id FROM (
			SELECT group_id FROM titles WHERE status = 1
			UNION
			SELECT group_id FROM contents WHERE status = 1
		) t
	`
	var groupIDs []int
	err := m.db.SelectContext(ctx, &groupIDs, query)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return []int{1}, nil // Default to group 1
	}
	return groupIDs, nil
}

// getOrCreatePool gets or creates a pool for the given type and group
func (m *PoolManager) getOrCreatePool(poolType string, groupID int) *MemoryPool {
	m.mu.Lock()
	defer m.mu.Unlock()

	var pools map[int]*MemoryPool
	var maxSize int

	if poolType == "titles" {
		pools = m.titles
		maxSize = m.config.TitlesSize
	} else {
		pools = m.contents
		maxSize = m.config.ContentsSize
	}

	pool, exists := pools[groupID]
	if !exists {
		pool = NewMemoryPool(groupID, poolType, maxSize)
		pools[groupID] = pool
		log.Debug().Str("type", poolType).Int("group", groupID).Msg("Created new pool")
	}

	return pool
}

// Pop retrieves an item from the pool
func (m *PoolManager) Pop(poolType string, groupID int) (string, error) {
	if err := validatePoolType(poolType); err != nil {
		return "", err
	}

	pool := m.getOrCreatePool(poolType, groupID)
	item, ok := pool.Pop()
	if !ok {
		// Try to refill and pop again
		m.refillPool(pool)
		item, ok = pool.Pop()
		if !ok {
			return "", ErrCachePoolEmpty
		}
	}

	// Async update status
	if !m.stopped.Load() {
		select {
		case m.updateCh <- UpdateTask{Table: poolType, ID: item.ID}:
		default:
			log.Warn().Str("table", poolType).Int64("id", item.ID).Msg("Update channel full")
		}
	}

	return item.Text, nil
}

// refillLoop runs the background refill check
func (m *PoolManager) refillLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.RefillInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.checkAndRefillAll()
		case <-m.ctx.Done():
			return
		}
	}
}

// checkAndRefillAll checks and refills all pools
func (m *PoolManager) checkAndRefillAll() {
	m.mu.RLock()
	titlePools := make([]*MemoryPool, 0, len(m.titles))
	contentPools := make([]*MemoryPool, 0, len(m.contents))
	for _, p := range m.titles {
		titlePools = append(titlePools, p)
	}
	for _, p := range m.contents {
		contentPools = append(contentPools, p)
	}
	threshold := m.config.Threshold
	m.mu.RUnlock()

	for _, pool := range titlePools {
		if pool.Len() < threshold {
			m.refillPool(pool)
		}
	}
	for _, pool := range contentPools {
		if pool.Len() < threshold {
			m.refillPool(pool)
		}
	}
}

// refillPool refills a single pool from database
func (m *PoolManager) refillPool(pool *MemoryPool) {
	poolType := pool.GetPoolType()
	groupID := pool.GetGroupID()
	currentLen := pool.Len()
	maxSize := pool.GetMaxSize()
	need := maxSize - currentLen

	if need <= 0 {
		return
	}

	column := "title"
	if poolType == "contents" {
		column = "content"
	}

	query := fmt.Sprintf(`
		SELECT id, %s as text FROM %s
		WHERE group_id = ? AND status = 1
		ORDER BY batch_id DESC, id ASC
		LIMIT ?
	`, column, poolType)

	var items []PoolItem
	err := m.db.SelectContext(m.ctx, &items, query, groupID, need)
	if err != nil {
		log.Error().Err(err).Str("type", poolType).Int("group", groupID).Msg("Failed to refill pool")
		return
	}

	if len(items) > 0 {
		pool.Push(items)
		log.Debug().
			Str("type", poolType).
			Int("group", groupID).
			Int("added", len(items)).
			Int("total", pool.Len()).
			Msg("Pool refilled")
	}
}

// updateWorker processes status updates
func (m *PoolManager) updateWorker() {
	defer m.wg.Done()

	for task := range m.updateCh {
		select {
		case <-m.ctx.Done():
			return
		default:
			m.processUpdate(task)
		}
	}
}

// processUpdate updates the status of a consumed item
func (m *PoolManager) processUpdate(task UpdateTask) {
	if err := validatePoolType(task.Table); err != nil {
		return
	}
	query := fmt.Sprintf("UPDATE %s SET status = 0 WHERE id = ?", task.Table)
	_, err := m.db.ExecContext(m.ctx, query, task.ID)
	if err != nil {
		log.Warn().Err(err).Str("table", task.Table).Int64("id", task.ID).Msg("Failed to update status")
	}
}

// Reload reloads configuration from database
func (m *PoolManager) Reload(ctx context.Context) error {
	config, err := LoadCachePoolConfig(ctx, m.db)
	if err != nil {
		return err
	}

	m.mu.Lock()
	oldConfig := m.config
	m.config = config

	// Resize pools if needed
	if config.TitlesSize != oldConfig.TitlesSize {
		for _, pool := range m.titles {
			pool.Resize(config.TitlesSize)
		}
	}
	if config.ContentsSize != oldConfig.ContentsSize {
		for _, pool := range m.contents {
			pool.Resize(config.ContentsSize)
		}
	}
	m.mu.Unlock()

	log.Info().
		Int("titles_size", config.TitlesSize).
		Int("contents_size", config.ContentsSize).
		Int("threshold", config.Threshold).
		Int("interval_ms", config.RefillIntervalMs).
		Msg("PoolManager config reloaded")

	return nil
}

// GetStats returns pool statistics
func (m *PoolManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	titlesStats := make(map[int]map[string]int)
	contentsStats := make(map[int]map[string]int)

	for gid, pool := range m.titles {
		titlesStats[gid] = map[string]int{
			"current":   pool.Len(),
			"max_size":  pool.GetMaxSize(),
			"threshold": m.config.Threshold,
		}
	}
	for gid, pool := range m.contents {
		contentsStats[gid] = map[string]int{
			"current":   pool.Len(),
			"max_size":  pool.GetMaxSize(),
			"threshold": m.config.Threshold,
		}
	}

	return map[string]interface{}{
		"titles":   titlesStats,
		"contents": contentsStats,
		"config": map[string]interface{}{
			"titles_size":        m.config.TitlesSize,
			"contents_size":      m.config.ContentsSize,
			"threshold":          m.config.Threshold,
			"refill_interval_ms": m.config.RefillIntervalMs,
		},
	}
}

// GetConfig returns the current configuration
func (m *PoolManager) GetConfig() *CachePoolConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
