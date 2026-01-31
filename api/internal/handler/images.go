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

	core "seo-generator/api/internal/service"
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
	if err := h.db.Get(&isDefault, "SELECT is_default FROM image_groups WHERE id = ?", id); err != nil {
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

	// 物理删除分组及其下所有图片
	tx, err := h.db.Begin()
	if err != nil {
		core.Success(c, gin.H{"success": false, "message": "开启事务失败"})
		return
	}
	defer tx.Rollback()

	// 先删除分组下的所有图片
	if _, err := tx.Exec("DELETE FROM images WHERE group_id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}
	// 再删除分组
	if _, err := tx.Exec("DELETE FROM image_groups WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		core.Success(c, gin.H{"success": false, "message": "提交事务失败"})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// ListURLs 获取图片URL列表
// GET /api/images/urls/list
func (h *ImagesHandler) ListURLs(c *gin.Context) {
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
		core.SuccessPaged(c, []ImageListItem{}, 0, page, pageSize)
		return
	}

	where := "group_id = ? AND status = 1"
	args := []interface{}{groupID}

	if search != "" {
		where += " AND url LIKE ?"
		args = append(args, "%"+search+"%")
	}

	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM images WHERE "+where, args...)

	args = append(args, pageSize, offset)
	query := `SELECT id, group_id, url, status, created_at
	          FROM images WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	var items []ImageListItem
	if err := h.db.Select(&items, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list images")
		items = []ImageListItem{}
	}

	core.SuccessPaged(c, items, total, page, pageSize)
}

// AddURL 添加单个图片URL
// POST /api/images/urls/add
func (h *ImagesHandler) AddURL(c *gin.Context) {
	var req ImageAddRequest
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
		"INSERT IGNORE INTO images (group_id, url) VALUES (?, ?)",
		groupID, req.URL)

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		core.Success(c, gin.H{"success": false, "message": "图片URL已存在"})
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}

// BatchAddURLs 批量添加图片URL
// POST /api/images/urls/batch
func (h *ImagesHandler) BatchAddURLs(c *gin.Context) {
	var req ImageBatchAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if len(req.URLs) == 0 {
		core.Success(c, gin.H{"success": false, "message": "URL列表不能为空"})
		return
	}

	if len(req.URLs) > 100000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "单次最多添加 100000 个URL")
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

	// 预处理：过滤空URL
	urls := make([]string, 0, len(req.URLs))
	for _, url := range req.URLs {
		url = strings.TrimSpace(url)
		if url != "" {
			urls = append(urls, url)
		}
	}

	if len(urls) == 0 {
		core.Success(c, gin.H{"success": false, "message": "没有有效的URL"})
		return
	}

	// 批量插入（5000条/批 + 事务）
	const batchSize = 5000
	added := 0
	skipped := len(req.URLs) - len(urls) // 空URL已跳过

	tx, err := h.db.Begin()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "开启事务失败")
		return
	}
	defer tx.Rollback()

	for i := 0; i < len(urls); i += batchSize {
		end := i + batchSize
		if end > len(urls) {
			end = len(urls)
		}
		batch := urls[i:end]

		// 构建批量 INSERT 语句
		valueStrings := make([]string, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*2)

		for j, url := range batch {
			valueStrings[j] = "(?, ?)"
			valueArgs = append(valueArgs, groupID, url)
		}

		query := "INSERT IGNORE INTO images (group_id, url) VALUES " + strings.Join(valueStrings, ",")
		result, err := tx.Exec(query, valueArgs...)
		if err != nil {
			log.Warn().Err(err).Int("batch", i/batchSize).Msg("Batch insert images failed")
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

	core.Success(c, gin.H{
		"success": true,
		"added":   added,
		"skipped": skipped,
		"total":   len(req.URLs),
	})
}

// Upload 上传TXT文件批量添加图片URL
// POST /api/images/upload
func (h *ImagesHandler) Upload(c *gin.Context) {
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

	// 解析URL
	lines := strings.Split(string(content), "\n")
	urls := []string{}
	for _, line := range lines {
		url := strings.TrimSpace(line)
		if url != "" {
			urls = append(urls, url)
		}
	}

	if len(urls) == 0 {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件中没有有效的URL")
		return
	}

	if len(urls) > 500000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "单次最多上传 500000 个URL")
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

	for i := 0; i < len(urls); i += batchSize {
		end := i + batchSize
		if end > len(urls) {
			end = len(urls)
		}
		batch := urls[i:end]

		// 构建批量 INSERT 语句
		valueStrings := make([]string, len(batch))
		valueArgs := make([]interface{}, 0, len(batch)*2)

		for j, url := range batch {
			valueStrings[j] = "(?, ?)"
			valueArgs = append(valueArgs, groupID, url)
		}

		query := "INSERT IGNORE INTO images (group_id, url) VALUES " + strings.Join(valueStrings, ",")
		result, err := tx.Exec(query, valueArgs...)
		if err != nil {
			log.Warn().Err(err).Int("batch", i/batchSize).Msg("Batch insert images failed")
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

	core.Success(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功添加 %d 个图片URL，跳过 %d 个重复", added, skipped),
		"total":   len(urls),
		"added":   added,
		"skipped": skipped,
	})
}

// UpdateURL 更新图片URL
// PUT /api/images/urls/:id
func (h *ImagesHandler) UpdateURL(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的图片 ID")
		return
	}

	var req ImageURLUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM images WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "图片不存在"})
		return
	}

	updates := []string{}
	args := []interface{}{}

	if req.URL != nil && *req.URL != "" {
		updates = append(updates, "url = ?")
		args = append(args, *req.URL)
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
	query := "UPDATE images SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		if strings.Contains(err.Error(), "Duplicate") {
			core.Success(c, gin.H{"success": false, "message": "图片URL已存在"})
			return
		}
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// DeleteURL 删除图片URL
// DELETE /api/images/urls/:id
func (h *ImagesHandler) DeleteURL(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的图片 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 物理删除
	if _, err := h.db.Exec("DELETE FROM images WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// BatchDelete 批量删除
// DELETE /api/images/batch
func (h *ImagesHandler) BatchDelete(c *gin.Context) {
	var req ImageBatchIdsRequest
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

	// 物理删除
	query := fmt.Sprintf("DELETE FROM images WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "deleted": len(req.IDs)})
}

// DeleteAll 删除全部
// DELETE /api/images/delete-all
func (h *ImagesHandler) DeleteAll(c *gin.Context) {
	var req ImageDeleteAllRequest
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

	// 物理删除
	if req.GroupID != nil {
		result, err = h.db.Exec("DELETE FROM images WHERE group_id = ?", *req.GroupID)
	} else {
		result, err = h.db.Exec("DELETE FROM images")
	}

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	deleted, _ := result.RowsAffected()
	core.Success(c, gin.H{"success": true, "deleted": deleted})
}

// BatchUpdateStatus 批量更新状态
// PUT /api/images/batch/status
func (h *ImagesHandler) BatchUpdateStatus(c *gin.Context) {
	var req ImageBatchStatusRequest
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

	query := fmt.Sprintf("UPDATE images SET status = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "updated": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "updated": len(req.IDs)})
}

// BatchMove 批量移动分组
// PUT /api/images/batch/move
func (h *ImagesHandler) BatchMove(c *gin.Context) {
	var req ImageBatchMoveRequest
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

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM image_groups WHERE id = ? AND status = 1", req.GroupID); err != nil {
		core.Success(c, gin.H{"success": false, "message": "目标分组不存在", "moved": 0})
		return
	}

	placeholders := strings.Repeat("?,", len(req.IDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := []interface{}{req.GroupID}
	for _, id := range req.IDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE images SET group_id = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "moved": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "moved": len(req.IDs)})
}

// Stats 获取统计信息
// GET /api/images/urls/stats
func (h *ImagesHandler) Stats(c *gin.Context) {
	if h.db == nil {
		core.Success(c, gin.H{"total": 0, "groups": []interface{}{}})
		return
	}

	type GroupStat struct {
		GroupID   int    `db:"group_id" json:"group_id"`
		GroupName string `db:"group_name" json:"group_name"`
		Count     int    `db:"count" json:"count"`
	}

	var stats []GroupStat
	h.db.Select(&stats, `
		SELECT ig.id as group_id, ig.name as group_name, COUNT(i.id) as count
		FROM image_groups ig
		LEFT JOIN images i ON i.group_id = ig.id AND i.status = 1
		WHERE ig.status = 1
		GROUP BY ig.id, ig.name
		ORDER BY ig.is_default DESC, ig.name
	`)

	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM images WHERE status = 1")

	core.Success(c, gin.H{
		"total":  total,
		"groups": stats,
	})
}

// Random 随机获取图片URL
// GET /api/images/urls/random
func (h *ImagesHandler) Random(c *gin.Context) {
	count, _ := strconv.Atoi(c.DefaultQuery("count", "10"))
	groupID := c.Query("group_id")

	if count < 1 {
		count = 10
	}
	if count > 100 {
		count = 100
	}

	if h.db == nil {
		core.Success(c, gin.H{"urls": []string{}, "count": 0})
		return
	}

	where := "status = 1"
	args := []interface{}{}

	if groupID != "" {
		where += " AND group_id = ?"
		args = append(args, groupID)
	}

	args = append(args, count)
	query := `SELECT url FROM images WHERE ` + where + ` ORDER BY RAND() LIMIT ?`

	var urls []string
	if err := h.db.Select(&urls, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to get random images")
		urls = []string{}
	}

	core.Success(c, gin.H{"urls": urls, "count": len(urls)})
}

// Reload 重新加载
// POST /api/images/urls/reload
func (h *ImagesHandler) Reload(c *gin.Context) {
	if h.db == nil {
		core.Success(c, gin.H{"success": true, "total": 0})
		return
	}

	var total int64
	h.db.Get(&total, "SELECT COUNT(*) FROM images WHERE status = 1")

	core.Success(c, gin.H{"success": true, "total": total})
}

// ClearCache 清理缓存
// POST /api/images/cache/clear
func (h *ImagesHandler) ClearCache(c *gin.Context) {
	core.Success(c, gin.H{"success": true, "cleared": 0, "message": "缓存已清理"})
}