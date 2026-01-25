# -*- coding: utf-8 -*-
"""
段落池管理器 - 一次性消费模式

特性：
- 从 contents 表存储的段落中消费
- 一次性消费：用过的段落加入已用池
- 可用池耗尽时自动轮转：已用池 -> 可用池
- 告警机制：预警、严重、枯竭三级告警

Redis 数据结构：
- contents:pool:{group_id}  SET  可用的段落ID池
- contents:used:{group_id}  SET  已使用的段落ID池
- alerts:content_pool       HASH 告警状态
"""

from datetime import datetime
from typing import Optional, Dict, Any, List
from loguru import logger


class ContentPoolManager:
    """
    段落池管理器 - 负责段落的生产和消费

    消费流程：
    1. SPOP 从可用池取 ID
    2. 可用池空则轮转已用池
    3. 从 MySQL 查内容返回
    4. 更新告警状态
    """

    def __init__(self, redis_client, db_pool, group_id: int = 1):
        """
        初始化段落池管理器

        Args:
            redis_client: Redis 客户端
            db_pool: MySQL 连接池
            group_id: 分组 ID
        """
        self.redis = redis_client
        self.db_pool = db_pool
        self.group_id = group_id

        # Redis Key 定义
        self.pool_key = f"contents:pool:{group_id}"
        self.used_key = f"contents:used:{group_id}"
        self.alert_key = "alerts:content_pool"

    async def get_content(self) -> Optional[str]:
        """
        获取一条段落内容（一次性消费）

        Returns:
            带拼音标注的段落内容，或 None（枯竭）
        """
        # 1. 从可用池取 ID
        content_id = await self.redis.spop(self.pool_key)

        if not content_id:
            # 2. 可用池空了，尝试轮转
            rotated = await self._try_rotate()
            if rotated:
                content_id = await self.redis.spop(self.pool_key)

        if not content_id:
            # 3. 真正枯竭
            await self._update_alert("exhausted", "段落池已枯竭，无可用数据")
            return None

        # 解码 ID
        if isinstance(content_id, bytes):
            content_id = content_id.decode('utf-8')
        content_id = int(content_id)

        # 4. 加入已用池
        await self.redis.sadd(self.used_key, content_id)

        # 5. 更新告警状态
        await self._update_alert_by_pool_status()

        # 6. 从 MySQL 获取内容
        content = await self._fetch_content(content_id)
        return content

    async def _try_rotate(self) -> bool:
        """
        尝试轮转：把已用池移到可用池

        Returns:
            是否成功轮转
        """
        used_count = await self.redis.scard(self.used_key)
        if used_count == 0:
            return False

        try:
            # 原子操作：重命名
            await self.redis.rename(self.used_key, self.pool_key)
            await self._update_alert(
                "critical",
                f"段落池已轮转，{used_count} 条数据将被重复使用"
            )
            logger.warning(f"Content pool rotated: {used_count} items recycled")
            return True
        except Exception as e:
            logger.error(f"Failed to rotate content pool: {e}")
            return False

    async def add_to_pool(self, content_id: int):
        """
        添加新段落 ID 到可用池

        Args:
            content_id: 段落 ID
        """
        await self.redis.sadd(self.pool_key, content_id)
        await self._update_alert_by_pool_status()

    async def add_batch_to_pool(self, content_ids: list):
        """
        批量添加段落 ID 到可用池

        Args:
            content_ids: 段落 ID 列表
        """
        if not content_ids:
            return
        await self.redis.sadd(self.pool_key, *content_ids)
        await self._update_alert_by_pool_status()

    async def _update_alert_by_pool_status(self):
        """根据池状态更新告警"""
        pool_size = await self.redis.scard(self.pool_key)
        used_size = await self.redis.scard(self.used_key)
        total = pool_size + used_size

        if total == 0:
            level, message = "exhausted", "段落池已枯竭，无可用数据"
        elif pool_size == 0:
            level, message = "critical", "段落池已耗尽，正在重复使用数据"
        elif pool_size < total * 0.2:
            level, message = "warning", f"段落池即将耗尽，剩余 {pool_size} 条"
        else:
            level, message = "normal", "段落池状态正常"

        await self._update_alert(level, message, pool_size, used_size, total)

    async def _update_alert(
        self,
        level: str,
        message: str,
        pool_size: Optional[int] = None,
        used_size: Optional[int] = None,
        total: Optional[int] = None
    ):
        """
        更新告警状态

        Args:
            level: 告警级别 (normal, warning, critical, exhausted)
            message: 告警消息
            pool_size: 可用池大小
            used_size: 已用池大小
            total: 总数
        """
        if pool_size is None:
            pool_size = await self.redis.scard(self.pool_key)
        if used_size is None:
            used_size = await self.redis.scard(self.used_key)
        if total is None:
            total = pool_size + used_size

        try:
            await self.redis.hset(self.alert_key, mapping={
                "level": level,
                "message": message,
                "pool_size": str(pool_size),
                "used_size": str(used_size),
                "total": str(total),
                "group_id": str(self.group_id),
                "updated_at": datetime.now().isoformat()
            })
        except Exception as e:
            logger.warning(f"Failed to update alert: {e}")

    async def _fetch_content(self, content_id: int) -> Optional[str]:
        """
        从 MySQL 获取段落内容

        Args:
            content_id: 段落 ID

        Returns:
            段落内容或 None
        """
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "SELECT content FROM contents WHERE id = %s",
                        (content_id,)
                    )
                    row = await cursor.fetchone()
                    return row[0] if row else None
        except Exception as e:
            logger.error(f"Failed to fetch content {content_id}: {e}")
            return None

    async def get_alert_status(self) -> Dict[str, Any]:
        """
        获取告警状态

        Returns:
            告警状态字典
        """
        try:
            alert = await self.redis.hgetall(self.alert_key)
            if not alert:
                return {
                    "level": "unknown",
                    "message": "未初始化",
                    "pool_size": 0,
                    "used_size": 0,
                    "total": 0,
                    "group_id": self.group_id,
                    "updated_at": ""
                }

            # 解码 bytes
            decoded = {}
            for k, v in alert.items():
                key = k.decode('utf-8') if isinstance(k, bytes) else k
                val = v.decode('utf-8') if isinstance(v, bytes) else v
                decoded[key] = val

            return {
                "level": decoded.get("level", "unknown"),
                "message": decoded.get("message", ""),
                "pool_size": int(decoded.get("pool_size", 0)),
                "used_size": int(decoded.get("used_size", 0)),
                "total": int(decoded.get("total", 0)),
                "group_id": int(decoded.get("group_id", self.group_id)),
                "updated_at": decoded.get("updated_at", "")
            }
        except Exception as e:
            logger.error(f"Failed to get alert status: {e}")
            return {
                "level": "error",
                "message": f"获取状态失败: {e}",
                "pool_size": 0,
                "used_size": 0,
                "total": 0,
                "group_id": self.group_id,
                "updated_at": ""
            }

    async def get_pool_stats(self) -> Dict[str, int]:
        """
        获取池统计信息

        Returns:
            统计信息字典
        """
        pool_size = await self.redis.scard(self.pool_key)
        used_size = await self.redis.scard(self.used_key)
        return {
            "pool_size": pool_size,
            "used_size": used_size,
            "total": pool_size + used_size
        }

    async def initialize_pool_from_db(self, max_size: int = 0) -> int:
        """
        从数据库初始化可用池（首次启动或重置时使用）

        注意：这会清空已用池，将所有 ID 加入可用池

        Args:
            max_size: 最大加载数量（0=不限制）

        Returns:
            加载的 ID 数量
        """
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    limit_clause = f" LIMIT {max_size}" if max_size > 0 else ""
                    await cursor.execute(
                        f"SELECT id FROM contents WHERE group_id = %s{limit_clause}",
                        (self.group_id,)
                    )
                    rows = await cursor.fetchall()

            if not rows:
                logger.warning(f"No contents found for group {self.group_id}")
                await self._update_alert("exhausted", "段落池已枯竭，无可用数据")
                return 0

            content_ids = [row[0] for row in rows]

            # 清空已用池
            await self.redis.delete(self.used_key)
            # 清空可用池
            await self.redis.delete(self.pool_key)
            # 批量添加到可用池
            await self.redis.sadd(self.pool_key, *content_ids)

            # 更新告警状态
            await self._update_alert(
                "normal",
                "段落池已初始化",
                len(content_ids),
                0,
                len(content_ids)
            )

            logger.info(f"Content pool initialized with {len(content_ids)} items")
            return len(content_ids)

        except Exception as e:
            logger.error(f"Failed to initialize content pool: {e}")
            return 0

    async def reset_pool(self) -> int:
        """
        重置池：清空已用池，重新从数据库加载可用池

        Returns:
            加载的 ID 数量
        """
        return await self.initialize_pool_from_db()


# ============================================
# 全局实例管理（多实例模式）
# ============================================

# 全局管理器字典（按 group_id 存储）
_content_pool_managers: Dict[int, ContentPoolManager] = {}
_redis_client = None
_db_pool = None


async def init_content_pool_manager(
    redis_client,
    db_pool,
    group_id: int = 1,
    auto_initialize: bool = True,
    max_size: int = 0
) -> ContentPoolManager:
    """
    初始化段落池管理器（支持多分组）

    Args:
        redis_client: Redis 客户端
        db_pool: MySQL 连接池
        group_id: 分组 ID
        auto_initialize: 是否自动从数据库加载 ID（仅当池为空时）
        max_size: 最大加载数量

    Returns:
        ContentPoolManager 实例
    """
    global _redis_client, _db_pool
    _redis_client = redis_client
    _db_pool = db_pool

    manager = ContentPoolManager(redis_client, db_pool, group_id)

    if auto_initialize:
        # 检查池是否为空
        stats = await manager.get_pool_stats()
        if stats['total'] == 0:
            await manager.initialize_pool_from_db(max_size)
        else:
            # 更新告警状态
            await manager._update_alert_by_pool_status()
            logger.info(
                f"Content pool (group {group_id}) already has {stats['pool_size']} available, "
                f"{stats['used_size']} used"
            )

    _content_pool_managers[group_id] = manager
    return manager


async def get_or_create_content_pool_manager(
    group_id: int,
    auto_initialize: bool = True,
    max_size: int = 0
) -> Optional[ContentPoolManager]:
    """
    获取或创建指定分组的段落池管理器（懒加载）

    Args:
        group_id: 分组ID
        auto_initialize: 是否自动初始化
        max_size: 最大加载数量

    Returns:
        ContentPoolManager实例或None
    """
    if group_id in _content_pool_managers:
        return _content_pool_managers[group_id]

    if not _redis_client or not _db_pool:
        return None

    manager = ContentPoolManager(_redis_client, _db_pool, group_id)

    if auto_initialize:
        stats = await manager.get_pool_stats()
        if stats['total'] == 0:
            await manager.initialize_pool_from_db(max_size)
        else:
            await manager._update_alert_by_pool_status()

    _content_pool_managers[group_id] = manager
    return manager


def get_content_pool_manager(group_id: int = 1) -> Optional[ContentPoolManager]:
    """
    获取指定分组的段落池管理器实例

    Args:
        group_id: 分组ID

    Returns:
        ContentPoolManager实例或None
    """
    return _content_pool_managers.get(group_id)


async def get_pool_content(group_id: int = 1) -> Optional[str]:
    """
    获取段落内容（便捷函数）

    Args:
        group_id: 分组ID

    Returns:
        段落内容或 None
    """
    manager = _content_pool_managers.get(group_id)
    if not manager:
        # 尝试懒加载
        manager = await get_or_create_content_pool_manager(group_id)
    if manager:
        return await manager.get_content()
    return None
