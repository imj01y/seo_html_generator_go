package core

import (
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	models "seo-generator/api/internal/model"
)

// spiderKeyword 蜘蛛关键词匹配规则
type spiderKeyword struct {
	Keyword string // 小写关键词
	Type    string // 蜘蛛类型标识
	Name    string // 蜘蛛中文名
}

// spiderKeywords 硬编码的蜘蛛关键词表（全部小写，用于大小写无关匹配）
var spiderKeywords = []spiderKeyword{
	{"baiduspider", "baidu", "百度蜘蛛"},
	{"baidu-yunguance", "baidu", "百度蜘蛛"},
	{"googlebot", "google", "谷歌蜘蛛"},
	{"google-inspectiontool", "google", "谷歌蜘蛛"},
	{"adsbot-google", "google", "谷歌蜘蛛"},
	{"mediapartners-google", "google", "谷歌蜘蛛"},
	{"bingbot", "bing", "必应蜘蛛"},
	{"msnbot", "bing", "必应蜘蛛"},
	{"bingpreview", "bing", "必应蜘蛛"},
	{"sogou", "sogou", "搜狗蜘蛛"},
	{"360spider", "360", "360蜘蛛"},
	{"haosoupider", "360", "360蜘蛛"},
	{"360jk", "360", "360蜘蛛"},
	{"yisouspider", "shenma", "神马蜘蛛"},
	{"bytespider", "bytedance", "字节跳动蜘蛛"},
	{"bytedance", "bytedance", "字节跳动蜘蛛"},
	{"yandexbot", "yandex", "Yandex蜘蛛"},
	{"yandeximages", "yandex", "Yandex蜘蛛"},
	{"yandexmobilebot", "yandex", "Yandex蜘蛛"},
	{"applebot", "other", "其他蜘蛛"},
	{"duckduckbot", "other", "其他蜘蛛"},
	{"facebookexternalhit", "other", "其他蜘蛛"},
	{"twitterbot", "other", "其他蜘蛛"},
	{"linkedinbot", "other", "其他蜘蛛"},
	{"slurp", "other", "其他蜘蛛"},
	{"ia_archiver", "other", "其他蜘蛛"},
}

// spiderTypeMap 按 type 去重的蜘蛛信息（用于 GetAllSpiderTypes / GetSpiderInfo）
var spiderTypeMap map[string]*SpiderInfo

func init() {
	spiderTypeMap = make(map[string]*SpiderInfo)
	for _, kw := range spiderKeywords {
		if _, exists := spiderTypeMap[kw.Type]; !exists {
			spiderTypeMap[kw.Type] = &SpiderInfo{
				Type: kw.Type,
				Name: kw.Name,
			}
		}
	}
}

// SpiderInfo holds spider information
type SpiderInfo struct {
	Type string
	Name string
}

// SpiderDetector detects search engine spiders by User-Agent keyword matching
type SpiderDetector struct{}

// Detect 检测 User-Agent 是否为蜘蛛
func (sd *SpiderDetector) Detect(userAgent string) *models.DetectionResult {
	if userAgent == "" {
		return &models.DetectionResult{IsSpider: false, UserAgent: userAgent}
	}

	lowerUA := strings.ToLower(userAgent)
	for _, kw := range spiderKeywords {
		if strings.Contains(lowerUA, kw.Keyword) {
			return &models.DetectionResult{
				IsSpider:   true,
				SpiderType: kw.Type,
				SpiderName: kw.Name,
				UserAgent:  userAgent,
			}
		}
	}

	return &models.DetectionResult{IsSpider: false, UserAgent: userAgent}
}

// IsSpider 快速判断 UA 是否为蜘蛛
func (sd *SpiderDetector) IsSpider(userAgent string) bool {
	return sd.Detect(userAgent).IsSpider
}

// GetSpiderInfo 返回指定蜘蛛类型的信息
func (sd *SpiderDetector) GetSpiderInfo(spiderType string) *SpiderInfo {
	return spiderTypeMap[spiderType]
}

// GetAllSpiderTypes 返回所有蜘蛛类型
func (sd *SpiderDetector) GetAllSpiderTypes() []string {
	types := make([]string, 0, len(spiderTypeMap))
	for t := range spiderTypeMap {
		types = append(types, t)
	}
	return types
}

// GetStats 返回统计信息（简化版，无缓存）
func (sd *SpiderDetector) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"mode":           "keyword",
		"keyword_count":  len(spiderKeywords),
		"spider_types":   len(spiderTypeMap),
	}
}

// Global singleton
var globalSpiderDetector *SpiderDetector
var spiderDetectorOnce sync.Once

// GetSpiderDetector 返回全局蜘蛛检测器实例
func GetSpiderDetector() *SpiderDetector {
	spiderDetectorOnce.Do(func() {
		globalSpiderDetector = &SpiderDetector{}
		log.Info().Int("keywords", len(spiderKeywords)).Msg("Spider detector initialized (keyword mode)")
	})
	return globalSpiderDetector
}

