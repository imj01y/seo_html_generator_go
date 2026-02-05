# Go 统一调度定时爬虫设计方案

## 背景

定时爬虫功能不生效，原因是：
1. `spider_projects.schedule` 字段被存储但从未被读取执行
2. Python 端的 `SpiderSchedulerWorker` 存在但未启动且依赖缺失
3. Go 端的 Scheduler 只支持 `refresh_data` 和 `refresh_template` 两种任务类型

## 设计目标

- Go 统一负责所有定时调度（包括爬虫执行）
- 保持前端 `ScheduleBuilder` 不变
- 通过 Redis pub/sub 触发 Python 执行爬虫
- 删除 Python 端冗余的调度代码

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                          Go API                                 │
│                                                                 │
│  ┌──────────────────┐      ┌──────────────────┐                │
│  │ SpiderProjects   │      │    Scheduler     │                │
│  │    Handler       │─────▶│                  │                │
│  │ (保存时同步)     │      │ - refresh_data   │                │
│  └──────────────────┘      │ - refresh_template│               │
│                            │ - run_spider ←新增│               │
│                            └────────┬─────────┘                │
│                                     │                          │
│  scheduled_tasks 表                 │ 定时触发                  │
│  ┌─────────────────────────────┐   │                          │
│  │ id=5, type=run_spider       │   ▼                          │
│  │ params={"project_id": 1}    │  RunSpiderHandler             │
│  │ cron_expr="0 0 8 * * *"     │      │                        │
│  └─────────────────────────────┘      │ Redis pub/sub          │
│                                       ▼                        │
└───────────────────────────────────────┼────────────────────────┘
                                        │
                    ┌───────────────────▼────────────────────┐
                    │           Python Worker                │
                    │                                        │
                    │  CommandListener (已有，无需改动)       │
                    │  - 接收 "run" 命令                     │
                    │  - 执行爬虫项目                        │
                    └────────────────────────────────────────┘
```

## 数据流

### 用户保存爬虫项目时

```
1. 前端提交 PUT /api/spider-projects/:id
   body: { schedule: '{"type":"daily","time":"08:00"}', enabled: 1, ... }

2. SpiderProjectsHandler.Update() 处理：
   ├─ 更新 spider_projects 表（保存原始 JSON）
   └─ 调用 SyncSpiderSchedule() 同步定时任务

3. SyncSpiderSchedule() 逻辑：
   ├─ 解析 schedule JSON
   ├─ 转换为 Cron 表达式："0 0 8 * * *"
   ├─ 查询 scheduled_tasks 是否已存在该项目的任务
   │   ├─ 存在 + schedule 有效 → UPDATE
   │   ├─ 存在 + schedule 为空/none → DELETE
   │   └─ 不存在 + schedule 有效 → INSERT
   └─ Scheduler 自动重新加载该任务
```

### JSON 到 Cron 转换规则

| JSON 类型 | 示例 | Cron 表达式 |
|-----------|------|-------------|
| `interval_minutes` | `{"interval": 30}` | `0 */30 * * * *` |
| `interval_hours` | `{"interval": 2}` | `0 0 */2 * * *` |
| `daily` | `{"time": "08:00"}` | `0 0 8 * * *` |
| `weekly` | `{"days": [1,3,5], "time": "09:00"}` | `0 0 9 * * 1,3,5` |
| `monthly` | `{"dates": [1,15], "time": "10:00"}` | `0 0 10 1,15 * *` |

## 代码变更

### Go 后端

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `scheduler_types.go` | 修改 | 新增 `TaskTypeRunSpider`、`RunSpiderParams`、`ParseRunSpiderParams` |
| `task_handlers.go` | 修改 | 新增 `RunSpiderHandler`，修改 `RegisterAllHandlers` 签名 |
| `schedule_sync.go` | 新建 | `SyncSpiderSchedule`、`DeleteSpiderSchedule`、`ScheduleJSONToCron` |
| `middleware.go` | 修改 | `DependencyInjectionMiddleware` 添加 `scheduler` 参数 |
| `router.go` | 修改 | 中间件调用添加 `deps.Scheduler` |
| `spider_projects.go` | 修改 | Create/Update/Delete/Toggle 添加定时任务同步逻辑 |
| `main.go` | 修改 | `RegisterAllHandlers` 调用添加 `db` 和 `redisClient` 参数 |

### Python 端

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `spider_scheduler.py` | 删除 | 整个文件删除 |
| `initializers.py` | 修改 | 移除 `_scheduler_worker` 相关代码 |
| `lifecycle.py` | 修改 | 移除 `_scheduler_worker` 清理代码 |

### 无需改动

- 前端 `ScheduleBuilder.vue`
- Python `CommandListener`
- 数据库表结构

## 关键代码

### scheduler_types.go 新增

```go
const TaskTypeRunSpider TaskType = "run_spider"

type RunSpiderParams struct {
    ProjectID   int    `json:"project_id"`
    ProjectName string `json:"project_name"`
}

func ParseRunSpiderParams(data json.RawMessage) (*RunSpiderParams, error) {
    var params RunSpiderParams
    if err := json.Unmarshal(data, &params); err != nil {
        return nil, err
    }
    if params.ProjectID == 0 {
        return nil, fmt.Errorf("project_id is required")
    }
    return &params, nil
}
```

### RunSpiderHandler

```go
type RunSpiderHandler struct {
    redis *redis.Client
    db    *sqlx.DB
}

func NewRunSpiderHandler(redis *redis.Client, db *sqlx.DB) *RunSpiderHandler {
    return &RunSpiderHandler{redis: redis, db: db}
}

func (h *RunSpiderHandler) TaskType() TaskType {
    return TaskTypeRunSpider
}

func (h *RunSpiderHandler) Handle(task *ScheduledTask) TaskResult {
    startTime := time.Now()
    ctx := context.Background()

    params, err := ParseRunSpiderParams(task.Params)
    if err != nil {
        return TaskResult{
            Success:  false,
            Message:  fmt.Sprintf("parse params failed: %v", err),
            Duration: time.Since(startTime).Milliseconds(),
        }
    }

    log.Info().
        Int("project_id", params.ProjectID).
        Str("project_name", params.ProjectName).
        Msg("Running scheduled spider")

    // 检查项目状态和是否启用
    var project struct {
        Status  string `db:"status"`
        Enabled int    `db:"enabled"`
    }
    if err := h.db.Get(&project, "SELECT status, enabled FROM spider_projects WHERE id = ?", params.ProjectID); err != nil {
        return TaskResult{
            Success:  false,
            Message:  "项目不存在",
            Duration: time.Since(startTime).Milliseconds(),
        }
    }
    if project.Enabled == 0 {
        return TaskResult{
            Success:  false,
            Message:  "项目已禁用，跳过",
            Duration: time.Since(startTime).Milliseconds(),
        }
    }
    if project.Status == "running" {
        return TaskResult{
            Success:  false,
            Message:  "项目正在运行中，跳过",
            Duration: time.Since(startTime).Milliseconds(),
        }
    }

    // 更新状态为 running
    h.db.Exec("UPDATE spider_projects SET status = 'running' WHERE id = ?", params.ProjectID)

    // 使用现有的 SpiderCommand 结构体
    cmd := models.SpiderCommand{
        Action:    "run",
        ProjectID: params.ProjectID,
        Timestamp: time.Now().Unix(),
    }
    cmdJSON, _ := json.Marshal(cmd)

    if err := h.redis.Publish(ctx, "spider:commands", cmdJSON).Err(); err != nil {
        // 回滚状态
        h.db.Exec("UPDATE spider_projects SET status = 'idle' WHERE id = ?", params.ProjectID)
        return TaskResult{
            Success:  false,
            Message:  fmt.Sprintf("发送命令失败: %v", err),
            Duration: time.Since(startTime).Milliseconds(),
        }
    }

    return TaskResult{
        Success:  true,
        Message:  fmt.Sprintf("已触发爬虫: %s (id=%d)", params.ProjectName, params.ProjectID),
        Duration: time.Since(startTime).Milliseconds(),
    }
}
```

### schedule_sync.go

```go
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

type ScheduleConfig struct {
    Type     string `json:"type"`
    Interval int    `json:"interval,omitempty"`
    Time     string `json:"time,omitempty"`
    Days     []int  `json:"days,omitempty"`
    Dates    []int  `json:"dates,omitempty"`
}

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

func intsToString(nums []int) string {
    strs := make([]string, len(nums))
    for i, n := range nums {
        strs[i] = strconv.Itoa(n)
    }
    return strings.Join(strs, ",")
}

func DeleteSpiderSchedule(ctx context.Context, db *sqlx.DB, scheduler *Scheduler, projectID int) error {
    var taskID int64
    err := db.GetContext(ctx, &taskID,
        `SELECT id FROM scheduled_tasks
         WHERE task_type = 'run_spider'
         AND JSON_UNQUOTE(JSON_EXTRACT(params, '$.project_id')) = ?`,
        strconv.Itoa(projectID))

    if err != nil {
        return nil
    }

    return scheduler.DeleteTask(ctx, taskID)
}
```

### middleware.go 修改

```go
func DependencyInjectionMiddleware(db *sqlx.DB, rdb *redis.Client, cfg *config.Config, scheduler *core.Scheduler) gin.HandlerFunc {
    return func(c *gin.Context) {
        if db != nil {
            c.Set("db", db)
        }
        if rdb != nil {
            c.Set("redis", rdb)
        }
        if cfg != nil {
            c.Set("config", cfg)
        }
        if scheduler != nil {
            c.Set("scheduler", scheduler)
        }
        c.Next()
    }
}
```

### spider_projects.go 集成示例

```go
// Create 方法末尾
if scheduler, exists := c.Get("scheduler"); exists && req.Schedule != nil {
    s := scheduler.(*core.Scheduler)
    ctx := context.Background()
    if err := core.SyncSpiderSchedule(ctx, sqlxDB, s, int(projectID), req.Name, req.Schedule, req.Enabled); err != nil {
        log.Warn().Err(err).Int64("project_id", projectID).Msg("Failed to sync spider schedule")
    }
}

// Update 方法末尾
if scheduler, exists := c.Get("scheduler"); exists && (req.Schedule != nil || req.Enabled != nil) {
    s := scheduler.(*core.Scheduler)
    var project struct {
        Name     string  `db:"name"`
        Schedule *string `db:"schedule"`
        Enabled  int     `db:"enabled"`
    }
    if err := sqlxDB.Get(&project, "SELECT name, schedule, enabled FROM spider_projects WHERE id = ?", id); err == nil {
        ctx := context.Background()
        core.SyncSpiderSchedule(ctx, sqlxDB, s, id, project.Name, project.Schedule, project.Enabled)
    }
}

// Delete 方法（删除项目数据之前）
if scheduler, exists := c.Get("scheduler"); exists {
    s := scheduler.(*core.Scheduler)
    ctx := context.Background()
    core.DeleteSpiderSchedule(ctx, sqlxDB, s, id)
}

// Toggle 方法
if scheduler, exists := c.Get("scheduler"); exists {
    s := scheduler.(*core.Scheduler)
    var project struct {
        Name     string  `db:"name"`
        Schedule *string `db:"schedule"`
    }
    if err := sqlxDB.Get(&project, "SELECT name, schedule FROM spider_projects WHERE id = ?", id); err == nil {
        ctx := context.Background()
        core.SyncSpiderSchedule(ctx, sqlxDB, s, id, project.Name, project.Schedule, newEnabled)
    }
}
```

## 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 调度架构 | Go 统一调度 | 状态持久化、统一管理、可观测性更好 |
| 同步时机 | 保存时同步 | 实时生效，数据一致 |
| 任务标识 | task_type + params.project_id | 与现有结构一致 |
| JSON→Cron 转换 | 后端转换 | 前端无需改动 |
| 级联处理 | 删除/禁用项目时同步处理定时任务 | 数据一致，无孤儿任务 |
| Python 端处理 | 删除 SpiderSchedulerWorker | 避免两套调度逻辑 |
| 失败重试 | 不自动重试 | 简单可控，避免重复执行 |
| SQL 查询 | 使用 JSON_EXTRACT | MySQL 5.7+ 支持，代码简洁 |

## 测试要点

1. 创建项目时配置定时规则，验证 scheduled_tasks 表生成记录
2. 修改项目定时规则，验证 scheduled_tasks 表更新
3. 禁用项目，验证定时任务被禁用
4. 删除项目，验证定时任务被删除
5. 等待定时触发，验证爬虫通过 Redis 命令启动
6. 验证 task_logs 表记录执行日志
