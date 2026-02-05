package models

import (
	"encoding/json"
	"time"
)

// SpiderProject 爬虫项目
type SpiderProject struct {
	ID              int             `db:"id" json:"id"`
	Name            string          `db:"name" json:"name"`
	Description     *string         `db:"description" json:"description"`
	EntryFile       string          `db:"entry_file" json:"entry_file"`
	EntryFunction   string          `db:"entry_function" json:"entry_function"`
	StartURL        *string         `db:"start_url" json:"start_url"`
	Config          *string         `db:"config" json:"-"`
	ConfigParsed    json.RawMessage `json:"config"`
	Concurrency     int             `db:"concurrency" json:"concurrency"`
	OutputGroupID   int             `db:"output_group_id" json:"output_group_id"`
	Schedule        *string         `db:"schedule" json:"schedule"`
	Enabled         int             `db:"enabled" json:"enabled"`
	Status          string          `db:"status" json:"status"`
	LastRunAt       *time.Time      `db:"last_run_at" json:"last_run_at"`
	LastRunDuration *int            `db:"last_run_duration" json:"last_run_duration"`
	LastRunItems    *int            `db:"last_run_items" json:"last_run_items"`
	LastError       *string         `db:"last_error" json:"last_error"`
	TotalRuns       int             `db:"total_runs" json:"total_runs"`
	TotalItems      int             `db:"total_items" json:"total_items"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`
}

// SpiderProjectFile 项目文件
type SpiderProjectFile struct {
	ID        int       `db:"id" json:"id"`
	ProjectID int       `db:"project_id" json:"project_id"`
	Path      string    `db:"path" json:"path"`
	Type      string    `db:"type" json:"type"` // "file" or "dir"
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// SpiderProjectCreate 创建请求
type SpiderProjectCreate struct {
	Name          string                 `json:"name" binding:"required"`
	Description   *string                `json:"description"`
	EntryFile     string                 `json:"entry_file"`
	EntryFunction string                 `json:"entry_function"`
	StartURL      *string                `json:"start_url"`
	Config        map[string]interface{} `json:"config"`
	Concurrency   int                    `json:"concurrency"`
	OutputGroupID int                    `json:"output_group_id"`
	Schedule      *string                `json:"schedule"`
	Enabled       int                    `json:"enabled"`
	Files         []SpiderFileCreate     `json:"files"`
}

// SpiderProjectUpdate 更新请求
type SpiderProjectUpdate struct {
	Name          *string                `json:"name"`
	Description   *string                `json:"description"`
	EntryFile     *string                `json:"entry_file"`
	EntryFunction *string                `json:"entry_function"`
	StartURL      *string                `json:"start_url"`
	Config        map[string]interface{} `json:"config"`
	Concurrency   *int                   `json:"concurrency"`
	OutputGroupID *int                   `json:"output_group_id"`
	Schedule      *string                `json:"schedule"`
	Enabled       *int                   `json:"enabled"`
}

// SpiderFileCreate 创建文件请求
type SpiderFileCreate struct {
	Filename string `json:"filename" binding:"required"`
	Content  string `json:"content"`
}

// SpiderFileUpdate 更新文件请求
type SpiderFileUpdate struct {
	Content string `json:"content" binding:"required"`
}

// SpiderCommand Redis 命令结构
type SpiderCommand struct {
	Action    string `json:"action"`
	ProjectID int    `json:"project_id"`
	MaxItems  int    `json:"max_items,omitempty"`
	Timestamp int64  `json:"timestamp"`
}

// SpiderFailedRequest 失败请求
type SpiderFailedRequest struct {
	ID           int       `db:"id" json:"id"`
	ProjectID    int       `db:"project_id" json:"project_id"`
	URL          string    `db:"url" json:"url"`
	Method       string    `db:"method" json:"method"`
	Callback     *string   `db:"callback" json:"callback"`
	Meta         *string   `db:"meta" json:"meta"`
	ErrorMessage *string   `db:"error_message" json:"error_message"`
	RetryCount   int       `db:"retry_count" json:"retry_count"`
	FailedAt     time.Time `db:"failed_at" json:"failed_at"`
	Status       string    `db:"status" json:"status"`
}

// SpiderStats 实时统计
type SpiderStats struct {
	Status      string  `json:"status"`
	Total       int     `json:"total"`
	Completed   int     `json:"completed"`
	Failed      int     `json:"failed"`
	Retried     int     `json:"retried"`
	Pending     int     `json:"pending"`
	Processing  int     `json:"processing"`
	SuccessRate float64 `json:"success_rate"`
}

// SpiderTreeNode 文件树节点
type SpiderTreeNode struct {
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Type     string            `json:"type"` // "file" or "dir"
	Children []*SpiderTreeNode `json:"children,omitempty"`
}

// SpiderCreateItemRequest 创建文件或目录请求
type SpiderCreateItemRequest struct {
	Name    string `json:"name" binding:"required"`
	Type    string `json:"type" binding:"required,oneof=file dir"`
	Content string `json:"content"` // 可选，仅对 file 类型有效
}

// SpiderMoveRequest 移动/重命名请求
type SpiderMoveRequest struct {
	NewPath string `json:"new_path" binding:"required"`
}

// StatsChartPoint 统计图表数据点（用于 API 响应，time 字段通过 SQL AS 别名映射）
type StatsChartPoint struct {
	Time      time.Time `db:"time" json:"time"`
	Total     int       `db:"total" json:"total"`
	Completed int       `db:"completed" json:"completed"`
	Failed    int       `db:"failed" json:"failed"`
	Retried   int       `db:"retried" json:"retried"`
	AvgSpeed  float64   `db:"avg_speed" json:"avg_speed"`
}

// SpiderStatsHistory 爬虫统计历史记录（对应 spider_stats_history 表）
type SpiderStatsHistory struct {
	ID          int       `db:"id"           json:"id"`
	ProjectID   int       `db:"project_id"   json:"project_id"`
	PeriodType  string    `db:"period_type"  json:"period_type"`
	PeriodStart time.Time `db:"period_start" json:"period_start"`
	Total       int       `db:"total"        json:"total"`
	Completed   int       `db:"completed"    json:"completed"`
	Failed      int       `db:"failed"       json:"failed"`
	Retried     int       `db:"retried"      json:"retried"`
	AvgSpeed    *float64  `db:"avg_speed"    json:"avg_speed"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}
