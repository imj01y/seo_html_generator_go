# -*- coding: utf-8 -*-
"""
统一实时日志模块

提供 RealtimeLogger 类，实现日志同时输出到控制台和 Redis（推送到前端）。

使用方式：
    from core.realtime_logger import RealtimeLogger

    log = RealtimeLogger(redis, "spider:logs:project_1")
    log.info("爬虫启动")
    log.warning("重试中")
    log.error("请求失败")
    await log.end()
"""

import sys
import json
import asyncio
import traceback
from datetime import datetime
from typing import Any, Dict, Optional, TYPE_CHECKING

from loguru import logger

if TYPE_CHECKING:
    from redis.asyncio import Redis


class RealtimeLogger:
    """
    实时日志器 - 每个实例绑定一个 Redis channel

    特性：
    - info/warning/error/debug/exception 是同步方法，可直接调用
    - 同时输出到控制台（loguru）和 Redis（前端）
    - 不同实例对应不同 channel，实现任务隔离
    """

    def __init__(self, redis: 'Redis', channel: str):
        """
        初始化实时日志器

        Args:
            redis: Redis 异步客户端
            channel: Redis Pub/Sub channel（如 "spider:logs:project_1"）
        """
        self.redis = redis
        self.channel = channel
        self._loop: Optional[asyncio.AbstractEventLoop] = None

    def _get_loop(self) -> Optional[asyncio.AbstractEventLoop]:
        """获取当前事件循环"""
        if self._loop is None:
            try:
                self._loop = asyncio.get_running_loop()
            except RuntimeError:
                pass
        return self._loop

    def _publish(self, level: str, msg: str):
        """
        同步方法，桥接到异步 Redis publish

        Args:
            level: 日志级别（INFO, WARNING, ERROR, DEBUG）
            msg: 日志消息
        """
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
        try:
            loop.call_soon_threadsafe(
                lambda d=data: asyncio.create_task(
                    self.redis.publish(self.channel, json.dumps(d, ensure_ascii=False))
                )
            )
        except RuntimeError:
            # 事件循环已关闭，忽略
            pass

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
        """
        异常日志，自动附带堆栈信息

        应在 except 块中调用
        """
        logger.opt(depth=1).exception(msg)

        # 获取异常堆栈
        exc_info = sys.exc_info()
        if exc_info[0]:
            tb_lines = traceback.format_exception(*exc_info)
            msg = msg + "\n" + "".join(tb_lines)

        self._publish("ERROR", msg)

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
