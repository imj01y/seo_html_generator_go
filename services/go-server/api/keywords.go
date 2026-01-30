// go-page-server/api/keywords.go
package api

import (
	"database/sql"
	"fmt"
	"io"
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

	query := fmt.Sprintf("UPDATE keywords SET status = 0 WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
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

	if req.GroupID != nil {
		result, err = h.db.Exec("UPDATE keywords SET status = 0 WHERE group_id = ? AND status = 1", *req.GroupID)
	} else {
		result, err = h.db.Exec("UPDATE keywords SET status = 0 WHERE status = 1")
	}

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	deleted, _ := result.RowsAffected()
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
		} else {
			skipped++
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

	// 批量插入
	added := 0
	skipped := 0

	for _, kw := range keywords {
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
		} else {
			skipped++
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

// Stats 获取统计信息
// GET /api/keywords/stats
func (h *KeywordsHandler) Stats(c *gin.Context) {
	if h.db == nil {
		core.Success(c, gin.H{"total": 0, "groups": []interface{}{}})
		return
	}

	// 获取各分组统计
	type GroupStat struct {
		GroupID   int    `db:"group_id" json:"group_id"`
		GroupName string `db:"group_name" json:"group_name"`
		Count     int    `db:"count" json:"count"`
	}

	var stats []GroupStat
	h.db.Select(&stats, `
		SELECT kg.id as group_id, kg.name as group_name, COUNT(k.id) as count
		FROM keyword_groups kg
		LEFT JOIN keywords k ON k.group_id = kg.id AND k.status = 1
		WHERE kg.status = 1
		GROUP BY kg.id, kg.name
		ORDER BY kg.is_default DESC, kg.name
	`)

	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM keywords WHERE status = 1")

	core.Success(c, gin.H{
		"total":  total,
		"groups": stats,
	})
}

// Random 随机获取关键词
// GET /api/keywords/random
func (h *KeywordsHandler) Random(c *gin.Context) {
	count, _ := strconv.Atoi(c.DefaultQuery("count", "10"))
	groupID := c.Query("group_id")

	if count < 1 {
		count = 10
	}
	if count > 100 {
		count = 100
	}

	if h.db == nil {
		core.Success(c, gin.H{"keywords": []string{}, "count": 0})
		return
	}

	where := "status = 1"
	args := []interface{}{}

	if groupID != "" {
		where += " AND group_id = ?"
		args = append(args, groupID)
	}

	args = append(args, count)
	query := `SELECT keyword FROM keywords WHERE ` + where + ` ORDER BY RAND() LIMIT ?`

	var keywords []string
	if err := h.db.Select(&keywords, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to get random keywords")
		keywords = []string{}
	}

	core.Success(c, gin.H{"keywords": keywords, "count": len(keywords)})
}

// Reload 重新加载（占位，Go 中暂不需要）
// POST /api/keywords/reload
func (h *KeywordsHandler) Reload(c *gin.Context) {
	if h.db == nil {
		core.Success(c, gin.H{"success": true, "total": 0})
		return
	}

	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM keywords WHERE status = 1")

	core.Success(c, gin.H{"success": true, "total": total})
}

// ClearCache 清理缓存（占位，Go 中暂不需要 Redis 缓存）
// POST /api/keywords/cache/clear
func (h *KeywordsHandler) ClearCache(c *gin.Context) {
	// Go 版本暂不使用 Redis 缓存关键词
	core.Success(c, gin.H{"success": true, "cleared": 0, "message": "缓存已清理"})
}