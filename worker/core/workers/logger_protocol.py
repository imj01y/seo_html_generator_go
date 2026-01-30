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
