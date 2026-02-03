package core

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
)

// EmojiData JSON 文件结构
type EmojiData struct {
	Emojis []string `json:"emojis"`
}

// EmojiManager 管理 Emoji 数据
type EmojiManager struct {
	emojis      []string
	mu          sync.RWMutex
	memoryBytes atomic.Int64 // 内存占用追踪
}

// NewEmojiManager 创建新的 Emoji 管理器
func NewEmojiManager() *EmojiManager {
	return &EmojiManager{emojis: []string{}}
}

// LoadFromFile 从 JSON 文件加载 Emoji 数据
func (m *EmojiManager) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var emojiData EmojiData
	if err := json.Unmarshal(data, &emojiData); err != nil {
		return err
	}

	// 计算内存占用
	var memSize int64
	for _, emoji := range emojiData.Emojis {
		memSize += StringMemorySize(emoji)
	}

	m.mu.Lock()
	m.emojis = emojiData.Emojis
	m.memoryBytes.Store(memSize)
	m.mu.Unlock()

	log.Info().Int("count", len(emojiData.Emojis)).Str("path", path).Msg("Emojis loaded")
	return nil
}

// GetRandom 获取随机 Emoji
func (m *EmojiManager) GetRandom() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.emojis) == 0 {
		return ""
	}
	return m.emojis[rand.IntN(len(m.emojis))]
}

// GetRandomExclude 获取不在 exclude 中的随机 Emoji
func (m *EmojiManager) GetRandomExclude(exclude map[string]bool) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	n := len(m.emojis)
	if n == 0 {
		return ""
	}

	// 排除列表为空时直接返回随机 emoji
	if len(exclude) == 0 {
		return m.emojis[rand.IntN(n)]
	}

	// 如果排除的数量超过总数的一半，构建可用列表更高效
	if len(exclude) > n/2 {
		available := make([]string, 0, n-len(exclude))
		for _, emoji := range m.emojis {
			if !exclude[emoji] {
				available = append(available, emoji)
			}
		}
		if len(available) == 0 {
			return m.emojis[rand.IntN(n)] // 回退到任意一个
		}
		return available[rand.IntN(len(available))]
	}

	// 排除列表较小时，随机尝试更高效
	// 尝试次数与排除列表大小相关，但最少 10 次
	maxAttempts := len(exclude)*3 + 10
	if maxAttempts > 100 {
		maxAttempts = 100
	}

	for i := 0; i < maxAttempts; i++ {
		emoji := m.emojis[rand.IntN(n)]
		if !exclude[emoji] {
			return emoji
		}
	}

	return m.emojis[rand.IntN(n)]
}

// Count 返回已加载的 Emoji 数量
func (m *EmojiManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.emojis)
}

// MemoryBytes 返回内存占用字节数
func (m *EmojiManager) MemoryBytes() int64 {
	return m.memoryBytes.Load()
}
