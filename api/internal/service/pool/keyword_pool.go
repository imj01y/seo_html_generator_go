package pool

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/repository"
)

var (
	// ErrEmptyPool 池为空错误
	ErrEmptyPool = errors.New("pool is empty")
)

// KeywordPool 关键词池实现
// 使用 Repository 层获取数据,支持多分组管理
type KeywordPool struct {
	repo repository.KeywordRepository

	// 数据存储
	data        map[int][]string // groupID -> encoded keywords
	rawData     map[int][]string // groupID -> raw keywords
	mu          sync.RWMutex
	memoryBytes int64 // 内存占用追踪

	// 统计
	hits   int64
	misses int64

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewKeywordPool 创建新的关键词池
func NewKeywordPool(db *sqlx.DB) *KeywordPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &KeywordPool{
		repo:    repository.NewKeywordRepository(db),
		data:    make(map[int][]string),
		rawData: make(map[int][]string),
		ctx:     ctx,
		cancel:  cancel,
		hits:    0,
		misses:  0,
	}
}

// Start 启动关键词池
func (p *KeywordPool) Start(ctx context.Context) error {
	log.Info().Msg("Starting keyword pool")

	// 发现所有分组
	groups, err := p.discoverGroups(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to discover keyword groups, using default")
		groups = []int{1}
	}

	// 预加载数据
	for _, groupID := range groups {
		if err := p.loadGroup(ctx, groupID); err != nil {
			log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load keyword group")
		}
	}

	log.Info().
		Int("groups", len(groups)).
		Int64("memory_bytes", p.memoryBytes).
		Msg("Keyword pool started")

	return nil
}

// Stop 停止关键词池
func (p *KeywordPool) Stop() error {
	log.Info().Msg("Stopping keyword pool")
	p.cancel()
	p.wg.Wait()
	return nil
}

// Pop 获取一个关键词(随机)
func (p *KeywordPool) Pop(groupID int) (string, error) {
	p.mu.RLock()
	items := p.data[groupID]
	if len(items) == 0 {
		// Fallback to default group
		items = p.data[1]
	}
	p.mu.RUnlock()

	if len(items) == 0 {
		p.misses++
		return "", ErrEmptyPool
	}

	p.hits++
	return items[rand.IntN(len(items))], nil
}

// GetStats 获取统计信息
func (p *KeywordPool) GetStats(groupID int) PoolStats {
	p.mu.RLock()
	items := p.data[groupID]
	count := len(items)
	p.mu.RUnlock()

	return PoolStats{
		Current:     count,
		Capacity:    count,
		GroupID:     groupID,
		CacheHits:   p.hits,
		CacheMisses: p.misses,
		MemoryBytes: p.memoryBytes,
	}
}

// Reload 重新加载指定分组
func (p *KeywordPool) Reload(ctx context.Context, groupIDs []int) error {
	for _, gid := range groupIDs {
		if err := p.loadGroup(ctx, gid); err != nil {
			return fmt.Errorf("reload group %d: %w", gid, err)
		}
	}
	return nil
}

// RefillIfNeeded 检查并补充(关键词池不需要补充,始终复用)
func (p *KeywordPool) RefillIfNeeded(ctx context.Context, groupID int) error {
	// Keywords are reusable, no refill needed
	return nil
}

// discoverGroups 发现所有关键词分组
func (p *KeywordPool) discoverGroups(ctx context.Context) ([]int, error) {
	// 使用 Repository 层的 List 方法
	status := 1
	filter := repository.KeywordFilter{
		Status: &status,
	}

	keywords, _, err := p.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// 提取唯一的分组ID
	groupMap := make(map[int]bool)
	for _, kw := range keywords {
		groupMap[kw.GroupID] = true
	}

	groups := make([]int, 0, len(groupMap))
	for gid := range groupMap {
		groups = append(groups, gid)
	}

	if len(groups) == 0 {
		return []int{1}, nil
	}

	return groups, nil
}

// loadGroup 加载指定分组的关键词
func (p *KeywordPool) loadGroup(ctx context.Context, groupID int) error {
	// 使用 Repository 获取数据
	status := 1
	filter := repository.KeywordFilter{
		GroupID: &groupID,
		Status:  &status,
	}

	keywords, _, err := p.repo.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("load keywords: %w", err)
	}

	if len(keywords) == 0 {
		log.Warn().Int("group_id", groupID).Msg("No keywords found for group")
		return nil
	}

	// Store raw keywords
	rawCopy := make([]string, len(keywords))
	for i, kw := range keywords {
		rawCopy[i] = kw.Keyword
	}

	// Pre-encode keywords
	encoded := make([]string, len(keywords))
	for i, kw := range keywords {
		encoded[i] = encodeText(kw.Keyword)
	}

	p.mu.Lock()
	// 计算旧数据内存
	oldMem := SliceMemorySize(p.data[groupID]) + SliceMemorySize(p.rawData[groupID])
	// 更新数据
	p.data[groupID] = encoded
	p.rawData[groupID] = rawCopy
	// 计算新数据内存
	newMem := SliceMemorySize(encoded) + SliceMemorySize(rawCopy)
	// 更新内存计数
	p.memoryBytes += newMem - oldMem
	p.mu.Unlock()

	log.Info().
		Int("group_id", groupID).
		Int("count", len(encoded)).
		Int64("memory_bytes", newMem).
		Msg("Keywords loaded for group")

	return nil
}

// GetRandomKeywords 返回随机关键词(已编码)
func (p *KeywordPool) GetRandomKeywords(groupID int, count int) []string {
	p.mu.RLock()
	items := p.data[groupID]
	if len(items) == 0 {
		items = p.data[1] // fallback to default group
	}
	p.mu.RUnlock()

	return getRandomItems(items, count)
}

// GetRawKeywords 返回原始关键词(未编码)
func (p *KeywordPool) GetRawKeywords(groupID int, count int) []string {
	p.mu.RLock()
	items := p.rawData[groupID]
	if len(items) == 0 {
		items = p.rawData[1]
	}
	p.mu.RUnlock()

	return getRandomItems(items, count)
}

// AppendKeywords 追加关键词到内存(新增时调用)
func (p *KeywordPool) AppendKeywords(groupID int, keywords []string) {
	if len(keywords) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.data[groupID] == nil {
		p.data[groupID] = []string{}
		p.rawData[groupID] = []string{}
	}

	// 追加原始关键词
	p.rawData[groupID] = append(p.rawData[groupID], keywords...)

	// 追加编码后的关键词并计算内存增量
	var addedMem int64
	for _, kw := range keywords {
		encoded := encodeText(kw)
		p.data[groupID] = append(p.data[groupID], encoded)
		addedMem += StringMemorySize(kw) + StringMemorySize(encoded)
	}
	p.memoryBytes += addedMem

	log.Debug().Int("group_id", groupID).Int("added", len(keywords)).Msg("Keywords appended to pool")
}

// ReloadGroup 重载指定分组的关键词缓存(删除时调用)
func (p *KeywordPool) ReloadGroup(ctx context.Context, groupID int) error {
	return p.loadGroup(ctx, groupID)
}

// GetTotalCount 获取所有关键词总数
func (p *KeywordPool) GetTotalCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, items := range p.data {
		total += len(items)
	}
	return total
}

// GetGroupCount 获取指定分组的关键词数量
func (p *KeywordPool) GetGroupCount(groupID int) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.data[groupID])
}

// GetAllGroups 获取所有分组信息
func (p *KeywordPool) GetAllGroups() map[int]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	groups := make(map[int]int)
	for gid, items := range p.data {
		groups[gid] = len(items)
	}
	return groups
}

// GetKeywords 返回指定分组的所有编码关键词
func (p *KeywordPool) GetKeywords(groupID int) []string {
	p.mu.RLock()
	items := p.data[groupID]
	if len(items) == 0 {
		items = p.data[1] // fallback to default group
	}
	// 复制避免外部修改
	result := make([]string, len(items))
	copy(result, items)
	p.mu.RUnlock()
	return result
}

// GetAllRawKeywords 返回指定分组的所有原始关键词
func (p *KeywordPool) GetAllRawKeywords(groupID int) []string {
	p.mu.RLock()
	items := p.rawData[groupID]
	if len(items) == 0 {
		items = p.rawData[1] // fallback to default group
	}
	// 复制避免外部修改
	result := make([]string, len(items))
	copy(result, items)
	p.mu.RUnlock()
	return result
}

// getRandomItems 从切片中随机选取指定数量的元素(Fisher-Yates 部分洗牌)
func getRandomItems(items []string, count int) []string {
	n := len(items)
	if n == 0 || count == 0 {
		return nil
	}
	if count > n {
		count = n
	}

	swapped := make(map[int]int, count)
	result := make([]string, count)

	for i := 0; i < count; i++ {
		j := i + rand.IntN(n-i)
		vi, oki := swapped[i]
		if !oki {
			vi = i
		}
		vj, okj := swapped[j]
		if !okj {
			vj = j
		}
		swapped[i] = vj
		swapped[j] = vi
		result[i] = items[vj]
	}
	return result
}
