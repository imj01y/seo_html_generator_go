# 标题和正文缓存池设计

## 概述

将标题和正文的缓存机制从"只读随机选取"改为"生产者-消费者模型"，实现数据的顺序消费和自动补充。

## 设计决策

| 项目 | 决策 |
|-----|------|
| 消费后处理 | 标记 `status=0`，数据保留在数据库 |
| 补充触发 | 阈值触发，低于 20% 时补充 |
| 池大小 | 标题 5000 / 正文 5000 |
| 补充阈值 | 低于 1000 条时触发 |
| 数据优先级 | 按 `batch_id DESC` 优先最新 |
| 架构 | Redis 中间层（Python 生产，Go 消费） |
| Redis 结构 | List（LPUSH 生产，RPOP 消费） |
| 状态更新 | Go 消费后异步立即更新 DB |
| 分组策略 | 每个分组独立队列 |

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Redis (中间层)                            │
│  ┌─────────────────────┐    ┌─────────────────────┐            │
│  │ titles:pool:1       │    │ contents:pool:1     │            │
│  │ titles:pool:2       │    │ contents:pool:2     │            │
│  │ ...                 │    │ ...                 │            │
│  └─────────────────────┘    └─────────────────────┘            │
│         ↑ LPUSH                      ↑ LPUSH                    │
│         │                            │                          │
└─────────┼────────────────────────────┼──────────────────────────┘
          │                            │
    ┌─────┴─────┐                ┌─────┴─────┐
    │  Python   │                │  Python   │
    │ 生产者    │                │ 生产者    │
    │ (补充池)  │                │ (补充池)  │
    └───────────┘                └───────────┘
          ↑ 查询 status=1              ↑
          │                            │
    ┌─────┴─────────────────────────────┴─────┐
    │              MySQL                       │
    │  titles 表 / contents 表                 │
    │  (status: 1=可用, 0=已使用)              │
    └─────────────────────────────────────────┘
          ↑ UPDATE status=0 (异步)
          │
    ┌─────┴─────┐
    │   Go API  │ ←── RPOP 消费
    │  消费者   │
    └───────────┘
```

**核心流程：**
1. **Python 生产者**：监控 Redis 队列长度，低于阈值时从 DB 查询 `status=1` 的数据补充到队列
2. **Go 消费者**：从 Redis RPOP 获取数据，使用后异步更新 DB `status=0`
3. **数据一致性**：通过 DB 的 status 字段保证数据不会重复使用

## Redis 数据结构

### 队列 Key 命名

```
titles:pool:{group_id}      # 标题队列
contents:pool:{group_id}    # 正文队列
```

### 队列中的数据格式

使用 JSON 存储，包含 ID 用于状态更新：

```json
{
  "id": 12345,
  "text": "这是标题或正文内容..."
}
```

### 辅助 Key

```
titles:pool:{group_id}:filling    # 补充锁（防止并发补充）
contents:pool:{group_id}:filling  # 补充锁
```

### 操作命令

| 操作 | 命令 | 说明 |
|-----|------|------|
| 生产 | `LPUSH titles:pool:1 {json}` | 从左侧入队 |
| 消费 | `RPOP titles:pool:1` | 从右侧出队（FIFO） |
| 查看长度 | `LLEN titles:pool:1` | 判断是否需要补充 |
| 补充锁 | `SET ... NX EX 60` | 防止并发补充，60秒过期 |

## Python 生产者

### 核心类：PoolFiller

```python
class PoolFiller:
    def __init__(self, redis, db_pool, group_id: int):
        self.redis = redis
        self.db = db_pool
        self.group_id = group_id
        self.pool_size = 5000       # 池大小
        self.threshold = 1000       # 补充阈值（20%）
        self.batch_size = 500       # 每次补充数量

    async def check_and_fill(self, pool_type: str):
        """检查并补充队列"""
        key = f"{pool_type}:pool:{self.group_id}"
        lock_key = f"{key}:filling"

        # 1. 检查队列长度
        length = await self.redis.llen(key)
        if length >= self.threshold:
            return

        # 2. 获取补充锁（防止并发）
        if not await self.redis.set(lock_key, "1", nx=True, ex=60):
            return

        try:
            # 3. 计算需要补充的数量
            need = self.pool_size - length

            # 4. 从 DB 查询可用数据
            items = await self.fetch_available(pool_type, need)

            # 5. 批量入队
            if items:
                await self.push_to_queue(key, items)
        finally:
            await self.redis.delete(lock_key)
```

### 数据库查询

```sql
-- 查询可用的标题/正文，优先最新批次
SELECT id, title as text FROM titles
WHERE group_id = ? AND status = 1
ORDER BY batch_id DESC, id ASC
LIMIT ?
```

### 运行方式

在 Python Worker 生命周期中启动后台任务，定期检查：

```python
async def pool_filler_loop(fillers: list[PoolFiller]):
    while True:
        for filler in fillers:
            try:
                await filler.check_and_fill("titles")
                await filler.check_and_fill("contents")
            except Exception as e:
                log.error(f"Fill error for group {filler.group_id}: {e}")

        await asyncio.sleep(5)
```

## Go 消费者

### 核心结构：PoolConsumer

```go
type PoolConsumer struct {
    redis    *redis.Client
    db       *sqlx.DB
    updateCh chan UpdateTask  // 异步更新通道
}

type PoolItem struct {
    ID   int64  `json:"id"`
    Text string `json:"text"`
}

type UpdateTask struct {
    Table   string  // "titles" 或 "contents"
    ID      int64
}
```

### 消费方法

```go
func (c *PoolConsumer) Pop(ctx context.Context, poolType string, groupID int) (string, error) {
    key := fmt.Sprintf("%s:pool:%d", poolType, groupID)

    // 1. 从 Redis RPOP
    data, err := c.redis.RPop(ctx, key).Result()
    if err == redis.Nil {
        return "", ErrPoolEmpty
    }
    if err != nil {
        return "", err
    }

    // 2. 解析 JSON
    var item PoolItem
    if err := json.Unmarshal([]byte(data), &item); err != nil {
        return "", err
    }

    // 3. 异步更新数据库状态
    c.updateCh <- UpdateTask{Table: poolType, ID: item.ID}

    // 4. 返回文本内容
    return item.Text, nil
}
```

### 异步状态更新

```go
func (c *PoolConsumer) startUpdateWorker(ctx context.Context) {
    go func() {
        for {
            select {
            case task := <-c.updateCh:
                query := fmt.Sprintf(
                    "UPDATE %s SET status = 0 WHERE id = ?",
                    task.Table,
                )
                c.db.ExecContext(ctx, query, task.ID)
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

### 在 page.go 中使用

```go
// 替换原有的 DataManager 调用
func (h *PageHandler) ServePage(c *gin.Context) {
    // ...

    // 获取标题（顺序消费）
    title, err := h.poolConsumer.Pop(ctx, "titles", articleGroupID)
    if err != nil {
        // 降级处理
    }

    // 获取正文（顺序消费）
    content, err := h.poolConsumer.Pop(ctx, "contents", articleGroupID)
    if err != nil {
        // 降级处理
    }

    // ...
}
```

## 错误处理与降级

### 场景分析

| 场景 | 处理方式 |
|-----|---------|
| Redis 队列为空 | 降级到 DB 直接查询 |
| Redis 连接失败 | 降级到 DB 直接查询 |
| DB 无可用数据 | 返回错误，页面降级处理 |
| 异步更新失败 | 记录日志，不影响主流程 |

### 降级查询实现

```go
func (c *PoolConsumer) PopWithFallback(ctx context.Context, poolType string, groupID int) (string, error) {
    // 1. 优先从 Redis 获取
    text, err := c.Pop(ctx, poolType, groupID)
    if err == nil {
        return text, nil
    }

    // 2. Redis 失败，降级到 DB 直接查询
    if err == ErrPoolEmpty || err == redis.Nil || isRedisError(err) {
        return c.fallbackFromDB(ctx, poolType, groupID)
    }

    return "", err
}

func (c *PoolConsumer) fallbackFromDB(ctx context.Context, poolType string, groupID int) (string, error) {
    table := poolType  // "titles" or "contents"
    column := "title"
    if poolType == "contents" {
        column = "content"
    }

    query := fmt.Sprintf(`
        SELECT id, %s as text FROM %s
        WHERE group_id = ? AND status = 1
        ORDER BY batch_id DESC, id ASC
        LIMIT 1
    `, column, table)

    var item PoolItem
    if err := c.db.GetContext(ctx, &item, query, groupID); err != nil {
        return "", err
    }

    // 异步更新状态
    c.updateCh <- UpdateTask{Table: table, ID: item.ID}

    return item.Text, nil
}
```

### 异步更新失败处理

```go
func (c *PoolConsumer) startUpdateWorker(ctx context.Context) {
    go func() {
        for task := range c.updateCh {
            query := fmt.Sprintf(
                "UPDATE %s SET status = 0 WHERE id = ?",
                task.Table,
            )
            _, err := c.db.ExecContext(ctx, query, task.ID)
            if err != nil {
                // 仅记录日志，不重试（下次补充时会跳过已消费的）
                log.Warn().Err(err).
                    Str("table", task.Table).
                    Int64("id", task.ID).
                    Msg("Failed to update status")
            }
        }
    }()
}
```

## 初始化与生命周期

### Go 端初始化

```go
// api/cmd/main.go

func main() {
    // ...

    // 创建 PoolConsumer
    poolConsumer := core.NewPoolConsumer(redisClient, db)

    // 启动异步更新 worker
    poolConsumer.Start(ctx)

    // 注入到 PageHandler
    pageHandler := handler.NewPageHandler(
        // ...
        poolConsumer,
    )

    // 优雅关闭
    defer poolConsumer.Stop()
}
```

### Python 端初始化

```python
# content_worker/main.py

async def main():
    # ...

    # 获取需要维护的 group_ids
    group_ids = await get_active_group_ids(db)

    # 为每个分组创建 PoolFiller
    fillers = [PoolFiller(redis, db, gid) for gid in group_ids]

    # 启动填充循环
    fill_task = asyncio.create_task(pool_filler_loop(fillers))

    # ...

    # 关闭时取消
    fill_task.cancel()
```

### 填充循环（支持多分组）

```python
async def pool_filler_loop(fillers: list[PoolFiller]):
    while True:
        for filler in fillers:
            try:
                await filler.check_and_fill("titles")
                await filler.check_and_fill("contents")
            except Exception as e:
                log.error(f"Fill error for group {filler.group_id}: {e}")

        await asyncio.sleep(5)
```

### 启动时预填充

```python
async def startup_fill(fillers: list[PoolFiller]):
    """启动时确保所有队列有足够数据"""
    for filler in fillers:
        await filler.check_and_fill("titles")
        await filler.check_and_fill("contents")
    log.info("Initial pool fill completed")
```

## 文件改动清单

### 新增文件

| 文件 | 说明 |
|-----|------|
| `api/internal/service/pool_consumer.go` | Go 消费者实现 |
| `content_worker/core/pool_filler.py` | Python 生产者实现 |

### 修改文件

| 文件 | 改动 |
|-----|------|
| `api/cmd/main.go` | 初始化 PoolConsumer |
| `api/internal/handler/page.go` | 使用 PoolConsumer 替代 DataManager 获取标题/正文 |
| `api/internal/service/data_manager.go` | 移除 titles/contents 相关代码（保留 keywords/images） |
| `content_worker/main.py` | 启动 PoolFiller 循环 |
| `content_worker/core/lifecycle.py` | 集成 PoolFiller 生命周期 |

### 可删除文件

| 文件 | 原因 |
|-----|------|
| `content_worker/core/title_manager.py` | 被 PoolFiller 替代 |
| `content_worker/core/content_manager.py` | 被 PoolFiller 替代 |

## 设计总结

```
┌──────────────────────────────────────────────────────────────┐
│                     生产者-消费者模型                          │
├──────────────────────────────────────────────────────────────┤
│  Python PoolFiller          Redis List         Go PoolConsumer│
│  ┌─────────────────┐    ┌───────────────┐    ┌─────────────┐ │
│  │ 监控队列长度     │    │titles:pool:1  │    │ RPOP 消费   │ │
│  │ < 1000 时补充   │───▶│contents:pool:1│◀───│ 返回 text   │ │
│  │ 从 DB 查 status=1│    │ (JSON: id+text)│    │ 异步更新 DB │ │
│  └─────────────────┘    └───────────────┘    └─────────────┘ │
│         │                                           │         │
│         ▼                                           ▼         │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │                    MySQL                                 │ │
│  │  titles / contents 表 (status: 1=可用, 0=已使用)         │ │
│  └─────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

| 特性 | 值 |
|-----|-----|
| 池大小 | 5000 条 |
| 补充阈值 | 1000 条 (20%) |
| 数据优先级 | batch_id DESC |
| 消费方式 | FIFO 顺序 |
| 状态更新 | 异步立即 |
| 分组策略 | 独立队列 |
| 降级策略 | DB 直接查询 |
