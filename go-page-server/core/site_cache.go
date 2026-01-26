package core

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"

	"go-page-server/models"
)

// SiteCache manages site configuration caching
type SiteCache struct {
	db    *sqlx.DB
	cache *cache.Cache
	mu    sync.RWMutex
}

// NewSiteCache creates a new site cache with TTL
func NewSiteCache(db *sqlx.DB, ttl time.Duration) *SiteCache {
	return &SiteCache{
		db:    db,
		cache: cache.New(ttl, ttl*2), // TTL and cleanup interval
	}
}

// Get retrieves site configuration by domain
func (sc *SiteCache) Get(ctx context.Context, domain string) (*models.Site, error) {
	// Check cache first
	if cached, found := sc.cache.Get(domain); found {
		if site, ok := cached.(*models.Site); ok {
			return site, nil
		}
	}

	// Query database
	site := &models.Site{}
	query := `SELECT * FROM sites WHERE domain = ? AND status = 1 LIMIT 1`

	err := sc.db.GetContext(ctx, site, query, domain)
	if err != nil {
		if err == sql.ErrNoRows {
			// Cache negative result to prevent repeated queries
			sc.cache.Set(domain, (*models.Site)(nil), cache.DefaultExpiration)
			return nil, nil
		}
		return nil, err
	}

	// Cache the result
	sc.cache.Set(domain, site, cache.DefaultExpiration)

	log.Debug().
		Str("domain", domain).
		Str("template", site.Template).
		Int("site_group_id", site.SiteGroupID).
		Msg("Site config loaded and cached")

	return site, nil
}

// Invalidate removes a domain from the cache
func (sc *SiteCache) Invalidate(domain string) {
	sc.cache.Delete(domain)
}

// InvalidateAll clears the entire cache
func (sc *SiteCache) InvalidateAll() {
	sc.cache.Flush()
}

// GetStats returns cache statistics
func (sc *SiteCache) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"item_count": sc.cache.ItemCount(),
	}
}
