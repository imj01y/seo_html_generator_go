package api

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	core "seo-generator/api/internal/service"
)

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

// validatePath 验证路径安全性，防止路径穿越攻击
// 返回 (fullPath, ok)
func (h *WorkerFilesHandler) validatePath(c *gin.Context, relativePath string) (string, bool) {
	// 清理路径
	cleanPath := filepath.Clean(relativePath)

	// 检查是否包含 ".."
	if strings.Contains(cleanPath, "..") {
		core.FailWithMessage(c, core.ErrForbidden, "路径包含非法字符")
		return "", false
	}

	// 构建完整路径
	fullPath := filepath.Join(h.workerDir, cleanPath)

	// 确保路径在 workerDir 内
	// 需要将两者都转为绝对路径进行比较
	absWorkerDir, err := filepath.Abs(h.workerDir)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "无法解析工作目录")
		return "", false
	}

	absFullPath, err := filepath.Abs(fullPath)
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "无法解析目标路径")
		return "", false
	}

	// 检查目标路径是否以 workerDir 为前缀
	if !strings.HasPrefix(absFullPath, absWorkerDir) {
		core.FailWithMessage(c, core.ErrForbidden, "路径超出允许范围")
		return "", false
	}

	return absFullPath, true
}
