# 统一实时日志系统设计

## 背景

当前日志系统存在以下问题：

1. **两套日志 API** - `loguru.logger` 和 `RedisLogger` 并存，使用混乱
2. **冗余判断** - 代码中大量 `if self.log:` 条件判断
3. **不一致** - 测试模式有 loguru sink 转发，正式运行没有
4. **命名混淆** - `self.log` 容易和 `logger` 混淆

## 目标

1. **简单直观** - 像 loguru 一样，实例化后直接调用
2. **双输出** - 控制台 + Redis 实时推送
3. **实例隔离** - 不同实例对应不同 channel
4. **无额外语法** - 不需要 async with、装饰器等

## 设计方案

### 核心类

```python
class RealtimeLogger:
    """
    实时日志器 - 每个实例绑定一个 Redis channel

    使用方式：
        log = RealtimeLogger(redis, "spider:logs:project_1")
        log.info("爬虫启动")      # → 控制台 + 前端
        log.warning("重试中")     # → 控制台 + 前端
        await log.end()           # 发送结束信号
    """

    def __init__(self, redis, channel: str):
        self.redis = redis
        self.channel = channel
        self._loop = None

    def info(self, msg: str):
        """INFO 级别日志"""
        logger.opt(depth=1).info(msg)
        self._publish("INFO", msg)

    def warning(self, msg: str):
        """WARNING 级别日志"""
        logger.opt(depth=1).warning(msg)
        self._publish("WARNING", msg)

    def error(self, msg: str):
        """ERROR 级别日志"""
        logger.opt(depth=1).error(msg)
        self._publish("ERROR", msg)

    def debug(self, msg: str):
        """DEBUG 级别日志"""
        logger.opt(depth=1).debug(msg)
        self._publish("DEBUG", msg)

    def exception(self, msg: str):
        """异常日志，自动附带堆栈"""
        logger.opt(depth=1).exception(msg)
        # 获取异常堆栈
        exc_info = sys.exc_info()
        if exc_info[0]:
            tb_lines = traceback.format_exception(*exc_info)
            msg = msg + "\n" + "".join(tb_lines)
        self._publish("ERROR", msg)

    def _get_loop(self):
        """获取事件循环"""
        if self._loop is None:
            try:
                self._loop = asyncio.get_running_loop()
            except RuntimeError:
                pass
        return self._loop

    def _publish(self, level: str, msg: str):
        """同步方法，桥接到异步 Redis publish"""
        loop = self._get_loop()
        if not loop or not self.redis:
            return

        data = {
            "type": "log",
            "level": level,
            "message": msg,
            "timestamp": datetime.now().isoformat()
        }

        # 桥接同步到异步
        loop.call_soon_threadsafe(
            lambda: asyncio.create_task(
                self.redis.publish(self.channel, json.dumps(data, ensure_ascii=False))
            )
        )

    async def end(self):
        """发送结束信号"""
        data = {
            "type": "end",
            "timestamp": datetime.now().isoformat()
        }
        await self.redis.publish(self.channel, json.dumps(data, ensure_ascii=False))

    async def item(self, data: dict):
        """发送数据项（用于测试运行时展示数据）"""
        msg = {
            "type": "log",
            "level": "ITEM",
            "message": json.dumps(data, ensure_ascii=False),
            "timestamp": datetime.now().isoformat()
        }
        await self.redis.publish(self.channel, json.dumps(msg, ensure_ascii=False))
```

### 架构图

```
┌──────────────────────────────────────────────────────────────────┐
│                    log = RealtimeLogger(redis, channel)          │
└──────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│                         log.info("msg")                          │
│  ┌────────────────────────┐    ┌────────────────────────────┐   │
│  │    logger.info(msg)    │    │   self._publish(level, msg) │   │
│  │      (控制台输出)       │    │   (桥接到异步 Redis)        │   │
│  └────────────────────────┘    └────────────────────────────┘   │
└──────────────────────────────────────────────────────────────────┘
         │                                    │
         ▼                                    ▼
    ┌─────────┐                    ┌───────────────────┐
    │  控制台  │                    │   Redis Pub/Sub   │
    └─────────┘                    │  (指定 channel)   │
                                   └───────────────────┘
                                             │
                                             ▼
                                   ┌───────────────────┐
                                   │  Go API WebSocket │
                                   └───────────────────┘
                                             │
                                             ▼
                                   ┌───────────────────┐
                                   │       前端        │
                                   └───────────────────┘
```

## 使用方式

### 爬虫正式运行

```python
from core.realtime_logger import RealtimeLogger

async def run_spider(project_id: int):
    log = RealtimeLogger(redis, f"spider:logs:project_{project_id}")

    log.info("爬虫启动")
    log.info(f"并发数: {concurrency}")

    async for item in runner.run():
        log.debug(f"获取: {item['title'][:20]}")

    log.info("爬虫完成")
    await log.end()
```

### 爬虫测试运行

```python
async def test_spider(project_id: int):
    log = RealtimeLogger(redis, f"spider:logs:test_{project_id}")

    log.info("开始测试运行")

    async for item in runner.run():
        log.info(f"获取: {item['title'][:30]}")
        await log.item(item)  # 发送数据供前端展示

    log.info("测试完成")
    await log.end()
```

### 数据加工

```python
async def process_articles():
    log = RealtimeLogger(redis, "processor:logs")

    log.info("开始处理文章")

    for i, article in enumerate(articles):
        if i % 100 == 0:
            log.info(f"已处理 {i} 篇")

    log.info("处理完成")
    await log.end()
```

### 异常处理

```python
async def run_task():
    log = RealtimeLogger(redis, "task:logs")

    try:
        log.info("任务开始")
        # ... 业务逻辑
    except Exception as e:
        log.exception(f"任务异常: {e}")  # 自动附带堆栈
    finally:
        await log.end()
```

## 文件结构

```
content_worker/core/
├── realtime_logger.py      # 新增：统一实时日志模块
└── workers/
    ├── command_listener.py # 修改：使用新日志模块，删除 RedisLogger 类
    ├── generator_worker.py # 修改：删除 self.log 参数
    └── logger_protocol.py  # 删除：不再需要
```

## 迁移计划

### 1. 新增 realtime_logger.py

创建 `content_worker/core/realtime_logger.py`

### 2. 修改 command_listener.py

**删除：**
- `RedisLogger` 类定义
- `test_project()` 中的 `log_sink` 和 `handler_id` 管理

**修改：**
```python
# 之前
log = RedisLogger(self.rdb, project_id, "project")
await log.info("正在加载项目...")

# 之后
from core.realtime_logger import RealtimeLogger
log = RealtimeLogger(self.rdb, f"spider:logs:project_{project_id}")
log.info("正在加载项目...")
```

### 3. 修改 project_runner.py

**删除：**
- `__init__` 中的 `log` 参数
- `self.log = log`
- 所有 `if self.log:` 判断

**修改：**
- 直接使用 `logger.info()` 输出到控制台（不推送到前端）
- 或者在需要时传入 `RealtimeLogger` 实例

### 4. 修改 queue_consumer.py

**删除：**
- `__init__` 中的 `log` 参数
- `self.log = log`

### 5. 修改 generator_worker.py

**删除：**
- `__init__` 中的 `log` 参数
- `self.log = log`
- 所有 `if self.log:` 判断

### 6. 删除 logger_protocol.py

不再需要这个协议定义文件。

## Redis Channel 规范

| 场景 | Channel 格式 | 示例 |
|------|-------------|------|
| 爬虫正式运行 | `spider:logs:project_{id}` | `spider:logs:project_1` |
| 爬虫测试运行 | `spider:logs:test_{id}` | `spider:logs:test_1` |
| 数据加工 | `processor:logs` | `processor:logs` |

## 日志消息格式

### 普通日志

```json
{
    "type": "log",
    "level": "INFO",
    "message": "爬虫启动",
    "timestamp": "2026-02-05T12:00:00.123456"
}
```

### 结束信号

```json
{
    "type": "end",
    "timestamp": "2026-02-05T12:00:00.123456"
}
```

### 数据项（测试运行）

```json
{
    "type": "log",
    "level": "ITEM",
    "message": "{\"title\": \"xxx\", \"content\": \"xxx\"}",
    "timestamp": "2026-02-05T12:00:00.123456"
}
```

## Go API 无需修改

当前 Go API 的 WebSocket 处理（`websocket.go`）已经通过 `subscribeAndForward` 订阅 Redis channel 并转发到前端，无需修改。

## 与 loguru 的对比

| 特性 | loguru | RealtimeLogger |
|------|--------|----------------|
| 实例化 | `from loguru import logger` | `log = RealtimeLogger(redis, channel)` |
| 调用方式 | `logger.info(msg)` | `log.info(msg)` |
| 输出目标 | 控制台/文件 | 控制台 + Redis |
| 实例隔离 | 通过 bind | 通过不同实例 |

## 注意事项

1. **info/warning/error/debug 是同步方法**，内部桥接到异步，可以在任何地方调用
2. **end() 是异步方法**，需要 `await log.end()`
3. **item() 是异步方法**，需要 `await log.item(data)`
4. **logger.opt(depth=1)** 确保控制台日志显示正确的调用位置
