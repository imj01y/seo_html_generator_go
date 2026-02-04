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
