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
