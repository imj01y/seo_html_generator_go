package api

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	core "seo-generator/api/internal/service"
)

// 文件大小限制
const maxFileSize = 10 * 1024 * 1024 // 10MB

// 支持的文本文件扩展名
var textExtensions = map[string]bool{
	".py": true, ".txt": true, ".json": true, ".yaml": true, ".yml": true,
	".md": true, ".html": true, ".css": true, ".js": true, ".ts": true,
	".go": true, ".sh": true, ".conf": true, ".ini": true, ".toml": true,
	".xml": true, ".sql": true, ".env": true, ".gitignore": true,
}

// isTextFile 判断文件是否为文本文件
func isTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		// 无扩展名的文件（如 Dockerfile, Makefile）
		base := filepath.Base(filename)
		return base == "Dockerfile" || base == "Makefile" || base == "requirements"
	}
	return textExtensions[ext]
}

// WorkerFilesHandler Worker 文件管理 handler
type WorkerFilesHandler struct {
	workerDir string
}

// NewWorkerFilesHandler 创建 WorkerFilesHandler
func NewWorkerFilesHandler(workerDir string) *WorkerFilesHandler {
	return &WorkerFilesHandler{workerDir: workerDir}
}

// FileInfo 文件信息
type FileInfo struct {
	Name  string    `json:"name"`
	Type  string    `json:"type"` // "file" 或 "dir"
	Size  int64     `json:"size,omitempty"`
	Mtime time.Time `json:"mtime"`
}

// TreeNode 目录树节点
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" 或 "dir"
	Children []*TreeNode `json:"children,omitempty"`
}

// validatePath 验证路径安全性，防止路径穿越
func (h *WorkerFilesHandler) validatePath(relativePath string) (string, bool) {
	// 清理路径
	cleanPath := filepath.Clean(relativePath)

	// 空路径或当前目录指向 workerDir 根目录，这是允许的（用于列出根目录）
	if cleanPath == "." || cleanPath == "" {
		return h.workerDir, true
	}

	// 检查是否包含 ".."
	if strings.HasPrefix(cleanPath, "..") || strings.Contains(cleanPath, ".."+string(filepath.Separator)) {
		return "", false
	}

	fullPath := filepath.Join(h.workerDir, cleanPath)

	// 使用 filepath.Rel 确保路径在 workerDir 内
	rel, err := filepath.Rel(h.workerDir, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}

	return fullPath, true
}

// ListDir 列出目录内容或读取文件
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

	var files []FileInfo
	for _, entry := range entries {
		name := entry.Name()

		// 跳过隐藏文件和 __pycache__ 目录
		if strings.HasPrefix(name, ".") || name == "__pycache__" {
			continue
		}

		entryInfo, err := entry.Info()
		if err != nil {
			continue
		}

		fileType := "file"
		if entry.IsDir() {
			fileType = "dir"
		}

		files = append(files, FileInfo{
			Name:  name,
			Type:  fileType,
			Size:  entryInfo.Size(),
			Mtime: entryInfo.ModTime(),
		})
	}

	// 排序：目录在前，按名称排序
	sort.Slice(files, func(i, j int) bool {
		if files[i].Type != files[j].Type {
			return files[i].Type == "dir"
		}
		return files[i].Name < files[j].Name
	})

	core.Success(c, gin.H{
		"path":  path,
		"files": files,
	})
}

// readFile 读取文件内容并返回 JSON
func (h *WorkerFilesHandler) readFile(c *gin.Context, fullPath, relativePath string, info os.FileInfo) {
	// 检查文件大小
	if info.Size() > maxFileSize {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件过大，最大支持 10MB")
		return
	}

	// 检查是否为文本文件
	if !isTextFile(relativePath) {
		core.FailWithMessage(c, core.ErrInvalidParam, "不支持编辑二进制文件")
		return
	}

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

// GetTree 获取目录树
// GET /api/worker/files?tree=true
func (h *WorkerFilesHandler) GetTree(c *gin.Context) {
	tree := h.buildTree(h.workerDir, "/")
	core.Success(c, tree)
}

// buildTree 递归构建目录树（包含文件和目录）
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
		// 跳过隐藏文件/目录和 __pycache__
		if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "__pycache__" {
			continue
		}

		childPath := filepath.Join(relativePath, entry.Name())
		if entry.IsDir() {
			// 递归处理目录
			child := h.buildTree(filepath.Join(dirPath, entry.Name()), childPath)
			if child != nil {
				node.Children = append(node.Children, child)
			}
		} else {
			// 直接添加文件节点
			node.Children = append(node.Children, &TreeNode{
				Name: entry.Name(),
				Path: childPath,
				Type: "file",
			})
		}
	}

	// 排序：目录在前，然后按名称排序
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].Type != node.Children[j].Type {
			return node.Children[i].Type == "dir"
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	return node
}

// CreateRequest 创建文件或目录请求
type CreateRequest struct {
	Type string `json:"type" binding:"required,oneof=file dir"`
	Name string `json:"name" binding:"required"`
}

// Create 创建文件或目录
// POST /api/worker/files/*path
func (h *WorkerFilesHandler) Create(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		path = ""
	}

	parentPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	// 确保父目录存在
	info, err := os.Stat(parentPath)
	if err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}
	if !info.IsDir() {
		core.FailWithMessage(c, core.ErrInvalidParam, "父路径不是目录")
		return
	}

	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	// 验证文件名不包含非法字符
	if strings.ContainsAny(req.Name, `/\:*?"<>|`) || strings.HasPrefix(req.Name, ".") {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件名包含非法字符")
		return
	}

	targetPath := filepath.Join(parentPath, req.Name)

	// 检查目标是否已存在
	if _, err := os.Stat(targetPath); err == nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "文件或目录已存在")
		return
	}

	if req.Type == "dir" {
		if err := os.MkdirAll(targetPath, 0755); err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, "创建目录失败: "+err.Error())
			return
		}
	} else {
		// 创建空文件
		file, err := os.Create(targetPath)
		if err != nil {
			core.FailWithMessage(c, core.ErrInternalServer, "创建文件失败: "+err.Error())
			return
		}
		file.Close()
	}

	core.Success(c, gin.H{
		"path": filepath.Join(path, req.Name),
		"name": req.Name,
		"type": req.Type,
	})
}

// SaveRequest 保存文件请求
type SaveRequest struct {
	Content string `json:"content"`
}

// Save 保存文件
// PUT /api/worker/files/*path
func (h *WorkerFilesHandler) Save(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		core.FailWithMessage(c, core.ErrInvalidParam, "路径不能为空")
		return
	}

	fullPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
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

	// 不能保存目录
	if info.IsDir() {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能保存目录")
		return
	}

	// 检查是否为文本文件
	if !isTextFile(path) {
		core.FailWithMessage(c, core.ErrInvalidParam, "不支持编辑二进制文件")
		return
	}

	var req SaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "参数错误: "+err.Error())
		return
	}

	// 写入内容
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "保存文件失败: "+err.Error())
		return
	}

	// 获取更新后的文件信息
	newInfo, _ := os.Stat(fullPath)
	mtime := time.Now()
	if newInfo != nil {
		mtime = newInfo.ModTime()
	}

	core.Success(c, gin.H{
		"path":  path,
		"size":  len(req.Content),
		"mtime": mtime,
	})
}

// Delete 删除文件或目录
// DELETE /api/worker/files/*path
func (h *WorkerFilesHandler) Delete(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能删除根目录")
		return
	}

	fullPath, ok := h.validatePath(path)
	if !ok {
		core.FailWithMessage(c, core.ErrInvalidParam, "无效的路径")
		return
	}

	// 再次检查是否试图删除根目录
	if fullPath == h.workerDir {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能删除根目录")
		return
	}

	// 检查文件或目录是否存在
	if _, err := os.Stat(fullPath); err != nil {
		if os.IsNotExist(err) {
			core.FailWithCode(c, core.ErrNotFound)
			return
		}
		core.FailWithMessage(c, core.ErrInternalServer, err.Error())
		return
	}

	// 删除文件或目录
	if err := os.RemoveAll(fullPath); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "删除失败: "+err.Error())
		return
	}

	core.SuccessWithMessage(c, "删除成功", nil)
}

// MoveRequest 移动/重命名请求
type MoveRequest struct {
	NewPath string `json:"new_path" binding:"required"`
}

// Move 重命名或移动文件/目录
// PATCH /api/worker/files/*path
func (h *WorkerFilesHandler) Move(c *gin.Context) {
	path := c.Param("path")
	if path == "" || path == "/" {
		core.FailWithMessage(c, core.ErrInvalidParam, "不能移动根目录")
		return
	}

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
		// 检查文件大小
		if file.Size > maxFileSize {
			continue // 跳过过大的文件
		}

		// 验证文件名（非法字符和隐藏文件）
		if strings.ContainsAny(file.Filename, "/\\:*?\"<>|") || strings.HasPrefix(file.Filename, ".") {
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

// RunFile 运行 Python 文件（WebSocket）
// WS /ws/worker/run
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

	// 创建上下文用于取消（5分钟超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 执行 Python 文件
	cmd := exec.CommandContext(ctx, "python", fullPath)
	cmd.Dir = h.workerDir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		h.sendWSError(conn, "创建 stdout 管道失败")
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		h.sendWSError(conn, "创建 stderr 管道失败")
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

	// 使用 context 设置超时（10分钟）
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
