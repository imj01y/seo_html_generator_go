// api/internal/service/keyword_emoji_generator.go
package core

import (
	"context"
	"math/rand/v2"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// KeywordEmojiPool 关键词表情池（基于 channel）
type KeywordEmojiPool struct {
	ch            chan string
	groupID       int
	memoryBytes   atomic.Int64 // 内存占用追踪
	consumedCount atomic.Int64 // 被消费的数量（Pop 计数）
}

// KeywordEmojiGenerator 关键词表情生成器（对标 TitleGenerator）
type KeywordEmojiGenerator struct {
	pools        map[int]*KeywordEmojiPool // groupID -> 池
	poolManager  *PoolManager              // 引用，获取关键词和emoji
	config       *CachePoolConfig
	encoder      *HTMLEntityEncoder
	emojiManager *EmojiManager
	mu           sync.RWMutex

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped atomic.Bool
}

// NewKeywordEmojiGenerator 创建关键词表情生成器
func NewKeywordEmojiGenerator(pm *PoolManager, config *CachePoolConfig, encoder *HTMLEntityEncoder, emojiManager *EmojiManager) *KeywordEmojiGenerator {
	ctx, cancel := context.WithCancel(context.Background())
	return &KeywordEmojiGenerator{
		pools:        make(map[int]*KeywordEmojiPool),
		poolManager:  pm,
		config:       config,
		encoder:      encoder,
		emojiManager: emojiManager,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// generateKeywordEmoji 生成单个关键词表情组合
// 从原始关键词中随机插入 1-2 个 emoji，然后 HTML 编码
func (g *KeywordEmojiGenerator) generateKeywordEmoji(groupID int) string {
	// 获取该分组全部原始关键词
	rawKeywords := g.poolManager.GetAllRawKeywords(groupID)
	if len(rawKeywords) == 0 {
		return ""
	}

	keyword := rawKeywords[rand.IntN(len(rawKeywords))]

	// 复用 generateKeywordWithEmojiFromRaw 的逻辑
	if g.emojiManager == nil || g.encoder == nil {
		if g.encoder != nil {
			return g.encoder.EncodeText(keyword)
		}
		return keyword
	}

	// 随机决定插入 1 或 2 个 emoji（50% 概率）
	emojiCount := 1
	if rand.Float64() < 0.5 {
		emojiCount = 2
	}

	runes := []rune(keyword)
	runeLen := len(runes)
	if runeLen == 0 {
		return g.encoder.EncodeText(keyword)
	}

	exclude := make(map[string]bool)
	for i := 0; i < emojiCount; i++ {
		pos := rand.IntN(runeLen + 1)
		emoji := g.emojiManager.GetRandomExclude(exclude)
		if emoji != "" {
			exclude[emoji] = true
			newRunes := make([]rune, 0, len(runes)+len([]rune(emoji)))
			newRunes = append(newRunes, runes[:pos]...)
			newRunes = append(newRunes, []rune(emoji)...)
			newRunes = append(newRunes, runes[pos:]...)
			runes = newRunes
			runeLen = len(runes)
		}
	}

	return g.encoder.EncodeText(string(runes))
}

// getOrCreatePool 获取或创建指定 groupID 的池
func (g *KeywordEmojiGenerator) getOrCreatePool(groupID int) *KeywordEmojiPool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if pool, exists := g.pools[groupID]; exists {
		return pool
	}

	pool := &KeywordEmojiPool{
		ch:      make(chan string, g.config.KeywordEmojiPoolSize),
		groupID: groupID,
	}
	g.pools[groupID] = pool
	log.Debug().Int("group_id", groupID).Int("size", g.config.KeywordEmojiPoolSize).Msg("Created keyword emoji pool")
	return pool
}

// Pop 从池获取一个关键词表情组合
func (g *KeywordEmojiGenerator) Pop(groupID int) string {
	pool := g.getOrCreatePool(groupID)

	select {
	case item := <-pool.ch:
		pool.memoryBytes.Add(-StringMemorySize(item))
		pool.consumedCount.Add(1)
		return item
	default:
		// 池空，同步生成一个返回
		pool.consumedCount.Add(1)
		return g.generateKeywordEmoji(groupID)
	}
}

// fillPool 填充池
func (g *KeywordEmojiGenerator) fillPool(groupID int, pool *KeywordEmojiPool) {
	need := g.config.KeywordEmojiPoolSize - len(pool.ch)
	if need <= 0 {
		return
	}

	filled := 0
	var addedMem int64
loop:
	for i := 0; i < need; i++ {
		item := g.generateKeywordEmoji(groupID)
		if item == "" {
			break
		}
		select {
		case pool.ch <- item:
			filled++
			addedMem += StringMemorySize(item)
		default:
			break loop
		}
	}

	if addedMem > 0 {
		pool.memoryBytes.Add(addedMem)
	}

	if filled > 0 {
		log.Debug().
			Int("group_id", groupID).
			Int("filled", filled).
			Int("total", len(pool.ch)).
			Msg("Keyword emoji pool filled")
	}
}

// refillWorker 后台填充协程
func (g *KeywordEmojiGenerator) refillWorker(groupID int, pool *KeywordEmojiPool) {
	defer g.wg.Done()

	// 启动后立即尝试一次填充（不等 ticker）
	if !g.stopped.Load() && len(g.poolManager.GetAllRawKeywords(groupID)) > 0 {
		g.fillPool(groupID, pool)
	}

	ticker := time.NewTicker(g.config.KeywordEmojiRefillInterval())
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if g.stopped.Load() {
				return
			}
			// 预检查：关键词池为空时跳过
			if len(g.poolManager.GetAllRawKeywords(groupID)) == 0 {
				continue
			}
			// 低于阈值时触发补充
			thresholdCount := int(float64(g.config.KeywordEmojiPoolSize) * g.config.KeywordEmojiThreshold)
			if len(pool.ch) < thresholdCount {
				g.fillPool(groupID, pool)
			}
		}
	}
}

// Start 启动生成器
func (g *KeywordEmojiGenerator) Start(groupIDs []int) {
	log.Info().
		Ints("group_ids", groupIDs).
		Int("pool_size", g.config.KeywordEmojiPoolSize).
		Int("workers", g.config.KeywordEmojiWorkers).
		Msg("Starting KeywordEmojiGenerator")

	for _, groupID := range groupIDs {
		pool := g.getOrCreatePool(groupID)

		// 不做同步初始填充，由 refillWorker 异步完成
		// Pop 方法有降级逻辑：池空时同步生成一条返回

		// 启动 N 个填充协程
		for i := 0; i < g.config.KeywordEmojiWorkers; i++ {
			g.wg.Add(1)
			go g.refillWorker(groupID, pool)
		}
	}
}

// Stop 停止生成器
func (g *KeywordEmojiGenerator) Stop() {
	g.stopped.Store(true)
	g.cancel()
	g.wg.Wait()
	log.Info().Msg("KeywordEmojiGenerator stopped")
}

// Reload 重载配置
func (g *KeywordEmojiGenerator) Reload(config *CachePoolConfig) {
	g.mu.Lock()
	oldConfig := g.config
	g.config = config
	needRestart := config.KeywordEmojiPoolSize != oldConfig.KeywordEmojiPoolSize
	g.mu.Unlock()

	if needRestart {
		groupIDs := g.poolManager.GetKeywordGroupIDs()
		if len(groupIDs) == 0 {
			groupIDs = []int{1}
		}

		log.Info().
			Int("old_size", oldConfig.KeywordEmojiPoolSize).
			Int("new_size", config.KeywordEmojiPoolSize).
			Ints("group_ids", groupIDs).
			Msg("Keyword emoji pool size changed, restarting workers")

		g.stopped.Store(true)
		g.cancel()
		g.wg.Wait()

		g.stopped.Store(false)
		g.ctx, g.cancel = context.WithCancel(context.Background())

		g.mu.Lock()
		g.pools = make(map[int]*KeywordEmojiPool)
		g.mu.Unlock()

		g.Start(groupIDs)
	}
}

// ForceReload 强制重载所有分组
func (g *KeywordEmojiGenerator) ForceReload() {
	groupIDs := g.poolManager.GetKeywordGroupIDs()
	if len(groupIDs) == 0 {
		groupIDs = []int{1}
	}

	log.Info().Ints("group_ids", groupIDs).Msg("KeywordEmojiGenerator: force reloading all pools")

	g.stopped.Store(true)
	g.cancel()
	g.wg.Wait()

	g.stopped.Store(false)
	g.ctx, g.cancel = context.WithCancel(context.Background())

	g.mu.Lock()
	g.pools = make(map[int]*KeywordEmojiPool)
	g.mu.Unlock()

	g.Start(groupIDs)
}

// ReloadGroup 重载指定分组
func (g *KeywordEmojiGenerator) ReloadGroup(groupID int) {
	if g.stopped.Load() {
		return
	}

	pool := g.getOrCreatePool(groupID)

	// 排空旧数据
	drained := 0
	var drainedMem int64
	for {
		select {
		case item := <-pool.ch:
			drained++
			drainedMem += StringMemorySize(item)
		default:
			goto done
		}
	}
done:
	if drainedMem > 0 {
		pool.memoryBytes.Add(-drainedMem)
	}

	g.fillPool(groupID, pool)

	log.Info().Int("group_id", groupID).Int("drained", drained).Msg("KeywordEmojiGenerator: reloaded group")
}

// SyncGroups 同步分组：为新增的关键词分组创建池和 worker
func (g *KeywordEmojiGenerator) SyncGroups(groupIDs []int) {
	if g.stopped.Load() {
		return
	}

	g.mu.RLock()
	var toAdd []int
	for _, gid := range groupIDs {
		if _, exists := g.pools[gid]; !exists {
			toAdd = append(toAdd, gid)
		}
	}
	g.mu.RUnlock()

	for _, gid := range toAdd {
		pool := g.getOrCreatePool(gid)
		g.fillPool(gid, pool)
		for i := 0; i < g.config.KeywordEmojiWorkers; i++ {
			g.wg.Add(1)
			go g.refillWorker(gid, pool)
		}
		log.Info().Int("group_id", gid).Msg("KeywordEmojiGenerator: added new group")
	}
}

// GetTotalStats 获取汇总统计
func (g *KeywordEmojiGenerator) GetTotalStats() (current, maxSize int, memoryBytes, consumedCount int64) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, pool := range g.pools {
		current += len(pool.ch)
		maxSize += g.config.KeywordEmojiPoolSize
		memoryBytes += pool.memoryBytes.Load()
		consumedCount += pool.consumedCount.Load()
	}
	return
}

// GetGroupStats 获取分组详情统计
func (g *KeywordEmojiGenerator) GetGroupStats() []PoolGroupInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	groups := make([]PoolGroupInfo, 0, len(g.pools))
	for gid, pool := range g.pools {
		current := len(pool.ch)
		maxSize := g.config.KeywordEmojiPoolSize
		consumed := int(pool.consumedCount.Load())
		util := 0.0
		if maxSize > 0 {
			util = float64(current) / float64(maxSize) * 100
		}
		groups = append(groups, PoolGroupInfo{
			ID:          gid,
			Count:       current,
			Size:        maxSize,
			Available:   current,
			Used:        consumed,
			Utilization: util,
			MemoryBytes: pool.memoryBytes.Load(),
		})
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	return groups
}
