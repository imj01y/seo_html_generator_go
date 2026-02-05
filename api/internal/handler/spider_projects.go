package api

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	models "seo-generator/api/internal/model"
	core "seo-generator/api/internal/service"
)

// SpiderProjectsHandler 爬虫项目处理器
type SpiderProjectsHandler struct{}

// 默认爬虫代码模板
const DefaultSpiderCode = `from loguru import logger
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

// List 获取项目列表
func (h *SpiderProjectsHandler) List(c *gin.Context) {
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

	var projects []models.SpiderProject
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
func (h *SpiderProjectsHandler) Get(c *gin.Context) {
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

	var project models.SpiderProject
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
func (h *SpiderProjectsHandler) Create(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var req models.SpiderProjectCreate
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
		req.Files = append(req.Files, models.SpiderFileCreate{
			Filename: req.EntryFile,
			Content:  DefaultSpiderCode,
		})
	}

	// 插入文件（带错误检查）
	for _, f := range req.Files {
		// 确保文件路径以 / 开头
		filePath := f.Filename
		if !strings.HasPrefix(filePath, "/") {
			filePath = "/" + filePath
		}
		_, err := tx.Exec(`
			INSERT INTO spider_project_files (project_id, path, type, content)
			VALUES (?, ?, 'file', ?)
		`, projectID, filePath, f.Content)
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

	// 同步定时任务配置
	if scheduler, exists := c.Get("scheduler"); exists && req.Schedule != nil && *req.Schedule != "" {
		s := scheduler.(*core.Scheduler)
		ctx := context.Background()
		if err := core.SyncSpiderSchedule(ctx, sqlxDB, s, int(projectID), req.Name, req.Schedule, req.Enabled); err != nil {
			log.Warn().Err(err).Int64("project_id", projectID).Msg("Failed to sync spider schedule")
		}
	}

	c.JSON(200, gin.H{"success": true, "id": projectID, "message": "创建成功"})
}

// Update 更新项目
func (h *SpiderProjectsHandler) Update(c *gin.Context) {
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

	var req models.SpiderProjectUpdate
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

	// 同步定时任务配置（如果 schedule 或 enabled 有变更）
	if scheduler, exists := c.Get("scheduler"); exists && (req.Schedule != nil || req.Enabled != nil) {
		s := scheduler.(*core.Scheduler)
		var project struct {
			Name     string  `db:"name"`
			Schedule *string `db:"schedule"`
			Enabled  int     `db:"enabled"`
		}
		if err := sqlxDB.Get(&project, "SELECT name, schedule, enabled FROM spider_projects WHERE id = ?", id); err == nil {
			ctx := context.Background()
			if syncErr := core.SyncSpiderSchedule(ctx, sqlxDB, s, id, project.Name, project.Schedule, project.Enabled); syncErr != nil {
				log.Warn().Err(syncErr).Int("project_id", id).Msg("Failed to sync spider schedule")
			}
		}
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}

// Delete 删除项目
func (h *SpiderProjectsHandler) Delete(c *gin.Context) {
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

	// 删除定时任务
	if scheduler, exists := c.Get("scheduler"); exists {
		s := scheduler.(*core.Scheduler)
		ctx := context.Background()
		if err := core.DeleteSpiderSchedule(ctx, sqlxDB, s, id); err != nil {
			log.Warn().Err(err).Int("project_id", id).Msg("Failed to delete spider schedule")
		}
	}

	sqlxDB.Exec("DELETE FROM spider_project_files WHERE project_id = ?", id)
	sqlxDB.Exec("DELETE FROM spider_projects WHERE id = ?", id)

	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}

// Toggle 切换启用状态
func (h *SpiderProjectsHandler) Toggle(c *gin.Context) {
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

	// 同步定时任务状态
	if scheduler, exists := c.Get("scheduler"); exists {
		s := scheduler.(*core.Scheduler)
		var project struct {
			Name     string  `db:"name"`
			Schedule *string `db:"schedule"`
		}
		if err := sqlxDB.Get(&project, "SELECT name, schedule FROM spider_projects WHERE id = ?", id); err == nil {
			ctx := context.Background()
			if syncErr := core.SyncSpiderSchedule(ctx, sqlxDB, s, id, project.Name, project.Schedule, newEnabled); syncErr != nil {
				log.Warn().Err(syncErr).Int("project_id", id).Msg("Failed to sync spider schedule")
			}
		}
	}

	message := "已启用"
	if newEnabled == 0 {
		message = "已禁用"
	}

	c.JSON(200, gin.H{"success": true, "enabled": newEnabled, "message": message})
}

// GetCodeTemplates 获取代码模板列表
func (h *SpiderProjectsHandler) GetCodeTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{
			"name":         "api_pagination",
			"display_name": "API 分页抓取",
			"description":  "适用于 JSON API 接口的分页抓取",
			"code":         DefaultSpiderCode,
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
