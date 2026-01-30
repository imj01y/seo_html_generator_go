# Batch 5: 文章管理模块实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现文章管理的 14 个 API 接口，包括分组管理、CRUD、批量操作

**Architecture:** Go Gin 框架，沿用已有认证中间件和响应格式

**Tech Stack:** Go 1.22+, Gin, sqlx, MySQL

---

## API 接口列表 (14个)

### 分组管理 (4个)
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/articles/groups | 获取分组列表 |
| POST | /api/articles/groups | 创建分组 |
| PUT | /api/articles/groups/:id | 更新分组 |
| DELETE | /api/articles/groups/:id | 删除分组 |

### 文章 CRUD (4个)
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/articles/list | 获取文章列表（分页） |
| GET | /api/articles/:id | 获取单篇文章 |
| PUT | /api/articles/:id | 更新文章 |
| DELETE | /api/articles/:id | 删除文章 |

### 批量操作 (4个)
| 方法 | 路径 | 功能 |
|------|------|------|
| DELETE | /api/articles/batch/delete | 批量删除 |
| DELETE | /api/articles/delete-all | 删除全部 |
| PUT | /api/articles/batch/status | 批量更新状态 |
| PUT | /api/articles/batch/move | 批量移动分组 |

### 添加文章 (2个)
| 方法 | 路径 | 功能 |
|------|------|------|
| POST | /api/articles/add | 添加单篇文章 |
| POST | /api/articles/batch | 批量添加文章 |

---

## Task 1: 创建文章模块基础结构

**Files:**
- Create: `go-page-server/api/articles.go`

```go
package api

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// ArticlesHandler 文章管理 handler
type ArticlesHandler struct {
	db *sqlx.DB
}

// NewArticlesHandler 创建 ArticlesHandler
func NewArticlesHandler(db *sqlx.DB) *ArticlesHandler {
	return &ArticlesHandler{db: db}
}

// ArticleGroup 文章分组
type ArticleGroup struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ArticleListItem 文章列表项
type ArticleListItem struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	Title     string    `json:"title" db:"title"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ArticleDetail 文章详情
type ArticleDetail struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ArticleGroupCreateRequest 创建分组请求
type ArticleGroupCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// ArticleGroupUpdateRequest 更新分组请求
type ArticleGroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *int    `json:"is_default"`
}

// ArticleUpdateRequest 更新文章请求
type ArticleUpdateRequest struct {
	GroupID *int    `json:"group_id"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
	Status  *int    `json:"status"`
}

// ArticleBatchIdsRequest 批量ID请求
type ArticleBatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// ArticleBatchStatusRequest 批量状态更新请求
type ArticleBatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// ArticleBatchMoveRequest 批量移动请求
type ArticleBatchMoveRequest struct {
	IDs     []int `json:"ids" binding:"required"`
	GroupID int   `json:"group_id" binding:"required"`
}

// ArticleDeleteAllRequest 删除全部请求
type ArticleDeleteAllRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
	GroupID *int `json:"group_id"`
}

// ArticleAddRequest 添加单篇文章请求
type ArticleAddRequest struct {
	GroupID int    `json:"group_id"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// ArticleBatchAddItem 批量添加文章项
type ArticleBatchAddItem struct {
	GroupID int    `json:"group_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ArticleBatchAddRequest 批量添加文章请求
type ArticleBatchAddRequest struct {
	Articles []ArticleBatchAddItem `json:"articles" binding:"required"`
}

// 确保导入包被使用
var (
	_ = sql.ErrNoRows
	_ = fmt.Sprintf
	_ = strconv.Atoi
	_ = strings.TrimSpace
	_ = log.Info
	_ = core.Success
)
```

**验证:** `go build ./...`
**提交:** `git commit -m "feat(api): 添加文章模块结构体定义"`

---

## Task 2: 实现分组管理接口

删除临时变量，添加 4 个分组 CRUD 方法：

```go
// ListGroups 获取分组列表
func (h *ArticlesHandler) ListGroups(c *gin.Context) {
	siteGroupID := c.Query("site_group_id")

	if h.db == nil {
		core.Success(c, gin.H{"groups": []ArticleGroup{}})
		return
	}

	where := "status = 1"
	args := []interface{}{}

	if siteGroupID != "" {
		where += " AND site_group_id = ?"
		args = append(args, siteGroupID)
	}

	query := `SELECT id, site_group_id, name, description, is_default, status, created_at
	          FROM article_groups WHERE ` + where + ` ORDER BY is_default DESC, name`

	var groups []ArticleGroup
	if err := h.db.Select(&groups, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list article groups")
		groups = []ArticleGroup{}
	}

	core.Success(c, gin.H{"groups": groups})
}

// CreateGroup 创建分组
func (h *ArticlesHandler) CreateGroup(c *gin.Context) {
	var req ArticleGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	if req.IsDefault {
		h.db.Exec("UPDATE article_groups SET is_default = 0 WHERE is_default = 1")
	}

	isDefault := 0
	if req.IsDefault {
		isDefault = 1
	}

	result, err := h.db.Exec(
		`INSERT INTO article_groups (site_group_id, name, description, is_default)
		 VALUES (?, ?, ?, ?)`,
		req.SiteGroupID, req.Name, req.Description, isDefault)

	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "分组名称已存在"})
			return
		}
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}

// UpdateGroup 更新分组
func (h *ArticlesHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的分组 ID")
		return
	}

	var req ArticleGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM article_groups WHERE id = ? AND status = 1", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "分组不存在"})
		return
	}

	updates := []string{}
	args := []interface{}{}

	if req.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *req.Description)
	}
	if req.IsDefault != nil {
		if *req.IsDefault == 1 {
			h.db.Exec("UPDATE article_groups SET is_default = 0 WHERE is_default = 1")
		}
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	query := "UPDATE article_groups SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// DeleteGroup 删除分组
func (h *ArticlesHandler) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的分组 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var isDefault int
	h.db.Get(&isDefault, "SELECT is_default FROM article_groups WHERE id = ?", id)
	if isDefault == 1 {
		core.Success(c, gin.H{"success": false, "message": "不能删除默认分组"})
		return
	}

	if _, err := h.db.Exec("UPDATE article_groups SET status = 0 WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}
```

**提交:** `git commit -m "feat(api): 实现文章分组管理接口"`

---

## Task 3: 实现文章 CRUD 接口

添加 4 个文章 CRUD 方法：

```go
// List 获取文章列表
func (h *ArticlesHandler) List(c *gin.Context) {
	groupID, _ := strconv.Atoi(c.DefaultQuery("group_id", "1"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if h.db == nil {
		core.SuccessPaged(c, []ArticleListItem{}, 0, page, pageSize)
		return
	}

	where := "group_id = ? AND status = 1"
	args := []interface{}{groupID}

	if search != "" {
		where += " AND (title LIKE ? OR content LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM original_articles WHERE "+where, args...)

	args = append(args, pageSize, offset)
	query := `SELECT id, group_id, title, status, created_at
	          FROM original_articles WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	var items []ArticleListItem
	if err := h.db.Select(&items, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list articles")
		items = []ArticleListItem{}
	}

	core.SuccessPaged(c, items, total, page, pageSize)
}

// Get 获取单篇文章
func (h *ArticlesHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的文章 ID")
		return
	}

	if h.db == nil {
		core.FailWithCode(c, core.ErrNotFound)
		return
	}

	var article ArticleDetail
	err = h.db.Get(&article,
		`SELECT id, group_id, title, content, status, created_at, updated_at
		 FROM original_articles WHERE id = ?`, id)

	if err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "文章不存在")
			return
		}
		core.FailWithCode(c, core.ErrInternalServer)
		return
	}

	core.Success(c, article)
}

// Update 更新文章
func (h *ArticlesHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的文章 ID")
		return
	}

	var req ArticleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM original_articles WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "文章不存在"})
		return
	}

	updates := []string{}
	args := []interface{}{}

	if req.GroupID != nil {
		updates = append(updates, "group_id = ?")
		args = append(args, *req.GroupID)
	}
	if req.Title != nil {
		updates = append(updates, "title = ?")
		args = append(args, *req.Title)
	}
	if req.Content != nil {
		updates = append(updates, "content = ?")
		args = append(args, *req.Content)
	}
	if req.Status != nil {
		updates = append(updates, "status = ?")
		args = append(args, *req.Status)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": false, "message": "没有要更新的字段"})
		return
	}

	args = append(args, id)
	query := "UPDATE original_articles SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// Delete 删除文章
func (h *ArticlesHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的文章 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	if _, err := h.db.Exec("UPDATE original_articles SET status = 0 WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}
```

**提交:** `git commit -m "feat(api): 实现文章 CRUD 接口"`

---

## Task 4: 实现批量操作接口

添加 4 个批量操作方法：

```go
// BatchDelete 批量删除
func (h *ArticlesHandler) BatchDelete(c *gin.Context) {
	var req ArticleBatchIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Success(c, gin.H{"success": false, "message": "ID列表不能为空", "deleted": 0})
		return
	}

	if len(req.IDs) == 0 {
		core.Success(c, gin.H{"success": false, "message": "ID列表不能为空", "deleted": 0})
		return
	}

	if h.db == nil {
		core.Success(c, gin.H{"success": false, "message": "数据库未初始化", "deleted": 0})
		return
	}

	placeholders := strings.Repeat("?,", len(req.IDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := make([]interface{}, len(req.IDs))
	for i, id := range req.IDs {
		args[i] = id
	}

	query := fmt.Sprintf("UPDATE original_articles SET status = 0 WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "deleted": len(req.IDs)})
}

// DeleteAll 删除全部
func (h *ArticlesHandler) DeleteAll(c *gin.Context) {
	var req ArticleDeleteAllRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Success(c, gin.H{"success": false, "message": "请求参数错误", "deleted": 0})
		return
	}

	if !req.Confirm {
		core.Success(c, gin.H{"success": false, "message": "请确认删除操作", "deleted": 0})
		return
	}

	if h.db == nil {
		core.Success(c, gin.H{"success": false, "message": "数据库未初始化", "deleted": 0})
		return
	}

	var result sql.Result
	var err error

	if req.GroupID != nil {
		result, err = h.db.Exec("UPDATE original_articles SET status = 0 WHERE group_id = ? AND status = 1", *req.GroupID)
	} else {
		result, err = h.db.Exec("UPDATE original_articles SET status = 0 WHERE status = 1")
	}

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	deleted, _ := result.RowsAffected()
	core.Success(c, gin.H{"success": true, "deleted": deleted})
}

// BatchUpdateStatus 批量更新状态
func (h *ArticlesHandler) BatchUpdateStatus(c *gin.Context) {
	var req ArticleBatchStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Success(c, gin.H{"success": false, "message": "请求参数错误", "updated": 0})
		return
	}

	if len(req.IDs) == 0 {
		core.Success(c, gin.H{"success": false, "message": "ID列表不能为空", "updated": 0})
		return
	}

	if h.db == nil {
		core.Success(c, gin.H{"success": false, "message": "数据库未初始化", "updated": 0})
		return
	}

	placeholders := strings.Repeat("?,", len(req.IDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := []interface{}{req.Status}
	for _, id := range req.IDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE original_articles SET status = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "updated": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "updated": len(req.IDs)})
}

// BatchMove 批量移动分组
func (h *ArticlesHandler) BatchMove(c *gin.Context) {
	var req ArticleBatchMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.Success(c, gin.H{"success": false, "message": "请求参数错误", "moved": 0})
		return
	}

	if len(req.IDs) == 0 {
		core.Success(c, gin.H{"success": false, "message": "ID列表不能为空", "moved": 0})
		return
	}

	if h.db == nil {
		core.Success(c, gin.H{"success": false, "message": "数据库未初始化", "moved": 0})
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM article_groups WHERE id = ? AND status = 1", req.GroupID); err != nil {
		core.Success(c, gin.H{"success": false, "message": "目标分组不存在", "moved": 0})
		return
	}

	placeholders := strings.Repeat("?,", len(req.IDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := []interface{}{req.GroupID}
	for _, id := range req.IDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE original_articles SET group_id = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "moved": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "moved": len(req.IDs)})
}
```

**提交:** `git commit -m "feat(api): 实现文章批量操作接口"`

---

## Task 5: 实现添加文章接口

添加 2 个添加文章方法：

```go
// Add 添加单篇文章
func (h *ArticlesHandler) Add(c *gin.Context) {
	var req ArticleAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	groupID := req.GroupID
	if groupID == 0 {
		groupID = 1
	}

	result, err := h.db.Exec(
		"INSERT IGNORE INTO original_articles (group_id, title, content) VALUES (?, ?, ?)",
		groupID, req.Title, req.Content)

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		core.Success(c, gin.H{"success": false, "message": "文章标题已存在"})
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}

// BatchAdd 批量添加文章
func (h *ArticlesHandler) BatchAdd(c *gin.Context) {
	var req ArticleBatchAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if len(req.Articles) == 0 {
		core.Success(c, gin.H{"success": false, "message": "文章列表不能为空"})
		return
	}

	if len(req.Articles) > 1000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "单次最多添加 1000 篇文章")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	added := 0
	skipped := 0

	for _, article := range req.Articles {
		if article.Title == "" || article.Content == "" {
			skipped++
			continue
		}

		groupID := article.GroupID
		if groupID == 0 {
			groupID = 1
		}

		result, err := h.db.Exec(
			"INSERT IGNORE INTO original_articles (group_id, title, content) VALUES (?, ?, ?)",
			groupID, article.Title, article.Content)
		if err != nil {
			skipped++
			continue
		}

		affected, _ := result.RowsAffected()
		if affected > 0 {
			added++
		} else {
			skipped++
		}
	}

	core.Success(c, gin.H{
		"success": true,
		"added":   added,
		"skipped": skipped,
		"total":   len(req.Articles),
	})
}
```

**提交:** `git commit -m "feat(api): 实现文章添加接口"`

---

## Task 6: 注册文章路由

在 router.go 中 images 路由之后添加：

```go
// Articles routes (require JWT)
articlesHandler := NewArticlesHandler(deps.DB)
articlesGroup := r.Group("/api/articles")
articlesGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
{
	// 分组管理
	articlesGroup.GET("/groups", articlesHandler.ListGroups)
	articlesGroup.POST("/groups", articlesHandler.CreateGroup)
	articlesGroup.PUT("/groups/:id", articlesHandler.UpdateGroup)
	articlesGroup.DELETE("/groups/:id", articlesHandler.DeleteGroup)

	// 文章 CRUD
	articlesGroup.GET("/list", articlesHandler.List)
	articlesGroup.GET("/:id", articlesHandler.Get)
	articlesGroup.PUT("/:id", articlesHandler.Update)
	articlesGroup.DELETE("/:id", articlesHandler.Delete)

	// 批量操作
	articlesGroup.DELETE("/batch/delete", articlesHandler.BatchDelete)
	articlesGroup.DELETE("/delete-all", articlesHandler.DeleteAll)
	articlesGroup.PUT("/batch/status", articlesHandler.BatchUpdateStatus)
	articlesGroup.PUT("/batch/move", articlesHandler.BatchMove)

	// 添加文章
	articlesGroup.POST("/add", articlesHandler.Add)
	articlesGroup.POST("/batch", articlesHandler.BatchAdd)
}
```

**提交:** `git commit -m "feat(api): 注册文章管理路由 (14个接口)"`

---

## Task 7: 添加文章 API 测试

创建 `go-page-server/api/articles_test.go`：

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestArticlesListGroups_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.GET("/api/articles/groups", handler.ListGroups)

	req := httptest.NewRequest("GET", "/api/articles/groups", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["code"].(float64) != 0 {
		t.Fatalf("Expected code 0, got %v", resp["code"])
	}
}

func TestArticlesList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.GET("/api/articles/list", handler.List)

	req := httptest.NewRequest("GET", "/api/articles/list?group_id=1&page=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestArticlesAdd_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.POST("/api/articles/add", handler.Add)

	body := `{"title": "test", "content": "test content", "group_id": 1}`
	req := httptest.NewRequest("POST", "/api/articles/add", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected 500, got %d", w.Code)
	}
}

func TestArticlesBatchDelete_EmptyIds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.DELETE("/api/articles/batch/delete", handler.BatchDelete)

	body := `{"ids": []}`
	req := httptest.NewRequest("DELETE", "/api/articles/batch/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})
	if data["success"].(bool) != false {
		t.Fatal("Expected success: false for empty ids")
	}
}

func TestArticlesGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	handler := &ArticlesHandler{}
	r.GET("/api/articles/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/articles/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}
```

**运行测试:** `go test ./api -run TestArticles -v`
**提交:** `git commit -m "test(api): 添加文章 API 测试"`

---

## Task 8: 集成测试

**运行所有测试:** `go test ./... -v`
**验证编译:** `go build -o go-page-server.exe .`

---

## 完成检查清单

- [ ] Task 1: 创建文章模块基础结构
- [ ] Task 2: 实现分组管理接口
- [ ] Task 3: 实现文章 CRUD 接口
- [ ] Task 4: 实现批量操作接口
- [ ] Task 5: 实现添加文章接口
- [ ] Task 6: 注册文章路由
- [ ] Task 7: 添加文章 API 测试
- [ ] Task 8: 集成测试

---

*文档创建时间: 2026-01-30*
