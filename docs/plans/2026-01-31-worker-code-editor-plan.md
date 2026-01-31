# Worker 在线代码编辑器实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在管理后台实现 Worker 代码在线编辑、运行测试、文件管理功能

**Architecture:** 前端使用 Element Plus + Monaco Editor，通过 Go API 操作 Docker 挂载卷中的 Worker 代码，WebSocket 实时推送运行日志

**Tech Stack:** Vue 3 + Element Plus + Monaco Editor + Go Gin + WebSocket + Redis Pub/Sub + Docker

---

## 前置条件

- 设计文档：`docs/plans/2026-01-31-worker-code-editor-design.md`
- 现有 API 结构：`api/internal/handler/router.go`
- 现有 WebSocket：`api/internal/handler/websocket.go`
- 现有 API 模式：`web/src/api/spiderProjects.ts`

---

## Task 1: 后端 - Worker 文件操作 Handler 基础结构

**Files:**
- Create: `api/internal/handler/worker_files.go`
- Modify: `api/internal/handler/router.go:332-345`

**Step 1: 创建 worker_files.go 基础结构**

创建文件 `api/internal/handler/worker_files.go`：

```go
package api

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	core "seo-generator/api/internal/service"
)

// WorkerFilesHandler Worker 文件管理处理器
type WorkerFilesHandler struct {
	workerDir string // Worker 代码目录，如 /project/worker
}

// NewWorkerFilesHandler 创建处理器
func NewWorkerFilesHandler(workerDir string) *WorkerFilesHandler {
	return &WorkerFilesHandler{workerDir: workerDir}
}

// FileInfo 文件信息
type FileInfo struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"` // "file" or "dir"
	Size  int64     `json:"size,omitempty"`
	Mtime time.Time `json:"mtime"`
}

// TreeNode 目录树节点
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"`
	Children []*TreeNode `json:"children,omitempty"`
}

// validatePath 验证路径安全性，防止路径穿越
func (h *WorkerFilesHandler) validatePath(path string) (string, bool) {
	// 清理路径
	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, "..") {
		return "", false
	}

	fullPath := filepath.Join(h.workerDir, cleanPath)

	// 确保路径在 workerDir 内
	rel, err := filepath.Rel(h.workerDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}

	return fullPath, true
}
```

**Step 2: 运行测试确认编译通过**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功，无错误

**Step 3: Commit**

```bash
git add api/internal/handler/worker_files.go
git commit -m "$(cat <<'EOF'
feat(api): add worker files handler base structure

Add WorkerFilesHandler with path validation for security.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: 后端 - 目录列表和文件读取 API

**Files:**
- Modify: `api/internal/handler/worker_files.go`

**Step 1: 添加 ListDir 方法**

在 `worker_files.go` 中添加：

```go
// ListDir 列出目录内容
// GET /api/worker/files/*path
func (h *WorkerFilesHandler) ListDir(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		path = ""
	}

	fullPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	// 如果是文件，返回文件内容
	if !info.IsDir() {
		h.readFile(c, fullPath, path, info)
		return
	}

	// 读取目录内容
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	items := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		// 跳过隐藏文件和 __pycache__
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__pycache__" {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		item := FileInfo{
			Name:  entry.Name(),
			Mtime: entryInfo.ModTime(),
		}
		if entry.IsDir() {
			item.Type = "dir"
		} else {
			item.Type = "file"
			item.Size = entryInfo.Size()
		}
		items = append(items, item)
	}

	// 排序：目录在前，按名称排序
	sort.Slice(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == "dir"
		}
		return items[i].Name < items[j].Name
	})

	core.Success(c, gin.H{
		"path":  path,
		"items": items,
	})
}

// readFile 读取文件内容
func (h *WorkerFilesHandler) readFile(c *gin.Context, fullPath, relativePath string, info os.FileInfo) {
	content, err := os.ReadFile(fullPath)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	core.Success(c, gin.H{
		"path":    relativePath,
		"content": string(content),
		"size":    info.Size(),
		"mtime":   info.ModTime(),
	})
}
```

**Step 2: 运行测试**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add api/internal/handler/worker_files.go
git commit -m "$(cat <<'EOF'
feat(api): add ListDir and readFile methods

Implement directory listing with sorting (dirs first) and file content reading.
Skip hidden files and __pycache__ directories.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: 后端 - 目录树、创建、保存、删除 API

**Files:**
- Modify: `api/internal/handler/worker_files.go`

**Step 1: 添加 GetTree 方法（用于移动弹窗）**

```go
// GetTree 获取目录树
// GET /api/worker/files?tree=true
func (h *WorkerFilesHandler) GetTree(c *gin.Context) {
	tree := h.buildTree(h.workerDir, "/")
	core.Success(c, tree)
}

// buildTree 递归构建目录树
func (h *WorkerFilesHandler) buildTree(dirPath, relativePath string) *TreeNode {
	info, err := os.Stat(dirPath)
	if err != nil {
		return nil
	}

	node := &TreeNode{
		Name: filepath.Base(dirPath),
		Path: relativePath,
		Type: "dir",
	}

	if relativePath == "/" {
		node.Name = "worker"
	}

	if !info.IsDir() {
		node.Type = "file"
		return node
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return node
	}

	for _, entry := range entries {
		// 只包含目录
		if !entry.IsDir() {
			continue
		}
		// 跳过隐藏目录和 __pycache__
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__pycache__" {
			continue
		}

		childPath := filepath.Join(relativePath, entry.Name())
		child := h.buildTree(filepath.Join(dirPath, entry.Name()), childPath)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	// 按名称排序
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Name < node.Children[j].Name
	})

	return node
}
```

**Step 2: 添加 Create, Save, Delete 方法**

```go
// CreateRequest 创建请求
type CreateRequest struct {
	Type string `json:"type" binding:"required,oneof=file dir"` // file 或 dir
	Name string `json:"name" binding:"required"`
}

// Create 创建文件或目录
// POST /api/worker/files/*path
func (h *WorkerFilesHandler) Create(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		path = ""
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
		return
	}

	// 验证文件名
	if strings.ContainsAny(req.Name, "/\\:*?\"<>|") {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件名包含非法字符")
		return
	}

	targetPath := filepath.Join(path, req.Name)
	fullPath, ok := h.validatePath(targetPath)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	// 检查是否已存在
	if _, err := os.Stat(fullPath); err == nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件或目录已存在")
		return
	}

	if req.Type == "dir" {
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}
	} else {
		// 确保父目录存在
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}
		// 创建空文件
		f, err := os.Create(fullPath)
		if err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}
		f.Close()
	}

	core.Success(c, gin.H{"message": "创建成功", "path": targetPath})
}

// SaveRequest 保存请求
type SaveRequest struct {
	Content string `json:"content"`
}

// Save 保存文件
// PUT /api/worker/files/*path
func (h *WorkerFilesHandler) Save(c *gin.Context) {
	path := c.Param("path")

	fullPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
		return
	}

	// 检查文件是否存在
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	if info.IsDir() {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能保存目录")
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	core.Success(c, gin.H{"message": "保存成功"})
}

// Delete 删除文件或目录
// DELETE /api/worker/files/*path
func (h *WorkerFilesHandler) Delete(c *gin.Context) {
	path := c.Param("path")

	fullPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	// 不允许删除根目录
	if path == "" || path == "/" {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能删除根目录")
		return
	}

	// 检查是否存在
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	core.Success(c, gin.H{"message": "删除成功"})
}
```

**Step 3: 运行测试**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add api/internal/handler/worker_files.go
git commit -m "$(cat <<'EOF'
feat(api): add GetTree, Create, Save, Delete methods

Implement full file CRUD operations for worker code management.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: 后端 - 重命名、移动、上传、下载 API

**Files:**
- Modify: `api/internal/handler/worker_files.go`

**Step 1: 添加 Move (重命名/移动) 方法**

```go
// MoveRequest 移动/重命名请求
type MoveRequest struct {
	NewPath string `json:"new_path" binding:"required"`
}

// Move 重命名或移动文件/目录
// PATCH /api/worker/files/*path
func (h *WorkerFilesHandler) Move(c *gin.Context) {
	path := c.Param("path")

	oldPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的源路径")
		return
	}

	var req MoveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
		return
	}

	newPath, ok := h.validatePath(req.NewPath)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的目标路径")
		return
	}

	// 检查源是否存在
	if _, err := os.Stat(oldPath); err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	// 检查目标是否已存在
	if _, err := os.Stat(newPath); err == nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "目标路径已存在")
		return
	}

	// 确保目标父目录存在
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	core.Success(c, gin.H{"message": "移动成功", "new_path": req.NewPath})
}
```

**Step 2: 添加 Upload 和 Download 方法**

```go
// Upload 上传文件
// POST /api/worker/upload/*path
func (h *WorkerFilesHandler) Upload(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		path = ""
	}

	dirPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, err.Error())
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		core.FailWithMessage(c, core.ErrInvalidParam, "没有上传文件")
		return
	}

	uploaded := make([]string, 0, len(files))
	for _, file := range files {
		// 验证文件名
		if strings.ContainsAny(file.Filename, "/\\:*?\"<>|") {
			continue
		}

		dst := filepath.Join(dirPath, file.Filename)
		if err := c.SaveUploadedFile(file, dst); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, err.Error())
			return
		}
		uploaded = append(uploaded, file.Filename)
	}

	core.Success(c, gin.H{
		"message": "上传成功",
		"files":   uploaded,
		"count":   len(uploaded),
	})
}

// Download 下载文件
// GET /api/worker/download/*path
func (h *WorkerFilesHandler) Download(c *gin.Context) {
	path := c.Param("path")

	fullPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	if info.IsDir() {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能下载目录")
		return
	}

	c.FileAttachment(fullPath, filepath.Base(path))
}
```

**Step 3: 运行测试**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 4: Commit**

```bash
git add api/internal/handler/worker_files.go
git commit -m "$(cat <<'EOF'
feat(api): add Move, Upload, Download methods

Complete file operation APIs for worker code editor.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: 后端 - 运行测试 WebSocket

**Files:**
- Modify: `api/internal/handler/worker_files.go`

**Step 1: 添加 RunFile WebSocket 方法**

```go
import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// RunFile 运行 Python 文件（WebSocket）
// WS /api/worker/run
func (h *WorkerFilesHandler) RunFile(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	// 读取运行请求
	var req struct {
		Action string `json:"action"`
		File   string `json:"file"`
	}
	if err := conn.ReadJSON(&req); err != nil {
		h.sendWSError(conn, "读取请求失败: "+err.Error())
		return
	}

	if req.Action != "run" || req.File == "" {
		h.sendWSError(conn, "无效的请求")
		return
	}

	// 验证路径
	fullPath, ok := h.validatePath(req.File)
	if !ok {
		h.sendWSError(conn, "无效的文件路径")
		return
	}

	// 检查文件存在
	if _, err := os.Stat(fullPath); err != nil {
		h.sendWSError(conn, "文件不存在")
		return
	}

	// 创建上下文用于取消
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 执行 Python 文件
	cmd := exec.CommandContext(ctx, "python", fullPath)
	cmd.Dir = h.workerDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		h.sendWSError(conn, "创建 stdout 管道失败: "+err.Error())
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		h.sendWSError(conn, "创建 stderr 管道失败: "+err.Error())
		return
	}

	start := time.Now()
	if err := cmd.Start(); err != nil {
		h.sendWSError(conn, "启动进程失败: "+err.Error())
		return
	}

	// 并发读取输出
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		h.pipeToWS(stdout, conn, "stdout")
	}()

	go func() {
		defer wg.Done()
		h.pipeToWS(stderr, conn, "stderr")
	}()

	wg.Wait()
	cmd.Wait()

	// 发送完成消息
	duration := time.Since(start).Milliseconds()
	exitCode := 0
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	conn.WriteJSON(map[string]interface{}{
		"type":        "done",
		"exit_code":   exitCode,
		"duration_ms": duration,
	})
}

// pipeToWS 将输出流发送到 WebSocket
func (h *WorkerFilesHandler) pipeToWS(r io.Reader, conn *websocket.Conn, typ string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		msg := map[string]string{
			"type": typ,
			"data": scanner.Text(),
		}
		if err := conn.WriteJSON(msg); err != nil {
			return
		}
	}
}

// sendWSError 发送 WebSocket 错误
func (h *WorkerFilesHandler) sendWSError(conn *websocket.Conn, msg string) {
	conn.WriteJSON(map[string]string{
		"type": "stderr",
		"data": msg,
	})
	conn.WriteJSON(map[string]interface{}{
		"type":      "done",
		"exit_code": 1,
	})
}
```

**Step 2: 运行测试**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add api/internal/handler/worker_files.go
git commit -m "$(cat <<'EOF'
feat(api): add RunFile WebSocket for code execution

Execute Python files with real-time stdout/stderr streaming via WebSocket.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: 后端 - 重启和重建 API

**Files:**
- Modify: `api/internal/handler/worker_files.go`

**Step 1: 添加 Restart 和 Rebuild 方法**

```go
import (
	"github.com/redis/go-redis/v9"
)

// Restart 重启 Worker 进程
// POST /api/worker/restart
func (h *WorkerFilesHandler) Restart(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		core.FailWithMessage(c, core.ErrInternalServer, "Redis 未连接")
		return
	}
	redisClient := rdb.(*redis.Client)

	// 发送重启信号到 Redis
	err := redisClient.Publish(c.Request.Context(), "worker:command", "restart").Err()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	core.Success(c, gin.H{"message": "重启指令已发送"})
}

// Rebuild 重新构建 Worker 镜像
// POST /api/worker/rebuild
func (h *WorkerFilesHandler) Rebuild(c *gin.Context) {
	// 获取 docker-compose 路径
	composeFile := "/project/docker-compose.yml"
	if _, err := os.Stat(composeFile); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "docker-compose.yml 未找到")
		return
	}

	// 使用 context 设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"docker-compose",
		"-f", composeFile,
		"up", "-d", "--build", "worker",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, string(output))
		return
	}

	core.Success(c, gin.H{
		"message": "Worker 重新构建完成",
		"output":  string(output),
	})
}
```

**Step 2: 运行测试**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add api/internal/handler/worker_files.go
git commit -m "$(cat <<'EOF'
feat(api): add Restart and Rebuild endpoints

Add Redis-based restart signal and docker-compose rebuild capability.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: 后端 - 注册路由

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 在 router.go 中添加 Worker 文件路由**

在 `SetupRouter` 函数中，在 Processor routes 后面添加：

```go
	// Worker Files routes (代码编辑器，require JWT)
	workerFilesHandler := NewWorkerFilesHandler("/project/worker")
	workerRoutes := r.Group("/api/worker")
	workerRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		// 目录树（用于移动弹窗）
		workerRoutes.GET("/files", func(c *gin.Context) {
			if c.Query("tree") == "true" {
				workerFilesHandler.GetTree(c)
			} else {
				workerFilesHandler.ListDir(c)
			}
		})
		// 文件/目录操作
		workerRoutes.GET("/files/*path", workerFilesHandler.ListDir)
		workerRoutes.POST("/files/*path", workerFilesHandler.Create)
		workerRoutes.PUT("/files/*path", workerFilesHandler.Save)
		workerRoutes.DELETE("/files/*path", workerFilesHandler.Delete)
		workerRoutes.PATCH("/files/*path", workerFilesHandler.Move)

		// 上传下载
		workerRoutes.POST("/upload/*path", workerFilesHandler.Upload)
		workerRoutes.GET("/download/*path", workerFilesHandler.Download)

		// 控制
		workerRoutes.POST("/restart", workerFilesHandler.Restart)
		workerRoutes.POST("/rebuild", workerFilesHandler.Rebuild)
	}

	// Worker Run WebSocket (需要认证)
	r.GET("/ws/worker/run", AuthMiddleware(deps.Config.Auth.SecretKey), workerFilesHandler.RunFile)
```

**Step 2: 运行测试**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "$(cat <<'EOF'
feat(api): register worker files routes

Add routes for file operations, upload/download, restart/rebuild, and run WebSocket.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Docker 配置 - 挂载卷

**Files:**
- Modify: `docker-compose.yml`

**Step 1: 更新 API 容器卷挂载**

在 `docker-compose.yml` 的 `api` 服务中添加卷挂载：

```yaml
  api:
    # ... existing config ...
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./api/templates:/app/templates
      - ./data/cache:/data/cache
      - ./data/emojis.json:/app/data/emojis.json:ro
      - ./worker:/project/worker                    # Worker 代码目录
      - ./docker-compose.yml:/project/docker-compose.yml:ro
      - /var/run/docker.sock:/var/run/docker.sock   # Docker socket (rebuild 需要)
```

更新 `worker` 服务卷挂载：

```yaml
  worker:
    # ... existing config ...
    volumes:
      - ./worker:/app                # 代码目录挂载（开发模式）
      - ./config.yaml:/app/config.yaml:ro
      - ./data/logs:/app/logs
```

**Step 2: 验证语法**

```bash
cd E:\j\模板\seo_html_generator && docker-compose config > /dev/null
```

Expected: 无错误输出

**Step 3: Commit**

```bash
git add docker-compose.yml
git commit -m "$(cat <<'EOF'
feat(docker): add volume mounts for worker code editor

Mount worker code directory and docker socket for API container.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9: Worker - 添加重启命令监听

**Files:**
- Modify: `worker/core/workers/command_listener.py`

**Step 1: 在 CommandListener 中添加重启监听**

在 `handle_command` 方法中添加重启处理：

```python
    async def handle_command(self, cmd: dict):
        """处理命令"""
        action = cmd.get("action")

        # 处理字符串命令（来自 worker:command 频道）
        if isinstance(cmd, str):
            if cmd == "restart":
                logger.info("收到重启指令，正在退出...")
                # 等待当前任务完成
                for project_id, task in list(self.running_tasks.items()):
                    if not task.done():
                        logger.info(f"等待项目 {project_id} 任务完成...")
                        task.cancel()
                        try:
                            await task
                        except asyncio.CancelledError:
                            pass
                # 退出进程，Docker 会自动重启
                import sys
                sys.exit(0)
            return

        # 原有的命令处理...
        project_id = cmd.get("project_id")
        # ...
```

同时更新 `start` 方法，订阅 `worker:command` 频道：

```python
    async def start(self):
        """启动监听器"""
        self.rdb = get_redis_client()
        if not self.rdb:
            logger.error("Redis 未初始化，无法启动命令监听器")
            return

        logger.info("命令监听器已启动，等待命令...")

        pubsub = self.rdb.pubsub()
        await pubsub.subscribe("spider:commands", "worker:command")  # 添加 worker:command

        async for message in pubsub.listen():
            if message["type"] == "message":
                try:
                    data = message["data"]
                    if isinstance(data, bytes):
                        data = data.decode('utf-8')

                    # 检查是否是简单字符串命令
                    if data == "restart":
                        await self.handle_command(data)
                    else:
                        cmd = json.loads(data)
                        await self.handle_command(cmd)
                except json.JSONDecodeError:
                    # 可能是简单字符串命令
                    await self.handle_command(data)
                except Exception as e:
                    logger.error(f"处理命令失败: {e}")
```

**Step 2: 验证语法**

```bash
cd E:\j\模板\seo_html_generator\worker && python -m py_compile core/workers/command_listener.py
```

Expected: 无错误

**Step 3: Commit**

```bash
git add worker/core/workers/command_listener.py
git commit -m "$(cat <<'EOF'
feat(worker): add restart command listener

Listen for restart command on worker:command Redis channel.
Gracefully cancel running tasks before exit.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10: 前端 - API 封装

**Files:**
- Create: `web/src/api/worker.ts`

**Step 1: 创建 worker.ts API 文件**

```typescript
import request from '@/utils/request'

// ============================================
// 类型定义
// ============================================

export interface FileInfo {
  name: string
  type: 'file' | 'dir'
  size?: number
  mtime: string
}

export interface FileContent {
  path: string
  content: string
  size: number
  mtime: string
}

export interface DirContent {
  path: string
  items: FileInfo[]
}

export interface TreeNode {
  name: string
  path: string
  type: 'file' | 'dir'
  children?: TreeNode[]
}

// ============================================
// 文件操作 API
// ============================================

/**
 * 获取目录内容
 */
export const getDir = async (path: string = ''): Promise<DirContent> => {
  const res = await request.get(`/worker/files/${path}`)
  return res.data
}

/**
 * 获取文件内容
 */
export const getFile = async (path: string): Promise<FileContent> => {
  const res = await request.get(`/worker/files/${path}`)
  return res.data
}

/**
 * 获取目录树
 */
export const getFileTree = async (): Promise<TreeNode> => {
  const res = await request.get('/worker/files', { params: { tree: 'true' } })
  return res.data
}

/**
 * 保存文件
 */
export const saveFile = async (path: string, content: string): Promise<void> => {
  await request.put(`/worker/files/${path}`, { content })
}

/**
 * 创建文件或目录
 */
export const createItem = async (
  parentPath: string,
  name: string,
  type: 'file' | 'dir'
): Promise<void> => {
  await request.post(`/worker/files/${parentPath}`, { name, type })
}

/**
 * 删除文件或目录
 */
export const deleteItem = async (path: string): Promise<void> => {
  await request.delete(`/worker/files/${path}`)
}

/**
 * 移动/重命名
 */
export const moveItem = async (oldPath: string, newPath: string): Promise<void> => {
  await request.patch(`/worker/files/${oldPath}`, { new_path: newPath })
}

/**
 * 获取下载 URL
 */
export const getDownloadUrl = (path: string): string => {
  const token = localStorage.getItem('token')
  return `/api/worker/download/${path}?token=${token}`
}

// ============================================
// 控制 API
// ============================================

/**
 * 重启 Worker
 */
export const restartWorker = async (): Promise<{ message: string }> => {
  const res = await request.post('/worker/restart')
  return res.data
}

/**
 * 重新构建 Worker
 */
export const rebuildWorker = async (): Promise<{ message: string; output: string }> => {
  const res = await request.post('/worker/rebuild')
  return res.data
}

// ============================================
// WebSocket API
// ============================================

interface RunLogHandlers {
  onStdout: (data: string) => void
  onStderr: (data: string) => void
  onDone: (exitCode: number, durationMs: number) => void
  onError?: (error: string) => void
}

/**
 * 运行 Python 文件
 */
export function runFile(filePath: string, handlers: RunLogHandlers): () => void {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/ws/worker/run`

  let ws: WebSocket | null = null

  try {
    ws = new WebSocket(wsUrl)
  } catch (e) {
    handlers.onError?.(`WebSocket 创建失败: ${e}`)
    return () => {}
  }

  ws.onopen = () => {
    ws?.send(JSON.stringify({ action: 'run', file: filePath }))
  }

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data)
      switch (msg.type) {
        case 'stdout':
          handlers.onStdout(msg.data)
          break
        case 'stderr':
          handlers.onStderr(msg.data)
          break
        case 'done':
          handlers.onDone(msg.exit_code, msg.duration_ms || 0)
          ws?.close()
          break
      }
    } catch {
      // 忽略解析错误
    }
  }

  ws.onerror = () => {
    handlers.onError?.('WebSocket 连接失败')
  }

  return () => {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      ws.close()
    }
  }
}
```

**Step 2: 验证 TypeScript**

```bash
cd E:\j\模板\seo_html_generator\web && npx tsc --noEmit src/api/worker.ts
```

Expected: 无错误

**Step 3: Commit**

```bash
git add web/src/api/worker.ts
git commit -m "$(cat <<'EOF'
feat(web): add worker API module

Implement file operations, control, and WebSocket run API.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11: 前端 - 主页面组件

**Files:**
- Create: `web/src/views/worker/WorkerCodeManager.vue`

**Step 1: 创建 WorkerCodeManager.vue**

```vue
<template>
  <div class="worker-code-manager">
    <!-- 页面标题和操作 -->
    <div class="page-header">
      <h2>Worker 代码管理</h2>
      <div class="header-actions">
        <el-button type="warning" :icon="Refresh" @click="handleRestart" :loading="restarting">
          重启 Worker
        </el-button>
        <el-button type="danger" :icon="Setting" @click="handleRebuild" :loading="rebuilding">
          重新构建
        </el-button>
      </div>
    </div>

    <!-- 编辑器模式 -->
    <FileEditor
      v-if="editingFile"
      :file-path="editingFile.path"
      :content="editingFile.content"
      @save="handleSave"
      @close="closeEditor"
    />

    <!-- 文件列表模式 -->
    <template v-else>
      <!-- 工具栏 -->
      <FileToolbar
        :current-path="currentPath"
        @navigate="navigateTo"
        @upload-success="loadDir"
        @create-file="showCreateDialog('file')"
        @create-dir="showCreateDialog('dir')"
      />

      <!-- 文件列表 -->
      <FileTable
        :files="files"
        :loading="loading"
        :current-path="currentPath"
        @open="handleOpen"
        @edit="handleEdit"
        @rename="showRenameDialog"
        @move="showMoveDialog"
        @download="handleDownload"
        @delete="handleDelete"
        @upload-success="loadDir"
      />
    </template>

    <!-- 新建弹窗 -->
    <CreateDialog
      v-model="createDialogVisible"
      :type="createType"
      @confirm="handleCreate"
    />

    <!-- 重命名弹窗 -->
    <RenameDialog
      v-model="renameDialogVisible"
      :current-name="renamingItem?.name || ''"
      @confirm="handleRename"
    />

    <!-- 移动弹窗 -->
    <MoveDialog
      v-model="moveDialogVisible"
      :file-path="movingItem?.name || ''"
      @confirm="handleMove"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Setting } from '@element-plus/icons-vue'
import FileToolbar from './components/FileToolbar.vue'
import FileTable from './components/FileTable.vue'
import FileEditor from './components/FileEditor.vue'
import CreateDialog from './components/CreateDialog.vue'
import RenameDialog from './components/RenameDialog.vue'
import MoveDialog from './components/MoveDialog.vue'
import {
  getDir,
  getFile,
  saveFile,
  createItem,
  deleteItem,
  moveItem,
  getDownloadUrl,
  restartWorker,
  rebuildWorker,
  type FileInfo
} from '@/api/worker'

// 状态
const currentPath = ref('')
const files = ref<FileInfo[]>([])
const loading = ref(false)
const restarting = ref(false)
const rebuilding = ref(false)

// 编辑状态
const editingFile = ref<{ path: string; content: string } | null>(null)

// 弹窗状态
const createDialogVisible = ref(false)
const createType = ref<'file' | 'dir'>('file')
const renameDialogVisible = ref(false)
const renamingItem = ref<FileInfo | null>(null)
const moveDialogVisible = ref(false)
const movingItem = ref<FileInfo | null>(null)

// 加载目录
async function loadDir() {
  loading.value = true
  try {
    const res = await getDir(currentPath.value)
    files.value = res.items
  } catch (e: any) {
    ElMessage.error(e.message || '加载失败')
  } finally {
    loading.value = false
  }
}

// 导航
function navigateTo(path: string) {
  currentPath.value = path
  loadDir()
}

// 打开（双击）
function handleOpen(item: FileInfo) {
  if (item.type === 'dir') {
    currentPath.value = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
    loadDir()
  } else {
    handleEdit(item)
  }
}

// 编辑文件
async function handleEdit(item: FileInfo) {
  try {
    const path = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
    const res = await getFile(path)
    editingFile.value = { path, content: res.content }
  } catch (e: any) {
    ElMessage.error(e.message || '读取文件失败')
  }
}

// 保存文件
async function handleSave(content: string) {
  if (!editingFile.value) return
  try {
    await saveFile(editingFile.value.path, content)
    ElMessage.success('保存成功')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  }
}

// 关闭编辑器
function closeEditor() {
  editingFile.value = null
}

// 新建
function showCreateDialog(type: 'file' | 'dir') {
  createType.value = type
  createDialogVisible.value = true
}

async function handleCreate(name: string) {
  try {
    await createItem(currentPath.value, name, createType.value)
    ElMessage.success('创建成功')
    loadDir()
  } catch (e: any) {
    ElMessage.error(e.message || '创建失败')
  }
}

// 重命名
function showRenameDialog(item: FileInfo) {
  renamingItem.value = item
  renameDialogVisible.value = true
}

async function handleRename(newName: string) {
  if (!renamingItem.value) return
  const oldPath = currentPath.value
    ? `${currentPath.value}/${renamingItem.value.name}`
    : renamingItem.value.name
  const newPath = currentPath.value ? `${currentPath.value}/${newName}` : newName
  try {
    await moveItem(oldPath, newPath)
    ElMessage.success('重命名成功')
    loadDir()
  } catch (e: any) {
    ElMessage.error(e.message || '重命名失败')
  }
}

// 移动
function showMoveDialog(item: FileInfo) {
  movingItem.value = item
  moveDialogVisible.value = true
}

async function handleMove(targetDir: string) {
  if (!movingItem.value) return
  const oldPath = currentPath.value
    ? `${currentPath.value}/${movingItem.value.name}`
    : movingItem.value.name
  const newPath = `${targetDir}/${movingItem.value.name}`
  try {
    await moveItem(oldPath, newPath)
    ElMessage.success('移动成功')
    loadDir()
  } catch (e: any) {
    ElMessage.error(e.message || '移动失败')
  }
}

// 下载
function handleDownload(item: FileInfo) {
  const path = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
  window.open(getDownloadUrl(path), '_blank')
}

// 删除
async function handleDelete(item: FileInfo) {
  try {
    await ElMessageBox.confirm(`确定删除 ${item.name} 吗？`, '确认删除', {
      type: 'warning'
    })
    const path = currentPath.value ? `${currentPath.value}/${item.name}` : item.name
    await deleteItem(path)
    ElMessage.success('删除成功')
    loadDir()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
}

// 重启
async function handleRestart() {
  try {
    await ElMessageBox.confirm('确定重启 Worker 吗？', '确认重启', { type: 'warning' })
    restarting.value = true
    await restartWorker()
    ElMessage.success('重启指令已发送')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重启失败')
    }
  } finally {
    restarting.value = false
  }
}

// 重建
async function handleRebuild() {
  try {
    await ElMessageBox.confirm(
      '重新构建将重新安装所有依赖，可能需要几分钟时间。确定继续吗？',
      '确认重建',
      { type: 'warning' }
    )
    rebuilding.value = true
    await rebuildWorker()
    ElMessage.success('Worker 重新构建完成')
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '重建失败')
    }
  } finally {
    rebuilding.value = false
  }
}

onMounted(() => {
  loadDir()
})
</script>

<style scoped>
.worker-code-manager {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 10px;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/WorkerCodeManager.vue
git commit -m "$(cat <<'EOF'
feat(web): add WorkerCodeManager main component

Implement main page with file listing, navigation, and control buttons.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12: 前端 - FileToolbar 组件

**Files:**
- Create: `web/src/views/worker/components/FileToolbar.vue`

**Step 1: 创建 FileToolbar.vue**

```vue
<template>
  <div class="file-toolbar">
    <!-- 面包屑导航 -->
    <el-breadcrumb separator="/">
      <el-breadcrumb-item @click="$emit('navigate', '')">
        <el-icon><Folder /></el-icon> worker
      </el-breadcrumb-item>
      <el-breadcrumb-item
        v-for="(segment, index) in pathSegments"
        :key="index"
        @click="navigateToSegment(index)"
      >
        {{ segment }}
      </el-breadcrumb-item>
    </el-breadcrumb>

    <!-- 操作按钮 -->
    <div class="actions">
      <el-upload
        :action="uploadUrl"
        :headers="uploadHeaders"
        :show-file-list="false"
        :on-success="onUploadSuccess"
        :on-error="onUploadError"
        multiple
        name="files"
      >
        <el-button :icon="Upload">上传</el-button>
      </el-upload>
      <el-button :icon="DocumentAdd" @click="$emit('create-file')">新建文件</el-button>
      <el-button :icon="FolderAdd" @click="$emit('create-dir')">新建目录</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Upload, DocumentAdd, FolderAdd, Folder } from '@element-plus/icons-vue'

const props = defineProps<{
  currentPath: string
}>()

const emit = defineEmits<{
  (e: 'navigate', path: string): void
  (e: 'upload-success'): void
  (e: 'create-file'): void
  (e: 'create-dir'): void
}>()

const pathSegments = computed(() => {
  if (!props.currentPath) return []
  return props.currentPath.split('/').filter(Boolean)
})

const uploadUrl = computed(() => {
  return `/api/worker/upload/${props.currentPath || ''}`
})

const uploadHeaders = computed(() => {
  return {
    Authorization: `Bearer ${localStorage.getItem('token')}`
  }
})

function navigateToSegment(index: number) {
  const segments = pathSegments.value.slice(0, index + 1)
  emit('navigate', segments.join('/'))
}

function onUploadSuccess() {
  ElMessage.success('上传成功')
  emit('upload-success')
}

function onUploadError(error: any) {
  ElMessage.error('上传失败: ' + (error.message || '未知错误'))
}
</script>

<style scoped>
.file-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 15px;
  background: #f5f7fa;
  border-radius: 4px;
  margin-bottom: 15px;
}

.el-breadcrumb {
  font-size: 14px;
}

.el-breadcrumb-item {
  cursor: pointer;
}

.el-breadcrumb-item:hover {
  color: #409eff;
}

.actions {
  display: flex;
  gap: 10px;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/FileToolbar.vue
git commit -m "$(cat <<'EOF'
feat(web): add FileToolbar component

Implement breadcrumb navigation and action buttons.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 13: 前端 - FileTable 组件

**Files:**
- Create: `web/src/views/worker/components/FileTable.vue`

**Step 1: 创建 FileTable.vue**

```vue
<template>
  <div class="file-table-container">
    <el-table
      :data="files"
      v-loading="loading"
      @row-dblclick="handleDblClick"
      style="width: 100%"
    >
      <!-- 文件名 -->
      <el-table-column label="名称" min-width="200">
        <template #default="{ row }">
          <div class="file-name-cell">
            <el-icon v-if="row.type === 'dir'" class="folder-icon"><Folder /></el-icon>
            <el-icon v-else class="file-icon"><Document /></el-icon>
            <span class="file-name">{{ row.name }}</span>
          </div>
        </template>
      </el-table-column>

      <!-- 大小 -->
      <el-table-column label="大小" width="100">
        <template #default="{ row }">
          {{ row.type === 'dir' ? '-' : formatSize(row.size) }}
        </template>
      </el-table-column>

      <!-- 修改时间 -->
      <el-table-column label="修改时间" width="160">
        <template #default="{ row }">
          {{ formatTime(row.mtime) }}
        </template>
      </el-table-column>

      <!-- 操作 -->
      <el-table-column label="操作" width="100" fixed="right">
        <template #default="{ row }">
          <el-dropdown @command="handleCommand($event, row)">
            <el-button text type="primary">
              更多 <el-icon><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item v-if="row.type === 'file'" command="edit">
                  编辑
                </el-dropdown-item>
                <el-dropdown-item command="rename">重命名</el-dropdown-item>
                <el-dropdown-item command="move">移动</el-dropdown-item>
                <el-dropdown-item v-if="row.type === 'file'" command="download">
                  下载
                </el-dropdown-item>
                <el-dropdown-item command="delete" divided>
                  <span style="color: #f56c6c">删除</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <!-- 拖拽上传区域 -->
    <el-upload
      class="upload-dragger"
      drag
      :action="uploadUrl"
      :headers="uploadHeaders"
      :show-file-list="false"
      :on-success="onUploadSuccess"
      multiple
      name="files"
    >
      <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
      <div class="el-upload__text">拖拽文件到此处上传</div>
    </el-upload>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Folder, Document, ArrowDown, UploadFilled } from '@element-plus/icons-vue'
import type { FileInfo } from '@/api/worker'

const props = defineProps<{
  files: FileInfo[]
  loading: boolean
  currentPath: string
}>()

const emit = defineEmits<{
  (e: 'open', item: FileInfo): void
  (e: 'edit', item: FileInfo): void
  (e: 'rename', item: FileInfo): void
  (e: 'move', item: FileInfo): void
  (e: 'download', item: FileInfo): void
  (e: 'delete', item: FileInfo): void
  (e: 'upload-success'): void
}>()

const uploadUrl = computed(() => {
  return `/api/worker/upload/${props.currentPath || ''}`
})

const uploadHeaders = computed(() => {
  return {
    Authorization: `Bearer ${localStorage.getItem('token')}`
  }
})

function handleDblClick(row: FileInfo) {
  emit('open', row)
}

function handleCommand(command: string, row: FileInfo) {
  switch (command) {
    case 'edit':
      emit('edit', row)
      break
    case 'rename':
      emit('rename', row)
      break
    case 'move':
      emit('move', row)
      break
    case 'download':
      emit('download', row)
      break
    case 'delete':
      emit('delete', row)
      break
  }
}

function formatSize(bytes?: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  while (bytes >= 1024 && i < units.length - 1) {
    bytes /= 1024
    i++
  }
  return `${bytes.toFixed(1)} ${units[i]}`
}

function formatTime(time: string): string {
  if (!time) return '-'
  const date = new Date(time)
  return date.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}

function onUploadSuccess() {
  ElMessage.success('上传成功')
  emit('upload-success')
}
</script>

<style scoped>
.file-table-container {
  margin-bottom: 20px;
}

.file-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.folder-icon {
  color: #e6a23c;
  font-size: 18px;
}

.file-icon {
  color: #909399;
  font-size: 18px;
}

.upload-dragger {
  margin-top: 15px;
}

.upload-dragger :deep(.el-upload-dragger) {
  padding: 20px;
  border-style: dashed;
}

.el-icon--upload {
  font-size: 40px;
  color: #c0c4cc;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/FileTable.vue
git commit -m "$(cat <<'EOF'
feat(web): add FileTable component

Implement file listing with dropdown actions and drag-drop upload.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 14: 前端 - FileEditor 组件

**Files:**
- Create: `web/src/views/worker/components/FileEditor.vue`

**Step 1: 创建 FileEditor.vue**

```vue
<template>
  <div class="file-editor">
    <!-- 工具栏 -->
    <div class="editor-toolbar">
      <span class="file-path">
        <el-icon><Document /></el-icon>
        {{ filePath }}
      </span>
      <div class="actions">
        <el-button type="primary" :icon="VideoPlay" @click="runFile" :loading="running">
          运行
        </el-button>
        <el-button type="success" :icon="Check" @click="handleSave" :loading="saving">
          保存
        </el-button>
        <el-button @click="$emit('close')">关闭</el-button>
      </div>
    </div>

    <!-- Monaco 编辑器 -->
    <div class="editor-container" ref="editorContainer"></div>

    <!-- 运行日志 -->
    <div class="log-panel">
      <div class="log-header">
        <span>运行日志</span>
        <el-button text @click="clearLog" size="small">清空</el-button>
      </div>
      <div class="log-content" ref="logContainer">
        <div
          v-for="(log, index) in logs"
          :key="index"
          :class="['log-line', log.type]"
        >
          {{ log.data }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { ElMessage } from 'element-plus'
import { Document, VideoPlay, Check } from '@element-plus/icons-vue'
import * as monaco from 'monaco-editor'
import { runFile as runFileApi } from '@/api/worker'

const props = defineProps<{
  filePath: string
  content: string
}>()

const emit = defineEmits<{
  (e: 'save', content: string): void
  (e: 'close'): void
}>()

const editorContainer = ref<HTMLElement>()
const logContainer = ref<HTMLElement>()
let editor: monaco.editor.IStandaloneCodeEditor | null = null

const logs = ref<{ type: string; data: string }[]>([])
const running = ref(false)
const saving = ref(false)
let stopRun: (() => void) | null = null

onMounted(() => {
  if (editorContainer.value) {
    editor = monaco.editor.create(editorContainer.value, {
      value: props.content,
      language: 'python',
      theme: 'vs-dark',
      automaticLayout: true,
      minimap: { enabled: true },
      fontSize: 14,
      tabSize: 4,
      lineNumbers: 'on',
      scrollBeyondLastLine: false,
    })

    // Ctrl+S 保存
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
      handleSave()
    })
  }
})

onUnmounted(() => {
  editor?.dispose()
  stopRun?.()
})

function handleSave() {
  if (!editor) return
  saving.value = true
  emit('save', editor.getValue())
  setTimeout(() => {
    saving.value = false
  }, 500)
}

function runFile() {
  if (!editor) return

  running.value = true
  logs.value = []
  logs.value.push({ type: 'info', data: `> python ${props.filePath}` })

  stopRun = runFileApi(props.filePath, {
    onStdout: (data) => {
      logs.value.push({ type: 'stdout', data })
      scrollToBottom()
    },
    onStderr: (data) => {
      logs.value.push({ type: 'stderr', data })
      scrollToBottom()
    },
    onDone: (exitCode, durationMs) => {
      logs.value.push({
        type: 'info',
        data: `> 进程退出，code=${exitCode}，耗时 ${durationMs}ms`
      })
      running.value = false
      scrollToBottom()
    },
    onError: (error) => {
      logs.value.push({ type: 'stderr', data: error })
      running.value = false
      scrollToBottom()
    }
  })
}

function clearLog() {
  logs.value = []
}

function scrollToBottom() {
  nextTick(() => {
    if (logContainer.value) {
      logContainer.value.scrollTop = logContainer.value.scrollHeight
    }
  })
}
</script>

<style scoped>
.file-editor {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 150px);
}

.editor-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 15px;
  background: #f5f7fa;
  border-radius: 4px;
  margin-bottom: 10px;
}

.file-path {
  display: flex;
  align-items: center;
  gap: 5px;
  font-weight: 500;
}

.actions {
  display: flex;
  gap: 10px;
}

.editor-container {
  flex: 1;
  min-height: 400px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.log-panel {
  margin-top: 10px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  max-height: 200px;
  display: flex;
  flex-direction: column;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: #f5f7fa;
  border-bottom: 1px solid #dcdfe6;
  font-weight: 500;
}

.log-content {
  flex: 1;
  overflow-y: auto;
  padding: 10px;
  background: #1e1e1e;
  font-family: 'Consolas', 'Monaco', monospace;
  font-size: 13px;
}

.log-line {
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.6;
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
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/worker/components/FileEditor.vue
git commit -m "$(cat <<'EOF'
feat(web): add FileEditor component with Monaco Editor

Implement Python code editor with run/save functionality and log panel.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 15: 前端 - 弹窗组件 (CreateDialog, RenameDialog, MoveDialog)

**Files:**
- Create: `web/src/views/worker/components/CreateDialog.vue`
- Create: `web/src/views/worker/components/RenameDialog.vue`
- Create: `web/src/views/worker/components/MoveDialog.vue`

**Step 1: 创建 CreateDialog.vue**

```vue
<template>
  <el-dialog
    v-model="visible"
    :title="type === 'file' ? '新建文件' : '新建目录'"
    width="400px"
  >
    <el-form @submit.prevent="confirm">
      <el-form-item :label="type === 'file' ? '文件名' : '目录名'">
        <el-input v-model="name" :placeholder="placeholder" ref="inputRef" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirm" :disabled="!name.trim()">
        创建
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  modelValue: boolean
  type: 'file' | 'dir'
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', name: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const name = ref('')
const inputRef = ref()

const placeholder = computed(() => {
  return props.type === 'file' ? '例如: processor.py' : '例如: utils'
})

watch(() => props.modelValue, (val) => {
  if (val) {
    name.value = ''
    nextTick(() => inputRef.value?.focus())
  }
})

function confirm() {
  if (name.value.trim()) {
    emit('confirm', name.value.trim())
    visible.value = false
  }
}
</script>
```

**Step 2: 创建 RenameDialog.vue**

```vue
<template>
  <el-dialog v-model="visible" title="重命名" width="400px">
    <el-form @submit.prevent="confirm">
      <el-form-item label="新名称">
        <el-input v-model="newName" ref="inputRef" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirm" :disabled="!newName.trim()">
        确定
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  modelValue: boolean
  currentName: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', newName: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const newName = ref('')
const inputRef = ref()

watch(() => props.modelValue, (val) => {
  if (val) {
    newName.value = props.currentName
    nextTick(() => {
      inputRef.value?.focus()
      inputRef.value?.select()
    })
  }
})

function confirm() {
  if (newName.value.trim() && newName.value !== props.currentName) {
    emit('confirm', newName.value.trim())
    visible.value = false
  }
}
</script>
```

**Step 3: 创建 MoveDialog.vue**

```vue
<template>
  <el-dialog v-model="visible" title="移动到" width="400px">
    <p style="margin-bottom: 10px">选择目标目录：</p>
    <el-tree
      :data="treeData"
      :props="{ label: 'name', children: 'children' }"
      node-key="path"
      highlight-current
      :expand-on-click-node="false"
      @node-click="selectDir"
      default-expand-all
      v-loading="loading"
    >
      <template #default="{ data }">
        <span class="tree-node">
          <el-icon><Folder /></el-icon>
          <span>{{ data.name }}</span>
        </span>
      </template>
    </el-tree>

    <p v-if="selectedPath" style="margin-top: 10px; color: #409eff">
      当前选择：{{ selectedPath }}
    </p>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirm" :disabled="!selectedPath">
        确定移动
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Folder } from '@element-plus/icons-vue'
import { getFileTree, type TreeNode } from '@/api/worker'

const props = defineProps<{
  modelValue: boolean
  filePath: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'confirm', targetDir: string): void
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (val) => emit('update:modelValue', val)
})

const treeData = ref<TreeNode[]>([])
const selectedPath = ref('')
const loading = ref(false)

watch(() => props.modelValue, async (val) => {
  if (val) {
    selectedPath.value = ''
    loading.value = true
    try {
      const tree = await getFileTree()
      treeData.value = [tree]
    } catch {
      treeData.value = []
    } finally {
      loading.value = false
    }
  }
})

function selectDir(data: TreeNode) {
  selectedPath.value = data.path
}

function confirm() {
  if (selectedPath.value) {
    emit('confirm', selectedPath.value)
    visible.value = false
  }
}
</script>

<style scoped>
.tree-node {
  display: flex;
  align-items: center;
  gap: 5px;
}
</style>
```

**Step 4: Commit**

```bash
git add web/src/views/worker/components/CreateDialog.vue web/src/views/worker/components/RenameDialog.vue web/src/views/worker/components/MoveDialog.vue
git commit -m "$(cat <<'EOF'
feat(web): add dialog components for file operations

Add CreateDialog, RenameDialog, and MoveDialog components.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 16: 前端 - 路由配置

**Files:**
- Modify: `web/src/router/index.ts`

**Step 1: 添加 Worker 代码管理路由**

在 `children` 数组中添加新路由（在 processor 后面）：

```typescript
      {
        path: 'worker',
        name: 'WorkerCode',
        component: () => import('@/views/worker/WorkerCodeManager.vue'),
        meta: { title: 'Worker代码', icon: 'EditPen' }
      },
```

**Step 2: 验证路由**

```bash
cd E:\j\模板\seo_html_generator\web && npx tsc --noEmit
```

Expected: 无错误

**Step 3: Commit**

```bash
git add web/src/router/index.ts
git commit -m "$(cat <<'EOF'
feat(web): add worker code manager route

Add route for /worker with WorkerCodeManager component.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 17: 安装 Monaco Editor

**Files:**
- Modify: `web/package.json`

**Step 1: 安装 Monaco Editor**

```bash
cd E:\j\模板\seo_html_generator\web && npm install monaco-editor
```

**Step 2: 配置 Vite (如果需要)**

如果使用 Vite，可能需要配置 Monaco Editor 的 worker。在 `vite.config.ts` 中添加：

```typescript
import monacoEditorPlugin from 'vite-plugin-monaco-editor'

export default defineConfig({
  plugins: [
    // ... existing plugins
    monacoEditorPlugin({})
  ]
})
```

或者安装：

```bash
npm install vite-plugin-monaco-editor
```

**Step 3: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "$(cat <<'EOF'
feat(web): add monaco-editor dependency

Install Monaco Editor for Python code editing.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## Task 18: 集成测试

**Step 1: 构建前端**

```bash
cd E:\j\模板\seo_html_generator\web && npm run build
```

Expected: 构建成功

**Step 2: 构建后端**

```bash
cd E:\j\模板\seo_html_generator\api && go build ./...
```

Expected: 编译成功

**Step 3: 启动 Docker 服务测试**

```bash
cd E:\j\模板\seo_html_generator && docker-compose up -d --build
```

**Step 4: 测试 API**

```bash
# 测试目录列表
curl -H "Authorization: Bearer <token>" http://localhost:8008/api/worker/files

# 测试目录树
curl -H "Authorization: Bearer <token>" http://localhost:8008/api/worker/files?tree=true
```

**Step 5: Commit 最终测试结果**

```bash
git add -A
git commit -m "$(cat <<'EOF'
test: verify worker code editor integration

All components built and tested successfully.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>
EOF
)"
```

---

## 完成清单

- [x] Task 1: 后端 - Worker 文件操作 Handler 基础结构
- [x] Task 2: 后端 - 目录列表和文件读取 API
- [x] Task 3: 后端 - 目录树、创建、保存、删除 API
- [x] Task 4: 后端 - 重命名、移动、上传、下载 API
- [x] Task 5: 后端 - 运行测试 WebSocket
- [x] Task 6: 后端 - 重启和重建 API
- [x] Task 7: 后端 - 注册路由
- [x] Task 8: Docker 配置 - 挂载卷
- [x] Task 9: Worker - 添加重启命令监听
- [x] Task 10: 前端 - API 封装
- [x] Task 11: 前端 - 主页面组件
- [x] Task 12: 前端 - FileToolbar 组件
- [x] Task 13: 前端 - FileTable 组件
- [x] Task 14: 前端 - FileEditor 组件
- [x] Task 15: 前端 - 弹窗组件
- [x] Task 16: 前端 - 路由配置
- [x] Task 17: 安装 Monaco Editor
- [x] Task 18: 集成测试
