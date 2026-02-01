// api/internal/service/pool_config.go
package core

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// CachePoolConfig holds cache pool configuration for titles and contents
type CachePoolConfig struct {
	ID               int       `db:"id" json:"id"`
	TitlesSize       int       `db:"titles_size" json:"titles_size"`
	ContentsSize     int       `db:"contents_size" json:"contents_size"`
	Threshold        int       `db:"threshold" json:"threshold"`
	RefillIntervalMs int       `db:"refill_interval_ms" json:"refill_interval_ms"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// RefillInterval returns the refill interval as time.Duration
func (c *CachePoolConfig) RefillInterval() time.Duration {
	return time.Duration(c.RefillIntervalMs) * time.Millisecond
}

// DefaultCachePoolConfig returns default configuration
func DefaultCachePoolConfig() *CachePoolConfig {
	return &CachePoolConfig{
		ID:               1,
		TitlesSize:       5000,
		ContentsSize:     5000,
		Threshold:        1000,
		RefillIntervalMs: 1000,
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
		INSERT INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms)
		VALUES (1, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			titles_size = VALUES(titles_size),
			contents_size = VALUES(contents_size),
			threshold = VALUES(threshold),
			refill_interval_ms = VALUES(refill_interval_ms)
	`
	_, err := db.ExecContext(ctx, query,
		config.TitlesSize,
		config.ContentsSize,
		config.Threshold,
		config.RefillIntervalMs,
	)
	return err
}
