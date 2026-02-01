# 数据处理监控面板 WebSocket 实时状态实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将数据处理监控面板从定时轮询改为 WebSocket 实时推送

**Architecture:** Worker 处理完任务后发布完整状态到 Redis 频道，Go 后端订阅转发，前端 WebSocket 接收更新

**Tech Stack:** Python asyncio, Go gin/gorilla-websocket, Vue 3 Composition API, Redis Pub/Sub

---

## Task 1: Go 后端 - 新增 WebSocket 端点

**Files:**
- Modify: `api/internal/handler/websocket.go`
- Modify: `api/internal/handler/router.go`

**Step 1: 在 websocket.go 添加 ProcessorStatus 方法**

在文件末尾添加：

```go
// ProcessorStatus 数据处理状态实时推送
// GET /ws/processor-status
func (h *WebSocketHandler) ProcessorStatus(c *gin.Context) {
	rdb, exists := c.Get("redis")
	if !exists {
		c.JSON(500, gin.H{"success": false, "message": "Redis未连接"})
		return
	}
	redisClient := rdb.(*redis.Client)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	subscribeAndForward(conn, redisClient, "processor:status:realtime")
}
```

**Step 2: 在 router.go 添加路由**

在 WebSocket routes 区块（约第 356 行 `/ws/processor-logs` 之后）添加：

```go
r.GET("/ws/processor-status", wsHandler.ProcessorStatus)
```

**Step 3: 验证编译**

Run: `cd api && go build ./...`
Expected: 无错误

**Step 4: 提交**

```bash
git add api/internal/handler/websocket.go api/internal/handler/router.go
git commit -m "feat(api): 添加 /ws/processor-status WebSocket 端点"
```

---

## Task 2: Python Worker - 添加回调机制

**Files:**
- Modify: `content_worker/core/workers/generator_worker.py`

**Step 1: 在 __init__ 中添加 on_complete 参数**

在 `__init__` 方法参数列表中添加（约第 53 行 `log=None,` 之后）：

```python
        on_complete=None,
```

在初始化代码中添加（约第 73 行 `self.log = log` 之后）：

```python
        self.on_complete = on_complete  # 任务完成回调
```

**Step 2: 在 process_with_retry 完成后调用回调**

修改 `process_with_retry` 方法（约第 494-519 行），在 return 之前调用回调：

```python
    async def process_with_retry(self, article_id: int) -> bool:
        """
        处理文章，带重试逻辑

        Args:
            article_id: 文章ID

        Returns:
            是否处理成功
        """
        try:
            article = await self.get_article_by_id(article_id)
            if article is None:
                logger.warning(f"Article {article_id} not found in database")
                if self.on_complete:
                    await self.on_complete()
                return False

            success = await self.process_article(article)
            if not success:
                await self.handle_failure(article_id, "Processing failed")
                if self.on_complete:
                    await self.on_complete()
                return False

            if self.on_complete:
                await self.on_complete()
            return True

        except Exception as e:
            await self.handle_failure(article_id, str(e))
            if self.on_complete:
                await self.on_complete()
            return False
```

**Step 3: 验证语法**

Run: `python -m py_compile content_worker/core/workers/generator_worker.py`
Expected: 无输出（无错误）

**Step 4: 提交**

```bash
git add content_worker/core/workers/generator_worker.py
git commit -m "feat(worker): 添加任务完成回调机制"
```

---

## Task 3: Python Manager - 添加实时状态发布

**Files:**
- Modify: `content_worker/core/workers/generator_manager.py`

**Step 1: 在 __init__ 中添加 _last_error 属性**

在 `__init__` 方法中（约第 93 行 `self.log` 之后）添加：

```python
        # 最后错误信息
        self._last_error: Optional[str] = None
```

**Step 2: 添加 _publish_realtime_status 方法**

在 `_update_status` 方法之后（约第 324 行）添加新方法：

```python
    async def _publish_realtime_status(self):
        """发布实时状态到 Redis 频道"""
        if not self.rdb:
            return

        try:
            # 查询队列长度
            queue_pending = await self.rdb.llen("pending:articles")
            queue_retry = await self.rdb.llen("pending:articles:retry")
            queue_dead = await self.rdb.llen("pending:articles:dead")

            # 汇总所有 Worker 统计
            total_processed = 0
            total_failed = 0
            for worker in self.worker_instances:
                stats = worker.get_stats()
                total_processed += stats.get('processed', 0)
                total_failed += stats.get('failed', 0)

            # 计算处理速度
            current_time = time.time()
            time_elapsed = current_time - self._last_stats_time if self._last_stats_time else 1
            processed_delta = total_processed - self._last_processed_count
            speed = processed_delta / time_elapsed if time_elapsed > 0 else 0.0

            # 获取今日处理量
            today_key = f"processor:processed:{datetime.now().strftime('%Y%m%d')}"
            today_count = await self.rdb.get(today_key)
            processed_today = int(today_count) if today_count else 0

            # 组装状态数据
            status = {
                "running": self.running,
                "workers": len(self.workers),
                "queue_pending": queue_pending,
                "queue_retry": queue_retry,
                "queue_dead": queue_dead,
                "processed_total": total_processed,
                "processed_today": processed_today,
                "speed": round(speed, 2),
                "last_error": self._last_error
            }

            # 发布到 Redis 频道
            await self.rdb.publish(
                "processor:status:realtime",
                json.dumps(status, ensure_ascii=False)
            )

        except Exception as e:
            logger.error(f"发布实时状态失败: {e}")
```

**Step 3: 修改 _start_workers 传入回调**

修改 `_start_workers` 方法中创建 Worker 的代码（约第 178-185 行）：

```python
            worker = GeneratorWorker(
                db_pool=self.db_pool,
                redis_client=self.rdb,
                batch_size=self.config.get('batch_size', 50),
                min_paragraph_length=self.config.get('min_paragraph_length', 20),
                retry_max=self.config.get('retry_max', 3),
                log=self.log,
                on_complete=self._publish_realtime_status,  # 传入回调
            )
```

**Step 4: 在任务失败时记录 last_error**

在 `_run_worker` 方法中（约第 198-208 行），捕获异常时记录错误：

```python
    async def _run_worker(self, worker: GeneratorWorker, index: int):
        """运行单个 Worker"""
        try:
            await worker.start()
            await worker.run_forever(stop_event=self._stop_event)
        except asyncio.CancelledError:
            logger.info(f"Worker {index} 被取消")
        except Exception as e:
            logger.error(f"Worker {index} 异常: {e}")
            self._last_error = str(e)
        finally:
            await worker.stop()
```

**Step 5: 验证语法**

Run: `python -m py_compile content_worker/core/workers/generator_manager.py`
Expected: 无输出（无错误）

**Step 6: 提交**

```bash
git add content_worker/core/workers/generator_manager.py
git commit -m "feat(worker): 添加实时状态发布到 Redis 频道"
```

---

## Task 4: Vue 前端 - WebSocket 替代轮询

**Files:**
- Modify: `web/src/views/processor/ProcessorManage.vue`

**Step 1: 更新 imports**

修改 import 部分（约第 107-117 行）：

```typescript
import { ref, reactive, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, VideoPlay, VideoPause, RefreshRight, Delete } from '@element-plus/icons-vue'
import { buildWsUrl } from '@/api/shared'
import {
  getProcessorStatus,
  startProcessor,
  stopProcessor,
  retryAllFailed,
  clearDeadQueue,
  type ProcessorStatus
} from '@/api/processor'
```

**Step 2: 替换定时器为 WebSocket 变量**

修改变量声明部分（约第 139-140 行），将：

```typescript
// 自动刷新定时器
let refreshTimer: number | null = null
```

替换为：

```typescript
// WebSocket 连接
let ws: WebSocket | null = null
let reconnectTimer: number | null = null
let reconnectDelay = 1000
```

**Step 3: 添加 WebSocket 连接函数**

在 `loadAll` 函数之后添加：

```typescript
// WebSocket 连接
const connectWebSocket = () => {
  ws = new WebSocket(buildWsUrl('/ws/processor-status'))

  ws.onopen = () => {
    console.log('WebSocket connected')
    reconnectDelay = 1000  // 重置重连延迟
    loadStatus()  // 获取初始状态
  }

  ws.onmessage = (event) => {
    try {
      const data = JSON.parse(event.data)
      Object.assign(status, data)
    } catch (e) {
      console.error('Failed to parse WebSocket message:', e)
    }
  }

  ws.onerror = (error) => {
    console.error('WebSocket error:', error)
  }

  ws.onclose = () => {
    console.log('WebSocket closed, reconnecting...')
    ws = null
    // 指数退避重连
    reconnectTimer = window.setTimeout(() => {
      connectWebSocket()
    }, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 2, 30000)
  }
}
```

**Step 4: 修改 onMounted 和 onUnmounted**

替换生命周期钩子（约第 232-242 行）：

```typescript
onMounted(() => {
  connectWebSocket()
})

onUnmounted(() => {
  // 清理 WebSocket 连接
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    ws.close()
    ws = null
  }
})
```

**Step 5: 验证编译**

Run: `cd web && npx vue-tsc --noEmit 2>&1 | grep ProcessorManage || echo "No errors in ProcessorManage.vue"`
Expected: "No errors in ProcessorManage.vue"

**Step 6: 提交**

```bash
git add web/src/views/processor/ProcessorManage.vue
git commit -m "feat(web): 使用 WebSocket 实时更新数据处理状态"
```

---

## Task 5: 集成验证

**Step 1: 验证所有编译通过**

```bash
cd api && go build ./...
cd ../web && npx vue-tsc --noEmit 2>&1 | head -5
python -m py_compile content_worker/core/workers/generator_worker.py content_worker/core/workers/generator_manager.py
```

**Step 2: 最终提交（如果有未提交的更改）**

```bash
git status
# 如果有未提交的更改，进行提交
```

---

## 验收标准

- [ ] Go 后端 `/ws/processor-status` 端点可用
- [ ] Python Worker 处理完任务后发布状态到 Redis
- [ ] 前端 WebSocket 连接成功，实时更新状态
- [ ] 断线后自动重连
- [ ] 手动刷新按钮仍可用
