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

	core "seo-generator/api/internal/service"
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
