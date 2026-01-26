package core

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"html/template"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TemplateRenderer handles template parsing and rendering
type TemplateRenderer struct {
	converter      *TemplateConverter
	funcsManager   *TemplateFuncsManager
	compiledCache  sync.Map // cache key -> *template.Template
	fastRenderer   *FastRenderer
	preloadContent string // Pre-loaded content for content() function
	mu             sync.Mutex
}

// RenderData holds data passed to templates
type RenderData struct {
	Title          string
	SiteID         int
	AnalyticsCode  template.HTML
	BaiduPushJS    template.HTML
	ArticleContent template.HTML
	Now            string
	Content        string

	// Function results (called during render)
	randomKeyword func() string
	randomURL     func() string
	randomImage   func() string
	cls           func(name string) string
	encode        func(text string) string
	randomNumber  func(min, max int) int
}

// NewTemplateRenderer creates a new template renderer
func NewTemplateRenderer(funcsManager *TemplateFuncsManager) *TemplateRenderer {
	return &TemplateRenderer{
		converter:    GetTemplateConverter(),
		funcsManager: funcsManager,
		fastRenderer: NewFastRenderer(funcsManager),
	}
}

// SetPreloadContent sets the pre-loaded content for the content() function
func (r *TemplateRenderer) SetPreloadContent(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preloadContent = content
}

// GetPreloadContent gets and clears the pre-loaded content
func (r *TemplateRenderer) GetPreloadContent() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	content := r.preloadContent
	r.preloadContent = ""
	return content
}

// Render renders a Jinja2 template with the given data
func (r *TemplateRenderer) Render(templateContent string, templateName string, data *RenderData) (string, error) {
	startTime := time.Now()

	// Generate cache key from template content hash
	hash := md5.Sum([]byte(templateContent))
	cacheKey := hex.EncodeToString(hash[:])

	// 1. 尝试快速渲染（绕过反射）
	if result, ok := r.fastRenderer.Render(cacheKey, data); ok {
		elapsed := time.Since(startTime)
		log.Debug().
			Str("template", templateName).
			Dur("duration", elapsed).
			Int("output_size", len(result)).
			Bool("fast_render", true).
			Msg("Template rendered (fast)")
		return result, nil
	}

	// 2. 首次渲染：使用 MarkerContext 生成占位符模板
	// Check compiled template cache
	var tmpl *template.Template
	if cached, ok := r.compiledCache.Load(cacheKey); ok {
		tmpl = cached.(*template.Template)
	} else {
		// Convert Jinja2 to Go template syntax
		goTemplate := r.converter.Convert(templateContent)

		// Create template with custom functions
		funcMap := template.FuncMap{
			"iterate": IterateFunc,
		}

		var err error
		tmpl, err = template.New(templateName).Funcs(funcMap).Parse(goTemplate)
		if err != nil {
			log.Error().Err(err).Str("template", templateName).Msg("Failed to parse template")
			return "", err
		}

		// Cache compiled template
		r.compiledCache.Store(cacheKey, tmpl)
	}

	// 使用 MarkerContext 渲染，收集占位符
	content := r.GetPreloadContent()
	markerCtx := NewMarkerContext(data, content)

	// 从对象池获取 buffer
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	if err := tmpl.Execute(buf, markerCtx); err != nil {
		bufferPool.Put(buf)
		log.Error().Err(err).Str("template", templateName).Msg("Failed to execute template with marker context")
		return "", err
	}

	// 3. 将模板拆分为静态片段并缓存快速模板
	placeholders := markerCtx.GetPlaceholders()
	templateStr := buf.String() // 需要复制，因为 buffer 要归还
	bufferPool.Put(buf)

	// 按占位符拆分模板为静态片段
	segments := splitByPlaceholders(templateStr, placeholders)

	fastTemplate := &CompiledFastTemplate{
		Segments:     segments,
		Placeholders: placeholders,
		TotalSize:    len(templateStr) + 50000, // 预留动态值空间
	}
	r.fastRenderer.Store(cacheKey, fastTemplate)

	// 4. 首次渲染：使用顺序写入方式返回结果
	resultBuf := bufferPool.Get().(*bytes.Buffer)
	resultBuf.Reset()
	resultBuf.Grow(len(templateStr) + 50000)

	for i, segment := range segments {
		resultBuf.WriteString(segment)
		if i < len(placeholders) {
			resultBuf.WriteString(r.getPlaceholderValue(placeholders[i]))
		}
	}

	result := resultBuf.String()
	bufferPool.Put(resultBuf)

	elapsed := time.Since(startTime)
	log.Debug().
		Str("template", templateName).
		Dur("duration", elapsed).
		Int("output_size", len(result)).
		Int("placeholders", len(placeholders)).
		Bool("fast_render", false).
		Msg("Template rendered (first time, compiled fast template)")

	return result, nil
}

// getPlaceholderValue 获取占位符的实际值
func (r *TemplateRenderer) getPlaceholderValue(p Placeholder) string {
	switch p.Type {
	case PlaceholderCls:
		return r.funcsManager.Cls(p.Arg)
	case PlaceholderURL:
		return r.funcsManager.RandomURL()
	case PlaceholderKeyword:
		return r.funcsManager.RandomKeyword()
	case PlaceholderImage:
		return r.funcsManager.RandomImage()
	case PlaceholderNumber:
		return formatInt(r.funcsManager.RandomNumber(p.MinMax[0], p.MinMax[1]))
	case PlaceholderNow:
		return NowFunc()
	case PlaceholderContent:
		return ""
	default:
		return ""
	}
}

// templateRenderContext is the context passed to templates
type templateRenderContext struct {
	Title          string
	SiteID         int
	AnalyticsCode  template.HTML
	BaiduPushJS    template.HTML
	ArticleContent template.HTML
	Now            string
	Content        string
	funcsManager   *TemplateFuncsManager
}

// Template function methods - these are called from templates

func (c *templateRenderContext) RandomKeyword() template.HTML {
	return template.HTML(c.funcsManager.RandomKeyword())
}

func (c *templateRenderContext) RandomURL() string {
	return c.funcsManager.RandomURL()
}

func (c *templateRenderContext) RandomImage() string {
	return c.funcsManager.RandomImage()
}

func (c *templateRenderContext) Cls(name string) string {
	return c.funcsManager.Cls(name)
}

func (c *templateRenderContext) Encode(text string) template.HTML {
	return template.HTML(c.funcsManager.Encode(text))
}

func (c *templateRenderContext) RandomNumber(min, max int) int {
	return c.funcsManager.RandomNumber(min, max)
}

// ClearCache clears the compiled template cache
func (r *TemplateRenderer) ClearCache() {
	r.compiledCache = sync.Map{}
}

// GetCacheStats returns cache statistics
func (r *TemplateRenderer) GetCacheStats() map[string]interface{} {
	var count int
	r.compiledCache.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	stats := map[string]interface{}{
		"compiled_templates": count,
	}

	// 添加快速渲染器统计
	if r.fastRenderer != nil {
		for k, v := range r.fastRenderer.GetStats() {
			stats[k] = v
		}
	}

	return stats
}

// splitByPlaceholders 将模板按占位符拆分为静态片段
// 返回 len(placeholders)+1 个片段，与占位符交替排列
// 例如: "A__PH_0__B__PH_1__C" -> ["A", "B", "C"]
func splitByPlaceholders(template string, placeholders []Placeholder) []string {
	if len(placeholders) == 0 {
		return []string{template}
	}

	segments := make([]string, 0, len(placeholders)+1)
	lastEnd := 0

	for _, p := range placeholders {
		idx := strings.Index(template[lastEnd:], p.Token)
		if idx >= 0 {
			segments = append(segments, template[lastEnd:lastEnd+idx])
			lastEnd = lastEnd + idx + len(p.Token)
		}
	}
	// 添加最后一个片段
	segments = append(segments, template[lastEnd:])

	return segments
}
