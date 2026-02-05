// Package core provides the scheduler implementation
package core

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler 定时任务调度器
type Scheduler struct {
	db       *sqlx.DB
	cron     *cron.Cron
	handlers map[TaskType]TaskHandler
	tasks    map[int64]*ScheduledTask
	mu       sync.RWMutex
	running  bool
}

// NewScheduler 创建调度器
func NewScheduler(db *sqlx.DB) *Scheduler {
	return &Scheduler{
		db:       db,
		cron:     cron.New(cron.WithSeconds()), // 支持秒级调度
		handlers: make(map[TaskType]TaskHandler),
		tasks:    make(map[int64]*ScheduledTask),
	}
}

// RegisterHandler 注册任务处理器
func (s *Scheduler) RegisterHandler(handler TaskHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[handler.TaskType()] = handler
	log.Info().Str("type", string(handler.TaskType())).Msg("Task handler registered")
}

// Start 启动调度器
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("scheduler is already running")
	}
	s.running = true
	s.mu.Unlock()

	// 加载任务
	if err := s.loadTasks(ctx); err != nil {
		return fmt.Errorf("load tasks: %w", err)
	}

	// 启动 cron
	s.cron.Start()
	log.Info().Msg("Scheduler started")

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()
	s.running = false
	log.Info().Msg("Scheduler stopped")
}

// loadTasks 从数据库加载任务
func (s *Scheduler) loadTasks(ctx context.Context) error {
	query := `SELECT id, name, task_type, cron_expr, params, enabled, last_run_at, next_run_at, created_at, updated_at
              FROM scheduled_tasks WHERE enabled = 1`

	var tasks []ScheduledTask
	if err := s.db.SelectContext(ctx, &tasks, query); err != nil {
		if err == sql.ErrNoRows {
			log.Info().Msg("No scheduled tasks found")
			return nil
		}
		return fmt.Errorf("query tasks: %w", err)
	}

	for i := range tasks {
		if err := s.scheduleTask(&tasks[i]); err != nil {
			log.Error().Err(err).Int64("task_id", tasks[i].ID).Str("name", tasks[i].Name).Msg("Failed to schedule task")
			continue
		}
	}

	log.Info().Int("count", len(tasks)).Msg("Tasks loaded and scheduled")
	return nil
}

// scheduleTask 调度单个任务
func (s *Scheduler) scheduleTask(task *ScheduledTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查处理器是否存在
	if _, exists := s.handlers[task.TaskType]; !exists {
		return fmt.Errorf("no handler for task type: %s", task.TaskType)
	}

	// 如果任务已存在，先移除
	if existingTask, exists := s.tasks[task.ID]; exists && existingTask.cronEntryID != 0 {
		s.cron.Remove(cron.EntryID(existingTask.cronEntryID))
	}

	// 创建任务副本
	taskCopy := *task

	// 添加 cron 任务
	entryID, err := s.cron.AddFunc(task.CronExpr, func() {
		s.executeTask(&taskCopy)
	})
	if err != nil {
		return fmt.Errorf("add cron job: %w", err)
	}

	task.cronEntryID = int(entryID)
	s.tasks[task.ID] = task

	// 更新下次执行时间
	entry := s.cron.Entry(entryID)
	nextRun := entry.Next
	s.updateNextRunAt(task.ID, nextRun)

	log.Info().
		Int64("task_id", task.ID).
		Str("name", task.Name).
		Str("cron", task.CronExpr).
		Time("next_run", nextRun).
		Msg("Task scheduled")

	return nil
}

// executeTask 执行任务
func (s *Scheduler) executeTask(task *ScheduledTask) {
	s.mu.RLock()
	handler, exists := s.handlers[task.TaskType]
	s.mu.RUnlock()

	if !exists {
		log.Error().Int64("task_id", task.ID).Str("type", string(task.TaskType)).Msg("No handler for task type")
		return
	}

	// 创建任务日志
	logID := s.createTaskLog(task.ID)

	log.Info().
		Int64("task_id", task.ID).
		Str("name", task.Name).
		Str("type", string(task.TaskType)).
		Msg("Executing task")

	startTime := time.Now()

	// 执行任务
	result := handler.Handle(task)

	// 更新日志
	s.updateTaskLog(logID, result)

	// 更新最后执行时间
	s.updateLastRunAt(task.ID)

	// 更新下次执行时间
	s.mu.RLock()
	if t, exists := s.tasks[task.ID]; exists && t.cronEntryID != 0 {
		entry := s.cron.Entry(cron.EntryID(t.cronEntryID))
		s.updateNextRunAt(task.ID, entry.Next)
	}
	s.mu.RUnlock()

	logLevel := log.Info()
	if !result.Success {
		logLevel = log.Error()
	}
	logLevel.
		Int64("task_id", task.ID).
		Str("name", task.Name).
		Bool("success", result.Success).
		Str("message", result.Message).
		Dur("duration", time.Since(startTime)).
		Msg("Task execution completed")
}

// createTaskLog 创建任务日志
func (s *Scheduler) createTaskLog(taskID int64) int64 {
	query := `INSERT INTO task_logs (task_id, status, started_at, created_at) VALUES (?, ?, ?, ?)`
	now := time.Now()
	result, err := s.db.Exec(query, taskID, TaskStatusRunning, now, now)
	if err != nil {
		log.Error().Err(err).Int64("task_id", taskID).Msg("Failed to create task log")
		return 0
	}
	id, _ := result.LastInsertId()
	return id
}

// updateTaskLog 更新任务日志
func (s *Scheduler) updateTaskLog(logID int64, result TaskResult) {
	if logID == 0 {
		return
	}

	status := TaskStatusSuccess
	if !result.Success {
		status = TaskStatusFailed
	}

	now := time.Now()
	query := `UPDATE task_logs SET status = ?, message = ?, duration = ?, ended_at = ? WHERE id = ?`
	if _, err := s.db.Exec(query, status, result.Message, result.Duration, now, logID); err != nil {
		log.Error().Err(err).Int64("log_id", logID).Msg("Failed to update task log")
	}
}

// updateLastRunAt 更新最后执行时间
func (s *Scheduler) updateLastRunAt(taskID int64) {
	query := `UPDATE scheduled_tasks SET last_run_at = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	if _, err := s.db.Exec(query, now, now, taskID); err != nil {
		log.Error().Err(err).Int64("task_id", taskID).Msg("Failed to update last_run_at")
	}
}

// updateNextRunAt 更新下次执行时间
func (s *Scheduler) updateNextRunAt(taskID int64, nextRun time.Time) {
	query := `UPDATE scheduled_tasks SET next_run_at = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	if _, err := s.db.Exec(query, nextRun, now, taskID); err != nil {
		log.Error().Err(err).Int64("task_id", taskID).Msg("Failed to update next_run_at")
	}
}

// TriggerTask 手动触发任务执行
func (s *Scheduler) TriggerTask(taskID int64) error {
	s.mu.RLock()
	task, exists := s.tasks[taskID]
	s.mu.RUnlock()

	if !exists {
		// 尝试从数据库加载
		var dbTask ScheduledTask
		query := `SELECT id, name, task_type, cron_expr, params, enabled, last_run_at, next_run_at, created_at, updated_at
                  FROM scheduled_tasks WHERE id = ?`
		if err := s.db.Get(&dbTask, query, taskID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("task not found: %d", taskID)
			}
			return fmt.Errorf("query task: %w", err)
		}
		task = &dbTask
	}

	// 异步执行
	go s.executeTask(task)

	return nil
}

// EnableTask 启用任务
func (s *Scheduler) EnableTask(ctx context.Context, taskID int64) error {
	// 更新数据库
	query := `UPDATE scheduled_tasks SET enabled = 1, updated_at = ? WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, query, time.Now(), taskID); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	// 加载并调度任务
	var task ScheduledTask
	selectQuery := `SELECT id, name, task_type, cron_expr, params, enabled, last_run_at, next_run_at, created_at, updated_at
                    FROM scheduled_tasks WHERE id = ?`
	if err := s.db.GetContext(ctx, &task, selectQuery, taskID); err != nil {
		return fmt.Errorf("query task: %w", err)
	}

	return s.scheduleTask(&task)
}

// DisableTask 禁用任务
func (s *Scheduler) DisableTask(ctx context.Context, taskID int64) error {
	// 更新数据库
	query := `UPDATE scheduled_tasks SET enabled = 0, updated_at = ? WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, query, time.Now(), taskID); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	// 从调度器移除
	s.mu.Lock()
	if task, exists := s.tasks[taskID]; exists {
		if task.cronEntryID != 0 {
			s.cron.Remove(cron.EntryID(task.cronEntryID))
		}
		delete(s.tasks, taskID)
	}
	s.mu.Unlock()

	log.Info().Int64("task_id", taskID).Msg("Task disabled")
	return nil
}

// GetTasks 获取所有任务
func (s *Scheduler) GetTasks(ctx context.Context) ([]ScheduledTask, error) {
	query := `SELECT id, name, task_type, cron_expr, params, enabled, last_run_at, next_run_at, created_at, updated_at
              FROM scheduled_tasks ORDER BY id`

	var tasks []ScheduledTask
	if err := s.db.SelectContext(ctx, &tasks, query); err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}

	return tasks, nil
}

// GetTaskLogs 获取任务日志
func (s *Scheduler) GetTaskLogs(ctx context.Context, taskID int64, limit int) ([]TaskLog, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT id, task_id, status, message, duration, started_at, ended_at, created_at
              FROM task_logs WHERE task_id = ? ORDER BY id DESC LIMIT ?`

	var logs []TaskLog
	if err := s.db.SelectContext(ctx, &logs, query, taskID, limit); err != nil {
		return nil, fmt.Errorf("query task logs: %w", err)
	}

	return logs, nil
}

// GetRecentLogs 获取最近的任务日志
func (s *Scheduler) GetRecentLogs(ctx context.Context, limit int) ([]TaskLog, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT id, task_id, status, message, duration, started_at, ended_at, created_at
              FROM task_logs ORDER BY id DESC LIMIT ?`

	var logs []TaskLog
	if err := s.db.SelectContext(ctx, &logs, query, limit); err != nil {
		return nil, fmt.Errorf("query recent logs: %w", err)
	}

	return logs, nil
}

// CreateTask 创建新任务
func (s *Scheduler) CreateTask(ctx context.Context, task *ScheduledTask) (int64, error) {
	query := `INSERT INTO scheduled_tasks (name, task_type, cron_expr, params, enabled, created_at, updated_at)
              VALUES (?, ?, ?, ?, ?, ?, ?)`

	now := time.Now()
	result, err := s.db.ExecContext(ctx, query,
		task.Name, task.TaskType, task.CronExpr, task.Params, task.Enabled, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert task: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}

	task.ID = id
	task.CreatedAt = now
	task.UpdatedAt = now

	// 如果启用则调度
	if task.Enabled {
		if err := s.scheduleTask(task); err != nil {
			log.Warn().Err(err).Int64("task_id", id).Msg("Task created but failed to schedule")
		}
	}

	return id, nil
}

// UpdateTask 更新任务
func (s *Scheduler) UpdateTask(ctx context.Context, task *ScheduledTask) error {
	query := `UPDATE scheduled_tasks SET name = ?, task_type = ?, cron_expr = ?, params = ?, enabled = ?, updated_at = ?
              WHERE id = ?`

	now := time.Now()
	if _, err := s.db.ExecContext(ctx, query,
		task.Name, task.TaskType, task.CronExpr, task.Params, task.Enabled, now, task.ID); err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	task.UpdatedAt = now

	// 重新调度
	s.mu.Lock()
	if existingTask, exists := s.tasks[task.ID]; exists && existingTask.cronEntryID != 0 {
		s.cron.Remove(cron.EntryID(existingTask.cronEntryID))
		delete(s.tasks, task.ID)
	}
	s.mu.Unlock()

	if task.Enabled {
		if err := s.scheduleTask(task); err != nil {
			return fmt.Errorf("reschedule task: %w", err)
		}
	}

	return nil
}

// DeleteTask 删除任务
func (s *Scheduler) DeleteTask(ctx context.Context, taskID int64) error {
	// 从调度器移除
	s.mu.Lock()
	if task, exists := s.tasks[taskID]; exists {
		if task.cronEntryID != 0 {
			s.cron.Remove(cron.EntryID(task.cronEntryID))
		}
		delete(s.tasks, taskID)
	}
	s.mu.Unlock()

	// 删除日志
	if _, err := s.db.ExecContext(ctx, "DELETE FROM task_logs WHERE task_id = ?", taskID); err != nil {
		log.Warn().Err(err).Int64("task_id", taskID).Msg("Failed to delete task logs")
	}

	// 删除任务
	query := `DELETE FROM scheduled_tasks WHERE id = ?`
	if _, err := s.db.ExecContext(ctx, query, taskID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	log.Info().Int64("task_id", taskID).Msg("Task deleted")
	return nil
}

// ReloadTasks 重新加载所有任务
func (s *Scheduler) ReloadTasks(ctx context.Context) error {
	// 清空当前任务
	s.mu.Lock()
	for id, task := range s.tasks {
		if task.cronEntryID != 0 {
			s.cron.Remove(cron.EntryID(task.cronEntryID))
		}
		delete(s.tasks, id)
	}
	s.mu.Unlock()

	// 重新加载
	return s.loadTasks(ctx)
}

// GetStats 获取调度器统计
func (s *Scheduler) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	activeCount := 0
	for _, task := range s.tasks {
		if task.Enabled {
			activeCount++
		}
	}

	return map[string]interface{}{
		"running":      s.running,
		"total_tasks":  len(s.tasks),
		"active_tasks": activeCount,
		"cron_entries": len(s.cron.Entries()),
		"handlers":     len(s.handlers),
	}
}
