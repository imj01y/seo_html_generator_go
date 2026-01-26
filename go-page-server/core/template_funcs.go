package core

import (
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"
)

// TemplateFuncsManager 模板函数管理器（高并发版）
type TemplateFuncsManager struct {
	// 预生成池
	clsPool          *ObjectPool[string]
	urlPool          *ObjectPool[string]
	keywordEmojiPool *ObjectPool[string] // 带 emoji 的关键词池
	numberPool       *NumberPool

	// 关键词（原子计数器访问）
	keywords   []string
	keywordIdx int64
	keywordLen int64

	// 原始关键词（未编码，用于生成带 emoji 的关键词）
	rawKeywords   []string
	rawKeywordIdx int64
	rawKeywordLen int64

	// 图片URL（原子计数器访问）
	imageURLs []string
	imageIdx  int64
	imageLen  int64

	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager // emoji 管理器引用
}

// NewTemplateFuncsManager 创建管理器
func NewTemplateFuncsManager(encoder *HTMLEntityEncoder) *TemplateFuncsManager {
	return &TemplateFuncsManager{
		encoder: encoder,
	}
}

// SetEmojiManager 设置 emoji 管理器引用
func (m *TemplateFuncsManager) SetEmojiManager(em *EmojiManager) {
	m.emojiManager = em
}

// InitPools 初始化所有池子（支持500 QPS）
func (m *TemplateFuncsManager) InitPools() {
	// cls池：500K容量，支持500QPS
	m.clsPool = NewObjectPool[string](PoolConfig{
		Name:          "cls",
		Size:          500000,
		LowWatermark:  0.3,
		RefillBatch:   100000,
		NumWorkers:    16,
		CheckInterval: 50 * time.Millisecond,
	}, generateRandomCls)

	// url池：300K容量
	m.urlPool = NewObjectPool[string](PoolConfig{
		Name:          "url",
		Size:          300000,
		LowWatermark:  0.3,
		RefillBatch:   80000,
		NumWorkers:    12,
		CheckInterval: 50 * time.Millisecond,
	}, generateRandomURL)

	// number池
	m.numberPool = NewNumberPool()

	// 启动所有池
	m.clsPool.Start()
	m.urlPool.Start()
	m.numberPool.Start()
}

// InitKeywordEmojiPool 初始化带 emoji 的关键词池
func (m *TemplateFuncsManager) InitKeywordEmojiPool() {
	if m.emojiManager == nil || atomic.LoadInt64(&m.rawKeywordLen) == 0 {
		return
	}

	// 500K 容量，与 clsPool 一致，支持 500 QPS
	m.keywordEmojiPool = NewObjectPool[string](PoolConfig{
		Name:          "keyword_emoji",
		Size:          500000,
		LowWatermark:  0.3,
		RefillBatch:   100000,
		NumWorkers:    16,
		CheckInterval: 50 * time.Millisecond,
	}, m.generateKeywordWithEmoji)

	m.keywordEmojiPool.Start()
}

// generateKeywordWithEmoji 生成带 emoji 的关键词
func (m *TemplateFuncsManager) generateKeywordWithEmoji() string {
	// 1. 获取原始关键词
	length := atomic.LoadInt64(&m.rawKeywordLen)
	if length == 0 {
		return ""
	}
	idx := atomic.AddInt64(&m.rawKeywordIdx, 1) - 1
	keyword := m.rawKeywords[idx%length]

	// 2. 如果 emojiManager 为 nil，直接返回关键词
	if m.emojiManager == nil {
		return m.encoder.EncodeText(keyword)
	}

	// 3. 随机决定插入 1 或 2 个 emoji（50% 概率）
	emojiCount := 1
	if rand.Float64() < 0.5 {
		emojiCount = 2
	}

	// 4. 转换为 rune 切片处理中文
	runes := []rune(keyword)
	runeLen := len(runes)
	if runeLen == 0 {
		return m.encoder.EncodeText(keyword)
	}

	// 5. 插入 emoji
	exclude := make(map[string]bool)
	for i := 0; i < emojiCount; i++ {
		pos := rand.Intn(runeLen + 1) // 0 到 len，包含首尾
		emoji := m.emojiManager.GetRandomExclude(exclude)
		if emoji != "" {
			exclude[emoji] = true
			// 在位置插入
			newRunes := make([]rune, 0, len(runes)+len([]rune(emoji)))
			newRunes = append(newRunes, runes[:pos]...)
			newRunes = append(newRunes, []rune(emoji)...)
			newRunes = append(newRunes, runes[pos:]...)
			runes = newRunes
			runeLen = len(runes)
		}
	}

	// 6. 编码并返回
	return m.encoder.EncodeText(string(runes))
}

// StopPools 停止所有池
func (m *TemplateFuncsManager) StopPools() {
	if m.clsPool != nil {
		m.clsPool.Stop()
	}
	if m.urlPool != nil {
		m.urlPool.Stop()
	}
	if m.keywordEmojiPool != nil {
		m.keywordEmojiPool.Stop()
	}
	if m.numberPool != nil {
		m.numberPool.Stop()
	}
}

// LoadKeywords 加载关键词
func (m *TemplateFuncsManager) LoadKeywords(keywords []string) int {
	// 预编码
	encoded := make([]string, len(keywords))
	for i, kw := range keywords {
		encoded[i] = m.encoder.EncodeText(kw)
	}

	// 洗牌
	rand.Shuffle(len(encoded), func(i, j int) {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	})

	m.keywords = encoded
	atomic.StoreInt64(&m.keywordLen, int64(len(encoded)))
	atomic.StoreInt64(&m.keywordIdx, 0)

	// 存储原始关键词（未编码，用于生成带 emoji 的关键词）
	rawCopy := make([]string, len(keywords))
	copy(rawCopy, keywords)
	rand.Shuffle(len(rawCopy), func(i, j int) {
		rawCopy[i], rawCopy[j] = rawCopy[j], rawCopy[i]
	})
	m.rawKeywords = rawCopy
	atomic.StoreInt64(&m.rawKeywordLen, int64(len(rawCopy)))
	atomic.StoreInt64(&m.rawKeywordIdx, 0)

	return len(encoded)
}

// LoadImageURLs 加载图片URL
func (m *TemplateFuncsManager) LoadImageURLs(urls []string) int {
	copied := make([]string, len(urls))
	copy(copied, urls)

	rand.Shuffle(len(copied), func(i, j int) {
		copied[i], copied[j] = copied[j], copied[i]
	})

	m.imageURLs = copied
	atomic.StoreInt64(&m.imageLen, int64(len(copied)))
	atomic.StoreInt64(&m.imageIdx, 0)

	return len(copied)
}

// ========== 模板函数（全部无锁，O(1)） ==========

// Cls 从池中获取随机class
func (m *TemplateFuncsManager) Cls(name string) string {
	if m.clsPool != nil {
		return m.clsPool.Get() + " " + name
	}
	// 降级到直接生成
	return generateRandomCls() + " " + name
}

// RandomURL 从池中获取随机URL
func (m *TemplateFuncsManager) RandomURL() string {
	if m.urlPool != nil {
		return m.urlPool.Get()
	}
	// 降级到直接生成
	return generateRandomURL()
}

// RandomKeyword 获取随机关键词（原子操作）
func (m *TemplateFuncsManager) RandomKeyword() string {
	length := atomic.LoadInt64(&m.keywordLen)
	if length == 0 {
		return ""
	}
	idx := atomic.AddInt64(&m.keywordIdx, 1) - 1
	return m.keywords[idx%length]
}

// RandomKeywordEmoji 从池中获取带 emoji 的随机关键词
func (m *TemplateFuncsManager) RandomKeywordEmoji() string {
	if m.keywordEmojiPool != nil {
		return m.keywordEmojiPool.Get()
	}
	// 降级：实时生成
	return m.generateKeywordWithEmoji()
}

// RandomImage 获取随机图片URL（原子操作）
func (m *TemplateFuncsManager) RandomImage() string {
	length := atomic.LoadInt64(&m.imageLen)
	if length == 0 {
		return ""
	}
	idx := atomic.AddInt64(&m.imageIdx, 1) - 1
	return m.imageURLs[idx%length]
}

// RandomNumber 获取随机数
func (m *TemplateFuncsManager) RandomNumber(min, max int) int {
	if min >= max {
		return min
	}
	if m.numberPool != nil {
		return m.numberPool.Get(min, max)
	}
	// 降级到直接生成
	return rand.Intn(max-min+1) + min
}

// Encode 编码文本
func (m *TemplateFuncsManager) Encode(text string) string {
	return m.encoder.EncodeText(text)
}

// ========== 生成函数 ==========

func generateRandomCls() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	part1 := make([]byte, 13)
	for i := range part1 {
		part1[i] = chars[rand.Intn(len(chars))]
	}
	part2 := make([]byte, 32)
	for i := range part2 {
		part2[i] = chars[rand.Intn(len(chars))]
	}
	return string(part1) + " " + string(part2)
}

func generateRandomURL() string {
	if rand.Float64() < 0.6 {
		num := rand.Intn(900000000) + 100000000
		return fmt.Sprintf("/?%d.html", num)
	}
	daysAgo := rand.Intn(30)
	date := time.Now().AddDate(0, 0, -daysAgo)
	dateStr := date.Format("20060102")
	num := rand.Intn(90000) + 10000
	return fmt.Sprintf("/?%s/%d.html", dateStr, num)
}

// ========== 统计 ==========

// GetStats returns statistics about loaded data
func (m *TemplateFuncsManager) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"keywords_count": atomic.LoadInt64(&m.keywordLen),
		"images_count":   atomic.LoadInt64(&m.imageLen),
		"keyword_idx":    atomic.LoadInt64(&m.keywordIdx),
		"image_idx":      atomic.LoadInt64(&m.imageIdx),
	}

	if m.clsPool != nil {
		stats["cls_pool"] = m.clsPool.Stats()
	}
	if m.urlPool != nil {
		stats["url_pool"] = m.urlPool.Stats()
	}
	if m.keywordEmojiPool != nil {
		stats["keyword_emoji_pool"] = m.keywordEmojiPool.Stats()
	}
	if m.numberPool != nil {
		stats["number_pools"] = m.numberPool.Stats()
	}

	return stats
}

// ========== 辅助函数（保持兼容） ==========

// IterateFunc returns a slice of integers for template iteration
func IterateFunc(n int) []int {
	result := make([]int, n)
	for i := 0; i < n; i++ {
		result[i] = i
	}
	return result
}

// NowFunc returns the current time formatted as string
func NowFunc() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// BuildArticleContent builds article content from titles and content
func BuildArticleContent(titles []string, content string) string {
	if len(titles) < 4 {
		return content
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")

	var sb strings.Builder
	sb.WriteString(titles[0])
	sb.WriteString("\n\n")
	sb.WriteString(titles[1])
	sb.WriteString("\n\n")
	sb.WriteString(titles[2])
	sb.WriteString("\n\n")
	sb.WriteString("厂商新闻：")
	sb.WriteString(titles[3])
	sb.WriteString(" 时间：")
	sb.WriteString(nowStr)
	sb.WriteString("\n\n编辑：admin\n")
	sb.WriteString(nowStr)
	sb.WriteString("\n\n　")
	sb.WriteString(content)
	sb.WriteString("\n\nadmin】")

	return sb.String()
}
