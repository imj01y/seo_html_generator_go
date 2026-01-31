package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
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
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required,oneof=file dir"`
}

// SpiderMoveRequest 移动/重命名请求
type SpiderMoveRequest struct {
	NewPath string `json:"new_path" binding:"required"`
}

// SpidersHandler 爬虫处理器
type SpidersHandler struct{}

// SpiderStatsHandler 爬虫统计处理器
type SpiderStatsHandler struct{}

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

// publishCommand 发布命令到 Redis
func publishCommand(rdb *redis.Client, cmd SpiderCommand) error {
	ctx := context.Background()
	cmdJSON, _ := json.Marshal(cmd)
	return rdb.Publish(ctx, "spider:commands", cmdJSON).Err()
}

// ============================================
// 项目 CRUD API
// ============================================

// List 获取项目列表
func (h *SpidersHandler) List(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}, "total": 0})
		return
	}
	sqlxDB := db.(*sqlx.DB)

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

	var total int
	countSQL := "SELECT COUNT(*) FROM spider_projects WHERE " + where
	sqlxDB.Get(&total, countSQL, args...)

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
	sqlxDB.Select(&projects, dataSQL, args...)

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

// Get 获取项目详情
func (h *SpidersHandler) Get(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(404, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var project SpiderProject
	err = sqlxDB.Get(&project, `
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

// Create 创建项目
func (h *SpidersHandler) Create(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var req SpiderProjectCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

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

	var configJSON *string
	if req.Config != nil {
		configBytes, _ := json.Marshal(req.Config)
		configStr := string(configBytes)
		configJSON = &configStr
	}

	// 使用事务确保项目和文件同时创建成功
	tx, err := sqlxDB.Beginx()
	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "开启事务失败: " + err.Error()})
		return
	}

	// 插入项目
	result, err := tx.Exec(`
		INSERT INTO spider_projects
		(name, description, entry_file, entry_function, start_url, config,
		 concurrency, output_group_id, schedule, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, req.Name, req.Description, req.EntryFile, req.EntryFunction,
		req.StartURL, configJSON, req.Concurrency, req.OutputGroupID,
		req.Schedule, req.Enabled)

	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"success": false, "message": "创建项目失败: " + err.Error()})
		return
	}

	projectID, _ := result.LastInsertId()

	// 确保入口文件存在
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

	// 插入文件（带错误检查）
	for _, f := range req.Files {
		_, err := tx.Exec(`
			INSERT INTO spider_project_files (project_id, filename, content)
			VALUES (?, ?, ?)
		`, projectID, f.Filename, f.Content)
		if err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"success": false, "message": "创建文件失败: " + err.Error()})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "提交事务失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "id": projectID, "message": "创建成功"})
}

// Update 更新项目
func (h *SpidersHandler) Update(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var status string
	err = sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
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
	_, err = sqlxDB.Exec(sql, args...)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "更新失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}

// Delete 删除项目
func (h *SpidersHandler) Delete(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var status string
	err = sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法删除"})
		return
	}

	sqlxDB.Exec("DELETE FROM spider_project_files WHERE project_id = ?", id)
	sqlxDB.Exec("DELETE FROM spider_projects WHERE id = ?", id)

	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}

// Toggle 切换启用状态
func (h *SpidersHandler) Toggle(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var enabled int
	err = sqlxDB.Get(&enabled, "SELECT enabled FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	newEnabled := 1 - enabled
	sqlxDB.Exec("UPDATE spider_projects SET enabled = ? WHERE id = ?", newEnabled, id)

	message := "已启用"
	if newEnabled == 0 {
		message = "已禁用"
	}

	c.JSON(200, gin.H{"success": true, "enabled": newEnabled, "message": message})
}

// ============================================
// 项目文件管理 API
// ============================================

// ListFiles 获取项目文件列表
func (h *SpidersHandler) ListFiles(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查是否请求树形结构
	if c.Query("tree") == "true" {
		h.GetFileTree(c)
		return
	}

	var exists2 int
	sqlxDB.Get(&exists2, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if exists2 == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	var files []SpiderProjectFile
	sqlxDB.Select(&files, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY path
	`, id)

	c.JSON(200, gin.H{"success": true, "data": files})
}

// GetFileTree 获取项目文件树
func (h *SpidersHandler) GetFileTree(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查项目是否存在
	var exists2 int
	sqlxDB.Get(&exists2, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if exists2 == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	// 获取所有文件和目录
	var files []SpiderProjectFile
	sqlxDB.Select(&files, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY path
	`, id)

	// 构建树结构
	root := &SpiderTreeNode{
		Name:     "project",
		Path:     "/",
		Type:     "dir",
		Children: []*SpiderTreeNode{},
	}

	for _, file := range files {
		h.insertNode(root, file.Path, file.Type)
	}

	c.JSON(200, gin.H{"success": true, "data": root})
}

// insertNode 将文件/目录插入树结构
func (h *SpidersHandler) insertNode(root *SpiderTreeNode, path string, nodeType string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	current := root

	for i, part := range parts {
		if part == "" {
			continue
		}

		isLast := i == len(parts)-1
		found := false

		for _, child := range current.Children {
			if child.Name == part {
				current = child
				found = true
				break
			}
		}

		if !found {
			newNode := &SpiderTreeNode{
				Name: part,
				Path: "/" + strings.Join(parts[:i+1], "/"),
				Type: "dir",
			}
			if isLast {
				newNode.Type = nodeType
			}
			current.Children = append(current.Children, newNode)
			current = newNode
		}
	}
}

// GetFile 获取单个文件内容
func (h *SpidersHandler) GetFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(404, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := c.Param("path") // *path 通配符已包含前导 /

	var file SpiderProjectFile
	err := sqlxDB.Get(&file, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? AND path = ?
	`, id, path)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": gin.H{
		"path":    file.Path,
		"content": file.Content,
	}})
}

// CreateItem 创建文件或目录
func (h *SpidersHandler) CreateItem(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	parentPath := c.Param("path") // *path 通配符已包含前导 /，根目录时为空
	if parentPath == "" {
		parentPath = "/"
	}

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法添加文件"})
		return
	}

	var req SpiderCreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 验证文件名
	if strings.ContainsAny(req.Name, `/\:*?"<>|`) || strings.HasPrefix(req.Name, ".") {
		c.JSON(400, gin.H{"success": false, "message": "文件名包含非法字符"})
		return
	}

	// 构建完整路径
	var fullPath string
	if parentPath == "/" {
		fullPath = "/" + req.Name
	} else {
		fullPath = parentPath + "/" + req.Name
	}

	// 检查是否已存在
	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM spider_project_files WHERE project_id = ? AND path = ?", id, fullPath)
	if existsCount > 0 {
		c.JSON(400, gin.H{"success": false, "message": "文件或目录已存在"})
		return
	}

	// 创建
	content := ""
	if req.Type == "file" {
		content = "# " + req.Name + "\n"
	}

	result, err := sqlxDB.Exec(`
		INSERT INTO spider_project_files (project_id, path, type, content) VALUES (?, ?, ?, ?)
	`, id, fullPath, req.Type, content)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建失败: " + err.Error()})
		return
	}

	fileID, _ := result.LastInsertId()
	c.JSON(200, gin.H{"success": true, "id": fileID, "path": fullPath, "message": "创建成功"})
}

// UpdateFile 更新文件内容
func (h *SpidersHandler) UpdateFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := c.Param("path") // *path 通配符已包含前导 /

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
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

	// 使用 upsert
	_, err = sqlxDB.Exec(`
		INSERT INTO spider_project_files (project_id, path, type, content)
		VALUES (?, ?, 'file', ?)
		ON DUPLICATE KEY UPDATE content = VALUES(content)
	`, id, path, req.Content)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "保存文件失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "保存成功"})
}

// DeleteFile 删除文件或目录
func (h *SpidersHandler) DeleteFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := c.Param("path") // *path 通配符已包含前导 /

	var project struct {
		Status    string `db:"status"`
		EntryFile string `db:"entry_file"`
	}
	err := sqlxDB.Get(&project, "SELECT status, entry_file FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if project.Status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法删除文件"})
		return
	}

	// 检查是否是入口文件
	entryPath := "/" + project.EntryFile
	if path == entryPath {
		c.JSON(400, gin.H{"success": false, "message": "不能删除入口文件"})
		return
	}

	// 删除文件或目录（目录会递归删除子项）
	result, err := sqlxDB.Exec(`
		DELETE FROM spider_project_files
		WHERE project_id = ? AND (path = ? OR path LIKE ?)
	`, id, path, path+"/%")

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

// MoveItem 移动或重命名文件/目录
func (h *SpidersHandler) MoveItem(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	oldPath := c.Param("path") // *path 通配符已包含前导 /

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中"})
		return
	}

	var req SpiderMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	newPath := req.NewPath
	if !strings.HasPrefix(newPath, "/") {
		newPath = "/" + newPath
	}

	// 检查目标是否已存在
	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM spider_project_files WHERE project_id = ? AND path = ?", id, newPath)
	if existsCount > 0 {
		c.JSON(400, gin.H{"success": false, "message": "目标路径已存在"})
		return
	}

	// 更新路径（包括子目录）
	tx, _ := sqlxDB.Beginx()

	// 更新当前项
	_, err = tx.Exec(`
		UPDATE spider_project_files SET path = ? WHERE project_id = ? AND path = ?
	`, newPath, id, oldPath)
	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"success": false, "message": "移动失败"})
		return
	}

	// 更新子项（如果是目录）
	_, err = tx.Exec(`
		UPDATE spider_project_files
		SET path = CONCAT(?, SUBSTRING(path, ?))
		WHERE project_id = ? AND path LIKE ?
	`, newPath, len(oldPath)+1, id, oldPath+"/%")
	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"success": false, "message": "移动子项失败"})
		return
	}

	tx.Commit()
	c.JSON(200, gin.H{"success": true, "message": "移动成功", "new_path": newPath})
}

// ============================================
// 任务控制 API
// ============================================

// Run 运行项目
func (h *SpidersHandler) Run(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	var status string
	err = sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中"})
		return
	}

	sqlxDB.Exec("UPDATE spider_projects SET status = 'running' WHERE id = ?", id)

	cmd := SpiderCommand{
		Action:    "run",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	if err := publishCommand(redisClient, cmd); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "发送命令失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "任务已启动"})
}

// Test 测试运行
func (h *SpidersHandler) Test(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))
	maxItems, _ := strconv.Atoi(c.DefaultQuery("max_items", "0"))

	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if existsCount == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	cmd := SpiderCommand{
		Action:    "test",
		ProjectID: id,
		MaxItems:  maxItems,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	sessionID := fmt.Sprintf("test_%d", id)
	c.JSON(200, gin.H{"success": true, "message": "测试已启动", "session_id": sessionID})
}

// TestStop 停止测试
func (h *SpidersHandler) TestStop(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := SpiderCommand{
		Action:    "test_stop",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	c.JSON(200, gin.H{"success": true, "message": "测试已停止"})
}

// Stop 停止项目
func (h *SpidersHandler) Stop(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))
	clearQueue := c.Query("clear_queue") == "true"

	cmd := SpiderCommand{
		Action:    "stop",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	sqlxDB.Exec("UPDATE spider_projects SET status = 'idle', last_error = ? WHERE id = ?",
		"用户手动停止", id)

	message := "已停止"
	if clearQueue {
		message += "并清空队列"
	}
	c.JSON(200, gin.H{"success": true, "message": message})
}

// Pause 暂停项目
func (h *SpidersHandler) Pause(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := SpiderCommand{
		Action:    "pause",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	c.JSON(200, gin.H{"success": true, "message": "已暂停"})
}

// Resume 恢复项目
func (h *SpidersHandler) Resume(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	cmd := SpiderCommand{
		Action:    "resume",
		ProjectID: id,
		Timestamp: time.Now().Unix(),
	}
	publishCommand(redisClient, cmd)

	c.JSON(200, gin.H{"success": true, "message": "已恢复"})
}

// ============================================
// 统计和队列管理 API
// ============================================

// GetRealtimeStats 获取实时统计
func (h *SpidersHandler) GetRealtimeStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": SpiderStats{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": SpiderStats{}})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	ctx := context.Background()
	statsKey := fmt.Sprintf("spider:stats:%d", id)
	statsData, err := redisClient.HGetAll(ctx, statsKey).Result()

	stats := SpiderStats{Status: status}
	if err == nil && len(statsData) > 0 {
		stats.Total, _ = strconv.Atoi(statsData["total"])
		stats.Completed, _ = strconv.Atoi(statsData["completed"])
		stats.Failed, _ = strconv.Atoi(statsData["failed"])
		stats.Retried, _ = strconv.Atoi(statsData["retried"])
		stats.Pending, _ = strconv.Atoi(statsData["pending"])
		stats.Processing, _ = strconv.Atoi(statsData["processing"])

		totalDone := stats.Completed + stats.Failed
		if totalDone > 0 {
			stats.SuccessRate = math.Round(float64(stats.Completed)/float64(totalDone)*10000) / 100
		}
	}

	c.JSON(200, gin.H{"success": true, "data": stats})
}

// GetChartStats 获取历史图表数据
func (h *SpidersHandler) GetChartStats(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	var data []StatsChartPoint
	err := sqlxDB.Select(&data, `
		SELECT period_start as time, total, completed, failed, retried, avg_speed
		FROM spider_stats_history
		WHERE project_id = ? AND period_type = ?
		ORDER BY period_start DESC
		LIMIT ?
	`, id, period, limit)

	if err != nil || data == nil {
		data = []StatsChartPoint{}
	}

	c.JSON(200, gin.H{"success": true, "data": data})
}

// ClearQueue 清空队列
func (h *SpidersHandler) ClearQueue(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，请先停止"})
		return
	}

	ctx := context.Background()
	keys := []string{
		fmt.Sprintf("spider:queue:%d", id),
		fmt.Sprintf("spider:seen:%d", id),
		fmt.Sprintf("spider:stats:%d", id),
	}
	redisClient.Del(ctx, keys...)

	c.JSON(200, gin.H{"success": true, "message": "队列已清空"})
}

// Reset 重置项目
func (h *SpidersHandler) Reset(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，请先停止"})
		return
	}

	ctx := context.Background()
	keys := []string{
		fmt.Sprintf("spider:queue:%d", id),
		fmt.Sprintf("spider:seen:%d", id),
		fmt.Sprintf("spider:stats:%d", id),
	}
	redisClient.Del(ctx, keys...)

	result, _ := sqlxDB.Exec("DELETE FROM spider_failed_requests WHERE project_id = ?", id)
	affected, _ := result.RowsAffected()

	c.JSON(200, gin.H{
		"success": true,
		"message": fmt.Sprintf("项目已重置，清空了 %d 条失败记录", affected),
	})
}

// ============================================
// 失败请求管理 API
// ============================================

// ListFailed 获取失败请求列表
func (h *SpidersHandler) ListFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}, "total": 0})
		return
	}
	sqlxDB := db.(*sqlx.DB)

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

	var total int
	sqlxDB.Get(&total, "SELECT COUNT(*) FROM spider_failed_requests WHERE "+where, args...)

	offset := (page - 1) * pageSize
	args = append(args, pageSize, offset)

	var data []SpiderFailedRequest
	sqlxDB.Select(&data, `
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
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": map[string]int{"pending": 0, "retried": 0, "ignored": 0, "total": 0}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var stats struct {
		Pending int `db:"pending"`
		Retried int `db:"retried"`
		Ignored int `db:"ignored"`
	}

	sqlxDB.Get(&stats, `
		SELECT
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) as pending,
			COALESCE(SUM(CASE WHEN status = 'retried' THEN 1 ELSE 0 END), 0) as retried,
			COALESCE(SUM(CASE WHEN status = 'ignored' THEN 1 ELSE 0 END), 0) as ignored
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
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	id, _ := strconv.Atoi(c.Param("id"))

	var failed []SpiderFailedRequest
	sqlxDB.Select(&failed, `
		SELECT id, url, method, callback, meta
		FROM spider_failed_requests
		WHERE project_id = ? AND status = 'pending'
	`, id)

	ctx := context.Background()
	queueKey := fmt.Sprintf("spider:queue:%d", id)

	count := 0
	for _, f := range failed {
		reqData, _ := json.Marshal(map[string]interface{}{
			"url":      f.URL,
			"method":   f.Method,
			"callback": f.Callback,
			"meta":     f.Meta,
		})
		redisClient.LPush(ctx, queueKey, reqData)
		sqlxDB.Exec("UPDATE spider_failed_requests SET status = 'retried' WHERE id = ?", f.ID)
		count++
	}

	c.JSON(200, gin.H{"success": true, "message": fmt.Sprintf("已重试 %d 个失败请求", count), "count": count})
}

// RetryOneFailed 重试单个失败请求
func (h *SpidersHandler) RetryOneFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectID, _ := strconv.Atoi(c.Param("id"))
	failedID, _ := strconv.Atoi(c.Param("fid"))

	var f SpiderFailedRequest
	err := sqlxDB.Get(&f, `
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
	redisClient.LPush(ctx, queueKey, reqData)
	sqlxDB.Exec("UPDATE spider_failed_requests SET status = 'retried' WHERE id = ?", failedID)

	c.JSON(200, gin.H{"success": true, "message": "已重试"})
}

// IgnoreFailed 忽略失败请求
func (h *SpidersHandler) IgnoreFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	failedID, _ := strconv.Atoi(c.Param("fid"))

	result, _ := sqlxDB.Exec("UPDATE spider_failed_requests SET status = 'ignored' WHERE id = ?", failedID)
	affected, _ := result.RowsAffected()

	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "已忽略"})
}

// DeleteFailed 删除失败请求
func (h *SpidersHandler) DeleteFailed(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	failedID, _ := strconv.Atoi(c.Param("fid"))

	result, _ := sqlxDB.Exec("DELETE FROM spider_failed_requests WHERE id = ?", failedID)
	affected, _ := result.RowsAffected()

	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "失败请求不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "已删除"})
}

// ============================================
// 代码模板 API
// ============================================

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
        results = []
        for seed in seed_keywords:
            suggestions = await fetch_baidu_suggestions(seed)
            if suggestions:
                results.append({
                    "type": "keywords",
                    "keywords": suggestions,
                })
        return results

    return asyncio.run(run())
`,
		},
	}

	c.JSON(200, gin.H{"success": true, "data": templates})
}

// ============================================
// 爬虫统计 API
// ============================================

// GetOverview 获取统计概览（从 Redis 读取实时数据）
func (h *SpiderStatsHandler) GetOverview(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": map[string]interface{}{
			"total": 0, "completed": 0, "failed": 0, "retried": 0, "success_rate": 0, "avg_speed": 0,
		}})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectIDStr := c.Query("project_id")
	ctx := context.Background()

	var total, completed, failed, retried int64

	if projectIDStr != "" && projectIDStr != "0" {
		// 单个项目统计
		projectID, _ := strconv.Atoi(projectIDStr)
		statsKey := fmt.Sprintf("spider:%d:stats", projectID)
		statsData, err := redisClient.HGetAll(ctx, statsKey).Result()
		if err == nil && len(statsData) > 0 {
			total, _ = strconv.ParseInt(statsData["total"], 10, 64)
			completed, _ = strconv.ParseInt(statsData["completed"], 10, 64)
			failed, _ = strconv.ParseInt(statsData["failed"], 10, 64)
			retried, _ = strconv.ParseInt(statsData["retried"], 10, 64)
		}
	} else {
		// 全部项目统计：扫描所有 spider:*:stats 键
		iter := redisClient.Scan(ctx, 0, "spider:*:stats", 100).Iterator()
		for iter.Next(ctx) {
			key := iter.Val()
			// 排除 archived 和 test 键
			if strings.Contains(key, ":archived") || strings.HasPrefix(key, "test_spider:") {
				continue
			}
			statsData, err := redisClient.HGetAll(ctx, key).Result()
			if err == nil {
				t, _ := strconv.ParseInt(statsData["total"], 10, 64)
				comp, _ := strconv.ParseInt(statsData["completed"], 10, 64)
				f, _ := strconv.ParseInt(statsData["failed"], 10, 64)
				r, _ := strconv.ParseInt(statsData["retried"], 10, 64)
				total += t
				completed += comp
				failed += f
				retried += r
			}
		}
		// 检查迭代器错误
		if err := iter.Err(); err != nil {
			c.JSON(500, gin.H{"success": false, "message": "Redis 扫描失败"})
			return
		}
	}

	var successRate float64
	totalDone := completed + failed
	if totalDone > 0 {
		successRate = math.Round(float64(completed)/float64(totalDone)*10000) / 100
	}

	c.JSON(200, gin.H{"success": true, "data": gin.H{
		"total":        total,
		"completed":    completed,
		"failed":       failed,
		"retried":      retried,
		"success_rate": successRate,
		"avg_speed":    0, // 实时统计不计算速度
	}})
}

// GetChart 获取图表数据
func (h *SpiderStatsHandler) GetChart(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	projectIDStr := c.Query("project_id")
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	// 周期回退顺序
	periodFallback := map[string]string{
		"month": "day",
		"day":   "hour",
		"hour":  "minute",
	}

	// 尝试查询，如果没有数据则回退
	for {
		where := "period_type = ?"
		args := []interface{}{period}

		if projectIDStr != "" && projectIDStr != "0" {
			projectID, _ := strconv.Atoi(projectIDStr)
			where += " AND project_id = ?"
			args = append(args, projectID)
		}

		args = append(args, limit)

		var data []StatsChartPoint
		err := sqlxDB.Select(&data, `
			SELECT period_start as time, SUM(total) as total, SUM(completed) as completed,
			       SUM(failed) as failed, SUM(retried) as retried, AVG(avg_speed) as avg_speed
			FROM spider_stats_history
			WHERE `+where+`
			GROUP BY period_start
			ORDER BY period_start DESC
			LIMIT ?
		`, args...)

		if err == nil && len(data) > 0 {
			// 反转为时间正序
			for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
				data[i], data[j] = data[j], data[i]
			}
			c.JSON(200, gin.H{"success": true, "data": data})
			return
		}

		// 回退到更细粒度
		fallback, ok := periodFallback[period]
		if !ok {
			// 已经是最细粒度，返回空
			c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
			return
		}
		period = fallback
	}
}

// GetScheduled 获取已调度项目
func (h *SpiderStatsHandler) GetScheduled(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var projects []struct {
		ID       int     `db:"id" json:"id"`
		Name     string  `db:"name" json:"name"`
		Schedule *string `db:"schedule" json:"schedule"`
		Enabled  int     `db:"enabled" json:"enabled"`
	}

	sqlxDB.Select(&projects, `
		SELECT id, name, schedule, enabled
		FROM spider_projects
		WHERE schedule IS NOT NULL AND schedule != ''
		ORDER BY id
	`)

	c.JSON(200, gin.H{"success": true, "data": projects})
}

// GetByProject 按项目统计（从 Redis 读取实时数据）
func (h *SpiderStatsHandler) GetByProject(c *gin.Context) {
	db, dbExists := c.Get("db")
	rdb, redisExists := c.Get("redis")
	if !dbExists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	if !redisExists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)
	redisClient := rdb.(*redis.Client)
	ctx := context.Background()

	// 获取所有项目
	var projects []struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	if err := sqlxDB.Select(&projects, "SELECT id, name FROM spider_projects ORDER BY id"); err != nil {
		c.JSON(500, gin.H{"success": false, "message": "查询项目列表失败"})
		return
	}

	// 从 Redis 获取每个项目的统计
	result := make([]gin.H, 0, len(projects))
	for _, p := range projects {
		statsKey := fmt.Sprintf("spider:%d:stats", p.ID)
		statsData, err := redisClient.HGetAll(ctx, statsKey).Result()

		var total, completed, failed, retried int64
		if err == nil && len(statsData) > 0 {
			total, _ = strconv.ParseInt(statsData["total"], 10, 64)
			completed, _ = strconv.ParseInt(statsData["completed"], 10, 64)
			failed, _ = strconv.ParseInt(statsData["failed"], 10, 64)
			retried, _ = strconv.ParseInt(statsData["retried"], 10, 64)
		}

		var successRate float64
		totalDone := completed + failed
		if totalDone > 0 {
			successRate = math.Round(float64(completed)/float64(totalDone)*10000) / 100
		}

		result = append(result, gin.H{
			"project_id":   p.ID,
			"project_name": p.Name,
			"total":        total,
			"completed":    completed,
			"failed":       failed,
			"retried":      retried,
			"success_rate": successRate,
		})
	}

	c.JSON(200, gin.H{"success": true, "data": result})
}
