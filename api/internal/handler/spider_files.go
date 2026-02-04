package api

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"seo-generator/api/internal/model"
)

// SpiderFilesHandler 爬虫文件处理器
type SpiderFilesHandler struct{}

// ListFiles 获取项目文件列表
func (h *SpiderFilesHandler) ListFiles(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"success": false, "message": "无效的ID"})
		return
	}

	// 检查是否请求树形结构
	if c.Query("tree") == "true" {
		h.GetFileTree(c)
		return
	}

	var projectCount int
	sqlxDB.Get(&projectCount, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if projectCount == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	var files []models.SpiderProjectFile
	sqlxDB.Select(&files, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY path
	`, id)

	c.JSON(200, gin.H{"success": true, "data": files})
}

// GetFileTree 获取项目文件树
func (h *SpiderFilesHandler) GetFileTree(c *gin.Context) {
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

	// 检查项目是否存在
	var projectCount int
	sqlxDB.Get(&projectCount, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if projectCount == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	// 获取所有文件和目录
	var files []models.SpiderProjectFile
	sqlxDB.Select(&files, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY path
	`, id)

	// 构建树结构
	root := &models.SpiderTreeNode{
		Name:     "project",
		Path:     "/",
		Type:     "dir",
		Children: []*models.SpiderTreeNode{},
	}

	for _, file := range files {
		h.insertNode(root, file.Path, file.Type)
	}

	c.JSON(200, gin.H{"success": true, "data": root})
}

// insertNode 将文件/目录插入树结构
func (h *SpiderFilesHandler) insertNode(root *models.SpiderTreeNode, path string, nodeType string) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	current := root

	for i, part := range parts {
		if part == "" {
			continue
		}

		isLast := i == len(parts)-1
		found := false

		for _, child := range current.Children {
			if child.Name == part {
				current = child
				found = true
				break
			}
		}

		if !found {
			newNode := &models.SpiderTreeNode{
				Name: part,
				Path: "/" + strings.Join(parts[:i+1], "/"),
				Type: "dir",
			}
			if isLast {
				newNode.Type = nodeType
			}
			current.Children = append(current.Children, newNode)
			current = newNode
		}
	}
}

// GetFile 获取单个文件内容
func (h *SpiderFilesHandler) GetFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(404, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := c.Param("path") // *path 通配符已包含前导 /

	var file models.SpiderProjectFile
	err := sqlxDB.Get(&file, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? AND path = ?
	`, id, path)

	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "data": gin.H{
		"path":    file.Path,
		"content": file.Content,
	}})
}

// CreateItem 创建文件或目录
func (h *SpiderFilesHandler) CreateItem(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	parentPath := c.Param("path") // *path 通配符已包含前导 /，根目录时为空
	if parentPath == "" {
		parentPath = "/"
	}

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法添加文件"})
		return
	}

	var req models.SpiderCreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误: " + err.Error()})
		return
	}

	// 验证文件名
	if strings.ContainsAny(req.Name, `/\:*?"<>|`) || strings.HasPrefix(req.Name, ".") {
		c.JSON(400, gin.H{"success": false, "message": "文件名包含非法字符"})
		return
	}

	// 构建完整路径
	var fullPath string
	if parentPath == "/" {
		fullPath = "/" + req.Name
	} else {
		fullPath = parentPath + "/" + req.Name
	}

	// 检查是否已存在
	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM spider_project_files WHERE project_id = ? AND path = ?", id, fullPath)
	if existsCount > 0 {
		c.JSON(400, gin.H{"success": false, "message": "文件或目录已存在"})
		return
	}

	// 创建
	content := ""
	if req.Type == "file" {
		content = "# " + req.Name + "\n"
	}

	result, err := sqlxDB.Exec(`
		INSERT INTO spider_project_files (project_id, path, type, content) VALUES (?, ?, ?, ?)
	`, id, fullPath, req.Type, content)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "创建失败: " + err.Error()})
		return
	}

	fileID, _ := result.LastInsertId()
	c.JSON(200, gin.H{"success": true, "id": fileID, "path": fullPath, "message": "创建成功"})
}

// UpdateFile 更新文件内容
func (h *SpiderFilesHandler) UpdateFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := c.Param("path") // *path 通配符已包含前导 /

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法修改文件"})
		return
	}

	var req models.SpiderFileUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 使用 upsert
	_, err = sqlxDB.Exec(`
		INSERT INTO spider_project_files (project_id, path, type, content)
		VALUES (?, ?, 'file', ?)
		ON DUPLICATE KEY UPDATE content = VALUES(content)
	`, id, path, req.Content)

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "保存文件失败: " + err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "保存成功"})
}

// DeleteFile 删除文件或目录
func (h *SpiderFilesHandler) DeleteFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := c.Param("path") // *path 通配符已包含前导 /

	var project struct {
		Status    string `db:"status"`
		EntryFile string `db:"entry_file"`
	}
	err := sqlxDB.Get(&project, "SELECT status, entry_file FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if project.Status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中，无法删除文件"})
		return
	}

	// 检查是否是入口文件
	entryPath := "/" + project.EntryFile
	if path == entryPath {
		c.JSON(400, gin.H{"success": false, "message": "不能删除入口文件"})
		return
	}

	// 删除文件或目录（目录会递归删除子项）
	result, err := sqlxDB.Exec(`
		DELETE FROM spider_project_files
		WHERE project_id = ? AND (path = ? OR path LIKE ?)
	`, id, path, path+"/%")

	if err != nil {
		c.JSON(500, gin.H{"success": false, "message": "删除失败"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		c.JSON(404, gin.H{"success": false, "message": "文件不存在"})
		return
	}

	c.JSON(200, gin.H{"success": true, "message": "删除成功"})
}

// MoveItem 移动或重命名文件/目录
func (h *SpiderFilesHandler) MoveItem(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	oldPath := c.Param("path") // *path 通配符已包含前导 /

	var status string
	err := sqlxDB.Get(&status, "SELECT status FROM spider_projects WHERE id = ?", id)
	if err != nil {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}
	if status == "running" {
		c.JSON(400, gin.H{"success": false, "message": "项目正在运行中"})
		return
	}

	var req models.SpiderMoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	newPath := req.NewPath
	if !strings.HasPrefix(newPath, "/") {
		newPath = "/" + newPath
	}

	// 检查目标是否已存在
	var existsCount int
	sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM spider_project_files WHERE project_id = ? AND path = ?", id, newPath)
	if existsCount > 0 {
		c.JSON(400, gin.H{"success": false, "message": "目标路径已存在"})
		return
	}

	// 更新路径（包括子目录）
	tx, _ := sqlxDB.Beginx()

	// 更新当前项
	_, err = tx.Exec(`
		UPDATE spider_project_files SET path = ? WHERE project_id = ? AND path = ?
	`, newPath, id, oldPath)
	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"success": false, "message": "移动失败"})
		return
	}

	// 更新子项（如果是目录）
	_, err = tx.Exec(`
		UPDATE spider_project_files
		SET path = CONCAT(?, SUBSTRING(path, ?))
		WHERE project_id = ? AND path LIKE ?
	`, newPath, len(oldPath)+1, id, oldPath+"/%")
	if err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"success": false, "message": "移动子项失败"})
		return
	}

	tx.Commit()
	c.JSON(200, gin.H{"success": true, "message": "移动成功", "new_path": newPath})
}
