# 数据加工统计显示修复 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复数据加工管理页面的统计数据全部显示为 0 的问题

**Architecture:** Python Worker 的 `_update_stats_loop()` 方法缺失向 Redis 写入 `processor:stats` Hash 和 `speed` 字段的逻辑。需要补充完整的统计数据写入，确保与 Go 后端 API 期望的数据结构一致。

**Tech Stack:** Python 3.11 (asyncio), Redis, Go (Gin)

---

## 问题根因

| 问题 | 影响字段 | 原因 |
|------|---------|------|
| `processor:stats` Hash 从未写入 | total_processed, total_failed, total_retried, avg_processing_ms | Python Worker 只写入 `processor:status`，未写入 `processor:stats` |
| `speed` 字段缺失 | 处理速度 | `_update_stats_loop()` 未计算和写入处理速度 |

### Redis Key 对应关系

**Go 后端期望读取：**
- `processor:status` → running, workers, processed_total, processed_today, **speed**
- `processor:stats` → **total_processed, total_failed, total_retried, avg_processing_ms**

**Python Worker 当前写入：**
- `processor:status` → running, workers, processed_total, processed_today (缺 speed)
- `processor:stats` → 从未写入

---

## Task 1: 修改 GeneratorWorker 添加处理时间统计

**Files:**
- Modify: `worker/core/workers/generator_worker.py:80-84` (添加统计字段)
- Modify: `worker/core/workers/generator_worker.py:420-467` (process_article 添加计时)
- Modify: `worker/core/workers/generator_worker.py:581-590` (get_stats 返回新字段)

**Step 1: 添加处理时间统计字段**

在 `generator_worker.py` 第 80-84 行的统计字段后添加：

```python
        # 统计
        self._processed_count = 0
        self._failed_count = 0
        self._retried_count = 0
        self._total_processing_time_ms = 0.0  # 新增：累计处理时间（毫秒）
```

**Step 2: 在 process_article 方法中添加计时**

修改 `process_article` 方法（第 420-467 行），在方法开头和成功处理后添加计时：

```python
    async def process_article(self, article: Dict[str, Any]) -> bool:
        """
        处理单篇文章

        1. 提取标题 → titles 表
        2. 拆分正文 → 拼音标注 → contents 表

        Args:
            article: {'id': int, 'title': str, 'content': str, 'group_id': int}

        Returns:
            是否处理成功
        """
        import time
        start_time = time.perf_counter()

        group_id = article.get('group_id', 1)
        title = article.get('title', '')
        content = article.get('content', '')

        try:
            # 1. 保存标题
            if title:
                await self.save_title(title, group_id)

            # 2. 处理正文：拆分段落 → 清理 → 拼音标注 → 保存
            if content:
                # 按换行拆分段落
                paragraphs = content.split('\n') if isinstance(content, str) else []
                # 清理（过滤太短的段落）
                paragraphs = self.cleaner.clean_paragraphs(paragraphs)

                for para in paragraphs:
                    # 拼音标注
                    annotated = self.annotator.annotate(para)
                    # 保存到 contents 表
                    await self.save_content(annotated, group_id)

            self._processed_count += 1

            # 记录处理时间
            elapsed_ms = (time.perf_counter() - start_time) * 1000
            self._total_processing_time_ms += elapsed_ms

            # 清除重试计数（如果有）
            await self.clear_retry_count(article['id'])

            # 更新今日处理量
            await self._update_daily_stats()

            return True

        except Exception as e:
            logger.error(f"Failed to process article {article.get('id')}: {e}")
            return False
```

**Step 3: 更新 get_stats 方法返回新字段**

修改 `get_stats` 方法（第 581-590 行）：

```python
    def get_stats(self) -> dict:
        """获取统计信息"""
        avg_ms = 0.0
        if self._processed_count > 0:
            avg_ms = self._total_processing_time_ms / self._processed_count

        return {
            'processed': self._processed_count,
            'failed': self._failed_count,
            'retried': self._retried_count,
            'total_processing_time_ms': self._total_processing_time_ms,
            'avg_processing_ms': avg_ms,
            'title_buffer_size': len(self._title_buffer),
            'content_buffer_size': len(self._content_buffer),
            'running': self._running
        }
```

**Step 4: Commit**

```bash
git add worker/core/workers/generator_worker.py
git commit -m "feat(worker): add processing time tracking to GeneratorWorker"
```

---

## Task 2: 修改 GeneratorManager 写入完整统计数据

**Files:**
- Modify: `worker/core/workers/generator_manager.py:31-39` (添加速度计算字段)
- Modify: `worker/core/workers/generator_manager.py:267-303` (_update_stats_loop 重写)

**Step 1: 添加速度计算所需的字段**

在 `generator_manager.py` 第 31-39 行的 `__init__` 方法中添加：

```python
    def __init__(self):
        self.workers: List[asyncio.Task] = []
        self.worker_instances: List[GeneratorWorker] = []
        self.config: Dict = {}
        self.running = False
        self.rdb = None
        self.db_pool = None
        self._stop_event = asyncio.Event()
        self._stats_task: Optional[asyncio.Task] = None
        # 速度计算
        self._last_processed_count = 0
        self._last_stats_time = None
```

**Step 2: 重写 _update_stats_loop 方法**

完全重写 `_update_stats_loop` 方法（第 267-303 行），添加 `processor:stats` 写入和速度计算：

```python
    async def _update_stats_loop(self):
        """定期更新统计信息到 Redis"""
        import time
        self._last_stats_time = time.time()
        self._last_processed_count = 0

        while self.running:
            try:
                current_time = time.time()

                # 汇总所有 Worker 的统计
                total_processed = 0
                total_failed = 0
                total_retried = 0
                total_processing_time_ms = 0.0

                for worker in self.worker_instances:
                    stats = worker.get_stats()
                    total_processed += stats.get('processed', 0)
                    total_failed += stats.get('failed', 0)
                    total_retried += stats.get('retried', 0)
                    total_processing_time_ms += stats.get('total_processing_time_ms', 0.0)

                # 计算处理速度（条/秒）
                time_elapsed = current_time - self._last_stats_time
                processed_delta = total_processed - self._last_processed_count
                speed = processed_delta / time_elapsed if time_elapsed > 0 else 0.0

                # 更新上次记录
                self._last_stats_time = current_time
                self._last_processed_count = total_processed

                # 计算平均处理时间
                avg_processing_ms = 0.0
                if total_processed > 0:
                    avg_processing_ms = total_processing_time_ms / total_processed

                # 获取今日处理量（从 Redis）
                today_key = f"processor:processed:{datetime.now().strftime('%Y%m%d')}"
                today_count = await self.rdb.get(today_key)
                processed_today = int(today_count) if today_count else 0

                # 1. 更新 processor:status（状态信息）
                status = {
                    "running": "true" if self.running else "false",
                    "workers": str(len(self.workers)),
                    "processed_total": str(total_processed),
                    "processed_today": str(processed_today),
                    "speed": f"{speed:.2f}",
                    "updated_at": datetime.now().isoformat(),
                }
                await self.rdb.hset("processor:status", mapping=status)

                # 2. 更新 processor:stats（统计信息）
                stats_data = {
                    "total_processed": str(total_processed),
                    "total_failed": str(total_failed),
                    "total_retried": str(total_retried),
                    "avg_processing_ms": f"{avg_processing_ms:.2f}",
                    "updated_at": datetime.now().isoformat(),
                }
                await self.rdb.hset("processor:stats", mapping=stats_data)

                # 每 5 秒更新一次
                await asyncio.sleep(5)

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"更新统计失败: {e}")
                await asyncio.sleep(5)
```

**Step 3: Commit**

```bash
git add worker/core/workers/generator_manager.py
git commit -m "feat(worker): write complete stats to processor:status and processor:stats"
```

---

## Task 3: 验证修复

**Step 1: 检查 Redis 数据结构**

启动 Worker 后，通过 Redis CLI 验证数据：

```bash
# 检查 processor:status
redis-cli HGETALL processor:status
# 期望输出包含: running, workers, processed_total, processed_today, speed, updated_at

# 检查 processor:stats
redis-cli HGETALL processor:stats
# 期望输出包含: total_processed, total_failed, total_retried, avg_processing_ms, updated_at

# 检查队列
redis-cli LLEN pending:articles
redis-cli LLEN pending:articles:retry
redis-cli LLEN pending:articles:dead
```

**Step 2: 验证前端页面**

1. 打开数据加工管理页面
2. 点击"启动"按钮
3. 等待 5-10 秒（统计循环更新周期）
4. 验证以下字段不再为 0：
   - 处理速度（speed）
   - 累计处理文章（total_processed）
   - 累计失败（total_failed）
   - 累计重试（total_retried）
   - 平均处理时间（avg_processing_ms）

**Step 3: 最终 Commit**

```bash
git add -A
git commit -m "fix(processor): complete stats data flow from worker to frontend

- Add processing time tracking in GeneratorWorker
- Write processor:stats Hash in GeneratorManager
- Add speed calculation to processor:status
- All stats now properly displayed on frontend"
```

---

## 数据流修复后

```
GeneratorWorker 处理文章
    ↓
统计 processed, failed, retried, total_processing_time_ms
    ↓
GeneratorManager._update_stats_loop() 汇总
    ↓
写入 Redis processor:status (含 speed)
写入 Redis processor:stats (含 total_processed, total_failed, total_retried, avg_processing_ms)
    ↓
Go API GetStatus() 读取 processor:status
Go API GetStats() 读取 processor:stats
    ↓
前端正确显示所有统计数据
```
