// Package pool provides modular data pool implementations.
// This package extracts pool logic from pool_manager.go into
// independent, reusable components.
package pool

import "context"

// DataPool 数据池接口
// 定义所有数据池必须实现的基础操作
type DataPool interface {
	// Start 启动池,加载初始数据
	Start(ctx context.Context) error

	// Stop 停止池,释放资源
	Stop() error

	// Pop 从指定分组获取一个数据项
	Pop(groupID int) (string, error)

	// GetStats 获取指定分组的统计信息
	GetStats(groupID int) PoolStats

	// Reload 重新加载指定分组的数据
	Reload(ctx context.Context, groupIDs []int) error

	// RefillIfNeeded 根据需要补充指定分组
	RefillIfNeeded(ctx context.Context, groupID int) error
}

// PoolStats 池统计信息
type PoolStats struct {
	Current     int   // 当前数据量
	Capacity    int   // 容量
	GroupID     int   // 分组ID
	CacheHits   int64 // 缓存命中次数
	CacheMisses int64 // 缓存未命中次数
	MemoryBytes int64 // 内存占用(字节)
}

// Config 池配置
type Config struct {
	Size             int // 池大小
	Threshold        int // 补充阈值(绝对值)
	RefillIntervalMS int // 补充检查间隔(毫秒)
}
