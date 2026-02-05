# -*- coding: utf-8 -*-
"""
Redis 请求队列管理器

管理爬虫请求的优先级队列，支持去重、超时恢复、暂停/恢复等功能。

Redis 键名设计：
- spider:{project_id}:pending    - ZSET 待处理队列（score=优先级）
- spider:{project_id}:processing - HASH 处理中（field=fingerprint, value=start_time）
- spider:{project_id}:seen       - SET 已入队URL指纹（防重复入队）
- spider:{project_id}:completed  - SET 已完成URL指纹（断点续抓跳过）
- spider:{project_id}:stats      - HASH 实时统计
- spider:{project_id}:state      - STRING 任务状态
"""

import json
import time
from dataclasses import dataclass
from typing import Optional, Dict, Any, TYPE_CHECKING
from loguru import logger

if TYPE_CHECKING:
    from redis.asyncio import Redis

from .request import Request


@dataclass
class QueueStats:
    """队列统计信息"""
    total: int = 0          # 总请求数
    completed: int = 0      # 成功数
    failed: int = 0         # 失败数
    retried: int = 0        # 重试次数
    pending: int = 0        # 队列中等待
    processing: int = 0     # 处理中

    @property
    def success_rate(self) -> float:
        """成功率"""
        total_done = self.completed + self.failed
        if total_done == 0:
            return 0.0
        return round(self.completed / total_done * 100, 2)

    def to_dict(self) -> Dict[str, Any]:
        return {
            'total': self.total,
            'completed': self.completed,
            'failed': self.failed,
            'retried': self.retried,
            'pending': self.pending,
            'processing': self.processing,
            'success_rate': self.success_rate,
        }


class RequestQueue:
    """
    Redis 优先级队列管理器

    管理爬虫请求的生命周期：入队 -> 处理中 -> 完成/失败/重试

    状态机：
        idle -> running -> paused -> running -> completed/stopped
    """

    # 任务状态常量
    STATE_IDLE = 'idle'
    STATE_RUNNING = 'running'
    STATE_PAUSED = 'paused'
    STATE_STOPPED = 'stopped'
    STATE_COMPLETED = 'completed'

    # 处理超时时间（秒）
    PROCESSING_TIMEOUT = 300  # 5分钟

    def __init__(self, redis: 'Redis', project_id: int, is_test: bool = False):
        """
        初始化队列管理器

        Args:
            redis: Redis 客户端
            project_id: 项目ID
            is_test: 是否为测试模式（使用独立的键前缀）
        """
        self.redis = redis
        self.project_id = project_id
        self.is_test = is_test

        # Redis 键名 - 根据模式选择前缀
        prefix = "test_spider" if is_test else "spider"
        self._key_prefix = f"{prefix}:{project_id}"
        self._key_pending = f"{self._key_prefix}:pending"
        self._key_processing = f"{self._key_prefix}:processing"
        self._key_seen = f"{self._key_prefix}:seen"
        self._key_completed = f"{self._key_prefix}:completed"
        self._key_stats = f"{self._key_prefix}:stats"
        self._key_state = f"{self._key_prefix}:state"
        self._key_item_count = f"{self._key_prefix}:item_count"  # 最终数据计数
        self._key_queued_count = f"{self._key_prefix}:queued_count"  # 回调产出的请求入队计数

    async def push(self, request: Request) -> bool:
        """
        将请求加入队列

        Args:
            request: 请求对象

        Returns:
            bool: 是否成功入队（False 表示已存在被跳过）
        """
        fingerprint = request.fingerprint()

        # 检查是否已入队（除非 dont_filter=True）
        if not request.dont_filter:
            is_seen = await self.redis.sismember(self._key_seen, fingerprint)
            if is_seen:
                logger.debug(f"Request already seen, skipped: {request.url[:50]}")
                return False

        # 加入 seen 集合
        await self.redis.sadd(self._key_seen, fingerprint)

        # 计算分数（优先级越高分数越大，使用负数是为了 ZPOPMIN 能取到最高优先级）
        # score = -priority + timestamp 保证同优先级按入队顺序处理
        score = -request.priority + time.time() / 1e10

        # 存储请求数据（JSON）
        request_data = request.to_json()
        await self.redis.zadd(self._key_pending, {request_data: score})

        # 更新统计（只统计详情页）
        if request.callback_name == 'parse_detail':
            await self.redis.hincrby(self._key_stats, 'total', 1)

        logger.debug(f"Request pushed: {request.url[:50]}, priority={request.priority}")
        return True

    async def push_many(self, requests: list) -> int:
        """
        批量入队

        Args:
            requests: 请求列表

        Returns:
            int: 成功入队数量
        """
        count = 0
        for request in requests:
            if await self.push(request):
                count += 1
        return count

    async def pop(self) -> Optional[Request]:
        """
        从队列取出一个请求

        Returns:
            Request: 请求对象，队列为空返回 None
        """
        # 检查状态
        state = await self.get_state()
        if state in (self.STATE_PAUSED, self.STATE_STOPPED):
            logger.debug(f"pop() skipped: state is {state}")
            return None

        # 从 pending 取出（ZPOPMIN 取分数最小的，即优先级最高的）
        import asyncio
        try:
            result = await asyncio.wait_for(
                self.redis.zpopmin(self._key_pending, count=1),
                timeout=5.0
            )
        except asyncio.TimeoutError:
            logger.error(f"Redis zpopmin timeout after 5s, key={self._key_pending}")
            return None

        if not result:
            return None

        request_data, _ = result[0]
        if isinstance(request_data, bytes):
            request_data = request_data.decode('utf-8')

        request = Request.from_json(request_data)

        # 加入 processing（记录开始时间）
        fingerprint = request.fingerprint()
        await self.redis.hset(
            self._key_processing,
            fingerprint,
            json.dumps({
                'request': request_data,
                'start_time': time.time(),
            })
        )

        logger.debug(f"Request popped: {request.url[:50]}")
        return request

    async def complete(self, request: Request, success: bool = True) -> None:
        """
        标记请求完成

        Args:
            request: 请求对象
            success: 是否成功
        """
        fingerprint = request.fingerprint()

        # 从 processing 移除
        await self.redis.hdel(self._key_processing, fingerprint)

        if success:
            # 加入 completed 集合
            await self.redis.sadd(self._key_completed, fingerprint)
            # 只统计详情页
            if request.callback_name == 'parse_detail':
                await self.redis.hincrby(self._key_stats, 'completed', 1)
            logger.debug(f"Request completed: {request.url[:50]}")
        else:
            # 只统计详情页
            if request.callback_name == 'parse_detail':
                await self.redis.hincrby(self._key_stats, 'failed', 1)
            logger.debug(f"Request failed: {request.url[:50]}")

    async def retry(self, request: Request) -> bool:
        """
        重试请求

        Args:
            request: 请求对象

        Returns:
            bool: 是否还能重试（False 表示已超过最大重试次数）
        """
        fingerprint = request.fingerprint()

        # 从 processing 移除
        await self.redis.hdel(self._key_processing, fingerprint)

        # 检查重试次数
        if request.retry_count >= request.max_retries:
            logger.warning(f"Request exceeded max retries: {request.url[:50]}")
            return False

        # 增加重试计数
        new_request = request.replace(retry_count=request.retry_count + 1)

        # 重新入队（强制入队，不检查去重）
        score = -new_request.priority + time.time() / 1e10
        await self.redis.zadd(self._key_pending, {new_request.to_json(): score})
        # 只统计详情页
        if request.callback_name == 'parse_detail':
            await self.redis.hincrby(self._key_stats, 'retried', 1)

        logger.debug(f"Request retry {new_request.retry_count}/{new_request.max_retries}: {request.url[:50]}")
        return True

    async def get_stats(self) -> QueueStats:
        """获取队列统计信息"""
        # 获取基础统计
        stats_data = await self.redis.hgetall(self._key_stats)

        # 转换类型
        def _get_int(key: str) -> int:
            val = stats_data.get(key) or stats_data.get(key.encode())
            if val is None:
                return 0
            if isinstance(val, bytes):
                val = val.decode()
            return int(val)

        # 获取队列长度
        pending = await self.redis.zcard(self._key_pending)
        processing = await self.redis.hlen(self._key_processing)

        return QueueStats(
            total=_get_int('total'),
            completed=_get_int('completed'),
            failed=_get_int('failed'),
            retried=_get_int('retried'),
            pending=pending,
            processing=processing,
        )

    async def get_state(self) -> str:
        """获取任务状态"""
        state = await self.redis.get(self._key_state)
        if state is None:
            return self.STATE_IDLE
        if isinstance(state, bytes):
            state = state.decode()
        return state

    async def set_state(self, state: str) -> None:
        """设置任务状态"""
        await self.redis.set(self._key_state, state)
        logger.info(f"Project {self.project_id} state changed to: {state}")

    async def pause(self) -> None:
        """暂停任务"""
        await self.set_state(self.STATE_PAUSED)

    async def resume(self) -> None:
        """恢复任务"""
        await self.set_state(self.STATE_RUNNING)

    async def stop(self, clear_queue: bool = False) -> None:
        """
        停止任务

        Args:
            clear_queue: 是否清空队列数据
        """
        await self.set_state(self.STATE_STOPPED)

        if clear_queue:
            await self.clear()

    async def clear(self) -> None:
        """清空所有队列数据"""
        keys = [
            self._key_pending,
            self._key_processing,
            self._key_seen,
            self._key_completed,
            self._key_stats,
            self._key_state,
            self._key_item_count,
            self._key_queued_count,
        ]
        await self.redis.delete(*keys)
        logger.info(f"Project {self.project_id} queue cleared")

    async def recover_timeout(self) -> int:
        """
        恢复超时的请求

        将 processing 中超时的请求重新放回 pending

        Returns:
            int: 恢复的请求数量
        """
        recovered = 0
        current_time = time.time()

        # 获取所有 processing 中的请求
        processing_data = await self.redis.hgetall(self._key_processing)

        for fingerprint, data in processing_data.items():
            if isinstance(fingerprint, bytes):
                fingerprint = fingerprint.decode()
            if isinstance(data, bytes):
                data = data.decode()

            try:
                info = json.loads(data)
                start_time = info.get('start_time', 0)

                # 检查是否超时
                if current_time - start_time > self.PROCESSING_TIMEOUT:
                    # 重新入队
                    request_data = info['request']
                    request = Request.from_json(request_data)

                    # 增加重试计数
                    new_request = request.replace(retry_count=request.retry_count + 1)

                    if new_request.retry_count <= new_request.max_retries:
                        score = -new_request.priority + time.time() / 1e10
                        await self.redis.zadd(self._key_pending, {new_request.to_json(): score})
                        # 只统计详情页
                        if request.callback_name == 'parse_detail':
                            await self.redis.hincrby(self._key_stats, 'retried', 1)
                        recovered += 1
                        logger.info(f"Recovered timeout request: {request.url[:50]}")
                    else:
                        # 只统计详情页
                        if request.callback_name == 'parse_detail':
                            await self.redis.hincrby(self._key_stats, 'failed', 1)
                        logger.warning(f"Timeout request exceeded max retries: {request.url[:50]}")

                    # 从 processing 移除
                    await self.redis.hdel(self._key_processing, fingerprint)

            except Exception as e:
                logger.error(f"Error recovering request: {e}")
                await self.redis.hdel(self._key_processing, fingerprint)

        if recovered:
            logger.info(f"Recovered {recovered} timeout requests for project {self.project_id}")

        return recovered

    async def is_empty(self) -> bool:
        """检查队列是否为空（pending 和 processing 都为空）"""
        pending = await self.redis.zcard(self._key_pending)
        processing = await self.redis.hlen(self._key_processing)
        return pending == 0 and processing == 0

    async def get_item_count(self) -> int:
        """获取已产出的数据条数"""
        count = await self.redis.get(self._key_item_count)
        if count is None:
            return 0
        if isinstance(count, bytes):
            count = count.decode()
        return int(count)

    async def incr_item_count(self) -> int:
        """递增数据计数，返回新值"""
        return await self.redis.incr(self._key_item_count)

    async def get_queued_count(self) -> int:
        """获取回调产出的请求入队数量"""
        count = await self.redis.get(self._key_queued_count)
        if count is None:
            return 0
        if isinstance(count, bytes):
            count = count.decode()
        return int(count)

    async def incr_queued_count(self) -> int:
        """递增回调请求入队计数，返回新值"""
        return await self.redis.incr(self._key_queued_count)

    async def is_url_completed(self, url: str) -> bool:
        """检查 URL 是否已完成（用于断点续抓）"""
        # 创建一个临时 Request 来获取 fingerprint
        temp_request = Request(url=url)
        fingerprint = temp_request.fingerprint()
        return await self.redis.sismember(self._key_completed, fingerprint)

    async def get_pending_count(self) -> int:
        """获取待处理请求数量"""
        return await self.redis.zcard(self._key_pending)

    async def get_processing_count(self) -> int:
        """获取处理中请求数量"""
        return await self.redis.hlen(self._key_processing)
