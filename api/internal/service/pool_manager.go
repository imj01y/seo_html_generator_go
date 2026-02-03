// api/internal/service/pool_manager.go
package core

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// ErrCachePoolEmpty is returned when the cache pool is empty
var ErrCachePoolEmpty = errors.New("cache pool is empty")

// PoolManager manages memory pools for titles, contents, keywords, and images
type PoolManager struct {
	// 消费型池（FIFO，消费后标记）
	titles   map[int]*MemoryPool // groupID -> pool
	contents map[int]*MemoryPool // groupID -> pool

	// 标题生成器（新增）
	titleGenerator *TitleGenerator

	// 复用型数据（随机获取，可重复）
	keywords    map[int][]string // groupID -> encoded keywords
	rawKeywords map[int][]string // groupID -> raw keywords
	images      map[int][]string // groupID -> image URLs
	keywordsMu  sync.RWMutex
	imagesMu    sync.RWMutex

	// 辅助组件
	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager

	// 配置和数据库
	config *CachePoolConfig
	db     *sqlx.DB
	mu     sync.RWMutex

	// 后台任务
	ctx      context.Context
	cancel   context.CancelFunc
	updateCh chan UpdateTask
	wg       sync.WaitGroup
	stopped  atomic.Bool

	// 状态追踪
	lastRefresh time.Time
}

// PoolGroupInfo 分组详情
type PoolGroupInfo struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
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
	// 新增字段（复用型池使用）
	PoolType string          `json:"pool_type"`        // "consumable" | "reusable" | "static"
	Groups   []PoolGroupInfo `json:"groups,omitempty"` // 分组详情（复用型池）
	Source   string          `json:"source,omitempty"` // 数据来源（表情库）
}

// NewPoolManager creates a new pool manager
func NewPoolManager(db *sqlx.DB) *PoolManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolManager{
		titles:       make(map[int]*MemoryPool),
		contents:     make(map[int]*MemoryPool),
		keywords:     make(map[int][]string),
		rawKeywords:  make(map[int][]string),
		images:       make(map[int][]string),
		encoder:      GetEncoder(),
		emojiManager: NewEmojiManager(),
		config:       DefaultCachePoolConfig(),
		db:           db,
		ctx:          ctx,
		cancel:       cancel,
		updateCh:     make(chan UpdateTask, 1000),
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
		m.getOrCreatePool("titles", gid)
		m.getOrCreatePool("contents", gid)
	}

	// Initial fill for titles/contents
	m.checkAndRefillAll()

	// Discover and load keywords/images
	keywordGroupIDs, _ := m.discoverKeywordGroups(ctx)
	imageGroupIDs, _ := m.discoverImageGroups(ctx)

	for _, gid := range keywordGroupIDs {
		if _, err := m.LoadKeywords(ctx, gid); err != nil {
			log.Warn().Err(err).Int("group", gid).Msg("Failed to load keywords")
		}
	}
	for _, gid := range imageGroupIDs {
		if _, err := m.LoadImages(ctx, gid); err != nil {
			log.Warn().Err(err).Int("group", gid).Msg("Failed to load images")
		}
	}

	// 初始化并启动 TitleGenerator（必须在关键词加载完成后）
	m.titleGenerator = NewTitleGenerator(m, m.config)
	m.titleGenerator.Start(keywordGroupIDs)

	// Set initial lastRefresh time
	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	// Start background workers
	m.wg.Add(2)
	go m.refillLoop()
	go m.updateWorker()
	// refreshLoop 已移除，复用型池不需要定时刷新

	log.Info().
		Int("article_groups", len(groupIDs)).
		Int("keyword_groups", len(keywordGroupIDs)).
		Int("image_groups", len(imageGroupIDs)).
		Int("title_pool_size", m.config.TitlePoolSize).
		Int("contents_size", m.config.ContentsSize).
		Msg("PoolManager started")

	return nil
}

// Stop stops the pool manager gracefully
func (m *PoolManager) Stop() {
	m.stopped.Store(true)
	m.cancel()
	close(m.updateCh)
	if m.titleGenerator != nil {
		m.titleGenerator.Stop()
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
	if config.ContentsSize != oldConfig.ContentsSize {
		for _, pool := range m.contents {
			pool.Resize(config.ContentsSize)
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
		Int("contents_size", config.ContentsSize).
		Int("threshold", config.Threshold).
		Int("interval_ms", config.RefillIntervalMs).
		Msg("PoolManager config reloaded")

	return nil
}

// GetStats returns pool statistics
func (m *PoolManager) GetStats() map[string]interface{} {
	m.mu.RLock()
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
	m.mu.RUnlock()

	m.keywordsMu.RLock()
	keywordsStats := make(map[int]int)
	for gid, items := range m.keywords {
		keywordsStats[gid] = len(items)
	}
	m.keywordsMu.RUnlock()

	m.imagesMu.RLock()
	imagesStats := make(map[int]int)
	for gid, items := range m.images {
		imagesStats[gid] = len(items)
	}
	m.imagesMu.RUnlock()

	return map[string]interface{}{
		"titles":   titlesStats,
		"contents": contentsStats,
		"keywords": keywordsStats,
		"images":   imagesStats,
		"emojis":   m.emojiManager.Count(),
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

// ============================================================
// Keywords 方法
// ============================================================

// LoadKeywords loads keywords for a group from the database
func (m *PoolManager) LoadKeywords(ctx context.Context, groupID int) (int, error) {
	query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1`

	var keywords []string
	if err := m.db.SelectContext(ctx, &keywords, query, groupID); err != nil {
		return 0, err
	}

	// Store raw keywords
	rawCopy := make([]string, len(keywords))
	copy(rawCopy, keywords)

	// Pre-encode keywords
	encoded := make([]string, len(keywords))
	for i, kw := range keywords {
		encoded[i] = m.encoder.EncodeText(kw)
	}

	m.keywordsMu.Lock()
	m.keywords[groupID] = encoded
	m.rawKeywords[groupID] = rawCopy
	m.keywordsMu.Unlock()

	log.Info().Int("group_id", groupID).Int("count", len(encoded)).Msg("Keywords loaded")
	return len(encoded), nil
}

// GetRandomKeywords returns random pre-encoded keywords
func (m *PoolManager) GetRandomKeywords(groupID int, count int) []string {
	m.keywordsMu.RLock()
	items := m.keywords[groupID]
	if len(items) == 0 {
		items = m.keywords[1] // fallback to default group
	}
	m.keywordsMu.RUnlock()

	return getRandomItems(items, count)
}

// GetRawKeywords returns raw (not encoded) keywords
func (m *PoolManager) GetRawKeywords(groupID int, count int) []string {
	m.keywordsMu.RLock()
	items := m.rawKeywords[groupID]
	if len(items) == 0 {
		items = m.rawKeywords[1]
	}
	m.keywordsMu.RUnlock()

	return getRandomItems(items, count)
}

// AppendKeywords 追加关键词到内存（新增时调用）
func (m *PoolManager) AppendKeywords(groupID int, keywords []string) {
	if len(keywords) == 0 {
		return
	}

	m.keywordsMu.Lock()
	defer m.keywordsMu.Unlock()

	if m.keywords[groupID] == nil {
		m.keywords[groupID] = []string{}
		m.rawKeywords[groupID] = []string{}
	}

	// 追加原始关键词
	m.rawKeywords[groupID] = append(m.rawKeywords[groupID], keywords...)

	// 追加编码后的关键词
	for _, kw := range keywords {
		m.keywords[groupID] = append(m.keywords[groupID], m.encoder.EncodeText(kw))
	}

	log.Debug().Int("group_id", groupID).Int("added", len(keywords)).Msg("Keywords appended to cache")
}

// ReloadKeywordGroup 重载指定分组的关键词缓存（删除时调用）
func (m *PoolManager) ReloadKeywordGroup(ctx context.Context, groupID int) error {
	_, err := m.LoadKeywords(ctx, groupID)
	if err != nil {
		log.Error().Err(err).Int("group_id", groupID).Msg("Failed to reload keyword group")
	}
	return err
}

// getRandomItems 从切片中随机选取指定数量的元素（Fisher-Yates 部分洗牌）
func getRandomItems(items []string, count int) []string {
	n := len(items)
	if n == 0 || count == 0 {
		return nil
	}
	if count > n {
		count = n
	}

	swapped := make(map[int]int, count)
	result := make([]string, count)

	for i := 0; i < count; i++ {
		j := i + rand.IntN(n-i)
		vi, oki := swapped[i]
		if !oki {
			vi = i
		}
		vj, okj := swapped[j]
		if !okj {
			vj = j
		}
		swapped[i] = vj
		swapped[j] = vi
		result[i] = items[vj]
	}
	return result
}

// ============================================================
// Images 方法
// ============================================================

// LoadImages loads image URLs for a group from the database
func (m *PoolManager) LoadImages(ctx context.Context, groupID int) (int, error) {
	query := `SELECT url FROM images WHERE group_id = ? AND status = 1`

	var urls []string
	if err := m.db.SelectContext(ctx, &urls, query, groupID); err != nil {
		return 0, err
	}

	m.imagesMu.Lock()
	m.images[groupID] = urls
	m.imagesMu.Unlock()

	log.Info().Int("group_id", groupID).Int("count", len(urls)).Msg("Images loaded")
	return len(urls), nil
}

// GetRandomImage returns a random image URL
func (m *PoolManager) GetRandomImage(groupID int) string {
	m.imagesMu.RLock()
	items := m.images[groupID]
	if len(items) == 0 {
		items = m.images[1]
	}
	m.imagesMu.RUnlock()

	if len(items) == 0 {
		return ""
	}
	return items[rand.IntN(len(items))]
}

// GetImages returns all image URLs for a group
func (m *PoolManager) GetImages(groupID int) []string {
	m.imagesMu.RLock()
	urls := m.images[groupID]
	m.imagesMu.RUnlock()

	if len(urls) == 0 {
		return nil
	}

	result := make([]string, len(urls))
	copy(result, urls)
	return result
}

// AppendImages 追加图片到内存（新增时调用）
func (m *PoolManager) AppendImages(groupID int, urls []string) {
	if len(urls) == 0 {
		return
	}

	m.imagesMu.Lock()
	defer m.imagesMu.Unlock()

	if m.images[groupID] == nil {
		m.images[groupID] = []string{}
	}
	m.images[groupID] = append(m.images[groupID], urls...)

	log.Debug().Int("group_id", groupID).Int("added", len(urls)).Msg("Images appended to cache")
}

// ReloadImageGroup 重载指定分组的图片缓存（删除时调用）
func (m *PoolManager) ReloadImageGroup(ctx context.Context, groupID int) error {
	_, err := m.LoadImages(ctx, groupID)
	if err != nil {
		log.Error().Err(err).Int("group_id", groupID).Msg("Failed to reload image group")
	}
	return err
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
func (m *PoolManager) discoverKeywordGroups(ctx context.Context) ([]int, error) {
	query := `SELECT DISTINCT id FROM keyword_groups`
	var ids []int
	if err := m.db.SelectContext(ctx, &ids, query); err != nil {
		log.Warn().Err(err).Msg("Failed to query keyword groups, using default")
		return []int{1}, nil
	}
	if len(ids) == 0 {
		return []int{1}, nil
	}
	return ids, nil
}

// discoverImageGroups finds all image group IDs
func (m *PoolManager) discoverImageGroups(ctx context.Context) ([]int, error) {
	query := `SELECT DISTINCT id FROM image_groups`
	var ids []int
	if err := m.db.SelectContext(ctx, &ids, query); err != nil {
		log.Warn().Err(err).Msg("Failed to query image groups, using default")
		return []int{1}, nil
	}
	if len(ids) == 0 {
		return []int{1}, nil
	}
	return ids, nil
}

// ============================================================
// 兼容性方法（供 router/websocket 使用）
// ============================================================

// GetDataPoolsStats 返回数据池运行状态统计（与前端展示格式一致）
// 返回全部 5 个池：标题池、正文池、关键词池、图片池、表情库
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
	var titlesCurrent, titlesMax int
	if m.titleGenerator != nil {
		titlesCurrent, titlesMax = m.titleGenerator.GetTotalStats()
	}
	titlesUsed := titlesMax - titlesCurrent
	titlesUtil := 0.0
	if titlesMax > 0 {
		titlesUtil = float64(titlesCurrent) / float64(titlesMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "标题池",
		Size:        titlesMax,
		Available:   titlesCurrent,
		Used:        titlesUsed,
		Utilization: titlesUtil,
		Status:      status,
		NumWorkers:  m.config.TitleWorkers,
		LastRefresh: lastRefreshPtr,
		PoolType:    "consumable",
	})

	// 2. 正文池（消费型，汇总所有分组）
	m.mu.RLock()
	var contentsMax, contentsCurrent int
	for _, pool := range m.contents {
		contentsMax += pool.GetMaxSize()
		contentsCurrent += pool.Len()
	}
	m.mu.RUnlock()

	contentsUsed := contentsMax - contentsCurrent
	contentsUtil := 0.0
	if contentsMax > 0 {
		contentsUtil = float64(contentsCurrent) / float64(contentsMax) * 100
	}
	pools = append(pools, PoolStatusStats{
		Name:        "正文池",
		Size:        contentsMax,
		Available:   contentsCurrent,
		Used:        contentsUsed,
		Utilization: contentsUtil,
		Status:      status,
		NumWorkers:  1,
		LastRefresh: lastRefreshPtr,
		PoolType:    "consumable",
	})

	// 3. 关键词池（复用型，增加分组详情）
	m.keywordsMu.RLock()
	var totalKeywords int
	keywordGroups := []PoolGroupInfo{}
	for gid, items := range m.keywords {
		count := len(items)
		totalKeywords += count
		keywordGroups = append(keywordGroups, PoolGroupInfo{
			ID:    gid,
			Name:  fmt.Sprintf("分组%d", gid),
			Count: count,
		})
	}
	m.keywordsMu.RUnlock()
	pools = append(pools, PoolStatusStats{
		Name:        "关键词池",
		Size:        totalKeywords,
		Available:   totalKeywords,
		Used:        0,
		Utilization: 100,
		Status:      status,
		NumWorkers:  0,
		LastRefresh: lastRefreshPtr,
		PoolType:    "reusable",
		Groups:      keywordGroups,
	})

	// 4. 图片池（复用型，增加分组详情）
	m.imagesMu.RLock()
	var totalImages int
	imageGroups := []PoolGroupInfo{}
	for gid, items := range m.images {
		count := len(items)
		totalImages += count
		imageGroups = append(imageGroups, PoolGroupInfo{
			ID:    gid,
			Name:  fmt.Sprintf("分组%d", gid),
			Count: count,
		})
	}
	m.imagesMu.RUnlock()
	pools = append(pools, PoolStatusStats{
		Name:        "图片池",
		Size:        totalImages,
		Available:   totalImages,
		Used:        0,
		Utilization: 100,
		Status:      status,
		NumWorkers:  0,
		LastRefresh: lastRefreshPtr,
		PoolType:    "reusable",
		Groups:      imageGroups,
	})

	// 5. 表情库（静态数据）
	emojiCount := m.emojiManager.Count()
	pools = append(pools, PoolStatusStats{
		Name:        "表情库",
		Size:        emojiCount,
		Available:   emojiCount,
		Used:        0,
		Utilization: 100,
		Status:      status,
		NumWorkers:  0,
		LastRefresh: nil,
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
func (m *PoolManager) GetPoolStatsSimple() SimplePoolStats {
	m.keywordsMu.RLock()
	var totalKeywords int
	for _, items := range m.keywords {
		totalKeywords += len(items)
	}
	m.keywordsMu.RUnlock()

	m.imagesMu.RLock()
	var totalImages int
	for _, items := range m.images {
		totalImages += len(items)
	}
	m.imagesMu.RUnlock()

	return SimplePoolStats{
		Keywords: totalKeywords,
		Images:   totalImages,
	}
}

// RefreshData 手动刷新指定数据池
func (m *PoolManager) RefreshData(ctx context.Context, pool string) error {
	switch pool {
	case "keywords", "all":
		groupIDs, _ := m.discoverKeywordGroups(ctx)
		for _, gid := range groupIDs {
			if _, err := m.LoadKeywords(ctx, gid); err != nil {
				return err
			}
		}
	}

	switch pool {
	case "images", "all":
		groupIDs, _ := m.discoverImageGroups(ctx)
		for _, gid := range groupIDs {
			if _, err := m.LoadImages(ctx, gid); err != nil {
				return err
			}
		}
	}

	m.mu.Lock()
	m.lastRefresh = time.Now()
	m.mu.Unlock()

	return nil
}
