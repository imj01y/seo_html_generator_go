package api

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// SystemSetting 系统设置
type SystemSetting struct {
	ID           int     `db:"id" json:"id"`
	SettingKey   string  `db:"setting_key" json:"setting_key"`
	SettingValue string  `db:"setting_value" json:"setting_value"`
	SettingType  string  `db:"setting_type" json:"setting_type"`
	Description  *string `db:"description" json:"description"`
}

// SettingsHandler 系统设置处理器
type SettingsHandler struct{}

// 缓存默认设置
var cacheDefaultSettings = map[string]struct {
	Value       string
	Type        string
	Description string
}{
	"keyword_cache_ttl":      {"86400", "number", "关键词缓存过期时间(秒)"},
	"image_cache_ttl":        {"86400", "number", "图片URL缓存过期时间(秒)"},
	"cache_compress_enabled": {"true", "boolean", "是否启用缓存压缩"},
	"cache_compress_level":   {"6", "number", "压缩级别(1-9)"},
	"keyword_pool_size":      {"500000", "number", "关键词池大小(0=不限制)"},
	"image_pool_size":        {"500000", "number", "图片池大小(0=不限制)"},
	"article_pool_size":      {"50000", "number", "文章池大小(0=不限制)"},
	"file_cache_enabled":     {"false", "boolean", "是否启用文件缓存"},
	"file_cache_dir":         {"./html_cache", "string", "文件缓存目录"},
	"file_cache_max_size_gb": {"50", "number", "最大缓存大小(GB)"},
	"file_cache_nginx_mode":  {"true", "boolean", "Nginx直服模式(不压缩)"},
}

// convertSettingValue 根据类型转换设置值
func convertSettingValue(value, stype string) interface{} {
	switch stype {
	case "number":
		if f, err := strconv.ParseFloat(value, 64); err == nil {
			if f == float64(int64(f)) {
				return int64(f)
			}
			return f
		}
		return value
	case "boolean":
		return value == "true" || value == "1" || value == "yes"
	default:
		return value
	}
}

// Get 获取系统配置
func (h *SettingsHandler) Get(c *gin.Context) {
	cfg, exists := c.Get("config")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "配置未加载"})
		return
	}

	// 从 config 中提取需要的配置
	// 这里简化处理，返回基本信息
	c.JSON(200, gin.H{
		"success": true,
		"data":    cfg,
	})
}

// GetCacheSettings 获取缓存配置
func (h *SettingsHandler) GetCacheSettings(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var settings []SystemSetting
	sqlxDB.Select(&settings, "SELECT id, setting_key, setting_value, setting_type, description FROM system_settings")

	result := make(map[string]gin.H)
	existingKeys := make(map[string]bool)

	for _, s := range settings {
		existingKeys[s.SettingKey] = true
		result[s.SettingKey] = gin.H{
			"value":       convertSettingValue(s.SettingValue, s.SettingType),
			"type":        s.SettingType,
			"description": s.Description,
		}
	}

	// 插入缺失的默认设置
	for key, def := range cacheDefaultSettings {
		if existingKeys[key] {
			continue
		}
		sqlxDB.Exec(`
			INSERT INTO system_settings (setting_key, setting_value, setting_type, description)
			VALUES (?, ?, ?, ?)
		`, key, def.Value, def.Type, def.Description)

		result[key] = gin.H{
			"value":       convertSettingValue(def.Value, def.Type),
			"type":        def.Type,
			"description": def.Description,
		}
	}

	c.JSON(200, gin.H{"success": true, "settings": result})
}

// UpdateCacheSettings 更新缓存配置
func (h *SettingsHandler) UpdateCacheSettings(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var data map[string]interface{}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	updated := 0
	for key, value := range data {
		valueStr := ""
		stype := "string"

		switch v := value.(type) {
		case bool:
			stype = "boolean"
			if v {
				valueStr = "true"
			} else {
				valueStr = "false"
			}
		case float64:
			stype = "number"
			valueStr = strconv.FormatFloat(v, 'f', -1, 64)
		case string:
			valueStr = v
		default:
			valueStr = ""
		}

		var existsCount int
		sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM system_settings WHERE setting_key = ?", key)

		if existsCount > 0 {
			sqlxDB.Exec("UPDATE system_settings SET setting_value = ? WHERE setting_key = ?", valueStr, key)
		} else {
			sqlxDB.Exec(`
				INSERT INTO system_settings (setting_key, setting_value, setting_type)
				VALUES (?, ?, ?)
			`, key, valueStr, stype)
		}
		updated++
	}

	c.JSON(200, gin.H{"success": true, "updated": updated})
}

// ApplyCacheSettings 应用缓存配置到运行时
func (h *SettingsHandler) ApplyCacheSettings(c *gin.Context) {
	// 这个功能需要与实际的缓存管理器配合
	// 目前返回提示信息
	c.JSON(200, gin.H{
		"success": true,
		"message": "配置已标记待应用，部分配置需要重启服务生效",
		"applied": []string{},
	})
}

// GetDatabaseStatus 获取数据库连接状态
func (h *SettingsHandler) GetDatabaseStatus(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"connected": false, "error": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	stats := sqlxDB.Stats()
	c.JSON(200, gin.H{
		"connected":        true,
		"max_open":         stats.MaxOpenConnections,
		"open":             stats.OpenConnections,
		"in_use":           stats.InUse,
		"idle":             stats.Idle,
		"wait_count":       stats.WaitCount,
		"wait_duration_ms": stats.WaitDuration.Milliseconds(),
	})
}

// GetAPIToken 获取 API Token 设置
func (h *SettingsHandler) GetAPIToken(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var token, enabled string
	sqlxDB.Get(&token, "SELECT setting_value FROM system_settings WHERE setting_key = 'api_token'")
	sqlxDB.Get(&enabled, "SELECT setting_value FROM system_settings WHERE setting_key = 'api_token_enabled'")

	if enabled == "" {
		enabled = "true"
	}

	c.JSON(200, gin.H{
		"success": true,
		"token":   token,
		"enabled": enabled == "true" || enabled == "1",
	})
}

// UpdateAPIToken 更新 API Token 设置
func (h *SettingsHandler) UpdateAPIToken(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "数据库未连接"})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	var data struct {
		Token   string `json:"token"`
		Enabled *bool  `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	if data.Token != "" {
		var existsCount int
		sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM system_settings WHERE setting_key = 'api_token'")
		if existsCount > 0 {
			sqlxDB.Exec("UPDATE system_settings SET setting_value = ? WHERE setting_key = 'api_token'", data.Token)
		} else {
			sqlxDB.Exec(`
				INSERT INTO system_settings (setting_key, setting_value, description)
				VALUES ('api_token', ?, 'API Token for external access')
			`, data.Token)
		}
	}

	if data.Enabled != nil {
		enabledStr := "false"
		if *data.Enabled {
			enabledStr = "true"
		}

		var existsCount int
		sqlxDB.Get(&existsCount, "SELECT COUNT(*) FROM system_settings WHERE setting_key = 'api_token_enabled'")
		if existsCount > 0 {
			sqlxDB.Exec("UPDATE system_settings SET setting_value = ? WHERE setting_key = 'api_token_enabled'", enabledStr)
		} else {
			sqlxDB.Exec(`
				INSERT INTO system_settings (setting_key, setting_value, description)
				VALUES ('api_token_enabled', ?, 'Enable API Token authentication')
			`, enabledStr)
		}
	}

	c.JSON(200, gin.H{"success": true, "message": "API Token 设置已更新"})
}

// GenerateAPIToken 生成新的随机 API Token
func (h *SettingsHandler) GenerateAPIToken(c *gin.Context) {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	token := "seo_" + hex.EncodeToString(bytes)

	c.JSON(200, gin.H{"success": true, "token": token})
}
