package core

import "fmt"

// PoolPreset 池预设配置
type PoolPreset struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Concurrency int    `json:"concurrency"`
}

// 预定义的预设配置 (map 结构，key 为预设标识)
var PoolPresets = map[string]PoolPreset{
	"low":     {Name: "低", Description: "适用于小站点、低配服务器", Concurrency: 50},
	"medium":  {Name: "中", Description: "适用于中等规模站群", Concurrency: 200},
	"high":    {Name: "高", Description: "适用于大规模站群", Concurrency: 500},
	"extreme": {Name: "极高", Description: "适用于高性能服务器", Concurrency: 1000},
}

// GetAllPoolPresets 获取所有预设
func GetAllPoolPresets() map[string]PoolPreset {
	return PoolPresets
}

// GetPoolPreset 根据 key 获取预设
func GetPoolPreset(key string) (PoolPreset, bool) {
	preset, ok := PoolPresets[key]
	return preset, ok
}

// 默认缓冲秒数
const DefaultBufferSeconds = 3

// 单条数据大小估算（字节）
const (
	AvgClsSize          = 20
	AvgURLSize          = 100
	AvgKeywordEmojiSize = 60
	AvgNumberSize       = 8
)

// CalculatePoolSizes 根据预设、缓冲秒数和模板统计计算池大小
func CalculatePoolSizes(preset PoolPreset, maxStats TemplateFuncStats, bufferSeconds int) *PoolSizeConfig {
	if bufferSeconds <= 0 {
		bufferSeconds = DefaultBufferSeconds
	}
	multiplier := preset.Concurrency * bufferSeconds

	return &PoolSizeConfig{
		ClsPoolSize:          maxStats.Cls * multiplier,
		URLPoolSize:          maxStats.RandomURL * multiplier,
		KeywordEmojiPoolSize: maxStats.KeywordWithEmoji * multiplier,
		NumberPoolSize:       maxStats.RandomNumber * multiplier,
	}
}

// EstimateMemoryUsage 估算内存使用量（字节）
func EstimateMemoryUsage(sizes *PoolSizeConfig) int64 {
	const overhead = 1.2 // 20% 额外开销

	clsBytes := int64(float64(sizes.ClsPoolSize*AvgClsSize) * overhead)
	urlBytes := int64(float64(sizes.URLPoolSize*AvgURLSize) * overhead)
	keywordEmojiBytes := int64(float64(sizes.KeywordEmojiPoolSize*AvgKeywordEmojiSize) * overhead)
	numberBytes := int64(float64(sizes.NumberPoolSize*AvgNumberSize) * overhead)

	return clsBytes + urlBytes + keywordEmojiBytes + numberBytes
}

// FormatMemorySize 格式化内存大小为人类可读格式
func FormatMemorySize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
