# -*- coding: utf-8 -*-
"""
统一日志管理器

基于 loguru 实现的日志系统，支持：
1. 控制台输出
2. 文件输出（按日期轮转）
3. WebSocket 实时推送
4. 数据库存储（ERROR 级别以上）

用法:
    from core.logging import init_logging, get_log_manager

    # 初始化（应用启动时调用一次）
    init_logging()

    # 在爬虫执行时设置上下文
    log_manager = get_log_manager()
    log_manager.set_context(spider_project_id=123)

    # 使用标准 loguru 记录日志
    from loguru import logger
    logger.info("这条日志会自动关联到项目 123")

    # 清除上下文
    log_manager.clear_context()
"""

import sys
import json
import asyncio
import threading
from typing import Set, Optional, Dict, Any, Callable

from loguru import logger


class LogManager:
    """
    统一日志管理器

    采用单例模式，管理所有日志输出目标。
    """

    _instance: Optional['LogManager'] = None
    _lock = threading.Lock()

    def __init__(self):
        self._websockets: Set = set()  # 已连接的 WebSocket 客户端
        self._current_context: Dict[str, Any] = {}  # 当前上下文
        self._main_loop: Optional[asyncio.AbstractEventLoop] = None
        self._db_insert_func: Optional[Callable] = None  # 数据库插入函数
        self._initialized = False

    @classmethod
    def get_instance(cls) -> 'LogManager':
        """获取单例实例"""
        if cls._instance is None:
            with cls._lock:
                if cls._instance is None:
                    cls._instance = cls()
        return cls._instance

    def setup(self, db_insert_func: Optional[Callable] = None):
        """
        配置日志系统

        Args:
            db_insert_func: 可选的数据库插入函数，签名: async def insert(table, data) -> int
        """
        if self._initialized:
            return

        self._db_insert_func = db_insert_func

        # 移除默认 handler
        logger.remove()

        # 1. 控制台输出
        logger.add(
            sys.stderr,
            format="<green>{time:YYYY-MM-DD HH:mm:ss}</green> | <level>{level: <8}</level> | <cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> - <level>{message}</level>",
            level="INFO",
            colorize=True,
        )

        # 2. 文件输出（按日期轮转）
        logger.add(
            "logs/{time:YYYY-MM-DD}.log",
            rotation="00:00",
            retention="30 days",
            format="{time:YYYY-MM-DD HH:mm:ss} | {level: <8} | {name}:{function}:{line} - {message}",
            level="INFO",
            encoding="utf-8",
        )

        # 3. WebSocket 推送（INFO 级别以上）
        logger.add(
            self._websocket_sink,
            format="{message}",
            level="INFO",
            filter=lambda record: not record["extra"].get("skip_ws", False),
        )

        # 4. 数据库存储（ERROR 级别以上）
        if db_insert_func:
            logger.add(
                self._database_sink,
                level="ERROR",
                format="{message}",
            )

        self._initialized = True

    def _websocket_sink(self, message):
        """推送日志到所有 WebSocket 客户端"""
        if not self._websockets:
            return

        record = message.record
        log_data = {
            "type": "log",
            "level": record["level"].name,
            "module": record["name"],
            "message": record["message"],
            "time": record["time"].isoformat(),
            "spider_project_id": self._current_context.get("spider_project_id"),
        }

        # 异步发送到所有客户端
        json_data = json.dumps(log_data, ensure_ascii=False)

        # 检查是否在事件循环中
        try:
            loop = asyncio.get_running_loop()
            # 在事件循环中，直接创建任务
            for ws in self._websockets.copy():
                asyncio.create_task(self._safe_send(ws, json_data))
        except RuntimeError:
            # 不在事件循环中，使用 call_soon_threadsafe
            if self._main_loop and self._main_loop.is_running():
                for ws in self._websockets.copy():
                    self._main_loop.call_soon_threadsafe(
                        lambda w=ws, d=json_data: asyncio.create_task(self._safe_send(w, d))
                    )

    async def _safe_send(self, ws, data: str):
        """安全发送消息到 WebSocket"""
        try:
            await ws.send_text(data)
        except Exception:
            self._websockets.discard(ws)

    def _database_sink(self, message):
        """存储日志到数据库"""
        if not self._db_insert_func:
            return

        record = message.record
        data = {
            "level": record["level"].name,
            "module": record["name"],
            "spider_project_id": self._current_context.get("spider_project_id"),
            "message": record["message"],
            "extra": json.dumps(dict(record["extra"]), ensure_ascii=False, default=str),
        }

        # 检查是否在事件循环中
        try:
            loop = asyncio.get_running_loop()
            asyncio.create_task(self._db_insert_func("system_logs", data))
        except RuntimeError:
            if self._main_loop and self._main_loop.is_running():
                self._main_loop.call_soon_threadsafe(
                    lambda: asyncio.create_task(self._db_insert_func("system_logs", data))
                )

    def set_event_loop(self, loop: asyncio.AbstractEventLoop):
        """设置主事件循环引用"""
        self._main_loop = loop

    def register_websocket(self, ws):
        """注册 WebSocket 客户端"""
        self._websockets.add(ws)

    def unregister_websocket(self, ws):
        """注销 WebSocket 客户端"""
        self._websockets.discard(ws)

    def set_context(self, **kwargs):
        """
        设置当前日志上下文

        Args:
            spider_project_id: 爬虫项目ID
            其他键值对...
        """
        self._current_context.update(kwargs)

    def clear_context(self):
        """清除日志上下文"""
        self._current_context.clear()

    def get_context(self) -> Dict[str, Any]:
        """获取当前上下文"""
        return self._current_context.copy()

    @property
    def websocket_count(self) -> int:
        """获取已连接的 WebSocket 数量"""
        return len(self._websockets)


# 全局实例
_log_manager: Optional[LogManager] = None


def init_logging(db_insert_func: Optional[Callable] = None):
    """
    初始化日志系统

    应在应用启动时调用一次。

    Args:
        db_insert_func: 可选的数据库插入函数
    """
    global _log_manager
    _log_manager = LogManager.get_instance()
    _log_manager.setup(db_insert_func)

    # 尝试保存当前事件循环
    try:
        loop = asyncio.get_event_loop()
        _log_manager.set_event_loop(loop)
    except RuntimeError:
        pass

    logger.info("Unified logging system initialized")


def get_log_manager() -> LogManager:
    """获取日志管理器实例"""
    global _log_manager
    if _log_manager is None:
        _log_manager = LogManager.get_instance()
    return _log_manager


class LogContext:
    """
    日志上下文管理器

    用法:
        with LogContext(spider_project_id=123):
            logger.info("这条日志会关联到项目 123")
    """

    def __init__(self, **kwargs):
        self._context = kwargs
        self._previous_context = {}

    def __enter__(self):
        log_manager = get_log_manager()
        self._previous_context = log_manager.get_context()
        log_manager.set_context(**self._context)
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        log_manager = get_log_manager()
        log_manager.clear_context()
        if self._previous_context:
            log_manager.set_context(**self._previous_context)
        return False


async def log_context(spider_project_id: Optional[int] = None, **kwargs):
    """
    异步日志上下文管理器

    用法:
        async with log_context(spider_project_id=123):
            logger.info("这条日志会关联到项目 123")
    """
    return LogContext(spider_project_id=spider_project_id, **kwargs)
