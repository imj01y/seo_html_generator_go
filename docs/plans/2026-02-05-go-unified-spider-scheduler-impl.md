# Go 统一调度定时爬虫实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现 Go 统一调度定时爬虫功能，使前端配置的定时规则生效

**Architecture:** Go Scheduler 负责所有定时调度，通过 Redis pub/sub 触发 Python 执行爬虫，删除 Python 端冗余调度代码

**Tech Stack:** Go (robfig/cron, sqlx, redis), Python (删除 APScheduler 相关)

**Design Document:** `docs/plans/2026-02-05-go-unified-spider-scheduler-design.md`

---

## Task 1: 新增 TaskTypeRunSpider 和参数解析

**Files:**
- Modify: `api/internal/service/scheduler_types.go`

**Step 1: 添加任务类型常量和参数结构**

在 `scheduler_types.go` 文件末尾添加：

```go
// TaskTypeRunSpider 运行爬虫任务类型
const TaskTypeRunSpider TaskType = "run_spider"

// RunSpiderParams 运行爬虫参数
type RunSpiderParams struct {
	ProjectID   int    `json:"project_id"`
	ProjectName string `json:"project_name"`
}

// ParseRunSpiderParams 解析运行爬虫参数
func ParseRunSpiderParams(data json.RawMessage) (*RunSpiderParams, error) {
	var params RunSpiderParams
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	if params.ProjectID == 0 {
		return nil, fmt.Errorf("project_id is required")
	}
	return &params, nil
}
```

**Step 2: 添加缺失的 import**

确保文件顶部 import 包含 `"fmt"`（如果没有的话）。

**Step 3: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 4: Commit**

```bash
git add api/internal/service/scheduler_types.go
git commit -m "feat(scheduler): 添加 TaskTypeRunSpider 和参数解析"
```

---

## Task 2: 实现 RunSpiderHandler

**Files:**
- Modify: `api/internal/service/task_handlers.go`

**Step 1: 添加必要的 import**

在 `task_handlers.go` 的 import 块中添加：

```go
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
```

**Step 2: 添加 RunSpiderHandler 结构体和方法**

在 `RefreshTemplateHandler` 后面添加：

```go
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
```

**Step 3: 修改 RegisterAllHandlers 函数签名**

将原来的：

```go
func RegisterAllHandlers(scheduler *Scheduler, poolManager *PoolManager, templateCache *TemplateCache) {
```

修改为：

```go
func RegisterAllHandlers(scheduler *Scheduler, poolManager *PoolManager, templateCache *TemplateCache, db *sqlx.DB, rdb *redis.Client) {
```

并在函数末尾添加：

```go
	// 注册运行爬虫处理器
	if rdb != nil && db != nil {
		scheduler.RegisterHandler(NewRunSpiderHandler(rdb, db))
	}
```

**Step 4: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译失败（main.go 调用签名不匹配），这是预期的

**Step 5: Commit**

```bash
git add api/internal/service/task_handlers.go
git commit -m "feat(scheduler): 实现 RunSpiderHandler"
```

---

## Task 3: 更新 main.go 调用

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 修改 RegisterAllHandlers 调用**

找到 `core.RegisterAllHandlers(scheduler, poolManager, templateCache)` 这一行（约第 162 行），修改为：

```go
core.RegisterAllHandlers(scheduler, poolManager, templateCache, db, redisClient)
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add api/cmd/main.go
git commit -m "fix(main): 更新 RegisterAllHandlers 调用签名"
```

---

## Task 4: 创建 schedule_sync.go

**Files:**
- Create: `api/internal/service/schedule_sync.go`

**Step 1: 创建文件**

```go
// Package core provides schedule synchronization utilities
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// ScheduleConfig 前端 JSON 配置结构
type ScheduleConfig struct {
	Type     string `json:"type"`               // none, interval_minutes, interval_hours, daily, weekly, monthly
	Interval int    `json:"interval,omitempty"` // 间隔（分钟或小时）
	Time     string `json:"time,omitempty"`     // HH:mm 格式
	Days     []int  `json:"days,omitempty"`     // 周几 (0=周日, 1-6=周一到周六)
	Dates    []int  `json:"dates,omitempty"`    // 每月几号 (1-31)
}

// SyncSpiderSchedule 同步爬虫项目的定时配置到 scheduled_tasks 表
func SyncSpiderSchedule(ctx context.Context, db *sqlx.DB, scheduler *Scheduler, projectID int, projectName string, scheduleJSON *string, enabled int) error {
	// 查找已存在的任务
	var existingTaskID int64
	err := db.GetContext(ctx, &existingTaskID,
		`SELECT id FROM scheduled_tasks
		 WHERE task_type = 'run_spider'
		 AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.project_id')) = ?`,
		strconv.Itoa(projectID))

	taskExists := err == nil && existingTaskID > 0

	// 无配置或类型为 none，删除已有任务
	if scheduleJSON == nil || *scheduleJSON == "" {
		if taskExists {
			return scheduler.DeleteTask(ctx, existingTaskID)
		}
		return nil
	}

	var config ScheduleConfig
	if err := json.Unmarshal([]byte(*scheduleJSON), &config); err != nil {
		log.Warn().Err(err).Int("project_id", projectID).Msg("Invalid schedule JSON")
		return nil
	}

	if config.Type == "none" {
		if taskExists {
			return scheduler.DeleteTask(ctx, existingTaskID)
		}
		return nil
	}

	// 转换为 Cron 表达式
	cronExpr, err := ScheduleJSONToCron(config)
	if err != nil {
		log.Warn().Err(err).Int("project_id", projectID).Msg("Failed to convert schedule to cron")
		return nil
	}

	// 构建任务参数
	params, _ := json.Marshal(map[string]interface{}{
		"project_id":   projectID,
		"project_name": projectName,
	})

	task := &ScheduledTask{
		Name:     fmt.Sprintf("爬虫: %s", projectName),
		TaskType: TaskTypeRunSpider,
		CronExpr: cronExpr,
		Params:   params,
		Enabled:  enabled == 1,
	}

	if taskExists {
		task.ID = existingTaskID
		return scheduler.UpdateTask(ctx, task)
	}

	_, err = scheduler.CreateTask(ctx, task)
	return err
}

// ScheduleJSONToCron 将前端 JSON 配置转换为 Cron 表达式
// Cron 格式: 秒 分 时 日 月 周
func ScheduleJSONToCron(config ScheduleConfig) (string, error) {
	switch config.Type {
	case "interval_minutes":
		if config.Interval <= 0 {
			return "", fmt.Errorf("invalid interval: %d", config.Interval)
		}
		return fmt.Sprintf("0 */%d * * * *", config.Interval), nil

	case "interval_hours":
		if config.Interval <= 0 {
			return "", fmt.Errorf("invalid interval: %d", config.Interval)
		}
		return fmt.Sprintf("0 0 */%d * * *", config.Interval), nil

	case "daily":
		hour, minute, err := parseTime(config.Time)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("0 %d %d * * *", minute, hour), nil

	case "weekly":
		if len(config.Days) == 0 {
			return "", fmt.Errorf("no days specified for weekly schedule")
		}
		hour, minute, err := parseTime(config.Time)
		if err != nil {
			return "", err
		}
		days := intsToString(config.Days)
		return fmt.Sprintf("0 %d %d * * %s", minute, hour, days), nil

	case "monthly":
		if len(config.Dates) == 0 {
			return "", fmt.Errorf("no dates specified for monthly schedule")
		}
		hour, minute, err := parseTime(config.Time)
		if err != nil {
			return "", err
		}
		dates := intsToString(config.Dates)
		return fmt.Sprintf("0 %d %d %s * *", minute, hour, dates), nil

	default:
		return "", fmt.Errorf("unknown schedule type: %s", config.Type)
	}
}

// parseTime 解析 HH:mm 格式时间
func parseTime(timeStr string) (hour, minute int, err error) {
	if timeStr == "" {
		return 0, 0, fmt.Errorf("time is empty")
	}
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", timeStr)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return hour, minute, nil
}

// intsToString 将整数数组转换为逗号分隔的字符串
func intsToString(nums []int) string {
	strs := make([]string, len(nums))
	for i, n := range nums {
		strs[i] = strconv.Itoa(n)
	}
	return strings.Join(strs, ",")
}

// DeleteSpiderSchedule 删除爬虫项目的定时任务
func DeleteSpiderSchedule(ctx context.Context, db *sqlx.DB, scheduler *Scheduler, projectID int) error {
	var taskID int64
	err := db.GetContext(ctx, &taskID,
		`SELECT id FROM scheduled_tasks
		 WHERE task_type = 'run_spider'
		 AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.project_id')) = ?`,
		strconv.Itoa(projectID))

	if err != nil {
		return nil // 不存在则无需删除
	}

	return scheduler.DeleteTask(ctx, taskID)
}
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add api/internal/service/schedule_sync.go
git commit -m "feat(scheduler): 创建 schedule_sync.go 实现定时配置同步"
```

---

## Task 5: 修改 middleware.go 添加 scheduler 注入

**Files:**
- Modify: `api/internal/handler/middleware.go`

**Step 1: 添加 import**

确保 import 块包含：

```go
import (
	// ... 现有 imports ...
	core "seo-generator/api/internal/service"
)
```

**Step 2: 修改 DependencyInjectionMiddleware 函数**

找到 `DependencyInjectionMiddleware` 函数，修改签名和内容：

```go
// DependencyInjectionMiddleware 依赖注入中间件
// 将数据库、Redis 连接、配置和调度器注入到 Gin context 中，供 Handler 使用
func DependencyInjectionMiddleware(db *sqlx.DB, rdb *redis.Client, cfg *config.Config, scheduler *core.Scheduler) gin.HandlerFunc {
	return func(c *gin.Context) {
		if db != nil {
			c.Set("db", db)
		}
		if rdb != nil {
			c.Set("redis", rdb)
		}
		if cfg != nil {
			c.Set("config", cfg)
		}
		if scheduler != nil {
			c.Set("scheduler", scheduler)
		}
		c.Next()
	}
}
```

**Step 3: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译失败（router.go 调用签名不匹配），这是预期的

**Step 4: Commit**

```bash
git add api/internal/handler/middleware.go
git commit -m "feat(middleware): 添加 scheduler 依赖注入"
```

---

## Task 6: 修改 router.go 传递 scheduler

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 修改 DependencyInjectionMiddleware 调用**

找到 `DependencyInjectionMiddleware(deps.DB, deps.Redis, deps.Config)` 调用，修改为：

```go
DependencyInjectionMiddleware(deps.DB, deps.Redis, deps.Config, deps.Scheduler)
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "fix(router): 传递 scheduler 到中间件"
```

---

## Task 7: 修改 spider_projects.go 集成同步逻辑

**Files:**
- Modify: `api/internal/handler/spider_projects.go`

**Step 1: 添加必要的 import**

确保 import 块包含：

```go
import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	models "seo-generator/api/internal/model"
	core "seo-generator/api/internal/service"
)
```

**Step 2: 修改 Create 方法**

在 `Create` 方法中，找到 `c.JSON(200, gin.H{"success": true, "id": projectID, "message": "创建成功"})` 这一行之前，添加：

```go
	// 同步定时任务配置
	if scheduler, exists := c.Get("scheduler"); exists && req.Schedule != nil && *req.Schedule != "" {
		s := scheduler.(*core.Scheduler)
		ctx := context.Background()
		if err := core.SyncSpiderSchedule(ctx, sqlxDB, s, int(projectID), req.Name, req.Schedule, req.Enabled); err != nil {
			log.Warn().Err(err).Int64("project_id", projectID).Msg("Failed to sync spider schedule")
		}
	}
```

**Step 3: 修改 Update 方法**

在 `Update` 方法中，找到 `c.JSON(200, gin.H{"success": true, "message": "更新成功"})` 这一行之前，添加：

```go
	// 同步定时任务配置（如果 schedule 或 enabled 有变更）
	if scheduler, exists := c.Get("scheduler"); exists && (req.Schedule != nil || req.Enabled != nil) {
		s := scheduler.(*core.Scheduler)
		var project struct {
			Name     string  `db:"name"`
			Schedule *string `db:"schedule"`
			Enabled  int     `db:"enabled"`
		}
		if err := sqlxDB.Get(&project, "SELECT name, schedule, enabled FROM spider_projects WHERE id = ?", id); err == nil {
			ctx := context.Background()
			if syncErr := core.SyncSpiderSchedule(ctx, sqlxDB, s, id, project.Name, project.Schedule, project.Enabled); syncErr != nil {
				log.Warn().Err(syncErr).Int("project_id", id).Msg("Failed to sync spider schedule")
			}
		}
	}
```

**Step 4: 修改 Delete 方法**

在 `Delete` 方法中，找到 `sqlxDB.Exec("DELETE FROM spider_project_files WHERE project_id = ?", id)` 这一行之前，添加：

```go
	// 删除定时任务
	if scheduler, exists := c.Get("scheduler"); exists {
		s := scheduler.(*core.Scheduler)
		ctx := context.Background()
		if err := core.DeleteSpiderSchedule(ctx, sqlxDB, s, id); err != nil {
			log.Warn().Err(err).Int("project_id", id).Msg("Failed to delete spider schedule")
		}
	}
```

**Step 5: 修改 Toggle 方法**

在 `Toggle` 方法中，找到 `message := "已启用"` 这一行之前，添加：

```go
	// 同步定时任务状态
	if scheduler, exists := c.Get("scheduler"); exists {
		s := scheduler.(*core.Scheduler)
		var project struct {
			Name     string  `db:"name"`
			Schedule *string `db:"schedule"`
		}
		if err := sqlxDB.Get(&project, "SELECT name, schedule FROM spider_projects WHERE id = ?", id); err == nil {
			ctx := context.Background()
			if syncErr := core.SyncSpiderSchedule(ctx, sqlxDB, s, id, project.Name, project.Schedule, newEnabled); syncErr != nil {
				log.Warn().Err(syncErr).Int("project_id", id).Msg("Failed to sync spider schedule")
			}
		}
	}
```

**Step 6: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译成功

**Step 7: Commit**

```bash
git add api/internal/handler/spider_projects.go
git commit -m "feat(spider): 集成定时任务同步逻辑到 CRUD 操作"
```

---

## Task 8: 删除 Python 端 SpiderSchedulerWorker

**Files:**
- Delete: `content_worker/core/workers/spider_scheduler.py`
- Modify: `content_worker/core/initializers.py`
- Modify: `content_worker/core/lifecycle.py`

**Step 1: 删除 spider_scheduler.py**

```bash
rm content_worker/core/workers/spider_scheduler.py
```

**Step 2: 修改 initializers.py**

找到并删除以下代码：

1. 删除全局变量声明（约第 25 行）：
```python
_scheduler_worker = None
```

2. 删除 `init_background_workers` 函数中的爬虫调度器相关代码（约第 127、148-155 行）：
```python
    global _scheduler_worker
```

以及：
```python
    # 爬虫定时调度器
    try:
        from core.workers.spider_scheduler import SpiderSchedulerWorker
        _scheduler_worker = SpiderSchedulerWorker(db_pool=db_pool, redis=redis_client)
        await _scheduler_worker.start()
        logger.info("Spider scheduler worker started")
    except Exception as e:
        logger.warning(f"Spider scheduler worker start failed: {e}")
```

**Step 3: 修改 lifecycle.py**

找到并删除以下代码（约第 34、42-43 行）：

1. 修改 import 行：
```python
# 原来
from core.initializers import _generator_worker, _scheduler_worker

# 修改为
from core.initializers import _generator_worker
```

2. 删除 `_scheduler_worker` 清理代码：
```python
    if _scheduler_worker:
        await _safe_stop(_scheduler_worker.stop(), "Spider scheduler worker")
```

**Step 4: 验证 Python 语法**

Run: `cd content_worker && python -m py_compile core/initializers.py core/lifecycle.py`
Expected: 无输出（表示语法正确）

**Step 5: Commit**

```bash
git add -A
git commit -m "refactor(python): 删除 SpiderSchedulerWorker，由 Go 统一调度"
```

---

## Task 9: 编译验证和基本测试

**Step 1: Go 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功

**Step 2: 运行 Go 单元测试（如果有）**

Run: `cd api && go test ./... -v 2>&1 | head -50`
Expected: 测试通过或无测试

**Step 3: Python 语法验证**

Run: `cd content_worker && python -c "from core.initializers import init_components; from core.lifecycle import cleanup_components; print('OK')"`
Expected: 输出 "OK"

**Step 4: Commit（如有修复）**

如果发现问题并修复，提交修复。

---

## Task 10: 合并到主分支

**Step 1: 确认所有变更**

Run: `git log --oneline feature/spider-scheduler ^main`
Expected: 显示所有提交

**Step 2: 切换到主分支并合并**

```bash
cd "E:\j\模板\seo_html_generator"
git checkout main
git merge feature/spider-scheduler --no-ff -m "feat: Go 统一调度定时爬虫功能"
```

**Step 3: 清理 worktree**

```bash
git worktree remove .worktrees/spider-scheduler
git branch -d feature/spider-scheduler
```

---

## 验收测试

完成实现后，手动测试以下场景：

1. **创建项目时配置定时规则**
   - 创建爬虫项目，设置"每天 08:00 执行"
   - 检查 `scheduled_tasks` 表是否生成 `task_type='run_spider'` 的记录

2. **修改项目定时规则**
   - 修改为"每隔 30 分钟执行"
   - 检查 `scheduled_tasks` 表记录是否更新

3. **禁用项目**
   - 禁用项目
   - 检查对应定时任务是否被禁用

4. **删除项目**
   - 删除项目
   - 检查对应定时任务是否被删除

5. **定时触发**（可选，需等待触发时间）
   - 配置一个 1 分钟后触发的任务
   - 检查 `task_logs` 表是否记录执行
   - 检查 Python 端是否收到 Redis 命令并执行爬虫
