# 标题和正文缓存池实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现生产者-消费者模型的标题和正文缓存池，Python 负责补充，Go 负责消费。

**Architecture:** Redis List 作为中间层，Python PoolFiller 监控队列长度并从 DB 补充数据，Go PoolConsumer 从 Redis 消费并异步更新 DB 状态。

**Tech Stack:** Go (Gin, go-redis, sqlx), Python (asyncio, aioredis, aiomysql), Redis List

---

## Task 1: 创建 Go PoolConsumer 基础结构

**Files:**
- Create: `api/internal/service/pool_consumer.go`

**Step 1: 创建 PoolConsumer 结构和构造函数**

```go
// api/internal/service/pool_consumer.go
package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

var (
	ErrPoolEmpty = errors.New("pool is empty")
)

// PoolItem represents an item in the pool
type PoolItem struct {
	ID   int64  `json:"id"`
	Text string `json:"text"`
}

// UpdateTask represents a status update task
type UpdateTask struct {
	Table string
	ID    int64
}

// PoolConsumer consumes titles and contents from Redis pools
type PoolConsumer struct {
	redis    *redis.Client
	db       *sqlx.DB
	updateCh chan UpdateTask
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewPoolConsumer creates a new pool consumer
func NewPoolConsumer(redisClient *redis.Client, db *sqlx.DB) *PoolConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	return &PoolConsumer{
		redis:    redisClient,
		db:       db,
		updateCh: make(chan UpdateTask, 1000),
		ctx:      ctx,
		cancel:   cancel,
	}
}
```

**Step 2: 验证文件创建成功**

Run: `type api\internal\service\pool_consumer.go | findstr "PoolConsumer"`
Expected: 显示 PoolConsumer 结构定义

**Step 3: Commit**

```bash
git add api/internal/service/pool_consumer.go
git commit -m "feat(api): add PoolConsumer basic structure"
```

---

## Task 2: 实现 PoolConsumer 启动和停止方法

**Files:**
- Modify: `api/internal/service/pool_consumer.go`

**Step 1: 添加 Start 和 Stop 方法**

在文件末尾追加：

```go
// Start starts the async update worker
func (c *PoolConsumer) Start() {
	c.wg.Add(1)
	go c.updateWorker()
	log.Info().Msg("PoolConsumer started")
}

// Stop stops the pool consumer gracefully
func (c *PoolConsumer) Stop() {
	c.cancel()
	close(c.updateCh)
	c.wg.Wait()
	log.Info().Msg("PoolConsumer stopped")
}

// updateWorker processes status update tasks
func (c *PoolConsumer) updateWorker() {
	defer c.wg.Done()

	for task := range c.updateCh {
		select {
		case <-c.ctx.Done():
			return
		default:
			c.processUpdate(task)
		}
	}
}

// processUpdate updates the status of a consumed item
func (c *PoolConsumer) processUpdate(task UpdateTask) {
	query := fmt.Sprintf("UPDATE %s SET status = 0 WHERE id = ?", task.Table)
	_, err := c.db.ExecContext(c.ctx, query, task.ID)
	if err != nil {
		log.Warn().Err(err).
			Str("table", task.Table).
			Int64("id", task.ID).
			Msg("Failed to update status")
	}
}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add api/internal/service/pool_consumer.go
git commit -m "feat(api): add PoolConsumer start/stop methods"
```

---

## Task 3: 实现 PoolConsumer Pop 方法

**Files:**
- Modify: `api/internal/service/pool_consumer.go`

**Step 1: 添加 Pop 方法**

在 `processUpdate` 方法后追加：

```go
// Pop retrieves and removes an item from the pool
func (c *PoolConsumer) Pop(ctx context.Context, poolType string, groupID int) (string, error) {
	if c.redis == nil {
		return "", errors.New("redis client is nil")
	}

	key := fmt.Sprintf("%s:pool:%d", poolType, groupID)

	// RPOP from Redis
	data, err := c.redis.RPop(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrPoolEmpty
	}
	if err != nil {
		return "", fmt.Errorf("redis rpop failed: %w", err)
	}

	// Parse JSON
	var item PoolItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return "", fmt.Errorf("json unmarshal failed: %w", err)
	}

	// Async update status
	select {
	case c.updateCh <- UpdateTask{Table: poolType, ID: item.ID}:
	default:
		log.Warn().Str("table", poolType).Int64("id", item.ID).Msg("Update channel full, dropping task")
	}

	return item.Text, nil
}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add api/internal/service/pool_consumer.go
git commit -m "feat(api): add PoolConsumer Pop method"
```

---

## Task 4: 实现 PoolConsumer 降级方法

**Files:**
- Modify: `api/internal/service/pool_consumer.go`

**Step 1: 添加 PopWithFallback 和 fallbackFromDB 方法**

在 `Pop` 方法后追加：

```go
// PopWithFallback tries Redis first, falls back to DB on failure
func (c *PoolConsumer) PopWithFallback(ctx context.Context, poolType string, groupID int) (string, error) {
	// Try Redis first
	text, err := c.Pop(ctx, poolType, groupID)
	if err == nil {
		return text, nil
	}

	// Fallback to DB on Redis errors
	if err == ErrPoolEmpty || errors.Is(err, redis.Nil) || c.redis == nil {
		log.Debug().Str("pool", poolType).Int("group", groupID).Msg("Falling back to DB")
		return c.fallbackFromDB(ctx, poolType, groupID)
	}

	return "", err
}

// fallbackFromDB queries DB directly when Redis is unavailable
func (c *PoolConsumer) fallbackFromDB(ctx context.Context, poolType string, groupID int) (string, error) {
	column := "title"
	if poolType == "contents" {
		column = "content"
	}

	query := fmt.Sprintf(`
		SELECT id, %s as text FROM %s
		WHERE group_id = ? AND status = 1
		ORDER BY batch_id DESC, id ASC
		LIMIT 1
	`, column, poolType)

	var item PoolItem
	if err := c.db.GetContext(ctx, &item, query, groupID); err != nil {
		return "", fmt.Errorf("db fallback failed: %w", err)
	}

	// Async update status
	select {
	case c.updateCh <- UpdateTask{Table: poolType, ID: item.ID}:
	default:
		log.Warn().Str("table", poolType).Int64("id", item.ID).Msg("Update channel full, dropping task")
	}

	return item.Text, nil
}

// GetPoolLength returns the current length of a pool
func (c *PoolConsumer) GetPoolLength(ctx context.Context, poolType string, groupID int) (int64, error) {
	if c.redis == nil {
		return 0, errors.New("redis client is nil")
	}
	key := fmt.Sprintf("%s:pool:%d", poolType, groupID)
	return c.redis.LLen(ctx, key).Result()
}
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add api/internal/service/pool_consumer.go
git commit -m "feat(api): add PoolConsumer fallback methods"
```

---

## Task 5: 集成 PoolConsumer 到 main.go

**Files:**
- Modify: `api/cmd/main.go:98` (在 dataManager 初始化后)

**Step 1: 添加 PoolConsumer 初始化**

在 `dataManager := core.NewDataManager(...)` 行（约第98行）之后添加：

```go
	// Initialize pool consumer for titles and contents
	var poolConsumer *core.PoolConsumer
	if redisClient != nil {
		poolConsumer = core.NewPoolConsumer(redisClient, db)
		poolConsumer.Start()
		log.Info().Msg("PoolConsumer initialized")
	} else {
		log.Warn().Msg("PoolConsumer disabled: Redis not available")
	}
```

**Step 2: 添加优雅关闭**

在 `defer database.Close()` 行（约第58行）之后添加：

```go
	// 注意：poolConsumer 的 Stop 在 main 函数末尾添加，因为需要在初始化后
```

在 `srv.Shutdown` 调用之后（约第200行附近）添加：

```go
		// Stop pool consumer
		if poolConsumer != nil {
			poolConsumer.Stop()
		}
```

**Step 3: 验证编译通过**

Run: `cd api && go build ./cmd/main.go`
Expected: 无错误输出

**Step 4: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(api): integrate PoolConsumer into main.go"
```

---

## Task 6: 修改 PageHandler 使用 PoolConsumer

**Files:**
- Modify: `api/internal/handler/page.go:22-55` (结构体和构造函数)
- Modify: `api/internal/handler/page.go:137-149` (数据获取部分)

**Step 1: 修改 PageHandler 结构体**

在 `PageHandler` 结构体中添加 `poolConsumer` 字段（约第31行后）：

```go
	poolConsumer     *core.PoolConsumer
```

**Step 2: 修改 NewPageHandler 构造函数**

添加 `poolConsumer` 参数：

```go
func NewPageHandler(
	db *sqlx.DB,
	cfg *config.Config,
	siteCache *core.SiteCache,
	templateCache *core.TemplateCache,
	htmlCache *core.HTMLCache,
	dataManager *core.DataManager,
	funcsManager *core.TemplateFuncsManager,
	poolConsumer *core.PoolConsumer,
) *PageHandler {
	return &PageHandler{
		db:               db,
		cfg:              cfg,
		spiderDetector:   core.GetSpiderDetector(),
		siteCache:        siteCache,
		templateCache:    templateCache,
		htmlCache:        htmlCache,
		dataManager:      dataManager,
		templateRenderer: core.NewTemplateRenderer(funcsManager),
		funcsManager:     funcsManager,
		poolConsumer:     poolConsumer,
	}
}
```

**Step 3: 修改 ServePage 中的数据获取**

替换第137-149行的数据获取代码：

```go
	// Get title and content from pool (or fallback)
	var title, content string
	if h.poolConsumer != nil {
		var err error
		title, err = h.poolConsumer.PopWithFallback(ctx, "titles", articleGroupID)
		if err != nil {
			log.Warn().Err(err).Int("group", articleGroupID).Msg("Failed to get title from pool")
			titles := h.dataManager.GetRandomTitles(articleGroupID, 1)
			if len(titles) > 0 {
				title = titles[0]
			}
		}
		content, err = h.poolConsumer.PopWithFallback(ctx, "contents", articleGroupID)
		if err != nil {
			log.Warn().Err(err).Int("group", articleGroupID).Msg("Failed to get content from pool")
			content = h.dataManager.GetRandomContent(articleGroupID)
		}
	} else {
		// Fallback to dataManager when poolConsumer is not available
		titles := h.dataManager.GetRandomTitles(articleGroupID, 1)
		if len(titles) > 0 {
			title = titles[0]
		}
		content = h.dataManager.GetRandomContent(articleGroupID)
	}

	// 获取关键词用于标题生成（使用关键词分组）
	titleKeywords := h.dataManager.GetRandomKeywords(keywordGroupID, 3)
	fetchTime := time.Since(t4)

	// Build article content using fetched title and content
	articleContent := core.BuildArticleContentFromSingle(title, content)
```

**Step 4: 添加 BuildArticleContentFromSingle 辅助函数**

在 `api/internal/service/article_builder.go` 或相关文件中添加：

```go
// BuildArticleContentFromSingle builds article content from single title and content
func BuildArticleContentFromSingle(title, content string) string {
	if title == "" && content == "" {
		return ""
	}
	if title == "" {
		return content
	}
	if content == "" {
		return title
	}
	return fmt.Sprintf("<h2>%s</h2>\n%s", title, content)
}
```

**Step 5: 验证编译通过**

Run: `cd api && go build ./...`
Expected: 无错误输出

**Step 6: Commit**

```bash
git add api/internal/handler/page.go api/internal/service/
git commit -m "feat(api): use PoolConsumer in PageHandler"
```

---

## Task 7: 更新 PageHandler 初始化调用

**Files:**
- Modify: `api/cmd/main.go` (NewPageHandler 调用处)

**Step 1: 更新 NewPageHandler 调用**

找到 `api.NewPageHandler` 调用（大约在第170-180行），添加 `poolConsumer` 参数：

```go
	pageHandler := api.NewPageHandler(
		db,
		cfg,
		siteCache,
		templateCache,
		htmlCache,
		dataManager,
		funcsManager,
		poolConsumer,
	)
```

**Step 2: 验证编译通过**

Run: `cd api && go build ./cmd/main.go`
Expected: 无错误输出

**Step 3: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(api): pass PoolConsumer to PageHandler"
```

---

## Task 8: 创建 Python PoolFiller 基础结构

**Files:**
- Create: `content_worker/core/pool_filler.py`

**Step 1: 创建 PoolFiller 类**

```python
# -*- coding: utf-8 -*-
"""
缓存池填充器

监控 Redis 队列长度，低于阈值时从数据库补充数据。
"""
import asyncio
import json
from typing import Optional

from loguru import logger


class PoolFiller:
    """标题和正文缓存池填充器"""

    def __init__(
        self,
        redis_client,
        db_pool,
        group_id: int,
        pool_size: int = 5000,
        threshold: int = 1000,
        batch_size: int = 500,
    ):
        """
        初始化填充器

        Args:
            redis_client: Redis 客户端
            db_pool: 数据库连接池
            group_id: 分组 ID
            pool_size: 池大小（默认 5000）
            threshold: 补充阈值（默认 1000，即 20%）
            batch_size: 每次补充数量（默认 500）
        """
        self.redis = redis_client
        self.db = db_pool
        self.group_id = group_id
        self.pool_size = pool_size
        self.threshold = threshold
        self.batch_size = batch_size

    def _get_key(self, pool_type: str) -> str:
        """获取 Redis key"""
        return f"{pool_type}:pool:{self.group_id}"

    def _get_lock_key(self, pool_type: str) -> str:
        """获取补充锁 key"""
        return f"{self._get_key(pool_type)}:filling"
```

**Step 2: 验证文件语法正确**

Run: `python -m py_compile content_worker/core/pool_filler.py`
Expected: 无输出（表示无语法错误）

**Step 3: Commit**

```bash
git add content_worker/core/pool_filler.py
git commit -m "feat(worker): add PoolFiller basic structure"
```

---

## Task 9: 实现 PoolFiller check_and_fill 方法

**Files:**
- Modify: `content_worker/core/pool_filler.py`

**Step 1: 添加 check_and_fill 方法**

在 `PoolFiller` 类末尾追加：

```python
    async def check_and_fill(self, pool_type: str) -> int:
        """
        检查队列长度，低于阈值时补充

        Args:
            pool_type: 池类型 ("titles" 或 "contents")

        Returns:
            补充的数据条数
        """
        key = self._get_key(pool_type)
        lock_key = self._get_lock_key(pool_type)

        # 1. 检查队列长度
        length = await self.redis.llen(key)
        if length >= self.threshold:
            return 0

        # 2. 尝试获取补充锁（防止并发）
        acquired = await self.redis.set(lock_key, "1", nx=True, ex=60)
        if not acquired:
            logger.debug(f"[{pool_type}:{self.group_id}] Another filler is working")
            return 0

        try:
            # 3. 计算需要补充的数量
            need = min(self.pool_size - length, self.batch_size)
            logger.info(f"[{pool_type}:{self.group_id}] Pool low ({length}/{self.threshold}), filling {need} items")

            # 4. 从 DB 查询可用数据
            items = await self._fetch_available(pool_type, need)
            if not items:
                logger.warning(f"[{pool_type}:{self.group_id}] No available items in DB")
                return 0

            # 5. 批量入队
            filled = await self._push_to_queue(key, items)
            logger.info(f"[{pool_type}:{self.group_id}] Filled {filled} items, new length: {length + filled}")
            return filled

        finally:
            await self.redis.delete(lock_key)
```

**Step 2: 验证文件语法正确**

Run: `python -m py_compile content_worker/core/pool_filler.py`
Expected: 无输出

**Step 3: Commit**

```bash
git add content_worker/core/pool_filler.py
git commit -m "feat(worker): add PoolFiller check_and_fill method"
```

---

## Task 10: 实现 PoolFiller 数据库查询和入队方法

**Files:**
- Modify: `content_worker/core/pool_filler.py`

**Step 1: 添加 _fetch_available 和 _push_to_queue 方法**

在 `check_and_fill` 方法后追加：

```python
    async def _fetch_available(self, pool_type: str, limit: int) -> list[dict]:
        """
        从数据库查询可用数据

        Args:
            pool_type: 池类型 ("titles" 或 "contents")
            limit: 查询数量

        Returns:
            数据列表 [{"id": int, "text": str}, ...]
        """
        column = "title" if pool_type == "titles" else "content"
        query = f"""
            SELECT id, {column} as text FROM {pool_type}
            WHERE group_id = %s AND status = 1
            ORDER BY batch_id DESC, id ASC
            LIMIT %s
        """

        async with self.db.acquire() as conn:
            async with conn.cursor() as cur:
                await cur.execute(query, (self.group_id, limit))
                rows = await cur.fetchall()
                return [{"id": row[0], "text": row[1]} for row in rows]

    async def _push_to_queue(self, key: str, items: list[dict]) -> int:
        """
        批量入队到 Redis

        Args:
            key: Redis key
            items: 数据列表

        Returns:
            成功入队的数量
        """
        if not items:
            return 0

        # 使用 pipeline 批量 LPUSH
        pipe = self.redis.pipeline()
        for item in items:
            data = json.dumps(item, ensure_ascii=False)
            pipe.lpush(key, data)

        await pipe.execute()
        return len(items)

    async def get_pool_stats(self, pool_type: str) -> dict:
        """获取池状态统计"""
        key = self._get_key(pool_type)
        length = await self.redis.llen(key)
        return {
            "pool_type": pool_type,
            "group_id": self.group_id,
            "length": length,
            "pool_size": self.pool_size,
            "threshold": self.threshold,
            "utilization": round(length / self.pool_size * 100, 2) if self.pool_size > 0 else 0,
        }
```

**Step 2: 验证文件语法正确**

Run: `python -m py_compile content_worker/core/pool_filler.py`
Expected: 无输出

**Step 3: Commit**

```bash
git add content_worker/core/pool_filler.py
git commit -m "feat(worker): add PoolFiller DB fetch and queue push methods"
```

---

## Task 11: 创建 PoolFillerManager 管理多个分组

**Files:**
- Modify: `content_worker/core/pool_filler.py`

**Step 1: 添加 PoolFillerManager 类**

在文件末尾追加：

```python
class PoolFillerManager:
    """管理多个分组的缓存池填充"""

    def __init__(self, redis_client, db_pool):
        self.redis = redis_client
        self.db = db_pool
        self.fillers: dict[int, PoolFiller] = {}
        self._running = False
        self._task: Optional[asyncio.Task] = None

    def add_group(self, group_id: int, **kwargs) -> None:
        """添加一个分组的填充器"""
        self.fillers[group_id] = PoolFiller(
            self.redis, self.db, group_id, **kwargs
        )
        logger.info(f"Added PoolFiller for group {group_id}")

    async def start(self, check_interval: float = 5.0) -> None:
        """启动填充循环"""
        if self._running:
            return

        self._running = True
        self._task = asyncio.create_task(self._fill_loop(check_interval))
        logger.info(f"PoolFillerManager started with {len(self.fillers)} groups")

    async def stop(self) -> None:
        """停止填充循环"""
        self._running = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        logger.info("PoolFillerManager stopped")

    async def _fill_loop(self, interval: float) -> None:
        """填充循环"""
        while self._running:
            for group_id, filler in self.fillers.items():
                try:
                    await filler.check_and_fill("titles")
                    await filler.check_and_fill("contents")
                except Exception as e:
                    logger.error(f"Fill error for group {group_id}: {e}")

            await asyncio.sleep(interval)

    async def fill_all_now(self) -> dict[int, dict]:
        """立即填充所有分组（启动时调用）"""
        results = {}
        for group_id, filler in self.fillers.items():
            try:
                titles_filled = await filler.check_and_fill("titles")
                contents_filled = await filler.check_and_fill("contents")
                results[group_id] = {
                    "titles": titles_filled,
                    "contents": contents_filled,
                }
            except Exception as e:
                logger.error(f"Initial fill error for group {group_id}: {e}")
                results[group_id] = {"error": str(e)}
        return results

    async def get_all_stats(self) -> dict[int, dict]:
        """获取所有分组的池状态"""
        stats = {}
        for group_id, filler in self.fillers.items():
            stats[group_id] = {
                "titles": await filler.get_pool_stats("titles"),
                "contents": await filler.get_pool_stats("contents"),
            }
        return stats
```

**Step 2: 验证文件语法正确**

Run: `python -m py_compile content_worker/core/pool_filler.py`
Expected: 无输出

**Step 3: Commit**

```bash
git add content_worker/core/pool_filler.py
git commit -m "feat(worker): add PoolFillerManager for multi-group support"
```

---

## Task 12: 集成 PoolFillerManager 到 main.py

**Files:**
- Modify: `content_worker/main.py`

**Step 1: 导入并初始化 PoolFillerManager**

在 `from core.pool_reloader import ...` 行（约第103行）后添加导入：

```python
    from core.pool_filler import PoolFillerManager
```

在 `pool_reloader = await start_pool_reloader()` 行（约第104行）后添加初始化：

```python
    # 初始化缓存池填充器
    from database.db import get_db_pool
    from core.redis_client import get_redis_client

    pool_filler_manager = PoolFillerManager(
        redis_client=get_redis_client(),
        db_pool=get_db_pool(),
    )

    # 添加默认分组（group_id=1）
    pool_filler_manager.add_group(1)

    # 启动时立即填充
    logger.info("初始化缓存池...")
    await pool_filler_manager.fill_all_now()

    # 启动填充循环
    await pool_filler_manager.start(check_interval=5.0)
```

**Step 2: 添加清理逻辑**

在 `finally` 块中（约第147行），`await stop_pool_reloader()` 之后添加：

```python
        await pool_filler_manager.stop()
```

**Step 3: 验证文件语法正确**

Run: `python -m py_compile content_worker/main.py`
Expected: 无输出

**Step 4: Commit**

```bash
git add content_worker/main.py
git commit -m "feat(worker): integrate PoolFillerManager into main.py"
```

---

## Task 13: 添加获取活跃分组的函数

**Files:**
- Modify: `content_worker/main.py`

**Step 1: 添加获取活跃分组函数**

在 `pool_filler_manager.add_group(1)` 调用前，替换为动态获取：

```python
    # 获取活跃的分组 ID
    async def get_active_group_ids():
        async with get_db_pool().acquire() as conn:
            async with conn.cursor() as cur:
                # 查询有数据的分组
                await cur.execute("""
                    SELECT DISTINCT group_id FROM (
                        SELECT group_id FROM titles WHERE status = 1
                        UNION
                        SELECT group_id FROM contents WHERE status = 1
                    ) t
                """)
                rows = await cur.fetchall()
                return [row[0] for row in rows] if rows else [1]

    group_ids = await get_active_group_ids()
    logger.info(f"发现 {len(group_ids)} 个活跃分组: {group_ids}")

    # 为每个分组添加填充器
    for gid in group_ids:
        pool_filler_manager.add_group(gid)
```

**Step 2: 验证文件语法正确**

Run: `python -m py_compile content_worker/main.py`
Expected: 无输出

**Step 3: Commit**

```bash
git add content_worker/main.py
git commit -m "feat(worker): add dynamic group discovery for PoolFiller"
```

---

## Task 14: 更新 __init__.py 导出

**Files:**
- Modify: `content_worker/core/__init__.py`

**Step 1: 添加导出**

在文件中添加：

```python
from core.pool_filler import PoolFiller, PoolFillerManager
```

**Step 2: 验证导入正常**

Run: `cd content_worker && python -c "from core import PoolFiller, PoolFillerManager; print('OK')"`
Expected: `OK`

**Step 3: Commit**

```bash
git add content_worker/core/__init__.py
git commit -m "feat(worker): export PoolFiller classes"
```

---

## Task 15: 整体验证

**Step 1: 验证 Go 编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 2: 验证 Python 语法**

Run: `cd content_worker && python -m py_compile main.py core/pool_filler.py`
Expected: 无输出

**Step 3: 最终提交**

```bash
git add -A
git commit -m "feat: complete title/content pool producer-consumer implementation

- Go PoolConsumer: consumes from Redis, async DB status update
- Python PoolFiller: monitors queue, fills from DB when low
- Multi-group support with dynamic discovery
- Fallback to DB when Redis unavailable"
```

---

## 文件清单

### 新增文件
- `api/internal/service/pool_consumer.go` - Go 消费者
- `content_worker/core/pool_filler.py` - Python 生产者

### 修改文件
- `api/cmd/main.go` - 初始化 PoolConsumer
- `api/internal/handler/page.go` - 使用 PoolConsumer
- `content_worker/main.py` - 集成 PoolFillerManager
- `content_worker/core/__init__.py` - 导出新类
