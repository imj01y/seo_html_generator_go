# 动态并发配置实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 用户只需调整一个"并发数"参数，系统自动计算并动态调整 Go 对象池和 Python 数据池的大小。

**Architecture:** 统一配置入口 → Redis 消息广播 → Go/Python 双端热更新

**Tech Stack:** Go (Gin, Redis), Python (asyncio, redis), Vue 3, MySQL

**依赖文档:** `docs/plans/2026-01-31-dynamic-concurrency-config-design.md`

---

## Task 1: 数据库迁移 - 添加模板统计字段

**Files:**
- Create: `migrations/004_template_stats.sql`

**Step 1: 创建迁移文件**

```sql
-- 004_template_stats.sql
-- 为 templates 表添加函数调用统计字段

ALTER TABLE templates
  ADD COLUMN cls_count INT DEFAULT 0 COMMENT 'cls() 调用次数',
  ADD COLUMN url_count INT DEFAULT 0 COMMENT 'random_url() 调用次数',
  ADD COLUMN keyword_emoji_count INT DEFAULT 0 COMMENT 'keyword_with_emoji() 调用次数',
  ADD COLUMN keyword_count INT DEFAULT 0 COMMENT 'random_keyword() 调用次数',
  ADD COLUMN image_count INT DEFAULT 0 COMMENT 'random_image() 调用次数',
  ADD COLUMN title_count INT DEFAULT 0 COMMENT 'random_title() 调用次数',
  ADD COLUMN content_count INT DEFAULT 0 COMMENT 'random_content() 调用次数',
  ADD COLUMN analyzed_at DATETIME DEFAULT NULL COMMENT '最后分析时间';

-- 添加池配置相关的系统设置
INSERT INTO system_settings (setting_key, setting_value, setting_type, description) VALUES
  ('pool.concurrency_preset', 'medium', 'string', '并发预设: low/medium/high/extreme/custom'),
  ('pool.concurrency_custom', '200', 'number', '自定义并发数'),
  ('pool.buffer_seconds', '10', 'number', '缓冲秒数 (5-30)')
ON DUPLICATE KEY UPDATE setting_key = setting_key;
```

**Step 2: 执行迁移**

Run: `docker exec -i seo-mysql mysql -uroot -pmysql_6yh7uJ seo_generator < migrations/004_template_stats.sql`

Expected: Query OK

**Step 3: Commit**

```bash
git add migrations/004_template_stats.sql
git commit -m "feat: add template stats columns for pool sizing"
```

---

## Task 2: Go 端 - 池配置预设定义

**Files:**
- Create: `api/internal/service/pool_presets.go`

**Step 1: 创建预设配置文件**

```go
package core

// PoolPreset 池预设配置
type PoolPreset struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Concurrency int    `json:"concurrency"`
}

// 预定义的预设配置
var PoolPresets = []PoolPreset{
	{Key: "low", Name: "低", Description: "适用于小站点、低配服务器", Concurrency: 50},
	{Key: "medium", Name: "中", Description: "适用于中等规模站群", Concurrency: 200},
	{Key: "high", Name: "高", Description: "适用于大规模站群", Concurrency: 500},
	{Key: "extreme", Name: "极高", Description: "适用于高性能服务器", Concurrency: 1000},
}

// GetPoolPreset 根据 key 获取预设
func GetPoolPreset(key string) *PoolPreset {
	for _, p := range PoolPresets {
		if p.Key == key {
			return &p
		}
	}
	return nil
}

// PoolConfig 完整的池配置
type PoolConfig struct {
	Preset        string `json:"preset"`         // 预设 key 或 "custom"
	Concurrency   int    `json:"concurrency"`    // 实际并发数
	BufferSeconds int    `json:"buffer_seconds"` // 缓冲秒数
}

// PoolSizeResult 池大小计算结果
type PoolSizeResult struct {
	// Go 对象池
	ClsPoolSize          int `json:"cls_pool_size"`
	URLPoolSize          int `json:"url_pool_size"`
	KeywordEmojiPoolSize int `json:"keyword_emoji_pool_size"`
	NumberPoolSize       int `json:"number_pool_size"`

	// Python 数据池
	KeywordPoolSize int `json:"keyword_pool_size"`
	ImagePoolSize   int `json:"image_pool_size"`
}

// MemoryEstimate 内存预估
type MemoryEstimate struct {
	ClsPoolMB          float64 `json:"cls_pool_mb"`
	URLPoolMB          float64 `json:"url_pool_mb"`
	KeywordEmojiPoolMB float64 `json:"keyword_emoji_pool_mb"`
	KeywordPoolMB      float64 `json:"keyword_pool_mb"`
	ImagePoolMB        float64 `json:"image_pool_mb"`
	TotalMB            float64 `json:"total_mb"`
}

// 单条数据大小估算（字节）
const (
	AvgClsSize          = 20
	AvgURLSize          = 100
	AvgKeywordEmojiSize = 60
	AvgKeywordSize      = 50
	AvgImageURLSize     = 150
)

// CalculatePoolSizes 根据配置和模板统计计算池大小
func CalculatePoolSizes(config *PoolConfig, maxStats *TemplateFuncStats) *PoolSizeResult {
	multiplier := config.Concurrency * config.BufferSeconds

	return &PoolSizeResult{
		ClsPoolSize:          maxStats.Cls * multiplier,
		URLPoolSize:          maxStats.RandomURL * multiplier,
		KeywordEmojiPoolSize: maxStats.KeywordWithEmoji * multiplier,
		NumberPoolSize:       maxStats.RandomNumber * multiplier,
		KeywordPoolSize:      maxStats.RandomKeyword * multiplier,
		ImagePoolSize:        maxStats.RandomImage * multiplier,
	}
}

// EstimateMemory 估算内存使用量
func EstimateMemory(sizes *PoolSizeResult) *MemoryEstimate {
	const bytesToMB = 1024.0 * 1024.0
	const overhead = 1.2 // 20% 额外开销

	clsMB := float64(sizes.ClsPoolSize*AvgClsSize) * overhead / bytesToMB
	urlMB := float64(sizes.URLPoolSize*AvgURLSize) * overhead / bytesToMB
	keywordEmojiMB := float64(sizes.KeywordEmojiPoolSize*AvgKeywordEmojiSize) * overhead / bytesToMB
	keywordMB := float64(sizes.KeywordPoolSize*AvgKeywordSize) * overhead / bytesToMB
	imageMB := float64(sizes.ImagePoolSize*AvgImageURLSize) * overhead / bytesToMB

	return &MemoryEstimate{
		ClsPoolMB:          clsMB,
		URLPoolMB:          urlMB,
		KeywordEmojiPoolMB: keywordEmojiMB,
		KeywordPoolMB:      keywordMB,
		ImagePoolMB:        imageMB,
		TotalMB:            clsMB + urlMB + keywordEmojiMB + keywordMB + imageMB,
	}
}
```

**Step 2: Commit**

```bash
git add api/internal/service/pool_presets.go
git commit -m "feat: add pool preset configurations and size calculator"
```

---

## Task 3: Go 端 - 池配置 API Handler

**Files:**
- Create: `api/internal/handler/pool_config.go`

**Step 1: 创建 Handler**

```go
package api

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	core "seo-generator/api/internal/service"
)

// PoolConfigHandler 池配置处理器
type PoolConfigHandler struct {
	db               *sqlx.DB
	redis            *redis.Client
	templateAnalyzer *core.TemplateAnalyzer
}

// NewPoolConfigHandler 创建处理器
func NewPoolConfigHandler(db *sqlx.DB, redis *redis.Client, analyzer *core.TemplateAnalyzer) *PoolConfigHandler {
	return &PoolConfigHandler{
		db:               db,
		redis:            redis,
		templateAnalyzer: analyzer,
	}
}

// GetPresets 获取预设列表
func (h *PoolConfigHandler) GetPresets(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"presets": core.PoolPresets,
	})
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
		if p := core.GetPoolPreset(preset); p != nil {
			concurrency = p.Concurrency
		}
	}

	config := &core.PoolConfig{
		Preset:        preset,
		Concurrency:   concurrency,
		BufferSeconds: buffer,
	}

	// 获取模板最大统计值
	maxStats := h.templateAnalyzer.GetMaxStats()

	// 查找统计值来源模板
	sourceTemplate := h.findSourceTemplate(maxStats)

	// 计算池大小
	sizes := core.CalculatePoolSizes(config, maxStats)

	// 计算内存预估
	memory := core.EstimateMemory(sizes)

	c.JSON(200, gin.H{
		"success": true,
		"config":  config,
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
		"memory":     memory,
	})
}

// findSourceTemplate 查找统计值最大的模板
func (h *PoolConfigHandler) findSourceTemplate(maxStats *core.TemplateFuncStats) string {
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
		c.JSON(400, gin.H{"success": false, "message": "参数错误"})
		return
	}

	// 验证参数
	if req.BufferSeconds < 5 || req.BufferSeconds > 30 {
		req.BufferSeconds = 10
	}

	// 验证并发数
	concurrency := req.Concurrency
	if req.Preset != "custom" {
		if p := core.GetPoolPreset(req.Preset); p != nil {
			concurrency = p.Concurrency
		} else {
			c.JSON(400, gin.H{"success": false, "message": "无效的预设"})
			return
		}
	} else if concurrency < 10 || concurrency > 10000 {
		c.JSON(400, gin.H{"success": false, "message": "并发数需在 10-10000 之间"})
		return
	}

	// 保存到数据库
	h.upsertSetting("pool.concurrency_preset", req.Preset)
	h.upsertSetting("pool.concurrency_custom", strconv.Itoa(concurrency))
	h.upsertSetting("pool.buffer_seconds", strconv.Itoa(req.BufferSeconds))

	// 获取模板统计
	maxStats := h.templateAnalyzer.GetMaxStats()

	// 计算新的池大小
	config := &core.PoolConfig{
		Preset:        req.Preset,
		Concurrency:   concurrency,
		BufferSeconds: req.BufferSeconds,
	}
	sizes := core.CalculatePoolSizes(config, maxStats)

	// 发布 Redis 消息通知热更新
	reloadMsg := map[string]interface{}{
		"action":         "reload",
		"concurrency":    concurrency,
		"buffer_seconds": req.BufferSeconds,
		"sizes":          sizes,
	}
	msgBytes, _ := json.Marshal(reloadMsg)
	h.redis.Publish(context.Background(), "pool:reload", string(msgBytes))

	c.JSON(200, gin.H{
		"success":    true,
		"message":    "配置已更新并生效",
		"calculated": sizes,
	})
}

// upsertSetting 更新或插入设置
func (h *PoolConfigHandler) upsertSetting(key, value string) {
	var exists int
	h.db.Get(&exists, "SELECT COUNT(*) FROM system_settings WHERE setting_key = ?", key)
	if exists > 0 {
		h.db.Exec("UPDATE system_settings SET setting_value = ? WHERE setting_key = ?", value, key)
	} else {
		h.db.Exec("INSERT INTO system_settings (setting_key, setting_value) VALUES (?, ?)", key, value)
	}
}
```

**Step 2: Commit**

```bash
git add api/internal/handler/pool_config.go
git commit -m "feat: add pool config API handler"
```

---

## Task 4: Go 端 - 注册路由

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 添加路由注册**

在 `SetupRouter` 函数中添加（在 Settings routes 附近）：

```go
// Pool config routes (require JWT)
poolConfigHandler := NewPoolConfigHandler(deps.DB, deps.Redis, deps.TemplateAnalyzer)
poolConfigGroup := r.Group("/api/pool-config")
poolConfigGroup.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
{
	poolConfigGroup.GET("/presets", poolConfigHandler.GetPresets)
	poolConfigGroup.GET("", poolConfigHandler.GetConfig)
	poolConfigGroup.PUT("", poolConfigHandler.UpdateConfig)
}
```

**Step 2: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat: register pool config API routes"
```

---

## Task 5: Go 端 - 热更新监听器

**Files:**
- Create: `api/internal/service/pool_reloader.go`

**Step 1: 创建热更新监听器**

```go
package core

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// PoolReloader 池配置热更新监听器
type PoolReloader struct {
	redis         *redis.Client
	templateFuncs *TemplateFuncsManager
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewPoolReloader 创建热更新监听器
func NewPoolReloader(redis *redis.Client, templateFuncs *TemplateFuncsManager) *PoolReloader {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolReloader{
		redis:         redis,
		templateFuncs: templateFuncs,
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start 启动监听
func (r *PoolReloader) Start() {
	go r.listen()
	log.Info().Msg("Pool reloader started, listening on pool:reload channel")
}

// Stop 停止监听
func (r *PoolReloader) Stop() {
	r.cancel()
	log.Info().Msg("Pool reloader stopped")
}

// listen 监听 Redis 消息
func (r *PoolReloader) listen() {
	pubsub := r.redis.Subscribe(r.ctx, "pool:reload")
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-r.ctx.Done():
			return
		case msg := <-ch:
			r.handleMessage(msg.Payload)
		}
	}
}

// handleMessage 处理消息
func (r *PoolReloader) handleMessage(payload string) {
	var msg struct {
		Action string          `json:"action"`
		Sizes  *PoolSizeResult `json:"sizes"`
	}
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		log.Error().Err(err).Msg("Failed to parse pool reload message")
		return
	}

	if msg.Action != "reload" || msg.Sizes == nil {
		return
	}

	log.Info().
		Int("cls", msg.Sizes.ClsPoolSize).
		Int("url", msg.Sizes.URLPoolSize).
		Int("keyword_emoji", msg.Sizes.KeywordEmojiPoolSize).
		Msg("Applying pool size configuration")

	// 调整 Go 对象池大小
	if r.templateFuncs != nil {
		r.templateFuncs.ResizePools(&PoolSizeConfig{
			ClsPoolSize:          msg.Sizes.ClsPoolSize,
			URLPoolSize:          msg.Sizes.URLPoolSize,
			KeywordEmojiPoolSize: msg.Sizes.KeywordEmojiPoolSize,
			NumberPoolSize:       msg.Sizes.NumberPoolSize,
		})
	}

	log.Info().Msg("Pool configuration applied successfully")
}
```

**Step 2: Commit**

```bash
git add api/internal/service/pool_reloader.go
git commit -m "feat: add pool config hot reloader"
```

---

## Task 6: Go 端 - 模板保存时异步分析

**Files:**
- Modify: `api/internal/handler/templates.go`

**Step 1: 在 Create/Update 方法中添加异步分析**

在模板保存成功后添加：

```go
// 异步分析模板
go func(templateName string, siteGroupID int, content string) {
	if analyzer := c.MustGet("templateAnalyzer").(*core.TemplateAnalyzer); analyzer != nil {
		analysis := analyzer.AnalyzeTemplate(templateName, siteGroupID, content)
		// 更新数据库中的统计字段
		db.Exec(`
			UPDATE templates SET
				cls_count = ?,
				url_count = ?,
				keyword_emoji_count = ?,
				keyword_count = ?,
				image_count = ?,
				title_count = ?,
				content_count = ?,
				analyzed_at = NOW()
			WHERE name = ? AND site_group_id = ?
		`,
			analysis.Stats.Cls,
			analysis.Stats.RandomURL,
			analysis.Stats.KeywordWithEmoji,
			analysis.Stats.RandomKeyword,
			analysis.Stats.RandomImage,
			analysis.Stats.RandomTitle,
			analysis.Stats.RandomContent,
			templateName,
			siteGroupID,
		)
	}
}(template.Name, template.SiteGroupID, template.Content)
```

**Step 2: Commit**

```bash
git add api/internal/handler/templates.go
git commit -m "feat: add async template analysis on save"
```

---

## Task 7: Python 端 - 热更新监听器

**Files:**
- Create: `worker/core/pool_reloader.py`

**Step 1: 创建热更新监听器**

```python
# -*- coding: utf-8 -*-
"""
池配置热更新监听器

监听 Redis pool:reload 频道，动态调整数据池大小。
"""

import asyncio
import json
from typing import Optional

from loguru import logger

from core.keyword_cache_pool import get_keyword_cache_pool
from core.image_cache_pool import get_image_cache_pool


class PoolReloader:
    """池配置热更新监听器"""

    def __init__(self, redis_client):
        self.redis = redis_client
        self._running = False
        self._task: Optional[asyncio.Task] = None

    async def start(self):
        """启动监听"""
        if self._running:
            return

        self._running = True
        self._task = asyncio.create_task(self._listen())
        logger.info("Pool reloader started, listening on pool:reload channel")

    async def stop(self):
        """停止监听"""
        self._running = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        logger.info("Pool reloader stopped")

    async def _listen(self):
        """监听 Redis 消息"""
        pubsub = self.redis.pubsub()
        await pubsub.subscribe("pool:reload")

        try:
            async for message in pubsub.listen():
                if not self._running:
                    break
                if message["type"] == "message":
                    await self._handle_message(message["data"])
        except asyncio.CancelledError:
            pass
        finally:
            await pubsub.unsubscribe("pool:reload")

    async def _handle_message(self, data):
        """处理消息"""
        try:
            if isinstance(data, bytes):
                data = data.decode('utf-8')
            msg = json.loads(data)

            if msg.get("action") != "reload":
                return

            sizes = msg.get("sizes", {})
            keyword_size = sizes.get("keyword_pool_size", 0)
            image_size = sizes.get("image_pool_size", 0)

            logger.info(f"Applying pool sizes: keyword={keyword_size}, image={image_size}")

            # 调整关键词池
            keyword_pool = get_keyword_cache_pool()
            if keyword_pool and keyword_size > 0:
                await self._resize_keyword_pool(keyword_pool, keyword_size)

            # 调整图片池
            image_pool = get_image_cache_pool()
            if image_pool and image_size > 0:
                await self._resize_image_pool(image_pool, image_size)

            logger.info("Pool configuration applied successfully")

        except Exception as e:
            logger.error(f"Failed to handle pool reload message: {e}")

    async def _resize_keyword_pool(self, pool, new_size: int):
        """调整关键词池大小"""
        pool._cache_size = new_size
        pool._low_watermark = int(new_size * pool._low_watermark_ratio)
        pool._refill_batch_size = min(new_size // 5, 50000)
        logger.info(f"Keyword pool resized: size={new_size}, low_mark={pool._low_watermark}")

    async def _resize_image_pool(self, pool, new_size: int):
        """调整图片池大小"""
        pool._cache_size = new_size
        pool._low_watermark = int(new_size * pool._low_watermark_ratio)
        pool._refill_batch_size = min(new_size // 5, 50000)
        logger.info(f"Image pool resized: size={new_size}, low_mark={pool._low_watermark}")


# 全局实例
_pool_reloader: Optional[PoolReloader] = None


async def start_pool_reloader(redis_client) -> PoolReloader:
    """启动全局池重载器"""
    global _pool_reloader
    _pool_reloader = PoolReloader(redis_client)
    await _pool_reloader.start()
    return _pool_reloader


async def stop_pool_reloader():
    """停止全局池重载器"""
    global _pool_reloader
    if _pool_reloader:
        await _pool_reloader.stop()
        _pool_reloader = None
```

**Step 2: Commit**

```bash
git add worker/core/pool_reloader.py
git commit -m "feat: add Python pool config hot reloader"
```

---

## Task 8: Python 端 - 启动时注册监听器

**Files:**
- Modify: `worker/main.py`

**Step 1: 在启动时添加监听器**

在初始化完成后添加：

```python
from core.pool_reloader import start_pool_reloader, stop_pool_reloader

# 在 startup 中添加
pool_reloader = await start_pool_reloader(redis_client)

# 在 shutdown 中添加
await stop_pool_reloader()
```

**Step 2: Commit**

```bash
git add worker/main.py
git commit -m "feat: register pool reloader on worker startup"
```

---

## Task 9: 前端 - API 封装

**Files:**
- Create: `web/src/api/pool-config.ts`

**Step 1: 创建 API 封装**

```typescript
import request from '@/utils/request'

export interface PoolPreset {
  key: string
  name: string
  description: string
  concurrency: number
}

export interface PoolConfig {
  preset: string
  concurrency: number
  buffer_seconds: number
}

export interface PoolSizes {
  cls_pool_size: number
  url_pool_size: number
  keyword_emoji_pool_size: number
  keyword_pool_size: number
  image_pool_size: number
}

export interface MemoryEstimate {
  cls_pool_mb: number
  url_pool_mb: number
  keyword_emoji_pool_mb: number
  keyword_pool_mb: number
  image_pool_mb: number
  total_mb: number
}

export interface TemplateStats {
  max_cls: number
  max_url: number
  max_keyword_emoji: number
  max_keyword: number
  max_image: number
  max_content: number
  source_template: string
}

export interface PoolConfigResponse {
  success: boolean
  config: PoolConfig
  template_stats: TemplateStats
  calculated: PoolSizes
  memory: MemoryEstimate
}

// 获取预设列表
export function getPresets() {
  return request.get<{ success: boolean; presets: PoolPreset[] }>('/api/pool-config/presets')
}

// 获取当前配置
export function getPoolConfig() {
  return request.get<PoolConfigResponse>('/api/pool-config')
}

// 更新配置
export function updatePoolConfig(data: {
  preset: string
  concurrency?: number
  buffer_seconds: number
}) {
  return request.put<{ success: boolean; message: string; calculated: PoolSizes }>(
    '/api/pool-config',
    data
  )
}
```

**Step 2: Commit**

```bash
git add web/src/api/pool-config.ts
git commit -m "feat: add pool config API client"
```

---

## Task 10: 前端 - 配置页面组件

**Files:**
- Create: `web/src/views/settings/PoolConfig.vue`

**Step 1: 创建配置页面**

```vue
<template>
  <div class="pool-config">
    <el-card>
      <template #header>
        <span>渲染并发配置</span>
      </template>

      <el-form label-width="120px" v-loading="loading">
        <!-- 并发等级选择 -->
        <el-form-item label="并发等级">
          <el-radio-group v-model="form.preset" @change="onPresetChange">
            <el-radio-button
              v-for="preset in presets"
              :key="preset.key"
              :value="preset.key"
            >
              {{ preset.name }} ({{ preset.concurrency }})
            </el-radio-button>
            <el-radio-button value="custom">自定义</el-radio-button>
          </el-radio-group>
        </el-form-item>

        <!-- 自定义并发数 -->
        <el-form-item v-if="form.preset === 'custom'" label="自定义并发">
          <el-input-number
            v-model="form.concurrency"
            :min="10"
            :max="10000"
            :step="50"
            @change="recalculate"
          />
        </el-form-item>

        <!-- 高级选项 -->
        <el-collapse>
          <el-collapse-item title="高级选项" name="advanced">
            <el-form-item label="缓冲时间">
              <el-slider
                v-model="form.buffer_seconds"
                :min="5"
                :max="30"
                :step="1"
                show-input
                @change="recalculate"
              />
              <span class="unit">秒</span>
            </el-form-item>
          </el-collapse-item>
        </el-collapse>

        <!-- 资源预估 -->
        <el-divider>资源预估</el-divider>

        <el-descriptions :column="2" border>
          <el-descriptions-item label="模板基准">
            {{ templateStats?.source_template || '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="单页关键词">
            {{ templateStats?.max_keyword || 0 }} 个
          </el-descriptions-item>
          <el-descriptions-item label="单页图片">
            {{ templateStats?.max_image || 0 }} 个
          </el-descriptions-item>
          <el-descriptions-item label="单页段落">
            {{ templateStats?.max_content || 0 }} 个
          </el-descriptions-item>
        </el-descriptions>

        <el-divider>池大小预估</el-divider>

        <el-descriptions :column="2" border>
          <el-descriptions-item label="关键词池">
            {{ formatNumber(calculated?.keyword_pool_size) }} 条
          </el-descriptions-item>
          <el-descriptions-item label="图片池">
            {{ formatNumber(calculated?.image_pool_size) }} 条
          </el-descriptions-item>
          <el-descriptions-item label="CSS 类名池">
            {{ formatNumber(calculated?.cls_pool_size) }} 条
          </el-descriptions-item>
          <el-descriptions-item label="URL 池">
            {{ formatNumber(calculated?.url_pool_size) }} 条
          </el-descriptions-item>
        </el-descriptions>

        <el-divider>内存预估</el-divider>

        <el-descriptions :column="2" border>
          <el-descriptions-item label="关键词池">
            {{ memory?.keyword_pool_mb?.toFixed(1) }} MB
          </el-descriptions-item>
          <el-descriptions-item label="图片池">
            {{ memory?.image_pool_mb?.toFixed(1) }} MB
          </el-descriptions-item>
          <el-descriptions-item label="Go 对象池">
            {{ goPoolMemory.toFixed(1) }} MB
          </el-descriptions-item>
          <el-descriptions-item label="总计">
            <el-tag :type="memoryTagType">
              {{ memory?.total_mb?.toFixed(1) }} MB
            </el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <!-- 操作按钮 -->
        <el-form-item style="margin-top: 20px">
          <el-button type="primary" @click="applyConfig" :loading="applying">
            应用配置
          </el-button>
          <el-button @click="resetForm">取消</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getPresets,
  getPoolConfig,
  updatePoolConfig,
  type PoolPreset,
  type PoolSizes,
  type MemoryEstimate,
  type TemplateStats
} from '@/api/pool-config'

const loading = ref(false)
const applying = ref(false)
const presets = ref<PoolPreset[]>([])
const templateStats = ref<TemplateStats | null>(null)
const calculated = ref<PoolSizes | null>(null)
const memory = ref<MemoryEstimate | null>(null)

const form = ref({
  preset: 'medium',
  concurrency: 200,
  buffer_seconds: 10
})

const originalForm = ref({ ...form.value })

const goPoolMemory = computed(() => {
  if (!memory.value) return 0
  return (
    memory.value.cls_pool_mb +
    memory.value.url_pool_mb +
    memory.value.keyword_emoji_pool_mb
  )
})

const memoryTagType = computed(() => {
  const total = memory.value?.total_mb || 0
  if (total > 500) return 'danger'
  if (total > 200) return 'warning'
  return 'success'
})

function formatNumber(num: number | undefined): string {
  if (!num) return '0'
  return num.toLocaleString()
}

function onPresetChange(preset: string) {
  if (preset !== 'custom') {
    const p = presets.value.find((x) => x.key === preset)
    if (p) {
      form.value.concurrency = p.concurrency
    }
  }
  recalculate()
}

async function recalculate() {
  // 本地计算预估值（简化版）
  // 实际可以调用后端 API 获取精确值
}

async function loadConfig() {
  loading.value = true
  try {
    const [presetsRes, configRes] = await Promise.all([
      getPresets(),
      getPoolConfig()
    ])

    presets.value = presetsRes.data.presets
    form.value = {
      preset: configRes.data.config.preset,
      concurrency: configRes.data.config.concurrency,
      buffer_seconds: configRes.data.config.buffer_seconds
    }
    originalForm.value = { ...form.value }
    templateStats.value = configRes.data.template_stats
    calculated.value = configRes.data.calculated
    memory.value = configRes.data.memory
  } catch (e) {
    ElMessage.error('加载配置失败')
  } finally {
    loading.value = false
  }
}

async function applyConfig() {
  applying.value = true
  try {
    const res = await updatePoolConfig({
      preset: form.value.preset,
      concurrency: form.value.concurrency,
      buffer_seconds: form.value.buffer_seconds
    })
    if (res.data.success) {
      ElMessage.success('配置已应用')
      calculated.value = res.data.calculated
      originalForm.value = { ...form.value }
    }
  } catch (e) {
    ElMessage.error('应用配置失败')
  } finally {
    applying.value = false
  }
}

function resetForm() {
  form.value = { ...originalForm.value }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped>
.pool-config {
  padding: 20px;
}
.unit {
  margin-left: 10px;
  color: #999;
}
</style>
```

**Step 2: Commit**

```bash
git add web/src/views/settings/PoolConfig.vue
git commit -m "feat: add pool config settings page"
```

---

## Task 11: 前端 - 添加路由

**Files:**
- Modify: `web/src/router/index.ts`

**Step 1: 添加路由**

在 settings 相关路由中添加：

```typescript
{
  path: '/settings/pool-config',
  name: 'PoolConfig',
  component: () => import('@/views/settings/PoolConfig.vue'),
  meta: { title: '并发配置', requiresAuth: true }
}
```

**Step 2: Commit**

```bash
git add web/src/router/index.ts
git commit -m "feat: add pool config route"
```

---

## Task 12: 前端 - 添加菜单项

**Files:**
- Modify: `web/src/components/Layout/MainLayout.vue`

**Step 1: 在设置菜单中添加**

找到设置相关菜单，添加：

```vue
<el-menu-item index="/settings/pool-config">
  <el-icon><Setting /></el-icon>
  <span>并发配置</span>
</el-menu-item>
```

**Step 2: Commit**

```bash
git add web/src/components/Layout/MainLayout.vue
git commit -m "feat: add pool config menu item"
```

---

## 完成检查清单

- [ ] Task 1: 数据库迁移
- [ ] Task 2: Go 预设配置
- [ ] Task 3: Go API Handler
- [ ] Task 4: Go 路由注册
- [ ] Task 5: Go 热更新监听
- [ ] Task 6: Go 模板异步分析
- [ ] Task 7: Python 热更新监听
- [ ] Task 8: Python 启动注册
- [ ] Task 9: 前端 API 封装
- [ ] Task 10: 前端配置页面
- [ ] Task 11: 前端路由
- [ ] Task 12: 前端菜单

---

## 测试验证

完成所有任务后：

1. 执行数据库迁移
2. 重启 API 和 Worker 服务
3. 访问管理后台 → 设置 → 并发配置
4. 选择不同预设，验证内存预估更新
5. 点击应用配置，验证热更新生效
6. 检查日志确认 Go 和 Python 端都收到消息
