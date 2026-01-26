// Package core contains the core business logic
package core

import (
	"regexp"
	"sync"

	"go-page-server/models"
)

// SpiderInfo holds spider configuration
type SpiderInfo struct {
	Type       string
	Name       string
	DNSDomains []string
}

// SpiderDetector detects search engine spiders by User-Agent
type SpiderDetector struct {
	patterns    []*spiderPattern
	cache       sync.Map // UA -> spider type cache
	cacheHits   int64
	cacheMisses int64
}

type spiderPattern struct {
	spiderType string
	pattern    *regexp.Regexp
}

// Spider configurations
var spiderConfigs = map[string]SpiderInfo{
	"baidu": {
		Type:       "baidu",
		Name:       "百度蜘蛛",
		DNSDomains: []string{"baidu.com", "baidu.jp"},
	},
	"google": {
		Type:       "google",
		Name:       "谷歌蜘蛛",
		DNSDomains: []string{"googlebot.com", "google.com"},
	},
	"bing": {
		Type:       "bing",
		Name:       "必应蜘蛛",
		DNSDomains: []string{"search.msn.com"},
	},
	"sogou": {
		Type:       "sogou",
		Name:       "搜狗蜘蛛",
		DNSDomains: []string{"sogou.com"},
	},
	"360": {
		Type:       "360",
		Name:       "360蜘蛛",
		DNSDomains: []string{"360.cn", "so.com"},
	},
	"shenma": {
		Type:       "shenma",
		Name:       "神马蜘蛛",
		DNSDomains: []string{"sm.cn"},
	},
	"toutiao": {
		Type:       "toutiao",
		Name:       "头条蜘蛛",
		DNSDomains: []string{"bytedance.com"},
	},
	"yandex": {
		Type:       "yandex",
		Name:       "Yandex蜘蛛",
		DNSDomains: []string{"yandex.ru", "yandex.com", "yandex.net"},
	},
}

// NewSpiderDetector creates a new spider detector
func NewSpiderDetector() *SpiderDetector {
	sd := &SpiderDetector{}

	// Compile regex patterns
	patternDefs := []struct {
		spiderType string
		pattern    string
	}{
		{"baidu", `(?i)Baiduspider|Baidu-YunGuanCe`},
		{"google", `(?i)Googlebot|Google-InspectionTool|Mediapartners-Google`},
		{"bing", `(?i)bingbot|msnbot|BingPreview`},
		{"sogou", `(?i)Sogou\s*(web\s*)?spider|Sogou\s*inst\s*spider`},
		{"360", `(?i)360Spider|HaosouSpider|360JK`},
		{"shenma", `(?i)YisouSpider|Yisouspider`},
		{"toutiao", `(?i)Bytespider|Bytedance`},
		{"yandex", `(?i)YandexBot|YandexImages|YandexMobileBot`},
	}

	for _, pd := range patternDefs {
		re := regexp.MustCompile(pd.pattern)
		sd.patterns = append(sd.patterns, &spiderPattern{
			spiderType: pd.spiderType,
			pattern:    re,
		})
	}

	return sd
}

// Detect detects if the User-Agent belongs to a spider
func (sd *SpiderDetector) Detect(userAgent string) *models.DetectionResult {
	if userAgent == "" {
		return &models.DetectionResult{
			IsSpider:  false,
			UserAgent: userAgent,
		}
	}

	// Check cache first
	if cached, ok := sd.cache.Load(userAgent); ok {
		sd.cacheHits++
		spiderType := cached.(string)
		if spiderType == "" {
			return &models.DetectionResult{
				IsSpider:  false,
				UserAgent: userAgent,
			}
		}
		info := spiderConfigs[spiderType]
		return &models.DetectionResult{
			IsSpider:   true,
			SpiderType: spiderType,
			SpiderName: info.Name,
			UserAgent:  userAgent,
		}
	}

	sd.cacheMisses++

	// Match against patterns
	for _, sp := range sd.patterns {
		if sp.pattern.MatchString(userAgent) {
			// Cache the result
			sd.cache.Store(userAgent, sp.spiderType)

			info := spiderConfigs[sp.spiderType]
			return &models.DetectionResult{
				IsSpider:   true,
				SpiderType: sp.spiderType,
				SpiderName: info.Name,
				UserAgent:  userAgent,
			}
		}
	}

	// Not a spider, cache negative result
	sd.cache.Store(userAgent, "")

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
	var cacheSize int
	sd.cache.Range(func(_, _ interface{}) bool {
		cacheSize++
		return true
	})

	return map[string]interface{}{
		"cache_size": cacheSize,
		"cache_hits": sd.cacheHits,
		"cache_misses": sd.cacheMisses,
	}
}

// Global spider detector instance
var globalSpiderDetector *SpiderDetector
var spiderDetectorOnce sync.Once

// GetSpiderDetector returns the global spider detector instance
func GetSpiderDetector() *SpiderDetector {
	spiderDetectorOnce.Do(func() {
		globalSpiderDetector = NewSpiderDetector()
	})
	return globalSpiderDetector
}
