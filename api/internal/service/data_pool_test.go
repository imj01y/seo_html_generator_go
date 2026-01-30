package core

import (
	"testing"
)

// TestDataPool_BasicOperations 基本操作测试
func TestDataPool_BasicOperations(t *testing.T) {
	pool := NewDataPool("test_pool")

	// 测试空池
	if pool.Count() != 0 {
		t.Errorf("新建的池应该是空的，实际 count = %d", pool.Count())
	}

	// 加载数据
	testData := []string{"item1", "item2", "item3"}
	pool.Load(testData)

	// 验证数据量
	if pool.Count() != 3 {
		t.Errorf("加载后池应该有 3 个元素，实际 count = %d", pool.Count())
	}

	// 测试 Get 返回的是池中的数据
	for i := 0; i < 100; i++ {
		item := pool.Get()
		found := false
		for _, expected := range testData {
			if item == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Get() 返回了不在池中的数据: %s", item)
		}
	}
}

// TestDataPool_GetN 测试 GetN 方法
func TestDataPool_GetN(t *testing.T) {
	pool := NewDataPool("test_pool")

	// 加载 3 个元素
	testData := []string{"a", "b", "c"}
	pool.Load(testData)

	// 请求 5 个
	result := pool.GetN(5)

	// 验证返回 5 个
	if len(result) != 5 {
		t.Errorf("GetN(5) 应该返回 5 个元素，实际返回 %d 个", len(result))
	}

	// 验证所有返回的元素都在池中
	for _, item := range result {
		found := false
		for _, expected := range testData {
			if item == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetN() 返回了不在池中的数据: %s", item)
		}
	}
}

// TestDataPool_GetUnique 测试 GetUnique 方法
func TestDataPool_GetUnique(t *testing.T) {
	pool := NewDataPool("test_pool")

	// 加载 5 个元素
	testData := []string{"a", "b", "c", "d", "e"}
	pool.Load(testData)

	// 获取 3 个不重复的
	result := pool.GetUnique(3)

	// 验证返回 3 个
	if len(result) != 3 {
		t.Errorf("GetUnique(3) 应该返回 3 个元素，实际返回 %d 个", len(result))
	}

	// 验证无重复
	seen := make(map[string]bool)
	for _, item := range result {
		if seen[item] {
			t.Errorf("GetUnique() 返回了重复的元素: %s", item)
		}
		seen[item] = true
	}

	// 验证所有返回的元素都在池中
	for _, item := range result {
		found := false
		for _, expected := range testData {
			if item == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("GetUnique() 返回了不在池中的数据: %s", item)
		}
	}
}

// TestDataPool_GetUniqueExceedsSize 测试 GetUnique 超出池大小的情况
func TestDataPool_GetUniqueExceedsSize(t *testing.T) {
	pool := NewDataPool("test_pool")

	// 加载 3 个元素
	testData := []string{"x", "y", "z"}
	pool.Load(testData)

	// 请求 10 个
	result := pool.GetUnique(10)

	// 验证只返回 3 个（池大小）
	if len(result) != 3 {
		t.Errorf("GetUnique(10) 在池大小为 3 时应该返回 3 个元素，实际返回 %d 个", len(result))
	}

	// 验证无重复
	seen := make(map[string]bool)
	for _, item := range result {
		if seen[item] {
			t.Errorf("GetUnique() 返回了重复的元素: %s", item)
		}
		seen[item] = true
	}
}

// TestDataPool_EmptyPool 测试空池
func TestDataPool_EmptyPool(t *testing.T) {
	pool := NewDataPool("empty_pool")

	// Get 返回空字符串
	item := pool.Get()
	if item != "" {
		t.Errorf("空池 Get() 应该返回空字符串，实际返回: %s", item)
	}

	// GetN 返回 nil
	items := pool.GetN(5)
	if items != nil {
		t.Errorf("空池 GetN() 应该返回 nil，实际返回: %v", items)
	}

	// GetUnique 返回 nil
	uniqueItems := pool.GetUnique(5)
	if uniqueItems != nil {
		t.Errorf("空池 GetUnique() 应该返回 nil，实际返回: %v", uniqueItems)
	}
}

// TestDataPool_Stats 测试统计功能
func TestDataPool_Stats(t *testing.T) {
	pool := NewDataPool("stats_pool")

	// 加载数据
	testData := []string{"item1", "item2", "item3"}
	pool.Load(testData)

	// 获取几次数据以产生统计
	pool.Get()
	pool.Get()
	pool.GetN(3)

	// 获取统计信息
	stats := pool.Stats()

	// 验证 name
	if stats["name"] != "stats_pool" {
		t.Errorf("统计中的 name 应该是 'stats_pool'，实际是: %v", stats["name"])
	}

	// 验证 count
	if stats["count"] != 3 {
		t.Errorf("统计中的 count 应该是 3，实际是: %v", stats["count"])
	}

	// 验证 total_selects (2 次 Get + 3 次 GetN = 5)
	expectedSelects := int64(5)
	if stats["total_selects"] != expectedSelects {
		t.Errorf("统计中的 total_selects 应该是 %d，实际是: %v", expectedSelects, stats["total_selects"])
	}
}

// TestPoolRecommendation 测试优化建议
func TestPoolRecommendation(t *testing.T) {
	testCases := []struct {
		name         string
		dataType     string
		currentCount int
		callsPerPage int
		expected     SEORating
	}{
		{
			name:         "excellent - 10000 数据, 100 调用",
			dataType:     "关键词",
			currentCount: 10000,
			callsPerPage: 100,
			expected:     SEORatingExcellent,
		},
		{
			name:         "good - 1000 数据, 100 调用",
			dataType:     "关键词",
			currentCount: 1000,
			callsPerPage: 100,
			expected:     SEORatingGood,
		},
		{
			name:         "fair - 500 数据, 100 调用",
			dataType:     "关键词",
			currentCount: 500,
			callsPerPage: 100,
			expected:     SEORatingFair,
		},
		{
			name:         "poor - 100 数据, 100 调用",
			dataType:     "关键词",
			currentCount: 100,
			callsPerPage: 100,
			expected:     SEORatingPoor,
		},
		{
			name:         "模板未使用 - 0 调用",
			dataType:     "关键词",
			currentCount: 0,
			callsPerPage: 0,
			expected:     SEORatingExcellent,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := getRecommendation(tc.dataType, tc.currentCount, tc.callsPerPage)
			if rec.Status != tc.expected {
				t.Errorf("getRecommendation(%s, %d, %d) 返回 Status = %s，期望 %s (重复率: %.2f%%)",
					tc.dataType, tc.currentCount, tc.callsPerPage, rec.Status, tc.expected, rec.RepeatRate)
			}
		})
	}
}
