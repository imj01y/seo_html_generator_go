# 数据加工功能设计文档

## 概述

实现数据加工自动化功能，爬虫抓取的文章自动进入处理队列，Worker 自动处理并生成标题库和段落库。

## 设计决策

1. **启动方式**：随 Worker 自动启动
2. **队列设计**：单一队列 `pending:articles`（移除分组）
3. **并发处理**：可配置并发 Worker 数
4. **错误处理**：重试队列机制（最多 N 次，超过进入死信队列）
5. **配置管理**：后台管理页面配置
6. **实现方案**：集成到现有 Python Worker

## 数据流

```
爬虫抓取 → original_articles 表 → pending:articles 队列 → GeneratorWorker → titles/contents 表
                                         ↓ (失败)
                                  pending:articles:retry 队列 (最多重试 N 次)
                                         ↓ (超过重试次数)
                                  pending:articles:dead 队列 (死信)
```

## 实现任务

### Task 1: 后端 API - 数据加工处理器

**文件**: `api/internal/handler/processor.go`

创建新的 Go API 处理器，包含以下接口：

```go
GET    /api/processor/config       // 获取配置
PUT    /api/processor/config       // 更新配置
GET    /api/processor/status       // 获取运行状态
POST   /api/processor/start        // 手动启动
POST   /api/processor/stop         // 手动停止
POST   /api/processor/retry-all    // 重试所有失败任务
DELETE /api/processor/dead-queue   // 清空死信队列
GET    /api/processor/stats        // 获取统计数据
```

配置结构：
```json
{
  "enabled": true,
  "concurrency": 3,
  "retry_max": 3,
  "min_paragraph_length": 20,
  "batch_size": 50
}
```

状态结构：
```json
{
  "running": true,
  "workers": 3,
  "queue_pending": 1234,
  "queue_retry": 56,
  "queue_dead": 7,
  "processed_total": 50000,
  "processed_today": 1200,
  "speed": 15.5,
  "last_error": null
}
```

通过 Redis 发布命令：
- 频道：`processor:commands`
- 命令：`{"action": "start|stop|reload_config", "timestamp": ...}`

**验证**: 使用 `go build` 确保编译通过

### Task 2: 注册路由

**文件**: `api/internal/handler/router.go`

添加数据加工相关路由：
```go
processor := api.Group("/processor")
{
    processor.GET("/config", processorHandler.GetConfig)
    processor.PUT("/config", processorHandler.UpdateConfig)
    processor.GET("/status", processorHandler.GetStatus)
    processor.POST("/start", processorHandler.Start)
    processor.POST("/stop", processorHandler.Stop)
    processor.POST("/retry-all", processorHandler.RetryAll)
    processor.DELETE("/dead-queue", processorHandler.ClearDeadQueue)
    processor.GET("/stats", processorHandler.GetStats)
}
```

**验证**: 使用 `go build` 确保编译通过

### Task 3: Python Worker - GeneratorManager

**文件**: `worker/core/workers/generator_manager.py`

创建 GeneratorManager 类，管理多个 GeneratorWorker 协程：

```python
class GeneratorManager:
    """管理多个 GeneratorWorker 协程"""

    async def start(self):
        """启动：从数据库加载配置，创建 N 个 worker 协程"""

    async def stop(self):
        """停止所有 worker"""

    async def listen_commands(self):
        """监听 processor:commands 频道"""

    async def reload_config(self):
        """重新加载配置"""

    async def get_status(self):
        """获取运行状态"""
```

**验证**: Python 语法检查

### Task 4: 修改 command_listener.py

**文件**: `worker/core/workers/command_listener.py`

简化队列推送逻辑，移除 group_id：

```python
# 原来：queue_key = f"pending:articles:{target_group}"
# 改为：
queue_key = "pending:articles"
await self.rdb.lpush(queue_key, article_id)
```

**验证**: Python 语法检查

### Task 5: 修改 GeneratorWorker

**文件**: `worker/core/workers/generator_worker.py`

1. 简化为监听单一队列 `pending:articles`
2. 添加重试逻辑：
   - 失败时检查重试次数
   - 未超限则放入 `pending:articles:retry`
   - 超限则放入 `pending:articles:dead`
3. 添加重试队列消费逻辑

**验证**: Python 语法检查

### Task 6: 修改 main.py

**文件**: `worker/main.py`

启动 GeneratorManager：

```python
async def main():
    await init_components()

    listener = CommandListener()
    generator = GeneratorManager()

    await asyncio.gather(
        listener.start(),
        generator.start()
    )
```

**验证**: Python 语法检查

### Task 7: 前端 API 封装

**文件**: `web/src/api/processor.ts`

```typescript
export function getProcessorConfig()
export function updateProcessorConfig(config: ProcessorConfig)
export function getProcessorStatus()
export function startProcessor()
export function stopProcessor()
export function retryAllFailed()
export function clearDeadQueue()
export function getProcessorStats()
```

**验证**: TypeScript 编译检查

### Task 8: 前端页面

**文件**: `web/src/views/processor/ProcessorManage.vue`

页面包含：
1. 状态卡片（待处理队列、重试队列、死信队列、今日处理、处理速度、运行状态）
2. 操作按钮（启动、停止、重试失败、清空死信）
3. 配置表单（启用开关、并发数、重试次数、段落最小长度、批量大小）
4. 每 5 秒自动刷新状态

**验证**: 无编译错误

### Task 9: 添加路由和菜单

**文件**:
- `web/src/router/index.ts`
- 菜单配置文件

添加：
1. 路由 `/processor` -> `ProcessorManage.vue`
2. 顶级菜单"数据加工"

**验证**: 前端可正常访问页面

### Task 10: 数据库初始化数据

**文件**: `migrations/000_init.sql` 或单独的迁移文件

添加默认配置：
```sql
INSERT INTO settings (key, value) VALUES
('processor.enabled', 'true'),
('processor.concurrency', '3'),
('processor.retry_max', '3'),
('processor.min_paragraph_length', '20'),
('processor.batch_size', '50');
```

**验证**: SQL 语法正确

## 文件清单

### 新增文件
- `api/internal/handler/processor.go`
- `worker/core/workers/generator_manager.py`
- `web/src/views/processor/ProcessorManage.vue`
- `web/src/api/processor.ts`

### 修改文件
- `api/internal/handler/router.go`
- `worker/core/workers/command_listener.py`
- `worker/core/workers/generator_worker.py`
- `worker/main.py`
- `web/src/router/index.ts`
- 菜单配置文件
- `migrations/000_init.sql`
