# 统一实时日志系统设计 - loguru sink + contextvars 方案

## 背景

当前日志系统存在以下问题：

1. **两套日志 API** - `loguru.logger` 和 `RealtimeLogger` 并存，使用混乱
2. **核心模块日志丢失** - queue_consumer、http_client 等使用 loguru，日志不会发送到前端
3. **手动调用冗余** - 需要显式调用 `log.info()` 而非 `logger.info()`
4. **上下文传递麻烦** - 需要把 log 实例作为参数层层传递

## 目标

1. **零代码改动** - 核心模块无需修改，所有 `logger.xxx()` 自动发送到 Redis
2. **双输出** - 控制台 + Redis 实时推送
3. **上下文隔离** - 不同任务的日志发送到不同 channel（spider:logs:test_1、processor:logs 等）
4. **简洁 API** - 使用 `async with` 上下文管理器

## 设计方案

### 核心组件

```python
# contextvars 实现异步上下文隔离
_redis_ctx: ContextVar[Optional[Redis]] = ContextVar('_redis_ctx', default=None)
_channel_ctx: ContextVar[Optional[str]] = ContextVar('_channel_ctx', default=None)

# 全局 loguru sink - 自动捕获所有日志
def _redis_sink(message):
    redis = _redis_ctx.get()
    channel = _channel_ctx.get()
    if redis and channel:
        # 发送到 Redis
        asyncio.create_task(redis.publish(channel, ...))

# 上下文管理器
class RealtimeContext:
    async def __aenter__(self):
        _redis_ctx.set(self.redis)
        _channel_ctx.set(self.channel)

    async def __aexit__(self, ...):
        await self.end()  # 自动发送 end 消息
        # 重置 contextvars
```

### 架构图

```
┌──────────────────────────────────────────────────────────────────┐
│          async with RealtimeContext(redis, channel):             │
│                    设置 contextvars                               │
└──────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
┌──────────────────────────────────────────────────────────────────┐
│                    logger.info("message")                         │
│               任何模块、任何深度的调用都会被捕获                     │
└──────────────────────────────────────────────────────────────────┘
                                  │
          ┌───────────────────────┴───────────────────────┐
          ▼                                               ▼
┌───────────────────┐                    ┌───────────────────────────┐
│    控制台输出      │                    │     _redis_sink()         │
│  (loguru 默认)    │                    │  读取 contextvars         │
└───────────────────┘                    │  发送到 Redis channel     │
                                         └───────────────────────────┘
                                                      │
                                                      ▼
                                         ┌───────────────────────────┐
                                         │     Redis Pub/Sub         │
                                         │   (spider:logs:test_1)    │
                                         └───────────────────────────┘
                                                      │
                                                      ▼
                                         ┌───────────────────────────┐
                                         │    Go API WebSocket       │
                                         └───────────────────────────┘
                                                      │
                                                      ▼
                                         ┌───────────────────────────┐
                                         │         前端              │
                                         └───────────────────────────┘
```

## API 接口

### 1. 初始化（应用启动时调用一次）

```python
from core.realtime_logger import init_realtime_sink

# 在 start() 中调用
init_realtime_sink()
```

### 2. 短期任务（爬虫运行/测试）

```python
from core.realtime_logger import RealtimeContext
from loguru import logger

async def run_spider(project_id: int):
    async with RealtimeContext(redis, f"spider:logs:project_{project_id}"):
        logger.info("爬虫启动")  # 自动发送到 Redis

        async for item in runner.run():
            # queue_consumer、http_client 的日志也会自动发送
            logger.debug(f"获取: {item['title'][:20]}")

        logger.info("爬虫完成")
    # 退出时自动发送 end 消息
```

### 3. 长期服务（数据加工 processor）

```python
from core.realtime_logger import set_realtime_context, clear_realtime_context, init_realtime_sink
from loguru import logger

async def start():
    init_realtime_sink()
    set_realtime_context(redis, "processor:logs")  # 设置持久上下文

    logger.info("Processor 启动")  # 自动发送到 processor:logs
    # ... 长期运行 ...

async def stop():
    logger.info("Processor 停止")
    clear_realtime_context()  # 清除上下文
```

### 4. 发送特殊消息

```python
from core.realtime_logger import send_end, send_item

# 在上下文外部发送 end（如 stop_test）
await send_end(redis, f"spider:logs:test_{project_id}")

# 在上下文内部发送数据项
async with RealtimeContext(redis, channel) as ctx:
    await ctx.item({"title": "xxx", "content": "xxx"})
```

## 使用示例

### 爬虫测试运行

```python
async def test_project(self, project_id: int, max_items: int = 0):
    channel = f"spider:logs:test_{project_id}"

    async with RealtimeContext(self.rdb, channel) as ctx:
        logger.info(f"开始测试运行...")

        # queue_consumer.py 中的 logger.warning() 也会发送到前端
        # http_client.py 中的 logger.error() 也会发送到前端

        async for item in runner.run():
            logger.info(f"[{items_count}] {item['title'][:50]}")
            await ctx.item(item)  # 发送数据项

        logger.info(f"测试完成")
    # 自动发送 end
```

### 数据加工

```python
async def start(self):
    init_realtime_sink()
    set_realtime_context(self.rdb, "processor:logs")

    logger.info("数据加工管理器启动")
    # 所有后续的 logger 调用都会发送到 processor:logs
```

## 文件结构

```
content_worker/core/
├── realtime_logger.py      # loguru sink + contextvars 实现
└── workers/
    ├── command_listener.py # 使用 RealtimeContext
    └── generator_manager.py # 使用 set_realtime_context
```

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

## 与旧方案对比

| 特性 | 旧方案 (RealtimeLogger) | 新方案 (loguru sink + contextvars) |
|------|------------------------|-----------------------------------|
| 核心模块日志 | ❌ 不发送到前端 | ✅ 自动发送 |
| 代码改动 | 需要传递 log 实例 | 零改动，使用标准 logger |
| 深层调用日志 | ❌ 丢失 | ✅ 自动捕获 |
| 上下文管理 | 手动管理 | 自动管理 |
| 日志隔离 | 通过实例 | 通过 contextvars |

## 注意事项

1. **init_realtime_sink() 是幂等的**，可以多次调用
2. **RealtimeContext 自动发送 end**，退出时无需手动调用
3. **set_realtime_context 用于长期服务**，不会自动发送 end
4. **contextvars 在异步任务间自动传播**，子任务会继承父任务的上下文
