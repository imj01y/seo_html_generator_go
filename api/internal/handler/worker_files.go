package api

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
