package core

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

// StatsArchiver 统计归档服务
// 定时将 Redis 中的实时统计数据归档到 MySQL，用于历史趋势图表
type StatsArchiver struct {
	db    *sqlx.DB
	redis *redis.Client

	mu            sync.Mutex
	running       bool
	stopCh        chan struct{}
	lastMinuteRun time.Time
	lastHourRun   time.Time
	lastDayRun    time.Time

	// 保存每个项目上次归档时的统计值（用于计算增量）
	archivedMu   sync.RWMutex
	lastArchived map[int]statsSnapshot
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
	// 重新创建 stopCh 以支持重启
	a.stopCh = make(chan struct{})
	a.mu.Unlock()

	// 确保退出时重置状态
	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	log.Info().Msg("StatsArchiver started")

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("StatsArchiver stopped (context cancelled)")
			return
		case <-a.stopCh:
			log.Info().Msg("StatsArchiver stopped")
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
	if a.running && a.stopCh != nil {
		close(a.stopCh)
		// running 将由 Start 的 defer 设置为 false
	}
}

func (a *StatsArchiver) runTasks(ctx context.Context, now time.Time) {
	// 每分钟：归档分钟统计
	if now.Sub(a.lastMinuteRun) >= time.Minute {
		if err := a.archiveMinuteStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("archiveMinuteStats error")
		}
		a.lastMinuteRun = now
	}

	// 每小时：聚合小时统计
	if now.Minute() == 0 && now.Sub(a.lastHourRun) >= time.Hour {
		if err := a.aggregateHourStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("aggregateHourStats error")
		}
		// 清理 7 天前的分钟数据
		a.cleanupOldData(ctx, "minute", 7)
		a.lastHourRun = now
	}

	// 每天凌晨：聚合天统计
	if now.Hour() == 0 && now.Minute() < 10 && now.Sub(a.lastDayRun) >= 24*time.Hour {
		if err := a.aggregateDayStats(ctx, now); err != nil {
			log.Error().Err(err).Msg("aggregateDayStats error")
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
		a.archivedMu.RLock()
		last := a.lastArchived[projectID]
		a.archivedMu.RUnlock()
		delta := statsSnapshot{
			Total:     maxInt64(0, current.Total-last.Total),
			Completed: maxInt64(0, current.Completed-last.Completed),
			Failed:    maxInt64(0, current.Failed-last.Failed),
			Retried:   maxInt64(0, current.Retried-last.Retried),
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
				log.Error().Err(err).Int("project_id", projectID).Msg("save minute stats error")
			}
		}

		// 更新基准值
		a.archivedMu.Lock()
		a.lastArchived[projectID] = current
		a.archivedMu.Unlock()
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
		log.Error().Err(err).Str("period_type", periodType).Msg("cleanupOldData error")
		return
	}

	if affected, _ := result.RowsAffected(); affected > 0 {
		log.Info().Int64("count", affected).Str("period_type", periodType).Msg("Cleaned up old stats records")
	}
}

func parseInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
