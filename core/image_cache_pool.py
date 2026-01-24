"""
图片缓存池管理器（生产者消费者模型）

实现高性能的图片URL缓存池，保证图片URL永远可用：
- 生产者：后台任务自动从 SlidingWindowImageManager 获取URL填充缓存
- 消费者：模板渲染同步获取URL，O(1) 无阻塞

架构：
- 内存缓存池：_cache: List[str]，预填充的URL列表
- cursor 游标：当前消费位置
- 低水位触发：remaining < low_watermark 时触发后台填充
- 新URL队列：Redis List，新增URL自动合并到缓存池

性能指标：
- 获取URL：O(1)，纯内存操作
- 填充操作：异步后台执行，不阻塞消费
- 线程安全：消费端 threading.Lock，生产端 asyncio.Lock
"""

import asyncio
import threading
import random
from typing import List, Dict, Any, Optional, Set, TYPE_CHECKING

from loguru import logger

if TYPE_CHECKING:
    from .image_group_manager import SlidingWindowImageManager


class ImageCachePool:
    """
    图片URL缓存池（生产者消费者模型）

    提供高性能、无阻塞的图片URL获取接口。

    Attributes:
        _cache: URL缓存列表
        _cursor: 当前消费位置
        _cache_size: 缓存池目标大小
        _low_watermark: 低水位阈值（触发填充）
        _image_manager: 滑动窗口图片管理器引用
        _redis: Redis 客户端引用

    Example:
        >>> pool = ImageCachePool(image_manager, redis_client)
        >>> await pool.initialize()
        >>> await pool.start()
        >>> url = pool.get_url_sync()  # 同步获取
        >>> await pool.stop()
    """

    def __init__(
        self,
        image_manager: "SlidingWindowImageManager",
        redis_client: Any,
        cache_size: int = 10000,
        low_watermark_ratio: float = 0.2,
        refill_batch_size: int = 2000,
        check_interval: float = 1.0
    ):
        """
        初始化缓存池

        Args:
            image_manager: 滑动窗口图片管理器
            redis_client: Redis 客户端
            cache_size: 缓存池大小
            low_watermark_ratio: 低水位比例（0.2 = 20%）
            refill_batch_size: 每次填充数量
            check_interval: 检查间隔（秒）
        """
        self._image_manager = image_manager
        self._redis = redis_client

        # 缓存池配置
        self._cache_size = cache_size
        self._low_watermark_ratio = low_watermark_ratio
        self._low_watermark = int(cache_size * low_watermark_ratio)
        self._refill_batch_size = refill_batch_size
        self._check_interval = check_interval

        # 缓存池状态
        self._cache: List[str] = []
        self._cursor: int = 0
        self._url_set: Set[str] = set()  # 用于去重

        # 线程安全锁
        self._consume_lock = threading.Lock()  # 消费端锁（同步）
        self._refill_lock: Optional[asyncio.Lock] = None  # 生产端锁（异步）

        # 后台任务
        self._refill_task: Optional[asyncio.Task] = None
        self._running = False

        # 统计信息
        self._total_consumed = 0
        self._total_refilled = 0

    async def initialize(self) -> int:
        """
        初始化缓存池（首次填充），处理数据为空的情况

        Returns:
            初始化后的URL数量
        """
        self._refill_lock = asyncio.Lock()

        # 首次填充缓存池
        try:
            urls = await self._image_manager.get_random(self._cache_size)

            if not urls:
                logger.warning("ImageCachePool: No data available, cache pool is empty")
                with self._consume_lock:
                    self._cache = []
                    self._url_set = set()
                    self._cursor = 0
                return 0

            with self._consume_lock:
                self._cache = urls.copy()
                self._url_set = set(urls)  # 同步去重集合
                random.shuffle(self._cache)
                self._cursor = 0

            logger.info(f"ImageCachePool initialized with {len(self._cache)} URLs")
            return len(self._cache)

        except Exception as e:
            logger.error(f"Failed to initialize ImageCachePool: {e}")
            with self._consume_lock:
                self._cache = []
                self._url_set = set()
                self._cursor = 0
            return 0

    async def start(self) -> None:
        """启动后台任务"""
        if self._running:
            return

        self._running = True

        # 启动低水位监控任务
        self._refill_task = asyncio.create_task(self._refill_monitor_loop())

        logger.info("ImageCachePool background tasks started")

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

        logger.info("ImageCachePool background tasks stopped")

    def get_url_sync(self) -> str:
        """
        同步获取单个图片URL（用于模板渲染）

        O(1) 无阻塞操作，线程安全。

        Returns:
            图片URL字符串，缓存为空时返回空字符串
        """
        with self._consume_lock:
            if not self._cache:
                return ""

            # 检查是否需要重置游标
            if self._cursor >= len(self._cache):
                random.shuffle(self._cache)
                self._cursor = 0

            url = self._cache[self._cursor]
            self._cursor += 1
            self._total_consumed += 1

            return url

    def append_direct(self, url: str) -> None:
        """
        直接追加新 URL 到缓存池（新增数据立即可用）

        用于新增图片时立即加入缓存池，无需等待下次 refill。

        Args:
            url: 新图片 URL
        """
        with self._consume_lock:
            if url not in self._url_set:
                self._cache.append(url)
                self._url_set.add(url)

    def get_urls_sync(self, count: int) -> List[str]:
        """
        同步获取多个图片URL（用于模板渲染）

        Args:
            count: 需要的URL数量

        Returns:
            URL列表
        """
        result = []
        for _ in range(count):
            url = self.get_url_sync()
            if url:
                result.append(url)
        return result

    def get_remaining(self) -> int:
        """获取剩余可用URL数量"""
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

        每隔 check_interval 秒检查一次，当剩余URL < low_watermark 时触发填充。
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

        从 SlidingWindowImageManager 获取新的URL填充到缓存池。

        Returns:
            填充的URL数量
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
                new_urls = await self._image_manager.get_random(batch_size)

                if not new_urls:
                    return 0

                # 过滤掉已存在的URL
                with self._consume_lock:
                    new_unique_urls = [url for url in new_urls if url not in self._url_set]

                if not new_unique_urls:
                    logger.debug("No new unique URLs to add")
                    return 0

                # 合并到缓存池
                with self._consume_lock:
                    # 保留未消费的URL
                    remaining_urls = self._cache[self._cursor:]

                    # 合并新URL
                    self._cache = remaining_urls + new_unique_urls
                    self._url_set.update(new_unique_urls)  # 更新去重集合
                    random.shuffle(self._cache)
                    self._cursor = 0

                self._total_refilled += len(new_unique_urls)
                logger.info(f"Refilled {len(new_unique_urls)} URLs, cache size: {len(self._cache)}")

                return len(new_urls)

            except Exception as e:
                logger.error(f"Failed to refill cache: {e}")
                return 0

# ============================================
# 全局实例管理
# ============================================

_image_cache_pool: Optional[ImageCachePool] = None


def get_image_cache_pool() -> Optional[ImageCachePool]:
    """获取全局图片缓存池实例"""
    return _image_cache_pool


async def init_image_cache_pool(
    image_manager: "SlidingWindowImageManager",
    redis_client: Any,
    cache_size: int = 10000,
    low_watermark_ratio: float = 0.2,
    refill_batch_size: int = 2000,
    check_interval: float = 1.0
) -> ImageCachePool:
    """
    初始化全局图片缓存池

    Args:
        image_manager: 滑动窗口图片管理器
        redis_client: Redis 客户端
        cache_size: 缓存池大小
        low_watermark_ratio: 低水位比例
        refill_batch_size: 每次填充数量
        check_interval: 检查间隔（秒）

    Returns:
        ImageCachePool 实例
    """
    global _image_cache_pool

    _image_cache_pool = ImageCachePool(
        image_manager=image_manager,
        redis_client=redis_client,
        cache_size=cache_size,
        low_watermark_ratio=low_watermark_ratio,
        refill_batch_size=refill_batch_size,
        check_interval=check_interval
    )

    # 初始化并启动
    await _image_cache_pool.initialize()
    await _image_cache_pool.start()

    return _image_cache_pool


async def stop_image_cache_pool() -> None:
    """停止全局图片缓存池"""
    global _image_cache_pool

    if _image_cache_pool:
        await _image_cache_pool.stop()
        _image_cache_pool = None
        logger.info("Image cache pool stopped")
