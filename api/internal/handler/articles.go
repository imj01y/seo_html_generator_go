package api

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	core "seo-generator/api/internal/service"
)

// ArticlesHandler 文章管理 handler
type ArticlesHandler struct {
	db  *sqlx.DB
	rdb *redis.Client
}

// NewArticlesHandler 创建 ArticlesHandler
func NewArticlesHandler(db *sqlx.DB, rdb *redis.Client) *ArticlesHandler {
	return &ArticlesHandler{db: db, rdb: rdb}
}

// ArticleGroup 文章分组
type ArticleGroup struct {
	ID          int       `json:"id" db:"id"`
	SiteGroupID int       `json:"site_group_id" db:"site_group_id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	IsDefault   int       `json:"is_default" db:"is_default"`
	Status      int       `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ArticleListItem 文章列表项
type ArticleListItem struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	Title     string    `json:"title" db:"title"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// ArticleDetail 文章详情
type ArticleDetail struct {
	ID        int       `json:"id" db:"id"`
	GroupID   int       `json:"group_id" db:"group_id"`
	Title     string    `json:"title" db:"title"`
	Content   string    `json:"content" db:"content"`
	Status    int       `json:"status" db:"status"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ArticleGroupCreateRequest 创建分组请求
type ArticleGroupCreateRequest struct {
	SiteGroupID int    `json:"site_group_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IsDefault   bool   `json:"is_default"`
}

// ArticleGroupUpdateRequest 更新分组请求
type ArticleGroupUpdateRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsDefault   *int    `json:"is_default"`
}

// ArticleUpdateRequest 更新文章请求
type ArticleUpdateRequest struct {
	GroupID *int    `json:"group_id"`
	Title   *string `json:"title"`
	Content *string `json:"content"`
	Status  *int    `json:"status"`
}

// ArticleBatchIdsRequest 批量ID请求
type ArticleBatchIdsRequest struct {
	IDs []int `json:"ids" binding:"required"`
}

// ArticleBatchStatusRequest 批量状态更新请求
type ArticleBatchStatusRequest struct {
	IDs    []int `json:"ids" binding:"required"`
	Status int   `json:"status"`
}

// ArticleBatchMoveRequest 批量移动请求
type ArticleBatchMoveRequest struct {
	IDs     []int `json:"ids" binding:"required"`
	GroupID int   `json:"group_id" binding:"required"`
}

// ArticleDeleteAllRequest 删除全部请求
type ArticleDeleteAllRequest struct {
	Confirm bool `json:"confirm" binding:"required"`
	GroupID *int `json:"group_id"`
}

// ArticleAddRequest 添加单篇文章请求
type ArticleAddRequest struct {
	GroupID int    `json:"group_id"`
	Title   string `json:"title" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// ArticleBatchAddItem 批量添加文章项
type ArticleBatchAddItem struct {
	GroupID int    `json:"group_id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ArticleBatchAddRequest 批量添加文章请求
type ArticleBatchAddRequest struct {
	Articles []ArticleBatchAddItem `json:"articles" binding:"required"`
}

// ========== 分组管理方法 ==========

// ListGroups 获取分组列表
// GET /api/articles/groups
func (h *ArticlesHandler) ListGroups(c *gin.Context) {
	siteGroupID := c.Query("site_group_id")

	if h.db == nil {
		core.Success(c, gin.H{"groups": []ArticleGroup{}})
		return
	}

	where := "status = 1"
	args := []interface{}{}

	if siteGroupID != "" {
		where += " AND site_group_id = ?"
		args = append(args, siteGroupID)
	}

	query := `SELECT id, site_group_id, name, description, is_default, status, created_at
	          FROM article_groups WHERE ` + where + ` ORDER BY is_default DESC, name`

	var groups []ArticleGroup
	if err := h.db.Select(&groups, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list article groups")
		groups = []ArticleGroup{}
	}

	core.Success(c, gin.H{"groups": groups})
}

// CreateGroup 创建分组
// POST /api/articles/groups
func (h *ArticlesHandler) CreateGroup(c *gin.Context) {
	var req ArticleGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	isDefault := 0
	if req.IsDefault {
		isDefault = 1
	}

	// 使用事务确保设置默认分组的原子性
	tx, err := h.db.Beginx()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "开启事务失败")
		return
	}
	defer tx.Rollback()

	if req.IsDefault {
		if _, err := tx.Exec("UPDATE article_groups SET is_default = 0 WHERE is_default = 1"); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, "更新默认分组失败")
			return
		}
	}

	result, err := tx.Exec(
		`INSERT INTO article_groups (site_group_id, name, description, is_default)
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

	if err := tx.Commit(); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "提交事务失败")
		return
	}

	id, _ := result.LastInsertId()
	core.Success(c, gin.H{"success": true, "id": id})
}

// UpdateGroup 更新分组
// PUT /api/articles/groups/:id
func (h *ArticlesHandler) UpdateGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的分组 ID")
		return
	}

	var req ArticleGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM article_groups WHERE id = ? AND status = 1", id); err != nil {
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
	needSetDefault := req.IsDefault != nil && *req.IsDefault == 1
	if req.IsDefault != nil {
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		core.Success(c, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	query := "UPDATE article_groups SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	// 使用事务确保设置默认分组的原子性
	tx, err := h.db.Beginx()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "开启事务失败")
		return
	}
	defer tx.Rollback()

	if needSetDefault {
		if _, err := tx.Exec("UPDATE article_groups SET is_default = 0 WHERE is_default = 1"); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, "更新默认分组失败")
			return
		}
	}

	if _, err := tx.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "提交事务失败")
		return
	}

	core.Success(c, gin.H{"success": true})
}

// DeleteGroup 删除分组
// DELETE /api/articles/groups/:id
func (h *ArticlesHandler) DeleteGroup(c *gin.Context) {
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
	if err := h.db.Get(&isDefault, "SELECT is_default FROM article_groups WHERE id = ?", id); err != nil {
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

	// 物理删除分组及其下所有文章
	tx, err := h.db.Begin()
	if err != nil {
		core.Success(c, gin.H{"success": false, "message": "开启事务失败"})
		return
	}
	defer tx.Rollback()

	// 先删除分组下的所有文章
	if _, err := tx.Exec("DELETE FROM original_articles WHERE group_id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}
	// 再删除分组
	if _, err := tx.Exec("DELETE FROM article_groups WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := tx.Commit(); err != nil {
		core.Success(c, gin.H{"success": false, "message": "提交事务失败"})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// ========== 文章 CRUD 方法 ==========

// List 获取文章列表
// GET /api/articles/list
func (h *ArticlesHandler) List(c *gin.Context) {
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
		core.SuccessPaged(c, []ArticleListItem{}, 0, page, pageSize)
		return
	}

	where := "group_id = ? AND status = 1"
	args := []interface{}{groupID}

	if search != "" {
		where += " AND (title LIKE ? OR content LIKE ?)"
		args = append(args, "%"+search+"%", "%"+search+"%")
	}

	var total int64
	if err := h.db.Get(&total, "SELECT COUNT(*) FROM original_articles WHERE "+where, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to count articles")
		total = 0
	}

	args = append(args, pageSize, offset)
	query := `SELECT id, group_id, title, status, created_at
	          FROM original_articles WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`

	var items []ArticleListItem
	if err := h.db.Select(&items, query, args...); err != nil {
		log.Warn().Err(err).Msg("Failed to list articles")
		items = []ArticleListItem{}
	}

	core.SuccessPaged(c, items, total, page, pageSize)
}

// Get 获取单篇文章
// GET /api/articles/:id
func (h *ArticlesHandler) Get(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的文章 ID")
		return
	}

	if h.db == nil {
		core.FailWithCode(c, core.ErrNotFound)
		return
	}

	var article ArticleDetail
	err = h.db.Get(&article,
		`SELECT id, group_id, title, content, status, created_at, updated_at
		 FROM original_articles WHERE id = ?`, id)

	if err != nil {
		if err == sql.ErrNoRows {
			core.FailWithMessage(c, core.ErrNotFound, "文章不存在")
			return
		}
		core.FailWithCode(c, core.ErrInternalServer)
		return
	}

	core.Success(c, article)
}

// Update 更新文章
// PUT /api/articles/:id
func (h *ArticlesHandler) Update(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的文章 ID")
		return
	}

	var req ArticleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	var exists int
	if err := h.db.Get(&exists, "SELECT 1 FROM original_articles WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": "文章不存在"})
		return
	}

	updates := []string{}
	args := []interface{}{}

	if req.GroupID != nil {
		updates = append(updates, "group_id = ?")
		args = append(args, *req.GroupID)
	}
	if req.Title != nil {
		updates = append(updates, "title = ?")
		args = append(args, *req.Title)
	}
	if req.Content != nil {
		updates = append(updates, "content = ?")
		args = append(args, *req.Content)
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
	query := "UPDATE original_articles SET " + strings.Join(updates, ", ") + " WHERE id = ?"

	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// Delete 删除文章
// DELETE /api/articles/:id
func (h *ArticlesHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的文章 ID")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	// 物理删除
	if _, err := h.db.Exec("DELETE FROM original_articles WHERE id = ?", id); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	core.Success(c, gin.H{"success": true})
}

// ========== 批量操作方法 ==========

// BatchDelete 批量删除
// DELETE /api/articles/batch/delete
func (h *ArticlesHandler) BatchDelete(c *gin.Context) {
	var req ArticleBatchIdsRequest
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
	query := fmt.Sprintf("DELETE FROM original_articles WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "deleted": len(req.IDs)})
}

// DeleteAll 删除全部
// DELETE /api/articles/delete-all
func (h *ArticlesHandler) DeleteAll(c *gin.Context) {
	var req ArticleDeleteAllRequest
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
		result, err = h.db.Exec("DELETE FROM original_articles WHERE group_id = ?", *req.GroupID)
	} else {
		result, err = h.db.Exec("DELETE FROM original_articles")
	}

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "deleted": 0})
		return
	}

	deleted, _ := result.RowsAffected()
	core.Success(c, gin.H{"success": true, "deleted": deleted})
}

// BatchUpdateStatus 批量更新状态
// PUT /api/articles/batch/status
func (h *ArticlesHandler) BatchUpdateStatus(c *gin.Context) {
	var req ArticleBatchStatusRequest
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

	query := fmt.Sprintf("UPDATE original_articles SET status = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "updated": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "updated": len(req.IDs)})
}

// BatchMove 批量移动分组
// PUT /api/articles/batch/move
func (h *ArticlesHandler) BatchMove(c *gin.Context) {
	var req ArticleBatchMoveRequest
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
	if err := h.db.Get(&exists, "SELECT 1 FROM article_groups WHERE id = ? AND status = 1", req.GroupID); err != nil {
		core.Success(c, gin.H{"success": false, "message": "目标分组不存在", "moved": 0})
		return
	}

	placeholders := strings.Repeat("?,", len(req.IDs))
	placeholders = placeholders[:len(placeholders)-1]

	args := []interface{}{req.GroupID}
	for _, id := range req.IDs {
		args = append(args, id)
	}

	query := fmt.Sprintf("UPDATE original_articles SET group_id = ? WHERE id IN (%s)", placeholders)
	if _, err := h.db.Exec(query, args...); err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error(), "moved": 0})
		return
	}

	core.Success(c, gin.H{"success": true, "moved": len(req.IDs)})
}

// ========== 添加文章方法 ==========

// Add 添加单篇文章
// POST /api/articles/add
func (h *ArticlesHandler) Add(c *gin.Context) {
	var req ArticleAddRequest
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
		"INSERT IGNORE INTO original_articles (group_id, title, content) VALUES (?, ?, ?)",
		groupID, req.Title, req.Content)

	if err != nil {
		core.Success(c, gin.H{"success": false, "message": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		core.Success(c, gin.H{"success": false, "message": "文章标题已存在"})
		return
	}

	id, _ := result.LastInsertId()

	// 推入待处理队列，由 Python Worker 加工
	h.pushToProcessQueue(c, id)

	core.Success(c, gin.H{"success": true, "id": id})
}

// BatchAdd 批量添加文章
// POST /api/articles/batch
func (h *ArticlesHandler) BatchAdd(c *gin.Context) {
	var req ArticleBatchAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "请求参数错误")
		return
	}

	if len(req.Articles) == 0 {
		core.Success(c, gin.H{"success": false, "message": "文章列表不能为空"})
		return
	}

	if len(req.Articles) > 1000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "单次最多添加 1000 篇文章")
		return
	}

	if h.db == nil {
		core.FailWithMessage(c, core.ErrInternalServer, "数据库未初始化")
		return
	}

	added := 0
	skipped := 0
	var addedIDs []int64

	for _, article := range req.Articles {
		if article.Title == "" || article.Content == "" {
			skipped++
			continue
		}

		groupID := article.GroupID
		if groupID == 0 {
			groupID = 1
		}

		result, err := h.db.Exec(
			"INSERT IGNORE INTO original_articles (group_id, title, content) VALUES (?, ?, ?)",
			groupID, article.Title, article.Content)
		if err != nil {
			skipped++
			continue
		}

		affected, _ := result.RowsAffected()
		if affected > 0 {
			added++
			if id, err := result.LastInsertId(); err == nil {
				addedIDs = append(addedIDs, id)
			}
		} else {
			skipped++
		}
	}

	// 批量推入待处理队列，由 Python Worker 加工
	h.pushBatchToProcessQueue(c, addedIDs)

	core.Success(c, gin.H{
		"success": true,
		"added":   added,
		"skipped": skipped,
		"total":   len(req.Articles),
	})
}

const articlePendingQueue = "pending:articles"

// pushToProcessQueue 将单个文章 ID 推入待处理队列
func (h *ArticlesHandler) pushToProcessQueue(c *gin.Context, id int64) {
	if h.rdb == nil {
		return
	}
	if err := h.rdb.LPush(context.Background(), articlePendingQueue, id).Err(); err != nil {
		log.Warn().Err(err).Int64("article_id", id).Msg("推送文章到待处理队列失败")
	}
}

// pushBatchToProcessQueue 将多个文章 ID 批量推入待处理队列
func (h *ArticlesHandler) pushBatchToProcessQueue(c *gin.Context, ids []int64) {
	if h.rdb == nil || len(ids) == 0 {
		return
	}
	vals := make([]interface{}, len(ids))
	for i, id := range ids {
		vals[i] = id
	}
	if err := h.rdb.LPush(context.Background(), articlePendingQueue, vals...).Err(); err != nil {
		log.Warn().Err(err).Int("count", len(ids)).Msg("批量推送文章到待处理队列失败")
	}
}
