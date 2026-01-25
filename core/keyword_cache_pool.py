"""
关键词缓存池管理器（生产者消费者模型）

实现高性能的关键词缓存池，保证关键词永远可用：
- 生产者：后台任务自动从 AsyncKeywordGroupManager 获取关键词填充缓存
- 消费者：模板渲染同步获取关键词，O(1) 无阻塞

架构：
- 内存缓存池：_cache: List[str]，预填充的关键词列表
- cursor 游标：当前消费位置
- 低水位触发：remaining < low_watermark 时触发后台填充
- 新关键词队列：Redis List，新增关键词自动合并到缓存池

性能指标：
- 获取关键词：O(1)，纯内存操作
- 填充操作：异步后台执行，不阻塞消费
- 线程安全：消费端 threading.Lock，生产端 asyncio.Lock
"""

import asyncio
import threading
import random
from typing import List, Dict, Any, Optional, Set, TYPE_CHECKING

from loguru import logger

if TYPE_CHECKING:
    from .keyword_group_manager import AsyncKeywordGroupManager


class KeywordCachePool:
    """
    关键词缓存池（生产者消费者模型）

    提供高性能、无阻塞的关键词获取接口。

    Attributes:
        _cache: 关键词缓存列表
        _cursor: 当前消费位置
        _cache_size: 缓存池目标大小
        _low_watermark: 低水位阈值（触发填充）
        _keyword_manager: 异步关键词管理器引用
        _redis: Redis 客户端引用
        _encoder: 编码器引用（用于预编码关键词）

    Example:
        >>> pool = KeywordCachePool(keyword_manager, redis_client, encoder=encoder)
        >>> await pool.initialize()
        >>> await pool.start()
        >>> keyword = pool.get_keyword_sync()  # 同步获取（已预编码）
        >>> await pool.stop()
    """

    def __init__(
        self,
        keyword_manager: "AsyncKeywordGroupManager",
        redis_client: Any,
        cache_size: int = 10000,
        low_watermark_ratio: float = 0.2,
        refill_batch_size: int = 2000,
        check_interval: float = 1.0,
        encoder: Any = None
    ):
        """
        初始化缓存池

        Args:
            keyword_manager: 异步关键词管理器
            redis_client: Redis 客户端
            cache_size: 缓存池大小
            low_watermark_ratio: 低水位比例（0.2 = 20%）
            refill_batch_size: 每次填充数量
            check_interval: 检查间隔（秒）
            encoder: 编码器（用于预编码关键词，提升渲染性能）
        """
        self._keyword_manager = keyword_manager
        self._redis = redis_client
        self._encoder = encoder  # 编码器引用

        # 缓存池配置
        self._cache_size = cache_size
        self._low_watermark_ratio = low_watermark_ratio
        self._low_watermark = int(cache_size * low_watermark_ratio)
        self._refill_batch_size = refill_batch_size
        self._check_interval = check_interval

        # 缓存池状态
        self._cache: List[str] = []
        self._cursor: int = 0
        self._keyword_set: Set[str] = set()  # 用于去重

        # 线程安全锁
        self._consume_lock = threading.Lock()  # 消费端锁（同步）
        self._refill_lock: Optional[asyncio.Lock] = None  # 生产端锁（异步）

        # 后台任务
        self._refill_task: Optional[asyncio.Task] = None
        self._running = False

        # 统计信息
        self._total_consumed = 0
        self._total_refilled = 0

    def _encode_keywords(self, keywords: List[str]) -> List[str]:
        """
        批量编码关键词

        Args:
            keywords: 原始关键词列表

        Returns:
            编码后的关键词列表
        """
        if self._encoder and keywords:
            return [self._encoder.encode_text(kw) for kw in keywords]
        return keywords

    async def initialize(self) -> int:
        """
        初始化缓存池（首次填充），处理数据为空的情况

        Returns:
            初始化后的关键词数量
        """
        self._refill_lock = asyncio.Lock()

        # 首次填充缓存池
        try:
            keywords = await self._keyword_manager.get_random(self._cache_size)

            if not keywords:
                logger.warning("KeywordCachePool: No data available, cache pool is empty")
                with self._consume_lock:
                    self._cache = []
                    self._keyword_set = set()
                    self._cursor = 0
                return 0

            # 预编码关键词（提升渲染性能）
            encoded_keywords = self._encode_keywords(keywords)

            with self._consume_lock:
                self._cache = encoded_keywords.copy()
                self._keyword_set = set(encoded_keywords)  # 同步去重集合
                random.shuffle(self._cache)
                self._cursor = 0

            logger.info(f"KeywordCachePool initialized with {len(self._cache)} pre-encoded keywords")
            return len(self._cache)

        except Exception as e:
            logger.error(f"Failed to initialize KeywordCachePool: {e}")
            with self._consume_lock:
                self._cache = []
                self._keyword_set = set()
                self._cursor = 0
            return 0

    async def start(self) -> None:
        """启动后台任务"""
        if self._running:
            return

        self._running = True

        # 启动低水位监控任务
        self._refill_task = asyncio.create_task(self._refill_monitor_loop())

        logger.info("KeywordCachePool background tasks started")

    async def stop(self) -> None:
        """停止后台任务"""
        self._running = False

        # 取消任务
        if self._refill_task:
            self._refill_task.cancel()
            try:
                await self._refill_task
            except asyncio.CancelledError:
                pass
            self._refill_task = None

        logger.info("KeywordCachePool background tasks stopped")

    def get_keyword_sync(self) -> str:
        """
        同步获取单个关键词（用于模板渲染）

        O(1) 无阻塞操作，线程安全。

        Returns:
            关键词字符串，缓存为空时返回空字符串
        """
        with self._consume_lock:
            if not self._cache:
                return ""

            # 检查是否需要重置游标
            if self._cursor >= len(self._cache):
                random.shuffle(self._cache)
                self._cursor = 0

            keyword = self._cache[self._cursor]
            self._cursor += 1
            self._total_consumed += 1

            return keyword

    def append_direct(self, keyword: str) -> None:
        """
        直接追加新关键词到缓存池（新增数据立即可用）

        用于新增关键词时立即加入缓存池，无需等待下次 refill。
        关键词会被预编码后存入缓存。

        Args:
            keyword: 新关键词（原始文本）
        """
        # 预编码关键词
        encoded_keyword = keyword
        if self._encoder:
            encoded_keyword = self._encoder.encode_text(keyword)

        with self._consume_lock:
            if encoded_keyword not in self._keyword_set:
                self._cache.append(encoded_keyword)
                self._keyword_set.add(encoded_keyword)

    def get_keywords_sync(self, count: int) -> List[str]:
        """
        同步获取多个关键词（用于模板渲染）

        Args:
            count: 需要的关键词数量

        Returns:
            关键词列表
        """
        return [kw for kw in (self.get_keyword_sync() for _ in range(count)) if kw]

    def get_remaining(self) -> int:
        """获取剩余可用关键词数量"""
        with self._consume_lock:
            return max(0, len(self._cache) - self._cursor)

    def get_stats(self) -> Dict[str, Any]:
        """
        获取统计信息

        Returns:
            统计信息字典
        """
        with self._consume_lock:
            cache_size = len(self._cache)
            cursor = self._cursor

        return {
            "cache_size": cache_size,
            "cursor": cursor,
            "remaining": max(0, cache_size - cursor),
            "low_watermark": self._low_watermark,
            "target_size": self._cache_size,
            "total_consumed": self._total_consumed,
            "total_refilled": self._total_refilled,
            "running": self._running,
        }

    async def _refill_monitor_loop(self) -> None:
        """
        后台任务：监控低水位并自动填充

        每隔 check_interval 秒检查一次，当剩余关键词 < low_watermark 时触发填充。
        """
        while self._running:
            try:
                await asyncio.sleep(self._check_interval)

                remaining = self.get_remaining()

                if remaining < self._low_watermark:
                    logger.debug(f"Low watermark reached: {remaining}/{self._low_watermark}, triggering refill")
                    await self._refill_cache()

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Refill monitor error: {e}")
                await asyncio.sleep(5)  # 出错后等待更长时间

    async def _refill_cache(self) -> int:
        """
        填充缓存池

        从 AsyncKeywordGroupManager 获取新的关键词填充到缓存池。

        Returns:
            填充的关键词数量
        """
        if not self._refill_lock:
            return 0

        async with self._refill_lock:
            try:
                # 计算需要填充的数量
                with self._consume_lock:
                    current_remaining = len(self._cache) - self._cursor

                # 填充到目标大小
                needed = self._cache_size - current_remaining
                if needed <= 0:
                    return 0

                # 分批获取，避免一次性获取过多
                batch_size = min(needed, self._refill_batch_size)
                new_keywords = await self._keyword_manager.get_random(batch_size)

                if not new_keywords:
                    return 0

                # 预编码新关键词
                encoded_keywords = self._encode_keywords(new_keywords)

                # 过滤掉已存在的关键词（使用编码后的关键词比较）
                with self._consume_lock:
                    new_unique_keywords = [kw for kw in encoded_keywords if kw not in self._keyword_set]

                if not new_unique_keywords:
                    logger.debug("No new unique keywords to add")
                    return 0

                # 合并到缓存池
                with self._consume_lock:
                    # 保留未消费的关键词
                    remaining_keywords = self._cache[self._cursor:]

                    # 合并新关键词
                    self._cache = remaining_keywords + new_unique_keywords
                    self._keyword_set.update(new_unique_keywords)  # 更新去重集合
                    random.shuffle(self._cache)
                    self._cursor = 0

                self._total_refilled += len(new_unique_keywords)
                logger.info(f"Refilled {len(new_unique_keywords)} pre-encoded keywords, cache size: {len(self._cache)}")

                return len(new_keywords)

            except Exception as e:
                logger.error(f"Failed to refill cache: {e}")
                return 0

# ============================================
# 全局实例管理
# ============================================

_keyword_cache_pool: Optional[KeywordCachePool] = None


def get_keyword_cache_pool() -> Optional[KeywordCachePool]:
    """获取全局关键词缓存池实例"""
    return _keyword_cache_pool


async def init_keyword_cache_pool(
    keyword_manager: "AsyncKeywordGroupManager",
    redis_client: Any,
    cache_size: int = 10000,
    low_watermark_ratio: float = 0.2,
    refill_batch_size: int = 2000,
    check_interval: float = 1.0,
    encoder: Any = None
) -> KeywordCachePool:
    """
    初始化全局关键词缓存池

    Args:
        keyword_manager: 异步关键词管理器
        redis_client: Redis 客户端
        cache_size: 缓存池大小
        low_watermark_ratio: 低水位比例
        refill_batch_size: 每次填充数量
        check_interval: 检查间隔（秒）
        encoder: 编码器（用于预编码关键词，提升渲染性能）

    Returns:
        KeywordCachePool 实例
    """
    global _keyword_cache_pool

    _keyword_cache_pool = KeywordCachePool(
        keyword_manager=keyword_manager,
        redis_client=redis_client,
        cache_size=cache_size,
        low_watermark_ratio=low_watermark_ratio,
        refill_batch_size=refill_batch_size,
        check_interval=check_interval,
        encoder=encoder
    )

    # 初始化并启动
    await _keyword_cache_pool.initialize()
    await _keyword_cache_pool.start()

    return _keyword_cache_pool


async def stop_keyword_cache_pool() -> None:
    """停止全局关键词缓存池"""
    global _keyword_cache_pool

    if _keyword_cache_pool:
        await _keyword_cache_pool.stop()
        _keyword_cache_pool = None
        logger.info("Keyword cache pool stopped")
