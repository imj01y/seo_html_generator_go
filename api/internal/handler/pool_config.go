package api

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	core "seo-generator/api/internal/service"
)

// PoolConfigHandler 池配置处理器
type PoolConfigHandler struct {
	db               *sqlx.DB
	redis            *redis.Client
	templateAnalyzer *core.TemplateAnalyzer
}

// NewPoolConfigHandler 创建处理器
func NewPoolConfigHandler(db *sqlx.DB, rdb *redis.Client, analyzer *core.TemplateAnalyzer) *PoolConfigHandler {
	return &PoolConfigHandler{
		db:               db,
		redis:            rdb,
		templateAnalyzer: analyzer,
	}
}

// GetConfig 获取当前配置
func (h *PoolConfigHandler) GetConfig(c *gin.Context) {
	// 从数据库读取配置
	var preset, customStr, bufferStr string
	h.db.Get(&preset, "SELECT setting_value FROM system_settings WHERE setting_key = 'pool.concurrency_preset'")
	h.db.Get(&customStr, "SELECT setting_value FROM system_settings WHERE setting_key = 'pool.concurrency_custom'")
	h.db.Get(&bufferStr, "SELECT setting_value FROM system_settings WHERE setting_key = 'pool.buffer_seconds'")

	// 默认值
	if preset == "" {
		preset = "medium"
	}
	custom, _ := strconv.Atoi(customStr)
	if custom == 0 {
		custom = 200
	}
	buffer, _ := strconv.Atoi(bufferStr)
	if buffer == 0 {
		buffer = 10
	}

	// 计算实际并发数
	concurrency := custom
	if preset != "custom" {
		if p, ok := core.GetPoolPreset(preset); ok {
			concurrency = p.Concurrency
		}
	}

	// 获取模板最大统计值
	var maxStats *core.TemplateFuncStats
	if h.templateAnalyzer != nil {
		maxStats = h.templateAnalyzer.GetMaxStats()
	} else {
		maxStats = &core.TemplateFuncStats{}
	}

	// 查找统计值来源模板
	sourceTemplate := h.findSourceTemplate(maxStats)

	// 计算池大小
	presetConfig, ok := core.GetPoolPreset(preset)
	if !ok {
		presetConfig = core.PoolPreset{Concurrency: concurrency}
	}
	sizes := core.CalculatePoolSizes(presetConfig, *maxStats, buffer)

	// 计算内存预估
	memoryBytes := core.EstimateMemoryUsage(sizes)

	core.Success(c, gin.H{
		"config": gin.H{
			"preset":         preset,
			"concurrency":    concurrency,
			"buffer_seconds": buffer,
		},
		"template_stats": gin.H{
			"max_cls":           maxStats.Cls,
			"max_url":           maxStats.RandomURL,
			"max_keyword_emoji": maxStats.KeywordWithEmoji,
			"max_keyword":       maxStats.RandomKeyword,
			"max_image":         maxStats.RandomImage,
			"max_content":       maxStats.RandomContent,
			"source_template":   sourceTemplate,
		},
		"calculated": sizes,
		"memory": gin.H{
			"bytes": memoryBytes,
			"human": core.FormatMemorySize(memoryBytes),
		},
	})
}

// findSourceTemplate 查找统计值最大的模板
func (h *PoolConfigHandler) findSourceTemplate(maxStats *core.TemplateFuncStats) string {
	if h.templateAnalyzer == nil {
		return "unknown"
	}
	analyses := h.templateAnalyzer.GetAllAnalyses()
	for _, a := range analyses {
		if a.Stats.Total() == maxStats.Total() {
			return a.TemplateName
		}
	}
	return "unknown"
}

// UpdateConfig 更新配置
func (h *PoolConfigHandler) UpdateConfig(c *gin.Context) {
	var req struct {
		Preset        string `json:"preset"`
		Concurrency   int    `json:"concurrency"`
		BufferSeconds int    `json:"buffer_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		core.FailWithMessage(c, core.ErrInvalidParam, "参数错误")
		return
	}

	// 验证参数
	if req.BufferSeconds < 5 || req.BufferSeconds > 30 {
		req.BufferSeconds = 10
	}

	// 验证并发数
	concurrency := req.Concurrency
	if req.Preset != "custom" {
		if p, ok := core.GetPoolPreset(req.Preset); ok {
			concurrency = p.Concurrency
		} else {
			core.FailWithMessage(c, core.ErrInvalidParam, "无效的预设")
			return
		}
	} else if concurrency < 10 || concurrency > 10000 {
		core.FailWithMessage(c, core.ErrInvalidParam, "并发数需在 10-10000 之间")
		return
	}

	// 使用事务保存到数据库
	tx, err := h.db.Beginx()
	if err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "开启事务失败")
		return
	}
	defer tx.Rollback()

	if err := h.upsertSetting(tx, "pool.concurrency_preset", req.Preset); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "保存配置失败")
		return
	}
	if err := h.upsertSetting(tx, "pool.concurrency_custom", strconv.Itoa(concurrency)); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "保存配置失败")
		return
	}
	if err := h.upsertSetting(tx, "pool.buffer_seconds", strconv.Itoa(req.BufferSeconds)); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "保存配置失败")
		return
	}

	if err := tx.Commit(); err != nil {
		core.FailWithMessage(c, core.ErrInternalServer, "提交事务失败")
		return
	}

	// 获取模板统计
	var maxStats *core.TemplateFuncStats
	if h.templateAnalyzer != nil {
		maxStats = h.templateAnalyzer.GetMaxStats()
	} else {
		maxStats = &core.TemplateFuncStats{}
	}

	// 计算新的池大小
	presetConfig, ok := core.GetPoolPreset(req.Preset)
	if !ok {
		presetConfig = core.PoolPreset{Concurrency: concurrency}
	}
	sizes := core.CalculatePoolSizes(presetConfig, *maxStats, req.BufferSeconds)

	// 同步池大小到 pool_config 表（高级配置 + 重启后生效）
	if _, err := h.db.ExecContext(c.Request.Context(),
		`UPDATE pool_config SET cls_pool_size = ?, url_pool_size = ?, keyword_emoji_pool_size = ? WHERE id = 1`,
		sizes.ClsPoolSize, sizes.URLPoolSize, sizes.KeywordEmojiPoolSize,
	); err != nil {
		log.Error().Err(err).Msg("Failed to sync pool sizes to pool_config")
	}

	// 发布 Redis 消息通知热更新
	if h.redis != nil {
		reloadMsg := map[string]interface{}{
			"action":         "reload",
			"concurrency":    concurrency,
			"buffer_seconds": req.BufferSeconds,
			"sizes": map[string]int{
				"cls_pool_size":           sizes.ClsPoolSize,
				"url_pool_size":           sizes.URLPoolSize,
				"keyword_emoji_pool_size": sizes.KeywordEmojiPoolSize,
			},
		}
		msgBytes, _ := json.Marshal(reloadMsg)
		if err := h.redis.Publish(c.Request.Context(), "pool:reload", string(msgBytes)).Err(); err != nil {
			log.Error().Err(err).Msg("Failed to publish pool reload message")
			// 不返回错误，因为配置已保存成功
		}
	}

	core.Success(c, gin.H{
		"message":    "配置已更新并生效",
		"calculated": sizes,
	})
}

// upsertSetting 更新或插入设置（使用事务）
func (h *PoolConfigHandler) upsertSetting(tx *sqlx.Tx, key, value string) error {
	var exists int
	if err := tx.Get(&exists, "SELECT COUNT(*) FROM system_settings WHERE setting_key = ?", key); err != nil {
		return err
	}
	if exists > 0 {
		_, err := tx.Exec("UPDATE system_settings SET setting_value = ? WHERE setting_key = ?", value, key)
		return err
	}
	_, err := tx.Exec("INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?)", key, value)
	return err
}
