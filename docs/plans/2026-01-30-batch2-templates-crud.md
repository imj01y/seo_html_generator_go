# Batch 2: 模板 CRUD 实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现模板管理的 7 个 CRUD 接口，完成第一批基础模块

**Architecture:** Go Gin 框架，沿用 Batch 1 的认证中间件和响应格式，复用已有的 models.Template 结构

**Tech Stack:** Go 1.22+, Gin, sqlx, MySQL

---

## 前置条件

- Batch 1 已完成（auth, dashboard, logs 模块）
- 数据库表 `templates` 已存在
- models/models.go 已有 Template 结构体

---

## API 接口列表

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/templates | 获取模板列表（分页） |
| GET | /api/templates/options | 获取模板下拉选项 |
| GET | /api/templates/:id | 获取模板详情 |
| GET | /api/templates/:id/sites | 获取使用此模板的站点 |
| POST | /api/templates | 创建模板 |
| PUT | /api/templates/:id | 更新模板 |
| DELETE | /api/templates/:id | 删除模板 |

---

## Task 1: 创建模板模型和请求结构

**Files:**
- Create: `go-page-server/api/templates.go`

**Step 1: 创建基础文件和结构体**

```go
// go-page-server/api/templates.go
package api

import (
	"database/sql"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// TemplatesHandler 模板管理 handler
type TemplatesHandler struct {
	db *sqlx.DB
}

// NewTemplatesHandler 创建 TemplatesHandler
func NewTemplatesHandler(db *sqlx.DB) *TemplatesHandler {
	return &TemplatesHandler{db: db}
}

// TemplateListItem 模板列表项（不含 content）
type TemplateListItem struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Description *string   `json:"description" db:"description"`
	Status      int       `json:"status" db:"status"`
	Version     int       `json:"version" db:"version"`
	SitesCount  int       `json:"sites_count" db:"sites_count"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TemplateDetail 模板详情（含 content）
type TemplateDetail struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	DisplayName string    `json:"display_name" db:"display_name"`
	Description *string   `json:"description" db:"description"`
	Content     string    `json:"content" db:"content"`
	Status      int       `json:"status" db:"status"`
	Version     int       `json:"version" db:"version"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// TemplateOption 模板下拉选项
type TemplateOption struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	DisplayName string `json:"display_name" db:"display_name"`
}

// TemplateCreateRequest 创建模板请求
type TemplateCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	DisplayName string `json:"display_name" binding:"required"`
	Description string `json:"description"`
	Content     string `json:"content" binding:"required"`
}

// TemplateUpdateRequest 更新模板请求
type TemplateUpdateRequest struct {
	SiteGroupID *int    `json:"site_group_id"`
	DisplayName *string `json:"display_name"`
	Description *string `json:"description"`
	Content     *string `json:"content"`
	Status      *int    `json:"status"`
}

// TemplateSite 使用模板的站点
type TemplateSite struct {
	ID        int       `json:"id" db:"id"`
	Domain    string    `json:"domain" db:"domain"`
	Name      string    `json:"name" db:"name"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/templates.go
git commit -m "feat(api): 添加模板管理结构体定义"
```

---

## Task 2: 实现获取模板列表

**Files:**
- Modify: `go-page-server/api/templates.go`

**Step 1: 实现 List 方法**

在 templates.go 中添加：

```go
// List 获取模板列表
// GET /api/templates
func (h *TemplatesHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	siteGroupID := c.Query("site_group_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if h.db == nil {
		core.SuccessPaged(c, []TemplateListItem{}, 0, page, pageSize)
		return
	}

	// 构建查询条件
	where := "1=1"
	args := []interface{}{}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if siteGroupID != "" {
		where += " AND site_group_id = ?"
		args = append(args, siteGroupID)
	}

	// 获取总数
	var total int64
	countQuery := "SELECT COUNT(*) FROM templates WHERE " + where
	if err := h.db.Get(&total, countQuery, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to count templates")
	}

	// 获取列表
	query := `SELECT t.id, t.site_group_id, t.name, t.display_name, t.description,
	                 t.status, t.version, t.created_at, t.updated_at,
	                 (SELECT COUNT(*) FROM sites WHERE sites.template = t.name) as sites_count
	          FROM templates t
	          WHERE ` + where + `
	          ORDER BY t.id DESC
	          LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	var items []TemplateListItem
	if err := h.db.Select(&items, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to query templates")
		items = []TemplateListItem{}
	}

	core.SuccessPaged(c, items, total, page, pageSize)
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/templates.go
git commit -m "feat(api): 实现模板列表接口 GET /api/templates"
```

---

## Task 3: 实现获取模板选项和详情

**Files:**
- Modify: `go-page-server/api/templates.go`

**Step 1: 实现 Options 和 Get 方法**

```go
// Options 获取模板下拉选项
// GET /api/templates/options
func (h *TemplatesHandler) Options(c *gin.Context) {
	siteGroupID := c.Query("site_group_id")

	if h.db == nil {
		core.Success(c, gin.H{"options": []TemplateOption{}})
		return
	}

	var options []TemplateOption
	var err error

	if siteGroupID != "" {
		err = h.db.Select(&options,
			`SELECT id, name, display_name FROM templates
			 WHERE status = 1 AND (site_group_id = ? OR site_group_id = 1)
			 ORDER BY site_group_id DESC, name`,
			siteGroupID)
	} else {
		err = h.db.Select(&options,
			`SELECT id, name, display_name FROM templates
			 WHERE status = 1
			 ORDER BY name`)
	}

	if err != nil {
		log.Warn().Err(err).Msg("Failed to get template options")
		options = []TemplateOption{}
	}

	core.Success(c, gin.H{"options": options})
}

// Get 获取模板详情
// GET /api/templates/:id
func (h *TemplatesHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的模板 ID")
		return
	}

	if h.db == nil {
		core.FailWithCode(c, core.ErrNotFound)
		return
	}

	var template TemplateDetail
	err = h.db.Get(&template,
		`SELECT id, site_group_id, name, display_name, description, content,
		        status, version, created_at, updated_at
		 FROM templates WHERE id = ?`, id)

	if err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "模板不存在")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("Failed to get template")
		core.FailWithCode(c, core.ErrInternalServer)
		return
	}

	core.Success(c, template)
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/templates.go
git commit -m "feat(api): 实现模板选项和详情接口"
```

---

## Task 4: 实现获取模板关联站点

**Files:**
- Modify: `go-page-server/api/templates.go`

**Step 1: 实现 GetSites 方法**

```go
// GetSites 获取使用此模板的站点
// GET /api/templates/:id/sites
func (h *TemplatesHandler) GetSites(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的模板 ID")
		return
	}

	if h.db == nil {
		core.FailWithCode(c, core.ErrNotFound)
		return
	}

	// 先获取模板名称
	var templateName string
	err = h.db.Get(&templateName, "SELECT name FROM templates WHERE id = ?", id)
	if err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "模板不存在")
			return
		}
		core.FailWithCode(c, core.ErrInternalServer)
		return
	}

	// 获取使用此模板的站点
	var sites []TemplateSite
	err = h.db.Select(&sites,
		`SELECT id, domain, name, status, created_at
		 FROM sites WHERE template = ?
		 ORDER BY id DESC`, templateName)

	if err != nil {
		log.Warn().Err(err).Str("template", templateName).Msg("Failed to get template sites")
		sites = []TemplateSite{}
	}

	core.Success(c, gin.H{
		"sites":         sites,
		"template_name": templateName,
	})
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/templates.go
git commit -m "feat(api): 实现获取模板关联站点接口"
```

---

## Task 5: 实现创建模板

**Files:**
- Modify: `go-page-server/api/templates.go`

**Step 1: 实现 Create 方法**

```go
// Create 创建模板
// POST /api/templates
func (h *TemplatesHandler) Create(c *gin.Context) {
	var req TemplateCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	result, err := h.db.Exec(
		`INSERT INTO templates (site_group_id, name, display_name, description, content, status, version)
		 VALUES (?, ?, ?, ?, ?, 1, 1)`,
		req.SiteGroupID, req.Name, req.DisplayName, req.Description, req.Content)

	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "该站群内模板标识名已存在"})
			return
		}
		log.Error().Err(err).Msg("Failed to create template")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}
```

**Step 2: 添加 strings 包导入**

在文件顶部导入中添加 `"strings"`

**Step 3: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/api/templates.go
git commit -m "feat(api): 实现创建模板接口 POST /api/templates"
```

---

## Task 6: 实现更新和删除模板

**Files:**
- Modify: `go-page-server/api/templates.go`

**Step 1: 实现 Update 方法**

```go
// Update 更新模板
// PUT /api/templates/:id
func (h *TemplatesHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的模板 ID")
		return
	}

	var req TemplateUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查模板是否存在
	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM templates WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "模板不存在"})
		return
	}

	// 构建更新语句
	updates := []string{}
	args := []interface{}{}

	if req.SiteGroupID != nil {
		updates = append(updates, "site_group_id = ?")
		args = append(args, *req.SiteGroupID)
	}
	if req.DisplayName != nil {
		updates = append(updates, "display_name = ?")
		args = append(args, *req.DisplayName)
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Content != nil {
		updates = append(updates, "content = ?")
		args = append(args, *req.Content)
		updates = append(updates, "version = version + 1")
	}
	if req.Status != nil {
		updates = append(updates, "status = ?")
		args = append(args, *req.Status)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "没有需要更新的字段"})
		return
	}

	args = append(args, id)
	query := "UPDATE templates SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		log.Error().Err(err).Int("id", id).Msg("Failed to update template")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}
```

**Step 2: 实现 Delete 方法**

```go
// Delete 删除模板
// DELETE /api/templates/:id
func (h *TemplatesHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的模板 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 获取模板名称
	var templateName string
	if err := h.db.Get(&templateName, "SELECT name FROM templates WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "模板不存在"})
		return
	}

	// 检查是否有站点在使用
	var sitesCount int
	h.db.Get(&sitesCount, "SELECT COUNT(*) FROM sites WHERE template = ? AND status = 1", templateName)

	if sitesCount > 0 {
		core.Success(c, gin.H{
			"success": false,
			"message": fmt.Sprintf("无法删除：有 %d 个站点正在使用此模板", sitesCount),
		})
		return
	}

	// 执行删除
	if _, err := h.db.Exec("DELETE FROM templates WHERE id = ?", id); err != nil {
		log.Error().Err(err).Int("id", id).Msg("Failed to delete template")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}
```

**Step 3: 添加 fmt 包导入**

在文件顶部导入中添加 `"fmt"`

**Step 4: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 5: Commit**

```bash
git add go-page-server/api/templates.go
git commit -m "feat(api): 实现更新和删除模板接口"
```

---

## Task 7: 注册模板路由

**Files:**
- Modify: `go-page-server/api/router.go`

**Step 1: 在 router.go 中注册模板路由**

在 `SetupRouter` 函数中，logs 路由之后添加：

```go
// Templates routes (require JWT)
templatesHandler := NewTemplatesHandler(deps.DB)
templatesGroup := r.Group("/api/templates")
templatesGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
{
	templatesGroup.GET("", templatesHandler.List)
	templatesGroup.GET("/options", templatesHandler.Options)
	templatesGroup.GET("/:id", templatesHandler.Get)
	templatesGroup.GET("/:id/sites", templatesHandler.GetSites)
	templatesGroup.POST("", templatesHandler.Create)
	templatesGroup.PUT("/:id", templatesHandler.Update)
	templatesGroup.DELETE("/:id", templatesHandler.Delete)
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/router.go
git commit -m "feat(api): 注册模板 CRUD 路由"
```

---

## Task 8: 添加模板 API 测试

**Files:**
- Create: `go-page-server/api/templates_test.go`

**Step 1: 创建测试文件**

```go
// go-page-server/api/templates_test.go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestTemplatesList_Unauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(AuthMiddleware("test-secret"))

	handler := &TemplatesHandler{}
	r.GET("/api/templates", handler.List)

	req := httptest.NewRequest("GET", "/api/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d", w.Code)
	}
}

func TestTemplatesOptions_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates/options", handler.Options)

	req := httptest.NewRequest("GET", "/api/templates/options", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp["code"].(float64) != 0 {
		t.Fatalf("Expected code 0, got %v", resp["code"])
	}

	data := resp["data"].(map[string]interface{})
	if _, ok := data["options"]; !ok {
		t.Error("Response should contain options field")
	}
}

func TestTemplatesGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &TemplatesHandler{}
	r.GET("/api/templates/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/templates/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}
```

**Step 2: 运行测试**

Run: `cd go-page-server && go test ./api -run TestTemplates -v`
Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/api/templates_test.go
git commit -m "test(api): 添加模板 API 测试"
```

---

## Task 9: 集成测试

**Step 1: 运行所有 API 测试**

Run: `cd go-page-server && go test ./api/... -v`
Expected: All tests PASS

**Step 2: 运行完整项目测试**

Run: `cd go-page-server && go test ./... -v`
Expected: All tests PASS

**Step 3: 验证编译**

Run: `cd go-page-server && go build -o go-page-server.exe . && echo "Build successful"`
Expected: Build successful

**Step 4: 最终 Commit**

```bash
git add -A
git commit -m "feat: Batch 2 完成 - 模板 CRUD 模块 (7 接口)"
```

---

## 完成检查清单

- [ ] Task 1: 创建模板结构体定义
- [ ] Task 2: 实现获取模板列表
- [ ] Task 3: 实现获取模板选项和详情
- [ ] Task 4: 实现获取模板关联站点
- [ ] Task 5: 实现创建模板
- [ ] Task 6: 实现更新和删除模板
- [ ] Task 7: 注册模板路由
- [ ] Task 8: 添加模板 API 测试
- [ ] Task 9: 集成测试

---

*文档创建时间: 2026-01-30*
