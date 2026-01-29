package core

import (
	"github.com/rs/zerolog/log"
)

// SEORating SEO 评级类型
type SEORating string

const (
	SEORatingExcellent SEORating = "excellent" // 优秀
	SEORatingGood      SEORating = "good"      // 良好
	SEORatingFair      SEORating = "fair"      // 一般
	SEORatingPoor      SEORating = "poor"      // 较差
)

// DataPoolStats 数据池统计
type DataPoolStats struct {
	Name             string    `json:"name"`
	CurrentSize      int       `json:"current_size"`
	RequiredSize     int       `json:"required_size"`
	RecommendedSize  int       `json:"recommended_size"`
	UtilizationRatio float64   `json:"utilization_ratio"` // 使用率 = required / current
	Rating           SEORating `json:"rating"`
	Issue            string    `json:"issue,omitempty"`
}

// DataPoolSEOAnalysis SEO 分析结果
type DataPoolSEOAnalysis struct {
	OverallRating SEORating        `json:"overall_rating"`
	Score         int              `json:"score"` // 0-100
	Pools         []*DataPoolStats `json:"pools"`
	Suggestions   []string         `json:"suggestions"`
	TargetQPS     int              `json:"target_qps"`
	SafetyFactor  float64          `json:"safety_factor"`
}

// SEOAnalyzer SEO 友好度分析器
type SEOAnalyzer struct {
	templateAnalyzer *TemplateAnalyzer
}

// NewSEOAnalyzer 创建 SEO 分析器
func NewSEOAnalyzer(analyzer *TemplateAnalyzer) *SEOAnalyzer {
	return &SEOAnalyzer{
		templateAnalyzer: analyzer,
	}
}

// AnalyzeSEOFriendliness 分析 SEO 友好度
func (s *SEOAnalyzer) AnalyzeSEOFriendliness(currentPools map[string]int) *DataPoolSEOAnalysis {
	if s.templateAnalyzer == nil {
		return &DataPoolSEOAnalysis{
			OverallRating: SEORatingPoor,
			Score:         0,
			Suggestions:   []string{"模板分析器未初始化"},
		}
	}

	poolConfig := s.templateAnalyzer.CalculatePoolSize()
	maxStats := s.templateAnalyzer.GetMaxStats()

	analysis := &DataPoolSEOAnalysis{
		Pools:        make([]*DataPoolStats, 0),
		Suggestions:  make([]string, 0),
		TargetQPS:    s.templateAnalyzer.targetQPS,
		SafetyFactor: s.templateAnalyzer.safetyFactor,
	}

	// 分析各个数据池
	poolsToAnalyze := []struct {
		name         string
		currentSize  int
		requiredSize int
		maxCalls     int
	}{
		{"cls", currentPools["cls"], poolConfig.ClsPoolSize, maxStats.Cls},
		{"url", currentPools["url"], poolConfig.URLPoolSize, maxStats.RandomURL},
		{"keyword_emoji", currentPools["keyword_emoji"], poolConfig.KeywordEmojiPoolSize, maxStats.KeywordWithEmoji},
		{"number", currentPools["number"], poolConfig.NumberPoolSize, maxStats.RandomNumber},
	}

	totalScore := 0
	analyzedPools := 0

	for _, pool := range poolsToAnalyze {
		if pool.maxCalls == 0 {
			// 此函数未被使用，跳过
			continue
		}

		stats := s.analyzePool(pool.name, pool.currentSize, pool.requiredSize, pool.maxCalls)
		analysis.Pools = append(analysis.Pools, stats)

		// 计算评分
		poolScore := s.calculatePoolScore(stats)
		totalScore += poolScore
		analyzedPools++

		// 添加建议
		if stats.Issue != "" {
			analysis.Suggestions = append(analysis.Suggestions, stats.Issue)
		}
	}

	// 计算总体评分
	if analyzedPools > 0 {
		analysis.Score = totalScore / analyzedPools
	}

	// 确定总体评级
	analysis.OverallRating = s.scoreToRating(analysis.Score)

	// 添加通用建议
	if analysis.Score < 60 {
		analysis.Suggestions = append(analysis.Suggestions, "建议增加数据池大小以提高 SEO 多样性")
	}
	if len(analysis.Pools) == 0 {
		analysis.Suggestions = append(analysis.Suggestions, "未检测到模板函数调用，请确保模板已正确分析")
	}

	log.Info().
		Str("rating", string(analysis.OverallRating)).
		Int("score", analysis.Score).
		Int("pools", len(analysis.Pools)).
		Msg("SEO friendliness analyzed")

	return analysis
}

// analyzePool 分析单个数据池
func (s *SEOAnalyzer) analyzePool(name string, currentSize, requiredSize, maxCalls int) *DataPoolStats {
	stats := &DataPoolStats{
		Name:            name,
		CurrentSize:     currentSize,
		RequiredSize:    requiredSize,
		RecommendedSize: s.GetRecommendedPoolSize(maxCalls),
	}

	// 计算使用率
	if currentSize > 0 {
		stats.UtilizationRatio = float64(requiredSize) / float64(currentSize)
	} else {
		stats.UtilizationRatio = 0
	}

	// 确定评级和问题
	if currentSize == 0 {
		stats.Rating = SEORatingPoor
		stats.Issue = name + " 池未初始化，无法提供随机数据"
	} else if currentSize < requiredSize {
		ratio := float64(currentSize) / float64(requiredSize)
		if ratio < 0.3 {
			stats.Rating = SEORatingPoor
			stats.Issue = name + " 池容量严重不足，可能导致数据重复率过高"
		} else if ratio < 0.6 {
			stats.Rating = SEORatingFair
			stats.Issue = name + " 池容量不足，建议增加到 " + seoFormatNumber(stats.RecommendedSize)
		} else {
			stats.Rating = SEORatingGood
			stats.Issue = name + " 池容量略有不足，可考虑适当增加"
		}
	} else {
		stats.Rating = SEORatingExcellent
	}

	return stats
}

// GetRecommendedPoolSize 获取推荐的池大小
func (s *SEOAnalyzer) GetRecommendedPoolSize(maxCallsPerRequest int) int {
	if s.templateAnalyzer == nil {
		return maxCallsPerRequest * 500 // 默认 500 QPS
	}

	s.templateAnalyzer.mu.RLock()
	targetQPS := s.templateAnalyzer.targetQPS
	safetyFactor := s.templateAnalyzer.safetyFactor
	s.templateAnalyzer.mu.RUnlock()

	return int(float64(maxCallsPerRequest) * float64(targetQPS) * safetyFactor)
}

// calculatePoolScore 计算单个池的评分
func (s *SEOAnalyzer) calculatePoolScore(stats *DataPoolStats) int {
	if stats.CurrentSize == 0 {
		return 0
	}
	if stats.RequiredSize == 0 {
		return 100
	}

	ratio := float64(stats.CurrentSize) / float64(stats.RequiredSize)
	if ratio >= 1.0 {
		return 100
	}
	return int(ratio * 100)
}

// scoreToRating 将评分转换为评级
func (s *SEOAnalyzer) scoreToRating(score int) SEORating {
	switch {
	case score >= 90:
		return SEORatingExcellent
	case score >= 70:
		return SEORatingGood
	case score >= 50:
		return SEORatingFair
	default:
		return SEORatingPoor
	}
}

// seoFormatNumber 格式化数字（SEO 分析器专用）
func seoFormatNumber(n int) string {
	if n >= 1000000 {
		return seoFormatFloat(float64(n)/1000000) + "M"
	}
	if n >= 1000 {
		return seoFormatFloat(float64(n)/1000) + "K"
	}
	return seoFormatInt(n)
}

func seoFormatFloat(f float64) string {
	if f == float64(int(f)) {
		return seoFormatInt(int(f))
	}
	return seoFormatFloatWithPrecision(f, 1)
}

func seoFormatInt(n int) string {
	s := ""
	for n > 0 {
		s = string('0'+byte(n%10)) + s
		n /= 10
	}
	if s == "" {
		return "0"
	}
	return s
}

func seoFormatFloatWithPrecision(f float64, precision int) string {
	intPart := int(f)
	fracPart := int((f - float64(intPart)) * 10)
	if fracPart < 0 {
		fracPart = -fracPart
	}
	return seoFormatInt(intPart) + "." + seoFormatInt(fracPart)
}
