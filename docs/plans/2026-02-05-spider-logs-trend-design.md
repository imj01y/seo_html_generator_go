# 蜘蛛日志趋势图表功能增强设计

## 背景

蜘蛛日志页面（SpiderLogs.vue）的访问趋势图表当前只支持按天和按小时两种粒度。本设计将其扩展为支持分钟、小时、天、月四种粒度，与爬虫统计页面（SpiderStats.vue）保持一致。

## 需求

- 蜘蛛日志页面支持分钟、小时、天、月四种时间粒度
- 使用预聚合存储提升查询性能
- 数据保留策略：分钟 7 天、小时 30 天、天和月永久保留

---

## 一、数据库设计

新建 `spider_logs_stats` 预聚合表：

```sql
CREATE TABLE IF NOT EXISTS spider_logs_stats (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    period_type ENUM('minute', 'hour', 'day', 'month') NOT NULL COMMENT '周期类型',
    period_start DATETIME NOT NULL COMMENT '周期开始时间',
    spider_type VARCHAR(20) DEFAULT NULL COMMENT '蜘蛛类型，NULL表示全部',
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

---

## 二、后端 API 设计

### 新增接口

`GET /api/spiders/trend`

**请求参数：**

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| period | string | 否 | hour | 粒度：minute/hour/day/month |
| spider_type | string | 否 | - | 蜘蛛类型筛选 |
| limit | int | 否 | 100 | 返回记录数（1-500） |

**响应结构：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "period": "hour",
    "items": [
      {
        "time": "2026-02-05T10:00:00+08:00",
        "total": 1234,
        "status_2xx": 1100,
        "status_3xx": 50,
        "status_4xx": 80,
        "status_5xx": 4,
        "avg_resp_time": 45
      }
    ]
  },
  "timestamp": 1738728000,
  "request_id": "xxx"
}
```

**回退机制：** 数据不足时自动降级（month→day→hour→minute）

**代码位置：** `api/internal/handler/spider_detector.go` 新增 `GetSpiderTrend` 方法

---

## 三、归档服务设计

**新增文件：** `api/internal/service/spider_logs_archiver.go`

### 数据流

```
┌─────────────────────────────────────────────────────────────┐
│                    spider_logs 原始表                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ 每分钟聚合
┌─────────────────────────────────────────────────────────────┐
│          spider_logs_stats (period_type='minute')           │
│                      保留 7 天                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ 每小时聚合（从分钟数据）
┌─────────────────────────────────────────────────────────────┐
│          spider_logs_stats (period_type='hour')             │
│                      保留 30 天                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ 每天凌晨聚合（从小时数据）
┌─────────────────────────────────────────────────────────────┐
│          spider_logs_stats (period_type='day')              │
│                      永久保留                                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼ 每月1日聚合（从天数据）
┌─────────────────────────────────────────────────────────────┐
│          spider_logs_stats (period_type='month')            │
│                      永久保留                                │
└─────────────────────────────────────────────────────────────┘
```

### 任务触发逻辑

| 任务 | 触发条件 | 数据来源 |
|------|---------|---------|
| 分钟聚合 | `now.Sub(lastMinuteRun) >= 1min` | `spider_logs` 原始表 |
| 小时聚合 | `now.Minute() == 0` | `spider_logs_stats` minute 数据 |
| 天聚合 | `now.Hour() == 0 && now.Minute() < 10` | `spider_logs_stats` hour 数据 |
| 月聚合 | `now.Day() == 1 && now.Hour() == 0 && now.Minute() < 15` | `spider_logs_stats` day 数据 |
| 清理 | 与小时/天聚合同时执行 | - |

### 聚合 SQL 示例（分钟）

```sql
INSERT INTO spider_logs_stats
  (period_type, period_start, spider_type, total, status_2xx, status_3xx, status_4xx, status_5xx, avg_resp_time)
SELECT
  'minute',
  DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:00'),
  spider_type,
  COUNT(*),
  SUM(CASE WHEN status >= 200 AND status < 300 THEN 1 ELSE 0 END),
  SUM(CASE WHEN status >= 300 AND status < 400 THEN 1 ELSE 0 END),
  SUM(CASE WHEN status >= 400 AND status < 500 THEN 1 ELSE 0 END),
  SUM(CASE WHEN status >= 500 THEN 1 ELSE 0 END),
  AVG(resp_time)
FROM spider_logs
WHERE created_at >= ? AND created_at < ?
GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d %H:%i:00'), spider_type
ON DUPLICATE KEY UPDATE
  total = VALUES(total),
  status_2xx = VALUES(status_2xx),
  status_3xx = VALUES(status_3xx),
  status_4xx = VALUES(status_4xx),
  status_5xx = VALUES(status_5xx),
  avg_resp_time = VALUES(avg_resp_time)
```

---

## 四、前端改动

### 修改文件

- `web/src/views/spiders/SpiderLogs.vue`
- `web/src/api/spiders.ts`

### 图表选择器改造

```vue
<!-- 改前 -->
<el-radio-group v-model="chartType" size="small" @change="loadChart">
  <el-radio-button label="daily">按天</el-radio-button>
  <el-radio-button label="hourly">按小时</el-radio-button>
</el-radio-group>

<!-- 改后 -->
<el-radio-group v-model="periodType" size="small" @change="loadChart">
  <el-radio-button label="minute">分钟</el-radio-button>
  <el-radio-button label="hour">小时</el-radio-button>
  <el-radio-button label="day">天</el-radio-button>
  <el-radio-button label="month">月</el-radio-button>
</el-radio-group>
```

### 变量改动

```typescript
// 改前
const chartType = ref<'daily' | 'hourly'>('daily')

// 改后
const periodType = ref<'minute' | 'hour' | 'day' | 'month'>('hour')
```

### 新增 API 函数

```typescript
// web/src/api/spiders.ts

export interface SpiderTrendPoint {
  time: string
  total: number
  status_2xx: number
  status_3xx: number
  status_4xx: number
  status_5xx: number
  avg_resp_time: number
}

export async function getSpiderTrend(params?: {
  period?: 'minute' | 'hour' | 'day' | 'month'
  spider_type?: string
  limit?: number
}): Promise<{ period: string; items: SpiderTrendPoint[] }> {
  const res = await request.get('/spiders/trend', { params })
  return res.data || { period: params?.period || 'hour', items: [] }
}
```

### 图表类型策略

| 粒度 | 图表类型 | 时间格式示例 |
|------|---------|-------------|
| minute | 折线图 | 10:30 |
| hour | 柱状图 | 02/05 10时 |
| day | 折线图 | 02/05 |
| month | 柱状图 | 2026/02 |

---

## 五、文件改动清单

| 类型 | 文件路径 | 改动说明 |
|------|---------|---------|
| 新增 | `migrations/003_spider_logs_stats.sql` | 创建预聚合表 |
| 新增 | `api/internal/service/spider_logs_archiver.go` | 归档服务 |
| 新增 | `api/internal/model/spider_logs_stats.go` | 数据模型 |
| 修改 | `api/internal/handler/spider_detector.go` | 新增 GetSpiderTrend 接口 |
| 修改 | `api/internal/handler/router.go` | 注册新路由 |
| 修改 | `api/internal/service/init.go` 或启动入口 | 启动归档服务 |
| 修改 | `web/src/api/spiders.ts` | 新增 getSpiderTrend API |
| 修改 | `web/src/views/spiders/SpiderLogs.vue` | 图表选择器和加载逻辑 |

---

## 六、实现顺序

1. 数据库迁移（创建表）
2. 后端模型定义
3. 归档服务实现
4. API 接口实现
5. 前端 API 函数
6. 前端页面改造
7. 测试验证

---

## 七、测试要点

- [ ] 归档服务正确聚合分钟数据
- [ ] 小时/天/月数据正确从下级聚合
- [ ] API 回退机制正常工作（无数据时降级）
- [ ] 前端四种粒度切换正常
- [ ] 图表正确显示时间格式
- [ ] 过期数据正确清理（分钟 7 天、小时 30 天）
