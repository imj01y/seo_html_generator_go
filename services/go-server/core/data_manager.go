package core

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

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
}

// NewDataManager creates a new data manager
func NewDataManager(db *sqlx.DB, encoder *HTMLEntityEncoder) *DataManager {
	return &DataManager{
		db:           db,
		keywords:     make(map[int][]string),
		rawKeywords:  make(map[int][]string),
		imageURLs:    make(map[int][]string),
		titles:       make(map[int][]string),
		contents:     make(map[int][]string),
		encoder:      encoder,
		emojiManager: NewEmojiManager(),
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
