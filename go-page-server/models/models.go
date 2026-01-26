// Package models defines data structures for the SEO page generator
package models

import (
	"database/sql"
	"time"
)

// Site represents a site configuration from the database
type Site struct {
	ID             int            `db:"id" json:"id"`
	SiteGroupID    int            `db:"site_group_id" json:"site_group_id"`
	Domain         string         `db:"domain" json:"domain"`
	Name           string         `db:"name" json:"name"`
	Template       string         `db:"template" json:"template"`
	KeywordGroupID sql.NullInt64  `db:"keyword_group_id" json:"keyword_group_id"`
	ImageGroupID   sql.NullInt64  `db:"image_group_id" json:"image_group_id"`
	ArticleGroupID sql.NullInt64  `db:"article_group_id" json:"article_group_id"`
	Status         int            `db:"status" json:"status"`
	ICPNumber      sql.NullString `db:"icp_number" json:"icp_number"`
	BaiduToken     sql.NullString `db:"baidu_token" json:"baidu_token"`
	Analytics      sql.NullString `db:"analytics" json:"analytics"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at"`
}

// Template represents a template from the database
type Template struct {
	ID          int            `db:"id" json:"id"`
	SiteGroupID int            `db:"site_group_id" json:"site_group_id"`
	Name        string         `db:"name" json:"name"`
	DisplayName string         `db:"display_name" json:"display_name"`
	Description sql.NullString `db:"description" json:"description"`
	Content     string         `db:"content" json:"content"`
	Status      int            `db:"status" json:"status"`
	Version     int            `db:"version" json:"version"`
	CreatedAt   time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at" json:"updated_at"`
}

// Keyword represents a keyword from the database
type Keyword struct {
	ID        uint      `db:"id" json:"id"`
	GroupID   int       `db:"group_id" json:"group_id"`
	Keyword   string    `db:"keyword" json:"keyword"`
	Status    int       `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Image represents an image URL from the database
type Image struct {
	ID        uint      `db:"id" json:"id"`
	GroupID   int       `db:"group_id" json:"group_id"`
	URL       string    `db:"url" json:"url"`
	Status    int       `db:"status" json:"status"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Title represents a title from the database
type Title struct {
	ID        uint64    `db:"id" json:"id"`
	GroupID   int       `db:"group_id" json:"group_id"`
	Title     string    `db:"title" json:"title"`
	BatchID   int       `db:"batch_id" json:"batch_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// Content represents a content from the database
type Content struct {
	ID        uint64    `db:"id" json:"id"`
	GroupID   int       `db:"group_id" json:"group_id"`
	Content   string    `db:"content" json:"content"`
	BatchID   int       `db:"batch_id" json:"batch_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// SpiderLog represents a spider visit log
type SpiderLog struct {
	ID         int64     `db:"id" json:"id"`
	SpiderType string    `db:"spider_type" json:"spider_type"`
	IP         string    `db:"ip" json:"ip"`
	UA         string    `db:"ua" json:"ua"`
	Domain     string    `db:"domain" json:"domain"`
	Path       string    `db:"path" json:"path"`
	DNSOk      int       `db:"dns_ok" json:"dns_ok"`
	RespTime   int       `db:"resp_time" json:"resp_time"`
	CacheHit   int       `db:"cache_hit" json:"cache_hit"`
	Status     int       `db:"status" json:"status"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
}

// DetectionResult represents the result of spider detection
type DetectionResult struct {
	IsSpider    bool   `json:"is_spider"`
	SpiderType  string `json:"spider_type"`
	SpiderName  string `json:"spider_name"`
	DNSVerified bool   `json:"dns_verified"`
	IP          string `json:"ip"`
	UserAgent   string `json:"user_agent"`
}

// RenderContext holds all data needed for template rendering
type RenderContext struct {
	Title           string
	SiteID          int
	AnalyticsCode   string
	BaiduPushJS     string
	ArticleContent  string
	Now             string
	Funcs           *TemplateFuncs
}

// TemplateFuncs holds function references for template rendering
type TemplateFuncs struct {
	RandomKeyword func() string
	Cls           func(name string) string
	RandomURL     func() string
	RandomImage   func() string
	Content       func() string
	Encode        func(text string) string
	RandomNumber  func(min, max int) int
}
