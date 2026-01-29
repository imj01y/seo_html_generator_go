# 模板分析器实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 自动分析模板中各函数的调用次数（含循环展开），用于计算缓存池大小和 SEO 友好度评估。

**Architecture:** 正则匹配 + 循环展开算法，检测模板内容变化时自动重新分析，多模板取最大值作为基准。

**Tech Stack:** Go regexp, crypto/sha256

---

## Task 1: 定义数据结构

**Files:**
- Create: `go-page-server/core/template_analyzer.go`

**Step 1: 创建基础数据结构**

```go
package core

import (
	"crypto/sha256"
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
	// 对象池相关（消耗型）
	Cls          int `json:"cls"`
	RandomURL    int `json:"random_url"`
	KeywordEmoji int `json:"keyword_with_emoji"`

	// 数据池相关（选取型）
	RandomKeyword int `json:"random_keyword"`
	RandomImage   int `json:"random_image"`
	RandomTitle   int `json:"random_title"`
	RandomContent int `json:"random_content"`
	ContentPinyin int `json:"content_with_pinyin"`

	// 其他（无需池）
	RandomNumber int `json:"random_number"`
	Now          int `json:"now"`
}

// Total 返回总调用次数
func (s *TemplateFuncStats) Total() int {
	return s.Cls + s.RandomURL + s.KeywordEmoji +
		s.RandomKeyword + s.RandomImage + s.RandomTitle +
		s.RandomContent + s.ContentPinyin +
		s.RandomNumber + s.Now
}

// TemplateAnalysis 单个模板的分析结果
type TemplateAnalysis struct {
	TemplateID  int               `json:"template_id"`
	Name        string            `json:"name"`
	Stats       TemplateFuncStats `json:"stats"`
	AnalyzedAt  time.Time         `json:"analyzed_at"`
	ContentHash string            `json:"content_hash"`
}

// TemplateAnalyzer 模板分析器
type TemplateAnalyzer struct {
	mu              sync.RWMutex
	analyses        map[int]*TemplateAnalysis
	maxStats        TemplateFuncStats
	maxTemplateName string

	// 配置
	targetQPS     int
	safetyFactor  float64
	bufferSeconds float64

	// 回调
	onConfigChanged func(*PoolSizeConfig)
}

// PoolSizeConfig 池大小配置
type PoolSizeConfig struct {
	ClsPoolSize          int `json:"cls_pool_size"`
	URLPoolSize          int `json:"url_pool_size"`
	KeywordEmojiPoolSize int `json:"keyword_emoji_pool_size"`

	BasedOnTemplate string            `json:"based_on_template"`
	MaxStats        TemplateFuncStats `json:"max_stats"`
	TargetQPS       int               `json:"target_qps"`
	SafetyFactor    float64           `json:"safety_factor"`
	BufferSeconds   float64           `json:"buffer_seconds"`
	CalculatedAt    time.Time         `json:"calculated_at"`
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/template_analyzer.go
git commit -m "feat: add template analyzer data structures"
```

---

## Task 2: 实现模板分析逻辑

**Files:**
- Modify: `go-page-server/core/template_analyzer.go`

**Step 1: 添加正则模式和分析方法**

在 `template_analyzer.go` 末尾追加：

```go
var (
	// 函数调用正则
	funcPatterns = map[string]*regexp.Regexp{
		"cls":                 regexp.MustCompile(`\{\{\s*cls\s*\(`),
		"random_url":          regexp.MustCompile(`\{\{\s*random_url\s*\(\s*\)`),
		"keyword_with_emoji":  regexp.MustCompile(`\{\{\s*keyword_with_emoji\s*\(\s*\)`),
		"random_keyword":      regexp.MustCompile(`\{\{\s*random_keyword\s*\(\s*\)`),
		"random_image":        regexp.MustCompile(`\{\{\s*random_image\s*\(\s*\)`),
		"random_title":        regexp.MustCompile(`\{\{\s*random_title\s*\(\s*\)`),
		"random_content":      regexp.MustCompile(`\{\{\s*random_content\s*\(\s*\)`),
		"content_with_pinyin": regexp.MustCompile(`\{\{\s*content_with_pinyin\s*\(\s*\)`),
		"random_number":       regexp.MustCompile(`\{\{\s*random_number\s*\(`),
		"now":                 regexp.MustCompile(`\{\{\s*now\s*\(`),
	}

	// 循环匹配正则: {% for i in range(N) %} ... {% endfor %}
	loopPattern = regexp.MustCompile(`\{%\s*for\s+\w+\s+in\s+range\s*\(\s*(\d+)\s*\)\s*%\}([\s\S]*?)\{%\s*endfor\s*%\}`)
)

// NewTemplateAnalyzer 创建模板分析器
func NewTemplateAnalyzer(targetQPS int, safetyFactor, bufferSeconds float64) *TemplateAnalyzer {
	return &TemplateAnalyzer{
		analyses:      make(map[int]*TemplateAnalysis),
		targetQPS:     targetQPS,
		safetyFactor:  safetyFactor,
		bufferSeconds: bufferSeconds,
	}
}

// AnalyzeTemplate 分析单个模板
func (a *TemplateAnalyzer) AnalyzeTemplate(templateID int, name, content string) *TemplateAnalysis {
	stats := a.analyzeContent(content)
	contentHash := a.hashContent(content)

	analysis := &TemplateAnalysis{
		TemplateID:  templateID,
		Name:        name,
		Stats:       stats,
		AnalyzedAt:  time.Now(),
		ContentHash: contentHash,
	}

	a.mu.Lock()
	oldAnalysis, existed := a.analyses[templateID]
	a.analyses[templateID] = analysis
	a.mu.Unlock()

	// 检查是否需要重新计算池大小
	needRecalc := !existed || oldAnalysis.ContentHash != contentHash
	if needRecalc {
		a.recalculateMaxStats()
	}

	log.Info().
		Int("template_id", templateID).
		Str("name", name).
		Int("cls", stats.Cls).
		Int("url", stats.RandomURL).
		Int("keyword", stats.RandomKeyword).
		Int("image", stats.RandomImage).
		Int("total", stats.Total()).
		Msg("Template analyzed")

	return analysis
}

// analyzeContent 分析模板内容
func (a *TemplateAnalyzer) analyzeContent(content string) TemplateFuncStats {
	var stats TemplateFuncStats

	// 递归处理嵌套循环
	processedContent := a.expandLoops(content)

	// 统计各函数调用次数
	stats.Cls = len(funcPatterns["cls"].FindAllString(processedContent, -1))
	stats.RandomURL = len(funcPatterns["random_url"].FindAllString(processedContent, -1))
	stats.KeywordEmoji = len(funcPatterns["keyword_with_emoji"].FindAllString(processedContent, -1))
	stats.RandomKeyword = len(funcPatterns["random_keyword"].FindAllString(processedContent, -1))
	stats.RandomImage = len(funcPatterns["random_image"].FindAllString(processedContent, -1))
	stats.RandomTitle = len(funcPatterns["random_title"].FindAllString(processedContent, -1))
	stats.RandomContent = len(funcPatterns["random_content"].FindAllString(processedContent, -1))
	stats.ContentPinyin = len(funcPatterns["content_with_pinyin"].FindAllString(processedContent, -1))
	stats.RandomNumber = len(funcPatterns["random_number"].FindAllString(processedContent, -1))
	stats.Now = len(funcPatterns["now"].FindAllString(processedContent, -1))

	return stats
}

// expandLoops 展开循环（处理嵌套）
func (a *TemplateAnalyzer) expandLoops(content string) string {
	// 最多展开10层嵌套，防止无限循环
	for i := 0; i < 10; i++ {
		matches := loopPattern.FindAllStringSubmatch(content, -1)
		if len(matches) == 0 {
			break
		}

		for _, match := range matches {
			fullMatch := match[0]
			countStr := match[1]
			loopBody := match[2]

			count, _ := strconv.Atoi(countStr)

			// 将循环体重复 count 次（用于统计）
			expanded := strings.Repeat(loopBody, count)

			content = strings.Replace(content, fullMatch, expanded, 1)
		}
	}

	return content
}

// hashContent 计算内容哈希
func (a *TemplateAnalyzer) hashContent(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/template_analyzer.go
git commit -m "feat: implement template analysis logic"
```

---

## Task 3: 实现最大值计算和池配置

**Files:**
- Modify: `go-page-server/core/template_analyzer.go`

**Step 1: 添加最大值计算方法**

在 `template_analyzer.go` 末尾追加：

```go
// recalculateMaxStats 重新计算所有模板的最大值
func (a *TemplateAnalyzer) recalculateMaxStats() {
	a.mu.Lock()
	defer a.mu.Unlock()

	var maxStats TemplateFuncStats
	var maxTemplateName string
	var maxTotal int

	for _, analysis := range a.analyses {
		// 取各字段的最大值
		if analysis.Stats.Cls > maxStats.Cls {
			maxStats.Cls = analysis.Stats.Cls
		}
		if analysis.Stats.RandomURL > maxStats.RandomURL {
			maxStats.RandomURL = analysis.Stats.RandomURL
		}
		if analysis.Stats.KeywordEmoji > maxStats.KeywordEmoji {
			maxStats.KeywordEmoji = analysis.Stats.KeywordEmoji
		}
		if analysis.Stats.RandomKeyword > maxStats.RandomKeyword {
			maxStats.RandomKeyword = analysis.Stats.RandomKeyword
		}
		if analysis.Stats.RandomImage > maxStats.RandomImage {
			maxStats.RandomImage = analysis.Stats.RandomImage
		}
		if analysis.Stats.RandomTitle > maxStats.RandomTitle {
			maxStats.RandomTitle = analysis.Stats.RandomTitle
		}
		if analysis.Stats.RandomContent > maxStats.RandomContent {
			maxStats.RandomContent = analysis.Stats.RandomContent
		}
		if analysis.Stats.ContentPinyin > maxStats.ContentPinyin {
			maxStats.ContentPinyin = analysis.Stats.ContentPinyin
		}

		// 记录总调用最多的模板名称
		if analysis.Stats.Total() > maxTotal {
			maxTotal = analysis.Stats.Total()
			maxTemplateName = analysis.Name
		}
	}

	a.maxStats = maxStats
	a.maxTemplateName = maxTemplateName

	// 触发回调
	if a.onConfigChanged != nil {
		config := a.calculatePoolSizeInternal()
		a.onConfigChanged(config)
	}

	log.Info().
		Str("max_template", maxTemplateName).
		Int("max_cls", maxStats.Cls).
		Int("max_url", maxStats.RandomURL).
		Int("max_keyword_emoji", maxStats.KeywordEmoji).
		Msg("Max stats recalculated")
}

// calculatePoolSizeInternal 内部计算池大小（需要持有锁）
func (a *TemplateAnalyzer) calculatePoolSizeInternal() *PoolSizeConfig {
	multiplier := float64(a.targetQPS) * a.safetyFactor * a.bufferSeconds

	return &PoolSizeConfig{
		ClsPoolSize:          int(float64(a.maxStats.Cls) * multiplier),
		URLPoolSize:          int(float64(a.maxStats.RandomURL) * multiplier),
		KeywordEmojiPoolSize: int(float64(a.maxStats.KeywordEmoji) * multiplier),

		BasedOnTemplate: a.maxTemplateName,
		MaxStats:        a.maxStats,
		TargetQPS:       a.targetQPS,
		SafetyFactor:    a.safetyFactor,
		BufferSeconds:   a.bufferSeconds,
		CalculatedAt:    time.Now(),
	}
}

// CalculatePoolSize 计算推荐的池大小
func (a *TemplateAnalyzer) CalculatePoolSize() *PoolSizeConfig {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.calculatePoolSizeInternal()
}

// SetTargetQPS 更新目标QPS并重新计算
func (a *TemplateAnalyzer) SetTargetQPS(qps int) *PoolSizeConfig {
	a.mu.Lock()
	a.targetQPS = qps
	config := a.calculatePoolSizeInternal()
	a.mu.Unlock()

	if a.onConfigChanged != nil {
		a.onConfigChanged(config)
	}
	return config
}

// SetSafetyFactor 更新安全系数
func (a *TemplateAnalyzer) SetSafetyFactor(factor float64) *PoolSizeConfig {
	a.mu.Lock()
	a.safetyFactor = factor
	config := a.calculatePoolSizeInternal()
	a.mu.Unlock()

	if a.onConfigChanged != nil {
		a.onConfigChanged(config)
	}
	return config
}

// OnConfigChanged 设置池配置变化回调
func (a *TemplateAnalyzer) OnConfigChanged(callback func(*PoolSizeConfig)) {
	a.onConfigChanged = callback
}

// GetAllAnalyses 获取所有模板分析结果
func (a *TemplateAnalyzer) GetAllAnalyses() map[int]*TemplateAnalysis {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make(map[int]*TemplateAnalysis, len(a.analyses))
	for k, v := range a.analyses {
		result[k] = v
	}
	return result
}

// GetAnalysis 获取单个模板分析结果
func (a *TemplateAnalyzer) GetAnalysis(templateID int) (*TemplateAnalysis, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	analysis, ok := a.analyses[templateID]
	return analysis, ok
}

// GetMaxStats 获取最大统计值
func (a *TemplateAnalyzer) GetMaxStats() (TemplateFuncStats, string) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.maxStats, a.maxTemplateName
}

// GetStats 获取统计信息
func (a *TemplateAnalyzer) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"template_count": len(a.analyses),
		"max_template":   a.maxTemplateName,
		"max_stats":      a.maxStats,
		"target_qps":     a.targetQPS,
		"safety_factor":  a.safetyFactor,
		"buffer_seconds": a.bufferSeconds,
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/template_analyzer.go
git commit -m "feat: add pool size calculation and config management"
```

---

## Task 4: 实现 SEO 友好度分析

**Files:**
- Create: `go-page-server/core/seo_analyzer.go`

**Step 1: 创建 SEO 分析器**

```go
package core

import "fmt"

// SEORating SEO评级
type SEORating string

const (
	SEORatingExcellent SEORating = "excellent" // < 5%
	SEORatingGood      SEORating = "good"      // 5-15%
	SEORatingFair      SEORating = "fair"      // 15-30%
	SEORatingPoor      SEORating = "poor"      // > 30%
)

// DataPoolSEOAnalysis 数据池SEO分析结果
type DataPoolSEOAnalysis struct {
	DataType           string    `json:"data_type"`
	PoolSize           int       `json:"pool_size"`
	CallsPerPage       int       `json:"calls_per_page"`
	ExpectedRepeatRate float64   `json:"expected_repeat_rate"`
	Rating             SEORating `json:"rating"`
	Suggestion         string    `json:"suggestion"`
}

// AnalyzeSEOFriendliness 分析SEO友好度
func (a *TemplateAnalyzer) AnalyzeSEOFriendliness(dataStats DataPoolStats) []DataPoolSEOAnalysis {
	a.mu.RLock()
	maxStats := a.maxStats
	a.mu.RUnlock()

	results := []DataPoolSEOAnalysis{}

	// 分析各数据池
	results = append(results, analyzePool("关键词", dataStats.KeywordsCount, maxStats.RandomKeyword))
	results = append(results, analyzePool("图片", dataStats.ImagesCount, maxStats.RandomImage))
	results = append(results, analyzePool("标题", dataStats.TitlesCount, maxStats.RandomTitle))
	results = append(results, analyzePool("正文", dataStats.ContentsCount, maxStats.ContentPinyin+maxStats.RandomContent))

	return results
}

// DataPoolStats 数据池统计
type DataPoolStats struct {
	KeywordsCount int `json:"keywords_count"`
	ImagesCount   int `json:"images_count"`
	TitlesCount   int `json:"titles_count"`
	ContentsCount int `json:"contents_count"`
}

func analyzePool(dataType string, poolSize, callsPerPage int) DataPoolSEOAnalysis {
	analysis := DataPoolSEOAnalysis{
		DataType:     dataType,
		PoolSize:     poolSize,
		CallsPerPage: callsPerPage,
	}

	if callsPerPage == 0 {
		analysis.ExpectedRepeatRate = 0
		analysis.Rating = SEORatingExcellent
		analysis.Suggestion = "模板未使用"
		return analysis
	}

	if poolSize == 0 {
		analysis.ExpectedRepeatRate = 100
		analysis.Rating = SEORatingPoor
		analysis.Suggestion = "数据池为空，请添加数据"
		return analysis
	}

	// 计算预期重复率
	analysis.ExpectedRepeatRate = float64(callsPerPage) / float64(poolSize) * 100
	if analysis.ExpectedRepeatRate > 100 {
		analysis.ExpectedRepeatRate = 100
	}

	// 评级
	switch {
	case analysis.ExpectedRepeatRate < 5:
		analysis.Rating = SEORatingExcellent
		analysis.Suggestion = "SEO友好度优秀"
	case analysis.ExpectedRepeatRate < 15:
		analysis.Rating = SEORatingGood
		analysis.Suggestion = "SEO友好度良好"
	case analysis.ExpectedRepeatRate < 30:
		analysis.Rating = SEORatingFair
		needed := callsPerPage * 100 / 15
		analysis.Suggestion = fmt.Sprintf("建议增加到 %d 条以上", needed)
	default:
		analysis.Rating = SEORatingPoor
		needed := callsPerPage * 100 / 15
		analysis.Suggestion = fmt.Sprintf("重复率过高，强烈建议增加到 %d 条以上", needed)
	}

	return analysis
}

// GetRecommendedPoolSize 获取推荐的数据池大小
func GetRecommendedPoolSize(callsPerPage int, targetRepeatRate float64) int {
	if targetRepeatRate <= 0 {
		targetRepeatRate = 5
	}
	return int(float64(callsPerPage) * 100 / targetRepeatRate)
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/seo_analyzer.go
git commit -m "feat: add SEO friendliness analyzer"
```

---

## Task 5: 添加测试

**Files:**
- Create: `go-page-server/core/template_analyzer_test.go`

**Step 1: 创建测试文件**

```go
package core

import (
	"testing"
)

func TestTemplateAnalyzer_AnalyzeContent(t *testing.T) {
	analyzer := NewTemplateAnalyzer(500, 1.5, 1.0)

	content := `
<!DOCTYPE html>
<html>
<head><title>{{ title }}</title></head>
<body>
<div class="{{ cls('box') }}">{{ random_keyword() }}</div>
{% for i in range(10) %}
<a href="{{ random_url() }}">{{ random_keyword() }}</a>
<img src="{{ random_image() }}">
{% endfor %}
<p>{{ content_with_pinyin() }}</p>
</body>
</html>
`

	analysis := analyzer.AnalyzeTemplate(1, "test.html", content)

	// 验证统计结果
	// cls: 1 (循环外)
	// random_url: 10 (循环内 1*10)
	// random_keyword: 11 (循环外1 + 循环内1*10)
	// random_image: 10 (循环内 1*10)
	// content_with_pinyin: 1

	if analysis.Stats.Cls != 1 {
		t.Errorf("Expected Cls=1, got %d", analysis.Stats.Cls)
	}
	if analysis.Stats.RandomURL != 10 {
		t.Errorf("Expected RandomURL=10, got %d", analysis.Stats.RandomURL)
	}
	if analysis.Stats.RandomKeyword != 11 {
		t.Errorf("Expected RandomKeyword=11, got %d", analysis.Stats.RandomKeyword)
	}
	if analysis.Stats.RandomImage != 10 {
		t.Errorf("Expected RandomImage=10, got %d", analysis.Stats.RandomImage)
	}
	if analysis.Stats.ContentPinyin != 1 {
		t.Errorf("Expected ContentPinyin=1, got %d", analysis.Stats.ContentPinyin)
	}
}

func TestTemplateAnalyzer_NestedLoops(t *testing.T) {
	analyzer := NewTemplateAnalyzer(500, 1.5, 1.0)

	content := `
{% for i in range(5) %}
{% for j in range(10) %}
{{ cls('item') }}
{% endfor %}
{% endfor %}
`

	analysis := analyzer.AnalyzeTemplate(1, "nested.html", content)

	// 5 * 10 = 50
	if analysis.Stats.Cls != 50 {
		t.Errorf("Expected Cls=50, got %d", analysis.Stats.Cls)
	}
}

func TestTemplateAnalyzer_PoolSizeCalculation(t *testing.T) {
	analyzer := NewTemplateAnalyzer(500, 1.5, 1.0)

	content := `
{% for i in range(100) %}
{{ cls('x') }}{{ random_url() }}
{% endfor %}
`

	analyzer.AnalyzeTemplate(1, "test.html", content)

	config := analyzer.CalculatePoolSize()

	// cls: 100 * 500 * 1.5 * 1 = 75000
	// url: 100 * 500 * 1.5 * 1 = 75000
	if config.ClsPoolSize != 75000 {
		t.Errorf("Expected ClsPoolSize=75000, got %d", config.ClsPoolSize)
	}
	if config.URLPoolSize != 75000 {
		t.Errorf("Expected URLPoolSize=75000, got %d", config.URLPoolSize)
	}
}

func TestTemplateAnalyzer_MaxStats(t *testing.T) {
	analyzer := NewTemplateAnalyzer(500, 1.5, 1.0)

	// 模板1: cls=100
	analyzer.AnalyzeTemplate(1, "t1.html", `{% for i in range(100) %}{{ cls('x') }}{% endfor %}`)

	// 模板2: cls=200
	analyzer.AnalyzeTemplate(2, "t2.html", `{% for i in range(200) %}{{ cls('x') }}{% endfor %}`)

	maxStats, maxName := analyzer.GetMaxStats()

	if maxStats.Cls != 200 {
		t.Errorf("Expected max Cls=200, got %d", maxStats.Cls)
	}
	if maxName != "t2.html" {
		t.Errorf("Expected max template name=t2.html, got %s", maxName)
	}
}

func TestTemplateAnalyzer_ContentHashChange(t *testing.T) {
	analyzer := NewTemplateAnalyzer(500, 1.5, 1.0)

	configChanged := false
	analyzer.OnConfigChanged(func(config *PoolSizeConfig) {
		configChanged = true
	})

	// 第一次分析
	analyzer.AnalyzeTemplate(1, "test.html", `{{ cls('a') }}`)
	if !configChanged {
		t.Error("Expected config changed callback on first analysis")
	}

	// 相同内容再次分析，不应触发回调
	configChanged = false
	analyzer.AnalyzeTemplate(1, "test.html", `{{ cls('a') }}`)
	if configChanged {
		t.Error("Should not trigger callback for same content")
	}

	// 不同内容分析，应触发回调
	configChanged = false
	analyzer.AnalyzeTemplate(1, "test.html", `{{ cls('b') }}`)
	if !configChanged {
		t.Error("Expected config changed callback for different content")
	}
}

func TestSEOAnalysis(t *testing.T) {
	analyzer := NewTemplateAnalyzer(500, 1.5, 1.0)

	// 模拟模板分析结果
	content := `{% for i in range(100) %}{{ random_keyword() }}{% endfor %}`
	analyzer.AnalyzeTemplate(1, "test.html", content)

	// 模拟数据池统计
	dataStats := DataPoolStats{
		KeywordsCount: 1000,  // 重复率 10%
		ImagesCount:   5000,  // 无使用
		TitlesCount:   100,   // 无使用
		ContentsCount: 50,    // 无使用
	}

	results := analyzer.AnalyzeSEOFriendliness(dataStats)

	// 检查关键词分析
	keywordAnalysis := results[0]
	if keywordAnalysis.DataType != "关键词" {
		t.Errorf("Expected DataType=关键词, got %s", keywordAnalysis.DataType)
	}
	if keywordAnalysis.Rating != SEORatingGood {
		t.Errorf("Expected Rating=good, got %s", keywordAnalysis.Rating)
	}
	if keywordAnalysis.ExpectedRepeatRate != 10 {
		t.Errorf("Expected RepeatRate=10, got %f", keywordAnalysis.ExpectedRepeatRate)
	}
}
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestTemplateAnalyzer
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/template_analyzer_test.go
git commit -m "test: add template analyzer tests"
```

---

## Task 6: 集成到模板缓存

**Files:**
- Modify: `go-page-server/core/template_cache.go`

**Step 1: 在模板加载时触发分析**

在 `LoadTemplate` 和 `LoadAll` 方法中添加分析调用：

```go
// 在 TemplateCache 结构体中添加分析器引用
type TemplateCache struct {
	// ... existing fields
	analyzer *TemplateAnalyzer
}

// SetAnalyzer 设置模板分析器
func (c *TemplateCache) SetAnalyzer(analyzer *TemplateAnalyzer) {
	c.analyzer = analyzer
}

// 在 LoadTemplate 方法末尾添加：
func (c *TemplateCache) LoadTemplate(ctx context.Context, templateID int) error {
	// ... existing code ...

	// 触发模板分析
	if c.analyzer != nil && tpl != nil {
		c.analyzer.AnalyzeTemplate(templateID, tpl.Name, tpl.Code)
	}

	return nil
}

// 在 LoadAll 方法末尾添加：
func (c *TemplateCache) LoadAll(ctx context.Context) error {
	// ... existing code ...

	// 分析所有模板
	if c.analyzer != nil {
		c.mu.RLock()
		for id, tpl := range c.templates {
			c.analyzer.AnalyzeTemplate(id, tpl.Name, tpl.Code)
		}
		c.mu.RUnlock()
	}

	return nil
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/template_cache.go
git commit -m "feat: integrate template analyzer with template cache"
```

---

## Task 7: 更新 main.go 初始化

**Files:**
- Modify: `go-page-server/main.go`

**Step 1: 添加模板分析器初始化**

在 `main()` 函数中，templateCache 初始化后添加：

```go
// 初始化模板分析器
templateAnalyzer := core.NewTemplateAnalyzer(500, 1.5, 1.0)
templateCache.SetAnalyzer(templateAnalyzer)

// 设置回调：当池配置变化时自动调整
templateAnalyzer.OnConfigChanged(func(config *core.PoolSizeConfig) {
	log.Info().
		Int("cls_pool", config.ClsPoolSize).
		Int("url_pool", config.URLPoolSize).
		Int("keyword_emoji_pool", config.KeywordEmojiPoolSize).
		Str("based_on", config.BasedOnTemplate).
		Msg("Pool size config updated based on template analysis")

	// TODO: 调用 funcsManager.ResizePools(config) 动态调整池大小
})
```

**Step 2: Commit**

```bash
git add go-page-server/main.go
git commit -m "feat: initialize template analyzer in main"
```

---

## 完成检查清单

- [ ] Task 1: 数据结构定义
- [ ] Task 2: 模板分析逻辑
- [ ] Task 3: 池大小计算
- [ ] Task 4: SEO 友好度分析
- [ ] Task 5: 测试覆盖
- [ ] Task 6: 集成到模板缓存
- [ ] Task 7: main.go 初始化

所有任务完成后运行完整测试：

```bash
cd go-page-server && go test -v ./...
```