package core

import (
	"bytes"
	"strings"
	"testing"
)

// 模拟当前方案：strings.NewReplacer
func BenchmarkCurrentReplacer(b *testing.B) {
	// 准备测试数据：模拟 26820 个占位符
	numPlaceholders := 26820

	// 构建模板：静态内容 + 占位符交替
	var templateBuilder strings.Builder
	placeholders := make([]string, numPlaceholders)
	values := make([]string, numPlaceholders)

	for i := 0; i < numPlaceholders; i++ {
		templateBuilder.WriteString("<div class=\"item\">")
		placeholder := "__PH_" + formatInt(i) + "__"
		templateBuilder.WriteString(placeholder)
		templateBuilder.WriteString("</div>\n")
		placeholders[i] = placeholder
		values[i] = "https://example.com/page/" + formatInt(i) + ".html"
	}
	template := templateBuilder.String()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 当前方案：每次构建 replacements 数组和 NewReplacer
		replacements := make([]string, 0, numPlaceholders*2)
		for j := 0; j < numPlaceholders; j++ {
			replacements = append(replacements, placeholders[j], values[j])
		}
		replacer := strings.NewReplacer(replacements...)
		_ = replacer.Replace(template)
	}
}

// 新方案：bytes.Buffer 顺序写入
func BenchmarkBufferWrite(b *testing.B) {
	numPlaceholders := 26820

	// 预编译：将模板拆分为静态片段
	segments := make([]string, numPlaceholders+1)
	values := make([]string, numPlaceholders)

	for i := 0; i < numPlaceholders; i++ {
		segments[i] = "<div class=\"item\">"
		values[i] = "https://example.com/page/" + formatInt(i) + ".html"
	}
	segments[numPlaceholders] = "</div>\n"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 新方案：顺序写入
		var buf bytes.Buffer
		buf.Grow(2500000) // 预分配 2.5MB
		for j := 0; j < numPlaceholders; j++ {
			buf.WriteString(segments[j])
			buf.WriteString(values[j])
		}
		buf.WriteString(segments[numPlaceholders])
		_ = buf.String()
	}
}

// 新方案变体：使用 sync.Pool 复用 buffer
var bufPool = &pooledBuffer{}

type pooledBuffer struct {
	buf bytes.Buffer
}

func BenchmarkBufferWritePooled(b *testing.B) {
	numPlaceholders := 26820

	// 预编译：将模板拆分为静态片段
	segments := make([]string, numPlaceholders+1)
	values := make([]string, numPlaceholders)

	for i := 0; i < numPlaceholders; i++ {
		segments[i] = "<div class=\"item\">"
		values[i] = "https://example.com/page/" + formatInt(i) + ".html"
	}
	segments[numPlaceholders] = "</div>\n"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.Grow(2500000)
		for j := 0; j < numPlaceholders; j++ {
			buf.WriteString(segments[j])
			buf.WriteString(values[j])
		}
		buf.WriteString(segments[numPlaceholders])
		_ = buf.String()
		bufferPool.Put(buf)
	}
}

// 模拟真实场景：包含 getValue() 调用开销
func BenchmarkCurrentReplacerWithGetValue(b *testing.B) {
	numPlaceholders := 26820

	var templateBuilder strings.Builder
	placeholders := make([]string, numPlaceholders)

	for i := 0; i < numPlaceholders; i++ {
		templateBuilder.WriteString("<div class=\"item\">")
		placeholder := "__PH_" + formatInt(i) + "__"
		templateBuilder.WriteString(placeholder)
		templateBuilder.WriteString("</div>\n")
		placeholders[i] = placeholder
	}
	template := templateBuilder.String()

	// 模拟 getValue 的数据源
	urlPool := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		urlPool[i] = "https://example.com/page/" + formatInt(i) + ".html"
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		replacements := make([]string, 0, numPlaceholders*2)
		for j := 0; j < numPlaceholders; j++ {
			replacements = append(replacements, placeholders[j])
			// 模拟 getValue() - 从池中随机选取
			replacements = append(replacements, urlPool[j%1000])
		}
		replacer := strings.NewReplacer(replacements...)
		_ = replacer.Replace(template)
	}
}

func BenchmarkBufferWriteWithGetValue(b *testing.B) {
	numPlaceholders := 26820

	segments := make([]string, numPlaceholders+1)
	for i := 0; i < numPlaceholders; i++ {
		segments[i] = "<div class=\"item\">"
	}
	segments[numPlaceholders] = "</div>\n"

	// 模拟 getValue 的数据源
	urlPool := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		urlPool[i] = "https://example.com/page/" + formatInt(i) + ".html"
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()
		buf.Grow(2500000)
		for j := 0; j < numPlaceholders; j++ {
			buf.WriteString(segments[j])
			// 模拟 getValue() - 从池中随机选取
			buf.WriteString(urlPool[j%1000])
		}
		buf.WriteString(segments[numPlaceholders])
		_ = buf.String()
		bufferPool.Put(buf)
	}
}
