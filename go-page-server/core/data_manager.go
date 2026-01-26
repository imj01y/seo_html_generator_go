package core

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// DataManager manages keywords, images, titles, and content data
type DataManager struct {
	db           *sqlx.DB
	keywords     map[int][]string // group_id -> keywords (pre-encoded)
	imageURLs    map[int][]string // group_id -> image URLs
	titles       map[int][]string // group_id -> titles
	contents     map[int][]string // group_id -> contents
	encoder      *HTMLEntityEncoder
	mu           sync.RWMutex
	lastReload   time.Time
	reloadMutex  sync.Mutex
}

// NewDataManager creates a new data manager
func NewDataManager(db *sqlx.DB, encoder *HTMLEntityEncoder) *DataManager {
	return &DataManager{
		db:        db,
		keywords:  make(map[int][]string),
		imageURLs: make(map[int][]string),
		titles:    make(map[int][]string),
		contents:  make(map[int][]string),
		encoder:   encoder,
	}
}

// LoadKeywords loads keywords for a group from the database
func (m *DataManager) LoadKeywords(ctx context.Context, groupID int, limit int) (int, error) {
	query := `SELECT keyword FROM keywords WHERE group_id = ? AND status = 1 ORDER BY RAND() LIMIT ?`

	var keywords []string
	if err := m.db.SelectContext(ctx, &keywords, query, groupID, limit); err != nil {
		return 0, err
	}

	// Pre-encode keywords
	encoded := make([]string, len(keywords))
	for i, kw := range keywords {
		encoded[i] = m.encoder.EncodeText(kw)
	}

	m.mu.Lock()
	m.keywords[groupID] = encoded
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

// GetRandomKeywords returns random pre-encoded keywords
func (m *DataManager) GetRandomKeywords(groupID int, count int) []string {
	m.mu.RLock()
	keywords, ok := m.keywords[groupID]
	m.mu.RUnlock()

	if !ok || len(keywords) == 0 {
		// Try default group
		m.mu.RLock()
		keywords = m.keywords[1]
		m.mu.RUnlock()
	}

	if len(keywords) == 0 {
		return nil
	}

	// Random selection
	result := make([]string, 0, count)
	indices := rand.Perm(len(keywords))
	for i := 0; i < count && i < len(indices); i++ {
		result = append(result, keywords[indices[i]])
	}

	return result
}

// GetRandomImageURL returns a random image URL
func (m *DataManager) GetRandomImageURL(groupID int) string {
	m.mu.RLock()
	urls, ok := m.imageURLs[groupID]
	m.mu.RUnlock()

	if !ok || len(urls) == 0 {
		// Try default group
		m.mu.RLock()
		urls = m.imageURLs[1]
		m.mu.RUnlock()
	}

	if len(urls) == 0 {
		return ""
	}

	return urls[rand.Intn(len(urls))]
}

// GetRandomTitles returns random titles
func (m *DataManager) GetRandomTitles(groupID int, count int) []string {
	m.mu.RLock()
	titles, ok := m.titles[groupID]
	m.mu.RUnlock()

	if !ok || len(titles) == 0 {
		// Try default group
		m.mu.RLock()
		titles = m.titles[1]
		m.mu.RUnlock()
	}

	if len(titles) == 0 {
		return nil
	}

	// Random selection
	result := make([]string, 0, count)
	indices := rand.Perm(len(titles))
	for i := 0; i < count && i < len(indices); i++ {
		result = append(result, titles[indices[i]])
	}

	return result
}

// GetRandomContent returns a random content
func (m *DataManager) GetRandomContent(groupID int) string {
	m.mu.RLock()
	contents, ok := m.contents[groupID]
	m.mu.RUnlock()

	if !ok || len(contents) == 0 {
		// Try default group
		m.mu.RLock()
		contents = m.contents[1]
		m.mu.RUnlock()
	}

	if len(contents) == 0 {
		return ""
	}

	return contents[rand.Intn(len(contents))]
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
