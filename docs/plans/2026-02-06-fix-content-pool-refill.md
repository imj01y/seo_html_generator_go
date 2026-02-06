# 修复正文缓存池加载即标记 bug

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 refillPool 加载数据时错误标记 status=0 导致服务重启后正文数据丢失的 bug。

**Architecture:** 在 MemoryPool 中增加 loadedIDs 集合防止重复加载，移除 refillPool 中加载时标记 status=0 的逻辑，仅在 Pop 消费时标记（现有逻辑已支持）。

**Tech Stack:** Go, MySQL

---

### Task 1: MemoryPool 增加 ID 去重 — 测试

**Files:**
- Create: `api/internal/service/memory_pool_test.go`

**Step 1: 编写 Push 去重测试**

```go
package core

import "testing"

func TestPush_SkipsDuplicateIDs(t *testing.T) {
	pool := NewMemoryPool(1, "contents", 100)

	items := []PoolItem{
		{ID: 1, Text: "aaa"},
		{ID: 2, Text: "bbb"},
		{ID: 3, Text: "ccc"},
	}
	pool.Push(items)

	if pool.Len() != 3 {
		t.Fatalf("expected 3, got %d", pool.Len())
	}

	// 再次 Push 相同 ID，应该被跳过
	pool.Push(items)

	if pool.Len() != 3 {
		t.Fatalf("expected 3 after duplicate push, got %d", pool.Len())
	}
}

func TestPush_AllowsNewIDsAfterDuplicate(t *testing.T) {
	pool := NewMemoryPool(1, "contents", 100)

	pool.Push([]PoolItem{{ID: 1, Text: "aaa"}})

	// 混合新旧 ID
	pool.Push([]PoolItem{
		{ID: 1, Text: "aaa"}, // 重复，跳过
		{ID: 2, Text: "bbb"}, // 新的，加入
	})

	if pool.Len() != 2 {
		t.Fatalf("expected 2, got %d", pool.Len())
	}
}

func TestPop_DoesNotRemoveFromLoadedIDs(t *testing.T) {
	pool := NewMemoryPool(1, "contents", 100)

	pool.Push([]PoolItem{{ID: 1, Text: "aaa"}})

	// Pop 消费
	item, ok := pool.Pop()
	if !ok || item.ID != 1 {
		t.Fatalf("expected to pop ID 1")
	}

	// 再次 Push 相同 ID，应该仍然被跳过（loadedIDs 未移除）
	pool.Push([]PoolItem{{ID: 1, Text: "aaa"}})

	if pool.Len() != 0 {
		t.Fatalf("expected 0 after re-push of consumed ID, got %d", pool.Len())
	}
}

func TestClear_ResetsLoadedIDs(t *testing.T) {
	pool := NewMemoryPool(1, "contents", 100)

	pool.Push([]PoolItem{{ID: 1, Text: "aaa"}})
	pool.Clear()

	// Clear 后相同 ID 可以重新加载
	pool.Push([]PoolItem{{ID: 1, Text: "aaa"}})

	if pool.Len() != 1 {
		t.Fatalf("expected 1 after clear and re-push, got %d", pool.Len())
	}
}

func TestPush_MemoryBytesCorrectWithDedup(t *testing.T) {
	pool := NewMemoryPool(1, "contents", 100)

	pool.Push([]PoolItem{
		{ID: 1, Text: "hello"},
		{ID: 2, Text: "world"},
	})
	memAfterFirst := pool.MemoryBytes()

	// 重复 Push，内存不应增加
	pool.Push([]PoolItem{
		{ID: 1, Text: "hello"},
		{ID: 2, Text: "world"},
	})

	if pool.MemoryBytes() != memAfterFirst {
		t.Fatalf("memory should not increase on duplicate push: %d != %d", pool.MemoryBytes(), memAfterFirst)
	}
}
```

**Step 2: 运行测试确认失败**

```bash
cd api && go test ./internal/service/ -run "TestPush_SkipsDuplicateIDs|TestPush_AllowsNewIDsAfterDuplicate|TestPop_DoesNotRemoveFromLoadedIDs|TestClear_ResetsLoadedIDs|TestPush_MemoryBytesCorrectWithDedup" -v
```

预期：`TestPush_SkipsDuplicateIDs` 失败（Push 不会去重，Len 返回 6 而不是 3）。

---

### Task 2: MemoryPool 增加 ID 去重 — 实现

**Files:**
- Modify: `api/internal/service/memory_pool.go`

**Step 3: 修改 MemoryPool 结构体和构造函数**

在 `MemoryPool` 结构体中增加 `loadedIDs` 字段，并在 `NewMemoryPool` 中初始化：

```go
// MemoryPool struct 增加字段:
loadedIDs map[int64]struct{} // 已加载的 ID 集合（防止重复加载）
```

```go
// NewMemoryPool 增加初始化:
loadedIDs: make(map[int64]struct{}),
```

**Step 4: 修改 Push 方法增加去重**

将 `Push` 方法替换为以下实现：

```go
func (p *MemoryPool) Push(items []PoolItem) {
	if len(items) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Respect max size
	available := p.maxSize - len(p.items)
	if available <= 0 {
		return
	}

	// 增加内存计数
	var addedMem int64
	added := 0
	for _, item := range items {
		if added >= available {
			break
		}
		// 跳过已加载的 ID
		if _, exists := p.loadedIDs[item.ID]; exists {
			continue
		}
		p.loadedIDs[item.ID] = struct{}{}
		p.items = append(p.items, item)
		addedMem += StringMemorySize(item.Text)
		added++
	}
	p.memoryBytes.Add(addedMem)
}
```

**Step 5: 修改 Clear 方法重置 loadedIDs**

```go
func (p *MemoryPool) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.items = p.items[:0]
	p.loadedIDs = make(map[int64]struct{})
	p.memoryBytes.Store(0)
}
```

**Step 6: 运行测试确认通过**

```bash
cd api && go test ./internal/service/ -run "TestPush_SkipsDuplicateIDs|TestPush_AllowsNewIDsAfterDuplicate|TestPop_DoesNotRemoveFromLoadedIDs|TestClear_ResetsLoadedIDs|TestPush_MemoryBytesCorrectWithDedup" -v
```

预期：5 个测试全部 PASS。

**Step 7: 提交**

```bash
git add api/internal/service/memory_pool.go api/internal/service/memory_pool_test.go
git commit -m "fix: add ID dedup to MemoryPool to prevent duplicate loading"
```

---

### Task 3: 移除 refillPool 中的加载时标记

**Files:**
- Modify: `api/internal/service/pool_manager.go:326-334`

**Step 8: 删除 refillPool 中的 batcher.Add 循环**

在 `refillPool` 方法中，删除第 326-334 行（`memPool.Push(items)` 之后的 batcher 标记逻辑）：

```go
// 删除以下代码:
		// 立即将加载的数据标记为已使用（status=0），防止重复加载
		if m.batcher != nil {
			for _, item := range items {
				m.batcher.Add(pool.UpdateTask{
					Table: poolType,
					ID:    item.ID,
				})
			}
		}
```

修改后 `refillPool` 中 `if len(items) > 0` 代码块仅保留：

```go
	if len(items) > 0 {
		memPool.Push(items)

		log.Info().
			Str("type", poolType).
			Int("group", groupID).
			Int("added", len(items)).
			Int("total", memPool.Len()).
			Msg("Pool refilled")
	}
```

**Step 9: 确认编译通过**

```bash
cd api && go build ./...
```

预期：编译成功，无错误。

**Step 10: 提交**

```bash
git add api/internal/service/pool_manager.go
git commit -m "fix: remove premature status=0 marking from refillPool

refillPool was marking contents as status=0 immediately upon loading
into memory, causing data loss on service restart. Now status=0 is
only set when items are actually consumed via Pop."
```

---

### Task 4: 部署验证

**Step 11: 重建 API 容器**

```bash
cd /项目根目录 && docker compose up -d --build api
```

**Step 12: 恢复现有数据**

```bash
docker exec seo-generator-mysql mysql -uroot -pmysql_6yh7uJ seo_generator -e "UPDATE contents SET status = 1 WHERE status = 0;"
```

**Step 13: 验证池状态**

等待 5 秒后检查日志，确认 "Pool refilled" 出现：

```bash
docker logs seo-generator-api 2>&1 | grep -i "Pool refilled.*contents" | tail -5
```

预期：看到类似 `Pool refilled type=contents group=1 added=481 total=481`。

**Step 14: 验证数据库状态**

```bash
docker exec seo-generator-mysql mysql -uroot -pmysql_6yh7uJ seo_generator -e "SELECT group_id, SUM(status=1) as available, SUM(status=0) as used FROM contents GROUP BY group_id;"
```

预期：available=481, used=0（数据在内存池中但 DB 仍为 status=1，因为未被 Pop 消费）。

**Step 15: 验证前端缓存管理页面**

打开缓存管理页面 → 运行状态 → 消费型缓存 → 正文：可用数应为 481。
