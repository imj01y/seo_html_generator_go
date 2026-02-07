// Package models defines data structures for the SEO page generator
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ============================================
// 站群和分组相关
// ============================================

// SiteGroup represents a site group from the database.
type SiteGroup struct {
	ID          int            `db:"id"          json:"id"`
	Name        string         `db:"name"        json:"name"`
	Description sql.NullString `db:"description" json:"description"`
	IsDefault   int            `db:"is_default"  json:"is_default"`
	Status      int            `db:"status"      json:"status"`
	CreatedAt   time.Time      `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"  json:"updated_at"`
}

// KeywordGroup represents a keyword group from the database.
type KeywordGroup struct {
	ID          int            `db:"id"           json:"id"`
	SiteGroupID int            `db:"site_group_id" json:"site_group_id"`
	Name        string         `db:"name"         json:"name"`
	Description sql.NullString `db:"description"  json:"description"`
	IsDefault   int            `db:"is_default"   json:"is_default"`
	Status      int            `db:"status"       json:"status"`
	CreatedAt   time.Time      `db:"created_at"   json:"created_at"`
}

// ImageGroup represents an image group from the database.
type ImageGroup struct {
	ID          int            `db:"id"           json:"id"`
	SiteGroupID int            `db:"site_group_id" json:"site_group_id"`
	Name        string         `db:"name"         json:"name"`
	Description sql.NullString `db:"description"  json:"description"`
	IsDefault   int            `db:"is_default"   json:"is_default"`
	Status      int            `db:"status"       json:"status"`
	CreatedAt   time.Time      `db:"created_at"   json:"created_at"`
}

// ArticleGroup represents an article group from the database.
type ArticleGroup struct {
	ID          int            `db:"id"           json:"id"`
	SiteGroupID int            `db:"site_group_id" json:"site_group_id"`
	Name        string         `db:"name"         json:"name"`
	Description sql.NullString `db:"description"  json:"description"`
	IsDefault   int            `db:"is_default"   json:"is_default"`
	Status      int            `db:"status"       json:"status"`
	CreatedAt   time.Time      `db:"created_at"   json:"created_at"`
}

// ============================================
// 管理员和系统相关
// ============================================

// Admin represents an administrator from the database.
type Admin struct {
	ID        int          `db:"id"         json:"id"`
	Username  string       `db:"username"   json:"username"`
	Password  string       `db:"password"   json:"-"` // 密码不输出到 JSON
	LastLogin sql.NullTime `db:"last_login" json:"last_login"`
	CreatedAt time.Time    `db:"created_at" json:"created_at"`
}

// SystemLog represents a system log entry from the database.
type SystemLog struct {
	ID              int64           `db:"id"                json:"id"`
	Level           string          `db:"level"             json:"level"`
	Module          sql.NullString  `db:"module"            json:"module"`
	SpiderProjectID sql.NullInt64   `db:"spider_project_id" json:"spider_project_id"`
	Message         string          `db:"message"           json:"message"`
	Extra           json.RawMessage `db:"extra"             json:"extra"`
	CreatedAt       time.Time       `db:"created_at"        json:"created_at"`
}

// ContentGenerator represents a content generator from the database.
type ContentGenerator struct {
	ID          int            `db:"id"           json:"id"`
	Name        string         `db:"name"         json:"name"`
	DisplayName string         `db:"display_name" json:"display_name"`
	Description sql.NullString `db:"description"  json:"description"`
	Code        string         `db:"code"         json:"code"`
	Enabled     int            `db:"enabled"      json:"enabled"`
	IsDefault   int            `db:"is_default"   json:"is_default"`
	Version     int            `db:"version"      json:"version"`
	CreatedAt   time.Time      `db:"created_at"   json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"   json:"updated_at"`
}

// ============================================
// 定时任务相关
// ============================================

// ScheduledTask represents a scheduled task from the database.
type ScheduledTask struct {
	ID        int             `db:"id"          json:"id"`
	Name      string          `db:"name"        json:"name"`
	TaskType  string          `db:"task_type"   json:"task_type"`
	CronExpr  string          `db:"cron_expr"   json:"cron_expr"`
	Params    json.RawMessage `db:"params"      json:"params"`
	Enabled   int             `db:"enabled"     json:"enabled"`
	LastRunAt sql.NullTime    `db:"last_run_at" json:"last_run_at"`
	NextRunAt sql.NullTime    `db:"next_run_at" json:"next_run_at"`
	CreatedAt time.Time       `db:"created_at"  json:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"  json:"updated_at"`
}

// TaskLog represents a task execution log from the database.
type TaskLog struct {
	ID        int64          `db:"id"         json:"id"`
	TaskID    int            `db:"task_id"    json:"task_id"`
	Status    string         `db:"status"     json:"status"`
	Message   sql.NullString `db:"message"    json:"message"`
	Duration  sql.NullInt64  `db:"duration"   json:"duration"`
	StartedAt time.Time      `db:"started_at" json:"started_at"`
	EndedAt   sql.NullTime   `db:"ended_at"   json:"ended_at"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
}

// ============================================
// 站点和模板相关
// ============================================

// Site represents a site configuration from the database.
// Fields are grouped by: identifiers, configuration, optional relations, metadata, timestamps.
type Site struct {
	// Identifiers
	ID          int    `db:"id"           json:"id"`
	SiteGroupID int    `db:"site_group_id" json:"site_group_id"`
	Domain      string `db:"domain"       json:"domain"`
	Name        string `db:"name"         json:"name"`

	// Configuration
	Template string `db:"template" json:"template"`
	Status   int    `db:"status"   json:"status"`

	// Optional relations (nullable)
	KeywordGroupID sql.NullInt64 `db:"keyword_group_id" json:"keyword_group_id"`
	ImageGroupID   sql.NullInt64 `db:"image_group_id"   json:"image_group_id"`
	ArticleGroupID sql.NullInt64 `db:"article_group_id" json:"article_group_id"`

	// Optional metadata (nullable)
	ICPNumber  sql.NullString `db:"icp_number"   json:"icp_number"`
	BaiduToken sql.NullString `db:"baidu_token"  json:"baidu_token"`
	Analytics  sql.NullString `db:"analytics"    json:"analytics"`

	// Timestamps
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Template represents a page template from the database.
// Fields are grouped by: identifiers, display info, content, metadata, timestamps.
type Template struct {
	// Identifiers
	ID          int    `db:"id"           json:"id"`
	SiteGroupID int    `db:"site_group_id" json:"site_group_id"`
	Name        string `db:"name"         json:"name"`

	// Display info
	DisplayName string         `db:"display_name" json:"display_name"`
	Description sql.NullString `db:"description"  json:"description"`

	// Content
	Content string `db:"content" json:"content"`

	// Metadata
	Status  int `db:"status"  json:"status"`
	Version int `db:"version" json:"version"`

	// Timestamps
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// Keyword represents a keyword entry from the database.
type Keyword struct {
	ID        uint      `db:"id"         json:"id"`
	GroupID   int       `db:"group_id"   json:"group_id"`
	Keyword   string    `db:"keyword"    json:"keyword"`
	Status    int       `db:"status"     json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Image represents an image URL entry from the database.
type Image struct {
	ID        uint      `db:"id"         json:"id"`
	GroupID   int       `db:"group_id"   json:"group_id"`
	URL       string    `db:"url"        json:"url"`
	Status    int       `db:"status"     json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Title represents an article title from the database.
type Title struct {
	ID        uint64    `db:"id"         json:"id"`
	GroupID   int       `db:"group_id"   json:"group_id"`
	Title     string    `db:"title"      json:"title"`
	BatchID   int       `db:"batch_id"   json:"batch_id"`
	Status    int       `db:"status"     json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Content represents article content from the database.
type Content struct {
	ID        uint64    `db:"id"         json:"id"`
	GroupID   int       `db:"group_id"   json:"group_id"`
	Content   string    `db:"content"    json:"content"`
	BatchID   int       `db:"batch_id"   json:"batch_id"`
	Status    int       `db:"status"     json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// OriginalArticle represents an original article from the database.
type OriginalArticle struct {
	ID        uint           `db:"id"         json:"id"`
	GroupID   int            `db:"group_id"   json:"group_id"`
	SourceID  sql.NullInt64  `db:"source_id"  json:"source_id"`
	SourceURL sql.NullString `db:"source_url" json:"source_url"`
	Title     string         `db:"title"      json:"title"`
	Content   string         `db:"content"    json:"content"`
	Status    int            `db:"status"     json:"status"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt time.Time      `db:"updated_at" json:"updated_at"`
}

// SpiderLog represents a spider/crawler visit log entry.
type SpiderLog struct {
	ID         int64     `db:"id"          json:"id"`
	SpiderType string    `db:"spider_type" json:"spider_type"`
	Domain     string    `db:"domain"      json:"domain"`
	Path       string    `db:"path"        json:"path"`
	IP         string    `db:"ip"          json:"ip"`
	UA         string    `db:"ua"          json:"ua"`
	DNSOk      int       `db:"dns_ok"      json:"dns_ok"`
	RespTime   int       `db:"resp_time"   json:"resp_time"`
	CacheHit   int       `db:"cache_hit"   json:"cache_hit"`
	Status     int       `db:"status"      json:"status"`
	CreatedAt  time.Time `db:"created_at"  json:"created_at"`
}

// DetectionResult represents the result of spider detection.
type DetectionResult struct {
	IsSpider   bool   `json:"is_spider"`
	SpiderType string `json:"spider_type"`
	SpiderName string `json:"spider_name"`
	UserAgent  string `json:"user_agent"`
}

// RenderContext holds all data needed for template rendering.
type RenderContext struct {
	SiteID         int
	Title          string
	ArticleContent string
	AnalyticsCode  string
	BaiduPushJS    string
	Now            string
	Funcs          *TemplateFuncs
}

// TemplateFuncs holds function references for template rendering.
type TemplateFuncs struct {
	RandomKeyword func() string
	RandomURL     func() string
	RandomImage   func() string
	Content       func() string
	Cls           func(name string) string
	Encode        func(text string) string
	RandomNumber  func(min, max int) int
}
