# 爬虫统计重构实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 重构爬虫统计功能，让实时统计直接从 Redis 读取（解决"统计为空"问题），历史趋势通过 Go 后台任务归档到 MySQL

**Architecture:**
- 实时统计（overview/by-project）：Go API 直接从 Redis 读取 `spider:{project_id}:stats`
- 历史趋势（chart）：Go 后台服务 `StatsArchiver` 定时归档 Redis → MySQL
- 移除 Python 中未使用的 `stats_worker.py`

**Tech Stack:** Go, Gin, Redis, MySQL, goroutine

---

## Task 1: 修改 GetOverview 从 Redis 读取实时数据

**Files:**
- Modify: `api/internal/handler/spiders.go:1430-1475`

**Step 1: 修改 GetOverview 方法**

将原来从 MySQL 读取改为从 Redis 读取实时统计。

```go
// GetOverview 获取统计概览（从 Redis 读取实时数据）
func (h *SpiderStatsHandler) GetOverview(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": map[string]interface{}{
			"total": 0, "completed": 0, "failed": 0, "retried": 0, "success_rate": 0, "avg_speed": 0,
		}})
		return
	}
	redisClient := rdb.(*redis.Client)

	projectIDStr := c.Query("project_id")
	ctx := context.Background()

	var total, completed, failed, retried int64

	if projectIDStr != "" && projectIDStr != "0" {
		// 单个项目统计
		projectID, _ := strconv.Atoi(projectIDStr)
		statsKey := fmt.Sprintf("spider:%d:stats", projectID)
		statsData, err := redisClient.HGetAll(ctx, statsKey).Result()
		if err == nil && len(statsData) > 0 {
			total, _ = strconv.ParseInt(statsData["total"], 10, 64)
			completed, _ = strconv.ParseInt(statsData["completed"], 10, 64)
			failed, _ = strconv.ParseInt(statsData["failed"], 10, 64)
			retried, _ = strconv.ParseInt(statsData["retried"], 10, 64)
		}
	} else {
		// 全部项目统计：扫描所有 spider:*:stats 键
		iter := redisClient.Scan(ctx, 0, "spider:*:stats", 100).Iterator()
		for iter.Next(ctx) {
			key := iter.Val()
			// 排除 archived 和 test 键
			if strings.Contains(key, ":archived") || strings.HasPrefix(key, "test_spider:") {
				continue
			}
			statsData, err := redisClient.HGetAll(ctx, key).Result()
			if err == nil {
				t, _ := strconv.ParseInt(statsData["total"], 10, 64)
				c, _ := strconv.ParseInt(statsData["completed"], 10, 64)
				f, _ := strconv.ParseInt(statsData["failed"], 10, 64)
				r, _ := strconv.ParseInt(statsData["retried"], 10, 64)
				total += t
				completed += c
				failed += f
				retried += r
			}
		}
	}

	var successRate float64
	if total > 0 {
		successRate = float64(completed) / float64(total) * 100
	}

	c.JSON(200, gin.H{"success": true, "data": gin.H{
		"total":        total,
		"completed":    completed,
		"failed":       failed,
		"retried":      retried,
		"success_rate": successRate,
		"avg_speed":    0, // 实时统计不计算速度
	}})
}
```

**Step 2: 验证修改**

运行 Go API，访问 `/api/spider-stats/overview`，确认返回 Redis 中的实时数据。

**Step 3: Commit**

```bash
git add api/internal/handler/spiders.go
git commit -m "feat: GetOverview 从 Redis 读取实时统计数据"
```

---

## Task 2: 修改 GetByProject 从 Redis 读取实时数据

**Files:**
- Modify: `api/internal/handler/spiders.go:1554-1589`

**Step 1: 修改 GetByProject 方法**

```go
// GetByProject 按项目统计（从 Redis 读取实时数据）
func (h *SpiderStatsHandler) GetByProject(c *gin.Context) {
	db, dbExists := c.Get("db")
	rdb, redisExists := c.Get("redis")
	if !dbExists || !redisExists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)
	redisClient := rdb.(*redis.Client)
	ctx := context.Background()

	// 获取所有项目
	var projects []struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}
	sqlxDB.Select(&projects, "SELECT id, name FROM spider_projects ORDER BY id")

	// 从 Redis 获取每个项目的统计
	result := make([]gin.H, 0, len(projects))
	for _, p := range projects {
		statsKey := fmt.Sprintf("spider:%d:stats", p.ID)
		statsData, err := redisClient.HGetAll(ctx, statsKey).Result()

		var total, completed, failed, retried int64
		if err == nil && len(statsData) > 0 {
			total, _ = strconv.ParseInt(statsData["total"], 10, 64)
			completed, _ = strconv.ParseInt(statsData["completed"], 10, 64)
			failed, _ = strconv.ParseInt(statsData["failed"], 10, 64)
			retried, _ = strconv.ParseInt(statsData["retried"], 10, 64)
		}

		var successRate float64
		if total > 0 {
			successRate = float64(completed) / float64(total) * 100
		}

		result = append(result, gin.H{
			"project_id":   p.ID,
			"project_name": p.Name,
			"total":        total,
			"completed":    completed,
			"failed":       failed,
			"retried":      retried,
			"success_rate": successRate,
		})
	}

	c.JSON(200, gin.H{"success": true, "data": result})
}
```

**Step 2: 验证修改**

访问 `/api/spider-stats/by-project`，确认返回实时数据。

**Step 3: Commit**

```bash
git add api/internal/handler/spiders.go
git commit -m "feat: GetByProject 从 Redis 读取实时统计数据"
```

---

## Task 3: 创建 StatsArchiver 服务

**Files:**
- Create: `api/internal/service/stats_archiver.go`

**Step 1: 创建 StatsArchiver 结构**

```go
package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

// StatsArchiver 统计归档服务
// 定时将 Redis 中的实时统计数据归档到 MySQL，用于历史趋势图表
type StatsArchiver struct {
	db     *sqlx.DB
	redis  *redis.Client
	logger *logrus.Logger

	mu              sync.Mutex
	running         bool
	stopCh          chan struct{}
	lastMinuteRun   time.Time
	lastHourRun     time.Time
	lastDayRun      time.Time

	// 保存每个项目上次归档时的统计值（用于计算增量）
	lastArchived    map[int]statsSnapshot
}

type statsSnapshot struct {
	Total     int64
	Completed int64
	Failed    int64
	Retried   int64
}

// NewStatsArchiver 创建统计归档服务
func NewStatsArchiver(db *sqlx.DB, redis *redis.Client) *StatsArchiver {
	return &StatsArchiver{
		db:           db,
		redis:        redis,
		logger:       logrus.StandardLogger(),
		stopCh:       make(chan struct{}),
		lastArchived: make(map[int]statsSnapshot),
	}
}

// Start 启动归档服务（在 goroutine 中调用）
func (a *StatsArchiver) Start(ctx context.Context) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	a.mu.Unlock()

	a.logger.Info("StatsArchiver started")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("StatsArchiver stopped (context cancelled)")
			return
		case <-a.stopCh:
			a.logger.Info("StatsArchiver stopped")
			return
		case now := <-ticker.C:
			a.runTasks(ctx, now)
		}
	}
}

// Stop 停止归档服务
func (a *StatsArchiver) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.running {
		close(a.stopCh)
		a.running = false
	}
}

func (a *StatsArchiver) runTasks(ctx context.Context, now time.Time) {
	// 每分钟：归档分钟统计
	if now.Sub(a.lastMinuteRun) >= time.Minute {
		if err := a.archiveMinuteStats(ctx, now); err != nil {
			a.logger.Errorf("archiveMinuteStats error: %v", err)
		}
		a.lastMinuteRun = now
	}

	// 每小时：聚合小时统计
	if now.Minute() == 0 && now.Sub(a.lastHourRun) >= time.Hour {
		if err := a.aggregateHourStats(ctx, now); err != nil {
			a.logger.Errorf("aggregateHourStats error: %v", err)
		}
		// 清理 7 天前的分钟数据
		a.cleanupOldData(ctx, "minute", 7)
		a.lastHourRun = now
	}

	// 每天凌晨：聚合天统计
	if now.Hour() == 0 && now.Minute() < 10 && now.Sub(a.lastDayRun) >= 24*time.Hour {
		if err := a.aggregateDayStats(ctx, now); err != nil {
			a.logger.Errorf("aggregateDayStats error: %v", err)
		}
		// 清理 30 天前的小时数据
		a.cleanupOldData(ctx, "hour", 30)
		a.lastDayRun = now
	}
}

// archiveMinuteStats 归档分钟统计（保存增量）
func (a *StatsArchiver) archiveMinuteStats(ctx context.Context, now time.Time) error {
	periodStart := now.Truncate(time.Minute)

	// 扫描所有 spider:*:stats 键
	iter := a.redis.Scan(ctx, 0, "spider:*:stats", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// 排除 archived 和 test 键
		if strings.Contains(key, ":archived") || strings.HasPrefix(key, "test_spider:") {
			continue
		}

		// 解析项目 ID
		parts := strings.Split(key, ":")
		if len(parts) < 2 {
			continue
		}
		projectID, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}

		// 获取当前统计
		statsData, err := a.redis.HGetAll(ctx, key).Result()
		if err != nil || len(statsData) == 0 {
			continue
		}

		current := statsSnapshot{
			Total:     parseInt64(statsData["total"]),
			Completed: parseInt64(statsData["completed"]),
			Failed:    parseInt64(statsData["failed"]),
			Retried:   parseInt64(statsData["retried"]),
		}

		// 计算增量
		last := a.lastArchived[projectID]
		delta := statsSnapshot{
			Total:     max(0, current.Total-last.Total),
			Completed: max(0, current.Completed-last.Completed),
			Failed:    max(0, current.Failed-last.Failed),
			Retried:   max(0, current.Retried-last.Retried),
		}

		// 有变化才保存
		if delta.Total > 0 || delta.Completed > 0 || delta.Failed > 0 {
			avgSpeed := float64(delta.Completed) // 每分钟完成数作为速度

			_, err = a.db.ExecContext(ctx, `
				INSERT INTO spider_stats_history
				(project_id, period_type, period_start, total, completed, failed, retried, avg_speed)
				VALUES (?, 'minute', ?, ?, ?, ?, ?, ?)
				ON DUPLICATE KEY UPDATE
					total = VALUES(total),
					completed = VALUES(completed),
					failed = VALUES(failed),
					retried = VALUES(retried),
					avg_speed = VALUES(avg_speed)
			`, projectID, periodStart, delta.Total, delta.Completed, delta.Failed, delta.Retried, avgSpeed)

			if err != nil {
				a.logger.Errorf("save minute stats error: %v", err)
			}
		}

		// 更新基准值
		a.lastArchived[projectID] = current
	}

	return iter.Err()
}

// aggregateHourStats 聚合小时统计
func (a *StatsArchiver) aggregateHourStats(ctx context.Context, now time.Time) error {
	hourStart := now.Truncate(time.Hour).Add(-time.Hour)
	hourEnd := hourStart.Add(time.Hour)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO spider_stats_history (project_id, period_type, period_start, total, completed, failed, retried, avg_speed)
		SELECT project_id, 'hour', ?, SUM(total), SUM(completed), SUM(failed), SUM(retried), SUM(avg_speed)
		FROM spider_stats_history
		WHERE period_type = 'minute' AND period_start >= ? AND period_start < ?
		GROUP BY project_id
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			completed = VALUES(completed),
			failed = VALUES(failed),
			retried = VALUES(retried),
			avg_speed = VALUES(avg_speed)
	`, hourStart, hourStart, hourEnd)

	return err
}

// aggregateDayStats 聚合天统计
func (a *StatsArchiver) aggregateDayStats(ctx context.Context, now time.Time) error {
	dayStart := now.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	dayEnd := dayStart.Add(24 * time.Hour)

	_, err := a.db.ExecContext(ctx, `
		INSERT INTO spider_stats_history (project_id, period_type, period_start, total, completed, failed, retried, avg_speed)
		SELECT project_id, 'day', ?, SUM(total), SUM(completed), SUM(failed), SUM(retried), SUM(avg_speed)
		FROM spider_stats_history
		WHERE period_type = 'hour' AND period_start >= ? AND period_start < ?
		GROUP BY project_id
		ON DUPLICATE KEY UPDATE
			total = VALUES(total),
			completed = VALUES(completed),
			failed = VALUES(failed),
			retried = VALUES(retried),
			avg_speed = VALUES(avg_speed)
	`, dayStart, dayStart, dayEnd)

	return err
}

// cleanupOldData 清理过期数据
func (a *StatsArchiver) cleanupOldData(ctx context.Context, periodType string, retentionDays int) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result, err := a.db.ExecContext(ctx, `
		DELETE FROM spider_stats_history WHERE period_type = ? AND period_start < ?
	`, periodType, cutoff)

	if err != nil {
		a.logger.Errorf("cleanupOldData error: %v", err)
		return
	}

	if affected, _ := result.RowsAffected(); affected > 0 {
		a.logger.Infof("Cleaned up %d old %s stats records", affected, periodType)
	}
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
```

**Step 2: Commit**

```bash
git add api/internal/service/stats_archiver.go
git commit -m "feat: 创建 StatsArchiver 统计归档服务"
```

---

## Task 4: 在 main.go 中启动 StatsArchiver

**Files:**
- Modify: `api/cmd/main.go`

**Step 1: 导入并启动 StatsArchiver**

在初始化完成后，启动 StatsArchiver：

```go
// 在 SetupRouter 之后添加
statsArchiver := core.NewStatsArchiver(db, redis)
go statsArchiver.Start(ctx)
defer statsArchiver.Stop()
```

**Step 2: 验证**

启动 Go API，检查日志确认 "StatsArchiver started"。

**Step 3: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat: 启动 StatsArchiver 后台服务"
```

---

## Task 5: 修改 GetChart 添加回退逻辑

**Files:**
- Modify: `api/internal/handler/spiders.go:1478-1525`

**Step 1: 修改 GetChart 方法**

当目标周期没有数据时，自动回退到更细粒度的数据：

```go
// GetChart 获取图表数据（从 MySQL 读取历史数据，支持回退）
func (h *SpiderStatsHandler) GetChart(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
		return
	}
	sqlxDB := db.(*sqlx.DB)

	projectIDStr := c.Query("project_id")
	period := c.DefaultQuery("period", "hour")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	// 周期回退顺序
	periodFallback := map[string]string{
		"month": "day",
		"day":   "hour",
		"hour":  "minute",
	}

	// 尝试查询，如果没有数据则回退
	for {
		where := "period_type = ?"
		args := []interface{}{period}

		if projectIDStr != "" && projectIDStr != "0" {
			projectID, _ := strconv.Atoi(projectIDStr)
			where += " AND project_id = ?"
			args = append(args, projectID)
		}

		args = append(args, limit)

		var data []map[string]interface{}
		rows, err := sqlxDB.Queryx(`
			SELECT period_start as time, SUM(total) as total, SUM(completed) as completed,
			       SUM(failed) as failed, SUM(retried) as retried, AVG(avg_speed) as avg_speed
			FROM spider_stats_history
			WHERE `+where+`
			GROUP BY period_start
			ORDER BY period_start DESC
			LIMIT ?
		`, args...)

		if err == nil {
			for rows.Next() {
				row := make(map[string]interface{})
				rows.MapScan(row)
				data = append(data, row)
			}
			rows.Close()
		}

		if len(data) > 0 {
			// 反转为时间正序
			for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
				data[i], data[j] = data[j], data[i]
			}
			c.JSON(200, gin.H{"success": true, "data": data})
			return
		}

		// 回退到更细粒度
		fallback, ok := periodFallback[period]
		if !ok {
			// 已经是最细粒度，返回空
			c.JSON(200, gin.H{"success": true, "data": []interface{}{}})
			return
		}
		period = fallback
	}
}
```

**Step 2: Commit**

```bash
git add api/internal/handler/spiders.go
git commit -m "feat: GetChart 支持周期回退逻辑"
```

---

## Task 6: 删除 Python stats_worker.py

**Files:**
- Delete: `worker/core/workers/stats_worker.py`

**Step 1: 删除文件**

```bash
rm worker/core/workers/stats_worker.py
```

**Step 2: 确认 main.py 没有引用**

检查 `worker/main.py`，确认没有导入或使用 `stats_worker`（当前确实没有）。

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: 删除未使用的 Python stats_worker.py"
```

---

## Task 7: 端到端测试

**Step 1: 启动服务**

```bash
# 启动 MySQL 和 Redis
docker-compose up -d mysql redis

# 启动 Go API
cd api && go run cmd/main.go
```

**Step 2: 运行爬虫测试**

在管理后台创建一个爬虫项目，运行测试，确认：
1. 爬虫执行时，`/spider-stats/overview` 能显示实时统计
2. 等待 1 分钟后，`/spider-stats/chart` 能显示历史数据

**Step 3: 最终 Commit**

```bash
git add -A
git commit -m "feat: 完成爬虫统计重构 - 实时 Redis + 历史 MySQL"
```

---

## 总结

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/internal/handler/spiders.go` | 修改 | GetOverview/GetByProject 从 Redis 读取 |
| `api/internal/service/stats_archiver.go` | 新建 | 定时归档服务 |
| `api/cmd/main.go` | 修改 | 启动 StatsArchiver |
| `worker/core/workers/stats_worker.py` | 删除 | 功能已迁移到 Go |
