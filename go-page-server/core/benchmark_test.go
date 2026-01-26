package core

import (
	"html/template"
	"math/rand"
	"testing"
	"time"
)

// 用于测试的模拟函数管理器
type mockFuncsManager struct {
	keywords []string
	images   []string
}

func newMockFuncsManager() *mockFuncsManager {
	// 生成测试关键词
	keywords := make([]string, 50000)
	for i := range keywords {
		keywords[i] = "测试关键词" + formatInt(i)
	}

	// 生成测试图片URL
	images := make([]string, 1000)
	for i := range images {
		images[i] = "/static/images/img" + formatInt(i) + ".png"
	}

	return &mockFuncsManager{
		keywords: keywords,
		images:   images,
	}
}

func (m *mockFuncsManager) Cls(name string) string {
	return generateRandomCls() + " " + name
}

func (m *mockFuncsManager) RandomURL() string {
	return generateRandomURL()
}

func (m *mockFuncsManager) RandomKeyword() string {
	return m.keywords[rand.Intn(len(m.keywords))]
}

func (m *mockFuncsManager) RandomImage() string {
	return m.images[rand.Intn(len(m.images))]
}

func (m *mockFuncsManager) RandomNumber(min, max int) int {
	if min >= max {
		return min
	}
	return rand.Intn(max-min+1) + min
}

// 创建测试数据
func createTestRenderData() *RenderData {
	return &RenderData{
		Title:          "测试页面标题 - 热门应用下载",
		SiteID:         12345,
		AnalyticsCode:  template.HTML("<script>console.log('analytics');</script>"),
		BaiduPushJS:    template.HTML("<script>console.log('baidu push');</script>"),
		ArticleContent: template.HTML("<p>这是测试文章内容，包含一些中文文字和特殊字符。</p>"),
	}
}

// ===========================================
// Benchmark: QuickTemplate 渲染
// ===========================================

func BenchmarkQuickTemplateRender(b *testing.B) {
	// 设置随机种子
	rand.Seed(time.Now().UnixNano())

	// 创建真实的 TemplateFuncsManager
	encoder := NewHTMLEntityEncoder(0.5)
	fm := NewTemplateFuncsManager(encoder)

	// 加载测试关键词
	keywords := make([]string, 50000)
	for i := range keywords {
		keywords[i] = "测试关键词" + formatInt(i)
	}
	fm.LoadKeywords(keywords)

	// 加载测试图片
	images := make([]string, 1000)
	for i := range images {
		images[i] = "/static/images/img" + formatInt(i) + ".png"
	}
	fm.LoadImageURLs(images)

	// 初始化对象池
	fm.InitPools()
	defer fm.StopPools()

	// 等待池子预热
	time.Sleep(100 * time.Millisecond)

	// 创建渲染器
	renderer := NewQuickTemplateRenderer(fm)
	data := createTestRenderData()
	content := "这是测试内容，用于 content() 函数。"

	// 预热
	for i := 0; i < 10; i++ {
		_ = renderer.Render(data, content)
	}

	b.ResetTimer()
	b.ReportAllocs()

	var totalSize int
	for i := 0; i < b.N; i++ {
		result := renderer.Render(data, content)
		totalSize += len(result)
	}

	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}

// ===========================================
// Benchmark: FastRenderer (strings.Replacer) - 需要先编译模板
// ===========================================

// 简化的测试模板（用于 FastRenderer 测试）
const testTemplateContent = `<!DOCTYPE html>
<html>
<head><title>{{ title }}</title></head>
<body>
<div class="{{ cls('header') }}">
{{ range iterate 100 }}
<a href="{{ random_url }}">{{ random_keyword }}</a>
{{ end }}
</div>
<div class="{{ cls('content') }}">
{{ range iterate 50 }}
<img src="{{ random_image }}" alt="{{ random_keyword }}" />
<p>{{ random_keyword }}</p>
{{ end }}
</div>
<p>Site: {{ site_id }}</p>
<p>Time: {{ now }}</p>
{{ range iterate 200 }}
<li>{{ random_keyword }} - {{ random_number 1 100 }}</li>
{{ end }}
{{ analytics_code }}
</body>
</html>`

// ===========================================
// Benchmark: 纯 QuickTemplate（不含池子初始化）
// ===========================================

func BenchmarkQuickTemplateRenderOnly(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	mock := newMockFuncsManager()

	// 创建一个使用 mock 函数的 TemplateFuncsManager
	encoder := NewHTMLEntityEncoder(0.5)
	fm := NewTemplateFuncsManager(encoder)
	fm.LoadKeywords(mock.keywords)
	fm.LoadImageURLs(mock.images)

	renderer := NewQuickTemplateRenderer(fm)
	data := createTestRenderData()
	content := "这是测试内容。"

	// 预热
	for i := 0; i < 10; i++ {
		_ = renderer.Render(data, content)
	}

	b.ResetTimer()
	b.ReportAllocs()

	var totalSize int
	for i := 0; i < b.N; i++ {
		result := renderer.Render(data, content)
		totalSize += len(result)
	}

	b.ReportMetric(float64(totalSize)/float64(b.N), "bytes/op")
}

// ===========================================
// Benchmark: 并行渲染测试
// ===========================================

func BenchmarkQuickTemplateParallel(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	encoder := NewHTMLEntityEncoder(0.5)
	fm := NewTemplateFuncsManager(encoder)

	keywords := make([]string, 50000)
	for i := range keywords {
		keywords[i] = "并行测试关键词" + formatInt(i)
	}
	fm.LoadKeywords(keywords)

	images := make([]string, 1000)
	for i := range images {
		images[i] = "/static/images/parallel" + formatInt(i) + ".png"
	}
	fm.LoadImageURLs(images)

	fm.InitPools()
	defer fm.StopPools()

	time.Sleep(100 * time.Millisecond)

	renderer := NewQuickTemplateRenderer(fm)
	data := createTestRenderData()
	content := "并行测试内容。"

	// 预热
	for i := 0; i < 10; i++ {
		_ = renderer.Render(data, content)
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = renderer.Render(data, content)
		}
	})
}

// ===========================================
// 对比测试：测量单次渲染时间
// ===========================================

func TestRenderPerformance(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	encoder := NewHTMLEntityEncoder(0.5)
	fm := NewTemplateFuncsManager(encoder)

	keywords := make([]string, 50000)
	for i := range keywords {
		keywords[i] = "性能测试关键词" + formatInt(i)
	}
	fm.LoadKeywords(keywords)

	images := make([]string, 1000)
	for i := range images {
		images[i] = "/static/images/perf" + formatInt(i) + ".png"
	}
	fm.LoadImageURLs(images)

	fm.InitPools()
	defer fm.StopPools()

	time.Sleep(200 * time.Millisecond) // 等待池子充满

	renderer := NewQuickTemplateRenderer(fm)
	data := createTestRenderData()
	content := "性能测试内容。"

	// 预热 5 次
	for i := 0; i < 5; i++ {
		_ = renderer.Render(data, content)
	}

	// 测试 100 次
	const iterations = 100
	var totalDuration time.Duration
	var minDuration = time.Hour
	var maxDuration time.Duration
	var totalSize int

	for i := 0; i < iterations; i++ {
		start := time.Now()
		result := renderer.Render(data, content)
		duration := time.Since(start)

		totalDuration += duration
		totalSize += len(result)

		if duration < minDuration {
			minDuration = duration
		}
		if duration > maxDuration {
			maxDuration = duration
		}
	}

	avgDuration := totalDuration / iterations
	avgSize := totalSize / iterations

	t.Logf("QuickTemplate 渲染性能测试结果:")
	t.Logf("  测试次数: %d", iterations)
	t.Logf("  平均耗时: %v", avgDuration)
	t.Logf("  最小耗时: %v", minDuration)
	t.Logf("  最大耗时: %v", maxDuration)
	t.Logf("  平均输出大小: %d bytes (%.2f KB)", avgSize, float64(avgSize)/1024)
	t.Logf("  预估 QPS (单线程): %.0f", float64(time.Second)/float64(avgDuration))
}

// ===========================================
// 测试输出内容正确性
// ===========================================

func TestRenderOutput(t *testing.T) {
	rand.Seed(42) // 固定种子以便复现

	encoder := NewHTMLEntityEncoder(0.5)
	fm := NewTemplateFuncsManager(encoder)

	keywords := []string{"关键词1", "关键词2", "关键词3"}
	fm.LoadKeywords(keywords)

	images := []string{"/img1.png", "/img2.png"}
	fm.LoadImageURLs(images)

	renderer := NewQuickTemplateRenderer(fm)
	data := &RenderData{
		Title:          "测试标题",
		SiteID:         123,
		AnalyticsCode:  template.HTML("<script>test</script>"),
		BaiduPushJS:    template.HTML(""),
		ArticleContent: template.HTML("<p>文章内容</p>"),
	}
	content := "正文内容"

	result := renderer.Render(data, content)

	// 验证基本内容
	if len(result) == 0 {
		t.Error("渲染结果为空")
	}

	// 验证标题
	if !contains(result, "测试标题") {
		t.Error("渲染结果不包含标题")
	}

	// 验证 SiteID
	if !contains(result, "123") {
		t.Error("渲染结果不包含 SiteID")
	}

	// 验证 HTML 结构
	if !contains(result, "<!DOCTYPE html>") {
		t.Error("渲染结果不包含 DOCTYPE")
	}

	if !contains(result, "</html>") {
		t.Error("渲染结果不包含闭合的 html 标签")
	}

	t.Logf("渲染输出大小: %d bytes", len(result))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
