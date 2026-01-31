package core

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// DataManagerStats 数据管理器统计
type DataManagerStats struct {
	Keywords     int       `json:"keywords"`
	Images       int       `json:"images"`
	Titles       int       `json:"titles"`
	Contents     int       `json:"contents"`
	GroupCount   int       `json:"group_count"`
	LastRefresh  time.Time `json:"last_refresh"`
	AutoRefresh  bool      `json:"auto_refresh"`
	RefreshCount int64     `json:"refresh_count"`
}

// PoolStatusStats 单个数据池的运行状态统计（与 Go 对象池格式一致）
type PoolStatusStats struct {
	Name        string     `json:"name"`
	Size        int        `json:"size"`
	Available   int        `json:"available"`
	Used        int        `json:"used"`
	Utilization float64    `json:"utilization"`
	Status      string     `json:"status"`
	NumWorkers  int        `json:"num_workers"`
	LastRefresh *time.Time `json:"last_refresh"`
}

// DataManager manages keywords, images, titles, and content data
type DataManager struct {
	db           *sqlx.DB
	keywords     map[int][]string // group_id -> keywords (pre-encoded)
	rawKeywords  map[int][]string // group_id -> keywords (raw, not encoded)
	imageURLs    map[int][]string // group_id -> image URLs
	titles       map[int][]string // group_id -> titles
	contents     map[int][]string // group_id -> contents
	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager // Emoji 管理器
	mu           sync.RWMutex
	lastReload   time.Time
	reloadMutex  sync.Mutex

	// 自动刷新相关字段
	refreshInterval time.Duration
	stopChan        chan struct{}
	running         atomic.Bool
	refreshCount    atomic.Int64
}

// NewDataManager creates a new data manager
func NewDataManager(db *sqlx.DB, encoder *HTMLEntityEncoder, refreshInterval time.Duration) *DataManager {
	return &DataManager{
		db:              db,
		keywords:        make(map[int][]string),
		rawKeywords:     make(map[int][]string),
		imageURLs:       make(map[int][]string),
		titles:          make(map[int][]string),
		contents:        make(map[int][]string),
		encoder:         encoder,
		emojiManager:    NewEmojiManager(),
		refreshInterval: refreshInterval,
	}
}

// LoadKeywords loads keywords for a group from the database
func (m *DataManager) LoadKeywords(ctx context.Context, groupID int, limit int) (int, error) {
	query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

	var keywords []string
	if err := m.db.SelectContext(ctx, &keywords, query, groupID, limit); err != nil {
		return 0, err
	}

	// Store raw keywords (not encoded)
	rawCopy := make([]string, len(keywords))
	copy(rawCopy, keywords)

	// Pre-encode keywords
	encoded := make([]string, len(keywords))
	for i, kw := range keywords {
		encoded[i] = m.encoder.EncodeText(kw)
	}

	m.mu.Lock()
	m.keywords[groupID] = encoded
	m.rawKeywords[groupID] = rawCopy
	m.mu.Unlock()

	log.Info().Int("group_id", groupID).Int("count", len(encoded)).Msg("Keywords loaded and pre-encoded")
	return len(encoded), nil
}

// LoadImageURLs loads image URLs for a group from the database
func (m *DataManager) LoadImageURLs(ctx context.Context, groupID int, limit int) (int, error) {
	query := `SELECT url FROM images WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

	var urls []string
	if err := m.db.SelectContext(ctx, &urls, query, groupID, limit); err != nil {
		return 0, err
	}

	m.mu.Lock()
	m.imageURLs[groupID] = urls
	m.mu.Unlock()

	log.Info().Int("group_id", groupID).Int("count", len(urls)).Msg("Image URLs loaded")
	return len(urls), nil
}

// LoadTitles loads titles for a group from the database
func (m *DataManager) LoadTitles(ctx context.Context, groupID int, limit int) (int, error) {
	query := `SELECT title FROM titles WHERE group_id = ? ORDER BY batch_id DESC, RAND() LIMIT ?`

	var titles []string
	if err := m.db.SelectContext(ctx, &titles, query, groupID, limit); err != nil {
		return 0, err
	}

	m.mu.Lock()
	m.titles[groupID] = titles
	m.mu.Unlock()

	log.Info().Int("group_id", groupID).Int("count", len(titles)).Msg("Titles loaded")
	return len(titles), nil
}

// LoadContents loads contents for a group from the database
func (m *DataManager) LoadContents(ctx context.Context, groupID int, limit int) (int, error) {
	query := `SELECT content FROM contents WHERE group_id = ? ORDER BY batch_id DESC, RAND() LIMIT ?`

	var contents []string
	if err := m.db.SelectContext(ctx, &contents, query, groupID, limit); err != nil {
		return 0, err
	}

	m.mu.Lock()
	m.contents[groupID] = contents
	m.mu.Unlock()

	log.Info().Int("group_id", groupID).Int("count", len(contents)).Msg("Contents loaded")
	return len(contents), nil
}

// GetImageURLs returns all image URLs for a group
func (m *DataManager) GetImageURLs(groupID int) []string {
	m.mu.RLock()
	urls, ok := m.imageURLs[groupID]
	m.mu.RUnlock()

	if !ok || len(urls) == 0 {
		return nil
	}

	// Return a copy to avoid external modification
	result := make([]string, len(urls))
	copy(result, urls)
	return result
}

// getSliceWithFallback 获取指定 groupID 的切片，如果为空则回退到默认组
func (m *DataManager) getSliceWithFallback(data map[int][]string, groupID int) []string {
	m.mu.RLock()
	items, ok := data[groupID]
	if !ok || len(items) == 0 {
		items = data[1] // fallback to default group
	}
	m.mu.RUnlock()
	return items
}

// getRandomItems 从切片中随机选取指定数量的元素（Fisher-Yates 部分洗牌，O(count) 复杂度）
func getRandomItems(items []string, count int) []string {
	n := len(items)
	if n == 0 || count == 0 {
		return nil
	}
	if count > n {
		count = n
	}

	// 使用 map 记录交换，避免 O(n) 空间分配
	swapped := make(map[int]int, count)
	result := make([]string, count)

	for i := 0; i < count; i++ {
		j := i + rand.IntN(n-i) // 从 [i, n) 随机选一个

		// 获取实际索引（考虑之前的交换）
		vi, oki := swapped[i]
		if !oki {
			vi = i
		}
		vj, okj := swapped[j]
		if !okj {
			vj = j
		}

		// 记录交换
		swapped[i] = vj
		swapped[j] = vi

		result[i] = items[vj]
	}
	return result
}

// getRandomItem 从切片中随机选取一个元素
func getRandomItem(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return items[rand.IntN(len(items))]
}

// GetRandomKeywords returns random pre-encoded keywords
func (m *DataManager) GetRandomKeywords(groupID int, count int) []string {
	return getRandomItems(m.getSliceWithFallback(m.keywords, groupID), count)
}

// GetRawKeywords returns raw (not encoded) keywords for a group
func (m *DataManager) GetRawKeywords(groupID int, count int) []string {
	return getRandomItems(m.getSliceWithFallback(m.rawKeywords, groupID), count)
}

// GetRandomImageURL returns a random image URL
func (m *DataManager) GetRandomImageURL(groupID int) string {
	return getRandomItem(m.getSliceWithFallback(m.imageURLs, groupID))
}

// GetRandomTitles returns random titles
func (m *DataManager) GetRandomTitles(groupID int, count int) []string {
	return getRandomItems(m.getSliceWithFallback(m.titles, groupID), count)
}

// GetRandomContent returns a random content
func (m *DataManager) GetRandomContent(groupID int) string {
	return getRandomItem(m.getSliceWithFallback(m.contents, groupID))
}

// LoadAllForGroup loads all data for a specific group
func (m *DataManager) LoadAllForGroup(ctx context.Context, groupID int) error {
	m.reloadMutex.Lock()
	defer m.reloadMutex.Unlock()

	// Load keywords (up to 50000)
	if _, err := m.LoadKeywords(ctx, groupID, 50000); err != nil {
		log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load keywords")
	}

	// Load image URLs (up to 50000)
	if _, err := m.LoadImageURLs(ctx, groupID, 50000); err != nil {
		log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load image URLs")
	}

	// Load titles (up to 10000)
	if _, err := m.LoadTitles(ctx, groupID, 10000); err != nil {
		log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load titles")
	}

	// Load contents (up to 5000)
	if _, err := m.LoadContents(ctx, groupID, 5000); err != nil {
		log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load contents")
	}

	m.lastReload = time.Now()
	return nil
}

// GetStats returns statistics about loaded data
func (m *DataManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"last_reload": m.lastReload,
		"groups":      make(map[int]map[string]int),
	}

	groups := stats["groups"].(map[int]map[string]int)

	// Collect all group IDs
	groupIDs := make(map[int]bool)
	for gid := range m.keywords {
		groupIDs[gid] = true
	}
	for gid := range m.imageURLs {
		groupIDs[gid] = true
	}
	for gid := range m.titles {
		groupIDs[gid] = true
	}
	for gid := range m.contents {
		groupIDs[gid] = true
	}

	for gid := range groupIDs {
		groups[gid] = map[string]int{
			"keywords": len(m.keywords[gid]),
			"images":   len(m.imageURLs[gid]),
			"titles":   len(m.titles[gid]),
			"contents": len(m.contents[gid]),
		}
	}

	return stats
}

// LoadEmojis 从文件加载 Emoji 数据
func (m *DataManager) LoadEmojis(path string) error {
	return m.emojiManager.LoadFromFile(path)
}

// GetRandomEmoji 获取随机 Emoji
func (m *DataManager) GetRandomEmoji() string {
	return m.emojiManager.GetRandom()
}

// GetRandomEmojiExclude 获取不在 exclude 中的随机 Emoji
func (m *DataManager) GetRandomEmojiExclude(exclude map[string]bool) string {
	return m.emojiManager.GetRandomExclude(exclude)
}

// GetEmojiCount 返回已加载的 Emoji 数量
func (m *DataManager) GetEmojiCount() int {
	return m.emojiManager.Count()
}

// StartAutoRefresh 启动自动刷新
func (m *DataManager) StartAutoRefresh(groupIDs []int) {
	if m.running.Swap(true) {
		return // 已经在运行
	}

	m.stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(m.refreshInterval)
		defer ticker.Stop()

		log.Info().Dur("interval", m.refreshInterval).Msg("DataManager auto refresh started")

		for {
			select {
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				for _, gid := range groupIDs {
					if err := m.LoadAllForGroup(ctx, gid); err != nil {
						log.Error().Err(err).Int("group_id", gid).Msg("Auto refresh failed for group")
					}
				}
				m.refreshCount.Add(1)
				log.Info().Int64("count", m.refreshCount.Load()).Msg("DataManager auto refresh completed")
				cancel()
			case <-m.stopChan:
				log.Info().Msg("DataManager auto refresh stopped")
				return
			}
		}
	}()
}

// StopAutoRefresh 停止自动刷新
func (m *DataManager) StopAutoRefresh() {
	if !m.running.Swap(false) {
		return // 已经停止
	}
	close(m.stopChan)
}

// IsAutoRefreshRunning 返回自动刷新是否在运行
func (m *DataManager) IsAutoRefreshRunning() bool {
	return m.running.Load()
}

// GetRefreshCount 返回刷新次数
func (m *DataManager) GetRefreshCount() int64 {
	return m.refreshCount.Load()
}

// Refresh 刷新指定分组的指定数据池
func (m *DataManager) Refresh(ctx context.Context, groupID int, poolName string) error {
	switch poolName {
	case "keywords":
		_, err := m.LoadKeywords(ctx, groupID, 50000)
		return err
	case "images":
		_, err := m.LoadImageURLs(ctx, groupID, 50000)
		return err
	case "titles":
		_, err := m.LoadTitles(ctx, groupID, 10000)
		return err
	case "contents":
		_, err := m.LoadContents(ctx, groupID, 5000)
		return err
	case "all":
		return m.LoadAllForGroup(ctx, groupID)
	default:
		return fmt.Errorf("unknown pool name: %s", poolName)
	}
}

// RefreshAll 刷新所有分组的所有数据
func (m *DataManager) RefreshAll(ctx context.Context, groupIDs []int) error {
	var errs []error
	for _, gid := range groupIDs {
		if err := m.LoadAllForGroup(ctx, gid); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to refresh %d groups", len(errs))
	}
	return nil
}

// GetPoolStats 返回数据池统计（兼容 DataPoolManager 接口）
func (m *DataManager) GetPoolStats() DataManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalKeywords, totalImages, totalTitles, totalContents int
	groupIDs := make(map[int]bool)

	for gid, items := range m.keywords {
		totalKeywords += len(items)
		groupIDs[gid] = true
	}
	for gid, items := range m.imageURLs {
		totalImages += len(items)
		groupIDs[gid] = true
	}
	for gid, items := range m.titles {
		totalTitles += len(items)
		groupIDs[gid] = true
	}
	for gid, items := range m.contents {
		totalContents += len(items)
		groupIDs[gid] = true
	}

	return DataManagerStats{
		Keywords:     totalKeywords,
		Images:       totalImages,
		Titles:       totalTitles,
		Contents:     totalContents,
		GroupCount:   len(groupIDs),
		LastRefresh:  m.lastReload,
		AutoRefresh:  m.running.Load(),
		RefreshCount: m.refreshCount.Load(),
	}
}

// GetDetailedStats 返回详细统计信息
func (m *DataManager) GetDetailedStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make(map[int]map[string]int)
	groupIDs := make(map[int]bool)

	for gid := range m.keywords {
		groupIDs[gid] = true
	}
	for gid := range m.imageURLs {
		groupIDs[gid] = true
	}
	for gid := range m.titles {
		groupIDs[gid] = true
	}
	for gid := range m.contents {
		groupIDs[gid] = true
	}

	for gid := range groupIDs {
		groups[gid] = map[string]int{
			"keywords": len(m.keywords[gid]),
			"images":   len(m.imageURLs[gid]),
			"titles":   len(m.titles[gid]),
			"contents": len(m.contents[gid]),
		}
	}

	return map[string]interface{}{
		"groups":        groups,
		"last_refresh":  m.lastReload,
		"auto_refresh":  m.running.Load(),
		"refresh_count": m.refreshCount.Load(),
	}
}

// GetDataPoolsStats 返回数据池运行状态统计（与 Go 对象池格式一致）
func (m *DataManager) GetDataPoolsStats() []PoolStatusStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算状态
	status := "running"
	if !m.running.Load() {
		status = "stopped"
	}

	// 获取 lastRefresh 指针
	var lastRefresh *time.Time
	if !m.lastReload.IsZero() {
		t := m.lastReload
		lastRefresh = &t
	}

	// 计算关键词池总数
	var totalKeywords int
	for _, items := range m.keywords {
		totalKeywords += len(items)
	}

	// 计算图片池总数
	var totalImages int
	for _, items := range m.imageURLs {
		totalImages += len(items)
	}

	pools := []PoolStatusStats{
		{
			Name:        "关键词缓存池",
			Size:        totalKeywords,
			Available:   totalKeywords,
			Used:        0,
			Utilization: 100.0,
			Status:      status,
			NumWorkers:  1,
			LastRefresh: lastRefresh,
		},
		{
			Name:        "图片缓存池",
			Size:        totalImages,
			Available:   totalImages,
			Used:        0,
			Utilization: 100.0,
			Status:      status,
			NumWorkers:  1,
			LastRefresh: lastRefresh,
		},
	}

	return pools
}
