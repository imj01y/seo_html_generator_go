package pool

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"seo-generator/api/internal/repository"
)

// ImagePool 图片池实现
// 使用 Repository 层获取数据,支持多分组管理
type ImagePool struct {
	repo repository.ImageRepository

	// 数据存储
	data        map[int][]string // groupID -> image URLs
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

// NewImagePool 创建新的图片池
func NewImagePool(db *sqlx.DB) *ImagePool {
	ctx, cancel := context.WithCancel(context.Background())
	return &ImagePool{
		repo:   repository.NewImageRepository(db),
		data:   make(map[int][]string),
		ctx:    ctx,
		cancel: cancel,
		hits:   0,
		misses: 0,
	}
}

// Start 启动图片池
func (p *ImagePool) Start(ctx context.Context) error {
	log.Info().Msg("Starting image pool")

	// 发现所有分组
	groups, err := p.discoverGroups(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to discover image groups, using default")
		groups = []int{1}
	}

	// 预加载数据
	for _, groupID := range groups {
		if err := p.loadGroup(ctx, groupID); err != nil {
			log.Warn().Err(err).Int("group_id", groupID).Msg("Failed to load image group")
		}
	}

	log.Info().
		Int("groups", len(groups)).
		Int64("memory_bytes", p.memoryBytes).
		Msg("Image pool started")

	return nil
}

// Stop 停止图片池
func (p *ImagePool) Stop() error {
	log.Info().Msg("Stopping image pool")
	p.cancel()
	p.wg.Wait()
	return nil
}

// Pop 获取一个图片URL(随机)
func (p *ImagePool) Pop(groupID int) (string, error) {
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
func (p *ImagePool) GetStats(groupID int) PoolStats {
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
func (p *ImagePool) Reload(ctx context.Context, groupIDs []int) error {
	for _, gid := range groupIDs {
		if err := p.loadGroup(ctx, gid); err != nil {
			return fmt.Errorf("reload group %d: %w", gid, err)
		}
	}
	return nil
}

// RefillIfNeeded 检查并补充(图片池不需要补充,始终复用)
func (p *ImagePool) RefillIfNeeded(ctx context.Context, groupID int) error {
	// Images are reusable, no refill needed
	return nil
}

// discoverGroups 发现所有图片分组
func (p *ImagePool) discoverGroups(ctx context.Context) ([]int, error) {
	// 使用 Repository 层的 List 方法
	status := 1
	filter := repository.ImageFilter{
		Status: &status,
	}

	images, _, err := p.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// 提取唯一的分组ID
	groupMap := make(map[int]bool)
	for _, img := range images {
		groupMap[img.GroupID] = true
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

// loadGroup 加载指定分组的图片
func (p *ImagePool) loadGroup(ctx context.Context, groupID int) error {
	// 使用 Repository 获取数据
	status := 1
	filter := repository.ImageFilter{
		GroupID: &groupID,
		Status:  &status,
	}

	images, _, err := p.repo.List(ctx, filter)
	if err != nil {
		return fmt.Errorf("load images: %w", err)
	}

	if len(images) == 0 {
		log.Warn().Int("group_id", groupID).Msg("No images found for group")
		return nil
	}

	// 提取 URL
	urls := make([]string, len(images))
	for i, img := range images {
		urls[i] = img.URL
	}

	p.mu.Lock()
	// 计算旧数据内存
	oldMem := SliceMemorySize(p.data[groupID])
	// 更新数据
	p.data[groupID] = urls
	// 计算新数据内存
	newMem := SliceMemorySize(urls)
	// 更新内存计数
	p.memoryBytes += newMem - oldMem
	p.mu.Unlock()

	log.Info().
		Int("group_id", groupID).
		Int("count", len(urls)).
		Int64("memory_bytes", newMem).
		Msg("Images loaded for group")

	return nil
}

// GetRandomImage 返回随机图片URL
func (p *ImagePool) GetRandomImage(groupID int) string {
	p.mu.RLock()
	items := p.data[groupID]
	if len(items) == 0 {
		items = p.data[1]
	}
	p.mu.RUnlock()

	if len(items) == 0 {
		return ""
	}
	return items[rand.IntN(len(items))]
}

// GetImages 返回指定分组的所有图片URL
func (p *ImagePool) GetImages(groupID int) []string {
	p.mu.RLock()
	urls := p.data[groupID]
	p.mu.RUnlock()

	if len(urls) == 0 {
		return nil
	}

	result := make([]string, len(urls))
	copy(result, urls)
	return result
}

// AppendImages 追加图片到内存(新增时调用)
func (p *ImagePool) AppendImages(groupID int, urls []string) {
	if len(urls) == 0 {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.data[groupID] == nil {
		p.data[groupID] = []string{}
	}
	p.data[groupID] = append(p.data[groupID], urls...)

	// 增加内存计数
	addedMem := SliceMemorySize(urls)
	p.memoryBytes += addedMem

	log.Debug().Int("group_id", groupID).Int("added", len(urls)).Msg("Images appended to pool")
}

// ReloadGroup 重载指定分组的图片缓存(删除时调用)
func (p *ImagePool) ReloadGroup(ctx context.Context, groupID int) error {
	return p.loadGroup(ctx, groupID)
}

// GetTotalCount 获取所有图片总数
func (p *ImagePool) GetTotalCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := 0
	for _, items := range p.data {
		total += len(items)
	}
	return total
}

// GetGroupCount 获取指定分组的图片数量
func (p *ImagePool) GetGroupCount(groupID int) int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.data[groupID])
}

// GetAllGroups 获取所有分组信息
func (p *ImagePool) GetAllGroups() map[int]int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	groups := make(map[int]int)
	for gid, items := range p.data {
		groups[gid] = len(items)
	}
	return groups
}
