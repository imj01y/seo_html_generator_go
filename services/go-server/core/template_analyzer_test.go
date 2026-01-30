package core

import (
	"testing"
	"time"
)

// TestTemplateAnalyzer_AnalyzeContent 测试基本分析功能
func TestTemplateAnalyzer_AnalyzeContent(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	content := `
<!DOCTYPE html>
<html>
<head>
    <title>{{ random_title() }}</title>
</head>
<body>
    <div class="{{ cls('header') }}">
        <a href="{{ random_url() }}">{{ random_keyword() }}</a>
        <span>{{ keyword_with_emoji() }}</span>
    </div>
    <img src="{{ random_image() }}" />
    <p>{{ random_content() }}</p>
    <span>{{ now() }}</span>
    <div>{{ random_number(1, 100) }}</div>
</body>
</html>
`

	analysis := analyzer.AnalyzeTemplate("test", 1, content)

	if analysis == nil {
		t.Fatal("Expected analysis result, got nil")
	}

	stats := analysis.Stats

	// 验证各函数调用次数
	tests := []struct {
		name     string
		got      int
		expected int
	}{
		{"cls", stats.Cls, 1},
		{"random_url", stats.RandomURL, 1},
		{"keyword_with_emoji", stats.KeywordWithEmoji, 1},
		{"random_keyword", stats.RandomKeyword, 1},
		{"random_image", stats.RandomImage, 1},
		{"random_title", stats.RandomTitle, 1},
		{"random_content", stats.RandomContent, 1},
		{"now", stats.Now, 1},
		{"random_number", stats.RandomNumber, 1},
	}

	for _, tt := range tests {
		if tt.got != tt.expected {
			t.Errorf("%s: expected %d, got %d", tt.name, tt.expected, tt.got)
		}
	}

	// 验证总调用次数
	expectedTotal := 9
	if stats.Total() != expectedTotal {
		t.Errorf("Total: expected %d, got %d", expectedTotal, stats.Total())
	}
}

// TestTemplateAnalyzer_NestedLoops 测试嵌套循环展开
func TestTemplateAnalyzer_NestedLoops(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	// 测试单层循环
	content1 := `
{% for i in range(5) %}
    <a href="{{ random_url() }}">{{ random_keyword() }}</a>
{% endfor %}
`
	analysis1 := analyzer.AnalyzeTemplate("loop1", 1, content1)

	if analysis1.Stats.RandomURL != 5 {
		t.Errorf("Single loop random_url: expected 5, got %d", analysis1.Stats.RandomURL)
	}
	if analysis1.Stats.RandomKeyword != 5 {
		t.Errorf("Single loop random_keyword: expected 5, got %d", analysis1.Stats.RandomKeyword)
	}
	if analysis1.LoopCount != 1 {
		t.Errorf("Single loop count: expected 1, got %d", analysis1.LoopCount)
	}

	// 测试嵌套循环
	content2 := `
{% for i in range(3) %}
    {% for j in range(4) %}
        <a href="{{ random_url() }}">Link</a>
    {% endfor %}
{% endfor %}
`
	analysis2 := analyzer.AnalyzeTemplate("loop2", 1, content2)

	// 3 * 4 = 12
	if analysis2.Stats.RandomURL != 12 {
		t.Errorf("Nested loop random_url: expected 12, got %d", analysis2.Stats.RandomURL)
	}
	if analysis2.LoopCount != 2 {
		t.Errorf("Nested loop count: expected 2, got %d", analysis2.LoopCount)
	}
	if analysis2.MaxLoopDepth < 1 {
		t.Errorf("Nested loop max depth: expected >= 1, got %d", analysis2.MaxLoopDepth)
	}

	// 测试三层嵌套
	content3 := `
{% for i in range(2) %}
    {% for j in range(3) %}
        {% for k in range(2) %}
            {{ cls('item') }}
        {% endfor %}
    {% endfor %}
{% endfor %}
`
	analysis3 := analyzer.AnalyzeTemplate("loop3", 1, content3)

	// 2 * 3 * 2 = 12
	if analysis3.Stats.Cls != 12 {
		t.Errorf("Triple nested loop cls: expected 12, got %d", analysis3.Stats.Cls)
	}
}

// TestTemplateAnalyzer_PoolSizeCalculation 测试池大小计算
func TestTemplateAnalyzer_PoolSizeCalculation(t *testing.T) {
	analyzer := NewTemplateAnalyzer()
	analyzer.SetTargetQPS(100)
	analyzer.SetSafetyFactor(2.0)

	content := `
{% for i in range(10) %}
    <div class="{{ cls('item') }}">
        <a href="{{ random_url() }}">{{ keyword_with_emoji() }}</a>
    </div>
{% endfor %}
`
	analyzer.AnalyzeTemplate("test", 1, content)

	poolSize := analyzer.CalculatePoolSize()

	// cls: 10 * 100 * 2.0 = 2000
	if poolSize.ClsPoolSize != 2000 {
		t.Errorf("ClsPoolSize: expected 2000, got %d", poolSize.ClsPoolSize)
	}

	// url: 10 * 100 * 2.0 = 2000
	if poolSize.URLPoolSize != 2000 {
		t.Errorf("URLPoolSize: expected 2000, got %d", poolSize.URLPoolSize)
	}

	// keyword_emoji: 10 * 100 * 2.0 = 2000
	if poolSize.KeywordEmojiPoolSize != 2000 {
		t.Errorf("KeywordEmojiPoolSize: expected 2000, got %d", poolSize.KeywordEmojiPoolSize)
	}
}

// TestTemplateAnalyzer_MaxStats 测试最大值计算
func TestTemplateAnalyzer_MaxStats(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	// 模板1: cls=5, url=3
	content1 := `
{% for i in range(5) %}
    <div class="{{ cls('item') }}"></div>
{% endfor %}
{% for i in range(3) %}
    <a href="{{ random_url() }}"></a>
{% endfor %}
`
	analyzer.AnalyzeTemplate("tmpl1", 1, content1)

	// 模板2: cls=3, url=8
	content2 := `
{% for i in range(3) %}
    <div class="{{ cls('item') }}"></div>
{% endfor %}
{% for i in range(8) %}
    <a href="{{ random_url() }}"></a>
{% endfor %}
`
	analyzer.AnalyzeTemplate("tmpl2", 1, content2)

	maxStats := analyzer.GetMaxStats()

	// 最大值: cls=5, url=8
	if maxStats.Cls != 5 {
		t.Errorf("MaxStats.Cls: expected 5, got %d", maxStats.Cls)
	}
	if maxStats.RandomURL != 8 {
		t.Errorf("MaxStats.RandomURL: expected 8, got %d", maxStats.RandomURL)
	}
}

// TestTemplateAnalyzer_ContentHashChange 测试内容变化检测
func TestTemplateAnalyzer_ContentHashChange(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	content1 := `<div class="{{ cls('test') }}"></div>`
	analysis1 := analyzer.AnalyzeTemplate("test", 1, content1)
	hash1 := analysis1.ContentHash

	// 相同内容，应该返回缓存结果
	analysis2 := analyzer.AnalyzeTemplate("test", 1, content1)
	if analysis2.ContentHash != hash1 {
		t.Error("Same content should have same hash")
	}

	// 不同内容，应该重新分析
	content2 := `<div class="{{ cls('test') }}">{{ random_url() }}</div>`
	analysis3 := analyzer.AnalyzeTemplate("test", 1, content2)

	if analysis3.ContentHash == hash1 {
		t.Error("Different content should have different hash")
	}
	if analysis3.Stats.RandomURL != 1 {
		t.Errorf("Updated content random_url: expected 1, got %d", analysis3.Stats.RandomURL)
	}
}

// TestTemplateAnalyzer_ConfigCallback 测试配置变化回调
func TestTemplateAnalyzer_ConfigCallback(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	callbackCalled := make(chan *PoolSizeConfig, 1)
	analyzer.OnConfigChanged(func(config *PoolSizeConfig) {
		callbackCalled <- config
	})

	content := `{% for i in range(10) %}{{ cls('test') }}{% endfor %}`
	analyzer.AnalyzeTemplate("test", 1, content)

	// 等待回调
	select {
	case config := <-callbackCalled:
		if config == nil {
			t.Error("Expected config in callback, got nil")
		}
	case <-time.After(time.Second):
		t.Error("Callback not called within timeout")
	}
}

// TestSEOAnalysis 测试 SEO 分析
func TestSEOAnalysis(t *testing.T) {
	analyzer := NewTemplateAnalyzer()
	analyzer.SetTargetQPS(100)
	analyzer.SetSafetyFactor(1.0)

	content := `
{% for i in range(10) %}
    <div class="{{ cls('item') }}">
        <a href="{{ random_url() }}">{{ keyword_with_emoji() }}</a>
    </div>
{% endfor %}
`
	analyzer.AnalyzeTemplate("test", 1, content)

	seoAnalyzer := NewSEOAnalyzer(analyzer)

	// 测试池大小充足的情况
	currentPools := map[string]int{
		"cls":           2000,
		"url":           2000,
		"keyword_emoji": 2000,
		"number":        1000,
	}

	analysis := seoAnalyzer.AnalyzeSEOFriendliness(currentPools)

	if analysis.OverallRating != SEORatingExcellent {
		t.Errorf("Expected excellent rating with sufficient pools, got %s", analysis.OverallRating)
	}
	if analysis.Score < 90 {
		t.Errorf("Expected score >= 90 with sufficient pools, got %d", analysis.Score)
	}

	// 测试池大小不足的情况
	insufficientPools := map[string]int{
		"cls":           100, // 需要 1000，只有 100
		"url":           100,
		"keyword_emoji": 100,
		"number":        100,
	}

	analysis2 := seoAnalyzer.AnalyzeSEOFriendliness(insufficientPools)

	if analysis2.OverallRating == SEORatingExcellent {
		t.Error("Should not have excellent rating with insufficient pools")
	}
	if len(analysis2.Suggestions) == 0 {
		t.Error("Should have suggestions with insufficient pools")
	}
}

// TestTemplateFuncStats_Operations 测试统计结构体操作
func TestTemplateFuncStats_Operations(t *testing.T) {
	stats1 := &TemplateFuncStats{
		Cls:           5,
		RandomURL:     3,
		RandomKeyword: 10,
	}

	stats2 := &TemplateFuncStats{
		Cls:           3,
		RandomURL:     8,
		RandomKeyword: 5,
	}

	// 测试 Add
	combined := &TemplateFuncStats{}
	combined.Add(stats1)
	combined.Add(stats2)

	if combined.Cls != 8 {
		t.Errorf("Add Cls: expected 8, got %d", combined.Cls)
	}
	if combined.RandomURL != 11 {
		t.Errorf("Add RandomURL: expected 11, got %d", combined.RandomURL)
	}

	// 测试 Multiply
	multiplied := stats1.Multiply(3)
	if multiplied.Cls != 15 {
		t.Errorf("Multiply Cls: expected 15, got %d", multiplied.Cls)
	}

	// 测试 Max
	maxStats := &TemplateFuncStats{}
	maxStats.Max(stats1)
	maxStats.Max(stats2)

	if maxStats.Cls != 5 {
		t.Errorf("Max Cls: expected 5, got %d", maxStats.Cls)
	}
	if maxStats.RandomURL != 8 {
		t.Errorf("Max RandomURL: expected 8, got %d", maxStats.RandomURL)
	}
	if maxStats.RandomKeyword != 10 {
		t.Errorf("Max RandomKeyword: expected 10, got %d", maxStats.RandomKeyword)
	}
}

// TestTemplateAnalyzer_RemoveAndClear 测试移除和清除功能
func TestTemplateAnalyzer_RemoveAndClear(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	content := `{{ cls('test') }}`
	analyzer.AnalyzeTemplate("test1", 1, content)
	analyzer.AnalyzeTemplate("test2", 1, content)

	// 验证两个模板都存在
	if len(analyzer.GetAllAnalyses()) != 2 {
		t.Errorf("Expected 2 analyses, got %d", len(analyzer.GetAllAnalyses()))
	}

	// 移除一个
	analyzer.RemoveAnalysis("test1", 1)
	if len(analyzer.GetAllAnalyses()) != 1 {
		t.Errorf("Expected 1 analysis after remove, got %d", len(analyzer.GetAllAnalyses()))
	}

	// 清除全部
	analyzer.Clear()
	if len(analyzer.GetAllAnalyses()) != 0 {
		t.Errorf("Expected 0 analyses after clear, got %d", len(analyzer.GetAllAnalyses()))
	}
}

// TestTemplateAnalyzer_LoopLimit 测试循环次数限制
func TestTemplateAnalyzer_LoopLimit(t *testing.T) {
	analyzer := NewTemplateAnalyzer()

	// 测试超大循环次数会被限制
	content := `
{% for i in range(9999) %}
    {{ cls('item') }}
{% endfor %}
`
	analysis := analyzer.AnalyzeTemplate("big_loop", 1, content)

	// 循环次数应该被限制在 1000
	if analysis.Stats.Cls != 1000 {
		t.Errorf("Loop limit cls: expected 1000 (limited), got %d", analysis.Stats.Cls)
	}
}

// TestSEOAnalyzer_RecommendedPoolSize 测试推荐池大小计算
func TestSEOAnalyzer_RecommendedPoolSize(t *testing.T) {
	analyzer := NewTemplateAnalyzer()
	analyzer.SetTargetQPS(500)
	analyzer.SetSafetyFactor(1.5)

	seoAnalyzer := NewSEOAnalyzer(analyzer)

	// 每个请求调用 10 次 -> 10 * 500 * 1.5 = 7500
	recommended := seoAnalyzer.GetRecommendedPoolSize(10)
	expected := 7500

	if recommended != expected {
		t.Errorf("Recommended pool size: expected %d, got %d", expected, recommended)
	}
}
