// api/internal/service/pool_config.go
package core

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// CachePoolConfig holds cache pool configuration for titles, contents, keywords, and images
type CachePoolConfig struct {
	ID               int       `db:"id" json:"id"`
	TitlesSize       int       `db:"titles_size" json:"titles_size"`
	ContentsSize     int       `db:"contents_size" json:"contents_size"`
	Threshold        int       `db:"threshold" json:"threshold"`
	RefillIntervalMs int       `db:"refill_interval_ms" json:"refill_interval_ms"`
	// keywords/images 配置
	KeywordsSize      int `db:"keywords_size" json:"keywords_size"`
	ImagesSize        int `db:"images_size" json:"images_size"`
	RefreshIntervalMs int `db:"refresh_interval_ms" json:"refresh_interval_ms"`
	// 标题生成配置（新增）
	TitlePoolSize         int     `db:"title_pool_size" json:"title_pool_size"`
	TitleWorkers          int     `db:"title_workers" json:"title_workers"`
	TitleRefillIntervalMs int     `db:"title_refill_interval_ms" json:"title_refill_interval_ms"`
	TitleThreshold        float64 `db:"title_threshold" json:"title_threshold"`
	// cls类名池配置
	ClsPoolSize         int     `db:"cls_pool_size" json:"cls_pool_size"`
	ClsWorkers          int     `db:"cls_workers" json:"cls_workers"`
	ClsRefillIntervalMs int     `db:"cls_refill_interval_ms" json:"cls_refill_interval_ms"`
	ClsThreshold        float64 `db:"cls_threshold" json:"cls_threshold"`
	// url池配置
	UrlPoolSize         int     `db:"url_pool_size" json:"url_pool_size"`
	UrlWorkers          int     `db:"url_workers" json:"url_workers"`
	UrlRefillIntervalMs int     `db:"url_refill_interval_ms" json:"url_refill_interval_ms"`
	UrlThreshold        float64 `db:"url_threshold" json:"url_threshold"`
	// 关键词表情池配置
	KeywordEmojiPoolSize         int     `db:"keyword_emoji_pool_size" json:"keyword_emoji_pool_size"`
	KeywordEmojiWorkers          int     `db:"keyword_emoji_workers" json:"keyword_emoji_workers"`
	KeywordEmojiRefillIntervalMs int     `db:"keyword_emoji_refill_interval_ms" json:"keyword_emoji_refill_interval_ms"`
	KeywordEmojiThreshold        float64 `db:"keyword_emoji_threshold" json:"keyword_emoji_threshold"`
	UpdatedAt                    time.Time `db:"updated_at" json:"updated_at"`
}

// RefillInterval returns the refill interval as time.Duration
func (c *CachePoolConfig) RefillInterval() time.Duration {
	return time.Duration(c.RefillIntervalMs) * time.Millisecond
}

// RefreshInterval returns the refresh interval for keywords/images as time.Duration
func (c *CachePoolConfig) RefreshInterval() time.Duration {
	return time.Duration(c.RefreshIntervalMs) * time.Millisecond
}

// TitleRefillInterval returns the title refill interval as time.Duration
func (c *CachePoolConfig) TitleRefillInterval() time.Duration {
	return time.Duration(c.TitleRefillIntervalMs) * time.Millisecond
}

// ClsRefillInterval returns the cls refill interval as time.Duration
func (c *CachePoolConfig) ClsRefillInterval() time.Duration {
	return time.Duration(c.ClsRefillIntervalMs) * time.Millisecond
}

// UrlRefillInterval returns the url refill interval as time.Duration
func (c *CachePoolConfig) UrlRefillInterval() time.Duration {
	return time.Duration(c.UrlRefillIntervalMs) * time.Millisecond
}

// KeywordEmojiRefillInterval returns the keyword emoji refill interval as time.Duration
func (c *CachePoolConfig) KeywordEmojiRefillInterval() time.Duration {
	return time.Duration(c.KeywordEmojiRefillIntervalMs) * time.Millisecond
}

// DefaultCachePoolConfig returns default configuration
func DefaultCachePoolConfig() *CachePoolConfig {
	return &CachePoolConfig{
		ID:                           1,
		TitlesSize:                   5000,
		ContentsSize:                 5000,
		Threshold:                    1000,
		RefillIntervalMs:             1000,
		KeywordsSize:                 50000,
		ImagesSize:                   50000,
		RefreshIntervalMs:            300000, // 5 minutes
		TitlePoolSize:                800000,
		TitleWorkers:                 20,
		TitleRefillIntervalMs:        30,
		TitleThreshold:               0.4,
		ClsPoolSize:                  800000,
		ClsWorkers:                   20,
		ClsRefillIntervalMs:          30,
		ClsThreshold:                 0.4,
		UrlPoolSize:                  500000,
		UrlWorkers:                   16,
		UrlRefillIntervalMs:          30,
		UrlThreshold:                 0.4,
		KeywordEmojiPoolSize:         800000,
		KeywordEmojiWorkers:          20,
		KeywordEmojiRefillIntervalMs: 30,
		KeywordEmojiThreshold:        0.4,
	}
}

// LoadCachePoolConfig loads configuration from database
func LoadCachePoolConfig(ctx context.Context, db *sqlx.DB) (*CachePoolConfig, error) {
	config := &CachePoolConfig{}
	err := db.GetContext(ctx, config, "SELECT * FROM pool_config WHERE id = 1")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load pool config, using defaults")
		return DefaultCachePoolConfig(), nil
	}
	return config, nil
}

// SaveCachePoolConfig saves configuration to database
func SaveCachePoolConfig(ctx context.Context, db *sqlx.DB, config *CachePoolConfig) error {
	query := `
		INSERT INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms, keywords_size, images_size, refresh_interval_ms, title_pool_size, title_workers, title_refill_interval_ms, title_threshold, cls_pool_size, cls_workers, cls_refill_interval_ms, cls_threshold, url_pool_size, url_workers, url_refill_interval_ms, url_threshold, keyword_emoji_pool_size, keyword_emoji_workers, keyword_emoji_refill_interval_ms, keyword_emoji_threshold)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			titles_size = VALUES(titles_size),
			contents_size = VALUES(contents_size),
			threshold = VALUES(threshold),
			refill_interval_ms = VALUES(refill_interval_ms),
			keywords_size = VALUES(keywords_size),
			images_size = VALUES(images_size),
			refresh_interval_ms = VALUES(refresh_interval_ms),
			title_pool_size = VALUES(title_pool_size),
			title_workers = VALUES(title_workers),
			title_refill_interval_ms = VALUES(title_refill_interval_ms),
			title_threshold = VALUES(title_threshold),
			cls_pool_size = VALUES(cls_pool_size),
			cls_workers = VALUES(cls_workers),
			cls_refill_interval_ms = VALUES(cls_refill_interval_ms),
			cls_threshold = VALUES(cls_threshold),
			url_pool_size = VALUES(url_pool_size),
			url_workers = VALUES(url_workers),
			url_refill_interval_ms = VALUES(url_refill_interval_ms),
			url_threshold = VALUES(url_threshold),
			keyword_emoji_pool_size = VALUES(keyword_emoji_pool_size),
			keyword_emoji_workers = VALUES(keyword_emoji_workers),
			keyword_emoji_refill_interval_ms = VALUES(keyword_emoji_refill_interval_ms),
			keyword_emoji_threshold = VALUES(keyword_emoji_threshold)
	`
	_, err := db.ExecContext(ctx, query,
		config.TitlesSize,
		config.ContentsSize,
		config.Threshold,
		config.RefillIntervalMs,
		config.KeywordsSize,
		config.ImagesSize,
		config.RefreshIntervalMs,
		config.TitlePoolSize,
		config.TitleWorkers,
		config.TitleRefillIntervalMs,
		config.TitleThreshold,
		config.ClsPoolSize,
		config.ClsWorkers,
		config.ClsRefillIntervalMs,
		config.ClsThreshold,
		config.UrlPoolSize,
		config.UrlWorkers,
		config.UrlRefillIntervalMs,
		config.UrlThreshold,
		config.KeywordEmojiPoolSize,
		config.KeywordEmojiWorkers,
		config.KeywordEmojiRefillIntervalMs,
		config.KeywordEmojiThreshold,
	)
	return err
}
