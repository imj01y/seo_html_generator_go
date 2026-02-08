package core

import (
	"bytes"
	"html/template"
	"sync"
	"sync/atomic"
)

// 全局对象池 - 复用 bytes.Buffer
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// PlaceholderType 占位符类型
type PlaceholderType int

const (
	PlaceholderCls PlaceholderType = iota
	PlaceholderURL
	PlaceholderKeyword
	PlaceholderKeywordEmoji // 带 emoji 的关键词
	PlaceholderImage
	PlaceholderNumber
	PlaceholderNow
	PlaceholderContent
	PlaceholderTitle          // Title 动态占位符
	PlaceholderArticleContent // ArticleContent 动态占位符
)

// Placeholder 占位符信息
type Placeholder struct {
	Token  string          // __PH_CLS_0__ 等
	Type   PlaceholderType // 类型
	Arg    string          // 参数，如 cls("header") 中的 "header"
	MinMax [2]int          // 用于 random_number
}

// CompiledFastTemplate 预编译的快速模板
// 使用分段存储，避免 strings.NewReplacer 的开销
type CompiledFastTemplate struct {
	Segments     []string      // 静态片段列表（与 Placeholders 交替）
	Placeholders []Placeholder // 占位符列表（按顺序）
	TotalSize    int           // 预估输出大小，用于 buffer 预分配
}

// FastRenderer 快速字符串替换渲染器
type FastRenderer struct {
	templates    sync.Map // cacheKey -> *CompiledFastTemplate
	funcsManager *TemplateFuncsManager
}

// NewFastRenderer 创建快速渲染器
func NewFastRenderer(fm *TemplateFuncsManager) *FastRenderer {
	return &FastRenderer{
		funcsManager: fm,
	}
}

// Store 存储编译后的快速模板
func (r *FastRenderer) Store(cacheKey string, ct *CompiledFastTemplate) {
	r.templates.Store(cacheKey, ct)
}

// Render 快速渲染 - 使用 bytes.Buffer 顺序写入
func (r *FastRenderer) Render(cacheKey string, data *RenderData) (string, bool) {
	cached, ok := r.templates.Load(cacheKey)
	if !ok {
		return "", false
	}

	ct := cached.(*CompiledFastTemplate)

	// 请求级缓存：NowFunc 只计算一次，避免 ~1200 次重复调用
	if data != nil && data.Now == "" {
		data.Now = NowFunc()
	}

	// 从对象池获取 buffer
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Grow(ct.TotalSize) // 预分配

	// 顺序写入：Segments[0] + getValue(PH[0]) + Segments[1] + getValue(PH[1]) + ...
	for i, segment := range ct.Segments {
		buf.WriteString(segment)
		if i < len(ct.Placeholders) {
			buf.WriteString(r.getValue(ct.Placeholders[i], data))
		}
	}

	result := buf.String()
	bufferPool.Put(buf)

	return result, true
}

// getValue 获取占位符对应的实际值
func (r *FastRenderer) getValue(p Placeholder, data *RenderData) string {
	return resolvePlaceholder(p, data, r.funcsManager)
}

// resolvePlaceholder 解析占位符获取实际值（公共函数，供多处复用）
func resolvePlaceholder(p Placeholder, data *RenderData, fm *TemplateFuncsManager) string {
	switch p.Type {
	case PlaceholderCls:
		return fm.Cls(p.Arg)
	case PlaceholderURL:
		return fm.RandomURL()
	case PlaceholderKeyword:
		if data != nil {
			return fm.RandomKeyword(data.KeywordGroupID)
		}
		return fm.RandomKeyword(1)
	case PlaceholderKeywordEmoji:
		if data != nil {
			return fm.RandomKeywordEmoji(data.KeywordGroupID)
		}
		return fm.RandomKeywordEmoji(1)
	case PlaceholderImage:
		if data != nil {
			return fm.RandomImage(data.ImageGroupID)
		}
		return fm.RandomImage(1) // 默认分组
	case PlaceholderNumber:
		return formatInt(fm.RandomNumber(p.MinMax[0], p.MinMax[1]))
	case PlaceholderNow:
		if data != nil && data.Now != "" {
			return data.Now
		}
		return NowFunc()
	case PlaceholderContent:
		if data != nil && data.Content != "" {
			return data.Content
		}
		return ""
	case PlaceholderTitle:
		if data != nil && data.TitleGenerator != nil {
			return data.TitleGenerator()
		}
		if data != nil {
			return data.Title
		}
		return ""
	case PlaceholderArticleContent:
		if data != nil {
			return string(data.ArticleContent)
		}
		return ""
	default:
		return ""
	}
}

// formatInt 快速整数转字符串
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	// 最大支持 10 位数字
	var buf [11]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

// GetStats 返回统计信息
func (r *FastRenderer) GetStats() map[string]interface{} {
	var count int
	r.templates.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return map[string]interface{}{
		"fast_templates": count,
	}
}

// ClearCache 清除快速模板缓存
func (r *FastRenderer) ClearCache() {
	r.templates = sync.Map{}
}

// ============================================================
// MarkerContext - 用于首次渲染，生成占位符模板
// ============================================================

// MarkerContext 标记上下文，用于生成占位符模板
type MarkerContext struct {
	// 静态字段（可以被模板直接访问）
	SiteID        int
	AnalyticsCode template.HTML
	BaiduPushJS   template.HTML

	// 私有字段（不能被模板直接访问，需要通过方法获取占位符）
	articleContent template.HTML // 重命名为小写
	now            string        // 重命名为小写
	content        string        // 重命名为小写

	// 占位符计数器
	clsCounter            int64
	urlCounter            int64
	keywordCounter        int64
	keywordEmojiCounter   int64 // 带 emoji 的关键词计数器
	imageCounter          int64
	numberCounter         int64
	nowCounter            int64
	titleCounter          int64 // Title 占位符计数器
	contentCounter        int64 // Content 占位符计数器
	articleContentCounter int64 // ArticleContent 占位符计数器

	// 收集的占位符
	placeholders []Placeholder
	mu           sync.Mutex
}

// NewMarkerContext 创建标记上下文
func NewMarkerContext(data *RenderData, content string) *MarkerContext {
	return &MarkerContext{
		SiteID:         data.SiteID,
		AnalyticsCode:  data.AnalyticsCode,
		BaiduPushJS:    data.BaiduPushJS,
		articleContent: data.ArticleContent, // 私有字段
		now:            NowFunc(),           // 私有字段
		content:        content,             // 私有字段
		placeholders:   make([]Placeholder, 0, 1000),
	}
}

// GetPlaceholders 获取收集的占位符列表
func (c *MarkerContext) GetPlaceholders() []Placeholder {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.placeholders
}

// addPlaceholder 添加占位符（线程安全）
func (c *MarkerContext) addPlaceholder(p Placeholder) {
	c.mu.Lock()
	c.placeholders = append(c.placeholders, p)
	c.mu.Unlock()
}

// RandomKeyword 返回占位符标记
func (c *MarkerContext) RandomKeyword() template.HTML {
	idx := atomic.AddInt64(&c.keywordCounter, 1) - 1
	token := "__PH_KW_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderKeyword,
	})
	return template.HTML(token)
}

// RandomKeywordEmoji 返回带 emoji 的关键词占位符标记
func (c *MarkerContext) RandomKeywordEmoji() template.HTML {
	idx := atomic.AddInt64(&c.keywordEmojiCounter, 1) - 1
	token := "__PH_KWE_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderKeywordEmoji,
	})
	return template.HTML(token)
}

// RandomURL 返回占位符标记
func (c *MarkerContext) RandomURL() string {
	idx := atomic.AddInt64(&c.urlCounter, 1) - 1
	token := "__PH_URL_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderURL,
	})
	return token
}

// RandomImage 返回占位符标记
func (c *MarkerContext) RandomImage() string {
	idx := atomic.AddInt64(&c.imageCounter, 1) - 1
	token := "__PH_IMG_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderImage,
	})
	return token
}

// Cls 返回占位符标记
func (c *MarkerContext) Cls(name string) string {
	idx := atomic.AddInt64(&c.clsCounter, 1) - 1
	token := "__PH_CLS_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderCls,
		Arg:   name,
	})
	return token
}

// Encode 编码文本（直接返回，不需要动态替换）
func (c *MarkerContext) Encode(text string) template.HTML {
	// Encode 是静态的，直接使用全局 encoder
	return template.HTML(GetEncoder().EncodeText(text))
}

// RandomNumber 返回占位符标记
func (c *MarkerContext) RandomNumber(min, max int) string {
	idx := atomic.AddInt64(&c.numberCounter, 1) - 1
	token := "__PH_NUM_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token:  token,
		Type:   PlaceholderNumber,
		MinMax: [2]int{min, max},
	})
	return token
}

// Title 返回 Title 占位符标记（动态生成）
func (c *MarkerContext) Title() string {
	idx := atomic.AddInt64(&c.titleCounter, 1) - 1
	token := "__PH_TITLE_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderTitle,
	})
	return token
}

// Content 返回 Content 占位符标记（动态生成）
func (c *MarkerContext) Content() string {
	idx := atomic.AddInt64(&c.contentCounter, 1) - 1
	token := "__PH_CONTENT_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderContent,
	})
	return token
}

// Now 返回 Now 占位符标记（动态生成）
func (c *MarkerContext) Now() string {
	idx := atomic.AddInt64(&c.nowCounter, 1) - 1
	token := "__PH_NOW_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderNow,
	})
	return token
}

// ArticleContent 返回 ArticleContent 占位符标记（动态生成）
func (c *MarkerContext) ArticleContent() template.HTML {
	idx := atomic.AddInt64(&c.articleContentCounter, 1) - 1
	token := "__PH_ARTICLE_" + formatInt(int(idx)) + "__"
	c.addPlaceholder(Placeholder{
		Token: token,
		Type:  PlaceholderArticleContent,
	})
	return template.HTML(token)
}
