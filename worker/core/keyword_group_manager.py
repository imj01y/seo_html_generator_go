"""
关键词分组管理器（直接查 MySQL 架构）

提供千万级关键词的极速随机获取功能:
- 内存中保存已打乱的ID列表（不重复）
- 直接从 MySQL 批量查询关键词
- MySQL作为持久化存储

概念说明:
- 分组(Group): 数据库中的keyword_groups表，用于数据逻辑分类
- ID列表: 内存中已加载并打乱的关键词ID列表

性能指标:
- 获取10个关键词: < 30ms (MySQL 批量查询)
- 内存占用: ~80MB (1000万ID)
- 不重复保证: 遍历完所有ID后才重复

架构:
- 内存: 已打乱的ID列表
- MySQL: keywords表 + keyword_groups表
"""

import random
import logging
import warnings
from typing import List, Dict, Any, Optional, Set

logger = logging.getLogger(__name__)

# 批量插入时每批的大小
BATCH_CHUNK_SIZE = 5000


class AsyncKeywordGroupManager:
    """
    异步关键词分组管理器

    用于FastAPI等异步框架，采用Shuffle模式实现不重复随机获取:
    1. 启动时从MySQL加载所有关键词ID到内存并打乱
    2. 顺序取出ID，用完后重新打乱
    3. 直接从 MySQL 批量查询关键词

    Attributes:
        db_pool: 异步数据库连接池
        _ids: 已打乱的关键词ID列表
        _cursor: 当前读取位置
        _total: 关键词总数

    Example:
        >>> manager = AsyncKeywordGroupManager(db_pool=aiomysql_pool)
        >>> await manager.load_ids()
        >>> keywords = await manager.get_random(10)
    """

    def __init__(
        self,
        db_pool: Any = None,
        group_id: Optional[int] = None
    ):
        self.db_pool = db_pool
        self._group_id = group_id
        self._ids: List[int] = []
        self._id_set: Set[int] = set()  # 用于 O(1) 去重
        self._cursor: int = 0
        self._total: int = 0
        self._loaded: bool = False
        self._sync_cache_max_size: int = 10000  # SEOCore 同步缓存大小限制

    async def load_ids(self, group_id: Optional[int] = None, max_size: int = 0) -> int:
        """
        异步加载关键词ID并打乱

        Args:
            group_id: 指定分组ID，None表示加载所有分组
            max_size: 最大加载数量，0表示不限制

        Returns:
            加载的关键词总数
        """
        if not self.db_pool:
            logger.warning("No database pool, cannot load keyword IDs")
            return 0

        target_group = group_id if group_id is not None else self._group_id

        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    # 构建SQL，支持限制数量
                    limit_clause = f" LIMIT {max_size}" if max_size > 0 else ""
                    if target_group is not None:
                        await cursor.execute(
                            f"SELECT id FROM keywords WHERE group_id = %s AND status = 1 ORDER BY RAND(){limit_clause}",
                            (target_group,)
                        )
                    else:
                        await cursor.execute(f"SELECT id FROM keywords WHERE status = 1 ORDER BY RAND(){limit_clause}")

                    rows = await cursor.fetchall()
                    self._ids = [row[0] for row in rows]
                    self._id_set = set(self._ids)  # 同步初始化集合

            self._total = len(self._ids)

            if self._total > 0:
                random.shuffle(self._ids)

            self._cursor = 0
            self._loaded = True

            logger.info(f"Loaded {self._total} keyword IDs into memory (group_id={target_group}, max_size={max_size})")

            # 从数据库读取 keyword_pool_size 配置
            await self._load_sync_cache_config()

            return self._total

        except Exception as e:
            logger.error(f"Failed to load keyword IDs: {e}")
            raise

    async def _load_sync_cache_config(self) -> None:
        """从数据库加载同步缓存配置"""
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "SELECT setting_value FROM system_settings WHERE setting_key = 'keyword_pool_size'"
                    )
                    row = await cursor.fetchone()
                    if row:
                        self._sync_cache_max_size = int(row[0]) if int(row[0]) > 0 else 10000
                        logger.info(f"Keyword sync cache max size: {self._sync_cache_max_size}")
        except Exception as e:
            logger.warning(f"Failed to load keyword sync cache config: {e}")

    def _reshuffle_if_needed(self, count: int) -> None:
        """检查是否需要重新打乱"""
        if self._cursor + count > self._total:
            random.shuffle(self._ids)
            self._cursor = 0

    async def get_random(self, count: int = 10) -> List[str]:
        """
        异步随机获取N个关键词

        流程:
        1. 从内存取ID（~0.001ms）
        2. 直接从 MySQL 批量查询关键词（~30ms）

        Args:
            count: 需要获取的关键词数量

        Returns:
            关键词列表
        """
        if not self._loaded:
            await self.load_ids()

        if self._total == 0:
            return []

        count = min(count, self._total)
        self._reshuffle_if_needed(count)

        # 从内存取 ID
        ids = self._ids[self._cursor:self._cursor + count]
        self._cursor += count

        # 直接从 MySQL 批量查询
        return await self._fetch_keywords_from_db(ids)

    async def _fetch_keywords_from_db(self, ids: List[int]) -> List[str]:
        """从 MySQL 批量查询关键词"""
        if not ids or not self.db_pool:
            return []

        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    placeholders = ','.join(['%s'] * len(ids))
                    await cursor.execute(
                        f"SELECT keyword FROM keywords WHERE id IN ({placeholders}) AND status = 1",
                        ids
                    )
                    rows = await cursor.fetchall()
                    return [row[0] for row in rows]
        except Exception as e:
            logger.error(f"Failed to fetch keywords from DB: {e}")
            return []

    async def get_one(self) -> str:
        """获取单个随机关键词"""
        result = await self.get_random(1)
        return result[0] if result else ""

    async def get_target_keywords(self, count: int = 3) -> List[str]:
        """获取目标关键词（用于Title、H1等）"""
        return await self.get_random(count)

    async def get_hotspot_keywords(self, count: int = 10) -> List[str]:
        """获取热点关键词（用于内链锚文本）"""
        return await self.get_random(count)

    async def add_keyword(self, keyword: str, group_id: int = 1) -> Optional[int]:
        """
        异步添加单个关键词

        Args:
            keyword: 关键词
            group_id: 分组ID

        Returns:
            新关键词ID，失败或重复返回None
        """
        if not self.db_pool:
            return None

        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "INSERT IGNORE INTO keywords (group_id, keyword) VALUES (%s, %s)",
                        (group_id, keyword)
                    )
                    await conn.commit()

                    if cursor.rowcount == 0:
                        return None

                    keyword_id = cursor.lastrowid

            # 追加到内存 ID 列表（去重检查）
            if keyword_id not in self._id_set:
                self._ids.append(keyword_id)
                self._id_set.add(keyword_id)
                self._total += 1

            # 直接追加到 CachePool（立即可消费）
            self._append_to_cache_pool(keyword)

            return keyword_id

        except Exception as e:
            logger.error(f"Failed to add keyword: {e}")
            return None

    def _append_to_cache_pool(self, keyword: str) -> bool:
        """直接追加关键词到 CachePool（立即可消费）"""
        try:
            from .keyword_cache_pool import get_keyword_cache_pool
            pool = get_keyword_cache_pool()
            if pool:
                pool.append_direct(keyword)
                return True
        except Exception as e:
            logger.warning(f"Failed to append keyword to cache pool: {e}")
        return False

    async def add_keywords_batch(self, keywords: List[str], group_id: int = 1) -> Dict[str, int]:
        """
        异步批量添加关键词

        Args:
            keywords: 关键词列表
            group_id: 分组ID

        Returns:
            {'added': 成功数, 'skipped': 跳过数}
        """
        if not self.db_pool or not keywords:
            return {'added': 0, 'skipped': len(keywords)}

        total_added = 0
        added_keywords = []  # 收集成功添加的关键词

        try:
            # 分批处理，每批BATCH_CHUNK_SIZE条，避免MySQL optimizer内存溢出
            for i in range(0, len(keywords), BATCH_CHUNK_SIZE):
                chunk = keywords[i:i + BATCH_CHUNK_SIZE]

                async with self.db_pool.acquire() as conn:
                    async with conn.cursor() as cursor:
                        # 抑制INSERT IGNORE产生的重复键警告
                        with warnings.catch_warnings():
                            warnings.filterwarnings('ignore', message='.*Duplicate entry.*')
                            await cursor.executemany(
                                "INSERT IGNORE INTO keywords (group_id, keyword) VALUES (%s, %s)",
                                [(group_id, kw) for kw in chunk]
                            )
                        await conn.commit()
                        chunk_added = cursor.rowcount
                        total_added += chunk_added

                    # 获取新插入的ID（仅当有新增时）
                    if chunk_added > 0:
                        async with conn.cursor() as cursor:
                            # 使用分批查询避免IN子句过大
                            await cursor.execute("""
                                SELECT id, keyword FROM keywords
                                WHERE group_id = %s AND id > (
                                    SELECT COALESCE(MAX(id), 0) - %s FROM keywords WHERE group_id = %s
                                )
                                ORDER BY id DESC LIMIT %s
                            """, (group_id, chunk_added + 100, group_id, chunk_added))

                            rows = await cursor.fetchall()

                        # 追加到内存 ID 列表（去重检查）
                        if rows:
                            for row in rows:
                                if row[0] not in self._id_set:
                                    self._ids.append(row[0])
                                    self._id_set.add(row[0])
                                    self._total += 1
                                    added_keywords.append(row[1])

                # 记录分批进度
                if len(keywords) > BATCH_CHUNK_SIZE:
                    processed = min(i + BATCH_CHUNK_SIZE, len(keywords))
                    logger.debug(f"Batch progress: {processed}/{len(keywords)} keywords processed")

            # 批量追加到 CachePool（立即可消费）
            if added_keywords:
                self._append_batch_to_cache_pool(added_keywords)

            skipped = len(keywords) - total_added
            return {'added': total_added, 'skipped': skipped}

        except Exception as e:
            logger.error(f"Failed to batch add keywords: {e}")
            return {'added': 0, 'skipped': len(keywords)}

    def _append_batch_to_cache_pool(self, keywords: List[str]) -> int:
        """批量直接追加关键词到 CachePool"""
        if not keywords:
            return 0
        try:
            from .keyword_cache_pool import get_keyword_cache_pool
            pool = get_keyword_cache_pool()
            if pool:
                for kw in keywords:
                    pool.append_direct(kw)
                return len(keywords)
        except Exception as e:
            logger.warning(f"Failed to append keywords batch to cache pool: {e}")
        return 0

    def get_stats(self) -> Dict[str, Any]:
        """获取统计信息"""
        return {
            'total': self._total,
            'cursor': self._cursor,
            'remaining': self._total - self._cursor if self._total > 0 else 0,
            'loaded': self._loaded,
            'group_id': self._group_id,
            'memory_mb': round((self._total * 8) / (1024 * 1024), 2),
        }

    def total_count(self) -> int:
        """获取总关键词数量"""
        return self._total

    async def reload(self, group_id: Optional[int] = None, max_size: int = 0) -> int:
        """重新加载"""
        self._loaded = False
        self._ids = []
        self._id_set = set()  # 清空集合
        self._cursor = 0
        self._total = 0
        return await self.load_ids(group_id, max_size)


# ============================================
# 全局实例管理
# ============================================

_keyword_group: Optional[AsyncKeywordGroupManager] = None


def get_keyword_group() -> Optional[AsyncKeywordGroupManager]:
    """获取全局关键词分组实例"""
    return _keyword_group


async def init_keyword_group(
    db_pool: Any,
    group_id: Optional[int] = None
) -> AsyncKeywordGroupManager:
    """
    初始化全局关键词分组

    Args:
        db_pool: 异步数据库连接池
        group_id: 指定分组ID

    Returns:
        AsyncKeywordGroupManager实例
    """
    global _keyword_group

    _keyword_group = AsyncKeywordGroupManager(
        db_pool=db_pool,
        group_id=group_id
    )

    await _keyword_group.load_ids()
    logger.info(f"Keyword group initialized with {_keyword_group._total} keywords")

    return _keyword_group


# 快捷函数
async def get_random_keywords(count: int = 10) -> List[str]:
    """获取随机关键词"""
    if _keyword_group:
        return await _keyword_group.get_random(count)
    return []


def random_keyword() -> str:
    """
    同步获取随机关键词ID（兼容模板使用）

    Note: 返回的是ID字符串，用于seo_core的sync cache预加载
    """
    if _keyword_group and _keyword_group._loaded and _keyword_group._total > 0:
        _keyword_group._reshuffle_if_needed(1)
        kw_id = _keyword_group._ids[_keyword_group._cursor]
        _keyword_group._cursor += 1
        return str(kw_id)
    return ""
