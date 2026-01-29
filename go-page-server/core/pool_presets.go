package core

import "fmt"

// PoolPreset 池预设配置
type PoolPreset struct {
	Name          string  `json:"name"`
	Description   string  `json:"description"`
	TargetQPS     int     `json:"target_qps"`
	SafetyFactor  float64 `json:"safety_factor"`
	BufferSeconds float64 `json:"buffer_seconds"`
}

// 预定义的预设配置
var PoolPresets = map[string]PoolPreset{
	"low": {
		Name:          "低并发",
		Description:   "适用于 100 QPS 以下的场景",
		TargetQPS:     100,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
	"medium": {
		Name:          "中并发",
		Description:   "适用于 500 QPS 左右的场景",
		TargetQPS:     500,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
	"high": {
		Name:          "高并发",
		Description:   "适用于 1000 QPS 左右的场景",
		TargetQPS:     1000,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
	"extreme": {
		Name:          "超高并发",
		Description:   "适用于 2000+ QPS 的场景",
		TargetQPS:     2000,
		SafetyFactor:  1.5,
		BufferSeconds: 1.0,
	},
}

// GetPoolPreset 获取预设配置
func GetPoolPreset(name string) (PoolPreset, bool) {
	preset, ok := PoolPresets[name]
	return preset, ok
}

// GetAllPoolPresets 获取所有预设配置
func GetAllPoolPresets() map[string]PoolPreset {
	// 返回副本，避免外部修改
	result := make(map[string]PoolPreset, len(PoolPresets))
	for k, v := range PoolPresets {
		result[k] = v
	}
	return result
}

// CalculatePoolSizes 根据预设和模板分析计算池大小
// 参数 maxStats 是 TemplateFuncStats 类型（已在 template_analyzer.go 定义）
// 计算公式：poolSize = calls * targetQPS * safetyFactor * bufferSeconds
func CalculatePoolSizes(preset PoolPreset, maxStats TemplateFuncStats) *PoolSizeConfig {
	multiplier := float64(preset.TargetQPS) * preset.SafetyFactor * preset.BufferSeconds

	return &PoolSizeConfig{
		ClsPoolSize:          int(float64(maxStats.Cls) * multiplier),
		URLPoolSize:          int(float64(maxStats.RandomURL) * multiplier),
		KeywordEmojiPoolSize: int(float64(maxStats.KeywordWithEmoji) * multiplier),
		NumberPoolSize:       int(float64(maxStats.RandomNumber) * multiplier),
	}
}

// EstimateMemoryUsage 估算内存使用量（字节）
// 假设：
// - 每个字符串平均 20 字节
// - emoji 相关的字符串可能更大，按 2 倍计算（40 字节）
func EstimateMemoryUsage(config *PoolSizeConfig) int64 {
	const (
		avgStringSize      = 20 // 普通字符串平均大小
		avgEmojiStringSize = 40 // emoji 字符串平均大小（x2）
	)

	var totalBytes int64

	// cls 池：普通字符串
	totalBytes += int64(config.ClsPoolSize) * avgStringSize

	// URL 池：普通字符串
	totalBytes += int64(config.URLPoolSize) * avgStringSize

	// emoji 关键词池：emoji 字符串
	totalBytes += int64(config.KeywordEmojiPoolSize) * avgEmojiStringSize

	// 数字池：普通字符串（数字转字符串通常很短，但按平均算）
	totalBytes += int64(config.NumberPoolSize) * avgStringSize

	return totalBytes
}

// FormatMemorySize 格式化内存大小为人类可读格式
func FormatMemorySize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
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
