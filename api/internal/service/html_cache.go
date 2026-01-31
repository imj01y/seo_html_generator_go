package core

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// HTMLCache manages HTML file caching with hash-layered directory structure
type HTMLCache struct {
	cacheDir  string
	maxSizeGB float64
	mu        sync.RWMutex
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

	log.Info().
		Str("dir", cacheDir).
		Float64("max_size_gb", maxSizeGB).
		Msg("HTML cache initialized")

	return &HTMLCache{
		cacheDir:  cacheDir,
		maxSizeGB: maxSizeGB,
	}
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

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return err
	}

	// Write HTML file
	if err := os.WriteFile(cachePath, []byte(html), 0644); err != nil {
		return err
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

	os.Remove(cachePath)
	os.Remove(metaPath)

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

// GetStats returns cache statistics
func (c *HTMLCache) GetStats() map[string]interface{} {
	cacheDir := c.getCacheDirSafe()
	totalSize := c.getDirSize(cacheDir)
	totalEntries := c.countFiles(cacheDir)

	return map[string]interface{}{
		"total_entries": totalEntries,
		"total_size_mb": float64(totalSize) / 1024 / 1024,
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
