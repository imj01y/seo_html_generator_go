"""
图片管理器（直接查 MySQL 架构）

千万级图片高性能方案：
- 内存存全量 ID 列表（~80MB for 1000万），保证随机性
- 直接从 MySQL 批量查询 URL
- 取 1 万张约 30-50ms

架构:
┌─────────────────────────────────────────────────────────────┐
│  内存: 全量 ID 列表 (打乱顺序)                               │
│  [id_a, id_x, id_m, ..., id_z]  ← 启动时 shuffle            │
│  容量: 1000万 × 8字节 = 80MB                                 │
│                                                              │
│  cursor ────────────────┐                                    │
│                         ↓                                    │
│  [...已用...] [...待消费...]                                 │
└─────────────────────────────────────────────────────────────┘
                         │
                         ↓ 取 ID 后直接查 MySQL
┌─────────────────────────────────────────────────────────────┐
│  MySQL: images 表                                            │
└─────────────────────────────────────────────────────────────┘

性能指标:
- 获取1万个URL: ~30-50ms (MySQL 批量查询)
- 内存占用: ~80MB (仅 ID 列表)
- 不重复保证: 遍历完所有ID后才重复
"""

import asyncio
import random
from loguru import logger
from typing import List, Dict, Any, Optional, Set


class SlidingWindowImageManager:
    """
    图片管理器（直接查 MySQL）

    特性：
    - 内存存全量 ID 列表（80MB），保证随机性
    - 直接从 MySQL 批量查询 URL
    - 取 1 万张约 30-50ms
    - 去重：MySQL INSERT IGNORE + 内存 _id_set（与关键词管理一致）

    使用方法:
        manager = SlidingWindowImageManager(db_pool, group_id=1)
        await manager.initialize()
        urls = await manager.get_random(10000)
    """

    def __init__(
        self,
        db_pool: Any,
        group_id: int = 1
    ):
        self.db_pool = db_pool
        self.group_id = group_id

        # 内存中的全量 ID 列表（打乱顺序）
        self._ids: List[int] = []
        self._id_set: Set[int] = set()  # 用于 O(1) 去重
        self._total: int = 0

        # 游标
        self._cursor: int = 0        # 下次取数据的位置

        # 初始化状态
        self._loaded = False

        # SEOCore 同步缓存大小限制
        self._sync_cache_max_size: int = 10000

    async def initialize(self):
        """初始化：只加载 ID 列表"""
        if self._loaded:
            return

        # 1. 从 MySQL 加载全量 ID
        await self._load_all_ids()

        # 2. 打乱顺序（保证随机性）
        if self._total > 0:
            random.shuffle(self._ids)

        # 3. 重置游标
        self._cursor = 0

        # 4. 从数据库读取 image_pool_size 配置
        await self._load_sync_cache_config()

        self._loaded = True
        logger.info(f"ImageManager initialized: {self._total} images, "
                   f"memory: {self._total * 8 / 1024 / 1024:.1f}MB")

    async def _load_all_ids(self):
        """从 MySQL 加载全量 ID 到内存"""
        if not self.db_pool:
            logger.warning("No database pool, cannot load image IDs")
            return

        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "SELECT id FROM images WHERE group_id = %s AND status = 1",
                        (self.group_id,)
                    )
                    rows = await cursor.fetchall()
                    self._ids = [row[0] for row in rows]
                    self._id_set = set(self._ids)  # 同步初始化集合
                    self._total = len(self._ids)

            logger.info(f"Loaded {self._total} image IDs ({self._total * 8 / 1024 / 1024:.1f}MB)")

        except Exception as e:
            logger.error(f"Failed to load image IDs: {e}")
            raise

    async def _load_sync_cache_config(self) -> None:
        """从数据库加载同步缓存配置"""
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "SELECT setting_value FROM system_settings WHERE setting_key = 'image_pool_size'"
                    )
                    row = await cursor.fetchone()
                    if row:
                        self._sync_cache_max_size = int(row[0]) if int(row[0]) > 0 else 10000
                        logger.info(f"Sync cache max size: {self._sync_cache_max_size}")
        except Exception as e:
            logger.warning(f"Failed to load sync cache config: {e}")

    async def get_random(self, count: int = 10) -> List[str]:
        """
        获取随机图片 URL

        Args:
            count: 需要的数量

        Returns:
            URL 列表
        """
        if not self._loaded:
            await self.initialize()

        if count <= 0 or self._total == 0:
            return []

        # 限制单次请求量
        count = min(count, self._total)

        # 处理循环（用完后从头开始）
        if self._cursor >= self._total:
            random.shuffle(self._ids)
            self._cursor = 0
            logger.debug("Cursor reset, reshuffled IDs")

        # 1. 从内存取 ID
        end_idx = min(self._cursor + count, self._total)
        ids = self._ids[self._cursor:end_idx]
        self._cursor = end_idx

        # 2. 直接从 MySQL 批量查询 URL
        return await self._fetch_urls_from_db_simple(ids)

    async def _fetch_urls_from_db_simple(self, ids: List[int]) -> List[str]:
        """从 MySQL 批量查询 URL"""
        if not ids or not self.db_pool:
            return []

        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    placeholders = ','.join(['%s'] * len(ids))
                    await cursor.execute(
                        f"SELECT url FROM images WHERE id IN ({placeholders}) AND status = 1",
                        ids
                    )
                    rows = await cursor.fetchall()
                    return [row[0] for row in rows]
        except Exception as e:
            logger.error(f"Failed to fetch URLs from DB: {e}")
            return []

    # ============================================
    # 动态写入支持
    # ============================================

    async def add_url(self, url: str, group_id: int = None) -> Optional[int]:
        """
        添加单个图片 URL（自动去重）

        Args:
            url: 图片URL
            group_id: 分组ID（默认使用实例的group_id）

        Returns:
            新图片ID，失败或重复返回None
        """
        target_group = group_id if group_id is not None else self.group_id

        # 1. 写入 MySQL（依赖 INSERT IGNORE 去重）
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "INSERT IGNORE INTO images (group_id, url, status) VALUES (%s, %s, 1)",
                        (target_group, url)
                    )
                    await conn.commit()

                    if cursor.rowcount == 0:
                        # 重复或失败
                        return None

                    image_id = cursor.lastrowid

        except Exception as e:
            logger.error(f"Failed to add image URL: {e}")
            return None

        # 2. 追加到内存 ID 列表（去重）
        if image_id not in self._id_set:
            self._ids.append(image_id)
            self._id_set.add(image_id)
            self._total += 1

        # 3. 直接追加到 CachePool（立即可消费）
        self._append_to_cache_pool(url)

        return image_id

    def _append_to_cache_pool(self, url: str) -> bool:
        """直接追加 URL 到 CachePool（立即可消费）"""
        try:
            from .image_cache_pool import get_image_cache_pool
            pool = get_image_cache_pool()
            if pool:
                pool.append_direct(url)
                return True
        except Exception as e:
            logger.warning(f"Failed to append URL to cache pool: {e}")
        return False

    async def add_urls_batch(self, urls: List[str], group_id: int = None) -> Dict[str, int]:
        """
        批量添加图片 URL（自动去重）

        Args:
            urls: 图片URL列表
            group_id: 分组ID

        Returns:
            {'added': 成功数, 'skipped': 跳过数, 'total': 总数}
        """
        target_group = group_id if group_id is not None else self.group_id
        added = 0
        skipped = 0
        added_urls = []

        # 分批处理
        batch_size = 1000
        for i in range(0, len(urls), batch_size):
            batch = urls[i:i + batch_size]

            for url in batch:
                result = await self.add_url(url, target_group)
                if result:
                    added += 1
                    added_urls.append(url)
                else:
                    skipped += 1

            # 记录进度
            if len(urls) > batch_size:
                processed = min(i + batch_size, len(urls))
                logger.debug(f"Batch progress: {processed}/{len(urls)} URLs")

        # 批量追加到 CachePool（已在 add_url 中单个处理，这里不再重复）

        return {
            'added': added,
            'skipped': skipped,
            'total': len(urls)
        }

    # ============================================
    # 统计和兼容性方法
    # ============================================

    def get_stats(self) -> Dict[str, Any]:
        """获取统计信息"""
        return {
            'total': self._total,
            'cursor': self._cursor,
            'remaining': self._total - self._cursor,
            'loaded': self._loaded,
            'group_id': self.group_id,
            'memory_mb': round(self._total * 8 / 1024 / 1024, 2),
        }

    def total_count(self) -> int:
        """获取总图片数量"""
        return self._total

    async def reload(self, group_id: Optional[int] = None, max_size: int = 0) -> int:
        """重新加载（兼容旧接口）"""
        self._loaded = False
        self._ids = []
        self._id_set = set()  # 清空集合
        self._cursor = 0
        self._total = 0

        if group_id is not None:
            self.group_id = group_id

        await self.initialize()
        return self._total

    # 兼容旧接口
    async def load_ids(self, group_id: Optional[int] = None, max_size: int = 0) -> int:
        """兼容旧接口：加载ID"""
        if group_id is not None:
            self.group_id = group_id

        await self.initialize()
        return self._total


# ============================================
# 类型别名（兼容旧代码）
# ============================================
AsyncImageGroupManager = SlidingWindowImageManager


# ============================================
# 全局实例管理
# ============================================

_image_group: Optional[SlidingWindowImageManager] = None


def get_image_group() -> Optional[SlidingWindowImageManager]:
    """获取全局图片分组实例"""
    return _image_group


async def init_image_group(
    db_pool: Any,
    group_id: Optional[int] = None
) -> SlidingWindowImageManager:
    """
    初始化全局图片分组

    Args:
        db_pool: 异步数据库连接池
        group_id: 指定分组ID

    Returns:
        SlidingWindowImageManager实例
    """
    global _image_group

    target_group = group_id if group_id is not None else 1

    _image_group = SlidingWindowImageManager(
        db_pool=db_pool,
        group_id=target_group
    )

    # 初始化
    await _image_group.initialize()

    logger.info(f"Image group initialized with {_image_group._total} images (group_id={target_group})")

    return _image_group


# ============================================
# 快捷函数
# ============================================

async def get_random_images(count: int = 10) -> List[str]:
    """获取随机图片URL"""
    if _image_group:
        return await _image_group.get_random(count)
    return []


def random_image() -> str:
    """
    同步获取随机图片ID（兼容模板使用）

    Note: 返回的是ID字符串，用于seo_core的sync cache预加载
    """
    if _image_group and _image_group._loaded and _image_group._total > 0:
        # 处理循环
        if _image_group._cursor >= _image_group._total:
            random.shuffle(_image_group._ids)
            _image_group._cursor = 0

        img_id = _image_group._ids[_image_group._cursor]
        _image_group._cursor += 1
        return str(img_id)
    return ""
