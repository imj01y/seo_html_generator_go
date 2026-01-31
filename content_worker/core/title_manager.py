# -*- coding: utf-8 -*-
"""
标题管理器
支持亿级标题的高性能随机抽取，已使用标记，分层优先最新
"""

import random
from typing import Any, Dict, List, Optional

from loguru import logger


class TitleManager:
    """
    标题管理器（分层随机抽取 + Redis Bitmap标记已使用）

    特性：
    - 分层抽取：优先最新标题但保证随机性
    - Bitmap标记：高效记录已使用状态（1亿仅占12.5MB）
    - 内存ID池：Shuffle后顺序取出，保证短期不重复
    - Redis缓存：标题内容缓存，减少MySQL访问
    """

    # 分层权重配置
    LAYER_WEIGHTS = {
        'newest': 0.50,  # 最新层 50%
        'middle': 0.35,  # 中间层 35%
        'oldest': 0.15   # 历史层 15%
    }

    # 缓存配置
    CACHE_PREFIX = "title:"
    BITMAP_PREFIX = "title:used:"
    CACHE_TTL = 86400  # 标题内容缓存1天

    def __init__(self, redis_client, db_pool, group_id: int = 1):
        """
        初始化标题管理器

        Args:
            redis_client: Redis客户端
            db_pool: MySQL连接池
            group_id: 分组ID
        """
        self.redis = redis_client
        self.db_pool = db_pool
        self.group_id = group_id

        # 三层ID池
        self.pools: Dict[str, Dict[str, Any]] = {
            'newest': {'ids': [], 'cursor': 0},
            'middle': {'ids': [], 'cursor': 0},
            'oldest': {'ids': [], 'cursor': 0}
        }

        # 统计信息
        self._total_loaded = 0
        self._max_batch_id = 0
        self._loaded = False

    async def load_pools(self, max_size: int = 500000) -> int:
        """
        加载三层ID池

        Args:
            max_size: 每层最大加载数量

        Returns:
            加载的总ID数量
        """
        # 获取最大batch_id用于分层
        self._max_batch_id = await self._get_max_batch_id()

        if self._max_batch_id == 0:
            logger.warning("No titles found in database")
            self._loaded = True
            return 0

        # 计算分层边界
        newest_threshold = max(0, self._max_batch_id - 10)   # 最近10个批次
        middle_threshold = max(0, self._max_batch_id - 50)   # 中间40个批次

        layer_bounds = [
            ('newest', newest_threshold, self._max_batch_id + 1),
            ('middle', middle_threshold, newest_threshold),
            ('oldest', 0, middle_threshold)
        ]

        total = 0
        for layer, min_batch, max_batch in layer_bounds:
            ids = await self._load_layer_ids(min_batch, max_batch, max_size)
            # 过滤已使用的ID
            unused_ids = await self._filter_unused_ids(ids)
            random.shuffle(unused_ids)
            self.pools[layer]['ids'] = unused_ids
            self.pools[layer]['cursor'] = 0
            total += len(unused_ids)
            logger.info(f"Loaded {len(unused_ids)} titles for layer '{layer}' (batch {min_batch}-{max_batch})")

        self._total_loaded = total
        self._loaded = True
        logger.info(f"TitleManager loaded {total} titles in 3 layers")
        return total

    async def _get_max_batch_id(self) -> int:
        """获取最大批次号"""
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(
                    "SELECT MAX(batch_id) FROM titles WHERE group_id = %s",
                    (self.group_id,)
                )
                row = await cursor.fetchone()
                return row[0] if row and row[0] else 0

    async def _load_layer_ids(self, min_batch: int, max_batch: int, limit: int) -> List[int]:
        """加载指定批次范围的ID"""
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(
                    "SELECT id FROM titles WHERE group_id = %s AND batch_id >= %s AND batch_id < %s LIMIT %s",
                    (self.group_id, min_batch, max_batch, limit)
                )
                rows = await cursor.fetchall()
                return [row[0] for row in rows]

    async def _filter_unused_ids(self, ids: List[int]) -> List[int]:
        """使用Bitmap过滤已使用的ID"""
        if not ids:
            return []

        bitmap_key = f"{self.BITMAP_PREFIX}{self.group_id}"

        try:
            pipe = self.redis.pipeline()
            for tid in ids:
                pipe.getbit(bitmap_key, tid)
            results = await pipe.execute()
            return [tid for tid, used in zip(ids, results) if not used]
        except Exception as e:
            logger.error(f"Failed to filter unused ids: {e}")
            return ids  # 出错时返回全部

    def _select_layer_by_weight(self) -> str:
        """按权重随机选择层"""
        r = random.random()
        cumulative = 0
        for layer, weight in self.LAYER_WEIGHTS.items():
            cumulative += weight
            if r < cumulative:
                return layer
        return 'oldest'

    def _find_available_layer(self) -> Optional[str]:
        """查找有可用ID的层"""
        # 按优先级查找
        for layer in ['newest', 'middle', 'oldest']:
            pool = self.pools[layer]
            if pool['cursor'] < len(pool['ids']):
                return layer
        return None

    async def extract_titles(self, count: int = 4) -> List[str]:
        """
        抽取N个不重复标题

        Args:
            count: 抽取数量

        Returns:
            标题列表
        """
        if not self._loaded:
            await self.load_pools()

        title_ids = []
        attempts = 0
        max_attempts = count * 10  # 防止死循环

        while len(title_ids) < count and attempts < max_attempts:
            attempts += 1

            # 按权重选择层
            layer = self._select_layer_by_weight()
            pool = self.pools[layer]

            # 检查当前层是否有可用ID
            if pool['cursor'] >= len(pool['ids']):
                # 尝试其他层
                layer = self._find_available_layer()
                if layer is None:
                    # 所有层都用尽，重置
                    logger.warning("All title pools exhausted, resetting...")
                    await self.reset_used()
                    await self.load_pools()
                    if self._total_loaded == 0:
                        break
                    continue
                pool = self.pools[layer]

            # 取出ID
            title_id = pool['ids'][pool['cursor']]
            pool['cursor'] += 1
            title_ids.append(title_id)

        if not title_ids:
            return []

        # 批量标记已使用
        await self._mark_used_batch(title_ids)

        # 批量获取标题内容
        titles = await self._get_titles_batch(title_ids)

        return titles

    async def _mark_used_batch(self, title_ids: List[int]):
        """批量标记已使用（Redis Bitmap）"""
        if not title_ids:
            return

        bitmap_key = f"{self.BITMAP_PREFIX}{self.group_id}"

        try:
            pipe = self.redis.pipeline()
            for tid in title_ids:
                pipe.setbit(bitmap_key, tid, 1)
            await pipe.execute()
        except Exception as e:
            logger.error(f"Failed to mark titles as used: {e}")

    async def _get_titles_batch(self, title_ids: List[int]) -> List[str]:
        """批量获取标题内容（Redis缓存 + MySQL回源）"""
        if not title_ids:
            return []

        # 构建缓存键
        cache_keys = [f"{self.CACHE_PREFIX}{tid}" for tid in title_ids]

        # 批量从Redis获取
        try:
            cached = await self.redis.mget(*cache_keys)
        except Exception as e:
            logger.error(f"Redis mget failed: {e}")
            cached = [None] * len(title_ids)

        titles = []
        missing_ids = []
        missing_indices = []

        for i, (tid, value) in enumerate(zip(title_ids, cached)):
            if value:
                titles.append(value.decode('utf-8') if isinstance(value, bytes) else value)
            else:
                titles.append(None)
                missing_ids.append(tid)
                missing_indices.append(i)

        # 回源MySQL
        if missing_ids:
            db_titles = await self._fetch_titles_from_db(missing_ids)

            # 填充结果并写入缓存
            pipe = self.redis.pipeline()
            for idx, tid in zip(missing_indices, missing_ids):
                title = db_titles.get(tid, '')
                titles[idx] = title
                if title:
                    pipe.setex(f"{self.CACHE_PREFIX}{tid}", self.CACHE_TTL, title)

            try:
                await pipe.execute()
            except Exception as e:
                logger.warning(f"Failed to cache titles: {e}")

        # 过滤空值
        return [t for t in titles if t]

    async def _fetch_titles_from_db(self, title_ids: List[int]) -> Dict[int, str]:
        """从数据库批量获取标题"""
        if not title_ids:
            return {}

        placeholders = ','.join(['%s'] * len(title_ids))
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(
                    f"SELECT id, title FROM titles WHERE id IN ({placeholders})",
                    tuple(title_ids)
                )
                rows = await cursor.fetchall()
                return {row[0]: row[1] for row in rows}

    async def reset_used(self):
        """重置已使用标记（清空Bitmap）"""
        bitmap_key = f"{self.BITMAP_PREFIX}{self.group_id}"
        try:
            await self.redis.delete(bitmap_key)
            logger.info(f"Reset used bitmap for group {self.group_id}")
        except Exception as e:
            logger.error(f"Failed to reset used bitmap: {e}")

    async def reload(self, max_size: int = 500000) -> int:
        """重新加载ID池"""
        self._loaded = False
        for layer in self.pools:
            self.pools[layer]['ids'] = []
            self.pools[layer]['cursor'] = 0
        return await self.load_pools(max_size)

    def get_stats(self) -> Dict[str, Any]:
        """获取统计信息"""
        return {
            'group_id': self.group_id,
            'loaded': self._loaded,
            'max_batch_id': self._max_batch_id,
            'total_loaded': self._total_loaded,
            'pools': {
                layer: {
                    'total': len(pool['ids']),
                    'cursor': pool['cursor'],
                    'remaining': len(pool['ids']) - pool['cursor']
                }
                for layer, pool in self.pools.items()
            }
        }


# ============================================
# 全局实例管理（多实例模式）
# ============================================

# 全局管理器字典（按 group_id 存储）
_title_managers: Dict[int, TitleManager] = {}
_redis_client = None
_db_pool = None


async def init_title_manager(redis_client, db_pool, group_id: int = 1, max_size: int = 500000) -> TitleManager:
    """
    初始化标题管理器（支持多分组）

    Args:
        redis_client: Redis客户端
        db_pool: MySQL连接池
        group_id: 分组ID
        max_size: 每层最大加载数量

    Returns:
        TitleManager实例
    """
    global _redis_client, _db_pool
    _redis_client = redis_client
    _db_pool = db_pool

    manager = TitleManager(redis_client, db_pool, group_id)
    await manager.load_pools(max_size)
    _title_managers[group_id] = manager
    return manager


async def get_or_create_title_manager(group_id: int, max_size: int = 500000) -> Optional[TitleManager]:
    """
    获取或创建指定分组的标题管理器（懒加载）

    Args:
        group_id: 分组ID
        max_size: 每层最大加载数量

    Returns:
        TitleManager实例或None
    """
    if group_id in _title_managers:
        return _title_managers[group_id]

    if not _redis_client or not _db_pool:
        return None

    manager = TitleManager(_redis_client, _db_pool, group_id)
    await manager.load_pools(max_size)
    _title_managers[group_id] = manager
    return manager


def get_title_manager(group_id: int = 1) -> Optional[TitleManager]:
    """
    获取指定分组的标题管理器实例

    Args:
        group_id: 分组ID

    Returns:
        TitleManager实例或None
    """
    return _title_managers.get(group_id)


async def get_random_titles(count: int = 4, group_id: int = 1) -> List[str]:
    """
    获取随机标题（便捷函数）

    Args:
        count: 数量
        group_id: 分组ID

    Returns:
        标题列表
    """
    manager = _title_managers.get(group_id)
    if not manager:
        # 尝试懒加载
        manager = await get_or_create_title_manager(group_id)
    if manager:
        return await manager.extract_titles(count)
    return []
