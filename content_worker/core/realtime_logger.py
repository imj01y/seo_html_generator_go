# -*- coding: utf-8 -*-
"""
统一实时日志模块 - loguru sink + contextvars 方案

通过 loguru 的 sink 机制，所有 logger.xxx() 调用都会自动发送到 Redis。
使用 contextvars 实现异步上下文隔离，不同任务的日志发送到不同 channel。

使用方式：
    from core.realtime_logger import RealtimeContext, send_end, send_item

    async with RealtimeContext(redis, "spider:logs:project_1"):
        logger.info("爬虫启动")  # 自动发送到 Redis
        logger.warning("重试中")
        logger.error("请求失败")
        await send_item({"title": "xxx"})  # 发送数据项
    # 退出时自动发送 end 消息
"""

import json
import asyncio
import traceback
from contextvars import ContextVar
from datetime import datetime
from typing import Any, Dict, Optional, TYPE_CHECKING

from loguru import logger

if TYPE_CHECKING:
    from redis.asyncio import Redis

# ============================================
# Context Variables - 实现异步上下文隔离
# ============================================

_redis_ctx: ContextVar[Optional['Redis']] = ContextVar('_redis_ctx', default=None)
_channel_ctx: ContextVar[Optional[str]] = ContextVar('_channel_ctx', default=None)

# 全局 sink ID，用于管理 sink 生命周期
_sink_id: Optional[int] = None


# ============================================
# Redis Sink - 自动捕获所有 loguru 日志
# ============================================

def _redis_sink(message):
    """
    Loguru sink：将日志发送到 Redis

    只有在 RealtimeContext 上下文中才会发送，否则静默忽略。
    """
    redis = _redis_ctx.get()
    channel = _channel_ctx.get()

    if not redis or not channel:
        return

    record = message.record
    level = record["level"].name
    msg = record["message"]

    # 如果有异常信息，附加完整堆栈
    if record["exception"] is not None:
        exc_type, exc_value, exc_traceback = record["exception"]
        if exc_traceback is not None:
            tb_lines = traceback.format_exception(exc_type, exc_value, exc_traceback)
            msg = msg + "\n" + "".join(tb_lines)

    # 构造日志消息
    data = {
        "type": "log",
        "level": level,
        "message": msg,
        "timestamp": datetime.now().isoformat()
    }

    # 获取事件循环并发送
    try:
        loop = asyncio.get_running_loop()
        loop.call_soon_threadsafe(
            lambda d=data, r=redis, c=channel: asyncio.create_task(
                r.publish(c, json.dumps(d, ensure_ascii=False))
            )
        )
    except RuntimeError:
        # 没有运行中的事件循环，忽略
        pass


def init_realtime_sink():
    """
    初始化全局 Redis sink

    应在应用启动时调用一次。
    """
    global _sink_id
    if _sink_id is None:
        _sink_id = logger.add(
            _redis_sink,
            level="DEBUG",  # 捕获所有级别
            format="{message}",  # sink 只需要消息内容
            filter=lambda record: _channel_ctx.get() is not None,  # 仅在上下文中生效
        )
        logger.debug("Realtime Redis sink initialized")


def remove_realtime_sink():
    """
    移除全局 Redis sink

    应在应用关闭时调用。
    """
    global _sink_id
    if _sink_id is not None:
        try:
            logger.remove(_sink_id)
        except ValueError:
            pass
        _sink_id = None


# ============================================
# RealtimeContext - 异步上下文管理器
# ============================================

class RealtimeContext:
    """
    实时日志上下文管理器

    进入上下文后，所有 logger.xxx() 调用都会自动发送到指定的 Redis channel。
    退出上下文时自动发送 end 消息。

    用法：
        async with RealtimeContext(redis, "spider:logs:project_1"):
            logger.info("任务开始")
            # ... 执行任务 ...
        # 自动发送 end 消息
    """

    def __init__(self, redis: 'Redis', channel: str, auto_end: bool = True):
        """
        初始化上下文

        Args:
            redis: Redis 异步客户端
            channel: Redis Pub/Sub channel
            auto_end: 退出时是否自动发送 end 消息（默认 True）
        """
        self.redis = redis
        self.channel = channel
        self.auto_end = auto_end
        self._redis_token = None
        self._channel_token = None

    async def __aenter__(self):
        """进入上下文，设置 contextvars"""
        self._redis_token = _redis_ctx.set(self.redis)
        self._channel_token = _channel_ctx.set(self.channel)
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """退出上下文，重置 contextvars 并发送 end 消息"""
        try:
            if self.auto_end:
                await self.end()
        finally:
            # 重置 contextvars
            _redis_ctx.reset(self._redis_token)
            _channel_ctx.reset(self._channel_token)

    async def end(self):
        """发送结束信号，通知前端任务已完成"""
        data = {
            "type": "end",
            "timestamp": datetime.now().isoformat()
        }
        await self.redis.publish(self.channel, json.dumps(data, ensure_ascii=False))

    async def item(self, data: Dict[str, Any]):
        """
        发送数据项（用于测试运行时展示数据）

        Args:
            data: 数据字典，如 {"title": "xxx", "content": "xxx"}
        """
        msg = {
            "type": "log",
            "level": "ITEM",
            "message": json.dumps(data, ensure_ascii=False),
            "timestamp": datetime.now().isoformat()
        }
        await self.redis.publish(self.channel, json.dumps(msg, ensure_ascii=False))


# ============================================
# 便捷函数 - 在上下文外部使用
# ============================================

async def send_end(redis: 'Redis', channel: str):
    """
    发送结束消息（在上下文外部使用）

    用于如 stop_test 等场景，需要在任务取消后发送 end 消息。
    """
    data = {
        "type": "end",
        "timestamp": datetime.now().isoformat()
    }
    await redis.publish(channel, json.dumps(data, ensure_ascii=False))


async def send_item(data: Dict[str, Any]):
    """
    发送数据项（在上下文内部使用）

    从 contextvars 获取 redis 和 channel。
    """
    redis = _redis_ctx.get()
    channel = _channel_ctx.get()

    if not redis or not channel:
        logger.warning("send_item called outside RealtimeContext")
        return

    msg = {
        "type": "log",
        "level": "ITEM",
        "message": json.dumps(data, ensure_ascii=False),
        "timestamp": datetime.now().isoformat()
    }
    await redis.publish(channel, json.dumps(msg, ensure_ascii=False))


# ============================================
# 持久化上下文 - 用于长期运行的服务
# ============================================

def set_realtime_context(redis: 'Redis', channel: str):
    """
    设置持久化实时日志上下文（不自动退出）

    用于如 GeneratorManager 等长期运行的服务。
    设置后，该任务内的所有 logger 调用都会发送到指定 channel。

    Args:
        redis: Redis 异步客户端
        channel: Redis Pub/Sub channel
    """
    _redis_ctx.set(redis)
    _channel_ctx.set(channel)


def clear_realtime_context():
    """
    清除持久化实时日志上下文

    在服务停止时调用。
    """
    _redis_ctx.set(None)
    _channel_ctx.set(None)
