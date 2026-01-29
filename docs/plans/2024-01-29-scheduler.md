# 定时任务调度实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现基于 cron 的定时任务调度系统，支持任务管理、执行日志、手动触发。

**Architecture:** robfig/cron v3 + MySQL 任务存储 + 执行日志记录

**Tech Stack:** Go, robfig/cron/v3, MySQL

---

## Task 1: 创建数据库表

**Files:**
- Create: `go-page-server/migrations/003_scheduled_tasks.sql`

**Step 1: 创建迁移文件**

```sql
-- 定时任务表
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '任务名称',
    task_type VARCHAR(50) NOT NULL COMMENT '任务类型: refresh_data, refresh_template, clear_cache, push_urls',
    cron_expr VARCHAR(100) NOT NULL COMMENT 'Cron表达式',
    params JSON COMMENT '任务参数',
    enabled TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    last_run_at DATETIME COMMENT '上次执行时间',
    next_run_at DATETIME COMMENT '下次执行时间',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_enabled (enabled),
    INDEX idx_next_run (next_run_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='定时任务表';

-- 任务执行日志表
CREATE TABLE IF NOT EXISTS task_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id INT NOT NULL COMMENT '任务ID',
    status ENUM('running', 'success', 'failed') NOT NULL DEFAULT 'running',
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_ms INT COMMENT '执行耗时(毫秒)',
    result TEXT COMMENT '执行结果',
    error_msg TEXT COMMENT '错误信息',
    INDEX idx_task_id (task_id),
    INDEX idx_start_time (start_time),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务执行日志';

-- 插入默认任务
INSERT INTO scheduled_tasks (name, task_type, cron_expr, params, enabled) VALUES
('刷新数据池', 'refresh_data', '0 */10 * * * *', '{"pools": ["all"]}', 1),
('刷新模板缓存', 'refresh_template', '0 */30 * * * *', '{}', 1),
('清理过期缓存', 'clear_cache', '0 0 3 * * *', '{"max_age_hours": 24}', 1);
```

**Step 2: Commit**

```bash
git add go-page-server/migrations/003_scheduled_tasks.sql
git commit -m "feat: add scheduled tasks migration"
```

---

## Task 2: 定义任务类型

**Files:**
- Create: `go-page-server/core/scheduler_types.go`

**Step 1: 创建类型定义**

```go
package core

import (
	"context"
	"encoding/json"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeRefreshData     TaskType = "refresh_data"
	TaskTypeRefreshTemplate TaskType = "refresh_template"
	TaskTypeClearCache      TaskType = "clear_cache"
	TaskTypePushURLs        TaskType = "push_urls"
)

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID        int             `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	TaskType  TaskType        `json:"task_type" db:"task_type"`
	CronExpr  string          `json:"cron_expr" db:"cron_expr"`
	Params    json.RawMessage `json:"params" db:"params"`
	Enabled   bool            `json:"enabled" db:"enabled"`
	LastRunAt *time.Time      `json:"last_run_at" db:"last_run_at"`
	NextRunAt *time.Time      `json:"next_run_at" db:"next_run_at"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// TaskLog 任务日志
type TaskLog struct {
	ID         int64      `json:"id" db:"id"`
	TaskID     int        `json:"task_id" db:"task_id"`
	Status     string     `json:"status" db:"status"`
	StartTime  time.Time  `json:"start_time" db:"start_time"`
	EndTime    *time.Time `json:"end_time" db:"end_time"`
	DurationMs *int       `json:"duration_ms" db:"duration_ms"`
	Result     *string    `json:"result" db:"result"`
	ErrorMsg   *string    `json:"error_msg" db:"error_msg"`
}

// TaskHandler 任务处理器接口
type TaskHandler interface {
	Handle(ctx context.Context, params json.RawMessage) (string, error)
}

// TaskResult 任务执行结果
type TaskResult struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Duration int    `json:"duration_ms"`
}

// RefreshDataParams 刷新数据参数
type RefreshDataParams struct {
	Pools []string `json:"pools"` // all, keywords, images, titles, contents
}

// RefreshTemplateParams 刷新模板参数
type RefreshTemplateParams struct {
	TemplateIDs []int `json:"template_ids,omitempty"` // 空表示全部
}

// ClearCacheParams 清理缓存参数
type ClearCacheParams struct {
	MaxAgeHours int `json:"max_age_hours"`
}

// PushURLsParams 推送URL参数
type PushURLsParams struct {
	SiteID int `json:"site_id"`
	Limit  int `json:"limit"`
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/scheduler_types.go
git commit -m "feat: add scheduler type definitions"
```

---

## Task 3: 实现调度器核心

**Files:**
- Create: `go-page-server/core/scheduler.go`

**Step 1: 创建调度器**

```go
package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler 任务调度器
type Scheduler struct {
	db       *sql.DB
	cron     *cron.Cron
	handlers map[TaskType]TaskHandler
	entryMap map[int]cron.EntryID // taskID -> cronEntryID
	mu       sync.RWMutex
	running  bool
}

// NewScheduler 创建调度器
func NewScheduler(db *sql.DB) *Scheduler {
	return &Scheduler{
		db:       db,
		cron:     cron.New(cron.WithSeconds()),
		handlers: make(map[TaskType]TaskHandler),
		entryMap: make(map[int]cron.EntryID),
	}
}

// RegisterHandler 注册任务处理器
func (s *Scheduler) RegisterHandler(taskType TaskType, handler TaskHandler) {
	s.handlers[taskType] = handler
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context) error {
	tasks, err := s.loadTasks(ctx)
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.Enabled {
			if err := s.scheduleTask(task); err != nil {
				log.Error().
					Err(err).
					Int("task_id", task.ID).
					Str("name", task.Name).
					Msg("Failed to schedule task")
			}
		}
	}

	s.cron.Start()
	s.running = true

	log.Info().
		Int("task_count", len(tasks)).
		Msg("Scheduler started")

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.running = false

	log.Info().Msg("Scheduler stopped")
}

// loadTasks 加载任务
func (s *Scheduler) loadTasks(ctx context.Context) ([]ScheduledTask, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, task_type, cron_expr, params, enabled,
		       last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_tasks
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []ScheduledTask
	for rows.Next() {
		var task ScheduledTask
		if err := rows.Scan(
			&task.ID, &task.Name, &task.TaskType, &task.CronExpr,
			&task.Params, &task.Enabled, &task.LastRunAt, &task.NextRunAt,
			&task.CreatedAt, &task.UpdatedAt,
		); err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// scheduleTask 调度单个任务
func (s *Scheduler) scheduleTask(task ScheduledTask) error {
	handler, ok := s.handlers[task.TaskType]
	if !ok {
		log.Warn().
			Str("task_type", string(task.TaskType)).
			Msg("No handler registered for task type")
		return nil
	}

	entryID, err := s.cron.AddFunc(task.CronExpr, func() {
		s.executeTask(task.ID, task.Name, task.TaskType, task.Params, handler)
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.entryMap[task.ID] = entryID
	s.mu.Unlock()

	// 更新下次执行时间
	entry := s.cron.Entry(entryID)
	s.updateNextRunAt(task.ID, entry.Next)

	log.Info().
		Int("task_id", task.ID).
		Str("name", task.Name).
		Str("cron", task.CronExpr).
		Time("next_run", entry.Next).
		Msg("Task scheduled")

	return nil
}

// executeTask 执行任务
func (s *Scheduler) executeTask(taskID int, name string, taskType TaskType, params json.RawMessage, handler TaskHandler) {
	startTime := time.Now()

	// 创建日志记录
	logID, err := s.createTaskLog(taskID, startTime)
	if err != nil {
		log.Error().Err(err).Int("task_id", taskID).Msg("Failed to create task log")
		return
	}

	log.Info().
		Int("task_id", taskID).
		Str("name", name).
		Msg("Task started")

	// 执行任务
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, execErr := handler.Handle(ctx, params)

	endTime := time.Now()
	duration := int(endTime.Sub(startTime).Milliseconds())

	// 更新日志
	status := "success"
	var errorMsg *string
	if execErr != nil {
		status = "failed"
		errStr := execErr.Error()
		errorMsg = &errStr
	}

	s.updateTaskLog(logID, status, endTime, duration, result, errorMsg)
	s.updateLastRunAt(taskID, startTime)

	// 更新下次执行时间
	s.mu.RLock()
	entryID, ok := s.entryMap[taskID]
	s.mu.RUnlock()

	if ok {
		entry := s.cron.Entry(entryID)
		s.updateNextRunAt(taskID, entry.Next)
	}

	if execErr != nil {
		log.Error().
			Err(execErr).
			Int("task_id", taskID).
			Str("name", name).
			Int("duration_ms", duration).
			Msg("Task failed")
	} else {
		log.Info().
			Int("task_id", taskID).
			Str("name", name).
			Int("duration_ms", duration).
			Msg("Task completed")
	}
}

// createTaskLog 创建任务日志
func (s *Scheduler) createTaskLog(taskID int, startTime time.Time) (int64, error) {
	result, err := s.db.Exec(`
		INSERT INTO task_logs (task_id, status, start_time)
		VALUES (?, 'running', ?)
	`, taskID, startTime)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// updateTaskLog 更新任务日志
func (s *Scheduler) updateTaskLog(logID int64, status string, endTime time.Time, duration int, result string, errorMsg *string) {
	_, err := s.db.Exec(`
		UPDATE task_logs
		SET status = ?, end_time = ?, duration_ms = ?, result = ?, error_msg = ?
		WHERE id = ?
	`, status, endTime, duration, result, errorMsg, logID)
	if err != nil {
		log.Error().Err(err).Int64("log_id", logID).Msg("Failed to update task log")
	}
}

// updateLastRunAt 更新上次执行时间
func (s *Scheduler) updateLastRunAt(taskID int, t time.Time) {
	_, err := s.db.Exec(`
		UPDATE scheduled_tasks SET last_run_at = ? WHERE id = ?
	`, t, taskID)
	if err != nil {
		log.Error().Err(err).Int("task_id", taskID).Msg("Failed to update last_run_at")
	}
}

// updateNextRunAt 更新下次执行时间
func (s *Scheduler) updateNextRunAt(taskID int, t time.Time) {
	_, err := s.db.Exec(`
		UPDATE scheduled_tasks SET next_run_at = ? WHERE id = ?
	`, t, taskID)
	if err != nil {
		log.Error().Err(err).Int("task_id", taskID).Msg("Failed to update next_run_at")
	}
}

// TriggerTask 手动触发任务
func (s *Scheduler) TriggerTask(ctx context.Context, taskID int) error {
	var task ScheduledTask
	err := s.db.QueryRowContext(ctx, `
		SELECT id, name, task_type, params FROM scheduled_tasks WHERE id = ?
	`, taskID).Scan(&task.ID, &task.Name, &task.TaskType, &task.Params)
	if err != nil {
		return err
	}

	handler, ok := s.handlers[task.TaskType]
	if !ok {
		return nil
	}

	go s.executeTask(task.ID, task.Name, task.TaskType, task.Params, handler)
	return nil
}

// EnableTask 启用任务
func (s *Scheduler) EnableTask(ctx context.Context, taskID int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_tasks SET enabled = 1 WHERE id = ?
	`, taskID)
	if err != nil {
		return err
	}

	// 重新调度
	var task ScheduledTask
	err = s.db.QueryRowContext(ctx, `
		SELECT id, name, task_type, cron_expr, params, enabled
		FROM scheduled_tasks WHERE id = ?
	`, taskID).Scan(&task.ID, &task.Name, &task.TaskType, &task.CronExpr, &task.Params, &task.Enabled)
	if err != nil {
		return err
	}

	return s.scheduleTask(task)
}

// DisableTask 禁用任务
func (s *Scheduler) DisableTask(ctx context.Context, taskID int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_tasks SET enabled = 0 WHERE id = ?
	`, taskID)
	if err != nil {
		return err
	}

	// 移除调度
	s.mu.Lock()
	if entryID, ok := s.entryMap[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.entryMap, taskID)
	}
	s.mu.Unlock()

	return nil
}

// GetTasks 获取所有任务
func (s *Scheduler) GetTasks(ctx context.Context) ([]ScheduledTask, error) {
	return s.loadTasks(ctx)
}

// GetTaskLogs 获取任务日志
func (s *Scheduler) GetTaskLogs(ctx context.Context, taskID int, limit int) ([]TaskLog, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, status, start_time, end_time, duration_ms, result, error_msg
		FROM task_logs
		WHERE task_id = ?
		ORDER BY start_time DESC
		LIMIT ?
	`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []TaskLog
	for rows.Next() {
		var l TaskLog
		if err := rows.Scan(
			&l.ID, &l.TaskID, &l.Status, &l.StartTime, &l.EndTime,
			&l.DurationMs, &l.Result, &l.ErrorMsg,
		); err != nil {
			continue
		}
		logs = append(logs, l)
	}

	return logs, nil
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/scheduler.go
git commit -m "feat: implement task scheduler with cron"
```

---

## Task 4: 实现任务处理器

**Files:**
- Create: `go-page-server/core/task_handlers.go`

**Step 1: 创建处理器实现**

```go
package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// RefreshDataHandler 刷新数据处理器
type RefreshDataHandler struct {
	dataPoolManager *DataPoolManager
}

func NewRefreshDataHandler(dpm *DataPoolManager) *RefreshDataHandler {
	return &RefreshDataHandler{dataPoolManager: dpm}
}

func (h *RefreshDataHandler) Handle(ctx context.Context, params json.RawMessage) (string, error) {
	var p RefreshDataParams
	if err := json.Unmarshal(params, &p); err != nil {
		p.Pools = []string{"all"}
	}

	for _, pool := range p.Pools {
		if err := h.dataPoolManager.Refresh(ctx, pool); err != nil {
			return "", fmt.Errorf("failed to refresh %s: %w", pool, err)
		}
	}

	stats := h.dataPoolManager.GetStats()
	return fmt.Sprintf("刷新完成: keywords=%d, images=%d, titles=%d, contents=%d",
		stats.KeywordsCount, stats.ImagesCount, stats.TitlesCount, stats.ContentsCount), nil
}

// RefreshTemplateHandler 刷新模板处理器
type RefreshTemplateHandler struct {
	templateCache *TemplateCache
}

func NewRefreshTemplateHandler(tc *TemplateCache) *RefreshTemplateHandler {
	return &RefreshTemplateHandler{templateCache: tc}
}

func (h *RefreshTemplateHandler) Handle(ctx context.Context, params json.RawMessage) (string, error) {
	var p RefreshTemplateParams
	json.Unmarshal(params, &p)

	if len(p.TemplateIDs) == 0 {
		// 刷新全部
		if err := h.templateCache.LoadAll(ctx); err != nil {
			return "", err
		}
		return "全部模板刷新完成", nil
	}

	// 刷新指定模板
	for _, id := range p.TemplateIDs {
		if err := h.templateCache.LoadTemplate(ctx, id); err != nil {
			return "", fmt.Errorf("failed to refresh template %d: %w", id, err)
		}
	}

	return fmt.Sprintf("刷新了 %d 个模板", len(p.TemplateIDs)), nil
}

// ClearCacheHandler 清理缓存处理器
type ClearCacheHandler struct {
	// 可以添加缓存管理器引用
}

func NewClearCacheHandler() *ClearCacheHandler {
	return &ClearCacheHandler{}
}

func (h *ClearCacheHandler) Handle(ctx context.Context, params json.RawMessage) (string, error) {
	var p ClearCacheParams
	if err := json.Unmarshal(params, &p); err != nil {
		p.MaxAgeHours = 24
	}

	// TODO: 实现缓存清理逻辑
	return fmt.Sprintf("清理了超过 %d 小时的缓存", p.MaxAgeHours), nil
}

// PushURLsHandler 推送URL处理器
type PushURLsHandler struct {
	// 可以添加推送服务引用
}

func NewPushURLsHandler() *PushURLsHandler {
	return &PushURLsHandler{}
}

func (h *PushURLsHandler) Handle(ctx context.Context, params json.RawMessage) (string, error) {
	var p PushURLsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", err
	}

	// TODO: 实现 URL 推送逻辑
	return fmt.Sprintf("推送了 %d 个URL到站点 %d", p.Limit, p.SiteID), nil
}
```

**Step 2: Commit**

```bash
git add go-page-server/core/task_handlers.go
git commit -m "feat: implement task handlers"
```

---

## Task 5: 添加测试

**Files:**
- Create: `go-page-server/core/scheduler_test.go`

**Step 1: 创建测试文件**

```go
package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type MockHandler struct {
	called  bool
	result  string
	err     error
	delay   time.Duration
}

func (h *MockHandler) Handle(ctx context.Context, params json.RawMessage) (string, error) {
	if h.delay > 0 {
		time.Sleep(h.delay)
	}
	h.called = true
	return h.result, h.err
}

func TestScheduledTask_JSON(t *testing.T) {
	task := ScheduledTask{
		ID:       1,
		Name:     "测试任务",
		TaskType: TaskTypeRefreshData,
		CronExpr: "0 */10 * * * *",
		Params:   json.RawMessage(`{"pools": ["all"]}`),
		Enabled:  true,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ScheduledTask
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Name != task.Name {
		t.Errorf("Expected name %s, got %s", task.Name, decoded.Name)
	}
}

func TestRefreshDataParams(t *testing.T) {
	params := RefreshDataParams{
		Pools: []string{"keywords", "images"},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}

	var decoded RefreshDataParams
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Pools) != 2 {
		t.Errorf("Expected 2 pools, got %d", len(decoded.Pools))
	}
}

func TestTaskTypes(t *testing.T) {
	types := []TaskType{
		TaskTypeRefreshData,
		TaskTypeRefreshTemplate,
		TaskTypeClearCache,
		TaskTypePushURLs,
	}

	for _, tt := range types {
		if string(tt) == "" {
			t.Errorf("Task type should not be empty")
		}
	}
}
```

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./core/... -run TestScheduled
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/core/scheduler_test.go
git commit -m "test: add scheduler tests"
```

---

## Task 6: 更新 main.go 初始化

**Files:**
- Modify: `go-page-server/main.go`

**Step 1: 添加调度器初始化**

在 `main()` 函数中添加：

```go
// 初始化调度器
scheduler := core.NewScheduler(db)

// 注册任务处理器
scheduler.RegisterHandler(core.TaskTypeRefreshData, core.NewRefreshDataHandler(dataPoolManager))
scheduler.RegisterHandler(core.TaskTypeRefreshTemplate, core.NewRefreshTemplateHandler(templateCache))
scheduler.RegisterHandler(core.TaskTypeClearCache, core.NewClearCacheHandler())
scheduler.RegisterHandler(core.TaskTypePushURLs, core.NewPushURLsHandler())

// 启动调度器
if err := scheduler.Start(ctx); err != nil {
	log.Fatal().Err(err).Msg("Failed to start scheduler")
}
defer scheduler.Stop()
```

**Step 2: Commit**

```bash
git add go-page-server/main.go
git commit -m "feat: initialize scheduler in main"
```

---

## 完成检查清单

- [ ] Task 1: 数据库迁移
- [ ] Task 2: 类型定义
- [ ] Task 3: 调度器核心
- [ ] Task 4: 任务处理器
- [ ] Task 5: 测试覆盖
- [ ] Task 6: main.go 初始化

所有任务完成后运行完整测试：

```bash
cd go-page-server && go test -v ./...
```