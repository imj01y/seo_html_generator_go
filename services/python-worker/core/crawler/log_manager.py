# -*- coding: utf-8 -*-
"""
实时日志管理器

支持按会话收集和订阅日志，用于 SSE 推送。
支持从子线程安全添加日志。
"""

import asyncio
import threading
import time
from typing import Dict, List, Optional
from dataclasses import dataclass, asdict
from datetime import datetime
from loguru import logger


@dataclass
class LogEntry:
    """日志条目"""
    timestamp: str
    level: str  # INFO, ERROR, REQUEST, ITEM, PRINT, WARNING
    message: str

    def to_dict(self) -> dict:
        return asdict(self)


class LogManager:
    """
    实时日志管理器

    使用发布-订阅模式，支持多个订阅者同时接收日志。
    支持从子线程安全添加日志。
    """

    _instance: Optional['LogManager'] = None
    _lock = threading.Lock()

    def __init__(self):
        # session_id -> [LogEntry]
        self._logs: Dict[str, List[LogEntry]] = {}
        # session_id -> [asyncio.Queue]
        self._subscribers: Dict[str, List[asyncio.Queue]] = {}
        # session_id -> bool (是否已结束)
        self._ended: Dict[str, bool] = {}
        # session_id -> 创建时间戳（用于版本跟踪，防止延迟清理误删新会话）
        self._session_versions: Dict[str, float] = {}
        # 保存主事件循环引用
        self._main_loop: Optional[asyncio.AbstractEventLoop] = None

    @classmethod
    def get_instance(cls) -> 'LogManager':
        """获取单例实例"""
        if cls._instance is None:
            cls._instance = LogManager()
        return cls._instance

    def set_event_loop(self, loop: asyncio.AbstractEventLoop):
        """设置主事件循环引用"""
        self._main_loop = loop

    def create_session(self, session_id: str):
        """创建日志会话（如果已存在且活跃则跳过，否则重新创建）"""
        # 保存当前事件循环
        try:
            self._main_loop = asyncio.get_event_loop()
        except RuntimeError:
            pass

        with self._lock:
            # 如果会话已存在且未结束，跳过创建
            if session_id in self._logs and not self._ended.get(session_id, False):
                logger.debug(f"Log session already exists and active: {session_id}")
                return

            # 如果会话已存在但已结束，清理旧订阅者后重新创建
            if session_id in self._logs:
                logger.debug(f"Recreating ended session: {session_id}")
                old_subscribers = self._subscribers.get(session_id, [])[:]
                for queue in old_subscribers:
                    try:
                        queue.put_nowait(None)
                    except asyncio.QueueFull:
                        pass

            # 创建/重置会话
            self._logs[session_id] = []
            self._subscribers[session_id] = []
            self._ended[session_id] = False
            self._session_versions[session_id] = time.time()  # 记录版本时间戳

        logger.debug(f"Log session created: {session_id}")

    def close_session(self, session_id: str):
        """关闭日志会话"""
        with self._lock:
            self._ended[session_id] = True
            subscribers = self._subscribers.get(session_id, [])[:]  # 复制列表
            version = self._session_versions.get(session_id)  # 获取当前版本

        # 通知所有订阅者会话结束（在锁外面执行）
        for queue in subscribers:
            try:
                queue.put_nowait(None)  # None 表示结束
            except asyncio.QueueFull:
                pass

        # 延迟清理，让用户有足够时间查看历史日志
        # 传入版本号，防止误删新创建的会话
        try:
            loop = asyncio.get_event_loop()
            loop.call_later(60.0, lambda v=version: self._cleanup_session(session_id, v))
        except RuntimeError:
            # 如果不在事件循环中，直接清理
            self._cleanup_session(session_id, version)

        logger.debug(f"Log session closed: {session_id}")

    def _cleanup_session(self, session_id: str, expected_version: float = None):
        """清理会话资源"""
        with self._lock:
            # 检查版本，如果会话已被重新创建则跳过清理
            current_version = self._session_versions.get(session_id)
            if expected_version is not None and current_version != expected_version:
                logger.debug(f"Skip cleanup for {session_id}: version mismatch (expected={expected_version}, current={current_version})")
                return

            self._logs.pop(session_id, None)
            self._subscribers.pop(session_id, None)
            self._ended.pop(session_id, None)
            self._session_versions.pop(session_id, None)

    def force_cleanup_session(self, session_id: str):
        """强制立即清理会话（用于项目重置）"""
        with self._lock:
            if session_id in self._ended:
                self._ended[session_id] = True
            subscribers = self._subscribers.get(session_id, [])[:]

        for queue in subscribers:
            try:
                queue.put_nowait(None)
            except asyncio.QueueFull:
                pass

        # 传入 None 表示强制清理，不检查版本
        self._cleanup_session(session_id, None)
        logger.debug(f"Force cleaned up session: {session_id}")

    def _push_to_subscribers(self, session_id: str, entry: LogEntry):
        """推送日志给订阅者（在主线程中执行）"""
        with self._lock:
            subscribers = self._subscribers.get(session_id, [])[:]  # 复制列表

        for queue in subscribers:
            try:
                queue.put_nowait(entry)
            except asyncio.QueueFull:
                pass  # 队列满了就丢弃

    def add_log(self, session_id: str, level: str, message: str):
        """
        添加日志（线程安全）

        可以从任意线程调用，日志会被安全地推送到主事件循环。
        """
        with self._lock:
            if session_id not in self._logs:
                return

            if self._ended.get(session_id, False):
                return

            entry = LogEntry(
                timestamp=datetime.now().strftime("%H:%M:%S"),
                level=level,
                message=message
            )
            self._logs[session_id].append(entry)

        # 检查是否在主线程中
        try:
            loop = asyncio.get_running_loop()
            # 在事件循环中，直接推送
            self._push_to_subscribers(session_id, entry)
        except RuntimeError:
            # 不在事件循环中（子线程），使用 call_soon_threadsafe
            if self._main_loop and self._main_loop.is_running():
                self._main_loop.call_soon_threadsafe(
                    self._push_to_subscribers, session_id, entry
                )

        logger.debug(f"[{session_id}] [{level}] {message}")

    def subscribe(self, session_id: str) -> asyncio.Queue:
        """
        订阅日志流

        Returns:
            asyncio.Queue: 日志队列，接收 LogEntry 对象，None 表示结束
        """
        queue: asyncio.Queue = asyncio.Queue(maxsize=1000)

        with self._lock:
            if session_id not in self._subscribers:
                self._subscribers[session_id] = []

            self._subscribers[session_id].append(queue)

            # 发送历史日志
            for entry in self._logs.get(session_id, []):
                try:
                    queue.put_nowait(entry)
                except asyncio.QueueFull:
                    break

            # 如果会话已结束，发送结束信号
            if self._ended.get(session_id, False):
                try:
                    queue.put_nowait(None)
                except asyncio.QueueFull:
                    pass

        return queue

    def unsubscribe(self, session_id: str, queue: asyncio.Queue):
        """取消订阅"""
        with self._lock:
            if session_id in self._subscribers:
                try:
                    self._subscribers[session_id].remove(queue)
                except ValueError:
                    pass

    def is_session_active(self, session_id: str) -> bool:
        """检查会话是否活跃"""
        with self._lock:
            return session_id in self._logs and not self._ended.get(session_id, False)

    def has_session(self, session_id: str) -> bool:
        """检查会话是否存在（包括已结束的会话）"""
        with self._lock:
            return session_id in self._logs

    def is_session_ended(self, session_id: str) -> bool:
        """检查会话是否已结束"""
        with self._lock:
            return self._ended.get(session_id, False)

    def get_logs(self, session_id: str) -> List[LogEntry]:
        """获取会话的所有日志"""
        with self._lock:
            return self._logs.get(session_id, []).copy()


# 全局实例
log_manager = LogManager.get_instance()
