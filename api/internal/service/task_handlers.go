// Package core provides task handler implementations
package core

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// RefreshDataHandler 刷新数据池处理器
type RefreshDataHandler struct {
	dataManager *DataManager
}

// NewRefreshDataHandler 创建刷新数据池处理器
func NewRefreshDataHandler(manager *DataManager) *RefreshDataHandler {
	return &RefreshDataHandler{
		dataManager: manager,
	}
}

// TaskType 返回任务类型
func (h *RefreshDataHandler) TaskType() TaskType {
	return TaskTypeRefreshData
}

// Handle 执行刷新数据池任务
func (h *RefreshDataHandler) Handle(task *ScheduledTask) TaskResult {
	startTime := time.Now()

	params, err := ParseRefreshDataParams(task.Params)
	if err != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("parse params failed: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.Info().
		Str("pool_name", params.PoolName).
		Int("site_id", params.SiteID).
		Msg("Refreshing data pool")

	// 使用分组 ID（默认为 1）
	groupID := 1
	if params.SiteID > 0 {
		groupID = params.SiteID // 如果指定了 site_id，用作 group_id
	}

	refreshErr := h.dataManager.Refresh(ctx, groupID, params.PoolName)

	if refreshErr != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("refresh failed: %v", refreshErr),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	stats := h.dataManager.GetPoolStats()
	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("refreshed %s, keywords=%d, images=%d, titles=%d, contents=%d", params.PoolName, stats.Keywords, stats.Images, stats.Titles, stats.Contents),
		Duration: time.Since(startTime).Milliseconds(),
	}
}

// RefreshTemplateHandler 刷新模板缓存处理器
type RefreshTemplateHandler struct {
	templateCache *TemplateCache
}

// NewRefreshTemplateHandler 创建刷新模板缓存处理器
func NewRefreshTemplateHandler(cache *TemplateCache) *RefreshTemplateHandler {
	return &RefreshTemplateHandler{
		templateCache: cache,
	}
}

// TaskType 返回任务类型
func (h *RefreshTemplateHandler) TaskType() TaskType {
	return TaskTypeRefreshTemplate
}

// Handle 执行刷新模板缓存任务
func (h *RefreshTemplateHandler) Handle(task *ScheduledTask) TaskResult {
	startTime := time.Now()

	params, err := ParseRefreshTemplateParams(task.Params)
	if err != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("parse params failed: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	log.Info().
		Str("template_name", params.TemplateName).
		Int("site_group_id", params.SiteGroupID).
		Msg("Refreshing template cache")

	var refreshErr error
	if params.TemplateName != "" {
		if params.SiteGroupID > 0 {
			// 刷新指定模板和站点组
			refreshErr = h.templateCache.Reload(ctx, params.TemplateName, params.SiteGroupID)
		} else {
			// 刷新指定模板所有版本
			refreshErr = h.templateCache.ReloadByName(ctx, params.TemplateName)
		}
	} else {
		// 刷新所有模板
		refreshErr = h.templateCache.LoadAll(ctx)
	}

	if refreshErr != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("refresh failed: %v", refreshErr),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	stats := h.templateCache.GetStats()
	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("templates refreshed, count=%v", stats["item_count"]),
		Duration: time.Since(startTime).Milliseconds(),
	}
}

// ClearCacheHandler 清理缓存处理器
type ClearCacheHandler struct {
	htmlCache *HTMLCache
	siteCache *SiteCache
}

// NewClearCacheHandler 创建清理缓存处理器
func NewClearCacheHandler(htmlCache *HTMLCache, siteCache *SiteCache) *ClearCacheHandler {
	return &ClearCacheHandler{
		htmlCache: htmlCache,
		siteCache: siteCache,
	}
}

// TaskType 返回任务类型
func (h *ClearCacheHandler) TaskType() TaskType {
	return TaskTypeClearCache
}

// Handle 执行清理缓存任务
func (h *ClearCacheHandler) Handle(task *ScheduledTask) TaskResult {
	startTime := time.Now()

	params, err := ParseClearCacheParams(task.Params)
	if err != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("parse params failed: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	log.Info().
		Str("cache_type", params.CacheType).
		Int64("max_age", params.MaxAge).
		Str("domain", params.Domain).
		Msg("Clearing cache")

	// TODO: 实现具体的缓存清理逻辑
	// 当前为占位实现
	var cleared int

	switch params.CacheType {
	case "html":
		if h.htmlCache != nil {
			// TODO: 实现 HTML 缓存清理
			log.Warn().Msg("HTML cache clear not implemented yet")
		}
	case "site":
		if h.siteCache != nil {
			// TODO: 实现站点缓存清理
			log.Warn().Msg("Site cache clear not implemented yet")
		}
	case "all":
		// TODO: 清理所有缓存
		log.Warn().Msg("All cache clear not implemented yet")
	default:
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("unknown cache type: %s", params.CacheType),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("cache cleared, type=%s, count=%d", params.CacheType, cleared),
		Duration: time.Since(startTime).Milliseconds(),
	}
}

// PushURLsHandler 推送URL处理器
type PushURLsHandler struct {
	// TODO: 添加需要的依赖
}

// NewPushURLsHandler 创建推送URL处理器
func NewPushURLsHandler() *PushURLsHandler {
	return &PushURLsHandler{}
}

// TaskType 返回任务类型
func (h *PushURLsHandler) TaskType() TaskType {
	return TaskTypePushURLs
}

// Handle 执行推送URL任务
func (h *PushURLsHandler) Handle(task *ScheduledTask) TaskResult {
	startTime := time.Now()

	params, err := ParsePushURLsParams(task.Params)
	if err != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("parse params failed: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	log.Info().
		Int("site_id", params.SiteID).
		Int("url_count", params.URLCount).
		Str("search_engine", params.SearchEngine).
		Msg("Pushing URLs")

	// TODO: 实现 URL 推送逻辑
	// 1. 从数据库获取待推送的 URL
	// 2. 根据搜索引擎调用对应的 API
	// 3. 记录推送结果

	log.Warn().Msg("URL push not implemented yet")

	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("URL push scheduled, engine=%s, count=%d (not implemented)", params.SearchEngine, params.URLCount),
		Duration: time.Since(startTime).Milliseconds(),
	}
}

// RegisterAllHandlers 注册所有任务处理器
func RegisterAllHandlers(scheduler *Scheduler, dataManager *DataManager, templateCache *TemplateCache, htmlCache *HTMLCache, siteCache *SiteCache) {
	// 注册刷新数据池处理器
	if dataManager != nil {
		scheduler.RegisterHandler(NewRefreshDataHandler(dataManager))
	}

	// 注册刷新模板缓存处理器
	if templateCache != nil {
		scheduler.RegisterHandler(NewRefreshTemplateHandler(templateCache))
	}

	// 注册清理缓存处理器
	scheduler.RegisterHandler(NewClearCacheHandler(htmlCache, siteCache))

	// 注册推送URL处理器
	scheduler.RegisterHandler(NewPushURLsHandler())

	log.Info().Msg("All task handlers registered")
}
