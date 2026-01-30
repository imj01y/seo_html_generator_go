package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// Generator 生成器
type Generator struct {
	ID          int       `db:"id" json:"id"`
	Name        string    `db:"name" json:"name"`
	DisplayName string    `db:"display_name" json:"display_name"`
	Description *string   `db:"description" json:"description"`
	Code        string    `db:"code" json:"code"`
	Enabled     int       `db:"enabled" json:"enabled"`
	IsDefault   int       `db:"is_default" json:"is_default"`
	Version     int       `db:"version" json:"version"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// GeneratorCreate 创建请求
type GeneratorCreate struct {
	Name        string  `json:"name" binding:"required"`
	DisplayName string  `json:"display_name" binding:"required"`
	Description *string `json:"description"`
	Code        string  `json:"code" binding:"required"`
	Enabled     int     `json:"enabled"`
	IsDefault   int     `json:"is_default"`
}

// GeneratorUpdate 更新请求
type GeneratorUpdate struct {
	DisplayName *string `json:"display_name"`
	Description *string `json:"description"`
	Code        *string `json:"code"`
	Enabled     *int    `json:"enabled"`
	IsDefault   *int    `json:"is_default"`
}

// GeneratorsHandler 生成器处理器
type GeneratorsHandler struct{}

// List 获取生成器列表
func (h *GeneratorsHandler) List(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}, "total": 0})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	enabledStr := c.Query("enabled")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	where := "1=1"
	args := []interface{}{}

	if enabledStr != "" {
		enabled, _ := strconv.Atoi(enabledStr)
		where += " AND enabled = ?"
		args = append(args, enabled)
	}

	var total int
	sqlxDB.Get(&total, "SELECT COUNT(*) FROM content_generators WHERE "+where, args...)

	offset := (page - 1) * pageSize
	args = append(args, offset, pageSize)

	var generators []Generator
	sqlxDB.Select(&generators, `
		SELECT id, name, display_name, description, code, enabled, is_default, version, created_at, updated_at
		FROM content_generators
		WHERE `+where+`
		ORDER BY is_default DESC, id ASC
		LIMIT ?, ?
	`, args...)

	c.JSON(200, gin.H{
		"success":   true,
		"data":      generators,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// Get 获取生成器详情
func (h *GeneratorsHandler) Get(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(404, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var gen Generator
	err := sqlxDB.Get(&gen, `
		SELECT id, name, display_name, description, code, enabled, is_default, version, created_at, updated_at
		FROM content_generators WHERE id = ?
	`, id)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": gen})
}

// Create 创建生成器
func (h *GeneratorsHandler) Create(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var req GeneratorCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM content_generators WHERE name = ?", req.Name)
	if existsCount > 0 {
		c.JSON(400, gin.H{"success": false, "message": "生成器名称已存在"})
		return
	}

	if req.IsDefault == 1 {
		sqlxDB.Exec("UPDATE content_generators SET is_default = 0 WHERE is_default = 1")
	}

	result, err := sqlxDB.Exec(`
		INSERT INTO content_generators (name, display_name, description, code, enabled, is_default)
		VALUES (?, ?, ?, ?, ?, ?)
	`, req.Name, req.DisplayName, req.Description, req.Code, req.Enabled, req.IsDefault)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建失败"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(200, gin.H{"success": true, "id": id, "message": "创建成功"})
}

// Update 更新生成器
func (h *GeneratorsHandler) Update(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var req GeneratorUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	if req.IsDefault != nil && *req.IsDefault == 1 {
		sqlxDB.Exec("UPDATE content_generators SET is_default = 0 WHERE is_default = 1 AND id != ?", id)
	}

	updates := []string{}
	args := []interface{}{}

	if req.DisplayName != nil {
		updates = append(updates, "display_name = ?")
		args = append(args, *req.DisplayName)
	}
	if req.Description != nil {
		updates = append(updates, "description = ?")
		args = append(args, *req.Description)
	}
	if req.Code != nil {
		updates = append(updates, "code = ?", "version = version + 1")
		args = append(args, *req.Code)
	}
	if req.Enabled != nil {
		updates = append(updates, "enabled = ?")
		args = append(args, *req.Enabled)
	}
	if req.IsDefault != nil {
		updates = append(updates, "is_default = ?")
		args = append(args, *req.IsDefault)
	}

	if len(updates) == 0 {
		c.JSON(200, gin.H{"success": true, "message": "无需更新"})
		return
	}

	args = append(args, id)
	_, err := sqlxDB.Exec("UPDATE content_generators SET "+strings.Join(updates, ", ")+" WHERE id = ?", args...)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "更新失败"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "更新成功"})
}

// Delete 删除生成器
func (h *GeneratorsHandler) Delete(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var isDefault int
	err := sqlxDB.Get(&isDefault, "SELECT is_default FROM content_generators WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}
	if isDefault == 1 {
		c.JSON(400, gin.H{"success": false, "message": "不能删除默认生成器"})
		return
	}

	sqlxDB.Exec("DELETE FROM content_generators WHERE id = ?", id)
	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}

// SetDefault 设为默认
func (h *GeneratorsHandler) SetDefault(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var enabled int
	err := sqlxDB.Get(&enabled, "SELECT enabled FROM content_generators WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}
	if enabled == 0 {
		c.JSON(400, gin.H{"success": false, "message": "不能将禁用的生成器设为默认"})
		return
	}

	sqlxDB.Exec("UPDATE content_generators SET is_default = 0 WHERE is_default = 1")
	sqlxDB.Exec("UPDATE content_generators SET is_default = 1 WHERE id = ?", id)

	c.JSON(200, gin.H{"success": true, "message": "已设为默认"})
}

// Toggle 切换启用状态
func (h *GeneratorsHandler) Toggle(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))

	var gen struct {
		Enabled   int `db:"enabled"`
		IsDefault int `db:"is_default"`
	}
	err := sqlxDB.Get(&gen, "SELECT enabled, is_default FROM content_generators WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "生成器不存在"})
		return
	}

	if gen.IsDefault == 1 && gen.Enabled == 1 {
		c.JSON(400, gin.H{"success": false, "message": "不能禁用默认生成器"})
		return
	}

	newEnabled := 1 - gen.Enabled
	sqlxDB.Exec("UPDATE content_generators SET enabled = ? WHERE id = ?", newEnabled, id)

	message := "已启用"
	if newEnabled == 0 {
		message = "已禁用"
	}
	c.JSON(200, gin.H{"success": true, "enabled": newEnabled, "message": message})
}

// GetTemplates 获取代码模板
func (h *GeneratorsHandler) GetTemplates(c *gin.Context) {
	templates := []map[string]interface{}{
		{
			"name":         "basic",
			"display_name": "基础模板",
			"description":  "简单的段落拼接+拼音标注",
			"code": `async def generate(ctx):
    if len(ctx.paragraphs) < 3:
        return None

    selected = ctx.paragraphs[:3]
    content = "\\n\\n".join(selected)
    return annotate_pinyin(content)
`,
		},
		{
			"name":         "with_title",
			"display_name": "带标题模板",
			"description":  "正文开头插入随机标题",
			"code": `async def generate(ctx):
    if len(ctx.paragraphs) < 3:
        return None

    parts = []
    if ctx.titles:
        parts.append(random.choice(ctx.titles))
        parts.append("")

    count = min(len(ctx.paragraphs), random.randint(3, 5))
    parts.extend(random.sample(ctx.paragraphs, count))

    return annotate_pinyin("\\n\\n".join(parts))
`,
		},
		{
			"name":         "shuffle",
			"display_name": "随机打乱模板",
			"description":  "随机打乱段落顺序",
			"code": `async def generate(ctx):
    if len(ctx.paragraphs) < 3:
        return None

    count = min(len(ctx.paragraphs), random.randint(4, 6))
    selected = random.sample(ctx.paragraphs, count)
    random.shuffle(selected)

    return annotate_pinyin("\\n\\n".join(selected))
`,
		},
	}

	c.JSON(200, gin.H{"success": true, "data": templates})
}
