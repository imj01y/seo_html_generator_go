package core

import (
	"context"
	"database/sql"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/model"
)

// SiteCache manages site configuration with permanent caching
// Sites are loaded at startup and updated on-demand via API
type SiteCache struct {
	db    *sqlx.DB
	cache sync.Map // domain -> *models.Site
	count int64    // cached site count
	mu    sync.RWMutex
}

// NewSiteCache creates a new site cache (permanent mode, no TTL)
func NewSiteCache(db *sqlx.DB) *SiteCache {
	return &SiteCache{
		db: db,
	}
}

// LoadAll loads all active sites into cache at startup
func (sc *SiteCache) LoadAll(ctx context.Context) error {
	sites := []models.Site{}
	query := `SELECT * FROM sites WHERE status = 1`

	if err := sc.db.SelectContext(ctx, &sites, query); err != nil {
		return err
	}

	sc.mu.Lock()
	sc.count = int64(len(sites))
	sc.mu.Unlock()

	for i := range sites {
		sc.cache.Store(sites[i].Domain, &sites[i])
	}

	log.Info().
		Int("count", len(sites)).
		Msg("All sites loaded into cache")

	return nil
}

// Get retrieves site configuration by domain (no DB query, pure memory)
func (sc *SiteCache) Get(ctx context.Context, domain string) (*models.Site, error) {
	if cached, found := sc.cache.Load(domain); found {
		if site, ok := cached.(*models.Site); ok {
			return site, nil
		}
		// nil marker for non-existent domain
		return nil, nil
	}

	// Domain not in cache - try to load from DB (for newly added domains)
	site := &models.Site{}
	query := `SELECT * FROM sites WHERE domain = ? AND status = 1 LIMIT 1`

	err := sc.db.GetContext(ctx, site, query, domain)
	if err != nil {
		if err == sql.ErrNoRows {
			// Cache negative result
			sc.cache.Store(domain, (*models.Site)(nil))
			return nil, nil
		}
		return nil, err
	}

	// Cache the result
	sc.cache.Store(domain, site)

	log.Debug().
		Str("domain", domain).
		Str("template", site.Template).
		Int("site_group_id", site.SiteGroupID).
		Msg("Site config loaded on-demand and cached")

	return site, nil
}

// Reload reloads a single site from database
func (sc *SiteCache) Reload(ctx context.Context, domain string) error {
	site := &models.Site{}
	query := `SELECT * FROM sites WHERE domain = ? AND status = 1 LIMIT 1`

	err := sc.db.GetContext(ctx, site, query, domain)
	if err != nil {
		if err == sql.ErrNoRows {
			// Site was deleted or disabled, remove from cache
			sc.cache.Delete(domain)
			log.Info().Str("domain", domain).Msg("Site removed from cache (not found or disabled)")
			return nil
		}
		return err
	}

	sc.cache.Store(domain, site)
	log.Info().
		Str("domain", domain).
		Str("template", site.Template).
		Msg("Site cache reloaded")

	return nil
}

// ReloadAll reloads all sites from database
func (sc *SiteCache) ReloadAll(ctx context.Context) error {
	// Clear existing cache
	sc.cache.Range(func(key, value interface{}) bool {
		sc.cache.Delete(key)
		return true
	})

	// Reload all
	return sc.LoadAll(ctx)
}

// Invalidate removes a domain from the cache
func (sc *SiteCache) Invalidate(domain string) {
	sc.cache.Delete(domain)
}

// InvalidateAll clears the entire cache
func (sc *SiteCache) InvalidateAll() {
	sc.cache.Range(func(key, value interface{}) bool {
		sc.cache.Delete(key)
		return true
	})

	sc.mu.Lock()
	sc.count = 0
	sc.mu.Unlock()
}

// siteMemorySize 计算单个 Site 的内存占用
func siteMemorySize(site *models.Site) int64 {
	if site == nil {
		return 0
	}
	// 结构体固定开销（int/NullInt64/time.Time等）
	const fixedOverhead = 240
	size := int64(fixedOverhead)
	size += int64(len(site.Domain))
	size += int64(len(site.Name))
	size += int64(len(site.Template))
	if site.ICPNumber.Valid {
		size += int64(len(site.ICPNumber.String))
	}
	if site.BaiduToken.Valid {
		size += int64(len(site.BaiduToken.String))
	}
	if site.Analytics.Valid {
		size += int64(len(site.Analytics.String))
	}
	return size
}

// GetStats returns cache statistics
func (sc *SiteCache) GetStats() map[string]interface{} {
	count := 0
	var memoryBytes int64
	sc.cache.Range(func(key, value interface{}) bool {
		count++
		if site, ok := value.(*models.Site); ok && site != nil {
			memoryBytes += siteMemorySize(site)
		}
		return true
	})

	return map[string]interface{}{
		"item_count":   count,
		"memory_bytes": memoryBytes,
	}
}
