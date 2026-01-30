// go-page-server/api/keywords.go
package api

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// KeywordsHandler 关键词管理 handler
type KeywordsHandler struct {
	db *sqlx.DB
}

// NewKeywordsHandler 创建 KeywordsHandler
func NewKeywordsHandler(db *sqlx.DB) *KeywordsHandler {
	return &KeywordsHandler{db: db}
}

// KeywordGroup 关键词分组
type KeywordGroup struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// KeywordListItem 关键词列表项
type KeywordListItem struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	Keyword   string    `json:"keyword" db:"keyword"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// GroupCreateRequest 创建分组请求
type GroupCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// GroupUpdateRequest 更新分组请求
type GroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *int    `json:"is_default"`
}

// KeywordUpdateRequest 更新关键词请求
type KeywordUpdateRequest struct {
	Keyword *string `json:"keyword"`
	GroupID *int    `json:"group_id"`
	Status  *int    `json:"status"`
}

// BatchIdsRequest 批量ID请求
type BatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// BatchStatusRequest 批量状态更新请求
type BatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// BatchMoveRequest 批量移动请求
type BatchMoveRequest struct {
	IDs     []int `json:"ids" binding:"required"`
	GroupID int   `json:"group_id" binding:"required"`
}

// DeleteAllRequest 删除全部请求
type DeleteAllRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
	GroupID *int `json:"group_id"`
}

// KeywordAddRequest 添加单个关键词请求
type KeywordAddRequest struct {
	Keyword string `json:"keyword" binding:"required"`
	GroupID int    `json:"group_id"`
}

// KeywordBatchAddRequest 批量添加关键词请求
type KeywordBatchAddRequest struct {
	Keywords []string `json:"keywords" binding:"required"`
	GroupID  int      `json:"group_id"`
}

// ListGroups 获取分组列表
// GET /api/keywords/groups
func (h *KeywordsHandler) ListGroups(c *gin.Context) {
	siteGroupID := c.Query("site_group_id")

	if h.db == nil {
		core.Success(c, gin.H{"groups": []KeywordGroup{}})
		return
	}

	where := "status = 1"
	args := []interface{}{}

	if siteGroupID != "" {
		where += " AND site_group_id = ?"
		args = append(args, siteGroupID)
	}

	query := `SELECT id, site_group_id, name, description, is_default, status, created_at
	          FROM keyword_groups WHERE ` + where + ` ORDER BY is_default DESC, name`

	var groups []KeywordGroup
	if err := h.db.Select(&groups, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list keyword groups")
		groups = []KeywordGroup{}
	}

	core.Success(c, gin.H{"groups": groups})
}

// CreateGroup 创建分组
// POST /api/keywords/groups
func (h *KeywordsHandler) CreateGroup(c *gin.Context) {
	var req GroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 如果设为默认，先取消其他默认
	if req.IsDefault {
		h.db.Exec("UPDATE keyword_groups SET is_default = 0 WHERE is_default = 1")
	}

	isDefault := 0
	if req.IsDefault {
		isDefault = 1
	}

	result, err := h.db.Exec(
		`INSERT INTO keyword_groups (site_group_id, name, description, is_default)
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
// PUT /api/keywords/groups/:id
func (h *KeywordsHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的分组 ID")
		return
	}

	var req GroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查分组是否存在
	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM keyword_groups WHERE id = ? AND status = 1", id); err != nil {
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
			h.db.Exec("UPDATE keyword_groups SET is_default = 0 WHERE is_default = 1")
		}
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	query := "UPDATE keyword_groups SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "分组名称已存在"})
			return
		}
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// DeleteGroup 删除分组
// DELETE /api/keywords/groups/:id
func (h *KeywordsHandler) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的分组 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查是否有站点在使用
	var sitesCount int
	h.db.Get(&sitesCount, "SELECT COUNT(*) FROM sites WHERE keyword_group_id = ?", id)
	if sitesCount > 0 {
		core.Success(c, gin.H{"success": false, "message": fmt.Sprintf("无法删除：有 %d 个站点正在使用此分组", sitesCount)})
		return
	}

	// 检查是否是默认分组
	var isDefault int
	h.db.Get(&isDefault, "SELECT is_default FROM keyword_groups WHERE id = ?", id)
	if isDefault == 1 {
		core.Success(c, gin.H{"success": false, "message": "不能删除默认分组"})
		return
	}

	// 软删除
	if _, err := h.db.Exec("UPDATE keyword_groups SET status = 0 WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// List 获取关键词列表
// GET /api/keywords/list
func (h *KeywordsHandler) List(c *gin.Context) {
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
		core.SuccessPaged(c, []KeywordListItem{}, 0, page, pageSize)
		return
	}

	where := "group_id = ? AND status = 1"
	args := []interface{}{groupID}

	if search != "" {
		where += " AND keyword LIKE ?"
		args = append(args, "%"+search+"%")
	}

	// 获取总数
	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM keywords WHERE "+where, args...)

	// 获取列表
	args = append(args, pageSize, offset)
	query := `SELECT id, group_id, keyword, status, created_at
	          FROM keywords WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	var items []KeywordListItem
	if err := h.db.Select(&items, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list keywords")
		items = []KeywordListItem{}
	}

	core.SuccessPaged(c, items, total, page, pageSize)
}

// Update 更新关键词
// PUT /api/keywords/:id
func (h *KeywordsHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的关键词 ID")
		return
	}

	var req KeywordUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 检查是否存在
	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM keywords WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "关键词不存在"})
		return
	}

	updates := []string{}
	args := []interface{}{}

	if req.Keyword != nil && *req.Keyword != "" {
		updates = append(updates, "keyword = ?")
		args = append(args, *req.Keyword)
	}
	if req.GroupID != nil {
		updates = append(updates, "group_id = ?")
		args = append(args, *req.GroupID)
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
	query := "UPDATE keywords SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "关键词已存在"})
			return
		}
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// Delete 删除关键词
// DELETE /api/keywords/:id
func (h *KeywordsHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的关键词 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 软删除
	if _, err := h.db.Exec("UPDATE keywords SET status = 0 WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}