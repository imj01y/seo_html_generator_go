# 蜘蛛日志趋势图表功能增强 - 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为蜘蛛日志页面添加分钟、小时、天、月四种时间粒度的趋势图表支持

**Architecture:** 使用预聚合表存储统计数据，通过归档服务定时聚合，API 接口支持粒度回退机制

**Tech Stack:** Go + Gin + MySQL + Vue 3 + TypeScript + ECharts

---

## Task 1: 数据库迁移

**Files:**
- Create: `migrations/003_spider_logs_stats.sql`

**Step 1: 创建迁移文件**

```sql
-- migrations/003_spider_logs_stats.sql
-- 蜘蛛日志统计预聚合表

CREATE TABLE IF NOT EXISTS spider_logs_stats (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    period_type ENUM('minute', 'hour', 'day', 'month') NOT NULL COMMENT '周期类型',
    period_start DATETIME NOT NULL COMMENT '周期开始时间',
    spider_type VARCHAR(20) DEFAULT NULL COMMENT '蜘蛛类型，NULL表示全部汇总',
    total INT UNSIGNED DEFAULT 0 COMMENT '访问次数',
    status_2xx INT UNSIGNED DEFAULT 0 COMMENT '2xx响应数',
    status_3xx INT UNSIGNED DEFAULT 0 COMMENT '3xx响应数',
    status_4xx INT UNSIGNED DEFAULT 0 COMMENT '4xx响应数',
    status_5xx INT UNSIGNED DEFAULT 0 COMMENT '5xx响应数',
    avg_resp_time INT UNSIGNED DEFAULT 0 COMMENT '平均响应时间(ms)',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uk_period_spider (period_type, period_start, spider_type),
    INDEX idx_query (period_type, period_start DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='蜘蛛日志统计表';
```

**Step 2: 执行迁移**

Run: `mysql -u root -p seo_generator < migrations/003_spider_logs_stats.sql`
Expected: Query OK

**Step 3: 验证表创建成功**

Run: `mysql -u root -p -e "DESCRIBE spider_logs_stats" seo_generator`
Expected: 显示表结构，包含 period_type, period_start, spider_type, total 等字段

**Step 4: Commit**

```bash
git add migrations/003_spider_logs_stats.sql
git commit -m "feat(db): 添加蜘蛛日志统计预聚合表"
```

---

## Task 2: 后端数据模型

**Files:**
- Create: `api/internal/model/spider_logs_stats.go`

**Step 1: 创建模型文件**

```go
// api/internal/model/spider_logs_stats.go
package models

import "time"

// SpiderLogsStatsPoint 蜘蛛日志统计数据点
type SpiderLogsStatsPoint struct {
	Time        time.Time `db:"time" json:"time"`
	Total       int       `db:"total" json:"total"`
	Status2xx   int       `db:"status_2xx" json:"status_2xx"`
	Status3xx   int       `db:"status_3xx" json:"status_3xx"`
	Status4xx   int       `db:"status_4xx" json:"status_4xx"`
	Status5xx   int       `db:"status_5xx" json:"status_5xx"`
	AvgRespTime int       `db:"avg_resp_time" json:"avg_resp_time"`
}

// SpiderLogsTrendResponse 趋势接口响应
type SpiderLogsTrendResponse struct {
	Period string                 `json:"period"`
	Items  []SpiderLogsStatsPoint `json:"items"`
}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add api/internal/model/spider_logs_stats.go
git commit -m "feat(model): 添加蜘蛛日志统计数据模型"
```

---

## Task 3: 归档服务 - 基础结构

**Files:**
- Create: `api/internal/service/spider_logs_archiver.go`

**Step 1: 创建归档服务基础结构**

```go
// api/internal/service/spider_logs_archiver.go
package core

import (
	"context"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// SpiderLogsArchiver 蜘蛛日志归档服务
// 定时将 spider_logs 原始数据聚合到 spider_logs_stats 表
type SpiderLogsArchiver struct {
	db *sqlx.DB

	mu            sync.Mutex
	running       bool
	stopCh        chan struct{}
	lastMinuteRun time.Time
	lastHourRun   time.Time
	lastDayRun    time.Time
	lastMonthRun  time.Time
}

// NewSpiderLogsArchiver 创建蜘蛛日志归档服务
func NewSpiderLogsArchiver(db *sqlx.DB) *SpiderLogsArchiver {
	return &SpiderLogsArchiver{
		db:     db,
		stopCh: make(chan struct{}),
	}
}

// Start 启动归档服务
func (a *SpiderLogsArchiver) Start(ctx context.Context) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	a.stopCh = make(chan struct{})
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	log.Info().Msg("SpiderLogsArchiver started")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("SpiderLogsArchiver stopped (context cancelled)")
			return
		case <-a.stopCh:
			log.Info().Msg("SpiderLogsArchiver stopped")
			return
		case now := <-ticker.C:
			a.runTasks(ctx, now)
		}
	}
}

// Stop 停止归档服务
func (a *SpiderLogsArchiver) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running && a.stopCh != nil {
		close(a.stopCh)
	}
}

func (a *SpiderLogsArchiver) runTasks(ctx context.Context, now time.Time) {
	// 每分钟：归档分钟统计
	if now.Sub(a.lastMinuteRun) >= time.Minute {
		if err := a.archiveMinuteStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("SpiderLogsArchiver: archiveMinuteStats error")
		}
		a.lastMinuteRun = now
	}

	// 每小时整点：聚合小时统计
	if now.Minute() == 0 && now.Sub(a.lastHourRun) >= time.Hour {
		if err := a.aggregateHourStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("SpiderLogsArchiver: aggregateHourStats error")
		}
		// 清理 7 天前的分钟数据
		a.cleanupOldData(ctx, "minute", 7)
		a.lastHourRun = now
	}

	// 每天凌晨：聚合天统计
	if now.Hour() == 0 && now.Minute() < 10 && now.Sub(a.lastDayRun) >= 24*time.Hour {
		if err := a.aggregateDayStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("SpiderLogsArchiver: aggregateDayStats error")
		}
		// 清理 30 天前的小时数据
		a.cleanupOldData(ctx, "hour", 30)
		a.lastDayRun = now
	}

	// 每月1日凌晨：聚合月统计
	if now.Day() == 1 && now.Hour() == 0 && now.Minute() < 15 && now.Sub(a.lastMonthRun) >= 24*time.Hour {
		if err := a.aggregateMonthStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("SpiderLogsArchiver: aggregateMonthStats error")
		}
		a.lastMonthRun = now
	}
}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 编译错误（缺少方法实现）

---

## Task 4: 归档服务 - 分钟聚合

**Files:**
- Modify: `api/internal/service/spider_logs_archiver.go`

**Step 1: 添加分钟聚合方法**

在文件末尾添加：

```go
// archiveMinuteStats 归档分钟统计（从原始日志聚合）
func (a *SpiderLogsArchiver) archiveMinuteStats(ctx context.Context, now time.Time) error {
	// 聚合上一分钟的数据
	periodStart := now.Truncate(time.Minute).Add(-time.Minute)
	periodEnd := periodStart.Add(time.Minute)

	// 按蜘蛛类型分组聚合
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO spider_logs_stats
			(period_type, period_start, spider_type, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time)
		SELECT
			'minute',
			?,
			spider_type,
			COUNT(*),
			SUM(CASE WHEN status >= 200 AND status < 300 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status >= 300 AND status < 400 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status >= 400 AND status < 500 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),
			COALESCE(AVG(resp_time), 0)
		FROM spider_logs
		WHERE created_at >= ? AND created_at < ?
		GROUP BY spider_type
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			status_2xx = VALUES(status_2xx),
			status_3xx = VALUES(status_3xx),
			status_4xx = VALUES(status_4xx),
			status_5xx = VALUES(status_5xx),
			avg_resp_time = VALUES(avg_resp_time)
	`, periodStart, periodStart, periodEnd)

	if err != nil {
		return err
	}

	// 聚合全部蜘蛛的汇总（spider_type = NULL）
	_, err = a.db.ExecContext(ctx, `
		INSERT INTO spider_logs_stats
			(period_type, period_start, spider_type, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time)
		SELECT
			'minute',
			?,
			NULL,
			COUNT(*),
			SUM(CASE WHEN status >= 200 AND status < 300 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status >= 300 AND status < 400 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status >= 400 AND status < 500 THEN 1 ELSE 0 END),
			SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),
			COALESCE(AVG(resp_time), 0)
		FROM spider_logs
		WHERE created_at >= ? AND created_at < ?
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			status_2xx = VALUES(status_2xx),
			status_3xx = VALUES(status_3xx),
			status_4xx = VALUES(status_4xx),
			status_5xx = VALUES(status_5xx),
			avg_resp_time = VALUES(avg_resp_time)
	`, periodStart, periodStart, periodEnd)

	return err
}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 编译错误（仍缺少其他方法）

---

## Task 5: 归档服务 - 小时/天/月聚合

**Files:**
- Modify: `api/internal/service/spider_logs_archiver.go`

**Step 1: 添加小时聚合方法**

```go
// aggregateHourStats 聚合小时统计（从分钟数据聚合）
func (a *SpiderLogsArchiver) aggregateHourStats(ctx context.Context, now time.Time) error {
	hourStart := now.Truncate(time.Hour).Add(-time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO spider_logs_stats
			(period_type, period_start, spider_type, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time)
		SELECT
			'hour',
			?,
			spider_type,
			SUM(total),
			SUM(status_2xx),
			SUM(status_3xx),
			SUM(status_4xx),
			SUM(status_5xx),
			COALESCE(AVG(avg_resp_time), 0)
		FROM spider_logs_stats
		WHERE period_type = 'minute' AND period_start >= ? AND period_start < ?
		GROUP BY spider_type
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			status_2xx = VALUES(status_2xx),
			status_3xx = VALUES(status_3xx),
			status_4xx = VALUES(status_4xx),
			status_5xx = VALUES(status_5xx),
			avg_resp_time = VALUES(avg_resp_time)
	`, hourStart, hourStart, hourEnd)

	return err
}
```

**Step 2: 添加天聚合方法**

```go
// aggregateDayStats 聚合天统计（从小时数据聚合）
func (a *SpiderLogsArchiver) aggregateDayStats(ctx context.Context, now time.Time) error {
	dayStart := now.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO spider_logs_stats
			(period_type, period_start, spider_type, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time)
		SELECT
			'day',
			?,
			spider_type,
			SUM(total),
			SUM(status_2xx),
			SUM(status_3xx),
			SUM(status_4xx),
			SUM(status_5xx),
			COALESCE(AVG(avg_resp_time), 0)
		FROM spider_logs_stats
		WHERE period_type = 'hour' AND period_start >= ? AND period_start < ?
		GROUP BY spider_type
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			status_2xx = VALUES(status_2xx),
			status_3xx = VALUES(status_3xx),
			status_4xx = VALUES(status_4xx),
			status_5xx = VALUES(status_5xx),
			avg_resp_time = VALUES(avg_resp_time)
	`, dayStart, dayStart, dayEnd)

	return err
}
```

**Step 3: 添加月聚合方法**

```go
// aggregateMonthStats 聚合月统计（从天数据聚合）
func (a *SpiderLogsArchiver) aggregateMonthStats(ctx context.Context, now time.Time) error {
	// 上个月的第一天
	firstOfThisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	firstOfLastMonth := firstOfThisMonth.AddDate(0, -1, 0)
	lastOfLastMonth := firstOfThisMonth.Add(-time.Second)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO spider_logs_stats
			(period_type, period_start, spider_type, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time)
		SELECT
			'month',
			?,
			spider_type,
			SUM(total),
			SUM(status_2xx),
			SUM(status_3xx),
			SUM(status_4xx),
			SUM(status_5xx),
			COALESCE(AVG(avg_resp_time), 0)
		FROM spider_logs_stats
		WHERE period_type = 'day' AND period_start >= ? AND period_start <= ?
		GROUP BY spider_type
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			status_2xx = VALUES(status_2xx),
			status_3xx = VALUES(status_3xx),
			status_4xx = VALUES(status_4xx),
			status_5xx = VALUES(status_5xx),
			avg_resp_time = VALUES(avg_resp_time)
	`, firstOfLastMonth, firstOfLastMonth, lastOfLastMonth)

	return err
}
```

**Step 4: 添加清理方法**

```go
// cleanupOldData 清理过期数据
func (a *SpiderLogsArchiver) cleanupOldData(ctx context.Context, periodType string, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result, err := a.db.ExecContext(ctx, `
		DELETE FROM spider_logs_stats WHERE period_type = ? AND period_start < ?
	`, periodType, cutoff)

	if err != nil {
		log.Error().Err(err).Str("period_type", periodType).Msg("SpiderLogsArchiver: cleanupOldData error")
		return
	}

	if affected, _ := result.RowsAffected(); affected > 0 {
		log.Info().Int64("count", affected).Str("period_type", periodType).Msg("SpiderLogsArchiver: cleaned up old stats")
	}
}
```

**Step 5: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 6: Commit**

```bash
git add api/internal/service/spider_logs_archiver.go
git commit -m "feat(service): 添加蜘蛛日志归档服务"
```

---

## Task 6: API 接口实现

**Files:**
- Modify: `api/internal/handler/spider_detector.go`

**Step 1: 添加 GetSpiderTrend 方法**

在 `spider_detector.go` 文件末尾添加：

```go
// GetSpiderTrend 获取蜘蛛访问趋势
// GET /api/spiders/trend
func (h *SpiderDetectorHandler) GetSpiderTrend(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		core.Success(c, models.SpiderLogsTrendResponse{Period: "hour", Items: []models.SpiderLogsStatsPoint{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	period := c.DefaultQuery("period", "hour")
	spiderType := c.Query("spider_type")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	if limit < 1 || limit > 500 {
		limit = 100
	}

	// 验证 period 参数
	validPeriods := map[string]bool{"minute": true, "hour": true, "day": true, "month": true}
	if !validPeriods[period] {
		period = "hour"
	}

	// 周期回退顺序
	periodFallback := map[string]string{
		"month": "day",
		"day":   "hour",
		"hour":  "minute",
	}

	// 尝试查询，如果没有数据则回退到更细粒度
	for {
		where := "period_type = ?"
		args := []interface{}{period}

		if spiderType != "" {
			where += " AND spider_type = ?"
			args = append(args, spiderType)
		} else {
			where += " AND spider_type IS NULL"
		}

		args = append(args, limit)

		var data []models.SpiderLogsStatsPoint
		err := sqlxDB.Select(&data, `
			SELECT period_start as time, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time
			FROM spider_logs_stats
			WHERE `+where+`
			ORDER BY period_start DESC
			LIMIT ?
		`, args...)

		if err == nil && len(data) > 0 {
			// 反转为时间正序
			for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
				data[i], data[j] = data[j], data[i]
			}
			core.Success(c, models.SpiderLogsTrendResponse{Period: period, Items: data})
			return
		}

		// 回退到更细粒度
		fallback, ok := periodFallback[period]
		if !ok {
			// 已经是最细粒度，返回空
			core.Success(c, models.SpiderLogsTrendResponse{Period: period, Items: []models.SpiderLogsStatsPoint{}})
			return
		}
		period = fallback
	}
}
```

**Step 2: 添加 import**

确保文件顶部有以下 import：

```go
import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"

	models "seo-generator/api/internal/model"
	core "seo-generator/api/internal/service"
)
```

**Step 3: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 4: Commit**

```bash
git add api/internal/handler/spider_detector.go
git commit -m "feat(api): 添加蜘蛛日志趋势接口"
```

---

## Task 7: 注册路由

**Files:**
- Modify: `api/internal/handler/router.go:313-323`

**Step 1: 添加新路由**

找到 `spiderDetectorRoutes` 路由组，在 `spiderDetectorRoutes.DELETE("/logs/clear", ...)` 之后添加：

```go
		spiderDetectorRoutes.GET("/trend", spiderDetectorHandler.GetSpiderTrend)
```

完整的路由组应该是：

```go
	// Spider Detector routes (require JWT)
	spiderDetectorHandler := &SpiderDetectorHandler{}
	spiderDetectorRoutes := r.Group("/api/spiders")
	spiderDetectorRoutes.Use(AuthMiddleware(deps.Config.Auth.SecretKey))
	{
		spiderDetectorRoutes.GET("/config", spiderDetectorHandler.GetSpiderConfig)
		spiderDetectorRoutes.POST("/test", spiderDetectorHandler.TestSpiderDetection)
		spiderDetectorRoutes.GET("/logs", spiderDetectorHandler.GetSpiderLogs)
		spiderDetectorRoutes.GET("/stats", spiderDetectorHandler.GetSpiderStats)
		spiderDetectorRoutes.GET("/daily-stats", spiderDetectorHandler.GetSpiderDailyStats)
		spiderDetectorRoutes.GET("/hourly-stats", spiderDetectorHandler.GetSpiderHourlyStats)
		spiderDetectorRoutes.DELETE("/logs/clear", spiderDetectorHandler.ClearSpiderLogs)
		spiderDetectorRoutes.GET("/trend", spiderDetectorHandler.GetSpiderTrend)
	}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "feat(router): 注册蜘蛛日志趋势路由"
```

---

## Task 8: 启动归档服务

**Files:**
- Modify: `api/main.go`

**Step 1: 在 main.go 中初始化并启动归档服务**

找到 `// Initialize and start StatsArchiver` 部分（约第 270 行），在其后添加：

```go
	// Initialize and start SpiderLogsArchiver
	spiderLogsArchiver := core.NewSpiderLogsArchiver(db)
	spiderLogsArchiverCtx, spiderLogsArchiverCancel := context.WithCancel(context.Background())
	go spiderLogsArchiver.Start(spiderLogsArchiverCtx)
	defer spiderLogsArchiverCancel()
	log.Info().Msg("SpiderLogsArchiver initialized and started")
```

**Step 2: 在 shutdown 部分停止归档服务**

找到 `// Stop StatsArchiver` 部分（约第 310 行），在其后添加：

```go
	// Stop SpiderLogsArchiver
	spiderLogsArchiver.Stop()
	log.Info().Msg("SpiderLogsArchiver stopped")
```

**Step 3: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 4: Commit**

```bash
git add api/main.go
git commit -m "feat(main): 启动蜘蛛日志归档服务"
```

---

## Task 9: 前端 API 函数

**Files:**
- Modify: `web/src/api/spiders.ts`

**Step 1: 添加类型定义和 API 函数**

在文件末尾添加：

```typescript
// ============================================
// 蜘蛛日志趋势 API
// ============================================

export interface SpiderTrendPoint {
  time: string
  total: number
  status_2xx: number
  status_3xx: number
  status_4xx: number
  status_5xx: number
  avg_resp_time: number
}

export interface SpiderTrendResponse {
  period: string
  items: SpiderTrendPoint[]
}

export async function getSpiderTrend(params?: {
  period?: 'minute' | 'hour' | 'day' | 'month'
  spider_type?: string
  limit?: number
}): Promise<SpiderTrendResponse> {
  const res: SpiderTrendResponse = await request.get('/spiders/trend', { params })
  return res || { period: params?.period || 'hour', items: [] }
}
```

**Step 2: 验证 TypeScript 编译**

Run: `cd web && npm run type-check` 或 `cd web && npx tsc --noEmit`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add web/src/api/spiders.ts
git commit -m "feat(web): 添加蜘蛛日志趋势 API 函数"
```

---

## Task 10: 前端页面改造 - 模板部分

**Files:**
- Modify: `web/src/views/spiders/SpiderLogs.vue:35-38`

**Step 1: 修改图表选择器**

将原来的代码：

```vue
<el-radio-group v-model="chartType" size="small" @change="loadChart">
  <el-radio-button label="daily">按天</el-radio-button>
  <el-radio-button label="hourly">按小时</el-radio-button>
</el-radio-group>
```

改为：

```vue
<el-radio-group v-model="periodType" size="small" @change="loadChart">
  <el-radio-button label="minute">分钟</el-radio-button>
  <el-radio-button label="hour">小时</el-radio-button>
  <el-radio-button label="day">天</el-radio-button>
  <el-radio-button label="month">月</el-radio-button>
</el-radio-group>
```

**Step 2: 验证保存成功**

Run: `cd web && npm run type-check`
Expected: 可能有 TypeScript 错误（变量名不匹配），下一步修复

---

## Task 11: 前端页面改造 - 脚本部分

**Files:**
- Modify: `web/src/views/spiders/SpiderLogs.vue:154-280`

**Step 1: 修改 import**

将：

```typescript
import {
  getSpiderLogs,
  getSpiderStats,
  getDailyStats,
  getHourlyStats,
  clearOldLogs
} from '@/api/spiders'
```

改为：

```typescript
import {
  getSpiderLogs,
  getSpiderStats,
  getSpiderTrend,
  clearOldLogs
} from '@/api/spiders'
import type { SpiderTrendPoint } from '@/api/spiders'
```

**Step 2: 修改变量定义**

将：

```typescript
const chartType = ref<'daily' | 'hourly'>('daily')
```

改为：

```typescript
const periodType = ref<'minute' | 'hour' | 'day' | 'month'>('hour')
```

**Step 3: 修改 loadChart 函数**

将整个 `loadChart` 函数替换为：

```typescript
const loadChart = async () => {
  if (!chartRef.value) return

  chart = chart || echarts.init(chartRef.value)

  try {
    const response = await getSpiderTrend({ period: periodType.value, limit: 100 })
    const trendData = response.items || []

    // 根据实际返回的 period 更新（可能发生了回退）
    if (response.period !== periodType.value) {
      periodType.value = response.period as typeof periodType.value
    }

    // 格式化时间标签
    const formatTime = (timeStr: string): string => {
      const date = new Date(timeStr)
      switch (periodType.value) {
        case 'minute':
          return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
        case 'hour':
          return date.toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit' })
        case 'day':
          return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
        case 'month':
          return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit' })
        default:
          return timeStr
      }
    }

    // 根据粒度选择图表类型
    const chartTypeMap: Record<string, 'line' | 'bar'> = {
      'minute': 'line',
      'hour': 'bar',
      'day': 'line',
      'month': 'bar'
    }
    const seriesType = chartTypeMap[periodType.value] || 'line'

    chart.setOption({
      tooltip: { trigger: 'axis' },
      xAxis: {
        type: 'category',
        data: trendData.map(d => formatTime(d.time))
      },
      yAxis: { type: 'value' },
      series: [{
        name: '访问量',
        type: seriesType,
        data: trendData.map(d => d.total),
        smooth: seriesType === 'line',
        areaStyle: seriesType === 'line' ? { opacity: 0.3 } : undefined
      }]
    }, true) // true 表示清除之前的配置
  } catch {
    // 错误已处理
  }
}
```

**Step 4: 验证 TypeScript 编译**

Run: `cd web && npm run type-check`
Expected: 无错误输出

**Step 5: Commit**

```bash
git add web/src/views/spiders/SpiderLogs.vue
git commit -m "feat(web): 蜘蛛日志页面支持四种时间粒度"
```

---

## Task 12: 集成测试

**Step 1: 启动后端服务**

Run: `cd api && go run main.go`
Expected: 日志显示 `SpiderLogsArchiver initialized and started`

**Step 2: 启动前端开发服务器**

Run: `cd web && npm run dev`
Expected: 开发服务器启动成功

**Step 3: 手动测试**

1. 访问蜘蛛日志页面
2. 切换四种时间粒度（分钟、小时、天、月）
3. 验证图表正确切换
4. 检查浏览器控制台无错误

**Step 4: 验证 API 接口**

Run: `curl -H "Authorization: Bearer <token>" "http://localhost:8080/api/spiders/trend?period=hour&limit=10"`
Expected: 返回 JSON 格式的趋势数据

**Step 5: Final Commit**

```bash
git add -A
git commit -m "feat: 完成蜘蛛日志趋势图表功能增强

- 新增 spider_logs_stats 预聚合表
- 实现 SpiderLogsArchiver 归档服务
- 添加 /api/spiders/trend 接口
- 前端支持分钟、小时、天、月四种粒度"
```

---

## 测试检查清单

- [ ] 数据库表 `spider_logs_stats` 创建成功
- [ ] 归档服务正确启动并记录日志
- [ ] API `/api/spiders/trend` 返回正确格式
- [ ] 回退机制正常工作（无数据时降级到更细粒度）
- [ ] 前端四种粒度切换正常
- [ ] 图表根据粒度显示正确的类型（折线图/柱状图）
- [ ] 时间标签格式正确
