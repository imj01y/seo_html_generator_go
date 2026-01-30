package core

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/models"
)

// TemplateCache manages template content with permanent caching
// Templates are loaded at startup and updated on-demand via API
type TemplateCache struct {
	db       *sqlx.DB
	cache    sync.Map // key: "name:groupID" -> *models.Template
	count    int64
	mu       sync.RWMutex
	analyzer *TemplateAnalyzer // 模板分析器
}

// NewTemplateCache creates a new template cache
func NewTemplateCache(db *sqlx.DB) *TemplateCache {
	return &TemplateCache{
		db: db,
	}
}

// cacheKey generates the cache key for a template
func cacheKey(name string, siteGroupID int) string {
	return fmt.Sprintf("%s:%d", name, siteGroupID)
}

// LoadAll loads all active templates into cache at startup
func (tc *TemplateCache) LoadAll(ctx context.Context) error {
	templates := []models.Template{}
	query := `SELECT * FROM templates WHERE status = 1`

	if err := tc.db.SelectContext(ctx, &templates, query); err != nil {
		return err
	}

	tc.mu.Lock()
	tc.count = int64(len(templates))
	analyzer := tc.analyzer
	tc.mu.Unlock()

	for i := range templates {
		key := cacheKey(templates[i].Name, templates[i].SiteGroupID)
		tc.cache.Store(key, &templates[i])

		// 触发模板分析
		if analyzer != nil {
			tc.analyzeTemplate(&templates[i])
		}
	}

	log.Info().
		Int("count", len(templates)).
		Msg("All templates loaded into cache")

	return nil
}

// Get retrieves template by name and site group ID
// First tries the specific site group, then falls back to default group (1)
func (tc *TemplateCache) Get(name string, siteGroupID int) *models.Template {
	// Try site group specific template first
	key := cacheKey(name, siteGroupID)
	if cached, found := tc.cache.Load(key); found {
		if tmpl, ok := cached.(*models.Template); ok {
			return tmpl
		}
	}

	// Fallback to default site group (ID = 1)
	if siteGroupID != 1 {
		defaultKey := cacheKey(name, 1)
		if cached, found := tc.cache.Load(defaultKey); found {
			if tmpl, ok := cached.(*models.Template); ok {
				return tmpl
			}
		}
	}

	return nil
}

// GetWithFallback retrieves template with DB fallback for newly added templates
func (tc *TemplateCache) GetWithFallback(ctx context.Context, name string, siteGroupID int) (*models.Template, error) {
	// Try cache first
	if tmpl := tc.Get(name, siteGroupID); tmpl != nil {
		return tmpl, nil
	}

	// Try to load from DB (for newly added templates)
	tmpl := &models.Template{}

	// Try site group specific template first
	query := `SELECT * FROM templates WHERE name = ? AND site_group_id = ? AND status = 1 LIMIT 1`
	err := tc.db.GetContext(ctx, tmpl, query, name, siteGroupID)
	if err == nil {
		key := cacheKey(name, siteGroupID)
		tc.cache.Store(key, tmpl)
		log.Debug().
			Str("name", name).
			Int("site_group_id", siteGroupID).
			Msg("Template loaded on-demand and cached")
		return tmpl, nil
	}

	if err != sql.ErrNoRows {
		return nil, err
	}

	// Fallback to default site group
	if siteGroupID != 1 {
		query = `SELECT * FROM templates WHERE name = ? AND site_group_id = 1 AND status = 1 LIMIT 1`
		err = tc.db.GetContext(ctx, tmpl, query, name)
		if err == nil {
			key := cacheKey(name, 1)
			tc.cache.Store(key, tmpl)
			return tmpl, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}

	return nil, nil
}

// Reload reloads a specific template from database
func (tc *TemplateCache) Reload(ctx context.Context, name string, siteGroupID int) error {
	tmpl := &models.Template{}
	query := `SELECT * FROM templates WHERE name = ? AND site_group_id = ? AND status = 1 LIMIT 1`

	err := tc.db.GetContext(ctx, tmpl, query, name, siteGroupID)
	if err != nil {
		if err == sql.ErrNoRows {
			// Template was deleted or disabled, remove from cache
			key := cacheKey(name, siteGroupID)
			tc.cache.Delete(key)

			// 从分析器中移除
			tc.mu.RLock()
			analyzer := tc.analyzer
			tc.mu.RUnlock()
			if analyzer != nil {
				analyzer.RemoveAnalysis(name, siteGroupID)
			}

			log.Info().
				Str("name", name).
				Int("site_group_id", siteGroupID).
				Msg("Template removed from cache (not found or disabled)")
			return nil
		}
		return err
	}

	key := cacheKey(name, siteGroupID)
	tc.cache.Store(key, tmpl)

	// 触发模板分析
	tc.analyzeTemplate(tmpl)

	log.Info().
		Str("name", name).
		Int("site_group_id", siteGroupID).
		Msg("Template cache reloaded")

	return nil
}

// ReloadByName reloads all versions of a template (all site groups)
func (tc *TemplateCache) ReloadByName(ctx context.Context, name string) error {
	templates := []models.Template{}
	query := `SELECT * FROM templates WHERE name = ? AND status = 1`

	if err := tc.db.SelectContext(ctx, &templates, query, name); err != nil {
		return err
	}

	// Delete all existing versions of this template
	tc.cache.Range(func(k, v interface{}) bool {
		key := k.(string)
		if tmpl, ok := v.(*models.Template); ok && tmpl != nil && tmpl.Name == name {
			tc.cache.Delete(key)
		}
		return true
	})

	// Store new versions
	for i := range templates {
		key := cacheKey(templates[i].Name, templates[i].SiteGroupID)
		tc.cache.Store(key, &templates[i])

		// 触发模板分析
		tc.analyzeTemplate(&templates[i])
	}

	log.Info().
		Str("name", name).
		Int("versions", len(templates)).
		Msg("Template cache reloaded (all versions)")

	return nil
}

// ReloadAll reloads all templates from database
func (tc *TemplateCache) ReloadAll(ctx context.Context) error {
	// Clear existing cache
	tc.cache.Range(func(key, value interface{}) bool {
		tc.cache.Delete(key)
		return true
	})

	// Reload all
	return tc.LoadAll(ctx)
}

// Invalidate removes a specific template from cache
func (tc *TemplateCache) Invalidate(name string, siteGroupID int) {
	key := cacheKey(name, siteGroupID)
	tc.cache.Delete(key)
}

// InvalidateAll clears the entire cache
func (tc *TemplateCache) InvalidateAll() {
	tc.cache.Range(func(key, value interface{}) bool {
		tc.cache.Delete(key)
		return true
	})

	tc.mu.Lock()
	tc.count = 0
	tc.mu.Unlock()
}

// GetStats returns cache statistics
func (tc *TemplateCache) GetStats() map[string]interface{} {
	count := 0
	tc.cache.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	return map[string]interface{}{
		"item_count": count,
		"mode":       "permanent",
	}
}

// GetAllNames returns all cached template names (for warmup)
func (tc *TemplateCache) GetAllNames() []string {
	names := make(map[string]bool)
	tc.cache.Range(func(key, value interface{}) bool {
		if tmpl, ok := value.(*models.Template); ok && tmpl != nil {
			names[tmpl.Name] = true
		}
		return true
	})

	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

// SetAnalyzer 设置模板分析器
func (tc *TemplateCache) SetAnalyzer(analyzer *TemplateAnalyzer) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.analyzer = analyzer

	log.Info().Msg("Template analyzer set on template cache")

	// 分析所有已缓存的模板
	tc.cache.Range(func(key, value interface{}) bool {
		if tmpl, ok := value.(*models.Template); ok && tmpl != nil {
			tc.analyzeTemplate(tmpl)
		}
		return true
	})
}

// GetAnalyzer 获取模板分析器
func (tc *TemplateCache) GetAnalyzer() *TemplateAnalyzer {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.analyzer
}

// analyzeTemplate 分析单个模板（内部方法）
func (tc *TemplateCache) analyzeTemplate(tmpl *models.Template) {
	if tc.analyzer == nil || tmpl == nil {
		return
	}

	// 在后台分析，避免阻塞
	go func() {
		tc.analyzer.AnalyzeTemplate(tmpl.Name, tmpl.SiteGroupID, tmpl.Content)
	}()
}
