package core

import (
	"crypto/md5"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TemplateFuncStats 模板函数调用统计
type TemplateFuncStats struct {
	Cls               int `json:"cls"`                 // cls() 调用次数
	RandomURL         int `json:"random_url"`          // random_url() 调用次数
	KeywordWithEmoji  int `json:"keyword_with_emoji"`  // keyword_with_emoji() 调用次数
	RandomKeyword     int `json:"random_keyword"`      // random_keyword() 调用次数
	RandomImage       int `json:"random_image"`        // random_image() 调用次数
	RandomTitle       int `json:"random_title"`        // random_title() 调用次数
	RandomContent     int `json:"random_content"`      // random_content() 调用次数
	ContentWithPinyin int `json:"content_with_pinyin"` // content_with_pinyin() 调用次数
	RandomNumber      int `json:"random_number"`       // random_number() 调用次数
	Now               int `json:"now"`                 // now() 调用次数
}

// Add 合并统计
func (s *TemplateFuncStats) Add(other *TemplateFuncStats) {
	s.Cls += other.Cls
	s.RandomURL += other.RandomURL
	s.KeywordWithEmoji += other.KeywordWithEmoji
	s.RandomKeyword += other.RandomKeyword
	s.RandomImage += other.RandomImage
	s.RandomTitle += other.RandomTitle
	s.RandomContent += other.RandomContent
	s.ContentWithPinyin += other.ContentWithPinyin
	s.RandomNumber += other.RandomNumber
	s.Now += other.Now
}

// Multiply 乘以倍数（用于循环展开）
func (s *TemplateFuncStats) Multiply(factor int) *TemplateFuncStats {
	return &TemplateFuncStats{
		Cls:               s.Cls * factor,
		RandomURL:         s.RandomURL * factor,
		KeywordWithEmoji:  s.KeywordWithEmoji * factor,
		RandomKeyword:     s.RandomKeyword * factor,
		RandomImage:       s.RandomImage * factor,
		RandomTitle:       s.RandomTitle * factor,
		RandomContent:     s.RandomContent * factor,
		ContentWithPinyin: s.ContentWithPinyin * factor,
		RandomNumber:      s.RandomNumber * factor,
		Now:               s.Now * factor,
	}
}

// Max 取各字段最大值
func (s *TemplateFuncStats) Max(other *TemplateFuncStats) {
	if other.Cls > s.Cls {
		s.Cls = other.Cls
	}
	if other.RandomURL > s.RandomURL {
		s.RandomURL = other.RandomURL
	}
	if other.KeywordWithEmoji > s.KeywordWithEmoji {
		s.KeywordWithEmoji = other.KeywordWithEmoji
	}
	if other.RandomKeyword > s.RandomKeyword {
		s.RandomKeyword = other.RandomKeyword
	}
	if other.RandomImage > s.RandomImage {
		s.RandomImage = other.RandomImage
	}
	if other.RandomTitle > s.RandomTitle {
		s.RandomTitle = other.RandomTitle
	}
	if other.RandomContent > s.RandomContent {
		s.RandomContent = other.RandomContent
	}
	if other.ContentWithPinyin > s.ContentWithPinyin {
		s.ContentWithPinyin = other.ContentWithPinyin
	}
	if other.RandomNumber > s.RandomNumber {
		s.RandomNumber = other.RandomNumber
	}
	if other.Now > s.Now {
		s.Now = other.Now
	}
}

// Total 返回所有函数调用总次数
func (s *TemplateFuncStats) Total() int {
	return s.Cls + s.RandomURL + s.KeywordWithEmoji + s.RandomKeyword +
		s.RandomImage + s.RandomTitle + s.RandomContent + s.ContentWithPinyin +
		s.RandomNumber + s.Now
}

// TemplateAnalysis 单个模板分析结果
type TemplateAnalysis struct {
	TemplateName string             `json:"template_name"`
	SiteGroupID  int                `json:"site_group_id"`
	ContentHash  string             `json:"content_hash"`
	Stats        *TemplateFuncStats `json:"stats"`
	LoopCount    int                `json:"loop_count"`     // 循环层数
	MaxLoopDepth int                `json:"max_loop_depth"` // 最大嵌套深度
	AnalyzedAt   int64              `json:"analyzed_at"`    // 分析时间戳
}

// PoolSizeConfig 池大小配置
type PoolSizeConfig struct {
	ClsPoolSize          int `json:"cls_pool_size"`
	URLPoolSize          int `json:"url_pool_size"`
	KeywordEmojiPoolSize int `json:"keyword_emoji_pool_size"`
	NumberPoolSize       int `json:"number_pool_size"`
}

// ConfigChangedCallback 配置变化回调类型
type ConfigChangedCallback func(config *PoolSizeConfig)

// TemplateAnalyzer 模板分析器
type TemplateAnalyzer struct {
	mu sync.RWMutex

	// 分析结果缓存: key = "templateName:siteGroupID"
	analyses map[string]*TemplateAnalysis

	// 所有模板的最大统计值
	maxStats *TemplateFuncStats

	// 配置
	targetQPS    int     // 目标 QPS
	safetyFactor float64 // 安全系数

	// 回调
	onConfigChanged ConfigChangedCallback

	// 正则模式
	funcPatterns map[string]*regexp.Regexp
	loopPattern  *regexp.Regexp
}

// 函数匹配模式
var defaultFuncPatterns = map[string]string{
	"cls":                 `\{\{\s*cls\s*\([^)]*\)\s*\}\}`,
	"random_url":          `\{\{\s*random_url\s*\(\s*\)\s*\}\}`,
	"keyword_with_emoji":  `\{\{\s*(keyword_with_emoji|random_keyword_emoji)\s*\(\s*\)\s*\}\}`,
	"random_keyword":      `\{\{\s*random_keyword\s*\(\s*\)\s*\}\}`,
	"random_image":        `\{\{\s*random_image\s*\(\s*\)\s*\}\}`,
	"random_title":        `\{\{\s*random_title\s*\(\s*\)\s*\}\}`,
	"random_content":      `\{\{\s*random_content\s*\(\s*\)\s*\}\}`,
	"content_with_pinyin": `\{\{\s*content_with_pinyin\s*\(\s*\)\s*\}\}`,
	"random_number":       `\{\{\s*random_number\s*\([^)]*\)\s*\}\}`,
	"now":                 `\{\{\s*now\s*\(\s*\)\s*\}\}`,
}

// 循环匹配模式: {% for i in range(N) %} ... {% endfor %}
var defaultLoopPattern = `\{%\s*for\s+\w+\s+in\s+range\s*\(\s*(\d+)\s*\)\s*%\}([\s\S]*?)\{%\s*endfor\s*%\}`

// NewTemplateAnalyzer 创建模板分析器
func NewTemplateAnalyzer() *TemplateAnalyzer {
	analyzer := &TemplateAnalyzer{
		analyses:     make(map[string]*TemplateAnalysis),
		maxStats:     &TemplateFuncStats{},
		targetQPS:    500,
		safetyFactor: 1.5,
	}

	// 编译正则模式
	analyzer.funcPatterns = make(map[string]*regexp.Regexp)
	for name, pattern := range defaultFuncPatterns {
		analyzer.funcPatterns[name] = regexp.MustCompile(pattern)
	}
	analyzer.loopPattern = regexp.MustCompile(defaultLoopPattern)

	return analyzer
}

// AnalyzeTemplate 分析单个模板
func (a *TemplateAnalyzer) AnalyzeTemplate(name string, siteGroupID int, content string) *TemplateAnalysis {
	// 计算内容哈希
	hash := a.hashContent(content)
	key := a.cacheKey(name, siteGroupID)

	// 使用写锁覆盖整个检查-分析-写入流程，避免 TOCTOU 竞态
	a.mu.Lock()

	// 检查是否已分析且内容未变（在锁内检查）
	if existing, ok := a.analyses[key]; ok && existing.ContentHash == hash {
		a.mu.Unlock()
		log.Debug().
			Str("template", name).
			Int("site_group_id", siteGroupID).
			Msg("Template content unchanged, skipping analysis")
		return existing
	}
	a.mu.Unlock()

	// 分析内容（在锁外进行，避免长时间持有锁）
	stats, loopCount, maxDepth := a.analyzeContent(content)

	analysis := &TemplateAnalysis{
		TemplateName: name,
		SiteGroupID:  siteGroupID,
		ContentHash:  hash,
		Stats:        stats,
		LoopCount:    loopCount,
		MaxLoopDepth: maxDepth,
		AnalyzedAt:   currentTimestamp(),
	}

	// 再次获取锁并检查（双重检查锁定模式）
	a.mu.Lock()
	// 再次检查，防止在分析期间另一个 goroutine 已经更新
	if existing, ok := a.analyses[key]; ok && existing.ContentHash == hash {
		a.mu.Unlock()
		log.Debug().
			Str("template", name).
			Int("site_group_id", siteGroupID).
			Msg("Template already analyzed by another goroutine")
		return existing
	}
	a.analyses[key] = analysis
	a.mu.Unlock()

	// 重新计算最大值
	a.recalculateMaxStats()

	log.Info().
		Str("template", name).
		Int("site_group_id", siteGroupID).
		Int("total_calls", stats.Total()).
		Int("loop_count", loopCount).
		Int("max_depth", maxDepth).
		Msg("Template analyzed")

	return analysis
}

// analyzeContent 分析内容（含循环展开）
func (a *TemplateAnalyzer) analyzeContent(content string) (stats *TemplateFuncStats, loopCount int, maxDepth int) {
	stats = &TemplateFuncStats{}

	// 展开循环
	expandedContent, loopCount, maxDepth := a.expandLoops(content, 0, 10)

	// 统计各函数调用次数
	stats.Cls = len(a.funcPatterns["cls"].FindAllString(expandedContent, -1))
	stats.RandomURL = len(a.funcPatterns["random_url"].FindAllString(expandedContent, -1))
	stats.KeywordWithEmoji = len(a.funcPatterns["keyword_with_emoji"].FindAllString(expandedContent, -1))
	stats.RandomKeyword = len(a.funcPatterns["random_keyword"].FindAllString(expandedContent, -1))
	stats.RandomImage = len(a.funcPatterns["random_image"].FindAllString(expandedContent, -1))
	stats.RandomTitle = len(a.funcPatterns["random_title"].FindAllString(expandedContent, -1))
	stats.RandomContent = len(a.funcPatterns["random_content"].FindAllString(expandedContent, -1))
	stats.ContentWithPinyin = len(a.funcPatterns["content_with_pinyin"].FindAllString(expandedContent, -1))
	stats.RandomNumber = len(a.funcPatterns["random_number"].FindAllString(expandedContent, -1))
	stats.Now = len(a.funcPatterns["now"].FindAllString(expandedContent, -1))

	return stats, loopCount, maxDepth
}

// expandLoops 展开循环（支持嵌套，最多 maxDepth 层）
func (a *TemplateAnalyzer) expandLoops(content string, currentDepth int, maxDepth int) (expanded string, loopCount int, depth int) {
	if currentDepth >= maxDepth {
		return content, 0, currentDepth
	}

	expanded = content
	totalLoops := 0
	maxReachedDepth := currentDepth

	for {
		matches := a.loopPattern.FindAllStringSubmatchIndex(expanded, -1)
		if len(matches) == 0 {
			break
		}

		// 从后往前替换，避免索引偏移问题
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			fullStart, fullEnd := match[0], match[1]
			countStart, countEnd := match[2], match[3]
			bodyStart, bodyEnd := match[4], match[5]

			countStr := expanded[countStart:countEnd]
			count, err := strconv.Atoi(countStr)
			if err != nil || count <= 0 {
				count = 1
			}
			// 限制最大循环次数，防止内存爆炸
			if count > 1000 {
				count = 1000
			}

			body := expanded[bodyStart:bodyEnd]
			totalLoops++

			// 递归展开嵌套循环
			expandedBody, nestedLoops, nestedDepth := a.expandLoops(body, currentDepth+1, maxDepth)
			totalLoops += nestedLoops
			if nestedDepth > maxReachedDepth {
				maxReachedDepth = nestedDepth
			}

			// 将循环体重复 count 次
			replacement := strings.Repeat(expandedBody, count)
			expanded = expanded[:fullStart] + replacement + expanded[fullEnd:]
		}
	}

	return expanded, totalLoops, maxReachedDepth
}

// hashContent 计算内容哈希
func (a *TemplateAnalyzer) hashContent(content string) string {
	hash := md5.Sum([]byte(content))
	return hex.EncodeToString(hash[:])
}

// cacheKey 生成缓存键
func (a *TemplateAnalyzer) cacheKey(name string, siteGroupID int) string {
	return name + ":" + strconv.Itoa(siteGroupID)
}

// recalculateMaxStats 重新计算所有模板的最大值
func (a *TemplateAnalyzer) recalculateMaxStats() {
	a.mu.Lock()
	defer a.mu.Unlock()

	newMax := &TemplateFuncStats{}
	for _, analysis := range a.analyses {
		newMax.Max(analysis.Stats)
	}
	a.maxStats = newMax

	// 触发回调
	if a.onConfigChanged != nil {
		config := a.calculatePoolSizeInternal()
		go a.onConfigChanged(config)
	}
}

// calculatePoolSizeInternal 内部计算池大小（需要持有锁）
func (a *TemplateAnalyzer) calculatePoolSizeInternal() *PoolSizeConfig {
	// 基于最大统计值和目标 QPS 计算
	// 池大小 = 最大调用次数 * 目标QPS * 安全系数
	multiplier := float64(a.targetQPS) * a.safetyFactor

	return &PoolSizeConfig{
		ClsPoolSize:          int(float64(a.maxStats.Cls) * multiplier),
		URLPoolSize:          int(float64(a.maxStats.RandomURL) * multiplier),
		KeywordEmojiPoolSize: int(float64(a.maxStats.KeywordWithEmoji) * multiplier),
		NumberPoolSize:       int(float64(a.maxStats.RandomNumber) * multiplier),
	}
}

// CalculatePoolSize 计算推荐的池大小
func (a *TemplateAnalyzer) CalculatePoolSize() *PoolSizeConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.calculatePoolSizeInternal()
}

// SetTargetQPS 设置目标 QPS
func (a *TemplateAnalyzer) SetTargetQPS(qps int) {
	a.mu.Lock()
	a.targetQPS = qps
	a.mu.Unlock()

	// 重新计算并触发回调
	a.recalculateMaxStats()

	log.Info().Int("target_qps", qps).Msg("Template analyzer target QPS updated")
}

// SetSafetyFactor 设置安全系数
func (a *TemplateAnalyzer) SetSafetyFactor(factor float64) {
	a.mu.Lock()
	a.safetyFactor = factor
	a.mu.Unlock()

	// 重新计算并触发回调
	a.recalculateMaxStats()

	log.Info().Float64("safety_factor", factor).Msg("Template analyzer safety factor updated")
}

// OnConfigChanged 设置配置变化回调
func (a *TemplateAnalyzer) OnConfigChanged(callback ConfigChangedCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onConfigChanged = callback
}

// GetAllAnalyses 获取所有分析结果
func (a *TemplateAnalyzer) GetAllAnalyses() map[string]*TemplateAnalysis {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[string]*TemplateAnalysis, len(a.analyses))
	for k, v := range a.analyses {
		result[k] = v
	}
	return result
}

// GetAnalysis 获取单个模板的分析结果
func (a *TemplateAnalyzer) GetAnalysis(name string, siteGroupID int) *TemplateAnalysis {
	key := a.cacheKey(name, siteGroupID)
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.analyses[key]
}

// GetMaxStats 获取最大统计值
func (a *TemplateAnalyzer) GetMaxStats() *TemplateFuncStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 返回副本
	return &TemplateFuncStats{
		Cls:               a.maxStats.Cls,
		RandomURL:         a.maxStats.RandomURL,
		KeywordWithEmoji:  a.maxStats.KeywordWithEmoji,
		RandomKeyword:     a.maxStats.RandomKeyword,
		RandomImage:       a.maxStats.RandomImage,
		RandomTitle:       a.maxStats.RandomTitle,
		RandomContent:     a.maxStats.RandomContent,
		ContentWithPinyin: a.maxStats.ContentWithPinyin,
		RandomNumber:      a.maxStats.RandomNumber,
		Now:               a.maxStats.Now,
	}
}

// GetTargetQPS 获取目标 QPS（线程安全）
func (a *TemplateAnalyzer) GetTargetQPS() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.targetQPS
}

// GetSafetyFactor 获取安全系数（线程安全）
func (a *TemplateAnalyzer) GetSafetyFactor() float64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.safetyFactor
}

// GetStats 获取分析器统计信息
func (a *TemplateAnalyzer) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"templates_analyzed": len(a.analyses),
		"target_qps":         a.targetQPS,
		"safety_factor":      a.safetyFactor,
		"max_stats": map[string]int{
			"cls":                 a.maxStats.Cls,
			"random_url":          a.maxStats.RandomURL,
			"keyword_with_emoji":  a.maxStats.KeywordWithEmoji,
			"random_keyword":      a.maxStats.RandomKeyword,
			"random_image":        a.maxStats.RandomImage,
			"random_title":        a.maxStats.RandomTitle,
			"random_content":      a.maxStats.RandomContent,
			"content_with_pinyin": a.maxStats.ContentWithPinyin,
			"random_number":       a.maxStats.RandomNumber,
			"now":                 a.maxStats.Now,
		},
	}
}

// RemoveAnalysis 移除模板分析结果
func (a *TemplateAnalyzer) RemoveAnalysis(name string, siteGroupID int) {
	key := a.cacheKey(name, siteGroupID)
	a.mu.Lock()
	delete(a.analyses, key)
	a.mu.Unlock()

	// 重新计算最大值
	a.recalculateMaxStats()

	log.Info().
		Str("template", name).
		Int("site_group_id", siteGroupID).
		Msg("Template analysis removed")
}

// Clear 清除所有分析结果
func (a *TemplateAnalyzer) Clear() {
	a.mu.Lock()
	a.analyses = make(map[string]*TemplateAnalysis)
	a.maxStats = &TemplateFuncStats{}
	a.mu.Unlock()

	log.Info().Msg("Template analyzer cleared")
}

// 辅助函数：获取当前时间戳
func currentTimestamp() int64 {
	return timeNowFunc()
}

// timeNowFunc 用于测试时的时间模拟（默认使用真实时间）
var timeNowFunc = func() int64 {
	return time.Now().Unix()
}
