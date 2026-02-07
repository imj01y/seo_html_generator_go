# 关键词随机化选取 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 `RandomKeyword()`、`RandomImage()`、`RandomKeywordEmoji()` 降级路径从顺序轮询改为 `rand.IntN` 真正随机选取，并清理不再需要的 `sync.Map` 索引字段。

**Architecture:** 将 3 个函数中的 `atomic.Int64` 计数器替换为 `rand.IntN(len(items))`，删除 `TemplateFuncsManager` 结构体中不再使用的 `keywordGroupIdx`、`rawKeywordGroupIdx`、`imageGroupIdx` 三个 `sync.Map` 字段。改动局限于 `template_funcs.go` 单文件。

**Tech Stack:** Go 1.22+（`math/rand/v2` 包，`rand.IntN` 并发安全）

---

### Task 1: 编写 RandomKeyword 随机性测试

**Files:**
- Create: `api/internal/service/template_funcs_test.go`

**Step 1: 编写测试文件**

```go
package core

import (
	"math/rand/v2"
	"testing"
)

// TestRandomKeyword_IsRandom 验证 RandomKeyword 不是顺序轮询
// 策略：调用 100 次，检查返回结果不是严格的数组顺序
func TestRandomKeyword_IsRandom(t *testing.T) {
	m := NewTemplateFuncsManager(NewHTMLEntityEncoder(0.5))

	// 加载 100 个关键词
	keywords := make([]string, 100)
	raw := make([]string, 100)
	for i := range keywords {
		kw := "kw" + string(rune('A'+i%26)) + string(rune('0'+i/26))
		keywords[i] = kw
		raw[i] = kw
	}
	m.LoadKeywordGroup(1, keywords, raw)

	// 调用 100 次
	results := make([]string, 100)
	for i := range results {
		results[i] = m.RandomKeyword(1)
	}

	// 如果是顺序轮询，results[i] == keywords[i] 对所有 i 成立
	// 随机选取几乎不可能完全匹配（概率 = (1/100)^100）
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

	// 每个关键词期望出现 2000 次，允许 ±30% 偏差
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
```

**Step 2: 运行测试验证失败**

Run: `cd api && go test ./internal/service/ -run TestRandomKeyword -v`

Expected: `TestRandomKeyword_IsRandom` 应该 PASS（当前顺序轮询会被检测到），但实际上当前实现就是顺序的，所以 `TestRandomKeyword_IsRandom` 会 **FAIL**（`sequential == true`）。

> 注意：当前实现是顺序轮询，所以 `sequential` 会为 `true`，测试会报错 "RandomKeyword appears to be sequential"。这正是我们期望的失败。

---

### Task 2: 修改 RandomKeyword 为随机选取

**Files:**
- Modify: `api/internal/service/template_funcs.go:181-201`

**Step 1: 修改 RandomKeyword 函数**

将 `template_funcs.go` 中的 `RandomKeyword` 函数体替换为：

```go
// RandomKeyword 获取随机关键词（支持分组）
func (m *TemplateFuncsManager) RandomKeyword(groupID int) string {
	data := m.keywordData.Load()
	if data == nil {
		return ""
	}

	keywords := data.groups[groupID]
	if len(keywords) == 0 {
		// 降级到默认分组
		keywords = data.groups[1]
		if len(keywords) == 0 {
			return ""
		}
	}

	return keywords[rand.IntN(len(keywords))]
}
```

**Step 2: 运行测试验证通过**

Run: `cd api && go test ./internal/service/ -run TestRandomKeyword -v`

Expected: 全部 PASS

---

### Task 3: 编写 RandomImage 随机性测试

**Files:**
- Modify: `api/internal/service/template_funcs_test.go`

**Step 1: 追加测试**

```go
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
```

**Step 2: 运行测试验证失败**

Run: `cd api && go test ./internal/service/ -run TestRandomImage_IsRandom -v`

Expected: FAIL（当前实现是顺序轮询）

---

### Task 4: 修改 RandomImage 为随机选取

**Files:**
- Modify: `api/internal/service/template_funcs.go:226-247`

**Step 1: 修改 RandomImage 函数**

```go
// RandomImage 获取随机图片URL（支持分组）
func (m *TemplateFuncsManager) RandomImage(groupID int) string {
	data := m.imageData.Load()
	if data == nil {
		return ""
	}

	urls := data.groups[groupID]
	if len(urls) == 0 {
		// 降级到默认分组
		urls = data.groups[1]
		if len(urls) == 0 {
			return ""
		}
	}

	return urls[rand.IntN(len(urls))]
}
```

**Step 2: 运行测试验证通过**

Run: `cd api && go test ./internal/service/ -run TestRandomImage -v`

Expected: PASS

---

### Task 5: 修改 RandomKeywordEmoji 降级路径

**Files:**
- Modify: `api/internal/service/template_funcs.go:203-224`

**Step 1: 修改 RandomKeywordEmoji 降级路径**

```go
// RandomKeywordEmoji 获取带 emoji 的随机关键词（支持分组，从对象池消费）
func (m *TemplateFuncsManager) RandomKeywordEmoji(groupID int) string {
	if m.keywordEmojiGenerator != nil {
		return m.keywordEmojiGenerator.Pop(groupID)
	}
	// 降级：生成器未初始化时实时生成
	data := m.keywordData.Load()
	if data == nil {
		return ""
	}
	rawKeywords := data.rawGroups[groupID]
	if len(rawKeywords) == 0 {
		rawKeywords = data.rawGroups[1]
		if len(rawKeywords) == 0 {
			return ""
		}
	}
	keyword := rawKeywords[rand.IntN(len(rawKeywords))]
	return m.generateKeywordWithEmojiFromRaw(keyword)
}
```

**Step 2: 运行全部测试验证通过**

Run: `cd api && go test ./internal/service/ -run "TestRandom" -v`

Expected: 全部 PASS

---

### Task 6: 清理不再使用的 sync.Map 字段

**Files:**
- Modify: `api/internal/service/template_funcs.go:25-48`

**Step 1: 从结构体中删除 3 个 sync.Map 字段**

删除 `TemplateFuncsManager` 中以下 6 行（字段 + 注释）：

```go
	// 分组索引（独立管理，避免数据替换时重置）
	keywordGroupIdx    sync.Map // groupID -> *atomic.Int64
	rawKeywordGroupIdx sync.Map // groupID -> *atomic.Int64

	// ...

	// 分组索引（独立管理，避免数据替换时重置）
	imageGroupIdx sync.Map // groupID -> *atomic.Int64
```

**Step 2: 清理不再需要的 import**

检查 `sync` 和 `sync/atomic` 是否仍被使用。`sync` 不再需要（`sync.Map` 已删除），`atomic` 仍被 `atomic.Pointer` 使用，保留。删除 `"sync"` import。

**Step 3: 编译验证**

Run: `cd api && go build ./...`

Expected: 编译成功，无错误

**Step 4: 运行全部测试**

Run: `cd api && go test ./internal/service/ -v`

Expected: 全部 PASS

---

### Task 7: 提交

**Step 1: 提交所有变更**

```bash
git add api/internal/service/template_funcs.go api/internal/service/template_funcs_test.go
git commit -m "fix: replace sequential round-robin with rand.IntN for keyword/image selection

RandomKeyword, RandomImage, RandomKeywordEmoji fallback were using atomic
counter round-robin which produced predictable sequential patterns.
Replace with rand.IntN for true random selection. Remove unused sync.Map
index fields.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```
