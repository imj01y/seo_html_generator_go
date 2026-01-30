# -*- coding: utf-8 -*-
"""
分布式任务锁

基于 Redis 实现分布式锁，防止多个 worker 同时处理同一任务。
"""

import time
import uuid
from typing import Optional, Dict, List
from loguru import logger


class DistributedLock:
    """
    Redis 分布式锁

    特性：
    - 原子性获取和释放
    - 支持锁续期
    - 防止误释放（通过 token 校验）
    """

    # Lua 脚本：只有 token 匹配才删除
    RELEASE_SCRIPT = """
    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("del", KEYS[1])
    else
        return 0
    end
    """

    # Lua 脚本：只有 token 匹配才续期
    EXTEND_SCRIPT = """
    if redis.call("get", KEYS[1]) == ARGV[1] then
        return redis.call("expire", KEYS[1], ARGV[2])
    else
        return 0
    end
    """

    def __init__(self, redis_client, lock_prefix: str = "lock:"):
        """
        初始化分布式锁

        Args:
            redis_client: Redis 异步客户端
            lock_prefix: 锁键前缀
        """
        self.redis = redis_client
        self.lock_prefix = lock_prefix

    def _generate_token(self) -> str:
        """生成唯一 token"""
        return f"{time.time()}:{uuid.uuid4().hex[:8]}"

    async def acquire(
        self,
        resource: str,
        ttl: int = 300,
        retry: int = 0,
        retry_delay: float = 0.1
    ) -> Optional[str]:
        """
        获取锁

        Args:
            resource: 资源标识（如数据源ID）
            ttl: 锁过期时间（秒），防止死锁
            retry: 重试次数（0=不重试）
            retry_delay: 重试间隔（秒）

        Returns:
            锁 token（成功）或 None（失败）
        """
        lock_key = f"{self.lock_prefix}{resource}"
        token = self._generate_token()

        for attempt in range(retry + 1):
            # SET NX EX 原子操作
            success = await self.redis.set(lock_key, token, nx=True, ex=ttl)
            if success:
                logger.debug(f"Lock acquired: {resource} (ttl={ttl}s)")
                return token

            if attempt < retry:
                import asyncio
                await asyncio.sleep(retry_delay)

        logger.debug(f"Failed to acquire lock: {resource}")
        return None

    async def release(self, resource: str, token: str) -> bool:
        """
        释放锁

        只能释放自己持有的锁（通过 token 校验）。

        Args:
            resource: 资源标识
            token: acquire() 返回的 token

        Returns:
            是否成功释放
        """
        lock_key = f"{self.lock_prefix}{resource}"

        try:
            result = await self.redis.eval(
                self.RELEASE_SCRIPT, 1, lock_key, token
            )
            success = result == 1
            if success:
                logger.debug(f"Lock released: {resource}")
            return success
        except Exception as e:
            logger.error(f"Failed to release lock {resource}: {e}")
            return False

    async def extend(self, resource: str, token: str, ttl: int = 300) -> bool:
        """
        延长锁的过期时间

        Args:
            resource: 资源标识
            token: acquire() 返回的 token
            ttl: 新的过期时间（秒）

        Returns:
            是否成功延期
        """
        lock_key = f"{self.lock_prefix}{resource}"

        try:
            result = await self.redis.eval(
                self.EXTEND_SCRIPT, 1, lock_key, token, ttl
            )
            success = result == 1
            if success:
                logger.debug(f"Lock extended: {resource} (ttl={ttl}s)")
            return success
        except Exception as e:
            logger.error(f"Failed to extend lock {resource}: {e}")
            return False

    async def is_locked(self, resource: str) -> bool:
        """检查资源是否被锁定"""
        lock_key = f"{self.lock_prefix}{resource}"
        return await self.redis.exists(lock_key)


class CrawlTaskLock:
    """
    爬虫任务锁

    封装 DistributedLock，提供更方便的 API。
    自动管理 token，防止多个 worker 同时抓取同一数据源。
    """

    def __init__(self, redis_client):
        """
        初始化爬虫任务锁

        Args:
            redis_client: Redis 异步客户端
        """
        self._lock = DistributedLock(redis_client, "lock:crawl:")
        self._tokens: Dict[int, str] = {}  # source_id -> token

    async def try_lock_source(self, source_id: int, ttl: int = 600) -> bool:
        """
        尝试锁定数据源

        Args:
            source_id: 数据源ID
            ttl: 锁过期时间（秒），默认10分钟

        Returns:
            是否成功获取锁
        """
        # 检查是否已持有锁
        if source_id in self._tokens:
            logger.debug(f"Already holding lock for source {source_id}")
            return True

        token = await self._lock.acquire(str(source_id), ttl)
        if token:
            self._tokens[source_id] = token
            logger.info(f"Locked source {source_id}")
            return True

        logger.debug(f"Source {source_id} is locked by another worker")
        return False

    async def unlock_source(self, source_id: int) -> bool:
        """
        解锁数据源

        Args:
            source_id: 数据源ID

        Returns:
            是否成功释放
        """
        token = self._tokens.pop(source_id, None)
        if token:
            success = await self._lock.release(str(source_id), token)
            if success:
                logger.info(f"Unlocked source {source_id}")
            return success
        return False

    async def extend_lock(self, source_id: int, ttl: int = 600) -> bool:
        """
        延长锁（长时间任务使用）

        Args:
            source_id: 数据源ID
            ttl: 新的过期时间（秒）

        Returns:
            是否成功延期
        """
        token = self._tokens.get(source_id)
        if token:
            return await self._lock.extend(str(source_id), token, ttl)
        return False

    async def unlock_all(self):
        """释放所有持有的锁"""
        for source_id in list(self._tokens.keys()):
            await self.unlock_source(source_id)

    def get_locked_sources(self) -> List[int]:
        """获取当前持有锁的数据源ID列表"""
        return list(self._tokens.keys())

    async def is_source_locked(self, source_id: int) -> bool:
        """检查数据源是否被锁定（任何 worker）"""
        return await self._lock.is_locked(str(source_id))
