// api/internal/service/title_generator.go
package core

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// TitlePool 标题池（基于 channel）
type TitlePool struct {
	ch            chan string
	groupID       int
	memoryBytes   atomic.Int64 // 内存占用追踪
	consumedCount atomic.Int64 // 被消费的数量（Pop 计数）
}

// TitleGenerator 动态标题生成器
type TitleGenerator struct {
	pools       map[int]*TitlePool // groupID -> 标题池
	poolManager *PoolManager       // 引用，获取关键词和emoji
	config      *CachePoolConfig
	mu          sync.RWMutex

	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	stopped atomic.Bool
}

// NewTitleGenerator 创建标题生成器
func NewTitleGenerator(pm *PoolManager, config *CachePoolConfig) *TitleGenerator {
	ctx, cancel := context.WithCancel(context.Background())
	return &TitleGenerator{
		pools:       make(map[int]*TitlePool),
		poolManager: pm,
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// generateTitle 生成单个标题
// 格式：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3
func (g *TitleGenerator) generateTitle(groupID int) string {
	// 获取 3 个随机编码关键词
	keywords := g.poolManager.GetRandomKeywords(groupID, 3)
	if len(keywords) < 3 {
		// 关键词不足，返回空或部分拼接
		if len(keywords) == 0 {
			return ""
		}
		// 尽可能拼接
		result := keywords[0]
		if len(keywords) > 1 {
			result += g.poolManager.GetRandomEmoji() + keywords[1]
		}
		return result
	}

	// 获取 2 个不重复的 emoji
	emoji1 := g.poolManager.GetRandomEmoji()
	emoji2 := g.poolManager.GetRandomEmojiExclude(map[string]bool{emoji1: true})

	// 拼接：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3
	return keywords[0] + emoji1 + keywords[1] + emoji2 + keywords[2]
}

// getOrCreatePool 获取或创建指定 groupID 的标题池
func (g *TitleGenerator) getOrCreatePool(groupID int) *TitlePool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if pool, exists := g.pools[groupID]; exists {
		return pool
	}

	pool := &TitlePool{
		ch:      make(chan string, g.config.TitlePoolSize),
		groupID: groupID,
	}
	g.pools[groupID] = pool
	log.Debug().Int("group_id", groupID).Int("size", g.config.TitlePoolSize).Msg("Created title pool")
	return pool
}

// Pop 从标题池获取一个标题
func (g *TitleGenerator) Pop(groupID int) (string, error) {
	pool := g.getOrCreatePool(groupID)

	select {
	case title := <-pool.ch:
		// 减少内存计数
		pool.memoryBytes.Add(-StringMemorySize(title))
		// 增加消费计数
		pool.consumedCount.Add(1)
		return title, nil
	default:
		// 池空，同步生成一个返回（也计入消费）
		pool.consumedCount.Add(1)
		return g.generateTitle(groupID), nil
	}
}

// fillPool 填充标题池
func (g *TitleGenerator) fillPool(groupID int, pool *TitlePool) {
	need := g.config.TitlePoolSize - len(pool.ch)
	if need <= 0 {
		return
	}

	filled := 0
	var addedMem int64
loop:
	for i := 0; i < need; i++ {
		title := g.generateTitle(groupID)
		if title == "" {
			// 关键词池为空，无法生成标题，退出循环避免 CPU 空转
			break
		}
		select {
		case pool.ch <- title:
			filled++
			addedMem += StringMemorySize(title)
		default:
			// 池满，停止
			break loop
		}
	}

	// 增加内存计数
	if addedMem > 0 {
		pool.memoryBytes.Add(addedMem)
	}

	if filled > 0 {
		log.Debug().
			Int("group_id", groupID).
			Int("filled", filled).
			Int("total", len(pool.ch)).
			Msg("Title pool filled")
	}
}

// refillWorker 后台填充协程
func (g *TitleGenerator) refillWorker(groupID int, pool *TitlePool) {
	defer g.wg.Done()

	// 启动后立即尝试一次填充（不等 ticker）
	if !g.stopped.Load() && len(g.poolManager.GetRandomKeywords(groupID, 1)) > 0 {
		g.fillPool(groupID, pool)
	}

	ticker := time.NewTicker(g.config.TitleRefillInterval())
	defer ticker.Stop()

	for {
		select {
		case <-g.ctx.Done():
			return
		case <-ticker.C:
			if g.stopped.Load() {
				return
			}
			// 预检查：关键词池为空时跳过填充，避免无效循环
			if len(g.poolManager.GetRandomKeywords(groupID, 1)) == 0 {
				continue
			}
			// 检查是否需要补充（低于阈值比例时触发）
			thresholdCount := int(float64(g.config.TitlePoolSize) * g.config.TitleThreshold)
			if len(pool.ch) < thresholdCount {
				g.fillPool(groupID, pool)
			}
		}
	}
}

// Start 启动标题生成器
func (g *TitleGenerator) Start(groupIDs []int) {
	log.Info().
		Ints("group_ids", groupIDs).
		Int("pool_size", g.config.TitlePoolSize).
		Int("workers", g.config.TitleWorkers).
		Msg("Starting TitleGenerator")

	for _, groupID := range groupIDs {
		pool := g.getOrCreatePool(groupID)

		// 不做同步初始填充，由 refillWorker 异步完成
		// Pop 方法有降级逻辑：池空时同步生成一条返回

		// 启动 N 个填充协程
		for i := 0; i < g.config.TitleWorkers; i++ {
			g.wg.Add(1)
			go g.refillWorker(groupID, pool)
		}
	}
}

// Stop 停止标题生成器
func (g *TitleGenerator) Stop() {
	g.stopped.Store(true)
	g.cancel()
	g.wg.Wait()
	log.Info().Msg("TitleGenerator stopped")
}

// Reload 重载配置
func (g *TitleGenerator) Reload(config *CachePoolConfig) {
	g.mu.Lock()
	oldConfig := g.config
	g.config = config
	needRestart := config.TitlePoolSize != oldConfig.TitlePoolSize
	g.mu.Unlock()

	if needRestart {
		// 从关键词池获取最新分组（而非复用旧分组）
		groupIDs := g.poolManager.GetKeywordGroupIDs()
		if len(groupIDs) == 0 {
			groupIDs = []int{1}
		}

		log.Info().
			Int("old_size", oldConfig.TitlePoolSize).
			Int("new_size", config.TitlePoolSize).
			Ints("group_ids", groupIDs).
			Msg("Title pool size changed, restarting workers")

		// 1. 停止旧 worker
		g.stopped.Store(true)
		g.cancel()
		g.wg.Wait()

		// 2. 重置状态
		g.stopped.Store(false)
		g.ctx, g.cancel = context.WithCancel(context.Background())

		// 3. 清空旧池
		g.mu.Lock()
		g.pools = make(map[int]*TitlePool)
		g.mu.Unlock()

		// 4. 重新启动（使用最新分组）
		g.Start(groupIDs)
	}
}

// ForceReload 强制重载所有标题池（不依赖配置变化）
func (g *TitleGenerator) ForceReload() {
	groupIDs := g.poolManager.GetKeywordGroupIDs()
	if len(groupIDs) == 0 {
		groupIDs = []int{1}
	}

	log.Info().Ints("group_ids", groupIDs).Msg("TitleGenerator: force reloading all pools")

	// 1. 停止旧 worker
	g.stopped.Store(true)
	g.cancel()
	g.wg.Wait()

	// 2. 重置状态
	g.stopped.Store(false)
	g.ctx, g.cancel = context.WithCancel(context.Background())

	// 3. 清空旧池
	g.mu.Lock()
	g.pools = make(map[int]*TitlePool)
	g.mu.Unlock()

	// 4. 重新启动
	g.Start(groupIDs)
}

// ReloadGroup 重载指定分组的标题池（清空并重新填充，不重启 worker）
func (g *TitleGenerator) ReloadGroup(groupID int) {
	if g.stopped.Load() {
		return
	}

	pool := g.getOrCreatePool(groupID)

	// 排空 channel 中的旧数据
	drained := 0
	var drainedMem int64
	for {
		select {
		case title := <-pool.ch:
			drained++
			drainedMem += StringMemorySize(title)
		default:
			goto done
		}
	}
done:
	if drainedMem > 0 {
		pool.memoryBytes.Add(-drainedMem)
	}

	// 重新填充
	g.fillPool(groupID, pool)

	log.Info().Int("group_id", groupID).Int("drained", drained).Msg("TitleGenerator: reloaded group")
}

// SyncGroups 同步分组：为新增的关键词分组创建标题池和 worker
func (g *TitleGenerator) SyncGroups(groupIDs []int) {
	if g.stopped.Load() {
		return
	}

	g.mu.RLock()
	// 找出需要新增的分组
	var toAdd []int
	for _, gid := range groupIDs {
		if _, exists := g.pools[gid]; !exists {
			toAdd = append(toAdd, gid)
		}
	}
	g.mu.RUnlock()

	// 为新分组创建池和启动 worker
	for _, gid := range toAdd {
		pool := g.getOrCreatePool(gid)
		g.fillPool(gid, pool)
		for i := 0; i < g.config.TitleWorkers; i++ {
			g.wg.Add(1)
			go g.refillWorker(gid, pool)
		}
		log.Info().Int("group_id", gid).Msg("TitleGenerator: added new group")
	}
}

// GetStats 获取标题池统计
func (g *TitleGenerator) GetStats() map[int]map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := make(map[int]map[string]int)
	for groupID, pool := range g.pools {
		thresholdCount := int(float64(g.config.TitlePoolSize) * g.config.TitleThreshold)
		stats[groupID] = map[string]int{
			"current":   len(pool.ch),
			"max_size":  g.config.TitlePoolSize,
			"threshold": thresholdCount,
		}
	}
	return stats
}

// GetTotalStats 获取汇总统计
func (g *TitleGenerator) GetTotalStats() (current, maxSize int, memoryBytes, consumedCount int64) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, pool := range g.pools {
		current += len(pool.ch)
		maxSize += g.config.TitlePoolSize
		memoryBytes += pool.memoryBytes.Load()
		consumedCount += pool.consumedCount.Load()
	}
	return
}

// GetGroupStats 获取按分组的统计信息（用于前端分组详情展示）
func (g *TitleGenerator) GetGroupStats() []PoolGroupInfo {
	g.mu.RLock()
	defer g.mu.RUnlock()

	groups := make([]PoolGroupInfo, 0, len(g.pools))
	for gid, pool := range g.pools {
		current := len(pool.ch)
		maxSize := g.config.TitlePoolSize
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
