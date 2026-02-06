// seo-generator/api/api/keywords.go
package api

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	core "seo-generator/api/internal/service"
)

// KeywordsHandler 关键词管理 handler
type KeywordsHandler struct {
	db           *sqlx.DB
	poolManager  *core.PoolManager
	funcsManager *core.TemplateFuncsManager
}

// NewKeywordsHandler 创建 KeywordsHandler
func NewKeywordsHandler(db *sqlx.DB, poolManager *core.PoolManager, funcsManager *core.TemplateFuncsManager) *KeywordsHandler {
	return &KeywordsHandler{
		db:           db,
		poolManager:  poolManager,
		funcsManager: funcsManager,
	}
}

// asyncReloadKeywordGroup 异步重载关键词分组到 TemplateFuncsManager
func (h *KeywordsHandler) asyncReloadKeywordGroup(groupID int) {
	go func() {
		ctx := context.Background()
		// 1. 等待 PoolManager 重载完成
		h.poolManager.ReloadKeywordGroup(ctx, groupID)
		// 2. 获取最新数据
		keywords := h.poolManager.GetKeywords(groupID)
		rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
		// 3. 同步到 TemplateFuncsManager
		if h.funcsManager != nil {
			h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
		}
	}()
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
	if err := h.db.Get(&sitesCount, "SELECT COUNT(*) FROM sites WHERE keyword_group_id = ?", id); err != nil && err != sql.ErrNoRows {
		log.Warn().Err(err).Int("group_id", id).Msg("Failed to count sites using keyword group")
	}
	if sitesCount > 0 {
		core.Success(c, gin.H{"success": false, "message": fmt.Sprintf("无法删除：有 %d 个站点正在使用此分组", sitesCount)})
		return
	}

	// 检查是否是默认分组
	var isDefault int
	if err := h.db.Get(&isDefault, "SELECT is_default FROM keyword_groups WHERE id = ?", id); err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "分组不存在")
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, "查询分组失败")
		return
	}
	if isDefault == 1 {
		core.Success(c, gin.H{"success": false, "message": "不能删除默认分组"})
		return
	}

	// 物理删除分组及其下所有关键词
	tx, err := h.db.Begin()
	if err != nil {
		core.Success(c, gin.H{"success": false, "message": "开启事务失败"})
		return
	}
	defer tx.Rollback()

	// 先删除分组下的所有关键词
	if _, err := tx.Exec("DELETE FROM keywords WHERE group_id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}
	// 再删除分组
	if _, err := tx.Exec("DELETE FROM keyword_groups WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		core.Success(c, gin.H{"success": false, "message": "提交事务失败"})
		return
	}

	// 删除后重载缓存（分组已删除，清除该分组的缓存）
	if h.poolManager != nil {
		h.asyncReloadKeywordGroup(id)
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

	// 先查询要删除的关键词所属分组
	var groupID int
	h.db.Get(&groupID, "SELECT group_id FROM keywords WHERE id = ?", id)

	// 物理删除
	if _, err := h.db.Exec("DELETE FROM keywords WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 删除后重载分组缓存
	if groupID > 0 && h.poolManager != nil {
		h.asyncReloadKeywordGroup(groupID)
	}

	core.Success(c, gin.H{"success": true})
}

// BatchDelete 批量删除
// DELETE /api/keywords/batch
func (h *KeywordsHandler) BatchDelete(c *gin.Context) {
	var req BatchIdsRequest
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

	// 查询涉及的分组
	var groupIDs []int
	if len(req.IDs) > 0 {
		h.db.Select(&groupIDs, fmt.Sprintf("SELECT DISTINCT group_id FROM keywords WHERE id IN (%s)", placeholders), args...)
	}

	// 物理删除
	query := fmt.Sprintf("DELETE FROM keywords WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	// 重载涉及的分组缓存
	if h.poolManager != nil {
		for _, gid := range groupIDs {
			h.asyncReloadKeywordGroup(gid)
		}
	}

	core.Success(c, gin.H{"success": true, "deleted": len(req.IDs)})
}

// DeleteAll 删除全部
// DELETE /api/keywords/delete-all
func (h *KeywordsHandler) DeleteAll(c *gin.Context) {
	var req DeleteAllRequest
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

	// 物理删除，避免唯一索引冲突导致后续上传失败
	if req.GroupID != nil {
		result, err = h.db.Exec("DELETE FROM keywords WHERE group_id = ?", *req.GroupID)
	} else {
		result, err = h.db.Exec("DELETE FROM keywords")
	}

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	deleted, _ := result.RowsAffected()

	// 删除后重载缓存
	if h.poolManager != nil {
		ctx := context.Background()
		if req.GroupID != nil {
			h.asyncReloadKeywordGroup(*req.GroupID)
		} else {
			// 全部删除，需要重载所有分组
			h.poolManager.RefreshData(ctx, "keywords")
			// 全部删除后，需要重载所有分组到 TemplateFuncsManager
			if h.funcsManager != nil {
				groupIDs := h.poolManager.GetKeywordGroupIDs()
				for _, gid := range groupIDs {
					keywords := h.poolManager.GetKeywords(gid)
					rawKeywords := h.poolManager.GetAllRawKeywords(gid)
					h.funcsManager.ReloadKeywordGroup(gid, keywords, rawKeywords)
				}
			}
		}
	}

	core.Success(c, gin.H{"success": true, "deleted": deleted})
}

// BatchUpdateStatus 批量更新状态
// PUT /api/keywords/batch/status
func (h *KeywordsHandler) BatchUpdateStatus(c *gin.Context) {
	var req BatchStatusRequest
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

	query := fmt.Sprintf("UPDATE keywords SET status = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "updated": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "updated": len(req.IDs)})
}

// BatchMove 批量移动分组
// PUT /api/keywords/batch/move
func (h *KeywordsHandler) BatchMove(c *gin.Context) {
	var req BatchMoveRequest
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

	// 检查目标分组是否存在
	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM keyword_groups WHERE id = ? AND status = 1", req.GroupID); err != nil {
		core.Success(c, gin.H{"success": false, "message": "目标分组不存在", "moved": 0})
		return
	}

	placeholders := strings.Repeat("?,", len(req.IDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := []interface{}{req.GroupID}
	for _, id := range req.IDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE keywords SET group_id = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "moved": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "moved": len(req.IDs)})
}

// BatchAdd 批量添加关键词
// POST /api/keywords/batch
func (h *KeywordsHandler) BatchAdd(c *gin.Context) {
	var req KeywordBatchAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if len(req.Keywords) == 0 {
		core.Success(c, gin.H{"success": false, "message": "关键词列表不能为空"})
		return
	}

	if len(req.Keywords) > 100000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "单次最多添加 100000 个关键词")
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

	// 使用 INSERT IGNORE 批量插入
	added := 0
	skipped := 0
	addedKeywords := []string{}

	for _, kw := range req.Keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			skipped++
			continue
		}

		result, err := h.db.Exec(
			"INSERT IGNORE INTO keywords (group_id, keyword) VALUES (?, ?)",
			groupID, kw)
		if err != nil {
			skipped++
			continue
		}

		affected, _ := result.RowsAffected()
		if affected > 0 {
			added++
			addedKeywords = append(addedKeywords, kw)
		} else {
			skipped++
		}
	}

	// 成功后追加到缓存
	if len(addedKeywords) > 0 && h.poolManager != nil {
		h.poolManager.AppendKeywords(groupID, addedKeywords)
		// 同步到 TemplateFuncsManager（需要重新获取完整数据）
		if h.funcsManager != nil {
			encodedKeywords := h.poolManager.GetKeywords(groupID)
			rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
			h.funcsManager.ReloadKeywordGroup(groupID, encodedKeywords, rawKeywords)
		}
	}

	core.Success(c, gin.H{
		"success": true,
		"added":   added,
		"skipped": skipped,
		"total":   len(req.Keywords),
	})
}

// Add 添加单个关键词
// POST /api/keywords/add
func (h *KeywordsHandler) Add(c *gin.Context) {
	var req KeywordAddRequest
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
		"INSERT IGNORE INTO keywords (group_id, keyword) VALUES (?, ?)",
		groupID, req.Keyword)

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		core.Success(c, gin.H{"success": false, "message": "关键词已存在"})
		return
	}

	id, _ := result.LastInsertId()

	// 成功后追加到缓存
	if h.poolManager != nil {
		h.poolManager.AppendKeywords(groupID, []string{req.Keyword})
		// 同步到 TemplateFuncsManager
		if h.funcsManager != nil {
			encodedKeywords := h.poolManager.GetKeywords(groupID)
			rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
			h.funcsManager.ReloadKeywordGroup(groupID, encodedKeywords, rawKeywords)
		}
	}

	core.Success(c, gin.H{"success": true, "id": id})
}

// Upload 上传TXT文件批量添加
// POST /api/keywords/upload
func (h *KeywordsHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请上传文件")
		return
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".txt") {
		core.FailWithMessage(c, core.ErrInvalidParam, "只支持 .txt 格式文件")
		return
	}

	groupID, _ := strconv.Atoi(c.DefaultPostForm("group_id", "1"))

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 读取文件内容
	f, err := file.Open()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "无法读取文件")
		return
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "无法读取文件内容")
		return
	}

	// 解析关键词
	lines := strings.Split(string(content), "\n")
	keywords := []string{}
	for _, line := range lines {
		kw := strings.TrimSpace(line)
		if kw != "" {
			keywords = append(keywords, kw)
		}
	}

	if len(keywords) == 0 {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件中没有有效的关键词")
		return
	}

	if len(keywords) > 500000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "单次最多上传 500000 个关键词")
		return
	}

	// 批量插入（5000条/批 + 事务）
	const batchSize = 5000
	added := 0
	skipped := 0

	tx, err := h.db.Begin()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "开启事务失败")
		return
	}
	defer tx.Rollback()

	for i := 0; i < len(keywords); i += batchSize {
		end := i + batchSize
		if end > len(keywords) {
			end = len(keywords)
		}
		batch := keywords[i:end]

		// 构建批量 INSERT 语句
		valueStrings := make([]string, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*2)

		for j, kw := range batch {
			valueStrings[j] = "(?, ?)"
			valueArgs = append(valueArgs, groupID, kw)
		}

		query := "INSERT IGNORE INTO keywords (group_id, keyword) VALUES " + strings.Join(valueStrings, ",")
		result, err := tx.Exec(query, valueArgs...)
		if err != nil {
			log.Warn().Err(err).Int("batch", i/batchSize).Msg("Batch insert failed")
			skipped += len(batch)
			continue
		}

		affected, _ := result.RowsAffected()
		added += int(affected)
		skipped += len(batch) - int(affected)
	}

	if err := tx.Commit(); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "提交事务失败")
		return
	}

	// 成功后重载该分组缓存（批量上传难以追踪具体成功的）
	if added > 0 && h.poolManager != nil {
		h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)
		// 同步到 TemplateFuncsManager
		if h.funcsManager != nil {
			keywords := h.poolManager.GetKeywords(groupID)
			rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
			h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
		}
	}

	core.Success(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功添加 %d 个关键词，跳过 %d 个重复", added, skipped),
		"total":   len(keywords),
		"added":   added,
		"skipped": skipped,
	})
}

// Reload 重新加载关键词缓存
// POST /api/keywords/reload
func (h *KeywordsHandler) Reload(c *gin.Context) {
	groupIDStr := c.Query("group_id")

	if h.poolManager != nil {
		if groupIDStr != "" {
			groupID, _ := strconv.Atoi(groupIDStr)
			if groupID > 0 {
				h.poolManager.ReloadKeywordGroup(c.Request.Context(), groupID)
			}
		} else {
			h.poolManager.RefreshData(c.Request.Context(), "keywords")
		}
	}

	// 同步到 TemplateFuncsManager
	if h.funcsManager != nil {
		if groupIDStr != "" {
			groupID, _ := strconv.Atoi(groupIDStr)
			if groupID > 0 {
				keywords := h.poolManager.GetKeywords(groupID)
				rawKeywords := h.poolManager.GetAllRawKeywords(groupID)
				h.funcsManager.ReloadKeywordGroup(groupID, keywords, rawKeywords)
			}
		} else {
			// 重载所有分组
			groupIDs := h.poolManager.GetKeywordGroupIDs()
			for _, gid := range groupIDs {
				keywords := h.poolManager.GetKeywords(gid)
				rawKeywords := h.poolManager.GetAllRawKeywords(gid)
				h.funcsManager.ReloadKeywordGroup(gid, keywords, rawKeywords)
			}
		}
	}

	var total int64
	if h.db != nil {
		h.db.Get(&total, "SELECT COUNT(*) FROM keywords WHERE status = 1")
	}

	core.Success(c, gin.H{"success": true, "total": total})
}
