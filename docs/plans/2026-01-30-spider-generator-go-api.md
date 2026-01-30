# 爬虫与生成器 Go API + Python Worker 实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将爬虫项目和生成器管理的 API 层迁移到 Go，保留 Python Worker 执行实际爬虫代码

**Architecture:** Go Gin 处理 HTTP API + WebSocket，通过 Redis 发送命令给 Python Worker 执行

**Tech Stack:** Go 1.22+, Gin, gorilla/websocket, go-redis, sqlx, MySQL

---

## 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                      Go API Server                               │
│  ┌────────────────┐  ┌────────────────┐  ┌──────────────────┐   │
│  │ 项目 CRUD API  │  │ 任务控制 API   │  │ WebSocket Server │   │
│  │ /api/spiders   │  │ run/stop/test  │  │ /ws/logs/:id     │   │
│  └────────────────┘  └────────────────┘  └──────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
           │                    │                    ▲
           ▼                    ▼                    │
┌─────────────────────────────────────────────────────────────────┐
│                         Redis                                    │
│  ├─ spider:commands       ← Go 发布运行/停止命令                  │
│  ├─ spider:logs:{id}      → Go 订阅，推送 WebSocket               │
│  └─ spider:status:{id}    ← Python 更新状态                      │
└─────────────────────────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                   Python Worker (复用现有核心)                    │
│  CommandListener → ProjectLoader → ProjectRunner → 数据入库      │
└─────────────────────────────────────────────────────────────────┘
```

---

## API 接口列表

### 爬虫项目管理 (28个接口)

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/spider-projects | 获取项目列表 |
| POST | /api/spider-projects | 创建项目 |
| GET | /api/spider-projects/:id | 获取项目详情 |
| PUT | /api/spider-projects/:id | 更新项目 |
| DELETE | /api/spider-projects/:id | 删除项目 |
| POST | /api/spider-projects/:id/toggle | 切换启用状态 |
| GET | /api/spider-projects/:id/files | 获取项目文件列表 |
| GET | /api/spider-projects/:id/files/:filename | 获取文件内容 |
| POST | /api/spider-projects/:id/files | 创建文件 |
| PUT | /api/spider-projects/:id/files/:filename | 更新文件 |
| DELETE | /api/spider-projects/:id/files/:filename | 删除文件 |
| POST | /api/spider-projects/:id/run | 运行项目 |
| POST | /api/spider-projects/:id/test | 测试运行 |
| POST | /api/spider-projects/:id/test/stop | 停止测试 |
| POST | /api/spider-projects/:id/stop | 停止项目 |
| POST | /api/spider-projects/:id/pause | 暂停项目 |
| POST | /api/spider-projects/:id/resume | 恢复项目 |
| GET | /api/spider-projects/:id/stats/realtime | 实时统计 |
| GET | /api/spider-projects/:id/stats/chart | 历史图表 |
| POST | /api/spider-projects/:id/queue/clear | 清空队列 |
| POST | /api/spider-projects/:id/reset | 重置项目 |
| GET | /api/spider-projects/:id/failed | 失败请求列表 |
| GET | /api/spider-projects/:id/failed/stats | 失败统计 |
| POST | /api/spider-projects/:id/failed/retry-all | 重试所有失败 |
| POST | /api/spider-projects/:id/failed/:fid/retry | 重试单个失败 |
| POST | /api/spider-projects/:id/failed/:fid/ignore | 忽略失败 |
| DELETE | /api/spider-projects/:id/failed/:fid | 删除失败记录 |
| GET | /api/spider-projects/templates | 代码模板列表 |

### 爬虫统计 (4个接口)

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/spider-stats/overview | 统计概览 |
| GET | /api/spider-stats/chart | 图表数据 |
| GET | /api/spider-stats/scheduled | 已调度项目 |
| GET | /api/spider-stats/by-project | 按项目统计 |

### 生成器管理 (10个接口)

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/generators | 获取生成器列表 |
| POST | /api/generators | 创建生成器 |
| GET | /api/generators/:id | 获取生成器详情 |
| PUT | /api/generators/:id | 更新生成器 |
| DELETE | /api/generators/:id | 删除生成器 |
| POST | /api/generators/:id/set-default | 设为默认 |
| POST | /api/generators/:id/toggle | 切换启用 |
| POST | /api/generators/test | 测试代码 |
| GET | /api/generators/templates/list | 代码模板 |
| POST | /api/generators/:id/reload | 热重载 |

### WebSocket (1个)

| 路径 | 功能 |
|------|------|
| /ws/spider-logs/:id | 实时日志推送 |

---

## Task 1: 创建爬虫项目基础结构体

**Files:**
- Create: `go-page-server/api/spiders.go`

**Step 1: 创建结构体和 Handler**

```go
package api

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
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
	Filename  string    `db:"filename" json:"filename"`
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

// SpidersHandler 爬虫处理器
type SpidersHandler struct{}

// 默认爬虫代码模板
const defaultSpiderCode = `from loguru import logger
import requests

def main():
    """
    爬虫入口函数

    数据格式（必填字段）：
    - title: 文章标题
    - content: 文章内容

    可选字段：source_url, author, publish_date, summary, cover_image, tags

    点击右上角 [指南] 查看完整文档
    """
    # 示例：API 分页抓取
    for page in range(1, 10):
        try:
            resp = requests.get(f'https://api.example.com/list?page={page}', timeout=10)
            resp.raise_for_status()
            data = resp.json()
        except Exception as e:
            logger.error(f'请求失败: {e}')
            break

        if not data.get('list'):
            break

        for item in data['list']:
            yield {
                'title': item['title'],           # 必填
                'content': item['content'],       # 必填
                'source_url': item.get('url'),    # 可选
                'author': item.get('author'),     # 可选
            }

        logger.info(f'第 {page} 页完成')


# 本地测试（可选）
if __name__ == '__main__':
    for item in main():
        print(f"标题: {item['title']}")
`
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): add spider project structs and handler"
```

---

## Task 2: 实现爬虫项目 CRUD API

**Files:**
- Modify: `go-page-server/api/spiders.go`

**Step 1: 实现 List 方法**

```go
// List 获取项目列表
func (h *SpidersHandler) List(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	// 解析查询参数
	status := c.Query("status")
	enabledStr := c.Query("enabled")
	search := c.Query("search")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 构建查询条件
	where := "1=1"
	args := []interface{}{}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if enabledStr != "" {
		enabled, _ := strconv.Atoi(enabledStr)
		where += " AND enabled = ?"
		args = append(args, enabled)
	}
	if search != "" {
		where += " AND (name LIKE ? OR description LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	// 查询总数
	var total int
	countSQL := "SELECT COUNT(*) FROM spider_projects WHERE " + where
	db.Get(&total, countSQL, args...)

	// 查询数据
	offset := (page - 1) * pageSize
	dataSQL := `
		SELECT id, name, description, entry_file, entry_function, start_url,
		       config, concurrency, output_group_id, schedule, enabled, status,
		       last_run_at, last_run_duration, last_run_items, last_error,
		       total_runs, total_items, created_at, updated_at
		FROM spider_projects
		WHERE ` + where + `
		ORDER BY id DESC
		LIMIT ? OFFSET ?
	`
	args = append(args, pageSize, offset)

	var projects []SpiderProject
	db.Select(&projects, dataSQL, args...)

	// 处理 config JSON
	for i := range projects {
		if projects[i].Config != nil {
			projects[i].ConfigParsed = json.RawMessage(*projects[i].Config)
		}
	}

	c.JSON(200, gin.H{
		"success":   true,
		"data":      projects,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
```

**Step 2: 实现 Get 方法**

```go
// Get 获取项目详情
func (h *SpidersHandler) Get(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var project SpiderProject
	err = db.Get(&project, `
		SELECT id, name, description, entry_file, entry_function, start_url,
		       config, concurrency, output_group_id, schedule, enabled, status,
		       last_run_at, last_run_duration, last_run_items, last_error,
		       total_runs, total_items, created_at, updated_at
		FROM spider_projects WHERE id = ?
	`, id)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	if project.Config != nil {
		project.ConfigParsed = json.RawMessage(*project.Config)
	}

	c.JSON(200, gin.H{"success": true, "data": project})
}
```

**Step 3: 实现 Create 方法**

```go
// Create 创建项目
func (h *SpidersHandler) Create(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	var req SpiderProjectCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 设置默认值
	if req.EntryFile == "" {
		req.EntryFile = "spider.py"
	}
	if req.EntryFunction == "" {
		req.EntryFunction = "main"
	}
	if req.Concurrency == 0 {
		req.Concurrency = 3
	}
	if req.OutputGroupID == 0 {
		req.OutputGroupID = 1
	}

	// 序列化 config
	var configJSON *string
	if req.Config != nil {
		configBytes, _ := json.Marshal(req.Config)
		configStr := string(configBytes)
		configJSON = &configStr
	}

	// 插入项目
	result, err := db.Exec(`
		INSERT INTO spider_projects
		(name, description, entry_file, entry_function, start_url, config,
		 concurrency, output_group_id, schedule, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.Name, req.Description, req.EntryFile, req.EntryFunction,
		req.StartURL, configJSON, req.Concurrency, req.OutputGroupID,
		req.Schedule, req.Enabled)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建失败: " + err.Error()})
		return
	}

	projectID, _ := result.LastInsertId()

	// 创建默认入口文件
	hasEntryFile := false
	for _, f := range req.Files {
		if f.Filename == req.EntryFile {
			hasEntryFile = true
			break
		}
	}

	if !hasEntryFile {
		req.Files = append(req.Files, SpiderFileCreate{
			Filename: req.EntryFile,
			Content:  defaultSpiderCode,
		})
	}

	// 创建文件
	for _, f := range req.Files {
		db.Exec(`
			INSERT INTO spider_project_files (project_id, filename, content)
			VALUES (?, ?, ?)
		`, projectID, f.Filename, f.Content)
	}

	c.JSON(200, gin.H{"success": true, "id": projectID, "message": "创建成功"})
}
```

**Step 4: 实现 Update 和 Delete 方法**

```go
// Update 更新项目
func (h *SpidersHandler) Update(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查项目是否存在
	var status string
	err = db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法修改"})
		return
	}

	var req SpiderProjectUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 动态构建更新语句
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
	if req.EntryFile != nil {
		updates = append(updates, "entry_file = ?")
		args = append(args, *req.EntryFile)
	}
	if req.EntryFunction != nil {
		updates = append(updates, "entry_function = ?")
		args = append(args, *req.EntryFunction)
	}
	if req.StartURL != nil {
		updates = append(updates, "start_url = ?")
		args = append(args, *req.StartURL)
	}
	if req.Config != nil {
		configBytes, _ := json.Marshal(req.Config)
		updates = append(updates, "config = ?")
		args = append(args, string(configBytes))
	}
	if req.Concurrency != nil {
		updates = append(updates, "concurrency = ?")
		args = append(args, *req.Concurrency)
	}
	if req.OutputGroupID != nil {
		updates = append(updates, "output_group_id = ?")
		args = append(args, *req.OutputGroupID)
	}
	if req.Schedule != nil {
		updates = append(updates, "schedule = ?")
		args = append(args, *req.Schedule)
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, *req.Enabled)
	}

	if len(updates) == 0 {
		c.JSON(200, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	sql := "UPDATE spider_projects SET " + strings.Join(updates, ", ") + " WHERE id = ?"
	_, err = db.Exec(sql, args...)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "更新失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}

// Delete 删除项目
func (h *SpidersHandler) Delete(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查项目状态
	var status string
	err = db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法删除"})
		return
	}

	// 删除文件和项目
	db.Exec("DELETE FROM spider_project_files WHERE project_id = ?", id)
	db.Exec("DELETE FROM spider_projects WHERE id = ?", id)

	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}

// Toggle 切换启用状态
func (h *SpidersHandler) Toggle(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var enabled int
	err = db.Get(&enabled, "SELECT enabled FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	newEnabled := 1 - enabled
	db.Exec("UPDATE spider_projects SET enabled = ? WHERE id = ?", newEnabled, id)

	message := "已启用"
	if newEnabled == 0 {
		message = "已禁用"
	}

	c.JSON(200, gin.H{"success": true, "enabled": newEnabled, "message": message})
}
```

**Step 5: 添加 strings import 并验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 6: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): implement spider project CRUD APIs"
```

---

## Task 3: 实现项目文件管理 API

**Files:**
- Modify: `go-page-server/api/spiders.go`

**Step 1: 实现文件列表和获取**

```go
// ListFiles 获取项目文件列表
func (h *SpidersHandler) ListFiles(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查项目是否存在
	var exists int
	db.Get(&exists, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if exists == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	var files []SpiderProjectFile
	db.Select(&files, `
		SELECT id, project_id, filename, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY filename
	`, id)

	c.JSON(200, gin.H{"success": true, "data": files})
}

// GetFile 获取单个文件内容
func (h *SpidersHandler) GetFile(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	filename := c.Param("filename")

	var file SpiderProjectFile
	err := db.Get(&file, `
		SELECT id, project_id, filename, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? AND filename = ?
	`, id, filename)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": file})
}
```

**Step 2: 实现文件创建、更新、删除**

```go
// CreateFile 创建文件
func (h *SpidersHandler) CreateFile(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	// 检查项目状态
	var status string
	err := db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法添加文件"})
		return
	}

	var req SpiderFileCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 验证文件名
	if !strings.HasSuffix(req.Filename, ".py") {
		c.JSON(400, gin.H{"success": false, "message": "文件名必须以 .py 结尾"})
		return
	}

	// 检查文件是否已存在
	var exists int
	db.Get(&exists, "SELECT COUNT(*) FROM spider_project_files WHERE project_id = ? AND filename = ?", id, req.Filename)
	if exists > 0 {
		c.JSON(400, gin.H{"success": false, "message": "文件已存在"})
		return
	}

	result, err := db.Exec(`
		INSERT INTO spider_project_files (project_id, filename, content) VALUES (?, ?, ?)
	`, id, req.Filename, req.Content)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建失败"})
		return
	}

	fileID, _ := result.LastInsertId()
	c.JSON(200, gin.H{"success": true, "id": fileID, "message": "创建成功"})
}

// UpdateFile 更新文件内容
func (h *SpidersHandler) UpdateFile(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	filename := c.Param("filename")

	// 检查项目状态
	var status string
	err := db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法修改文件"})
		return
	}

	var req SpiderFileUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	result, err := db.Exec(`
		UPDATE spider_project_files SET content = ? WHERE project_id = ? AND filename = ?
	`, req.Content, id, filename)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "更新失败"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}

// DeleteFile 删除文件
func (h *SpidersHandler) DeleteFile(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	filename := c.Param("filename")

	// 检查项目状态和入口文件
	var project struct {
		Status    string `db:"status"`
		EntryFile string `db:"entry_file"`
	}
	err := db.Get(&project, "SELECT status, entry_file FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if project.Status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法删除文件"})
		return
	}
	if filename == project.EntryFile {
		c.JSON(400, gin.H{"success": false, "message": "不能删除入口文件"})
		return
	}

	result, err := db.Exec("DELETE FROM spider_project_files WHERE project_id = ? AND filename = ?", id, filename)
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "删除失败"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}
```

**Step 3: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): implement project file management APIs"
```

---

## Task 4: 实现任务控制 API（通过 Redis 发送命令）

**Files:**
- Modify: `go-page-server/api/spiders.go`

**Step 1: 添加 Redis 命令发布功能**

```go
// SpiderCommand Redis 命令结构
type SpiderCommand struct {
	Action    string `json:"action"`     // run, stop, test, pause, resume
	ProjectID int    `json:"project_id"`
	MaxItems  int    `json:"max_items,omitempty"` // 测试模式最大条数
	Timestamp int64  `json:"timestamp"`
}

// publishCommand 发布命令到 Redis
func publishCommand(rdb *redis.Client, cmd SpiderCommand) error {
	ctx := context.Background()
	cmdJSON, _ := json.Marshal(cmd)
	return rdb.Publish(ctx, "spider:commands", cmdJSON).Err()
}

// Run 运行项目
func (h *SpidersHandler) Run(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查项目状态
	var status string
	err = db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中"})
		return
	}

	// 更新状态为 running
	db.Exec("UPDATE spider_projects SET status = 'running' WHERE id = ?", id)

	// 发布运行命令
	cmd := SpiderCommand{
		Action:    "run",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	if err := publishCommand(rdb, cmd); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "发送命令失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "任务已启动"})
}

// Test 测试运行
func (h *SpidersHandler) Test(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))
	maxItems, _ := strconv.Atoi(c.DefaultQuery("max_items", "0"))

	// 检查项目是否存在
	var exists int
	db.Get(&exists, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if exists == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	// 发布测试命令
	cmd := SpiderCommand{
		Action:    "test",
		ProjectID: id,
		MaxItems:  maxItems,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(rdb, cmd)

	sessionID := fmt.Sprintf("test_%d", id)
	c.JSON(200, gin.H{"success": true, "message": "测试已启动", "session_id": sessionID})
}

// TestStop 停止测试
func (h *SpidersHandler) TestStop(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := SpiderCommand{
		Action:    "test_stop",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(rdb, cmd)

	c.JSON(200, gin.H{"success": true, "message": "测试已停止"})
}

// Stop 停止项目
func (h *SpidersHandler) Stop(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))
	clearQueue := c.Query("clear_queue") == "true"

	cmd := SpiderCommand{
		Action:    "stop",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(rdb, cmd)

	// 更新数据库状态
	db.Exec("UPDATE spider_projects SET status = 'idle', last_error = ? WHERE id = ?",
		"用户手动停止", id)

	message := "已停止"
	if clearQueue {
		message += "并清空队列"
	}
	c.JSON(200, gin.H{"success": true, "message": message})
}

// Pause 暂停项目
func (h *SpidersHandler) Pause(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := SpiderCommand{
		Action:    "pause",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(rdb, cmd)

	c.JSON(200, gin.H{"success": true, "message": "已暂停"})
}

// Resume 恢复项目
func (h *SpidersHandler) Resume(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := SpiderCommand{
		Action:    "resume",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(rdb, cmd)

	c.JSON(200, gin.H{"success": true, "message": "已恢复"})
}
```

**Step 2: 添加必要的 import**

```go
import (
	"context"
	"fmt"
	// ... 其他 imports
)
```

**Step 3: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): implement task control APIs via Redis"
```

---

## Task 5: 实现实时统计 API

**Files:**
- Modify: `go-page-server/api/spiders.go`

**Step 1: 实现统计相关结构和方法**

```go
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

// GetRealtimeStats 获取实时统计
func (h *SpidersHandler) GetRealtimeStats(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	// 检查项目是否存在
	var status string
	err := db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	ctx := context.Background()

	// 从 Redis 获取统计数据
	statsKey := fmt.Sprintf("spider:stats:%d", id)
	statsData, err := rdb.HGetAll(ctx, statsKey).Result()

	stats := SpiderStats{Status: status}
	if err == nil && len(statsData) > 0 {
		stats.Total, _ = strconv.Atoi(statsData["total"])
		stats.Completed, _ = strconv.Atoi(statsData["completed"])
		stats.Failed, _ = strconv.Atoi(statsData["failed"])
		stats.Retried, _ = strconv.Atoi(statsData["retried"])
		stats.Pending, _ = strconv.Atoi(statsData["pending"])
		stats.Processing, _ = strconv.Atoi(statsData["processing"])

		if stats.Total > 0 {
			stats.SuccessRate = float64(stats.Completed) / float64(stats.Total) * 100
		}
	}

	c.JSON(200, gin.H{"success": true, "data": stats})
}

// GetChartStats 获取历史图表数据
func (h *SpidersHandler) GetChartStats(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	var data []map[string]interface{}
	err := db.Select(&data, `
		SELECT period_start, total, completed, failed, retried, avg_speed
		FROM spider_stats_history
		WHERE project_id = ? AND period_type = ?
		ORDER BY period_start DESC
		LIMIT ?
	`, id, period, limit)

	if err != nil {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": data})
}

// ClearQueue 清空队列
func (h *SpidersHandler) ClearQueue(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	// 检查项目状态
	var status string
	err := db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，请先停止"})
		return
	}

	ctx := context.Background()
	// 清空相关 Redis 键
	keys := []string{
		fmt.Sprintf("spider:queue:%d", id),
		fmt.Sprintf("spider:seen:%d", id),
		fmt.Sprintf("spider:stats:%d", id),
	}
	rdb.Del(ctx, keys...)

	c.JSON(200, gin.H{"success": true, "message": "队列已清空"})
}

// Reset 重置项目
func (h *SpidersHandler) Reset(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	// 检查项目状态
	var status string
	err := db.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，请先停止"})
		return
	}

	ctx := context.Background()
	// 清空 Redis 队列
	keys := []string{
		fmt.Sprintf("spider:queue:%d", id),
		fmt.Sprintf("spider:seen:%d", id),
		fmt.Sprintf("spider:stats:%d", id),
	}
	rdb.Del(ctx, keys...)

	// 清空失败请求记录
	result, _ := db.Exec("DELETE FROM spider_failed_requests WHERE project_id = ?", id)
	affected, _ := result.RowsAffected()

	c.JSON(200, gin.H{
		"success": true,
		"message": fmt.Sprintf("项目已重置，清空了 %d 条失败记录", affected),
	})
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): implement realtime stats and queue management APIs"
```

---

## Task 6: 实现失败请求管理 API

**Files:**
- Modify: `go-page-server/api/spiders.go`

**Step 1: 添加失败请求结构体和方法**

```go
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

// ListFailed 获取失败请求列表
func (h *SpidersHandler) ListFailed(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")

	where := "project_id = ?"
	args := []interface{}{id}

	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	// 总数
	var total int
	db.Get(&total, "SELECT COUNT(*) FROM spider_failed_requests WHERE "+where, args...)

	// 数据
	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	var data []SpiderFailedRequest
	db.Select(&data, `
		SELECT id, project_id, url, method, callback, meta, error_message,
		       retry_count, failed_at, status
		FROM spider_failed_requests
		WHERE `+where+`
		ORDER BY failed_at DESC
		LIMIT ? OFFSET ?
	`, args...)

	c.JSON(200, gin.H{"success": true, "data": data, "total": total})
}

// GetFailedStats 获取失败统计
func (h *SpidersHandler) GetFailedStats(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var stats struct {
		Pending int `db:"pending"`
		Retried int `db:"retried"`
		Ignored int `db:"ignored"`
	}

	db.Get(&stats, `
		SELECT
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) as pending,
			SUM(CASE WHEN status = 'retried' THEN 1 ELSE 0 END) as retried,
			SUM(CASE WHEN status = 'ignored' THEN 1 ELSE 0 END) as ignored
		FROM spider_failed_requests WHERE project_id = ?
	`, id)

	c.JSON(200, gin.H{
		"success": true,
		"data": map[string]int{
			"pending": stats.Pending,
			"retried": stats.Retried,
			"ignored": stats.Ignored,
			"total":   stats.Pending + stats.Retried + stats.Ignored,
		},
	})
}

// RetryAllFailed 重试所有失败请求
func (h *SpidersHandler) RetryAllFailed(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	// 获取所有待重试的请求
	var failed []SpiderFailedRequest
	db.Select(&failed, `
		SELECT id, url, method, callback, meta
		FROM spider_failed_requests
		WHERE project_id = ? AND status = 'pending'
	`, id)

	ctx := context.Background()
	queueKey := fmt.Sprintf("spider:queue:%d", id)

	count := 0
	for _, f := range failed {
		// 将请求重新放入队列
		reqData, _ := json.Marshal(map[string]interface{}{
			"url":      f.URL,
			"method":   f.Method,
			"callback": f.Callback,
			"meta":     f.Meta,
		})
		rdb.LPush(ctx, queueKey, reqData)

		// 更新状态
		db.Exec("UPDATE spider_failed_requests SET status = 'retried' WHERE id = ?", f.ID)
		count++
	}

	c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("已重试 %d 个失败请求", count), "count": count})
}

// RetryOneFailed 重试单个失败请求
func (h *SpidersHandler) RetryOneFailed(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)
	rdb := c.MustGet("redis").(*redis.Client)

	projectID, _ := strconv.Atoi(c.Param("id"))
	failedID, _ := strconv.Atoi(c.Param("fid"))

	var f SpiderFailedRequest
	err := db.Get(&f, `
		SELECT id, url, method, callback, meta
		FROM spider_failed_requests
		WHERE id = ? AND project_id = ? AND status = 'pending'
	`, failedID, projectID)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在或状态不正确"})
		return
	}

	ctx := context.Background()
	queueKey := fmt.Sprintf("spider:queue:%d", projectID)

	reqData, _ := json.Marshal(map[string]interface{}{
		"url":      f.URL,
		"method":   f.Method,
		"callback": f.Callback,
		"meta":     f.Meta,
	})
	rdb.LPush(ctx, queueKey, reqData)

	db.Exec("UPDATE spider_failed_requests SET status = 'retried' WHERE id = ?", failedID)

	c.JSON(200, gin.H{"success": true, "message": "已重试"})
}

// IgnoreFailed 忽略失败请求
func (h *SpidersHandler) IgnoreFailed(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	failedID, _ := strconv.Atoi(c.Param("fid"))

	result, _ := db.Exec("UPDATE spider_failed_requests SET status = 'ignored' WHERE id = ?", failedID)
	affected, _ := result.RowsAffected()

	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "已忽略"})
}

// DeleteFailed 删除失败请求
func (h *SpidersHandler) DeleteFailed(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	failedID, _ := strconv.Atoi(c.Param("fid"))

	result, _ := db.Exec("DELETE FROM spider_failed_requests WHERE id = ?", failedID)
	affected, _ := result.RowsAffected()

	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "已删除"})
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): implement failed request management APIs"
```

---

## Task 7: 实现代码模板和统计路由 API

**Files:**
- Modify: `go-page-server/api/spiders.go`

**Step 1: 实现代码模板 API**

```go
// GetCodeTemplates 获取代码模板列表
func (h *SpidersHandler) GetCodeTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{
			"name":         "api_pagination",
			"display_name": "API 分页抓取",
			"description":  "适用于 JSON API 接口的分页抓取",
			"code":         defaultSpiderCode,
		},
		{
			"name":         "html_list_detail",
			"display_name": "HTML 列表+详情",
			"description":  "适用于 HTML 页面的列表页和详情页抓取",
			"code": `from loguru import logger
import requests
from parsel import Selector

def main():
    """HTML 列表页 + 详情页抓取"""
    resp = requests.get('https://example.com/list', timeout=10)
    sel = Selector(resp.text)

    for article in sel.css('.article-item'):
        url = article.css('a::attr(href)').get()
        if not url:
            continue

        try:
            detail_resp = requests.get(url, timeout=10)
            detail_sel = Selector(detail_resp.text)

            yield {
                'title': detail_sel.css('h1::text').get(),
                'content': detail_sel.css('.content').get(),
                'author': detail_sel.css('.author::text').get(),
                'source_url': url,
            }

            logger.info(f'已抓取: {url}')

        except Exception as e:
            logger.error(f'抓取详情失败: {url} - {e}')
`,
		},
		{
			"name":         "keyword_crawler",
			"display_name": "关键词爬虫",
			"description":  "抓取百度下拉词等关键词数据",
			"code": `from loguru import logger
import httpx
import json

async def fetch_baidu_suggestions(keyword: str) -> list:
    """抓取百度下拉词"""
    url = f"https://suggestion.baidu.com/su?wd={keyword}&cb=window.baidu.sug"
    async with httpx.AsyncClient(timeout=10) as client:
        resp = await client.get(url)
        text = resp.text
        json_str = text[text.find("(")+1:text.rfind(")")]
        data = json.loads(json_str)
        return data.get("s", [])

def main():
    """关键词爬虫入口"""
    import asyncio

    seed_keywords = ["SEO优化", "网站建设"]

    async def run():
        for seed in seed_keywords:
            suggestions = await fetch_baidu_suggestions(seed)
            if suggestions:
                yield {
                    "type": "keywords",
                    "keywords": suggestions,
                }

    return list(asyncio.run(run()))
`,
		},
	}

	c.JSON(200, gin.H{"success": true, "data": templates})
}
```

**Step 2: 添加爬虫统计路由处理器**

```go
// SpiderStatsHandler 爬虫统计处理器
type SpiderStatsHandler struct{}

// GetOverview 获取统计概览
func (h *SpiderStatsHandler) GetOverview(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	projectIDStr := c.Query("project_id")
	period := c.DefaultQuery("period", "day")

	where := "period_type = ?"
	args := []interface{}{period}

	if projectIDStr != "" && projectIDStr != "0" {
		projectID, _ := strconv.Atoi(projectIDStr)
		where += " AND project_id = ?"
		args = append(args, projectID)
	}

	var stats struct {
		Total       int     `db:"total"`
		Completed   int     `db:"completed"`
		Failed      int     `db:"failed"`
		Retried     int     `db:"retried"`
		AvgSpeed    float64 `db:"avg_speed"`
		SuccessRate float64 `db:"success_rate"`
	}

	db.Get(&stats, `
		SELECT
			COALESCE(SUM(total), 0) as total,
			COALESCE(SUM(completed), 0) as completed,
			COALESCE(SUM(failed), 0) as failed,
			COALESCE(SUM(retried), 0) as retried,
			COALESCE(AVG(avg_speed), 0) as avg_speed
		FROM spider_stats_history
		WHERE `+where, args...)

	if stats.Total > 0 {
		stats.SuccessRate = float64(stats.Completed) / float64(stats.Total) * 100
	}

	c.JSON(200, gin.H{"success": true, "data": stats})
}

// GetChart 获取图表数据
func (h *SpiderStatsHandler) GetChart(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	projectIDStr := c.Query("project_id")
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	where := "period_type = ?"
	args := []interface{}{period}

	if projectIDStr != "" && projectIDStr != "0" {
		projectID, _ := strconv.Atoi(projectIDStr)
		where += " AND project_id = ?"
		args = append(args, projectID)
	}

	args = append(args, limit)

	var data []map[string]interface{}
	db.Select(&data, `
		SELECT period_start, SUM(total) as total, SUM(completed) as completed,
		       SUM(failed) as failed, AVG(avg_speed) as avg_speed
		FROM spider_stats_history
		WHERE `+where+`
		GROUP BY period_start
		ORDER BY period_start DESC
		LIMIT ?
	`, args...)

	c.JSON(200, gin.H{"success": true, "data": data})
}

// GetScheduled 获取已调度项目
func (h *SpiderStatsHandler) GetScheduled(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	var projects []struct {
		ID       int     `db:"id" json:"id"`
		Name     string  `db:"name" json:"name"`
		Schedule *string `db:"schedule" json:"schedule"`
		Enabled  int     `db:"enabled" json:"enabled"`
	}

	db.Select(&projects, `
		SELECT id, name, schedule, enabled
		FROM spider_projects
		WHERE schedule IS NOT NULL AND schedule != ''
		ORDER BY id
	`)

	c.JSON(200, gin.H{"success": true, "data": projects})
}

// GetByProject 按项目统计
func (h *SpiderStatsHandler) GetByProject(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	period := c.DefaultQuery("period", "day")

	var data []map[string]interface{}
	db.Select(&data, `
		SELECT sp.id, sp.name,
		       COALESCE(SUM(sh.total), 0) as total,
		       COALESCE(SUM(sh.completed), 0) as completed,
		       COALESCE(SUM(sh.failed), 0) as failed
		FROM spider_projects sp
		LEFT JOIN spider_stats_history sh ON sp.id = sh.project_id AND sh.period_type = ?
		GROUP BY sp.id, sp.name
		ORDER BY total DESC
	`, period)

	c.JSON(200, gin.H{"success": true, "data": data})
}
```

**Step 3: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 4: Commit**

```bash
git add go-page-server/api/spiders.go
git commit -m "feat(spiders): implement code templates and stats APIs"
```

---

## Task 8: 实现 WebSocket 日志推送

**Files:**
- Create: `go-page-server/api/websocket.go`

**Step 1: 创建 WebSocket 处理器**

```go
package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源，生产环境应该限制
	},
}

// WebSocketHandler WebSocket 处理器
type WebSocketHandler struct{}

// SpiderLogs 爬虫日志 WebSocket
func (h *WebSocketHandler) SpiderLogs(c *gin.Context) {
	rdb := c.MustGet("redis").(*redis.Client)
	projectID := c.Param("id")

	// 升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 订阅 Redis 日志频道
	channel := "spider:logs:" + projectID
	pubsub := rdb.Subscribe(ctx, channel)
	defer pubsub.Close()

	// 监听客户端断开
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
		}
	}()

	// 接收并转发日志
	ch := pubsub.Channel()
	for {
		select {
		case msg := <-ch:
			if msg == nil {
				return
			}
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/websocket.go
git commit -m "feat(websocket): implement spider logs WebSocket endpoint"
```

---

## Task 9: 实现生成器管理 API

**Files:**
- Create: `go-page-server/api/generators.go`

**Step 1: 创建生成器结构体和 CRUD 方法**

```go
package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Generator 生成器
type Generator struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	DisplayName string    `db:"display_name" json:"display_name"`
	Description *string   `db:"description" json:"description"`
	Code        string    `db:"code" json:"code"`
	Enabled     int       `db:"enabled" json:"enabled"`
	IsDefault   int       `db:"is_default" json:"is_default"`
	Version     int       `db:"version" json:"version"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// GeneratorCreate 创建请求
type GeneratorCreate struct {
	Name        string  `json:"name" binding:"required"`
	DisplayName string  `json:"display_name" binding:"required"`
	Description *string `json:"description"`
	Code        string  `json:"code" binding:"required"`
	Enabled     int     `json:"enabled"`
	IsDefault   int     `json:"is_default"`
}

// GeneratorUpdate 更新请求
type GeneratorUpdate struct {
	DisplayName *string `json:"display_name"`
	Description *string `json:"description"`
	Code        *string `json:"code"`
	Enabled     *int    `json:"enabled"`
	IsDefault   *int    `json:"is_default"`
}

// GeneratorsHandler 生成器处理器
type GeneratorsHandler struct{}

// List 获取生成器列表
func (h *GeneratorsHandler) List(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	enabledStr := c.Query("enabled")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	where := "1=1"
	args := []interface{}{}

	if enabledStr != "" {
		enabled, _ := strconv.Atoi(enabledStr)
		where += " AND enabled = ?"
		args = append(args, enabled)
	}

	// 总数
	var total int
	db.Get(&total, "SELECT COUNT(*) FROM content_generators WHERE "+where, args...)

	// 数据
	offset := (page - 1) * pageSize
	args = append(args, offset, pageSize)

	var generators []Generator
	db.Select(&generators, `
		SELECT id, name, display_name, description, code, enabled, is_default, version, created_at, updated_at
		FROM content_generators
		WHERE `+where+`
		ORDER BY is_default DESC, id ASC
		LIMIT ?, ?
	`, args...)

	c.JSON(200, gin.H{
		"success":   true,
		"data":      generators,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get 获取生成器详情
func (h *GeneratorsHandler) Get(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var gen Generator
	err := db.Get(&gen, `
		SELECT id, name, display_name, description, code, enabled, is_default, version, created_at, updated_at
		FROM content_generators WHERE id = ?
	`, id)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": gen})
}

// Create 创建生成器
func (h *GeneratorsHandler) Create(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	var req GeneratorCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 检查名称是否已存在
	var exists int
	db.Get(&exists, "SELECT COUNT(*) FROM content_generators WHERE name = ?", req.Name)
	if exists > 0 {
		c.JSON(400, gin.H{"success": false, "message": "生成器名称已存在"})
		return
	}

	// 如果设为默认，先取消其他默认
	if req.IsDefault == 1 {
		db.Exec("UPDATE content_generators SET is_default = 0 WHERE is_default = 1")
	}

	result, err := db.Exec(`
		INSERT INTO content_generators (name, display_name, description, code, enabled, is_default)
		VALUES (?, ?, ?, ?, ?, ?)
	`, req.Name, req.DisplayName, req.Description, req.Code, req.Enabled, req.IsDefault)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建失败"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(200, gin.H{"success": true, "id": id, "message": "创建成功"})
}

// Update 更新生成器
func (h *GeneratorsHandler) Update(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var req GeneratorUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 如果设为默认，先取消其他默认
	if req.IsDefault != nil && *req.IsDefault == 1 {
		db.Exec("UPDATE content_generators SET is_default = 0 WHERE is_default = 1 AND id != ?", id)
	}

	updates := []string{}
	args := []interface{}{}

	if req.DisplayName != nil {
		updates = append(updates, "display_name = ?")
		args = append(args, *req.DisplayName)
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Code != nil {
		updates = append(updates, "code = ?", "version = version + 1")
		args = append(args, *req.Code)
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, *req.Enabled)
	}
	if req.IsDefault != nil {
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		c.JSON(200, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	_, err := db.Exec("UPDATE content_generators SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "更新失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}

// Delete 删除生成器
func (h *GeneratorsHandler) Delete(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	// 检查是否为默认
	var isDefault int
	err := db.Get(&isDefault, "SELECT is_default FROM content_generators WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}
	if isDefault == 1 {
		c.JSON(400, gin.H{"success": false, "message": "不能删除默认生成器"})
		return
	}

	db.Exec("DELETE FROM content_generators WHERE id = ?", id)
	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}

// SetDefault 设为默认
func (h *GeneratorsHandler) SetDefault(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	// 检查是否存在且启用
	var enabled int
	err := db.Get(&enabled, "SELECT enabled FROM content_generators WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}
	if enabled == 0 {
		c.JSON(400, gin.H{"success": false, "message": "不能将禁用的生成器设为默认"})
		return
	}

	db.Exec("UPDATE content_generators SET is_default = 0 WHERE is_default = 1")
	db.Exec("UPDATE content_generators SET is_default = 1 WHERE id = ?", id)

	c.JSON(200, gin.H{"success": true, "message": "已设为默认"})
}

// Toggle 切换启用状态
func (h *GeneratorsHandler) Toggle(c *gin.Context) {
	db := c.MustGet("db").(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var gen struct {
		Enabled   int `db:"enabled"`
		IsDefault int `db:"is_default"`
	}
	err := db.Get(&gen, "SELECT enabled, is_default FROM content_generators WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}

	if gen.IsDefault == 1 && gen.Enabled == 1 {
		c.JSON(400, gin.H{"success": false, "message": "不能禁用默认生成器"})
		return
	}

	newEnabled := 1 - gen.Enabled
	db.Exec("UPDATE content_generators SET enabled = ? WHERE id = ?", newEnabled, id)

	message := "已启用"
	if newEnabled == 0 {
		message = "已禁用"
	}
	c.JSON(200, gin.H{"success": true, "enabled": newEnabled, "message": message})
}

// GetTemplates 获取代码模板
func (h *GeneratorsHandler) GetTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{
			"name":         "basic",
			"display_name": "基础模板",
			"description":  "简单的段落拼接+拼音标注",
			"code": `async def generate(ctx):
    if len(ctx.paragraphs) < 3:
        return None

    selected = ctx.paragraphs[:3]
    content = "\\n\\n".join(selected)
    return annotate_pinyin(content)
`,
		},
		{
			"name":         "with_title",
			"display_name": "带标题模板",
			"description":  "正文开头插入随机标题",
			"code": `async def generate(ctx):
    if len(ctx.paragraphs) < 3:
        return None

    parts = []
    if ctx.titles:
        parts.append(random.choice(ctx.titles))
        parts.append("")

    count = min(len(ctx.paragraphs), random.randint(3, 5))
    parts.extend(random.sample(ctx.paragraphs, count))

    return annotate_pinyin("\\n\\n".join(parts))
`,
		},
	}

	c.JSON(200, gin.H{"success": true, "data": templates})
}
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/generators.go
git commit -m "feat(generators): implement generator management APIs"
```

---

## Task 10: 注册所有路由

**Files:**
- Modify: `go-page-server/api/router.go`

**Step 1: 添加爬虫、生成器和 WebSocket 路由**

在 `SetupRouter` 函数的 `api` 路由组中添加：

```go
// 爬虫项目管理
spiders := &SpidersHandler{}
spiderRoutes := api.Group("/spider-projects")
{
    spiderRoutes.GET("", spiders.List)
    spiderRoutes.POST("", spiders.Create)
    spiderRoutes.GET("/:id", spiders.Get)
    spiderRoutes.PUT("/:id", spiders.Update)
    spiderRoutes.DELETE("/:id", spiders.Delete)
    spiderRoutes.POST("/:id/toggle", spiders.Toggle)

    // 文件管理
    spiderRoutes.GET("/:id/files", spiders.ListFiles)
    spiderRoutes.GET("/:id/files/:filename", spiders.GetFile)
    spiderRoutes.POST("/:id/files", spiders.CreateFile)
    spiderRoutes.PUT("/:id/files/:filename", spiders.UpdateFile)
    spiderRoutes.DELETE("/:id/files/:filename", spiders.DeleteFile)

    // 任务控制
    spiderRoutes.POST("/:id/run", spiders.Run)
    spiderRoutes.POST("/:id/test", spiders.Test)
    spiderRoutes.POST("/:id/test/stop", spiders.TestStop)
    spiderRoutes.POST("/:id/stop", spiders.Stop)
    spiderRoutes.POST("/:id/pause", spiders.Pause)
    spiderRoutes.POST("/:id/resume", spiders.Resume)

    // 统计
    spiderRoutes.GET("/:id/stats/realtime", spiders.GetRealtimeStats)
    spiderRoutes.GET("/:id/stats/chart", spiders.GetChartStats)

    // 队列管理
    spiderRoutes.POST("/:id/queue/clear", spiders.ClearQueue)
    spiderRoutes.POST("/:id/reset", spiders.Reset)

    // 失败请求
    spiderRoutes.GET("/:id/failed", spiders.ListFailed)
    spiderRoutes.GET("/:id/failed/stats", spiders.GetFailedStats)
    spiderRoutes.POST("/:id/failed/retry-all", spiders.RetryAllFailed)
    spiderRoutes.POST("/:id/failed/:fid/retry", spiders.RetryOneFailed)
    spiderRoutes.POST("/:id/failed/:fid/ignore", spiders.IgnoreFailed)
    spiderRoutes.DELETE("/:id/failed/:fid", spiders.DeleteFailed)

    // 代码模板
    spiderRoutes.GET("/templates", spiders.GetCodeTemplates)
}

// 爬虫统计
spiderStats := &SpiderStatsHandler{}
statsRoutes := api.Group("/spider-stats")
{
    statsRoutes.GET("/overview", spiderStats.GetOverview)
    statsRoutes.GET("/chart", spiderStats.GetChart)
    statsRoutes.GET("/scheduled", spiderStats.GetScheduled)
    statsRoutes.GET("/by-project", spiderStats.GetByProject)
}

// 生成器管理
generators := &GeneratorsHandler{}
genRoutes := api.Group("/generators")
{
    genRoutes.GET("", generators.List)
    genRoutes.POST("", generators.Create)
    genRoutes.GET("/:id", generators.Get)
    genRoutes.PUT("/:id", generators.Update)
    genRoutes.DELETE("/:id", generators.Delete)
    genRoutes.POST("/:id/set-default", generators.SetDefault)
    genRoutes.POST("/:id/toggle", generators.Toggle)
    genRoutes.GET("/templates/list", generators.GetTemplates)
}

// WebSocket 路由（不需要认证）
ws := &WebSocketHandler{}
r.GET("/ws/spider-logs/:id", ws.SpiderLogs)
```

**Step 2: 验证编译**

Run: `cd go-page-server && go build ./...`
Expected: 编译成功

**Step 3: Commit**

```bash
git add go-page-server/api/router.go
git commit -m "feat(router): register spider, generator and websocket routes"
```

---

## Task 11: 创建 Python Worker 命令监听器

**Files:**
- Create: `python-worker/command_listener.py`

**Step 1: 创建命令监听器**

```python
# -*- coding: utf-8 -*-
"""
Python Worker 命令监听器

监听 Go API 发送的 Redis 命令，执行爬虫任务。
复用现有的 ProjectLoader 和 ProjectRunner。
"""

import asyncio
import json
from datetime import datetime
from typing import Dict, Optional

import redis.asyncio as redis
from loguru import logger

# 复用现有核心模块
from core.crawler.project_loader import ProjectLoader
from core.crawler.project_runner import ProjectRunner
from core.crawler.request_queue import RequestQueue
from database.db import fetch_one, insert, execute_query, get_db_pool
from core.redis_client import get_redis_client


class RedisLogger:
    """日志发布到 Redis，Go 订阅后推送给前端"""

    def __init__(self, rdb: redis.Redis, project_id: int, prefix: str = "project"):
        self.rdb = rdb
        self.channel = f"spider:logs:{prefix}_{project_id}"

    async def _publish(self, level: str, message: str):
        log = {
            "level": level,
            "message": message,
            "timestamp": datetime.now().isoformat()
        }
        await self.rdb.publish(self.channel, json.dumps(log, ensure_ascii=False))

    async def info(self, msg: str):
        await self._publish("INFO", msg)
        logger.info(msg)

    async def warning(self, msg: str):
        await self._publish("WARNING", msg)
        logger.warning(msg)

    async def error(self, msg: str):
        await self._publish("ERROR", msg)
        logger.error(msg)

    async def debug(self, msg: str):
        await self._publish("DEBUG", msg)
        logger.debug(msg)


class CommandListener:
    """监听 Go 发来的命令"""

    def __init__(self):
        self.running_tasks: Dict[int, asyncio.Task] = {}
        self.rdb: Optional[redis.Redis] = None

    async def start(self):
        """启动监听器"""
        self.rdb = get_redis_client()
        if not self.rdb:
            logger.error("Redis 未初始化，无法启动命令监听器")
            return

        logger.info("命令监听器已启动，等待命令...")

        pubsub = self.rdb.pubsub()
        await pubsub.subscribe("spider:commands")

        async for message in pubsub.listen():
            if message["type"] == "message":
                try:
                    cmd = json.loads(message["data"])
                    await self.handle_command(cmd)
                except Exception as e:
                    logger.error(f"处理命令失败: {e}")

    async def handle_command(self, cmd: dict):
        """处理命令"""
        action = cmd.get("action")
        project_id = cmd.get("project_id")

        logger.info(f"收到命令: {action} for project {project_id}")

        if action == "run":
            await self.run_project(project_id)
        elif action == "test":
            max_items = cmd.get("max_items", 0)
            await self.test_project(project_id, max_items)
        elif action == "stop":
            await self.stop_project(project_id)
        elif action == "test_stop":
            await self.stop_test(project_id)
        elif action == "pause":
            await self.pause_project(project_id)
        elif action == "resume":
            await self.resume_project(project_id)

    async def run_project(self, project_id: int):
        """运行爬虫项目"""
        log = RedisLogger(self.rdb, project_id, "project")

        # 更新状态
        await self.rdb.set(
            f"spider:status:{project_id}",
            json.dumps({"status": "running", "started_at": datetime.now().isoformat()})
        )

        try:
            await log.info("正在加载项目...")

            # 获取项目配置
            row = await fetch_one(
                "SELECT id, name, entry_file, config, concurrency, output_group_id FROM spider_projects WHERE id = %s",
                (project_id,)
            )
            if not row:
                await log.error("项目不存在")
                return

            config = json.loads(row['config']) if row['config'] else {}

            # 加载项目文件
            loader = ProjectLoader(project_id)
            modules = await loader.load()
            await log.info(f"已加载 {len(modules)} 个模块")

            # 创建运行器
            runner = ProjectRunner(
                project_id=project_id,
                modules=modules,
                config=config,
                redis=self.rdb,
                db_pool=get_db_pool(),
                concurrency=row.get('concurrency', 3),
            )

            await log.info("开始执行 Spider...")

            items_count = 0
            group_id = row['output_group_id']

            async for item in runner.run():
                # 检查停止信号
                stop_key = f"spider_project:{project_id}:stop"
                if await self.rdb.get(stop_key):
                    await log.info("收到停止信号，任务终止")
                    await self.rdb.delete(stop_key)
                    break

                # 保存数据（复用现有逻辑）
                item_type = item.get('type', 'article')

                if item_type == 'keywords':
                    # 写入关键词表
                    pass  # 复用现有 keyword_manager
                elif item_type == 'images':
                    # 写入图片表
                    pass  # 复用现有 image_manager
                else:
                    # 写入文章表
                    if item.get('title') and item.get('content'):
                        await insert("original_articles", {
                            "group_id": item.get('group_id', group_id),
                            "source_id": project_id,
                            "source_url": item.get('source_url'),
                            "title": item['title'][:500],
                            "content": item['content'],
                        })
                        items_count += 1

                if items_count % 10 == 0:
                    await log.info(f"已抓取 {items_count} 条数据")

            await log.info(f"任务完成：共 {items_count} 条数据")

            # 更新统计
            await execute_query(
                """
                UPDATE spider_projects SET
                    status = 'idle',
                    last_run_at = NOW(),
                    last_run_items = %s,
                    total_runs = total_runs + 1,
                    total_items = total_items + %s
                WHERE id = %s
                """,
                (items_count, items_count, project_id),
                commit=True
            )

        except Exception as e:
            await log.error(f"任务异常: {str(e)}")
            await execute_query(
                "UPDATE spider_projects SET status = 'error', last_error = %s WHERE id = %s",
                (str(e), project_id),
                commit=True
            )
        finally:
            await self.rdb.set(
                f"spider:status:{project_id}",
                json.dumps({"status": "idle"})
            )

    async def test_project(self, project_id: int, max_items: int = 0):
        """测试运行项目"""
        log = RedisLogger(self.rdb, project_id, "test")

        try:
            await log.info(f"开始测试运行（最多 {max_items} 条）..." if max_items else "开始测试运行...")

            # 类似 run_project，但不保存数据
            row = await fetch_one(
                "SELECT id, name, entry_file, config, concurrency FROM spider_projects WHERE id = %s",
                (project_id,)
            )
            if not row:
                await log.error("项目不存在")
                return

            config = json.loads(row['config']) if row['config'] else {}

            loader = ProjectLoader(project_id)
            modules = await loader.load()

            runner = ProjectRunner(
                project_id=project_id,
                modules=modules,
                config=config,
                redis=self.rdb,
                db_pool=get_db_pool(),
                concurrency=row.get('concurrency', 3),
                is_test=True,
                max_items=max_items,
            )

            items_count = 0
            async for item in runner.run():
                items_count += 1
                await log.info(f"[{items_count}] {item.get('title', '(无标题)')[:50]}")

                if max_items > 0 and items_count >= max_items:
                    break

            await log.info(f"测试完成：共 {items_count} 条数据")

        except Exception as e:
            await log.error(f"测试异常: {str(e)}")

    async def stop_project(self, project_id: int):
        """停止项目"""
        stop_key = f"spider_project:{project_id}:stop"
        await self.rdb.set(stop_key, "1", ex=3600)

        queue = RequestQueue(self.rdb, project_id)
        await queue.stop()

    async def stop_test(self, project_id: int):
        """停止测试"""
        queue = RequestQueue(self.rdb, project_id, is_test=True)
        await queue.stop(clear_queue=True)

    async def pause_project(self, project_id: int):
        """暂停项目"""
        queue = RequestQueue(self.rdb, project_id)
        await queue.pause()

    async def resume_project(self, project_id: int):
        """恢复项目"""
        queue = RequestQueue(self.rdb, project_id)
        await queue.resume()


async def main():
    """主入口"""
    listener = CommandListener()
    await listener.start()


if __name__ == "__main__":
    asyncio.run(main())
```

**Step 2: Commit**

```bash
git add python-worker/command_listener.py
git commit -m "feat(worker): add Python command listener for spider execution"
```

---

## Task 12: 添加测试

**Files:**
- Create: `go-page-server/api/spiders_test.go`

**Step 1: 创建基础测试**

```go
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSpidersList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpidersHandler{}
	r.GET("/api/spider-projects", handler.List)

	req := httptest.NewRequest("GET", "/api/spider-projects", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 没有数据库时应该返回 200 但数据为空
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestSpidersGet_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpidersHandler{}
	r.GET("/api/spider-projects/:id", handler.Get)

	req := httptest.NewRequest("GET", "/api/spider-projects/invalid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
}

func TestGeneratorsList_NoDb(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &GeneratorsHandler{}
	r.GET("/api/generators", handler.List)

	req := httptest.NewRequest("GET", "/api/generators", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestSpidersCodeTemplates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := &SpidersHandler{}
	r.GET("/api/spider-projects/templates", handler.GetCodeTemplates)

	req := httptest.NewRequest("GET", "/api/spider-projects/templates", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	data, ok := resp["data"].([]interface{})
	if !ok || len(data) == 0 {
		t.Error("Expected non-empty templates array")
	}
}
```

**Step 2: 运行测试**

Run: `cd go-page-server && go test ./api/... -v`
Expected: 测试通过

**Step 3: Commit**

```bash
git add go-page-server/api/spiders_test.go
git commit -m "test(spiders): add basic tests for spider and generator APIs"
```

---

## Task 13: 集成测试和验证

**Step 1: 运行所有测试**

Run: `cd go-page-server && go test ./... -v`
Expected: 所有测试通过

**Step 2: 验证编译**

Run: `cd go-page-server && go build -o server.exe ./cmd/server`
Expected: 编译成功

**Step 3: 最终 Commit**

```bash
git add -A
git commit -m "feat: complete spider and generator Go API migration

- Spider project CRUD (28 APIs)
- Generator management (10 APIs)
- WebSocket for real-time logs
- Redis command publishing for Python Worker
- Python CommandListener for task execution"
```

---

## 验证清单

- [ ] Go 编译通过
- [ ] 所有测试通过
- [ ] 爬虫项目 CRUD 可用
- [ ] 项目文件管理可用
- [ ] 任务控制 (run/stop/test) 可用
- [ ] WebSocket 日志推送可用
- [ ] 生成器管理可用
- [ ] Python Worker 可监听命令

---

*文档创建时间: 2026-01-30*
