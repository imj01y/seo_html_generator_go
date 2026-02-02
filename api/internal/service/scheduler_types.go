// Package core provides scheduler type definitions
package core

import (
	"encoding/json"
	"time"
)

// TaskType 定时任务类型
type TaskType string

const (
	// TaskTypeRefreshData 刷新数据池
	TaskTypeRefreshData TaskType = "refresh_data"
	// TaskTypeRefreshTemplate 刷新模板缓存
	TaskTypeRefreshTemplate TaskType = "refresh_template"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	// TaskStatusPending 待执行
	TaskStatusPending TaskStatus = "pending"
	// TaskStatusRunning 执行中
	TaskStatusRunning TaskStatus = "running"
	// TaskStatusSuccess 执行成功
	TaskStatusSuccess TaskStatus = "success"
	// TaskStatusFailed 执行失败
	TaskStatusFailed TaskStatus = "failed"
)

// ScheduledTask 定时任务
type ScheduledTask struct {
	ID        int64           `db:"id" json:"id"`
	Name      string          `db:"name" json:"name"`
	TaskType  TaskType        `db:"task_type" json:"task_type"`
	CronExpr  string          `db:"cron_expr" json:"cron_expr"`
	Params    json.RawMessage `db:"params" json:"params"`
	Enabled   bool            `db:"enabled" json:"enabled"`
	LastRunAt *time.Time      `db:"last_run_at" json:"last_run_at"`
	NextRunAt *time.Time      `db:"next_run_at" json:"next_run_at"`
	CreatedAt time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at" json:"updated_at"`

	// 运行时字段，不存储到数据库
	cronEntryID int `db:"-" json:"-"`
}

// TaskLog 任务执行日志
type TaskLog struct {
	ID        int64      `db:"id" json:"id"`
	TaskID    int64      `db:"task_id" json:"task_id"`
	Status    TaskStatus `db:"status" json:"status"`
	Message   string     `db:"message" json:"message"`
	Duration  int64      `db:"duration" json:"duration"` // 执行时长（毫秒）
	StartedAt time.Time  `db:"started_at" json:"started_at"`
	EndedAt   *time.Time `db:"ended_at" json:"ended_at"`
	CreatedAt time.Time  `db:"created_at" json:"created_at"`
}

// TaskResult 任务执行结果
type TaskResult struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Duration int64  `json:"duration"` // 执行时长（毫秒）
}

// TaskHandler 任务处理器接口
type TaskHandler interface {
	// Handle 执行任务
	Handle(task *ScheduledTask) TaskResult
	// TaskType 返回处理器支持的任务类型
	TaskType() TaskType
}

// RefreshDataParams 刷新数据池参数
type RefreshDataParams struct {
	// PoolName 要刷新的数据池名称
	// 可选值: keywords, images, titles, contents, all
	PoolName string `json:"pool_name"`
	// SiteID 指定站点ID，0表示全局
	SiteID int `json:"site_id,omitempty"`
}

// RefreshTemplateParams 刷新模板缓存参数
type RefreshTemplateParams struct {
	// TemplateName 要刷新的模板名称，空表示全部
	TemplateName string `json:"template_name,omitempty"`
	// SiteGroupID 站点组ID，0表示全部
	SiteGroupID int `json:"site_group_id,omitempty"`
}

// ParseRefreshDataParams 解析刷新数据池参数
func ParseRefreshDataParams(data json.RawMessage) (*RefreshDataParams, error) {
	if len(data) == 0 {
		return &RefreshDataParams{PoolName: "all"}, nil
	}
	var params RefreshDataParams
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	if params.PoolName == "" {
		params.PoolName = "all"
	}
	return &params, nil
}

// ParseRefreshTemplateParams 解析刷新模板参数
func ParseRefreshTemplateParams(data json.RawMessage) (*RefreshTemplateParams, error) {
	if len(data) == 0 {
		return &RefreshTemplateParams{}, nil
	}
	var params RefreshTemplateParams
	if err := json.Unmarshal(data, &params); err != nil {
		return nil, err
	}
	return &params, nil
}
