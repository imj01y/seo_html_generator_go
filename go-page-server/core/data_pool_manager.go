package core

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// DataPoolManagerStats 数据池管理器统计
type DataPoolManagerStats struct {
	Keywords     int       `json:"keywords"`
	Images       int       `json:"images"`
	Titles       int       `json:"titles"`
	Contents     int       `json:"contents"`
	SiteCount    int       `json:"site_count"`
	LastRefresh  time.Time `json:"last_refresh"`
	AutoRefresh  bool      `json:"auto_refresh"`
	RefreshCount int64     `json:"refresh_count"`
}

// DataPoolManager 数据池管理器
type DataPoolManager struct {
	db *sql.DB

	// 全局数据池
	keywords *DataPool
	images   *DataPool
	titles   *DataPool
	contents *DataPool

	// 分站点数据池
	siteKeywords map[int]*DataPool
	siteImages   map[int]*DataPool
	siteTitles   map[int]*DataPool
	siteContents map[int]*DataPool

	mu sync.RWMutex

	// 自动刷新
	refreshInterval time.Duration
	stopChan        chan struct{}
	running         atomic.Bool // 使用 atomic.Bool 避免竞态

	// 统计
	lastRefresh  time.Time
	refreshCount atomic.Int64
}

// NewDataPoolManager 创建管理器
func NewDataPoolManager(db *sql.DB, refreshInterval time.Duration) *DataPoolManager {
	return &DataPoolManager{
		db:              db,
		keywords:        NewDataPool("global_keywords"),
		images:          NewDataPool("global_images"),
		titles:          NewDataPool("global_titles"),
		contents:        NewDataPool("global_contents"),
		siteKeywords:    make(map[int]*DataPool),
		siteImages:      make(map[int]*DataPool),
		siteTitles:      make(map[int]*DataPool),
		siteContents:    make(map[int]*DataPool),
		refreshInterval: refreshInterval,
	}
}

// LoadAll 加载所有数据池
func (m *DataPoolManager) LoadAll(ctx context.Context) error {
	log.Info().Msg("Loading all data pools...")

	var errs []error

	if err := m.loadKeywords(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load keywords")
		errs = append(errs, err)
	}

	if err := m.loadImages(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load images")
		errs = append(errs, err)
	}

	if err := m.loadTitles(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load titles")
		errs = append(errs, err)
	}

	if err := m.loadContents(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load contents")
		errs = append(errs, err)
	}

	m.lastRefresh = time.Now()

	if len(errs) > 0 {
		return fmt.Errorf("failed to load some data pools: %d errors", len(errs))
	}

	log.Info().Msg("All data pools loaded successfully")
	return nil
}

// loadKeywords 加载关键词数据池
func (m *DataPoolManager) loadKeywords(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT word FROM keywords WHERE status = 1`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query keywords: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return fmt.Errorf("scan keyword: %w", err)
		}
		items = append(items, word)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate keywords: %w", err)
	}

	m.keywords.Load(items)
	return nil
}

// loadImages 加载图片数据池
func (m *DataPoolManager) loadImages(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT url FROM images WHERE status = 1`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query images: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return fmt.Errorf("scan image url: %w", err)
		}
		items = append(items, url)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate images: %w", err)
	}

	m.images.Load(items)
	return nil
}

// loadTitles 加载标题数据池
func (m *DataPoolManager) loadTitles(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT title FROM titles WHERE status = 1`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query titles: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return fmt.Errorf("scan title: %w", err)
		}
		items = append(items, title)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate titles: %w", err)
	}

	m.titles.Load(items)
	return nil
}

// loadContents 加载内容数据池
func (m *DataPoolManager) loadContents(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT content FROM contents WHERE status = 1`
	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query contents: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return fmt.Errorf("scan content: %w", err)
		}
		items = append(items, content)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate contents: %w", err)
	}

	m.contents.Load(items)
	return nil
}

// LoadSiteData 加载站点专属数据
func (m *DataPoolManager) LoadSiteData(ctx context.Context, siteID int) error {
	log.Info().Int("site_id", siteID).Msg("Loading site-specific data...")

	m.mu.Lock()
	defer m.mu.Unlock()

	// 加载站点关键词
	if err := m.loadSiteKeywords(ctx, siteID); err != nil {
		log.Warn().Err(err).Int("site_id", siteID).Msg("Failed to load site keywords")
	}

	// 加载站点图片
	if err := m.loadSiteImages(ctx, siteID); err != nil {
		log.Warn().Err(err).Int("site_id", siteID).Msg("Failed to load site images")
	}

	// 加载站点标题
	if err := m.loadSiteTitles(ctx, siteID); err != nil {
		log.Warn().Err(err).Int("site_id", siteID).Msg("Failed to load site titles")
	}

	// 加载站点内容
	if err := m.loadSiteContents(ctx, siteID); err != nil {
		log.Warn().Err(err).Int("site_id", siteID).Msg("Failed to load site contents")
	}

	log.Info().Int("site_id", siteID).Msg("Site-specific data loaded")
	return nil
}

// loadSiteKeywords 加载站点关键词
func (m *DataPoolManager) loadSiteKeywords(ctx context.Context, siteID int) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT word FROM site_keywords WHERE site_id = ? AND status = 1`
	rows, err := m.db.QueryContext(ctx, query, siteID)
	if err != nil {
		return fmt.Errorf("query site keywords: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var word string
		if err := rows.Scan(&word); err != nil {
			return fmt.Errorf("scan site keyword: %w", err)
		}
		items = append(items, word)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate site keywords: %w", err)
	}

	if len(items) > 0 {
		pool := NewDataPool(fmt.Sprintf("site_%d_keywords", siteID))
		pool.Load(items)
		m.siteKeywords[siteID] = pool
	}

	return nil
}

// loadSiteImages 加载站点图片
func (m *DataPoolManager) loadSiteImages(ctx context.Context, siteID int) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT url FROM site_images WHERE site_id = ? AND status = 1`
	rows, err := m.db.QueryContext(ctx, query, siteID)
	if err != nil {
		return fmt.Errorf("query site images: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var url string
		if err := rows.Scan(&url); err != nil {
			return fmt.Errorf("scan site image url: %w", err)
		}
		items = append(items, url)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate site images: %w", err)
	}

	if len(items) > 0 {
		pool := NewDataPool(fmt.Sprintf("site_%d_images", siteID))
		pool.Load(items)
		m.siteImages[siteID] = pool
	}

	return nil
}

// loadSiteTitles 加载站点标题
func (m *DataPoolManager) loadSiteTitles(ctx context.Context, siteID int) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT title FROM site_titles WHERE site_id = ? AND status = 1`
	rows, err := m.db.QueryContext(ctx, query, siteID)
	if err != nil {
		return fmt.Errorf("query site titles: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return fmt.Errorf("scan site title: %w", err)
		}
		items = append(items, title)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate site titles: %w", err)
	}

	if len(items) > 0 {
		pool := NewDataPool(fmt.Sprintf("site_%d_titles", siteID))
		pool.Load(items)
		m.siteTitles[siteID] = pool
	}

	return nil
}

// loadSiteContents 加载站点内容
func (m *DataPoolManager) loadSiteContents(ctx context.Context, siteID int) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	query := `SELECT content FROM site_contents WHERE site_id = ? AND status = 1`
	rows, err := m.db.QueryContext(ctx, query, siteID)
	if err != nil {
		return fmt.Errorf("query site contents: %w", err)
	}
	defer rows.Close()

	var items []string
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return fmt.Errorf("scan site content: %w", err)
		}
		items = append(items, content)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate site contents: %w", err)
	}

	if len(items) > 0 {
		pool := NewDataPool(fmt.Sprintf("site_%d_contents", siteID))
		pool.Load(items)
		m.siteContents[siteID] = pool
	}

	return nil
}

// GetKeyword 获取关键词（优先站点专属）
func (m *DataPoolManager) GetKeyword(siteID int) string {
	m.mu.RLock()
	sitePool, exists := m.siteKeywords[siteID]
	m.mu.RUnlock()

	if exists && sitePool.Count() > 0 {
		return sitePool.Get()
	}

	return m.keywords.Get()
}

// GetImage 获取图片（优先站点专属）
func (m *DataPoolManager) GetImage(siteID int) string {
	m.mu.RLock()
	sitePool, exists := m.siteImages[siteID]
	m.mu.RUnlock()

	if exists && sitePool.Count() > 0 {
		return sitePool.Get()
	}

	return m.images.Get()
}

// GetTitle 获取标题（优先站点专属）
func (m *DataPoolManager) GetTitle(siteID int) string {
	m.mu.RLock()
	sitePool, exists := m.siteTitles[siteID]
	m.mu.RUnlock()

	if exists && sitePool.Count() > 0 {
		return sitePool.Get()
	}

	return m.titles.Get()
}

// GetContent 获取内容（优先站点专属）
func (m *DataPoolManager) GetContent(siteID int) string {
	m.mu.RLock()
	sitePool, exists := m.siteContents[siteID]
	m.mu.RUnlock()

	if exists && sitePool.Count() > 0 {
		return sitePool.Get()
	}

	return m.contents.Get()
}

// StartAutoRefresh 启动自动刷新
func (m *DataPoolManager) StartAutoRefresh() {
	if m.running.Swap(true) {
		// 已经在运行
		return
	}

	m.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(m.refreshInterval)
		defer ticker.Stop()

		log.Info().Dur("interval", m.refreshInterval).Msg("Auto refresh started")

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				if err := m.LoadAll(ctx); err != nil {
					log.Error().Err(err).Msg("Auto refresh failed")
				} else {
					m.refreshCount.Add(1)
					log.Info().Int64("count", m.refreshCount.Load()).Msg("Auto refresh completed")
				}
				cancel()
			case <-m.stopChan:
				log.Info().Msg("Auto refresh stopped")
				return
			}
		}
	}()
}

// StopAutoRefresh 停止自动刷新
func (m *DataPoolManager) StopAutoRefresh() {
	if !m.running.Swap(false) {
		// 已经停止
		return
	}

	// 关闭 channel 通知 goroutine 退出
	close(m.stopChan)
}

// GetStats 获取统计
func (m *DataPoolManager) GetStats() DataPoolManagerStats {
	m.mu.RLock()
	siteCount := len(m.siteKeywords)
	m.mu.RUnlock()

	return DataPoolManagerStats{
		Keywords:     m.keywords.Count(),
		Images:       m.images.Count(),
		Titles:       m.titles.Count(),
		Contents:     m.contents.Count(),
		SiteCount:    siteCount,
		LastRefresh:  m.lastRefresh,
		AutoRefresh:  m.running.Load(),
		RefreshCount: m.refreshCount.Load(),
	}
}

// GetDetailedStats 获取详细统计
func (m *DataPoolManager) GetDetailedStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"global": map[string]interface{}{
			"keywords": m.keywords.Stats(),
			"images":   m.images.Stats(),
			"titles":   m.titles.Stats(),
			"contents": m.contents.Stats(),
		},
		"sites":         make(map[int]map[string]interface{}),
		"last_refresh":  m.lastRefresh,
		"auto_refresh":  m.running.Load(),
		"refresh_count": m.refreshCount.Load(),
	}

	sites := stats["sites"].(map[int]map[string]interface{})

	// 收集所有站点 ID
	siteIDs := make(map[int]bool)
	for id := range m.siteKeywords {
		siteIDs[id] = true
	}
	for id := range m.siteImages {
		siteIDs[id] = true
	}
	for id := range m.siteTitles {
		siteIDs[id] = true
	}
	for id := range m.siteContents {
		siteIDs[id] = true
	}

	for siteID := range siteIDs {
		siteStats := make(map[string]interface{})

		if pool, ok := m.siteKeywords[siteID]; ok {
			siteStats["keywords"] = pool.Stats()
		}
		if pool, ok := m.siteImages[siteID]; ok {
			siteStats["images"] = pool.Stats()
		}
		if pool, ok := m.siteTitles[siteID]; ok {
			siteStats["titles"] = pool.Stats()
		}
		if pool, ok := m.siteContents[siteID]; ok {
			siteStats["contents"] = pool.Stats()
		}

		sites[siteID] = siteStats
	}

	return stats
}

// Refresh 立即刷新指定池
func (m *DataPoolManager) Refresh(ctx context.Context, poolName string) error {
	switch poolName {
	case "keywords":
		return m.loadKeywords(ctx)
	case "images":
		return m.loadImages(ctx)
	case "titles":
		return m.loadTitles(ctx)
	case "contents":
		return m.loadContents(ctx)
	case "all":
		return m.LoadAll(ctx)
	default:
		return fmt.Errorf("unknown pool name: %s", poolName)
	}
}
