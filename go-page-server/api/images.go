package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	"go-page-server/core"
)

// ImagesHandler 图片管理 handler
type ImagesHandler struct {
	db *sqlx.DB
}

// NewImagesHandler 创建 ImagesHandler
func NewImagesHandler(db *sqlx.DB) *ImagesHandler {
	return &ImagesHandler{db: db}
}

// ImageGroup 图片分组
type ImageGroup struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ImageListItem 图片列表项
type ImageListItem struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	URL       string    `json:"url" db:"url"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ImageGroupCreateRequest 创建分组请求
type ImageGroupCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// ImageGroupUpdateRequest 更新分组请求
type ImageGroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *int    `json:"is_default"`
}

// ImageURLUpdateRequest 更新图片URL请求
type ImageURLUpdateRequest struct {
	URL     *string `json:"url"`
	GroupID *int    `json:"group_id"`
	Status  *int    `json:"status"`
}

// ImageBatchIdsRequest 批量ID请求
type ImageBatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// ImageBatchStatusRequest 批量状态更新请求
type ImageBatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// ImageBatchMoveRequest 批量移动请求
type ImageBatchMoveRequest struct {
	IDs     []int `json:"ids" binding:"required"`
	GroupID int   `json:"group_id" binding:"required"`
}

// ImageDeleteAllRequest 删除全部请求
type ImageDeleteAllRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
	GroupID *int `json:"group_id"`
}

// ImageAddRequest 添加单个图片请求
type ImageAddRequest struct {
	URL     string `json:"url" binding:"required"`
	GroupID int    `json:"group_id"`
}

// ImageBatchAddRequest 批量添加图片请求
type ImageBatchAddRequest struct {
	URLs    []string `json:"urls" binding:"required"`
	GroupID int      `json:"group_id"`
}

// ListGroups 获取分组列表
// GET /api/images/groups
func (h *ImagesHandler) ListGroups(c *gin.Context) {
	siteGroupID := c.Query("site_group_id")

	if h.db == nil {
		core.Success(c, gin.H{"groups": []ImageGroup{}})
		return
	}

	where := "status = 1"
	args := []interface{}{}

	if siteGroupID != "" {
		where += " AND site_group_id = ?"
		args = append(args, siteGroupID)
	}

	query := `SELECT id, site_group_id, name, description, is_default, status, created_at
	          FROM image_groups WHERE ` + where + ` ORDER BY is_default DESC, name`

	var groups []ImageGroup
	if err := h.db.Select(&groups, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list image groups")
		groups = []ImageGroup{}
	}

	core.Success(c, gin.H{"groups": groups})
}

// CreateGroup 创建分组
// POST /api/images/groups
func (h *ImagesHandler) CreateGroup(c *gin.Context) {
	var req ImageGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	if req.IsDefault {
		h.db.Exec("UPDATE image_groups SET is_default = 0 WHERE is_default = 1")
	}

	isDefault := 0
	if req.IsDefault {
		isDefault = 1
	}

	result, err := h.db.Exec(
		`INSERT INTO image_groups (site_group_id, name, description, is_default)
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
// PUT /api/images/groups/:id
func (h *ImagesHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的分组 ID")
		return
	}

	var req ImageGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM image_groups WHERE id = ? AND status = 1", id); err != nil {
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
			h.db.Exec("UPDATE image_groups SET is_default = 0 WHERE is_default = 1")
		}
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	query := "UPDATE image_groups SET " + strings.Join(updates, ", ") + " WHERE id = ?"

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
// DELETE /api/images/groups/:id
func (h *ImagesHandler) DeleteGroup(c *gin.Context) {
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
	h.db.Get(&isDefault, "SELECT is_default FROM image_groups WHERE id = ?", id)
	if isDefault == 1 {
		core.Success(c, gin.H{"success": false, "message": "不能删除默认分组"})
		return
	}

	if _, err := h.db.Exec("UPDATE image_groups SET status = 0 WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}
