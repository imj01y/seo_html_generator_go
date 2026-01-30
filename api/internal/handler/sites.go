// seo-generator/api/api/sites.go
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

	"seo-generator/api/internal/service"
)

// SitesHandler 站点管理 handler
type SitesHandler struct {
	db *sqlx.DB
}

// NewSitesHandler 创建 SitesHandler
func NewSitesHandler(db *sqlx.DB) *SitesHandler {
	return &SitesHandler{db: db}
}

// Site 站点
type Site struct {
	ID             int       `json:"id" db:"id"`
	SiteGroupID    int       `json:"site_group_id" db:"site_group_id"`
	Domain         string    `json:"domain" db:"domain"`
	Name           string    `json:"name" db:"name"`
	Template       string    `json:"template" db:"template"`
	KeywordGroupID *int      `json:"keyword_group_id" db:"keyword_group_id"`
	ImageGroupID   *int      `json:"image_group_id" db:"image_group_id"`
	ArticleGroupID *int      `json:"article_group_id" db:"article_group_id"`
	Status         int       `json:"status" db:"status"`
	IcpNumber      *string   `json:"icp_number" db:"icp_number"`
	BaiduToken     *string   `json:"baidu_token" db:"baidu_token"`
	Analytics      *string   `json:"analytics" db:"analytics"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// SiteGroup 站群
type SiteGroup struct {
	ID          int       `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// SiteGroupWithStats 站群（含统计）
type SiteGroupWithStats struct {
	SiteGroup
	SitesCount         int `json:"sites_count" db:"sites_count"`
	KeywordGroupsCount int `json:"keyword_groups_count" db:"keyword_groups_count"`
	ImageGroupsCount   int `json:"image_groups_count" db:"image_groups_count"`
	ArticleGroupsCount int `json:"article_groups_count" db:"article_groups_count"`
	TemplatesCount     int `json:"templates_count" db:"templates_count"`
}

// SiteCreateRequest 创建站点请求
type SiteCreateRequest struct {
	SiteGroupID    int     `json:"site_group_id"`
	Domain         string  `json:"domain" binding:"required"`
	Name           string  `json:"name" binding:"required"`
	Template       string  `json:"template"`
	KeywordGroupID *int    `json:"keyword_group_id"`
	ImageGroupID   *int    `json:"image_group_id"`
	ArticleGroupID *int    `json:"article_group_id"`
	IcpNumber      *string `json:"icp_number"`
	BaiduToken     *string `json:"baidu_token"`
	Analytics      *string `json:"analytics"`
}

// SiteUpdateRequest 更新站点请求
type SiteUpdateRequest struct {
	SiteGroupID    *int    `json:"site_group_id"`
	Name           *string `json:"name"`
	Template       *string `json:"template"`
	KeywordGroupID *int    `json:"keyword_group_id"`
	ImageGroupID   *int    `json:"image_group_id"`
	ArticleGroupID *int    `json:"article_group_id"`
	Status         *int    `json:"status"`
	IcpNumber      *string `json:"icp_number"`
	BaiduToken     *string `json:"baidu_token"`
	Analytics      *string `json:"analytics"`
}

// SiteBatchIdsRequest 批量ID请求
type SiteBatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// SiteBatchStatusRequest 批量状态更新请求
type SiteBatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// SiteGroupCreateRequest 创建站群请求
type SiteGroupCreateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// SiteGroupUpdateRequest 更新站群请求
type SiteGroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Status      *int    `json:"status"`
	IsDefault   *int    `json:"is_default"`
}

// GroupOption 分组选项
type GroupOption struct {
	ID        int    `json:"id" db:"id"`
	Name      string `json:"name" db:"name"`
	IsDefault int    `json:"is_default" db:"is_default"`
}

// SiteTemplateOption 模板选项
type SiteTemplateOption struct {
	ID          int    `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	DisplayName string `json:"display_name" db:"display_name"`
}

// GroupOptionsResponse 站群下的分组选项响应
type GroupOptionsResponse struct {
	KeywordGroups []GroupOption        `json:"keyword_groups"`
	ImageGroups   []GroupOption        `json:"image_groups"`
	ArticleGroups []GroupOption        `json:"article_groups"`
	Templates     []SiteTemplateOption `json:"templates"`
}

// AllGroupOptionsResponse 所有分组选项响应
type AllGroupOptionsResponse struct {
	KeywordGroups []GroupOption `json:"keyword_groups"`
	ImageGroups   []GroupOption `json:"image_groups"`
}

// ============ 站点管理 (5个) ============

// List 获取站点列表
// GET /api/sites
func (h *SitesHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	siteGroupID := c.Query("site_group_id")
	status := c.Query("status")
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	if h.db == nil {
		core.SuccessPaged(c, []Site{}, 0, page, pageSize)
		return
	}

	// 构建查询条件
	where := "1=1"
	args := []interface{}{}

	if siteGroupID != "" {
		where += " AND site_group_id = ?"
		args = append(args, siteGroupID)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if search != "" {
		where += " AND (domain LIKE ? OR name LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	// 获取总数
	var total int64
	countQuery := "SELECT COUNT(*) FROM sites WHERE " + where
	if err := h.db.Get(&total, countQuery, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to count sites")
	}

	// 获取列表
	query := `SELECT id, site_group_id, domain, name, template,
	                 keyword_group_id, image_group_id, article_group_id,
	                 status, icp_number, baidu_token, analytics,
	                 created_at, updated_at
	          FROM sites
	          WHERE ` + where + `
	          ORDER BY id DESC
	          LIMIT ? OFFSET ?`
	args = append(args, pageSize, offset)

	var items []Site
	if err := h.db.Select(&items, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to query sites")
		items = []Site{}
	}

	core.SuccessPaged(c, items, total, page, pageSize)
}

// Create 创建站点
// POST /api/sites
func (h *SitesHandler) Create(c *gin.Context) {
	var req SiteCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 设置默认站群ID
	if req.SiteGroupID == 0 {
		req.SiteGroupID = 1
	}

	result, err := h.db.Exec(
		`INSERT INTO sites (site_group_id, domain, name, template,
		                    keyword_group_id, image_group_id, article_group_id,
		                    icp_number, baidu_token, analytics, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)`,
		req.SiteGroupID, req.Domain, req.Name, req.Template,
		req.KeywordGroupID, req.ImageGroupID, req.ArticleGroupID,
		req.IcpNumber, req.BaiduToken, req.Analytics)

	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "域名已存在"})
			return
		}
		log.Error().Err(err).Msg("Failed to create site")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}

// Get 获取站点详情
// GET /api/sites/:id
func (h *SitesHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站点 ID")
		return
	}

	if h.db == nil {
		core.FailWithCode(c, core.ErrNotFound)
		return
	}

	var site Site
	err = h.db.Get(&site,
		`SELECT id, site_group_id, domain, name, template,
		        keyword_group_id, image_group_id, article_group_id,
		        status, icp_number, baidu_token, analytics,
		        created_at, updated_at
		 FROM sites WHERE id = ?`, id)

	if err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "站点不存在")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("Failed to get site")
		core.FailWithCode(c, core.ErrInternalServer)
		return
	}

	core.Success(c, site)
}

// Update 更新站点
// PUT /api/sites/:id
func (h *SitesHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站点 ID")
		return
	}

	var req SiteUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查站点是否存在
	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM sites WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "站点不存在"})
		return
	}

	// 构建更新语句
	updates := []string{}
	args := []interface{}{}

	if req.SiteGroupID != nil {
		updates = append(updates, "site_group_id = ?")
		args = append(args, *req.SiteGroupID)
	}
	if req.Name != nil {
		updates = append(updates, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Template != nil {
		updates = append(updates, "template = ?")
		args = append(args, *req.Template)
	}
	if req.KeywordGroupID != nil {
		updates = append(updates, "keyword_group_id = ?")
		args = append(args, *req.KeywordGroupID)
	}
	if req.ImageGroupID != nil {
		updates = append(updates, "image_group_id = ?")
		args = append(args, *req.ImageGroupID)
	}
	if req.ArticleGroupID != nil {
		updates = append(updates, "article_group_id = ?")
		args = append(args, *req.ArticleGroupID)
	}
	if req.Status != nil {
		updates = append(updates, "status = ?")
		args = append(args, *req.Status)
	}
	if req.IcpNumber != nil {
		updates = append(updates, "icp_number = ?")
		args = append(args, *req.IcpNumber)
	}
	if req.BaiduToken != nil {
		updates = append(updates, "baidu_token = ?")
		args = append(args, *req.BaiduToken)
	}
	if req.Analytics != nil {
		updates = append(updates, "analytics = ?")
		args = append(args, *req.Analytics)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "没有需要更新的字段"})
		return
	}

	args = append(args, id)
	query := "UPDATE sites SET " + strings.Join(updates, ", ") + ", updated_at = NOW() WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		log.Error().Err(err).Int("id", id).Msg("Failed to update site")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// Delete 删除站点
// DELETE /api/sites/:id
func (h *SitesHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站点 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 软删除
	if _, err := h.db.Exec("UPDATE sites SET status = 0, updated_at = NOW() WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// ============ 站点批量操作 (2个) ============

// BatchDelete 批量删除站点
// DELETE /api/sites/batch/delete
func (h *SitesHandler) BatchDelete(c *gin.Context) {
	var req SiteBatchIdsRequest
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

	query := fmt.Sprintf("UPDATE sites SET status = 0, updated_at = NOW() WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "deleted": len(req.IDs)})
}

// BatchUpdateStatus 批量更新站点状态
// PUT /api/sites/batch/status
func (h *SitesHandler) BatchUpdateStatus(c *gin.Context) {
	var req SiteBatchStatusRequest
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

	query := fmt.Sprintf("UPDATE sites SET status = ?, updated_at = NOW() WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "updated": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "updated": len(req.IDs)})
}

// ============ 站群管理 (6个) ============

// ListGroups 获取站群列表
// GET /api/site-groups
func (h *SitesHandler) ListGroups(c *gin.Context) {
	if h.db == nil {
		core.Success(c, gin.H{"groups": []SiteGroupWithStats{}})
		return
	}

	query := `SELECT
	            sg.id, sg.name, sg.description, sg.is_default, sg.status, sg.created_at, sg.updated_at,
	            COALESCE((SELECT COUNT(*) FROM sites WHERE site_group_id = sg.id AND status = 1), 0) as sites_count,
	            COALESCE((SELECT COUNT(*) FROM keyword_groups WHERE site_group_id = sg.id AND status = 1), 0) as keyword_groups_count,
	            COALESCE((SELECT COUNT(*) FROM image_groups WHERE site_group_id = sg.id AND status = 1), 0) as image_groups_count,
	            COALESCE((SELECT COUNT(*) FROM article_groups WHERE site_group_id = sg.id AND status = 1), 0) as article_groups_count,
	            COALESCE((SELECT COUNT(*) FROM templates WHERE site_group_id = sg.id AND status = 1), 0) as templates_count
	          FROM site_groups sg
	          WHERE sg.status = 1
	          ORDER BY sg.is_default DESC, sg.id`

	var groups []SiteGroupWithStats
	if err := h.db.Select(&groups, query); err != nil {
		log.Warn().Err(err).Msg("Failed to list site groups")
		groups = []SiteGroupWithStats{}
	}

	core.Success(c, gin.H{"groups": groups})
}

// GetGroup 获取站群详情
// GET /api/site-groups/:id
func (h *SitesHandler) GetGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站群 ID")
		return
	}

	if h.db == nil {
		core.FailWithCode(c, core.ErrNotFound)
		return
	}

	query := `SELECT
	            sg.id, sg.name, sg.description, sg.is_default, sg.status, sg.created_at, sg.updated_at,
	            COALESCE((SELECT COUNT(*) FROM sites WHERE site_group_id = sg.id AND status = 1), 0) as sites_count,
	            COALESCE((SELECT COUNT(*) FROM keyword_groups WHERE site_group_id = sg.id AND status = 1), 0) as keyword_groups_count,
	            COALESCE((SELECT COUNT(*) FROM image_groups WHERE site_group_id = sg.id AND status = 1), 0) as image_groups_count,
	            COALESCE((SELECT COUNT(*) FROM article_groups WHERE site_group_id = sg.id AND status = 1), 0) as article_groups_count,
	            COALESCE((SELECT COUNT(*) FROM templates WHERE site_group_id = sg.id AND status = 1), 0) as templates_count
	          FROM site_groups sg
	          WHERE sg.id = ?`

	var group SiteGroupWithStats
	if err := h.db.Get(&group, query, id); err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "站群不存在")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("Failed to get site group")
		core.FailWithCode(c, core.ErrInternalServer)
		return
	}

	core.Success(c, group)
}

// GetGroupOptions 获取站群下的各类分组选项
// GET /api/site-groups/:id/options
func (h *SitesHandler) GetGroupOptions(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站群 ID")
		return
	}

	if h.db == nil {
		core.Success(c, GroupOptionsResponse{
			KeywordGroups: []GroupOption{},
			ImageGroups:   []GroupOption{},
			ArticleGroups: []GroupOption{},
			Templates:     []SiteTemplateOption{},
		})
		return
	}

	response := GroupOptionsResponse{
		KeywordGroups: []GroupOption{},
		ImageGroups:   []GroupOption{},
		ArticleGroups: []GroupOption{},
		Templates:     []SiteTemplateOption{},
	}

	// 获取关键词分组
	h.db.Select(&response.KeywordGroups,
		`SELECT id, name, is_default FROM keyword_groups
		 WHERE site_group_id = ? AND status = 1
		 ORDER BY is_default DESC, name`, id)

	// 获取图片分组
	h.db.Select(&response.ImageGroups,
		`SELECT id, name, is_default FROM image_groups
		 WHERE site_group_id = ? AND status = 1
		 ORDER BY is_default DESC, name`, id)

	// 获取文章分组
	h.db.Select(&response.ArticleGroups,
		`SELECT id, name, is_default FROM article_groups
		 WHERE site_group_id = ? AND status = 1
		 ORDER BY is_default DESC, name`, id)

	// 获取模板
	h.db.Select(&response.Templates,
		`SELECT id, name, display_name FROM templates
		 WHERE site_group_id = ? AND status = 1
		 ORDER BY name`, id)

	core.Success(c, response)
}

// CreateGroup 创建站群
// POST /api/site-groups
func (h *SitesHandler) CreateGroup(c *gin.Context) {
	var req SiteGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	result, err := h.db.Exec(
		`INSERT INTO site_groups (name, description, is_default, status)
		 VALUES (?, ?, 0, 1)`,
		req.Name, req.Description)

	if err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "站群名称已存在"})
			return
		}
		log.Error().Err(err).Msg("Failed to create site group")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}

// UpdateGroup 更新站群
// PUT /api/site-groups/:id
func (h *SitesHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站群 ID")
		return
	}

	var req SiteGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查站群是否存在
	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM site_groups WHERE id = ? AND status = 1", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "站群不存在"})
		return
	}

	// 构建更新语句
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
	if req.Status != nil {
		updates = append(updates, "status = ?")
		args = append(args, *req.Status)
	}
	if req.IsDefault != nil {
		// 如果设为默认，先取消其他默认
		if *req.IsDefault == 1 {
			h.db.Exec("UPDATE site_groups SET is_default = 0 WHERE is_default = 1")
		}
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "没有需要更新的字段"})
		return
	}

	args = append(args, id)
	query := "UPDATE site_groups SET " + strings.Join(updates, ", ") + ", updated_at = NOW() WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "站群名称已存在"})
			return
		}
		log.Error().Err(err).Int("id", id).Msg("Failed to update site group")
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// DeleteGroup 删除站群
// DELETE /api/site-groups/:id
func (h *SitesHandler) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的站群 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查是否是默认站群
	var isDefault int
	h.db.Get(&isDefault, "SELECT is_default FROM site_groups WHERE id = ?", id)
	if isDefault == 1 {
		core.Success(c, gin.H{"success": false, "message": "不能删除默认站群"})
		return
	}

	// 检查是否有站点在使用
	var sitesCount int
	h.db.Get(&sitesCount, "SELECT COUNT(*) FROM sites WHERE site_group_id = ? AND status = 1", id)
	if sitesCount > 0 {
		core.Success(c, gin.H{
			"success": false,
			"message": fmt.Sprintf("无法删除：有 %d 个站点属于此站群", sitesCount),
		})
		return
	}

	// 软删除
	if _, err := h.db.Exec("UPDATE site_groups SET status = 0, updated_at = NOW() WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// ============ 分组选项 (1个) ============

// GetAllGroupOptions 获取所有分组选项
// GET /api/groups/options
func (h *SitesHandler) GetAllGroupOptions(c *gin.Context) {
	siteGroupID := c.Query("site_group_id")

	if h.db == nil {
		core.Success(c, AllGroupOptionsResponse{
			KeywordGroups: []GroupOption{},
			ImageGroups:   []GroupOption{},
		})
		return
	}

	response := AllGroupOptionsResponse{
		KeywordGroups: []GroupOption{},
		ImageGroups:   []GroupOption{},
	}

	if siteGroupID != "" {
		// 获取指定站群的分组
		h.db.Select(&response.KeywordGroups,
			`SELECT id, name, is_default FROM keyword_groups
			 WHERE site_group_id = ? AND status = 1
			 ORDER BY is_default DESC, name`, siteGroupID)

		h.db.Select(&response.ImageGroups,
			`SELECT id, name, is_default FROM image_groups
			 WHERE site_group_id = ? AND status = 1
			 ORDER BY is_default DESC, name`, siteGroupID)
	} else {
		// 获取所有分组
		h.db.Select(&response.KeywordGroups,
			`SELECT id, name, is_default FROM keyword_groups
			 WHERE status = 1
			 ORDER BY is_default DESC, name`)

		h.db.Select(&response.ImageGroups,
			`SELECT id, name, is_default FROM image_groups
			 WHERE status = 1
			 ORDER BY is_default DESC, name`)
	}

	core.Success(c, response)
}
