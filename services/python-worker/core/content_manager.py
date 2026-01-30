# -*- coding: utf-8 -*-
"""
正文管理器
从 contents 表读取已生成好的正文，Shuffle随机抽取
"""

import json
import random
import logging
from typing import Optional, List, Dict, Any

logger = logging.getLogger(__name__)


class ContentManager:
    """
    正文管理器（Shuffle随机抽取）

    特性：
    - Shuffle模式：内存ID列表打乱后顺序取出
    - 可重复使用：遍历完后重新Shuffle
    - Redis缓存：正文内容缓存
    - 分层抽取：优先最新数据（按batch_id分层）
    """

    CACHE_PREFIX = "content:"
    CACHE_TTL = 86400  # 正文内容缓存1天

    # 分层比例配置（最新50%，中间35%，最老15%）
    LAYER_RATIOS = [0.50, 0.35, 0.15]

    def __init__(self, redis_client, db_pool, group_id: int = 1):
        """
        初始化正文管理器

        Args:
            redis_client: Redis客户端
            db_pool: MySQL连接池
            group_id: 分组ID
        """
        self.redis = redis_client
        self.db_pool = db_pool
        self.group_id = group_id

        # 分层ID池（按batch_id分层）
        self._layers: List[List[int]] = [[], [], []]  # 最新、中间、最老
        self._cursors: List[int] = [0, 0, 0]
        self._total: int = 0
        self._loaded: bool = False

    async def load_ids(self, max_size: int = 50000) -> int:
        """
        加载正文ID到内存（分层）

        Args:
            max_size: 最大加载数量（0=不限制）

        Returns:
            加载的ID数量
        """
        # 获取所有ID和batch_id
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                limit_clause = f" LIMIT {max_size}" if max_size > 0 else ""
                await cursor.execute(
                    f"SELECT id, batch_id FROM contents WHERE group_id = %s "
                    f"ORDER BY batch_id DESC{limit_clause}",
                    (self.group_id,)
                )
                rows = await cursor.fetchall()

        if not rows:
            self._loaded = True
            return 0

        # 按位置分层
        total = len(rows)
        layer1_end = int(total * self.LAYER_RATIOS[0])
        layer2_end = layer1_end + int(total * self.LAYER_RATIOS[1])

        self._layers[0] = [row[0] for row in rows[:layer1_end]]
        self._layers[1] = [row[0] for row in rows[layer1_end:layer2_end]]
        self._layers[2] = [row[0] for row in rows[layer2_end:]]

        # 打乱每层
        for layer in self._layers:
            random.shuffle(layer)

        self._cursors = [0, 0, 0]
        self._total = total
        self._loaded = True

        logger.info(
            f"ContentManager loaded {total} content IDs "
            f"(layers: {len(self._layers[0])}/{len(self._layers[1])}/{len(self._layers[2])})"
        )
        return total

    def _select_layer(self) -> int:
        """按权重随机选择一个层"""
        r = random.random()
        cumulative = 0.0

        # 按权重选择层
        for i, ratio in enumerate(self.LAYER_RATIOS):
            cumulative += ratio
            if r < cumulative and self._layers[i]:
                return i

        # 如果选中的层为空，选择第一个非空层
        for i, layer in enumerate(self._layers):
            if layer:
                return i
        return 0

    def _reshuffle_layer_if_needed(self, layer_idx: int):
        """游标到达末尾时重新打乱该层"""
        layer = self._layers[layer_idx]
        if layer and self._cursors[layer_idx] >= len(layer):
            random.shuffle(layer)
            self._cursors[layer_idx] = 0
            logger.debug(f"Content layer {layer_idx} reshuffled")

    async def get_random_content(self) -> str:
        """
        获取一条随机正文

        Returns:
            正文内容（已含拼音标注）
        """
        if not self._loaded or self._total == 0:
            count = await self.load_ids()
            if count == 0:
                return ""

        # 选择层并获取ID
        layer_idx = self._select_layer()
        self._reshuffle_layer_if_needed(layer_idx)

        layer = self._layers[layer_idx]
        if not layer:
            return ""

        content_id = layer[self._cursors[layer_idx]]
        self._cursors[layer_idx] += 1

        # 获取正文内容
        content = await self._get_content(content_id)
        if not content:
            # 递归重试
            return await self.get_random_content()

        return content

    async def _get_content(self, content_id: int) -> Optional[str]:
        """获取正文内容（Redis缓存 + MySQL回源）"""
        cache_key = f"{self.CACHE_PREFIX}{content_id}"

        # 尝试Redis缓存
        try:
            cached = await self.redis.get(cache_key)
            if cached:
                cached_str = cached.decode('utf-8') if isinstance(cached, bytes) else cached
                data = json.loads(cached_str)
                return data.get('content', '')
        except Exception as e:
            logger.warning(f"Failed to get content from cache: {e}")

        # MySQL回源
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(
                    "SELECT content FROM contents WHERE id = %s",
                    (content_id,)
                )
                row = await cursor.fetchone()
                if row:
                    content = row[0]
                    # 写入缓存
                    try:
                        await self.redis.setex(
                            cache_key,
                            self.CACHE_TTL,
                            json.dumps({'content': content}, ensure_ascii=False)
                        )
                    except Exception as e:
                        logger.warning(f"Failed to cache content: {e}")
                    return content

        return None

    async def reload(self, max_size: int = 50000) -> int:
        """重新加载ID池"""
        self._loaded = False
        self._layers = [[], [], []]
        self._cursors = [0, 0, 0]
        self._total = 0
        return await self.load_ids(max_size)

    def get_stats(self) -> Dict[str, Any]:
        """获取统计信息"""
        return {
            'group_id': self.group_id,
            'loaded': self._loaded,
            'total': self._total,
            'layers': [len(l) for l in self._layers],
            'cursors': self._cursors,
            'remaining': [
                len(self._layers[i]) - self._cursors[i]
                for i in range(3)
            ]
        }


# ============================================
# 全局实例管理（多实例模式）
# ============================================

# 全局管理器字典（按 group_id 存储）
_content_managers: Dict[int, ContentManager] = {}
_redis_client = None
_db_pool = None


async def init_content_manager(
    redis_client,
    db_pool,
    group_id: int = 1,
    max_size: int = 50000
) -> ContentManager:
    """
    初始化正文管理器（支持多分组）

    Args:
        redis_client: Redis客户端
        db_pool: MySQL连接池
        group_id: 分组ID
        max_size: 最大加载数量

    Returns:
        ContentManager实例
    """
    global _redis_client, _db_pool
    _redis_client = redis_client
    _db_pool = db_pool

    manager = ContentManager(redis_client, db_pool, group_id)
    await manager.load_ids(max_size)
    _content_managers[group_id] = manager
    return manager


async def get_or_create_content_manager(group_id: int, max_size: int = 50000) -> Optional[ContentManager]:
    """
    获取或创建指定分组的正文管理器（懒加载）

    Args:
        group_id: 分组ID
        max_size: 最大加载数量

    Returns:
        ContentManager实例或None
    """
    if group_id in _content_managers:
        return _content_managers[group_id]

    if not _redis_client or not _db_pool:
        return None

    manager = ContentManager(_redis_client, _db_pool, group_id)
    await manager.load_ids(max_size)
    _content_managers[group_id] = manager
    return manager


def get_content_manager(group_id: int = 1) -> Optional[ContentManager]:
    """
    获取指定分组的正文管理器实例

    Args:
        group_id: 分组ID

    Returns:
        ContentManager实例或None
    """
    return _content_managers.get(group_id)


async def get_random_content(group_id: int = 1) -> str:
    """
    获取随机正文（便捷函数）

    Args:
        group_id: 分组ID

    Returns:
        正文内容
    """
    manager = _content_managers.get(group_id)
    if not manager:
        # 尝试懒加载
        manager = await get_or_create_content_manager(group_id)
    if manager:
        return await manager.get_random_content()
    return ""
