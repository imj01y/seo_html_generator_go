package core

import (
	"fmt"
	"math/rand/v2"
	"time"
)

// NumberPool 随机数池管理器
type NumberPool struct {
	pools map[string]*ObjectPool[int]
}

// NewNumberPool 创建随机数池
func NewNumberPool() *NumberPool {
	np := &NumberPool{
		pools: make(map[string]*ObjectPool[int]),
	}

	// 预定义常用范围（根据模板中实际使用的范围）
	ranges := map[string][2]int{
		"0-9":         {0, 9},
		"0-99":        {0, 99},
		"1-9":         {1, 9},
		"1-10":        {1, 10},
		"1-20":        {1, 20},
		"5-10":        {5, 10},
		"10-99":       {10, 99},
		"10-100":      {10, 100},
		"10-200":      {10, 200},
		"30-90":       {30, 90},
		"50-200":      {50, 200},
		"100-999":     {100, 999},
		"1000-9999":   {1000, 9999},
		"10000-99999": {10000, 99999},
	}

	for key, r := range ranges {
		minVal, maxVal := r[0], r[1]
		cfg := PoolConfig{
			Name:          "number_" + key,
			Size:          200000,
			LowWatermark:  0.4,
			RefillBatch:   50000,
			NumWorkers:    8,
			CheckInterval: 30 * time.Millisecond,
		}

		// 使用闭包捕获min/max
		generator := func(min, max int) func() int {
			return func() int {
				return rand.IntN(max-min+1) + min
			}
		}(minVal, maxVal)

		np.pools[key] = NewObjectPool[int](cfg, generator)
	}

	return np
}

// Start 启动所有池
func (np *NumberPool) Start() {
	for _, pool := range np.pools {
		pool.Start()
	}
}

// Get 获取随机数
func (np *NumberPool) Get(min, max int) int {
	key := fmt.Sprintf("%d-%d", min, max)
	if pool, ok := np.pools[key]; ok {
		return pool.Get()
	}
	// 降级到直接生成
	return rand.IntN(max-min+1) + min
}

// Stop 停止所有池
func (np *NumberPool) Stop() {
	for _, pool := range np.pools {
		pool.Stop()
	}
}

// Stats 返回所有池统计
func (np *NumberPool) Stats() map[string]interface{} {
	stats := make(map[string]interface{})
	for key, pool := range np.pools {
		stats[key] = pool.Stats()
	}
	return stats
}
