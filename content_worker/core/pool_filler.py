# -*- coding: utf-8 -*-
"""
缓存池填充器

监控 Redis 队列长度，低于阈值时从数据库补充数据。
"""
import asyncio
import json
from typing import Optional

from loguru import logger


class PoolFiller:
    """标题和正文缓存池填充器"""

    def __init__(
        self,
        redis_client,
        db_pool,
        group_id: int,
        pool_size: int = 5000,
        threshold: int = 1000,
        batch_size: int = 500,
    ):
        """
        初始化填充器

        Args:
            redis_client: Redis 客户端
            db_pool: 数据库连接池
            group_id: 分组 ID
            pool_size: 池大小（默认 5000）
            threshold: 补充阈值（默认 1000，即 20%）
            batch_size: 每次补充数量（默认 500）
        """
        self.redis = redis_client
        self.db = db_pool
        self.group_id = group_id
        self.pool_size = pool_size
        self.threshold = threshold
        self.batch_size = batch_size

    def _get_key(self, pool_type: str) -> str:
        """获取 Redis key"""
        return f"{pool_type}:pool:{self.group_id}"

    def _get_lock_key(self, pool_type: str) -> str:
        """获取补充锁 key"""
        return f"{self._get_key(pool_type)}:filling"

    async def check_and_fill(self, pool_type: str) -> int:
        """
        检查队列长度，低于阈值时补充

        Args:
            pool_type: 池类型 ("titles" 或 "contents")

        Returns:
            补充的数据条数
        """
        key = self._get_key(pool_type)
        lock_key = self._get_lock_key(pool_type)

        # 1. 检查队列长度
        length = await self.redis.llen(key)
        if length >= self.threshold:
            return 0

        # 2. 尝试获取补充锁（防止并发）
        acquired = await self.redis.set(lock_key, "1", nx=True, ex=60)
        if not acquired:
            logger.debug(f"[{pool_type}:{self.group_id}] Another filler is working")
            return 0

        try:
            # 3. 计算需要补充的数量
            need = min(self.pool_size - length, self.batch_size)
            logger.info(f"[{pool_type}:{self.group_id}] Pool low ({length}/{self.threshold}), filling {need} items")

            # 4. 从 DB 查询可用数据
            items = await self._fetch_available(pool_type, need)
            if not items:
                logger.warning(f"[{pool_type}:{self.group_id}] No available items in DB")
                return 0

            # 5. 批量入队
            filled = await self._push_to_queue(key, items)
            logger.info(f"[{pool_type}:{self.group_id}] Filled {filled} items, new length: {length + filled}")
            return filled

        finally:
            await self.redis.delete(lock_key)

    async def _fetch_available(self, pool_type: str, limit: int) -> list[dict]:
        """
        从数据库查询可用数据

        Args:
            pool_type: 池类型 ("titles" 或 "contents")
            limit: 查询数量

        Returns:
            数据列表 [{"id": int, "text": str}, ...]
        """
        column = "title" if pool_type == "titles" else "content"
        query = f"""
            SELECT id, {column} as text FROM {pool_type}
            WHERE group_id = %s AND status = 1
            ORDER BY batch_id DESC, id ASC
            LIMIT %s
        """

        async with self.db.acquire() as conn:
            async with conn.cursor() as cur:
                await cur.execute(query, (self.group_id, limit))
                rows = await cur.fetchall()
                return [{"id": row[0], "text": row[1]} for row in rows]

    async def _push_to_queue(self, key: str, items: list[dict]) -> int:
        """
        批量入队到 Redis

        Args:
            key: Redis key
            items: 数据列表

        Returns:
            成功入队的数量
        """
        if not items:
            return 0

        # 使用 pipeline 批量 LPUSH
        pipe = self.redis.pipeline()
        for item in items:
            data = json.dumps(item, ensure_ascii=False)
            pipe.lpush(key, data)

        await pipe.execute()
        return len(items)

    async def get_pool_stats(self, pool_type: str) -> dict:
        """获取池状态统计"""
        key = self._get_key(pool_type)
        length = await self.redis.llen(key)
        return {
            "pool_type": pool_type,
            "group_id": self.group_id,
            "length": length,
            "pool_size": self.pool_size,
            "threshold": self.threshold,
            "utilization": round(length / self.pool_size * 100, 2) if self.pool_size > 0 else 0,
        }


class PoolFillerManager:
    """管理多个分组的缓存池填充"""

    def __init__(self, redis_client, db_pool):
        self.redis = redis_client
        self.db = db_pool
        self.fillers: dict[int, PoolFiller] = {}
        self._running = False
        self._task: Optional[asyncio.Task] = None

    def add_group(self, group_id: int, **kwargs) -> None:
        """添加一个分组的填充器"""
        self.fillers[group_id] = PoolFiller(
            self.redis, self.db, group_id, **kwargs
        )
        logger.info(f"Added PoolFiller for group {group_id}")

    async def start(self, check_interval: float = 5.0) -> None:
        """启动填充循环"""
        if self._running:
            return

        self._running = True
        self._task = asyncio.create_task(self._fill_loop(check_interval))
        logger.info(f"PoolFillerManager started with {len(self.fillers)} groups")

    async def stop(self) -> None:
        """停止填充循环"""
        self._running = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
        logger.info("PoolFillerManager stopped")

    async def _fill_loop(self, interval: float) -> None:
        """填充循环"""
        while self._running:
            for group_id, filler in self.fillers.items():
                try:
                    await filler.check_and_fill("titles")
                    await filler.check_and_fill("contents")
                except Exception as e:
                    logger.error(f"Fill error for group {group_id}: {e}")

            await asyncio.sleep(interval)

    async def fill_all_now(self) -> dict[int, dict]:
        """立即填充所有分组（启动时调用）"""
        results = {}
        for group_id, filler in self.fillers.items():
            try:
                titles_filled = await filler.check_and_fill("titles")
                contents_filled = await filler.check_and_fill("contents")
                results[group_id] = {
                    "titles": titles_filled,
                    "contents": contents_filled,
                }
            except Exception as e:
                logger.error(f"Initial fill error for group {group_id}: {e}")
                results[group_id] = {"error": str(e)}
        return results

    async def get_all_stats(self) -> dict[int, dict]:
        """获取所有分组的池状态"""
        stats = {}
        for group_id, filler in self.fillers.items():
            stats[group_id] = {
                "titles": await filler.get_pool_stats("titles"),
                "contents": await filler.get_pool_stats("contents"),
            }
        return stats
