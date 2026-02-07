package core

import (
	"testing"
)

// TestRandomKeyword_IsRandom 验证 RandomKeyword 不是顺序轮询
func TestRandomKeyword_IsRandom(t *testing.T) {
	m := NewTemplateFuncsManager(NewHTMLEntityEncoder(0.5))

	keywords := make([]string, 100)
	raw := make([]string, 100)
	for i := range keywords {
		kw := "kw" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		keywords[i] = kw
		raw[i] = kw
	}
	m.LoadKeywordGroup(1, keywords, raw)

	results := make([]string, 100)
	for i := range results {
		results[i] = m.RandomKeyword(1)
	}

	sequential := true
	for i, r := range results {
		if r != keywords[i] {
			sequential = false
			break
		}
	}
	if sequential {
		t.Error("RandomKeyword appears to be sequential, not random")
	}
}

// TestRandomKeyword_Distribution 验证随机分布基本均匀
func TestRandomKeyword_Distribution(t *testing.T) {
	m := NewTemplateFuncsManager(NewHTMLEntityEncoder(0.5))

	keywords := []string{"a", "b", "c", "d", "e"}
	m.LoadKeywordGroup(1, keywords, keywords)

	counts := make(map[string]int)
	n := 10000
	for i := 0; i < n; i++ {
		counts[m.RandomKeyword(1)]++
	}

	for _, kw := range keywords {
		c := counts[kw]
		expected := n / len(keywords)
		if c < expected*70/100 || c > expected*130/100 {
			t.Errorf("keyword %q appeared %d times, expected ~%d (±30%%)", kw, c, expected)
		}
	}
}

// TestRandomKeyword_EmptyGroup 验证空分组返回空字符串
func TestRandomKeyword_EmptyGroup(t *testing.T) {
	m := NewTemplateFuncsManager(NewHTMLEntityEncoder(0.5))
	result := m.RandomKeyword(999)
	if result != "" {
		t.Errorf("expected empty string for non-existent group, got %q", result)
	}
}

// TestRandomImage_IsRandom 验证 RandomImage 不是顺序轮询
func TestRandomImage_IsRandom(t *testing.T) {
	m := NewTemplateFuncsManager(NewHTMLEntityEncoder(0.5))

	urls := make([]string, 100)
	for i := range urls {
		urls[i] = "https://img.example.com/" + string(rune('a'+i%26)) + string(rune('0'+i/26)) + ".jpg"
	}
	m.LoadImageGroup(1, urls)

	results := make([]string, 100)
	for i := range results {
		results[i] = m.RandomImage(1)
	}

	sequential := true
	for i, r := range results {
		if r != urls[i] {
			sequential = false
			break
		}
	}
	if sequential {
		t.Error("RandomImage appears to be sequential, not random")
	}
}
