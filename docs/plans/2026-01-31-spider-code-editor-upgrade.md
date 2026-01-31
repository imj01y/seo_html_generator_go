# 爬虫项目代码编辑器升级实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将爬虫项目的代码编辑器升级为 VS Code 风格，复用 CodeEditorPanel 组件，支持嵌套目录结构。

**Architecture:** 分离项目配置（移至列表页弹窗）和代码编辑（新页面复用 CodeEditorPanel）。后端 API 改造支持树形文件结构，LogPanel 扩展支持标签页显示测试数据。

**Tech Stack:** Vue 3 + TypeScript + Element Plus + Monaco Editor + Go Gin + MySQL

---

## Task 1: 数据库迁移 - 支持树形文件结构

**Files:**
- Create: `migrations/002_spider_files_tree.sql`
- Modify: `api/internal/handler/spiders.go:43-50` (SpiderProjectFile 结构体)

**Step 1: 创建数据库迁移脚本**

```sql
-- migrations/002_spider_files_tree.sql
-- 爬虫项目文件表支持树形结构迁移

-- 1. 重命名 filename 为 path，修改长度
ALTER TABLE spider_project_files
  CHANGE COLUMN filename path VARCHAR(500) NOT NULL COMMENT '文件路径（如 /spider.py, /lib/utils.py）';

-- 2. 为现有数据添加 / 前缀
UPDATE spider_project_files
  SET path = CONCAT('/', path)
  WHERE path NOT LIKE '/%';

-- 3. 添加 type 字段区分文件和目录
ALTER TABLE spider_project_files
  ADD COLUMN type ENUM('file', 'dir') NOT NULL DEFAULT 'file' COMMENT '类型：file=文件, dir=目录' AFTER path;

-- 4. 更新唯一索引
ALTER TABLE spider_project_files
  DROP INDEX uk_project_file,
  ADD UNIQUE INDEX uk_project_path (project_id, path);
```

**Step 2: 运行迁移脚本**

Run: `docker exec -i seo_mysql mysql -uroot -pmysql_6yh7uJ seo_generator < migrations/002_spider_files_tree.sql`
Expected: Query OK

**Step 3: 更新 Go 结构体**

修改 `api/internal/handler/spiders.go:43-50`:

```go
// SpiderProjectFile 项目文件
type SpiderProjectFile struct {
	ID        int       `db:"id" json:"id"`
	ProjectID int       `db:"project_id" json:"project_id"`
	Path      string    `db:"path" json:"path"`
	Type      string    `db:"type" json:"type"` // "file" or "dir"
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
```

**Step 4: 验证数据库结构**

Run: `docker exec -i seo_mysql mysql -uroot -pmysql_6yh7uJ seo_generator -e "DESCRIBE spider_project_files;"`
Expected: 显示 path, type 字段

**Step 5: Commit**

```bash
git add migrations/002_spider_files_tree.sql api/internal/handler/spiders.go
git commit -m "$(cat <<'EOF'
feat(spider): 数据库迁移支持树形文件结构

- filename 重命名为 path，支持嵌套路径
- 新增 type 字段区分文件和目录
- 更新 SpiderProjectFile 结构体

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 后端 API 改造 - 树形文件操作

**Files:**
- Modify: `api/internal/handler/spiders.go:560-760` (文件操作 API)
- Modify: `api/internal/handler/router.go:220-250` (路由配置)

**Step 1: 添加 TreeNode 结构体**

在 `api/internal/handler/spiders.go` 的结构体定义区域添加:

```go
// SpiderTreeNode 文件树节点
type SpiderTreeNode struct {
	Name     string            `json:"name"`
	Path     string            `json:"path"`
	Type     string            `json:"type"` // "file" or "dir"
	Children []*SpiderTreeNode `json:"children,omitempty"`
}
```

**Step 2: 实现 GetFileTree API**

```go
// GetFileTree 获取项目文件树
func (h *SpidersHandler) GetFileTree(c *gin.Context) {
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
	var exists2 int
	sqlxDB.Get(&exists2, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if exists2 == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	// 获取所有文件和目录
	var files []SpiderProjectFile
	sqlxDB.Select(&files, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY path
	`, id)

	// 构建树结构
	root := &SpiderTreeNode{
		Name:     "project",
		Path:     "/",
		Type:     "dir",
		Children: []*SpiderTreeNode{},
	}

	for _, file := range files {
		h.insertNode(root, file.Path, file.Type)
	}

	c.JSON(200, gin.H{"success": true, "data": root})
}

// insertNode 将文件/目录插入树结构
func (h *SpidersHandler) insertNode(root *SpiderTreeNode, path string, nodeType string) {
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
			newNode := &SpiderTreeNode{
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
```

**Step 3: 更新 ListFiles 返回 path 字段**

修改 `ListFiles` 函数的 SQL 查询:

```go
// ListFiles 获取项目文件列表
func (h *SpidersHandler) ListFiles(c *gin.Context) {
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

	var exists2 int
	sqlxDB.Get(&exists2, "SELECT COUNT(*) FROM spider_projects WHERE id = ?", id)
	if exists2 == 0 {
		c.JSON(404, gin.H{"success": false, "message": "项目不存在"})
		return
	}

	var files []SpiderProjectFile
	sqlxDB.Select(&files, `
		SELECT id, project_id, path, type, content, created_at, updated_at
		FROM spider_project_files WHERE project_id = ? ORDER BY path
	`, id)

	c.JSON(200, gin.H{"success": true, "data": files})
}
```

**Step 4: 更新 GetFile 使用 path**

```go
// GetFile 获取单个文件内容
func (h *SpidersHandler) GetFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(404, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := "/" + c.Param("path") // 路由参数不包含前导 /

	var file SpiderProjectFile
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
```

**Step 5: 更新 CreateFile 支持目录**

```go
// SpiderCreateItemRequest 创建文件或目录请求
type SpiderCreateItemRequest struct {
	Name string `json:"name" binding:"required"`
	Type string `json:"type" binding:"required,oneof=file dir"`
}

// CreateItem 创建文件或目录
func (h *SpidersHandler) CreateItem(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	parentPath := c.Param("path")
	if parentPath == "" {
		parentPath = "/"
	} else {
		parentPath = "/" + parentPath
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

	var req SpiderCreateItemRequest
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
```

**Step 6: 更新 UpdateFile 使用 path**

```go
// UpdateFile 更新文件内容
func (h *SpidersHandler) UpdateFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := "/" + c.Param("path")

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

	var req SpiderFileUpdate
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
```

**Step 7: 更新 DeleteFile 支持目录递归删除**

```go
// DeleteFile 删除文件或目录
func (h *SpidersHandler) DeleteFile(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	path := "/" + c.Param("path")

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
```

**Step 8: 添加 MoveItem API**

```go
// SpiderMoveRequest 移动/重命名请求
type SpiderMoveRequest struct {
	NewPath string `json:"new_path" binding:"required"`
}

// MoveItem 移动或重命名文件/目录
func (h *SpidersHandler) MoveItem(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	id, _ := strconv.Atoi(c.Param("id"))
	oldPath := "/" + c.Param("path")

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

	var req SpiderMoveRequest
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
```

**Step 9: 更新路由配置**

修改 `api/internal/handler/router.go` 中 spiderRoutes 部分:

```go
spiderRoutes := r.Group("/api/spider-projects")
{
	spiderRoutes.GET("", spiders.List)
	spiderRoutes.POST("", spiders.Create)
	spiderRoutes.GET("/:id", spiders.Get)
	spiderRoutes.PUT("/:id", spiders.Update)
	spiderRoutes.DELETE("/:id", spiders.Delete)
	spiderRoutes.POST("/:id/toggle", spiders.Toggle)
	spiderRoutes.POST("/:id/run", spiders.Run)
	spiderRoutes.POST("/:id/stop", spiders.Stop)
	spiderRoutes.POST("/:id/reset", spiders.Reset)
	spiderRoutes.POST("/:id/test", spiders.Test)
	spiderRoutes.POST("/:id/test/stop", spiders.StopTest)

	// 文件操作 - 支持树形结构
	spiderRoutes.GET("/:id/files", spiders.ListFiles)           // ?tree=true 返回树形结构
	spiderRoutes.GET("/:id/files/*path", spiders.GetFile)       // 获取文件内容
	spiderRoutes.POST("/:id/files", spiders.CreateItem)         // 根目录创建
	spiderRoutes.POST("/:id/files/*path", spiders.CreateItem)   // 指定目录创建
	spiderRoutes.PUT("/:id/files/*path", spiders.UpdateFile)    // 更新文件
	spiderRoutes.DELETE("/:id/files/*path", spiders.DeleteFile) // 删除文件/目录
	spiderRoutes.PATCH("/:id/files/*path", spiders.MoveItem)    // 移动/重命名
}
```

**Step 10: 编译验证**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 11: Commit**

```bash
git add api/internal/handler/spiders.go api/internal/handler/router.go
git commit -m "$(cat <<'EOF'
feat(spider): 后端 API 支持树形文件结构

- 新增 GetFileTree API 返回树形结构
- 更新文件操作 API 使用 path 字段
- 支持创建目录和嵌套文件
- 支持目录递归删除
- 新增 MoveItem API 支持移动/重命名

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 前端 API 适配器

**Files:**
- Modify: `web/src/api/spiderProjects.ts`

**Step 1: 添加树形结构类型定义**

在 `web/src/api/spiderProjects.ts` 顶部添加:

```typescript
// 树形文件结构
export interface SpiderTreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: SpiderTreeNode[]
}

// 兼容 CodeEditorPanel 的 TreeNode 类型
export type { SpiderTreeNode as TreeNode }
```

**Step 2: 添加树形文件 API**

```typescript
// ============================================
// 树形文件操作 API（CodeEditorPanel 适配）
// ============================================

/**
 * 获取项目文件树
 */
export const getProjectFileTree = async (projectId: number): Promise<SpiderTreeNode> => {
  const res = await request.get(`/spider-projects/${projectId}/files`, {
    params: { tree: 'true' }
  })
  return res.data
}

/**
 * 获取文件内容（路径版本）
 */
export const getProjectFileByPath = async (
  projectId: number,
  path: string
): Promise<{ content: string }> => {
  // 移除前导 /
  const cleanPath = path.startsWith('/') ? path.slice(1) : path
  const res = await request.get(`/spider-projects/${projectId}/files/${cleanPath}`)
  return res.data
}

/**
 * 保存文件（路径版本）
 */
export const saveProjectFileByPath = async (
  projectId: number,
  path: string,
  content: string
): Promise<void> => {
  const cleanPath = path.startsWith('/') ? path.slice(1) : path
  await request.put(`/spider-projects/${projectId}/files/${cleanPath}`, { content })
}

/**
 * 创建文件或目录
 */
export const createProjectItem = async (
  projectId: number,
  parentPath: string,
  name: string,
  type: 'file' | 'dir'
): Promise<void> => {
  const cleanPath = parentPath.startsWith('/') ? parentPath.slice(1) : parentPath
  const url = cleanPath
    ? `/spider-projects/${projectId}/files/${cleanPath}`
    : `/spider-projects/${projectId}/files`
  await request.post(url, { name, type })
}

/**
 * 删除文件或目录
 */
export const deleteProjectItem = async (
  projectId: number,
  path: string
): Promise<void> => {
  const cleanPath = path.startsWith('/') ? path.slice(1) : path
  await request.delete(`/spider-projects/${projectId}/files/${cleanPath}`)
}

/**
 * 移动/重命名文件或目录
 */
export const moveProjectItem = async (
  projectId: number,
  oldPath: string,
  newPath: string
): Promise<void> => {
  const cleanPath = oldPath.startsWith('/') ? oldPath.slice(1) : oldPath
  await request.patch(`/spider-projects/${projectId}/files/${cleanPath}`, {
    new_path: newPath
  })
}

/**
 * 创建 CodeEditorPanel API 适配器
 */
export function createSpiderEditorApi(projectId: number) {
  return {
    getFileTree: () => getProjectFileTree(projectId),
    getFile: (path: string) => getProjectFileByPath(projectId, path),
    saveFile: (path: string, content: string) => saveProjectFileByPath(projectId, path, content),
    createItem: (parentPath: string, name: string, type: 'file' | 'dir') =>
      createProjectItem(projectId, parentPath, name, type),
    deleteItem: (path: string) => deleteProjectItem(projectId, path),
    moveItem: (oldPath: string, newPath: string) => moveProjectItem(projectId, oldPath, newPath),
    // 不提供 runFile，因为爬虫使用项目级测试运行
  }
}
```

**Step 3: 验证 TypeScript 编译**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

**Step 4: Commit**

```bash
git add web/src/api/spiderProjects.ts
git commit -m "$(cat <<'EOF'
feat(spider): 前端 API 适配器支持树形文件结构

- 新增 SpiderTreeNode 类型
- 新增树形文件操作 API
- 创建 CodeEditorPanel API 适配器工厂函数

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 扩展 LogPanel 支持标签页

**Files:**
- Modify: `web/src/components/CodeEditorPanel/types.ts`
- Modify: `web/src/components/CodeEditorPanel/components/LogPanel.vue`

**Step 1: 添加 ExtraTab 类型定义**

在 `web/src/components/CodeEditorPanel/types.ts` 添加:

```typescript
// ============================================
// 日志面板扩展标签页类型
// ============================================

export interface ExtraTab {
  key: string
  label: string
  badge?: number
  component: Component
  props?: Record<string, any>
}
```

**Step 2: 修改 LogPanel 支持 extraTabs**

修改 `web/src/components/CodeEditorPanel/components/LogPanel.vue`:

```vue
<template>
  <div
    class="log-panel"
    :class="{ expanded: store.logExpanded.value }"
    :style="{ height: store.logExpanded.value ? height + 'px' : '28px' }"
  >
    <!-- 拖拽调整高度 -->
    <div
      v-if="store.logExpanded.value"
      class="resize-handle"
      @mousedown="startResize"
    ></div>

    <!-- 标题栏 -->
    <div class="panel-header" @click="toggleExpand">
      <div class="header-left">
        <el-icon class="expand-icon">
          <CaretRight v-if="!store.logExpanded.value" />
          <CaretBottom v-else />
        </el-icon>
        <span class="title">{{ currentTabLabel }}</span>
        <span v-if="store.logRunning.value" class="running-badge">运行中...</span>
        <span v-else-if="store.logs.value.length === 0 && activeTab === 'logs'" class="empty-badge">无输出</span>
      </div>
      <div class="header-right" @click.stop>
        <el-button
          v-if="store.logRunning.value"
          text
          size="small"
          type="danger"
          @click="$emit('stop')"
        >
          停止
        </el-button>
        <el-button text size="small" @click="handleCopy">复制</el-button>
        <el-button text size="small" @click="store.clearLogs">清空</el-button>
      </div>
    </div>

    <!-- 内容区 -->
    <div v-if="store.logExpanded.value" class="log-body">
      <!-- 标签页切换（如果有额外标签页） -->
      <div v-if="extraTabs && extraTabs.length > 0" class="tab-bar">
        <div
          :class="['tab-item', { active: activeTab === 'logs' }]"
          @click="activeTab = 'logs'"
        >
          日志
        </div>
        <div
          v-for="tab in extraTabs"
          :key="tab.key"
          :class="['tab-item', { active: activeTab === tab.key }]"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
          <span v-if="tab.badge" class="tab-badge">{{ tab.badge }}</span>
        </div>
      </div>

      <!-- 日志内容 -->
      <div v-show="activeTab === 'logs'" class="log-content" ref="logContent">
        <div
          v-for="(log, index) in store.logs.value"
          :key="index"
          :class="['log-line', log.type]"
        >
          <span class="log-text">{{ log.data }}</span>
          <span class="log-time">{{ formatTime(log.timestamp) }}</span>
        </div>
        <div v-if="store.logs.value.length === 0" class="empty-log">
          运行文件查看输出
        </div>
      </div>

      <!-- 额外标签页内容 -->
      <div
        v-for="tab in extraTabs"
        :key="tab.key"
        v-show="activeTab === tab.key"
        class="extra-tab-content"
      >
        <component :is="tab.component" v-bind="tab.props" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick, computed } from 'vue'
import { CaretRight, CaretBottom } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { EditorStore } from '../composables/useEditorStore'
import type { ExtraTab } from '../types'

const props = defineProps<{
  store: EditorStore
  extraTabs?: ExtraTab[]
}>()

defineEmits<{
  (e: 'stop'): void
}>()

const logContent = ref<HTMLElement>()
const height = ref(200)
const activeTab = ref('logs')

const currentTabLabel = computed(() => {
  if (activeTab.value === 'logs') return '运行日志'
  const tab = props.extraTabs?.find(t => t.key === activeTab.value)
  return tab?.label || '运行日志'
})

function toggleExpand() {
  props.store.logExpanded.value = !props.store.logExpanded.value
}

function startResize(event: MouseEvent) {
  const startY = event.clientY
  const startHeight = height.value

  function onMouseMove(e: MouseEvent) {
    const newHeight = Math.max(100, Math.min(500, startHeight - (e.clientY - startY)))
    height.value = newHeight
  }

  function onMouseUp() {
    document.removeEventListener('mousemove', onMouseMove)
    document.removeEventListener('mouseup', onMouseUp)
  }

  document.addEventListener('mousemove', onMouseMove)
  document.addEventListener('mouseup', onMouseUp)
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  })
}

function handleCopy() {
  const text = props.store.logs.value.map(l => l.data).join('\n')
  navigator.clipboard.writeText(text)
  ElMessage.success('已复制到剪贴板')
}

// 自动滚动到底部
watch(() => props.store.logs.value.length, () => {
  nextTick(() => {
    if (logContent.value) {
      logContent.value.scrollTop = logContent.value.scrollHeight
    }
  })
})
</script>

<style scoped>
.log-panel {
  background: #1e1e1e;
  border-top: 1px solid #3c3c3c;
  display: flex;
  flex-direction: column;
  transition: height 0.15s;
  flex-shrink: 0;
}

.resize-handle {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  cursor: ns-resize;
  z-index: 10;
}

.resize-handle:hover {
  background: #007acc;
}

.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 12px;
  background: #252526;
  cursor: pointer;
  user-select: none;
  height: 28px;
  box-sizing: border-box;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.expand-icon {
  font-size: 12px;
  color: #cccccc;
}

.title {
  font-size: 12px;
  font-weight: 500;
  color: #cccccc;
}

.running-badge {
  font-size: 11px;
  color: #3794ff;
}

.empty-badge {
  font-size: 11px;
  color: #6e6e6e;
}

.header-right {
  display: flex;
  gap: 4px;
}

.log-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.tab-bar {
  display: flex;
  gap: 0;
  padding: 0 12px;
  background: #252526;
  border-bottom: 1px solid #3c3c3c;
}

.tab-item {
  padding: 6px 12px;
  font-size: 12px;
  color: #808080;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  margin-bottom: -1px;
}

.tab-item:hover {
  color: #cccccc;
}

.tab-item.active {
  color: #cccccc;
  border-bottom-color: #007acc;
}

.tab-badge {
  margin-left: 4px;
  padding: 0 6px;
  font-size: 10px;
  background: #007acc;
  color: #fff;
  border-radius: 10px;
}

.log-content,
.extra-tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 8px 12px;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 12px;
  line-height: 1.6;
}

.log-line {
  display: flex;
  justify-content: space-between;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-text {
  flex: 1;
}

.log-time {
  flex-shrink: 0;
  margin-left: 16px;
  color: #4e4e4e;
}

.log-line.command {
  color: #808080;
}

.log-line.stdout {
  color: #d4d4d4;
}

.log-line.stderr {
  color: #f48771;
}

.log-line.info {
  color: #808080;
}

.empty-log {
  color: #6e6e6e;
  text-align: center;
  padding: 20px;
}
</style>
```

**Step 3: 验证 TypeScript 编译**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

**Step 4: Commit**

```bash
git add web/src/components/CodeEditorPanel/types.ts web/src/components/CodeEditorPanel/components/LogPanel.vue
git commit -m "$(cat <<'EOF'
feat(editor): LogPanel 支持可选标签页

- 新增 ExtraTab 类型定义
- LogPanel 支持 extraTabs prop
- 标签页切换和角标显示

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 创建数据预览组件

**Files:**
- Create: `web/src/views/spiders/components/DataPreview.vue`

**Step 1: 创建 DataPreview 组件**

```vue
<!-- web/src/views/spiders/components/DataPreview.vue -->
<template>
  <div class="data-preview">
    <div v-if="items.length === 0" class="empty-state">
      暂无数据
    </div>
    <div v-else class="items-list">
      <div
        v-for="(item, index) in items"
        :key="index"
        class="item-card"
        @click="showDetail(item)"
      >
        <div class="item-title">{{ item.title || '(无标题)' }}</div>
        <div class="item-content">{{ truncateText(item.content, 150) }}</div>
      </div>
    </div>

    <!-- 详情弹窗 -->
    <el-dialog v-model="detailVisible" title="数据详情" width="700px" top="5vh">
      <div class="item-detail" v-if="currentItem">
        <div class="detail-row" v-for="(value, key) in currentItem" :key="key">
          <div class="detail-label">{{ key }}</div>
          <div class="detail-value" v-if="key === 'content'" v-html="value"></div>
          <div class="detail-value" v-else-if="key === 'source_url'">
            <a :href="value" target="_blank" rel="noopener noreferrer" class="source-link">{{ value }}</a>
          </div>
          <div class="detail-value" v-else>{{ value }}</div>
        </div>
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  items: Record<string, any>[]
}>()

const detailVisible = ref(false)
const currentItem = ref<Record<string, any> | null>(null)

function showDetail(item: Record<string, any>) {
  currentItem.value = item
  detailVisible.value = true
}

function truncateText(text: string, maxLength: number) {
  if (!text) return ''
  const plainText = text.replace(/<[^>]+>/g, '')
  if (plainText.length <= maxLength) return plainText
  return plainText.substring(0, maxLength) + '...'
}
</script>

<style scoped>
.data-preview {
  height: 100%;
  overflow-y: auto;
}

.empty-state {
  color: #6e6e6e;
  text-align: center;
  padding: 20px;
}

.items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.item-card {
  padding: 12px;
  background: #2d2d2d;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.2s;
}

.item-card:hover {
  background: #3c3c3c;
}

.item-title {
  font-weight: 500;
  color: #cccccc;
  margin-bottom: 6px;
}

.item-content {
  font-size: 12px;
  color: #808080;
  line-height: 1.5;
}

.item-detail {
  max-height: 70vh;
  overflow-y: auto;
}

.detail-row {
  margin-bottom: 16px;
  border-bottom: 1px solid #ebeef5;
  padding-bottom: 12px;
}

.detail-row:last-child {
  border-bottom: none;
}

.detail-label {
  font-weight: 600;
  color: #303133;
  margin-bottom: 8px;
}

.detail-value {
  color: #606266;
  line-height: 1.6;
  word-break: break-word;
  white-space: pre-wrap;
}

.source-link {
  color: #409EFF;
  text-decoration: none;
  word-break: break-all;
}

.source-link:hover {
  text-decoration: underline;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/spiders/components/DataPreview.vue
git commit -m "$(cat <<'EOF'
feat(spider): 创建 DataPreview 数据预览组件

- 列表展示爬取数据
- 点击查看详情弹窗
- 支持 HTML 内容和链接

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 创建代码编辑页面

**Files:**
- Create: `web/src/views/spiders/ProjectCode.vue`
- Modify: `web/src/router/index.ts`

**Step 1: 创建 ProjectCode.vue**

```vue
<!-- web/src/views/spiders/ProjectCode.vue -->
<template>
  <div class="project-code-page">
    <CodeEditorPanel
      ref="editorRef"
      :api="editorApi"
      :title="pageTitle"
      :runnable="false"
      :show-log-panel="true"
    >
      <template #header-actions>
        <el-button @click="showGuide">
          <el-icon><QuestionFilled /></el-icon>
          指南
        </el-button>
        <el-input-number
          v-model="testMaxItems"
          :min="0"
          :max="10000"
          placeholder="0=不限制"
          style="width: 120px"
        />
        <el-tooltip content="0 表示不限制测试条数" placement="top">
          <el-button type="success" :loading="testing" @click="handleTest">
            {{ testing ? '测试中...' : '测试运行' }}
          </el-button>
        </el-tooltip>
        <el-button v-if="testing" type="danger" @click="handleStopTest">
          停止
        </el-button>
        <el-button @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
      </template>
    </CodeEditorPanel>

    <!-- 爬虫指南 -->
    <SpiderGuide ref="guideRef" />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, markRaw } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, QuestionFilled } from '@element-plus/icons-vue'
import CodeEditorPanel from '@/components/CodeEditorPanel/index.vue'
import SpiderGuide from '@/components/SpiderGuide.vue'
import DataPreview from './components/DataPreview.vue'
import {
  getProject,
  createSpiderEditorApi,
  testProject,
  stopTestProject,
  subscribeTestLogs,
  type SpiderProject
} from '@/api/spiderProjects'
import type { ExtraTab } from '@/components/CodeEditorPanel/types'

const route = useRoute()
const router = useRouter()
const editorRef = ref<InstanceType<typeof CodeEditorPanel>>()
const guideRef = ref<InstanceType<typeof SpiderGuide>>()

// 项目信息
const projectId = computed(() => Number(route.params.id))
const project = ref<SpiderProject | null>(null)
const pageTitle = computed(() => project.value ? `${project.value.name} - 代码编辑` : '代码编辑')

// API 适配器
const editorApi = computed(() => createSpiderEditorApi(projectId.value))

// 测试状态
const testing = ref(false)
const testMaxItems = ref(0)
const testItems = ref<Record<string, any>[]>([])
let unsubscribeTest: (() => void) | null = null

// 日志面板额外标签页
const extraTabs = computed<ExtraTab[]>(() => [
  {
    key: 'data',
    label: '数据',
    badge: testItems.value.length || undefined,
    component: markRaw(DataPreview),
    props: { items: testItems.value }
  }
])

// 加载项目信息
async function loadProject() {
  try {
    project.value = await getProject(projectId.value)
  } catch (e: any) {
    ElMessage.error(e.message || '加载项目失败')
    router.push('/spiders/projects')
  }
}

// 测试运行
async function handleTest() {
  // 先保存所有修改
  const store = editorRef.value?.store
  if (store?.hasModifiedFiles.value) {
    for (const tab of store.modifiedTabs.value) {
      await store.saveTab(tab.id)
    }
  }

  // 清理状态
  unsubscribeTest?.()
  testing.value = true
  testItems.value = []

  // 展开日志面板
  if (store) {
    store.logExpanded.value = true
    store.clearLogs()
    store.addLog({ type: 'command', data: '> 开始测试运行...' })
  }

  try {
    const res = await testProject(projectId.value, testMaxItems.value)
    if (!res.success) {
      store?.addLog({ type: 'stderr', data: res.message || '启动测试失败' })
      testing.value = false
      return
    }

    // 订阅日志
    unsubscribeTest = subscribeTestLogs(
      projectId.value,
      (level, message) => {
        store?.addLog({
          type: level === 'ERROR' ? 'stderr' : 'stdout',
          data: `[${level}] ${message}`
        })
      },
      (item) => {
        testItems.value.push(item)
      },
      () => {
        testing.value = false
        store?.addLog({ type: 'info', data: '> 测试运行完成' })
      },
      (error) => {
        store?.addLog({ type: 'stderr', data: error })
        testing.value = false
      }
    )
  } catch (e: any) {
    store?.addLog({ type: 'stderr', data: e.message || '测试失败' })
    testing.value = false
  }
}

// 停止测试
async function handleStopTest() {
  try {
    await stopTestProject(projectId.value)
    editorRef.value?.store?.addLog({ type: 'info', data: '> 正在停止测试...' })
  } catch (e: any) {
    ElMessage.error(e.message || '停止失败')
  }
}

// 显示指南
function showGuide() {
  guideRef.value?.show()
}

// 返回列表
function goBack() {
  router.push('/spiders/projects')
}

onMounted(() => {
  loadProject()
})

onUnmounted(() => {
  unsubscribeTest?.()
})
</script>

<style scoped>
.project-code-page {
  padding: 20px;
  height: calc(100vh - 60px);
  box-sizing: border-box;
}
</style>
```

**Step 2: 更新路由配置**

修改 `web/src/router/index.ts`:

```typescript
// 在 spiders/projects/:id 路由后添加
{
  path: 'spiders/projects/:id/code',
  name: 'SpiderProjectCode',
  component: () => import('@/views/spiders/ProjectCode.vue'),
  meta: { title: '代码编辑' }
},
```

**Step 3: 验证 TypeScript 编译**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

**Step 4: Commit**

```bash
git add web/src/views/spiders/ProjectCode.vue web/src/router/index.ts
git commit -m "$(cat <<'EOF'
feat(spider): 创建代码编辑页面

- 复用 CodeEditorPanel 组件
- 集成测试运行功能
- 日志面板支持数据预览标签页
- 新增路由 /spiders/projects/:id/code

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 项目列表页增加配置弹窗

**Files:**
- Modify: `web/src/views/spiders/ProjectList.vue`

**Step 1: 添加配置弹窗模板**

在 `ProjectList.vue` 的 `</el-drawer>` 后添加:

```vue
<!-- 配置弹窗 -->
<el-dialog
  v-model="configDialogVisible"
  :title="configProject ? `配置 - ${configProject.name}` : '项目配置'"
  width="600px"
>
  <el-form
    v-if="configProject"
    ref="configFormRef"
    :model="configForm"
    label-position="top"
  >
    <el-form-item label="项目名称" prop="name" required>
      <el-input v-model="configForm.name" placeholder="请输入项目名称" />
    </el-form-item>
    <el-form-item label="描述" prop="description">
      <el-input
        v-model="configForm.description"
        type="textarea"
        :rows="2"
        placeholder="项目描述（可选）"
      />
    </el-form-item>
    <el-form-item label="入口文件" prop="entry_file">
      <el-select v-model="configForm.entry_file" style="width: 100%">
        <el-option
          v-for="file in configFiles"
          :key="file.path"
          :label="file.path"
          :value="file.path.replace(/^\//, '')"
        />
      </el-select>
    </el-form-item>
    <el-row :gutter="16">
      <el-col :span="12">
        <el-form-item label="并发数" prop="concurrency">
          <el-input-number
            v-model="configForm.concurrency"
            :min="1"
            :max="10"
            style="width: 100%"
          />
        </el-form-item>
      </el-col>
      <el-col :span="12">
        <el-form-item label="输出分组" prop="output_group_id">
          <el-select v-model="configForm.output_group_id" style="width: 100%">
            <el-option
              v-for="group in articleGroups"
              :key="group.id"
              :label="group.name"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
      </el-col>
    </el-row>
    <el-form-item label="调度规则" prop="schedule">
      <ScheduleBuilder v-model="configForm.schedule" />
    </el-form-item>
  </el-form>
  <template #footer>
    <el-button @click="configDialogVisible = false">取消</el-button>
    <el-button type="primary" :loading="configSaving" @click="handleSaveConfig">
      保存
    </el-button>
  </template>
</el-dialog>
```

**Step 2: 添加操作按钮**

在操作列的"编辑"按钮前添加:

```vue
<el-button size="small" @click="handleConfig(row)">配置</el-button>
```

修改"编辑"按钮为"代码":

```vue
<el-button size="small" type="primary" @click="handleEditCode(row)">代码</el-button>
```

**Step 3: 添加配置相关逻辑**

在 `<script setup>` 中添加:

```typescript
import ScheduleBuilder from '@/components/ScheduleBuilder.vue'
import { updateProject, getProjectFiles, type ProjectFile } from '@/api/spiderProjects'

// 配置弹窗
const configDialogVisible = ref(false)
const configProject = ref<SpiderProject | null>(null)
const configFiles = ref<ProjectFile[]>([])
const configSaving = ref(false)
const configFormRef = ref()
const configForm = ref({
  name: '',
  description: '',
  entry_file: '',
  concurrency: 3,
  output_group_id: 1,
  schedule: ''
})

// 文章分组（简化版本）
const articleGroups = ref([{ id: 1, name: '默认文章分组' }])

// 打开配置弹窗
async function handleConfig(row: SpiderProject) {
  configProject.value = row
  configForm.value = {
    name: row.name,
    description: row.description || '',
    entry_file: row.entry_file,
    concurrency: row.concurrency,
    output_group_id: row.output_group_id,
    schedule: row.schedule || ''
  }

  // 加载文件列表
  try {
    configFiles.value = await getProjectFiles(row.id)
  } catch {
    configFiles.value = []
  }

  configDialogVisible.value = true
}

// 保存配置
async function handleSaveConfig() {
  if (!configProject.value) return

  if (!configForm.value.name.trim()) {
    ElMessage.warning('请输入项目名称')
    return
  }

  configSaving.value = true
  try {
    await updateProject(configProject.value.id, {
      name: configForm.value.name,
      description: configForm.value.description || undefined,
      entry_file: configForm.value.entry_file,
      concurrency: configForm.value.concurrency,
      output_group_id: configForm.value.output_group_id,
      schedule: configForm.value.schedule || undefined
    })

    ElMessage.success('保存成功')
    configDialogVisible.value = false
    fetchProjects()
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    configSaving.value = false
  }
}

// 编辑代码
function handleEditCode(row: SpiderProject) {
  router.push(`/spiders/projects/${row.id}/code`)
}
```

**Step 4: 修改原有编辑跳转**

将 `handleEdit` 函数修改为跳转到代码编辑页:

```typescript
// 编辑项目（跳转到代码页）
function handleEdit(row: SpiderProject) {
  router.push(`/spiders/projects/${row.id}/code`)
}
```

**Step 5: 验证 TypeScript 编译**

Run: `cd web && npx tsc --noEmit`
Expected: 无错误

**Step 6: Commit**

```bash
git add web/src/views/spiders/ProjectList.vue
git commit -m "$(cat <<'EOF'
feat(spider): 项目列表页增加配置弹窗

- 新增"配置"按钮打开配置弹窗
- 配置弹窗包含所有项目配置字段
- "编辑"按钮改为跳转代码编辑页
- 保留所有现有操作功能

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: 更新 CodeEditorPanel 支持 extraTabs

**Files:**
- Modify: `web/src/components/CodeEditorPanel/index.vue`

**Step 1: 添加 extraTabs prop**

在 props 定义中添加:

```typescript
import type { TreeNode, CodeEditorApi, CodeEditorPanelProps, ExtraTab } from './types'

const props = withDefaults(defineProps<CodeEditorPanelProps & {
  extraTabs?: ExtraTab[]
}>(), {
  title: '代码编辑器',
  runnable: false,
  showLogPanel: false,
  showRestartButton: false,
  showRebuildButton: false,
  runnableExtensions: () => ['.py']
})
```

**Step 2: 传递 extraTabs 给 LogPanel**

修改 LogPanel 组件使用:

```vue
<LogPanel
  v-if="showLogPanel"
  :store="store"
  :extra-tabs="extraTabs"
  @stop="handleStop"
/>
```

**Step 3: Commit**

```bash
git add web/src/components/CodeEditorPanel/index.vue
git commit -m "$(cat <<'EOF'
feat(editor): CodeEditorPanel 支持传递 extraTabs

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: 集成测试

**Step 1: 启动开发服务器**

Run: `cd web && npm run dev`
Expected: 开发服务器启动成功

**Step 2: 测试配置弹窗**

1. 访问项目列表页
2. 点击"配置"按钮
3. 验证弹窗显示所有配置字段
4. 修改配置并保存

**Step 3: 测试代码编辑页**

1. 点击"代码"按钮
2. 验证 VS Code 风格布局
3. 测试文件树导航
4. 测试创建文件/目录
5. 测试测试运行功能

**Step 4: 验证数据预览**

1. 运行测试
2. 切换到"数据"标签页
3. 验证数据显示

**Step 5: Commit 最终状态**

```bash
git add -A
git commit -m "$(cat <<'EOF'
chore(spider): 集成测试通过

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## 完成检查清单

- [ ] 数据库迁移成功
- [ ] 后端 API 支持树形结构
- [ ] 前端 API 适配器工作正常
- [ ] LogPanel 支持标签页
- [ ] DataPreview 组件正常显示
- [ ] 代码编辑页面布局正确
- [ ] 配置弹窗功能完整
- [ ] 测试运行功能正常
- [ ] 所有现有功能保持正常
