package api

import (
	"path/filepath"
	"strings"
	"time"
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

// validatePath 验证路径安全性，防止路径穿越
func (h *WorkerFilesHandler) validatePath(relativePath string) (string, bool) {
	// 清理路径
	cleanPath := filepath.Clean(relativePath)

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
