# 爬虫实时日志增强 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将爬虫执行过程中的详细日志（HTTP 请求、重试、失败、解析等）通过 Redis 实时推送到前端显示。

**Architecture:** 创建 `RedisLoggerProtocol` 协议类，将 `RedisLogger` 实例从 `command_listener.py` 传递到 `ProjectRunner` 和 `QueueConsumer`，在关键执行点调用日志方法。使用可选参数保持向后兼容。

**Tech Stack:** Python 3.11+, asyncio, redis.asyncio, typing.Protocol

---

## Task 1: 创建日志协议类

**Files:**
- Create: `worker/core/workers/logger_protocol.py`

**Step 1: 创建协议文件**

```python
# -*- coding: utf-8 -*-
"""
日志协议定义

定义 RedisLogger 的协议接口，供其他模块类型检查使用。
"""

from typing import Protocol, runtime_checkable


@runtime_checkable
class LoggerProtocol(Protocol):
    """日志协议，定义 RedisLogger 需要实现的方法"""

    async def info(self, msg: str) -> None: ...
    async def warning(self, msg: str) -> None: ...
    async def error(self, msg: str) -> None: ...
    async def debug(self, msg: str) -> None: ...
```

**Step 2: 验证文件语法**

Run: `cd worker && python -c "from core.workers.logger_protocol import LoggerProtocol; print('OK')"`
Expected: `OK`

**Step 3: Commit**

```bash
git add worker/core/workers/logger_protocol.py
git commit -m "feat: add LoggerProtocol for type-safe logging"
```

---

## Task 2: 扩展 RedisLogger 添加 debug 方法

**Files:**
- Modify: `worker/core/workers/command_listener.py:20-57`

**Step 1: 在 RedisLogger 类中添加 debug 方法**

在 `command_listener.py` 的 `RedisLogger` 类中，找到 `async def debug(self, msg: str)` 方法（约第 47-49 行），确认已存在。如果不存在则添加：

```python
async def debug(self, msg: str):
    await self._publish("DEBUG", msg)
    logger.debug(msg)
```

**Step 2: 验证 RedisLogger 实现协议**

Run: `cd worker && python -c "from core.workers.command_listener import RedisLogger; from core.workers.logger_protocol import LoggerProtocol; print(isinstance(RedisLogger.__new__(RedisLogger), LoggerProtocol))"`
Expected: 输出包含 `True` 或无报错

**Step 3: Commit (如有修改)**

```bash
git add worker/core/workers/command_listener.py
git commit -m "feat: ensure RedisLogger implements LoggerProtocol"
```

---

## Task 3: 修改 QueueConsumer 接收日志实例

**Files:**
- Modify: `worker/core/crawler/queue_consumer.py`

**Step 1: 添加导入和参数**

在文件顶部的 `TYPE_CHECKING` 块中添加导入（约第 14-15 行后）：

```python
if TYPE_CHECKING:
    from redis.asyncio import Redis
    from core.workers.logger_protocol import LoggerProtocol
```

**Step 2: 修改 `__init__` 方法签名**

在 `__init__` 方法中添加 `log` 参数（约第 54 行后）：

```python
def __init__(
    self,
    redis: 'Redis',
    project_id: int,
    spider: Spider,
    concurrency: int = 3,
    http_client: Optional[AsyncHttpClient] = None,
    stop_callback: Optional[Callable[[], bool]] = None,
    is_test: bool = False,
    max_items: int = 0,
    start_requests_iter: Optional[Iterator] = None,
    log: Optional['LoggerProtocol'] = None,  # 新增
):
```

**Step 3: 保存 log 实例**

在 `__init__` 方法体中（约第 85 行后）添加：

```python
# 日志（可选，用于推送到前端）
self.log = log
```

**Step 4: 在 `_fetch_request` 方法中添加日志**

找到 `_fetch_request` 方法（约第 150 行），在 `logger.info(f"Fetching...` 行前添加：

```python
# 推送到前端
if self.log:
    url_short = request.url[:60] + ('...' if len(request.url) > 60 else '')
    await self.log.debug(f"正在请求: {url_short}")
```

在请求成功后（约第 198 行 `response = Response.from_request` 后）添加：

```python
if self.log:
    size_kb = len(body) / 1024
    await self.log.debug(f"请求成功 (200, {size_kb:.1f}KB)")
```

在请求失败处（约第 195 行 `if body is None` 块内）添加：

```python
if self.log:
    error_msg = self.http_client.last_error or '未知错误'
    await self.log.warning(f"请求失败: {request.url[:50]} - {error_msg}")
```

**Step 5: 在重试逻辑中添加日志**

找到 `_process_request` 方法中的重试延迟代码（约第 237-244 行），在 `logger.debug(f"Retry delay...` 后添加：

```python
if self.log:
    await self.log.warning(f"正在重试 ({request.retry_count}/{request.max_retries}): {request.url[:50]}")
```

**Step 6: 在回调出错处添加日志**

找到 `_process_request` 方法中的 `except Exception as e` 块（约第 341 行），在 `logger.exception` 后添加：

```python
if self.log:
    await self.log.error(f"解析出错: {str(e)[:100]}")
```

**Step 7: 验证语法**

Run: `cd worker && python -c "from core.crawler.queue_consumer import QueueConsumer; print('OK')"`
Expected: `OK`

**Step 8: Commit**

```bash
git add worker/core/crawler/queue_consumer.py
git commit -m "feat: add optional log parameter to QueueConsumer for realtime logging"
```

---

## Task 4: 修改 ProjectRunner 接收并传递日志实例

**Files:**
- Modify: `worker/core/crawler/project_runner.py`

**Step 1: 添加导入**

在 `TYPE_CHECKING` 块中添加（约第 15-17 行）：

```python
if TYPE_CHECKING:
    from redis.asyncio import Redis
    from aiomysql import Pool
    from core.workers.logger_protocol import LoggerProtocol
```

**Step 2: 修改 `__init__` 方法签名**

添加 `log` 参数（约第 39 行后）：

```python
def __init__(
    self,
    project_id: int,
    modules: Dict[str, types.ModuleType],
    config: Optional[Dict[str, Any]] = None,
    redis: Optional['Redis'] = None,
    db_pool: Optional['Pool'] = None,
    concurrency: int = 3,
    is_test: bool = False,
    max_items: int = 0,
    log: Optional['LoggerProtocol'] = None,  # 新增
):
```

**Step 3: 保存 log 实例**

在 `__init__` 方法体末尾（约第 63 行后）添加：

```python
self.log = log
```

**Step 4: 在 run 方法中传递 log 给 QueueConsumer**

找到 `run` 方法中创建 `QueueConsumer` 的代码（约第 141-150 行），添加 `log` 参数：

```python
consumer = QueueConsumer(
    redis=self.redis,
    project_id=self.project_id,
    spider=spider,
    concurrency=self.concurrency,
    stop_callback=self._check_stop,
    is_test=self.is_test,
    max_items=self.max_items,
    start_requests_iter=start_requests_iter,
    log=self.log,  # 新增
)
```

**Step 5: 添加 Spider 启动日志**

在 `run` 方法中 `logger.info(f"Spider '{spider_name}' starting")` 后（约第 130 行）添加：

```python
if self.log:
    await self.log.info(f"Spider '{spider_name}' 启动，并发数: {self.concurrency}")
```

**Step 6: 验证语法**

Run: `cd worker && python -c "from core.crawler.project_runner import ProjectRunner; print('OK')"`
Expected: `OK`

**Step 7: Commit**

```bash
git add worker/core/crawler/project_runner.py
git commit -m "feat: add optional log parameter to ProjectRunner and pass to QueueConsumer"
```

---

## Task 5: 修改 command_listener 传递 RedisLogger

**Files:**
- Modify: `worker/core/workers/command_listener.py`

**Step 1: 在 run_project 方法中传递 log**

找到 `run_project` 方法中创建 `ProjectRunner` 的代码（约第 161-168 行），添加 `log` 参数：

```python
runner = ProjectRunner(
    project_id=project_id,
    modules=modules,
    config=config,
    redis=self.rdb,
    db_pool=get_db_pool(),
    concurrency=row.get('concurrency', 3),
    log=log,  # 新增
)
```

**Step 2: 在 test_project 方法中传递 log**

找到 `test_project` 方法中创建 `ProjectRunner` 的代码（约第 313-322 行），添加 `log` 参数：

```python
runner = ProjectRunner(
    project_id=project_id,
    modules=modules,
    config=config,
    redis=self.rdb,
    db_pool=get_db_pool(),
    concurrency=row.get('concurrency', 3),
    is_test=True,
    max_items=max_items,
    log=log,  # 新增
)
```

**Step 3: 验证语法**

Run: `cd worker && python -c "from core.workers.command_listener import CommandListener; print('OK')"`
Expected: `OK`

**Step 4: Commit**

```bash
git add worker/core/workers/command_listener.py
git commit -m "feat: pass RedisLogger to ProjectRunner for realtime frontend logs"
```

---

## Task 6: 手动测试验证

**Step 1: 重启 Worker 容器**

Run: `docker-compose restart worker`
Expected: Worker 容器重启成功

**Step 2: 在前端测试爬虫**

1. 打开浏览器访问 `http://localhost:8008`
2. 进入爬虫项目编辑页面
3. 点击"测试运行"
4. 观察日志面板，应该能看到：
   - DEBUG 级别的 HTTP 请求日志
   - WARNING 级别的重试/失败日志
   - ERROR 级别的解析错误日志

**Step 3: 确认日志正常显示后 Commit**

```bash
git add -A
git commit -m "feat: complete realtime spider logs enhancement"
```

---

## Summary

| Task | 描述 | 文件 |
|------|------|------|
| 1 | 创建日志协议类 | `logger_protocol.py` (新建) |
| 2 | 确保 RedisLogger 实现协议 | `command_listener.py` |
| 3 | QueueConsumer 接收 log 参数 | `queue_consumer.py` |
| 4 | ProjectRunner 接收并传递 log | `project_runner.py` |
| 5 | command_listener 传递 RedisLogger | `command_listener.py` |
| 6 | 手动测试验证 | - |
