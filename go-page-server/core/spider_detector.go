// Package core contains the core business logic
package core

import (
	"sync"
	"sync/atomic"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog/log"
	"go-page-server/models"
)

// SpiderInfo holds spider configuration (kept for backward compatibility)
type SpiderInfo struct {
	Type       string
	Name       string
	DNSDomains []string
}

// cacheEntry represents a cached detection result with TTL
type cacheEntry struct {
	spiderType string
	expireAt   time.Time
}

// SpiderDetector detects search engine spiders by User-Agent
type SpiderDetector struct {
	configLoader *SpiderConfigLoader
	cache        *lru.Cache[string, *cacheEntry]
	cacheEnabled bool
	cacheTTL     time.Duration
	cacheHits    int64
	cacheMisses  int64
	mu           sync.RWMutex
}

// NewSpiderDetector creates a new spider detector (backward compatible version)
func NewSpiderDetector() *SpiderDetector {
	// Try to load from default config path
	configPath := DefaultSpiderConfigPath()
	detector, err := NewSpiderDetectorWithConfig(configPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load spider config, using fallback hardcoded patterns")
		return newSpiderDetectorFallback()
	}
	return detector
}

// NewSpiderDetectorWithConfig creates a new spider detector with specified config path
func NewSpiderDetectorWithConfig(configPath string) (*SpiderDetector, error) {
	loader, err := NewSpiderConfigLoader(configPath)
	if err != nil {
		return nil, err
	}

	config := loader.GetConfig()

	sd := &SpiderDetector{
		configLoader: loader,
		cacheEnabled: config.Cache.Enabled,
		cacheTTL:     time.Duration(config.Cache.TTLSeconds) * time.Second,
	}

	// Initialize LRU cache if enabled
	if config.Cache.Enabled {
		maxSize := config.Cache.MaxSize
		if maxSize <= 0 {
			maxSize = 10000 // default
		}
		cache, err := lru.New[string, *cacheEntry](maxSize)
		if err != nil {
			return nil, err
		}
		sd.cache = cache
	}

	// Set up hot-reload callback
	loader.OnChange(func(newConfig *SpiderConfig, rules []*CompiledSpiderRule) {
		sd.onConfigChange(newConfig)
	})

	return sd, nil
}

// onConfigChange handles configuration changes
func (sd *SpiderDetector) onConfigChange(newConfig *SpiderConfig) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// Update cache settings
	sd.cacheEnabled = newConfig.Cache.Enabled
	sd.cacheTTL = time.Duration(newConfig.Cache.TTLSeconds) * time.Second

	// Recreate cache if size changed
	if sd.cacheEnabled && sd.cache != nil {
		newSize := newConfig.Cache.MaxSize
		if newSize <= 0 {
			newSize = 10000
		}
		// Only recreate if size changed significantly
		if newSize != sd.cache.Len() {
			newCache, err := lru.New[string, *cacheEntry](newSize)
			if err != nil {
				log.Error().Err(err).Msg("Failed to recreate cache on config change")
				return
			}
			sd.cache = newCache
			log.Info().Int("new_size", newSize).Msg("Cache recreated with new size")
		}
	}

	// Clear cache on config change to ensure new rules take effect
	if sd.cache != nil {
		sd.cache.Purge()
		log.Info().Msg("Cache purged due to configuration change")
	}
}

// StartWatching starts watching for configuration changes
func (sd *SpiderDetector) StartWatching() error {
	if sd.configLoader == nil {
		return nil
	}
	return sd.configLoader.WatchChanges()
}

// StopWatching stops watching for configuration changes
func (sd *SpiderDetector) StopWatching() {
	if sd.configLoader != nil {
		sd.configLoader.Stop()
	}
}

// Detect detects if the User-Agent belongs to a spider
func (sd *SpiderDetector) Detect(userAgent string) *models.DetectionResult {
	if userAgent == "" {
		return &models.DetectionResult{
			IsSpider:  false,
			UserAgent: userAgent,
		}
	}

	// Check cache first (if enabled)
	sd.mu.RLock()
	cacheEnabled := sd.cacheEnabled
	cache := sd.cache
	cacheTTL := sd.cacheTTL
	sd.mu.RUnlock()

	if cacheEnabled && cache != nil {
		if entry, ok := cache.Get(userAgent); ok {
			// Check if entry has expired
			if time.Now().Before(entry.expireAt) {
				atomic.AddInt64(&sd.cacheHits, 1)
				if entry.spiderType == "" {
					return &models.DetectionResult{
						IsSpider:  false,
						UserAgent: userAgent,
					}
				}
				rule := sd.configLoader.GetRuleByType(entry.spiderType)
				if rule != nil {
					return &models.DetectionResult{
						IsSpider:   true,
						SpiderType: entry.spiderType,
						SpiderName: rule.Name,
						UserAgent:  userAgent,
					}
				}
			}
			// Entry expired, remove it
			cache.Remove(userAgent)
		}
	}

	atomic.AddInt64(&sd.cacheMisses, 1)

	// Get compiled rules from config loader
	rules := sd.configLoader.GetCompiledRules()

	// Match against patterns
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		for _, pattern := range rule.Patterns {
			if pattern.MatchString(userAgent) {
				// Cache the result
				if cacheEnabled && cache != nil {
					cache.Add(userAgent, &cacheEntry{
						spiderType: rule.Type,
						expireAt:   time.Now().Add(cacheTTL),
					})
				}

				return &models.DetectionResult{
					IsSpider:   true,
					SpiderType: rule.Type,
					SpiderName: rule.Name,
					UserAgent:  userAgent,
				}
			}
		}
	}

	// Not a spider, cache negative result
	if cacheEnabled && cache != nil {
		cache.Add(userAgent, &cacheEntry{
			spiderType: "",
			expireAt:   time.Now().Add(cacheTTL),
		})
	}

	return &models.DetectionResult{
		IsSpider:  false,
		UserAgent: userAgent,
	}
}

// IsSpider is a quick check if the UA is a spider
func (sd *SpiderDetector) IsSpider(userAgent string) bool {
	result := sd.Detect(userAgent)
	return result.IsSpider
}

// GetStats returns cache statistics
func (sd *SpiderDetector) GetStats() map[string]interface{} {
	sd.mu.RLock()
	cache := sd.cache
	sd.mu.RUnlock()

	cacheSize := 0
	if cache != nil {
		cacheSize = cache.Len()
	}

	return map[string]interface{}{
		"cache_size":   cacheSize,
		"cache_hits":   atomic.LoadInt64(&sd.cacheHits),
		"cache_misses": atomic.LoadInt64(&sd.cacheMisses),
	}
}

// GetSpiderInfo returns information about a specific spider type
func (sd *SpiderDetector) GetSpiderInfo(spiderType string) *SpiderInfo {
	if sd.configLoader == nil {
		return nil
	}
	rule := sd.configLoader.GetRuleByType(spiderType)
	if rule == nil {
		return nil
	}
	return &SpiderInfo{
		Type:       rule.Type,
		Name:       rule.Name,
		DNSDomains: rule.DNSDomains,
	}
}

// GetAllSpiderTypes returns all configured spider types
func (sd *SpiderDetector) GetAllSpiderTypes() []string {
	if sd.configLoader == nil {
		return nil
	}
	config := sd.configLoader.GetConfig()
	if config == nil {
		return nil
	}
	types := make([]string, 0, len(config.Spiders))
	for spiderType := range config.Spiders {
		types = append(types, spiderType)
	}
	return types
}

// newSpiderDetectorFallback creates a fallback detector with hardcoded patterns
// This is used when configuration file is not available
func newSpiderDetectorFallback() *SpiderDetector {
	sd := &SpiderDetector{
		cacheEnabled: true,
		cacheTTL:     time.Hour,
	}

	// Create a simple in-memory cache
	cache, _ := lru.New[string, *cacheEntry](10000)
	sd.cache = cache

	// Create a minimal config loader with hardcoded rules
	sd.configLoader = &SpiderConfigLoader{
		config: &SpiderConfig{
			Cache: SpiderCacheConfig{
				Enabled:    true,
				MaxSize:    10000,
				TTLSeconds: 3600,
			},
			Spiders: getDefaultSpiderRules(),
		},
		rulesByType: make(map[string]*CompiledSpiderRule),
	}

	// Compile hardcoded rules
	rules, rulesByType, _ := sd.configLoader.compileRules(sd.configLoader.config)
	sd.configLoader.compiledRules = rules
	sd.configLoader.rulesByType = rulesByType

	log.Info().Msg("Spider detector initialized with fallback hardcoded patterns")
	return sd
}

// getDefaultSpiderRules returns the default hardcoded spider rules
func getDefaultSpiderRules() map[string]SpiderRule {
	return map[string]SpiderRule{
		"baidu": {
			Name:       "百度蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)Baiduspider`, `(?i)Baidu-YunGuanCe`},
			DNSDomains: []string{"baidu.com", "baidu.jp"},
		},
		"google": {
			Name:       "谷歌蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)Googlebot`, `(?i)Google-InspectionTool`, `(?i)Mediapartners-Google`},
			DNSDomains: []string{"googlebot.com", "google.com"},
		},
		"bing": {
			Name:       "必应蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)bingbot`, `(?i)msnbot`, `(?i)BingPreview`},
			DNSDomains: []string{"search.msn.com"},
		},
		"sogou": {
			Name:       "搜狗蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)Sogou\s*(web\s*)?spider`, `(?i)Sogou\s*inst\s*spider`},
			DNSDomains: []string{"sogou.com"},
		},
		"360": {
			Name:       "360蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)360Spider`, `(?i)HaosouSpider`, `(?i)360JK`},
			DNSDomains: []string{"360.cn", "so.com"},
		},
		"shenma": {
			Name:       "神马蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)YisouSpider`, `(?i)Yisouspider`},
			DNSDomains: []string{"sm.cn"},
		},
		"toutiao": {
			Name:       "头条蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)Bytespider`, `(?i)Bytedance`},
			DNSDomains: []string{"bytedance.com"},
		},
		"yandex": {
			Name:       "Yandex蜘蛛",
			Enabled:    true,
			Patterns:   []string{`(?i)YandexBot`, `(?i)YandexImages`, `(?i)YandexMobileBot`},
			DNSDomains: []string{"yandex.ru", "yandex.com", "yandex.net"},
		},
	}
}

// Global spider detector instance
var globalSpiderDetector *SpiderDetector
var spiderDetectorOnce sync.Once
var spiderDetectorMu sync.RWMutex
var globalConfigPath string

// SetSpiderConfigPath sets the global config path for spider detector
// Must be called before GetSpiderDetector() for the first time
func SetSpiderConfigPath(configPath string) {
	spiderDetectorMu.Lock()
	defer spiderDetectorMu.Unlock()
	globalConfigPath = configPath
}

// GetSpiderDetector returns the global spider detector instance
func GetSpiderDetector() *SpiderDetector {
	spiderDetectorOnce.Do(func() {
		spiderDetectorMu.RLock()
		configPath := globalConfigPath
		spiderDetectorMu.RUnlock()

		if configPath == "" {
			configPath = DefaultSpiderConfigPath()
		}

		var err error
		globalSpiderDetector, err = NewSpiderDetectorWithConfig(configPath)
		if err != nil {
			log.Warn().Err(err).Str("path", configPath).Msg("Failed to load spider config, using fallback")
			globalSpiderDetector = newSpiderDetectorFallback()
		}
	})
	return globalSpiderDetector
}

// ResetSpiderDetector resets the global spider detector (mainly for testing)
func ResetSpiderDetector() {
	spiderDetectorMu.Lock()
	defer spiderDetectorMu.Unlock()

	if globalSpiderDetector != nil {
		globalSpiderDetector.StopWatching()
	}
	globalSpiderDetector = nil
	spiderDetectorOnce = sync.Once{}
	globalConfigPath = ""
}
