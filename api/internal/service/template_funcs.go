package core

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// ImageData 图片数据（不可变，通过原子指针替换）
type ImageData struct {
	groups map[int][]string // groupID -> urls
}

// KeywordData 关键词数据（不可变，通过原子指针替换）
type KeywordData struct {
	groups    map[int][]string // groupID -> encoded keywords
	rawGroups map[int][]string // groupID -> raw keywords
}

// TemplateFuncsManager 模板函数管理器（高并发版）
type TemplateFuncsManager struct {
	// 预生成池
	clsPool    *ObjectPool[string]
	urlPool    *ObjectPool[string]
	numberPool *NumberPool

	// 关键词数据（原子指针，支持无锁读取和热更新）
	keywordData atomic.Pointer[KeywordData]

	// 分组索引（独立管理，避免数据替换时重置）
	keywordGroupIdx    sync.Map // groupID -> *atomic.Int64
	rawKeywordGroupIdx sync.Map // groupID -> *atomic.Int64

	// 图片数据（原子指针，支持无锁读取和热更新）
	imageData atomic.Pointer[ImageData]

	// 分组索引（独立管理，避免数据替换时重置）
	imageGroupIdx sync.Map // groupID -> *atomic.Int64

	encoder               *HTMLEntityEncoder
	emojiManager          *EmojiManager          // emoji 管理器引用
	keywordEmojiGenerator *KeywordEmojiGenerator // 关键词表情生成器引用
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

// SetKeywordEmojiGenerator 设置关键词表情生成器引用
func (m *TemplateFuncsManager) SetKeywordEmojiGenerator(gen *KeywordEmojiGenerator) {
	m.keywordEmojiGenerator = gen
}

// stringMemorySizer 计算字符串内存占用的函数
func stringMemorySizer(v any) int64 {
	if s, ok := v.(string); ok {
		return StringMemorySize(s)
	}
	return 0
}

// InitPools 初始化所有池子（从配置读取）
func (m *TemplateFuncsManager) InitPools(config *CachePoolConfig) {
	// cls池
	m.clsPool = NewObjectPool[string](PoolConfig{
		Name:          "cls",
		Size:          config.ClsPoolSize,
		Threshold:     config.ClsThreshold,
		NumWorkers:    config.ClsWorkers,
		CheckInterval: config.ClsRefillInterval(),
		MemorySizer:   stringMemorySizer,
	}, generateRandomCls)

	// url池
	m.urlPool = NewObjectPool[string](PoolConfig{
		Name:          "url",
		Size:          config.UrlPoolSize,
		Threshold:     config.UrlThreshold,
		NumWorkers:    config.UrlWorkers,
		CheckInterval: config.UrlRefillInterval(),
		MemorySizer:   stringMemorySizer,
	}, generateRandomURL)

	// number池
	m.numberPool = NewNumberPool()

	// 启动所有池
	m.clsPool.Start()
	m.urlPool.Start()
	m.numberPool.Start()
}

// generateKeywordWithEmojiFromRaw 从原始关键词生成带 emoji 的版本
func (m *TemplateFuncsManager) generateKeywordWithEmojiFromRaw(keyword string) string {
	// 如果 emojiManager 为 nil，直接返回编码后的关键词
	if m.emojiManager == nil {
		return m.encoder.EncodeText(keyword)
	}

	// 随机决定插入 1 或 2 个 emoji（50% 概率）
	emojiCount := 1
	if rand.Float64() < 0.5 {
		emojiCount = 2
	}

	// 转换为 rune 切片处理中文
	runes := []rune(keyword)
	runeLen := len(runes)
	if runeLen == 0 {
		return m.encoder.EncodeText(keyword)
	}

	// 插入 emoji
	exclude := make(map[string]bool)
	for i := 0; i < emojiCount; i++ {
		pos := rand.IntN(runeLen + 1) // 0 到 len，包含首尾
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

	// 编码并返回
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
	if m.numberPool != nil {
		m.numberPool.Stop()
	}
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

	// 获取或创建该分组的索引
	idxPtr, _ := m.keywordGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
	idx := idxPtr.(*atomic.Int64).Add(1) - 1

	return keywords[idx%int64(len(keywords))]
}

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
	idxPtr, _ := m.rawKeywordGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
	idx := idxPtr.(*atomic.Int64).Add(1) - 1
	keyword := rawKeywords[idx%int64(len(rawKeywords))]
	return m.generateKeywordWithEmojiFromRaw(keyword)
}

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

	// 获取或创建该分组的索引
	idxPtr, _ := m.imageGroupIdx.LoadOrStore(groupID, &atomic.Int64{})
	idx := idxPtr.(*atomic.Int64).Add(1) - 1

	return urls[idx%int64(len(urls))]
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
	return rand.IntN(max-min+1) + min
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
		part1[i] = chars[rand.IntN(len(chars))]
	}
	part2 := make([]byte, 32)
	for i := range part2 {
		part2[i] = chars[rand.IntN(len(chars))]
	}
	return string(part1) + " " + string(part2)
}

func generateRandomURL() string {
	if rand.Float64() < 0.6 {
		num := rand.IntN(900000000) + 100000000
		return fmt.Sprintf("/?%d.html", num)
	}
	daysAgo := rand.IntN(30)
	date := time.Now().AddDate(0, 0, -daysAgo)
	dateStr := date.Format("20060102")
	num := rand.IntN(90000) + 10000
	return fmt.Sprintf("/?%s/%d.html", dateStr, num)
}

// ========== 统计 ==========

// GetStats returns statistics about loaded data
func (m *TemplateFuncsManager) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"keyword_groups": m.GetKeywordStats(),
		"image_groups":   m.GetImageStats(),
	}

	if m.clsPool != nil {
		stats["cls_pool"] = m.clsPool.Stats()
	}
	if m.urlPool != nil {
		stats["url_pool"] = m.urlPool.Stats()
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

// BuildArticleContentFromSingle builds article content from single title and content
func BuildArticleContentFromSingle(title, content string) string {
	if title == "" && content == "" {
		return ""
	}
	if title == "" {
		return content
	}
	if content == "" {
		return title
	}
	return fmt.Sprintf("<h2>%s</h2>\n%s", title, content)
}

// ========== 池管理方法 ==========

// ReloadPools 根据 CachePoolConfig 重载所有对象池配置（即时生效）
func (m *TemplateFuncsManager) ReloadPools(config *CachePoolConfig) {
	if m.clsPool != nil {
		m.clsPool.UpdateConfig(
			config.ClsPoolSize,
			config.ClsThreshold,
			config.ClsWorkers,
			config.ClsRefillInterval(),
		)
	}

	if m.urlPool != nil {
		m.urlPool.UpdateConfig(
			config.UrlPoolSize,
			config.UrlThreshold,
			config.UrlWorkers,
			config.UrlRefillInterval(),
		)
	}

	log.Info().
		Int("cls_size", config.ClsPoolSize).
		Int("url_size", config.UrlPoolSize).
		Msg("TemplateFuncsManager pools reloaded")
}

// ResizePools 根据配置调整所有池大小
func (m *TemplateFuncsManager) ResizePools(config *PoolSizeConfig) {
	if config.ClsPoolSize > 0 && m.clsPool != nil {
		m.clsPool.Resize(config.ClsPoolSize)
	}
	if config.URLPoolSize > 0 && m.urlPool != nil {
		m.urlPool.Resize(config.URLPoolSize)
	}

	log.Info().
		Int("cls", config.ClsPoolSize).
		Int("url", config.URLPoolSize).
		Msg("Pools resized")
}

// ClearPools 清空所有池
func (m *TemplateFuncsManager) ClearPools() {
	if m.clsPool != nil {
		m.clsPool.Clear()
	}
	if m.urlPool != nil {
		m.urlPool.Clear()
	}
	log.Info().Msg("All pools cleared")
}

// GetPoolStats 获取所有池的统计信息
func (m *TemplateFuncsManager) GetPoolStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if m.clsPool != nil {
		stats["cls"] = m.clsPool.Stats()
	}
	if m.urlPool != nil {
		stats["url"] = m.urlPool.Stats()
	}

	return stats
}

// ============ 图片分组管理方法 ============

// LoadImageGroup 加载指定分组的图片（初始化时使用）
func (m *TemplateFuncsManager) LoadImageGroup(groupID int, urls []string) {
	for {
		old := m.imageData.Load()

		var newGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
		}

		// 复制 urls 避免外部修改
		copied := make([]string, len(urls))
		copy(copied, urls)
		newGroups[groupID] = copied

		newData := &ImageData{groups: newGroups}
		if m.imageData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// AppendImages 追加图片到指定分组（添加图片时使用）
func (m *TemplateFuncsManager) AppendImages(groupID int, urls []string) {
	if len(urls) == 0 {
		return
	}

	for {
		old := m.imageData.Load()

		var newGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
		}

		// 追加到目标分组（显式复制避免并发问题）
		oldUrls := newGroups[groupID]
		newUrls := make([]string, len(oldUrls)+len(urls))
		copy(newUrls, oldUrls)
		copy(newUrls[len(oldUrls):], urls)
		newGroups[groupID] = newUrls

		newData := &ImageData{groups: newGroups}
		if m.imageData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// ReloadImageGroup 重载指定分组（删除后异步调用）
func (m *TemplateFuncsManager) ReloadImageGroup(groupID int, urls []string) {
	for {
		old := m.imageData.Load()

		var newGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups))
			for k, v := range old.groups {
				newGroups[k] = v
			}
		}

		// 复制 urls 避免外部修改
		if len(urls) > 0 {
			copied := make([]string, len(urls))
			copy(copied, urls)
			newGroups[groupID] = copied
		} else {
			delete(newGroups, groupID)
		}

		newData := &ImageData{groups: newGroups}
		if m.imageData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// GetImageStats 获取图片统计信息
func (m *TemplateFuncsManager) GetImageStats() map[int]int {
	data := m.imageData.Load()
	if data == nil {
		return make(map[int]int)
	}

	stats := make(map[int]int, len(data.groups))
	for gid, urls := range data.groups {
		stats[gid] = len(urls)
	}
	return stats
}

// ============ 关键词分组管理方法 ============

// LoadKeywordGroup 加载指定分组的关键词（初始化时使用）
func (m *TemplateFuncsManager) LoadKeywordGroup(groupID int, keywords, rawKeywords []string) {
	for {
		old := m.keywordData.Load()

		var newGroups map[int][]string
		var newRawGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
			newRawGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			newRawGroups = make(map[int][]string, len(old.rawGroups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
			for k, v := range old.rawGroups {
				newRawGroups[k] = v
			}
		}

		// 复制数据避免外部修改
		copiedKeywords := make([]string, len(keywords))
		copy(copiedKeywords, keywords)
		newGroups[groupID] = copiedKeywords

		copiedRaw := make([]string, len(rawKeywords))
		copy(copiedRaw, rawKeywords)
		newRawGroups[groupID] = copiedRaw

		newData := &KeywordData{groups: newGroups, rawGroups: newRawGroups}
		if m.keywordData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// AppendKeywords 追加关键词到指定分组（添加关键词时使用）
func (m *TemplateFuncsManager) AppendKeywords(groupID int, keywords, rawKeywords []string) {
	if len(keywords) == 0 {
		return
	}

	for {
		old := m.keywordData.Load()

		var newGroups map[int][]string
		var newRawGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
			newRawGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups)+1)
			newRawGroups = make(map[int][]string, len(old.rawGroups)+1)
			for k, v := range old.groups {
				newGroups[k] = v
			}
			for k, v := range old.rawGroups {
				newRawGroups[k] = v
			}
		}

		// 追加到目标分组（显式复制避免并发问题）
		oldKeywords := newGroups[groupID]
		newKeywords := make([]string, len(oldKeywords)+len(keywords))
		copy(newKeywords, oldKeywords)
		copy(newKeywords[len(oldKeywords):], keywords)
		newGroups[groupID] = newKeywords

		oldRaw := newRawGroups[groupID]
		newRaw := make([]string, len(oldRaw)+len(rawKeywords))
		copy(newRaw, oldRaw)
		copy(newRaw[len(oldRaw):], rawKeywords)
		newRawGroups[groupID] = newRaw

		newData := &KeywordData{groups: newGroups, rawGroups: newRawGroups}
		if m.keywordData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// ReloadKeywordGroup 重载指定分组（删除后异步调用）
func (m *TemplateFuncsManager) ReloadKeywordGroup(groupID int, keywords, rawKeywords []string) {
	for {
		old := m.keywordData.Load()

		var newGroups map[int][]string
		var newRawGroups map[int][]string
		if old == nil {
			newGroups = make(map[int][]string)
			newRawGroups = make(map[int][]string)
		} else {
			newGroups = make(map[int][]string, len(old.groups))
			newRawGroups = make(map[int][]string, len(old.rawGroups))
			for k, v := range old.groups {
				newGroups[k] = v
			}
			for k, v := range old.rawGroups {
				newRawGroups[k] = v
			}
		}

		// 替换或删除分组
		if len(keywords) > 0 {
			copiedKeywords := make([]string, len(keywords))
			copy(copiedKeywords, keywords)
			newGroups[groupID] = copiedKeywords

			copiedRaw := make([]string, len(rawKeywords))
			copy(copiedRaw, rawKeywords)
			newRawGroups[groupID] = copiedRaw
		} else {
			delete(newGroups, groupID)
			delete(newRawGroups, groupID)
		}

		newData := &KeywordData{groups: newGroups, rawGroups: newRawGroups}
		if m.keywordData.CompareAndSwap(old, newData) {
			return
		}
	}
}

// GetKeywordStats 获取关键词统计信息
func (m *TemplateFuncsManager) GetKeywordStats() map[int]int {
	data := m.keywordData.Load()
	if data == nil {
		return make(map[int]int)
	}

	stats := make(map[int]int, len(data.groups))
	for gid, keywords := range data.groups {
		stats[gid] = len(keywords)
	}
	return stats
}
