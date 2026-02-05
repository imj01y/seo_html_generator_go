// Package core provides task handler implementations
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	models "seo-generator/api/internal/model"
)

// RefreshDataHandler 刷新数据池处理器
type RefreshDataHandler struct {
	poolManager *PoolManager
}

// NewRefreshDataHandler 创建刷新数据池处理器
func NewRefreshDataHandler(manager *PoolManager) *RefreshDataHandler {
	return &RefreshDataHandler{
		poolManager: manager,
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

	refreshErr := h.poolManager.RefreshData(ctx, params.PoolName)

	if refreshErr != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("refresh failed: %v", refreshErr),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	stats := h.poolManager.GetPoolStatsSimple()
	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("refreshed %s, keywords=%d, images=%d", params.PoolName, stats.Keywords, stats.Images),
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

// RunSpiderHandler 运行爬虫处理器
type RunSpiderHandler struct {
	redis *redis.Client
	db    *sqlx.DB
}

// NewRunSpiderHandler 创建运行爬虫处理器
func NewRunSpiderHandler(rdb *redis.Client, db *sqlx.DB) *RunSpiderHandler {
	return &RunSpiderHandler{redis: rdb, db: db}
}

// TaskType 返回任务类型
func (h *RunSpiderHandler) TaskType() TaskType {
	return TaskTypeRunSpider
}

// Handle 执行运行爬虫任务
func (h *RunSpiderHandler) Handle(task *ScheduledTask) TaskResult {
	startTime := time.Now()
	ctx := context.Background()

	params, err := ParseRunSpiderParams(task.Params)
	if err != nil {
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("parse params failed: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	log.Info().
		Int("project_id", params.ProjectID).
		Str("project_name", params.ProjectName).
		Msg("Running scheduled spider")

	// 检查项目状态和是否启用
	var project struct {
		Status  string `db:"status"`
		Enabled int    `db:"enabled"`
	}
	if err := h.db.Get(&project, "SELECT status, enabled FROM spider_projects WHERE id = ?", params.ProjectID); err != nil {
		return TaskResult{
			Success:  false,
			Message:  "项目不存在",
			Duration: time.Since(startTime).Milliseconds(),
		}
	}
	if project.Enabled == 0 {
		return TaskResult{
			Success:  false,
			Message:  "项目已禁用，跳过",
			Duration: time.Since(startTime).Milliseconds(),
		}
	}
	if project.Status == "running" {
		return TaskResult{
			Success:  false,
			Message:  "项目正在运行中，跳过",
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	// 更新状态为 running
	h.db.Exec("UPDATE spider_projects SET status = 'running' WHERE id = ?", params.ProjectID)

	// 使用现有的 SpiderCommand 结构体
	cmd := models.SpiderCommand{
		Action:    "run",
		ProjectID: params.ProjectID,
		Timestamp: time.Now().Unix(),
	}
	cmdJSON, _ := json.Marshal(cmd)

	if err := h.redis.Publish(ctx, "spider:commands", cmdJSON).Err(); err != nil {
		// 回滚状态
		h.db.Exec("UPDATE spider_projects SET status = 'idle' WHERE id = ?", params.ProjectID)
		return TaskResult{
			Success:  false,
			Message:  fmt.Sprintf("发送命令失败: %v", err),
			Duration: time.Since(startTime).Milliseconds(),
		}
	}

	return TaskResult{
		Success:  true,
		Message:  fmt.Sprintf("已触发爬虫: %s (id=%d)", params.ProjectName, params.ProjectID),
		Duration: time.Since(startTime).Milliseconds(),
	}
}

// RegisterAllHandlers 注册所有任务处理器
func RegisterAllHandlers(scheduler *Scheduler, poolManager *PoolManager, templateCache *TemplateCache, db *sqlx.DB, rdb *redis.Client) {
	// 注册刷新数据池处理器
	if poolManager != nil {
		scheduler.RegisterHandler(NewRefreshDataHandler(poolManager))
	}

	// 注册刷新模板缓存处理器
	if templateCache != nil {
		scheduler.RegisterHandler(NewRefreshTemplateHandler(templateCache))
	}

	// 注册运行爬虫处理器
	if rdb != nil && db != nil {
		scheduler.RegisterHandler(NewRunSpiderHandler(rdb, db))
	}

	log.Info().Msg("All task handlers registered")
}
