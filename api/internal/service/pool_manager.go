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

	"seo-generator/api/internal/service/pool"
)

// ErrCachePoolEmpty is returned when the cache pool is empty
var ErrCachePoolEmpty = errors.New("cache pool is empty")

// PoolManager manages memory pools for titles, contents, keywords, and images
// 注意: 关键词和图片池已重构到 pool 子包, 此处作为兼容层
type PoolManager struct {
	// 消费型池（FIFO，消费后标记）
	titles   map[int]*MemoryPool // groupID -> pool
	contents map[int]*MemoryPool // groupID -> pool

	// 标题生成器
	titleGenerator *TitleGenerator

	// 复用型池管理器（新架构）
	poolManager *pool.Manager

	// 辅助组件
	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager

	// 配置和数据库
	config *CachePoolConfig
	db     *sqlx.DB
	mu     sync.RWMutex

	// 后台任务
	ctx     context.Context
	cancel  context.CancelFunc
	batcher *pool.UpdateBatcher // 批量更新器（替代 updateCh）
	wg      sync.WaitGroup
	stopped atomic.Bool

	// 状态追踪
	lastRefresh time.Time
}

// PoolGroupInfo 分组详情
type PoolGroupInfo struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Count       int     `json:"count"`
	Size        int     `json:"size,omitempty"`
	Available   int     `json:"available,omitempty"`
	Used        int     `json:"used,omitempty"`
	Utilization float64 `json:"utilization,omitempty"`
	MemoryBytes int64   `json:"memory_bytes,omitempty"`
}

// PoolStatusStats 数据池运行状态统计（用于前端显示）
type PoolStatusStats struct {
	Name        string     `json:"name"`
	Size        int        `json:"size"`
	Available   int        `json:"available"`
	Used        int        `json:"used"`
	Utilization float64    `json:"utilization"`
	Status      string     `json:"status"`
	NumWorkers  int        `json:"num_workers"`
	LastRefresh *time.Time `json:"last_refresh"`
	MemoryBytes int64      `json:"memory_bytes"` // 内存占用（字节）
	// 新增字段（复用型池使用）
	PoolType string          `json:"pool_type"`        // "consumable" | "reusable" | "static"
	Groups   []PoolGroupInfo `json:"groups,omitempty"` // 分组详情（复用型池）
	Source   string          `json:"source,omitempty"` // 数据来源（表情库）
}

// NewPoolManager creates a new pool manager
func NewPoolManager(db *sqlx.DB) *PoolManager {
	ctx, cancel := context.WithCancel(context.Background())

	// 配置批量更新器：最多 100 条记录或 5 秒刷新一次
	batcherConfig := pool.BatcherConfig{
		MaxBatch:      100,
		FlushInterval: 5 * time.Second,
	}

	return &PoolManager{
		titles:       make(map[int]*MemoryPool),
		contents:     make(map[int]*MemoryPool),
		poolManager:  pool.NewManager(db),
		encoder:      GetEncoder(),
		emojiManager: NewEmojiManager(),
		config:       DefaultCachePoolConfig(),
		db:           db,
		ctx:          ctx,
		cancel:       cancel,
		batcher:      pool.NewUpdateBatcher(db, batcherConfig),
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

	// Discover and initialize pools for all groups (titles/contents)
	groupIDs, err := m.discoverGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to discover groups: %w", err)
	}

	for _, gid := range groupIDs {
		m.getOrCreatePool("contents", gid)
	}

	// Initial fill for titles/contents
	m.checkAndRefillAll()

	// Start pool manager (keywords and images)
	if err := m.poolManager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start pool manager: %w", err)
	}

	// Get keyword groups for title generator
	keywordGroupIDs := make([]int, 0)
	for gid := range m.poolManager.GetKeywordPool().GetAllGroups() {
		keywordGroupIDs = append(keywordGroupIDs, gid)
	}
	if len(keywordGroupIDs) == 0 {
		keywordGroupIDs = []int{1}
	}

	// 初始化并启动 TitleGenerator（必须在关键词加载完成后）
	m.titleGenerator = NewTitleGenerator(m, m.config)
	m.titleGenerator.Start(keywordGroupIDs)

	// Set initial lastRefresh time
	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	// Start background workers
	m.wg.Add(1)
	go m.refillLoop()
	// updateWorker 已替换为 UpdateBatcher（自动批量处理）

	imageGroupCount := len(m.poolManager.GetImagePool().GetAllGroups())

	log.Info().
		Int("article_groups", len(groupIDs)).
		Int("keyword_groups", len(keywordGroupIDs)).
		Int("image_groups", imageGroupCount).
		Int("title_pool_size", m.config.TitlePoolSize).
		Int("content_pool_size", m.config.ContentPoolSize).
		Msg("PoolManager started")

	return nil
}

// Stop stops the pool manager gracefully
func (m *PoolManager) Stop() {
	m.stopped.Store(true)
	m.cancel()
	if m.batcher != nil {
		m.batcher.Stop() // 刷新并关闭批量更新器
	}
	if m.titleGenerator != nil {
		m.titleGenerator.Stop()
	}
	if m.poolManager != nil {
		m.poolManager.Stop()
	}
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

	// 只处理 contents 类型，titles 已改用 TitleGenerator
	pools := m.contents
	maxSize := m.config.ContentPoolSize

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
	// titles 使用 TitleGenerator
	if poolType == "titles" {
		if m.titleGenerator == nil {
			return "", ErrCachePoolEmpty
		}
		return m.titleGenerator.Pop(groupID)
	}

	if err := validatePoolType(poolType); err != nil {
		return "", err
	}

	memPool := m.getOrCreatePool(poolType, groupID)
	item, ok := memPool.Pop()
	if !ok {
		// Try to refill and pop again
		m.refillPool(memPool)
		item, ok = memPool.Pop()
		if !ok {
			return "", ErrCachePoolEmpty
		}
	}

	// Async batch update status (never drops messages)
	if !m.stopped.Load() && m.batcher != nil {
		m.batcher.Add(pool.UpdateTask{Table: poolType, ID: item.ID})
	}

	return item.Text, nil
}

// refillLoop runs the background refill check
func (m *PoolManager) refillLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.ContentRefillInterval())
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

// checkAndRefillAll checks and refills all content pools
func (m *PoolManager) checkAndRefillAll() {
	m.mu.RLock()
	contentPools := make([]*MemoryPool, 0, len(m.contents))
	for _, p := range m.contents {
		contentPools = append(contentPools, p)
	}
	poolSize := m.config.ContentPoolSize
	thresholdRatio := m.config.ContentThreshold
	m.mu.RUnlock()

	// 计算阈值：池大小 * 阈值比例
	threshold := int(float64(poolSize) * thresholdRatio)

	for _, pool := range contentPools {
		if pool.Len() < threshold {
			m.refillPool(pool)
		}
	}
}

// refillPool refills a single pool from database
func (m *PoolManager) refillPool(memPool *MemoryPool) {
	poolType := memPool.GetPoolType()
	groupID := memPool.GetGroupID()
	currentLen := memPool.Len()
	maxSize := memPool.GetMaxSize()
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
		added := memPool.Push(items)

		if added > 0 {
			log.Info().
				Str("type", poolType).
				Int("group", groupID).
				Int("added", added).
				Int("total", memPool.Len()).
				Msg("Pool refilled")
		}
	} else {
		log.Info().
			Str("type", poolType).
			Int("group", groupID).
			Int("need", need).
			Msg("No items to refill")
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

	// Resize content pools if needed
	if config.ContentPoolSize != oldConfig.ContentPoolSize {
		for _, pool := range m.contents {
			pool.Resize(config.ContentPoolSize)
		}
	}
	m.mu.Unlock()

	// Reload TitleGenerator config
	if m.titleGenerator != nil {
		m.titleGenerator.Reload(config)
	}

	log.Info().
		Int("title_pool_size", config.TitlePoolSize).
		Int("title_workers", config.TitleWorkers).
		Int("content_pool_size", config.ContentPoolSize).
		Int("content_workers", config.ContentWorkers).
		Msg("PoolManager config reloaded")

	return nil
}

// GetStats returns pool statistics
func (m *PoolManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	titlesStats := make(map[int]map[string]interface{})
	contentsStats := make(map[int]map[string]interface{})
	for gid, pool := range m.titles {
		titlesStats[gid] = map[string]interface{}{
			"current":   pool.Len(),
			"max_size":  pool.GetMaxSize(),
			"threshold": m.config.TitleThreshold,
		}
	}
	for gid, pool := range m.contents {
		contentsStats[gid] = map[string]interface{}{
			"current":   pool.Len(),
			"max_size":  pool.GetMaxSize(),
			"threshold": m.config.ContentThreshold,
		}
	}
	m.mu.RUnlock()

	// 从新的池管理器获取统计
	keywordsStats := m.poolManager.GetKeywordPool().GetAllGroups()
	imagesStats := m.poolManager.GetImagePool().GetAllGroups()

	return map[string]interface{}{
		"titles":   titlesStats,
		"contents": contentsStats,
		"keywords": keywordsStats,
		"images":   imagesStats,
		"emojis":   m.emojiManager.Count(),
		"config": map[string]interface{}{
			"title_pool_size":            m.config.TitlePoolSize,
			"title_workers":              m.config.TitleWorkers,
			"title_refill_interval_ms":   m.config.TitleRefillIntervalMs,
			"title_threshold":            m.config.TitleThreshold,
			"content_pool_size":          m.config.ContentPoolSize,
			"content_workers":            m.config.ContentWorkers,
			"content_refill_interval_ms": m.config.ContentRefillIntervalMs,
			"content_threshold":          m.config.ContentThreshold,
		},
	}
}

// GetConfig returns the current configuration
func (m *PoolManager) GetConfig() *CachePoolConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ============================================================
// Keywords 方法
// ============================================================

// LoadKeywords loads keywords for a group from the database
// 兼容层: 代理到 pool.KeywordPool
func (m *PoolManager) LoadKeywords(ctx context.Context, groupID int) (int, error) {
	if err := m.poolManager.GetKeywordPool().ReloadGroup(ctx, groupID); err != nil {
		return 0, err
	}
	count := m.poolManager.GetKeywordPool().GetGroupCount(groupID)
	return count, nil
}

// GetRandomKeywords returns random pre-encoded keywords
// 兼容层: 代理到 pool.KeywordPool
func (m *PoolManager) GetRandomKeywords(groupID int, count int) []string {
	return m.poolManager.GetKeywordPool().GetRandomKeywords(groupID, count)
}

// GetRawKeywords returns raw (not encoded) keywords
// 兼容层: 代理到 pool.KeywordPool
func (m *PoolManager) GetRawKeywords(groupID int, count int) []string {
	return m.poolManager.GetKeywordPool().GetRawKeywords(groupID, count)
}

// AppendKeywords 追加关键词到内存（新增时调用）
// 兼容层: 代理到 pool.KeywordPool
func (m *PoolManager) AppendKeywords(groupID int, keywords []string) {
	m.poolManager.GetKeywordPool().AppendKeywords(groupID, keywords)
}

// ReloadKeywordGroup 重载指定分组的关键词缓存（删除时调用）
// 兼容层: 代理到 pool.KeywordPool
func (m *PoolManager) ReloadKeywordGroup(ctx context.Context, groupID int) error {
	if err := m.poolManager.GetKeywordPool().ReloadGroup(ctx, groupID); err != nil {
		return err
	}
	// 同步 TitleGenerator 分组
	if m.titleGenerator != nil {
		m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
	}
	return nil
}

// GetKeywordGroupIDs 返回所有关键词分组ID
func (m *PoolManager) GetKeywordGroupIDs() []int {
	groups := m.poolManager.GetKeywordPool().GetAllGroups()
	ids := make([]int, 0, len(groups))
	for gid := range groups {
		ids = append(ids, gid)
	}
	return ids
}

// GetKeywords 获取指定分组的所有编码关键词
func (m *PoolManager) GetKeywords(groupID int) []string {
	return m.poolManager.GetKeywordPool().GetKeywords(groupID)
}

// GetAllRawKeywords 获取指定分组的所有原始关键词
func (m *PoolManager) GetAllRawKeywords(groupID int) []string {
	return m.poolManager.GetKeywordPool().GetAllRawKeywords(groupID)
}

// ============================================================
// Images 方法
// ============================================================

// LoadImages loads image URLs for a group from the database
// 兼容层: 代理到 pool.ImagePool
func (m *PoolManager) LoadImages(ctx context.Context, groupID int) (int, error) {
	if err := m.poolManager.GetImagePool().ReloadGroup(ctx, groupID); err != nil {
		return 0, err
	}
	count := m.poolManager.GetImagePool().GetGroupCount(groupID)
	return count, nil
}

// GetRandomImage returns a random image URL
// 兼容层: 代理到 pool.ImagePool
func (m *PoolManager) GetRandomImage(groupID int) string {
	return m.poolManager.GetImagePool().GetRandomImage(groupID)
}

// GetImages returns all image URLs for a group
// 兼容层: 代理到 pool.ImagePool
func (m *PoolManager) GetImages(groupID int) []string {
	return m.poolManager.GetImagePool().GetImages(groupID)
}

// GetImageGroupIDs 返回所有图片分组ID
func (m *PoolManager) GetImageGroupIDs() []int {
	groups := m.poolManager.GetImagePool().GetAllGroups()
	ids := make([]int, 0, len(groups))
	for gid := range groups {
		ids = append(ids, gid)
	}
	return ids
}

// AppendImages 追加图片到内存（新增时调用）
// 兼容层: 代理到 pool.ImagePool
func (m *PoolManager) AppendImages(groupID int, urls []string) {
	m.poolManager.GetImagePool().AppendImages(groupID, urls)
}

// ReloadImageGroup 重载指定分组的图片缓存（删除时调用）
// 兼容层: 代理到 pool.ImagePool
func (m *PoolManager) ReloadImageGroup(ctx context.Context, groupID int) error {
	return m.poolManager.GetImagePool().ReloadGroup(ctx, groupID)
}

// ============================================================
// Emoji 方法
// ============================================================

// LoadEmojis loads emojis from a JSON file
func (m *PoolManager) LoadEmojis(path string) error {
	return m.emojiManager.LoadFromFile(path)
}

// GetRandomEmoji returns a random emoji
func (m *PoolManager) GetRandomEmoji() string {
	return m.emojiManager.GetRandom()
}

// GetRandomEmojiExclude returns a random emoji not in the exclude set
func (m *PoolManager) GetRandomEmojiExclude(exclude map[string]bool) string {
	return m.emojiManager.GetRandomExclude(exclude)
}

// GetEmojiCount returns the number of loaded emojis
func (m *PoolManager) GetEmojiCount() int {
	return m.emojiManager.Count()
}

// ReloadEmojis 重载表情库
func (m *PoolManager) ReloadEmojis(path string) error {
	return m.emojiManager.LoadFromFile(path)
}

// ============================================================
// 分组发现和刷新循环
// ============================================================

// discoverKeywordGroups finds all keyword group IDs
// 兼容层: 使用池管理器的数据
func (m *PoolManager) discoverKeywordGroups(ctx context.Context) ([]int, error) {
	groups := make([]int, 0)
	for gid := range m.poolManager.GetKeywordPool().GetAllGroups() {
		groups = append(groups, gid)
	}
	if len(groups) == 0 {
		return []int{1}, nil
	}
	return groups, nil
}

// discoverImageGroups finds all image group IDs
// 兼容层: 使用池管理器的数据
func (m *PoolManager) discoverImageGroups(ctx context.Context) ([]int, error) {
	groups := make([]int, 0)
	for gid := range m.poolManager.GetImagePool().GetAllGroups() {
		groups = append(groups, gid)
	}
	if len(groups) == 0 {
		return []int{1}, nil
	}
	return groups, nil
}

// ============================================================
// 兼容性方法（供 router/websocket 使用）
// ============================================================

// getKeywordGroupNames 从数据库获取关键词分组名称映射
func (m *PoolManager) getKeywordGroupNames() map[int]string {
	names := make(map[int]string)
	rows, err := m.db.QueryContext(m.ctx, "SELECT id, name FROM keyword_groups")
	if err != nil {
		return names
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			names[id] = name
		}
	}
	return names
}

// getImageGroupNames 从数据库获取图片分组名称映射
func (m *PoolManager) getImageGroupNames() map[int]string {
	names := make(map[int]string)
	rows, err := m.db.QueryContext(m.ctx, "SELECT id, name FROM image_groups")
	if err != nil {
		return names
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err == nil {
			names[id] = name
		}
	}
	return names
}

// getContentGroupNames 获取正文/标题分组名称映射
// 标题和正文的 group_id 对应 keyword_groups 表的 id
func (m *PoolManager) getContentGroupNames() map[int]string {
	return m.getKeywordGroupNames()
}

// GetDataPoolsStats 返回数据池运行状态统计（与前端展示格式一致）
// 返回全部 5 个池：标题、正文、关键词、图片、表情
func (m *PoolManager) GetDataPoolsStats() []PoolStatusStats {
	m.mu.RLock()
	lastRefresh := m.lastRefresh
	stopped := m.stopped.Load()
	m.mu.RUnlock()

	status := "running"
	if stopped {
		status = "stopped"
	}

	var lastRefreshPtr *time.Time
	if !lastRefresh.IsZero() {
		lastRefreshPtr = &lastRefresh
	}

	pools := []PoolStatusStats{}

	// 1. 标题池（改用 TitleGenerator 统计）
	groupNames := m.getContentGroupNames()

	var titlesCurrent, titlesMax int
	var titlesMemory, titlesConsumed int64
	var titleGroups []PoolGroupInfo
	if m.titleGenerator != nil {
		titlesCurrent, titlesMax, titlesMemory, titlesConsumed = m.titleGenerator.GetTotalStats()
		titleGroups = m.titleGenerator.GetGroupStats()
		// 填充分组名称
		for i := range titleGroups {
			if name, ok := groupNames[titleGroups[i].ID]; ok {
				titleGroups[i].Name = name
			} else {
				titleGroups[i].Name = fmt.Sprintf("分组%d", titleGroups[i].ID)
			}
		}
	}
	titlesUsed := int(titlesConsumed)
	titlesUtil := 0.0
	if titlesMax > 0 {
		titlesUtil = float64(titlesCurrent) / float64(titlesMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "标题",
		Size:        titlesMax,
		Available:   titlesCurrent,
		Used:        titlesUsed,
		Utilization: titlesUtil,
		Status:      status,
		NumWorkers:  m.config.TitleWorkers,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: titlesMemory,
		PoolType:    "consumable",
		Groups:      titleGroups,
	})

	// 2. 正文池（消费型，汇总所有分组 + 分组详情）
	m.mu.RLock()
	var contentsMax, contentsCurrent int
	var contentsMemory int64
	var contentsConsumed int64
	contentGroups := make([]PoolGroupInfo, 0, len(m.contents))
	for gid, pool := range m.contents {
		current := pool.Len()
		maxSize := pool.GetMaxSize()
		consumed := int(pool.ConsumedCount())
		mem := pool.MemoryBytes()

		contentsMax += maxSize
		contentsCurrent += current
		contentsMemory += mem
		contentsConsumed += pool.ConsumedCount()

		util := 0.0
		if maxSize > 0 {
			util = float64(current) / float64(maxSize) * 100
		}
		name := groupNames[gid]
		if name == "" {
			name = fmt.Sprintf("分组%d", gid)
		}
		contentGroups = append(contentGroups, PoolGroupInfo{
			ID:          gid,
			Name:        name,
			Count:       current,
			Size:        maxSize,
			Available:   current,
			Used:        consumed,
			Utilization: util,
			MemoryBytes: mem,
		})
	}
	m.mu.RUnlock()

	contentsUsed := int(contentsConsumed)
	contentsUtil := 0.0
	if contentsMax > 0 {
		contentsUtil = float64(contentsCurrent) / float64(contentsMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "正文",
		Size:        contentsMax,
		Available:   contentsCurrent,
		Used:        contentsUsed,
		Utilization: contentsUtil,
		Status:      status,
		NumWorkers:  1,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: contentsMemory,
		PoolType:    "consumable",
		Groups:      contentGroups,
	})

	// 3. 关键词（复用型，增加分组详情）
	keywordGroupNames := m.getKeywordGroupNames()
	kwGroups := m.poolManager.GetKeywordPool().GetAllGroups()
	var totalKeywords int
	keywordGroups := []PoolGroupInfo{}
	for gid, count := range kwGroups {
		totalKeywords += count
		name := keywordGroupNames[gid]
		if name == "" {
			name = fmt.Sprintf("分组%d", gid)
		}
		keywordGroups = append(keywordGroups, PoolGroupInfo{
			ID:    gid,
			Name:  name,
			Count: count,
		})
	}
	// 获取内存统计
	kwStats := m.poolManager.GetKeywordPool().GetStats(0)
	keywordsMemory := kwStats.MemoryBytes
	pools = append(pools, PoolStatusStats{
		Name:        "关键词",
		Size:        totalKeywords,
		Available:   totalKeywords,
		Used:        0,
		Utilization: 100,
		Status:      status,
		NumWorkers:  0,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: keywordsMemory,
		PoolType:    "reusable",
		Groups:      keywordGroups,
	})

	// 4. 图片（复用型，增加分组详情）
	imageGroupNames := m.getImageGroupNames()
	imgGroups := m.poolManager.GetImagePool().GetAllGroups()
	var totalImages int
	imageGroups := []PoolGroupInfo{}
	for gid, count := range imgGroups {
		totalImages += count
		name := imageGroupNames[gid]
		if name == "" {
			name = fmt.Sprintf("分组%d", gid)
		}
		imageGroups = append(imageGroups, PoolGroupInfo{
			ID:    gid,
			Name:  name,
			Count: count,
		})
	}
	// 获取内存统计
	imgStats := m.poolManager.GetImagePool().GetStats(0)
	imagesMemory := imgStats.MemoryBytes
	pools = append(pools, PoolStatusStats{
		Name:        "图片",
		Size:        totalImages,
		Available:   totalImages,
		Used:        0,
		Utilization: 100,
		Status:      status,
		NumWorkers:  0,
		LastRefresh: lastRefreshPtr,
		MemoryBytes: imagesMemory,
		PoolType:    "reusable",
		Groups:      imageGroups,
	})

	// 5. 表情库（静态数据）
	emojiCount := m.emojiManager.Count()
	emojiMemory := m.emojiManager.MemoryBytes()
	pools = append(pools, PoolStatusStats{
		Name:        "表情",
		Size:        emojiCount,
		Available:   emojiCount,
		Used:        0,
		Utilization: 100,
		Status:      status,
		NumWorkers:  0,
		LastRefresh: nil,
		MemoryBytes: emojiMemory,
		PoolType:    "static",
		Source:      "emojis.json",
	})

	return pools
}

// SimplePoolStats 简化的池统计（用于健康检查）
type SimplePoolStats struct {
	Keywords int `json:"keywords"`
	Images   int `json:"images"`
}

// GetPoolStatsSimple 返回简化的池统计
// 兼容层: 使用池管理器的数据
func (m *PoolManager) GetPoolStatsSimple() SimplePoolStats {
	return SimplePoolStats{
		Keywords: m.poolManager.GetKeywordPool().GetTotalCount(),
		Images:   m.poolManager.GetImagePool().GetTotalCount(),
	}
}

// RefreshData 手动刷新指定数据池
// 兼容层: 使用池管理器重新加载
func (m *PoolManager) RefreshData(ctx context.Context, poolType string) error {
	switch poolType {
	case "keywords":
		groupIDs, _ := m.discoverKeywordGroups(ctx)
		if err := m.poolManager.GetKeywordPool().Reload(ctx, groupIDs); err != nil {
			return fmt.Errorf("reload keywords: %w", err)
		}
		if m.titleGenerator != nil {
			m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
		}
	case "images":
		groupIDs, _ := m.discoverImageGroups(ctx)
		if err := m.poolManager.GetImagePool().Reload(ctx, groupIDs); err != nil {
			return fmt.Errorf("reload images: %w", err)
		}
	case "all":
		if err := m.poolManager.ReloadAll(ctx); err != nil {
			return fmt.Errorf("reload all pools: %w", err)
		}
		if m.titleGenerator != nil {
			m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
		}
	}

	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	return nil
}
