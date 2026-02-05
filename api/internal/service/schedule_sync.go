// Package core provides schedule synchronization utilities
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// ScheduleConfig 前端 JSON 配置结构
type ScheduleConfig struct {
	Type     string `json:"type"`               // none, interval_minutes, interval_hours, daily, weekly, monthly
	Interval int    `json:"interval,omitempty"` // 间隔（分钟或小时）
	Time     string `json:"time,omitempty"`     // HH:mm 格式
	Days     []int  `json:"days,omitempty"`     // 周几 (0=周日, 1-6=周一到周六)
	Dates    []int  `json:"dates,omitempty"`    // 每月几号 (1-31)
}

// SyncSpiderSchedule 同步爬虫项目的定时配置到 scheduled_tasks 表
func SyncSpiderSchedule(ctx context.Context, db *sqlx.DB, scheduler *Scheduler, projectID int, projectName string, scheduleJSON *string, enabled int) error {
	// 查找已存在的任务
	var existingTaskID int64
	err := db.GetContext(ctx, &existingTaskID,
		`SELECT id FROM scheduled_tasks
		 WHERE task_type = 'run_spider'
		 AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.project_id')) = ?`,
		strconv.Itoa(projectID))

	taskExists := err == nil && existingTaskID > 0

	// 无配置或类型为 none，删除已有任务
	if scheduleJSON == nil || *scheduleJSON == "" {
		if taskExists {
			return scheduler.DeleteTask(ctx, existingTaskID)
		}
		return nil
	}

	var config ScheduleConfig
	if err := json.Unmarshal([]byte(*scheduleJSON), &config); err != nil {
		log.Warn().Err(err).Int("project_id", projectID).Msg("Invalid schedule JSON")
		return nil
	}

	if config.Type == "none" {
		if taskExists {
			return scheduler.DeleteTask(ctx, existingTaskID)
		}
		return nil
	}

	// 转换为 Cron 表达式
	cronExpr, err := ScheduleJSONToCron(config)
	if err != nil {
		log.Warn().Err(err).Int("project_id", projectID).Msg("Failed to convert schedule to cron")
		return nil
	}

	// 构建任务参数
	params, _ := json.Marshal(map[string]interface{}{
		"project_id":   projectID,
		"project_name": projectName,
	})

	task := &ScheduledTask{
		Name:     fmt.Sprintf("爬虫: %s", projectName),
		TaskType: TaskTypeRunSpider,
		CronExpr: cronExpr,
		Params:   params,
		Enabled:  enabled == 1,
	}

	if taskExists {
		task.ID = existingTaskID
		return scheduler.UpdateTask(ctx, task)
	}

	_, err = scheduler.CreateTask(ctx, task)
	return err
}

// ScheduleJSONToCron 将前端 JSON 配置转换为 Cron 表达式
// Cron 格式: 秒 分 时 日 月 周
func ScheduleJSONToCron(config ScheduleConfig) (string, error) {
	switch config.Type {
	case "interval_minutes":
		if config.Interval <= 0 {
			return "", fmt.Errorf("invalid interval: %d", config.Interval)
		}
		return fmt.Sprintf("0 */%d * * * *", config.Interval), nil

	case "interval_hours":
		if config.Interval <= 0 {
			return "", fmt.Errorf("invalid interval: %d", config.Interval)
		}
		return fmt.Sprintf("0 0 */%d * * *", config.Interval), nil

	case "daily":
		hour, minute, err := parseTime(config.Time)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("0 %d %d * * *", minute, hour), nil

	case "weekly":
		if len(config.Days) == 0 {
			return "", fmt.Errorf("no days specified for weekly schedule")
		}
		hour, minute, err := parseTime(config.Time)
		if err != nil {
			return "", err
		}
		days := intsToString(config.Days)
		return fmt.Sprintf("0 %d %d * * %s", minute, hour, days), nil

	case "monthly":
		if len(config.Dates) == 0 {
			return "", fmt.Errorf("no dates specified for monthly schedule")
		}
		hour, minute, err := parseTime(config.Time)
		if err != nil {
			return "", err
		}
		dates := intsToString(config.Dates)
		return fmt.Sprintf("0 %d %d %s * *", minute, hour, dates), nil

	default:
		return "", fmt.Errorf("unknown schedule type: %s", config.Type)
	}
}

// parseTime 解析 HH:mm 格式时间
func parseTime(timeStr string) (hour, minute int, err error) {
	if timeStr == "" {
		return 0, 0, fmt.Errorf("time is empty")
	}
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s", timeStr)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return hour, minute, nil
}

// intsToString 将整数数组转换为逗号分隔的字符串
func intsToString(nums []int) string {
	strs := make([]string, len(nums))
	for i, n := range nums {
		strs[i] = strconv.Itoa(n)
	}
	return strings.Join(strs, ",")
}

// DeleteSpiderSchedule 删除爬虫项目的定时任务
func DeleteSpiderSchedule(ctx context.Context, db *sqlx.DB, scheduler *Scheduler, projectID int) error {
	var taskID int64
	err := db.GetContext(ctx, &taskID,
		`SELECT id FROM scheduled_tasks
		 WHERE task_type = 'run_spider'
		 AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.project_id')) = ?`,
		strconv.Itoa(projectID))

	if err != nil {
		return nil // 不存在则无需删除
	}

	return scheduler.DeleteTask(ctx, taskID)
}
