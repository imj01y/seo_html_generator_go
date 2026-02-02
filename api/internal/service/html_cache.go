package core

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

// CacheStats holds cache statistics with atomic counters
type CacheStats struct {
	totalFiles  atomic.Int64 // 文件总数
	totalBytes  atomic.Int64 // 总字节数
	initialized atomic.Bool  // 是否完成初始化扫描
	lastScanAt  atomic.Int64 // 上次扫描完成时间戳
	scanning    atomic.Bool  // 是否正在扫描中
}

// HTMLCache manages HTML file caching with hash-layered directory structure
type HTMLCache struct {
	cacheDir  string
	maxSizeGB float64
	mu        sync.RWMutex
	stats     *CacheStats
}

// CacheMeta holds metadata for a cached file
type CacheMeta struct {
	Key       string    `json:"key"`
	Domain    string    `json:"domain"`
	Path      string    `json:"path"`
	Size      int       `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// NewHTMLCache creates a new HTML cache manager
func NewHTMLCache(cacheDir string, maxSizeGB float64) *HTMLCache {
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Error().Err(err).Str("dir", cacheDir).Msg("Failed to create cache directory")
	}

	// Create meta directory
	metaDir := filepath.Join(cacheDir, "_meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		log.Error().Err(err).Str("dir", metaDir).Msg("Failed to create meta directory")
	}

	cache := &HTMLCache{
		cacheDir:  cacheDir,
		maxSizeGB: maxSizeGB,
		stats:     &CacheStats{},
	}

	// 启动后台扫描统计
	go cache.scanAndUpdateStats()

	log.Info().
		Str("dir", cacheDir).
		Float64("max_size_gb", maxSizeGB).
		Msg("HTML cache initialized, background scan started")

	return cache
}

// generateCacheKey generates a cache key from domain and path
func (c *HTMLCache) generateCacheKey(domain, path string) string {
	raw := domain + ":" + path
	hash := md5.Sum([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// getPathHash generates a hash for the path
func (c *HTMLCache) getPathHash(path string) string {
	hash := md5.Sum([]byte(path))
	return hex.EncodeToString(hash[:])
}

// getCacheDir returns the current cache directory (thread-safe)
func (c *HTMLCache) getCacheDirSafe() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cacheDir
}

// normalizePath normalizes a URL path for file storage
func (c *HTMLCache) normalizePath(path string) string {
	// Remove leading slashes
	for len(path) > 0 && path[0] == '/' {
		path = path[1:]
	}

	// Empty or root path becomes index.html
	if path == "" || path == "/" {
		return "index.html"
	}

	// Add .html extension if missing
	if filepath.Ext(path) == "" {
		path = path + ".html"
	}

	return path
}

// getCachePath returns the cache file path using hash-layered structure
func (c *HTMLCache) getCachePath(domain, path string) string {
	normalized := c.normalizePath(path)
	pathHash := c.getPathHash(path)
	// Structure: {cache_dir}/{domain}/{hash[0:2]}/{hash[2:4]}/{normalized_path}
	return filepath.Join(c.getCacheDirSafe(), domain, pathHash[:2], pathHash[2:4], normalized)
}

// getMetaPath returns the metadata file path
func (c *HTMLCache) getMetaPath(domain, path string) string {
	cacheKey := c.generateCacheKey(domain, path)
	pathHash := c.getPathHash(path)
	return filepath.Join(c.getCacheDirSafe(), "_meta", domain, pathHash[:2], pathHash[2:4], cacheKey+".json")
}

// Set stores HTML content in the cache
func (c *HTMLCache) Set(domain, path, html string) error {
	cachePath := c.getCachePath(domain, path)
	metaPath := c.getMetaPath(domain, path)

	// 检查是否是覆盖已有文件
	var oldSize int64
	isNewFile := true
	if info, err := os.Stat(cachePath); err == nil {
		isNewFile = false
		oldSize = info.Size()
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return err
	}

	// Write HTML file
	newSize := int64(len(html))
	if err := os.WriteFile(cachePath, []byte(html), 0644); err != nil {
		return err
	}

	// 更新统计计数器
	if c.stats.initialized.Load() {
		if isNewFile {
			c.stats.totalFiles.Add(1)
			c.stats.totalBytes.Add(newSize)
		} else {
			// 覆盖文件：只更新大小差值
			c.stats.totalBytes.Add(newSize - oldSize)
		}
	}

	// Write metadata
	meta := CacheMeta{
		Key:       c.generateCacheKey(domain, path),
		Domain:    domain,
		Path:      path,
		Size:      len(html),
		CreatedAt: time.Now(),
	}

	metaData, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, metaData, 0644)
}

// Delete removes a cached file
func (c *HTMLCache) Delete(domain, path string) error {
	cachePath := c.getCachePath(domain, path)
	metaPath := c.getMetaPath(domain, path)

	// 删除前获取文件大小用于更新统计
	var fileSize int64
	if info, err := os.Stat(cachePath); err == nil {
		fileSize = info.Size()
	}

	err1 := os.Remove(cachePath)
	os.Remove(metaPath)

	// 文件删除成功后更新统计计数器
	if err1 == nil && c.stats.initialized.Load() {
		c.stats.totalFiles.Add(-1)
		c.stats.totalBytes.Add(-fileSize)
	}

	return nil
}

// Exists checks if a cache entry exists
func (c *HTMLCache) Exists(domain, path string) bool {
	cachePath := c.getCachePath(domain, path)
	_, err := os.Stat(cachePath)
	return err == nil
}

// Clear clears all cache for a domain (or all if domain is empty)
func (c *HTMLCache) Clear(domain string) (int, error) {
	var count int
	cacheDir := c.getCacheDirSafe()

	if domain != "" {
		// Clear specific domain
		domainDir := filepath.Join(cacheDir, domain)
		metaDir := filepath.Join(cacheDir, "_meta", domain)

		count = c.countFiles(domainDir)
		os.RemoveAll(domainDir)
		os.RemoveAll(metaDir)

		// 清空单个域名后重新扫描以确保准确
		go c.scanAndUpdateStats()
	} else {
		// Clear all
		count = c.countFiles(cacheDir)

		// Remove all subdirectories except _meta
		entries, err := os.ReadDir(cacheDir)
		if err != nil {
			return 0, err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				os.RemoveAll(filepath.Join(cacheDir, entry.Name()))
			}
		}

		// Recreate _meta directory
		os.MkdirAll(filepath.Join(cacheDir, "_meta"), 0755)

		// 清空所有后重置计数器为 0
		c.stats.totalFiles.Store(0)
		c.stats.totalBytes.Store(0)
		c.stats.lastScanAt.Store(time.Now().Unix())
	}

	log.Info().Int("count", count).Str("domain", domain).Msg("Cache cleared")
	return count, nil
}

// countFiles counts HTML files in a directory
func (c *HTMLCache) countFiles(dir string) int {
	var count int
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Ext(path) == ".html" {
			count++
		}
		return nil
	})
	return count
}

// getDirSize returns the total size of a directory
func (c *HTMLCache) getDirSize(dir string) int64 {
	var size int64
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// GetStats returns cache statistics (O(1) from memory counters)
func (c *HTMLCache) GetStats() map[string]interface{} {
	lastScanAt := c.stats.lastScanAt.Load()
	var lastScanTime *time.Time
	if lastScanAt > 0 {
		t := time.Unix(lastScanAt, 0)
		lastScanTime = &t
	}

	return map[string]interface{}{
		"total_entries": c.stats.totalFiles.Load(),
		"total_size_mb": float64(c.stats.totalBytes.Load()) / 1024 / 1024,
		"initialized":   c.stats.initialized.Load(),
		"scanning":      c.stats.scanning.Load(),
		"last_scan_at":  lastScanTime,
	}
}

// ReloadCacheDir 动态重载缓存目录
func (c *HTMLCache) ReloadCacheDir(newDir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 验证并创建新目录
	if err := os.MkdirAll(newDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// 创建 _meta 目录
	metaDir := filepath.Join(newDir, "_meta")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		return fmt.Errorf("failed to create meta directory: %w", err)
	}

	oldDir := c.cacheDir
	c.cacheDir = newDir

	log.Info().
		Str("old_dir", oldDir).
		Str("new_dir", newDir).
		Msg("Cache directory reloaded")

	return nil
}

// GetCacheDir 获取当前缓存目录
func (c *HTMLCache) GetCacheDir() string {
	return c.getCacheDirSafe()
}

// scanAndUpdateStats 扫描目录并更新统计数据
func (c *HTMLCache) scanAndUpdateStats() {
	// 防止并发扫描
	if !c.stats.scanning.CompareAndSwap(false, true) {
		log.Debug().Msg("Cache scan already in progress, skipping")
		return
	}
	defer c.stats.scanning.Store(false)

	startTime := time.Now()
	cacheDir := c.getCacheDirSafe()

	var totalFiles int64
	var totalBytes int64

	// 使用 WalkDir 比 Walk 更快（减少 stat 调用）
	err := filepath.WalkDir(cacheDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 忽略错误，继续扫描
		}
		if d.IsDir() {
			return nil
		}
		// 只统计 .html 文件
		if filepath.Ext(path) == ".html" {
			totalFiles++
			if info, err := d.Info(); err == nil {
				totalBytes += info.Size()
			}
		}
		return nil
	})

	if err != nil {
		log.Error().Err(err).Msg("Failed to scan cache directory")
		return
	}

	// 原子更新统计数据
	c.stats.totalFiles.Store(totalFiles)
	c.stats.totalBytes.Store(totalBytes)
	c.stats.lastScanAt.Store(time.Now().Unix())
	c.stats.initialized.Store(true)

	duration := time.Since(startTime)
	log.Info().
		Int64("files", totalFiles).
		Int64("bytes", totalBytes).
		Dur("duration", duration).
		Msg("Cache directory scan completed")
}

// Recalculate 手动触发重新计算统计数据
func (c *HTMLCache) Recalculate() (map[string]interface{}, error) {
	startTime := time.Now()

	// 同步执行扫描
	c.scanAndUpdateStats()

	duration := time.Since(startTime)

	return map[string]interface{}{
		"total_entries": c.stats.totalFiles.Load(),
		"total_size_mb": float64(c.stats.totalBytes.Load()) / 1024 / 1024,
		"duration_ms":   duration.Milliseconds(),
		"message":       "重新计算完成",
	}, nil
}
