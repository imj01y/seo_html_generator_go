package pool

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// Manager 池管理器
// 协调 KeywordPool 和 ImagePool 的生命周期
type Manager struct {
	keywordPool *KeywordPool
	imagePool   *ImagePool
	db          *sqlx.DB
}

// NewManager 创建池管理器
func NewManager(db *sqlx.DB) *Manager {
	return &Manager{
		keywordPool: NewKeywordPool(db),
		imagePool:   NewImagePool(db),
		db:          db,
	}
}

// Start 启动所有池
func (m *Manager) Start(ctx context.Context) error {
	log.Info().Msg("Starting pool manager")

	// 启动关键词池
	if err := m.keywordPool.Start(ctx); err != nil {
		return fmt.Errorf("start keyword pool: %w", err)
	}

	// 启动图片池
	if err := m.imagePool.Start(ctx); err != nil {
		m.keywordPool.Stop() // 清理已启动的池
		return fmt.Errorf("start image pool: %w", err)
	}

	log.Info().Msg("Pool manager started successfully")
	return nil
}

// Stop 停止所有池
func (m *Manager) Stop() error {
	log.Info().Msg("Stopping pool manager")

	// 停止关键词池
	if err := m.keywordPool.Stop(); err != nil {
		log.Error().Err(err).Msg("Error stopping keyword pool")
	}

	// 停止图片池
	if err := m.imagePool.Stop(); err != nil {
		log.Error().Err(err).Msg("Error stopping image pool")
	}

	log.Info().Msg("Pool manager stopped")
	return nil
}

// GetKeywordPool 返回关键词池
func (m *Manager) GetKeywordPool() *KeywordPool {
	return m.keywordPool
}

// GetImagePool 返回图片池
func (m *Manager) GetImagePool() *ImagePool {
	return m.imagePool
}

// ReloadAll 重新加载所有池
func (m *Manager) ReloadAll(ctx context.Context) error {
	log.Info().Msg("Reloading all pools")

	// 重新加载关键词池
	kwGroups := make([]int, 0)
	for gid := range m.keywordPool.GetAllGroups() {
		kwGroups = append(kwGroups, gid)
	}
	if err := m.keywordPool.Reload(ctx, kwGroups); err != nil {
		return fmt.Errorf("reload keyword pool: %w", err)
	}

	// 重新加载图片池
	imgGroups := make([]int, 0)
	for gid := range m.imagePool.GetAllGroups() {
		imgGroups = append(imgGroups, gid)
	}
	if err := m.imagePool.Reload(ctx, imgGroups); err != nil {
		return fmt.Errorf("reload image pool: %w", err)
	}

	log.Info().Msg("All pools reloaded successfully")
	return nil
}

// GetStats 获取所有池的统计信息
func (m *Manager) GetStats() map[string]interface{} {
	// 关键词池统计
	kwGroups := m.keywordPool.GetAllGroups()
	kwStats := make(map[int]map[string]interface{})
	for gid := range kwGroups {
		stats := m.keywordPool.GetStats(gid)
		kwStats[gid] = map[string]interface{}{
			"current":       stats.Current,
			"capacity":      stats.Capacity,
			"cache_hits":    stats.CacheHits,
			"cache_misses":  stats.CacheMisses,
			"memory_bytes":  stats.MemoryBytes,
		}
	}

	// 图片池统计
	imgGroups := m.imagePool.GetAllGroups()
	imgStats := make(map[int]map[string]interface{})
	for gid := range imgGroups {
		stats := m.imagePool.GetStats(gid)
		imgStats[gid] = map[string]interface{}{
			"current":       stats.Current,
			"capacity":      stats.Capacity,
			"cache_hits":    stats.CacheHits,
			"cache_misses":  stats.CacheMisses,
			"memory_bytes":  stats.MemoryBytes,
		}
	}

	return map[string]interface{}{
		"keywords": kwStats,
		"images":   imgStats,
		"totals": map[string]interface{}{
			"keyword_count": m.keywordPool.GetTotalCount(),
			"image_count":   m.imagePool.GetTotalCount(),
			"keyword_groups": len(kwGroups),
			"image_groups":   len(imgGroups),
		},
	}
}
