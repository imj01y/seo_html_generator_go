# -*- coding: utf-8 -*-
"""
Bloom Filter 去重器

特性：
- 内存占用小：1亿条约 ~30MB（误判率0.1%）
- 定期持久化到 Redis（默认5分钟）
- 支持 URL、标题、段落多层去重
"""

import hashlib
import asyncio
import pickle
from typing import Optional, List
from loguru import logger

try:
    from pybloom_live import ScalableBloomFilter
except ImportError:
    logger.warning("pybloom_live not installed, run: pip install pybloom-live")
    ScalableBloomFilter = None


class BloomDeduplicator:
    """
    Bloom Filter 去重器

    特性：
    - 内存占用小：1亿条约 ~30MB（误判率0.1%）
    - 定期持久化到 Redis（默认5分钟）
    - 可扩展容量
    """

    def __init__(self, redis_client, key_prefix: str, error_rate: float = 0.001,
                 save_interval: int = 300, initial_capacity: int = 1000000):
        """
        初始化 Bloom Filter 去重器

        Args:
            redis_client: Redis 异步客户端
            key_prefix: Redis 键前缀
            error_rate: 误判率（默认 0.1%）
            save_interval: 持久化间隔（秒，默认5分钟）
            initial_capacity: 初始容量
        """
        self.redis = redis_client
        self.key_prefix = key_prefix
        self.error_rate = error_rate
        self.save_interval = save_interval
        self.initial_capacity = initial_capacity

        if ScalableBloomFilter:
            self._bf = ScalableBloomFilter(
                initial_capacity=initial_capacity,
                error_rate=error_rate,
                mode=ScalableBloomFilter.LARGE_SET_GROWTH
            )
        else:
            self._bf = set()  # 降级为普通 set
            logger.warning(f"Using set instead of BloomFilter for {key_prefix}")

        self._save_task: Optional[asyncio.Task] = None
        self._dirty: bool = False
        self._count: int = 0

    def _hash(self, content: str) -> str:
        """
        计算内容哈希

        去除空白字符后计算 MD5，确保相似内容被识别为重复
        """
        normalized = ''.join(content.split())
        return hashlib.md5(normalized.encode('utf-8')).hexdigest()

    def exists(self, content: str) -> bool:
        """检查内容是否存在（可能有极小误判）"""
        h = self._hash(content)
        if isinstance(self._bf, set):
            return h in self._bf
        return h in self._bf

    def add(self, content: str) -> bool:
        """
        添加到过滤器

        Returns:
            是否为新增（True=新增成功，False=已存在）
        """
        h = self._hash(content)
        if isinstance(self._bf, set):
            if h in self._bf:
                return False
            self._bf.add(h)
        else:
            if h in self._bf:
                return False
            self._bf.add(h)

        self._dirty = True
        self._count += 1
        return True

    def add_if_new(self, content: str) -> bool:
        """如果不存在则添加，返回是否成功添加（add 的别名）"""
        return self.add(content)

    async def save_to_redis(self):
        """持久化到 Redis"""
        if not self._dirty:
            return

        try:
            data = pickle.dumps(self._bf)
            await self.redis.set(f"{self.key_prefix}:bloom", data)
            self._dirty = False
            logger.debug(f"Bloom filter '{self.key_prefix}' saved to Redis ({self._count} items)")
        except Exception as e:
            logger.error(f"Failed to save bloom filter '{self.key_prefix}': {e}")

    async def load_from_redis(self):
        """从 Redis 加载"""
        try:
            data = await self.redis.get(f"{self.key_prefix}:bloom")
            if data:
                self._bf = pickle.loads(data)
                # 估算数量
                if isinstance(self._bf, set):
                    self._count = len(self._bf)
                else:
                    self._count = getattr(self._bf, 'count', 0)
                logger.info(f"Bloom filter '{self.key_prefix}' loaded from Redis (~{self._count} items)")
        except Exception as e:
            logger.warning(f"Failed to load bloom filter '{self.key_prefix}': {e}")

    async def start_auto_save(self):
        """启动定期保存任务"""
        if self._save_task:
            return

        async def _save_loop():
            while True:
                try:
                    await asyncio.sleep(self.save_interval)
                    await self.save_to_redis()
                except asyncio.CancelledError:
                    break
                except Exception as e:
                    logger.error(f"Auto save error for '{self.key_prefix}': {e}")

        self._save_task = asyncio.create_task(_save_loop())
        logger.debug(f"Auto save started for '{self.key_prefix}' (interval: {self.save_interval}s)")

    async def stop_auto_save(self):
        """停止定期保存并执行最后一次保存"""
        if self._save_task:
            self._save_task.cancel()
            try:
                await self._save_task
            except asyncio.CancelledError:
                pass
            self._save_task = None

        # 退出前保存
        await self.save_to_redis()
        logger.debug(f"Auto save stopped for '{self.key_prefix}'")

    def get_count(self) -> int:
        """获取已添加的数量（估算值）"""
        return self._count

    def clear(self):
        """清空过滤器"""
        if ScalableBloomFilter:
            self._bf = ScalableBloomFilter(
                initial_capacity=self.initial_capacity,
                error_rate=self.error_rate,
                mode=ScalableBloomFilter.LARGE_SET_GROWTH
            )
        else:
            self._bf = set()
        self._count = 0
        self._dirty = True


class ContentDeduplicator:
    """
    内容去重器

    整合 URL、标题、段落的去重功能，并管理段落队列。
    """

    def __init__(self, redis_client, queue_key: str = "queue:paragraphs",
                 save_interval: int = 300):
        """
        初始化内容去重器

        Args:
            redis_client: Redis 异步客户端
            queue_key: 段落队列的 Redis 键
            save_interval: Bloom Filter 持久化间隔（秒）
        """
        self.redis = redis_client
        self.queue_key = queue_key

        # URL 去重
        self.url_dedup = BloomDeduplicator(
            redis_client, "dedup:url",
            save_interval=save_interval
        )
        # 标题去重
        self.title_dedup = BloomDeduplicator(
            redis_client, "dedup:title",
            save_interval=save_interval
        )
        # 段落内容去重
        self.para_dedup = BloomDeduplicator(
            redis_client, "dedup:para",
            save_interval=save_interval
        )

        self._initialized = False

    async def init(self):
        """初始化：从 Redis 加载已有数据，启动定期保存"""
        if self._initialized:
            return

        logger.info("Initializing content deduplicator...")

        # 加载已有数据
        await self.url_dedup.load_from_redis()
        await self.title_dedup.load_from_redis()
        await self.para_dedup.load_from_redis()

        # 启动定期保存
        await self.url_dedup.start_auto_save()
        await self.title_dedup.start_auto_save()
        await self.para_dedup.start_auto_save()

        self._initialized = True
        logger.info("Content deduplicator initialized")

    async def cleanup(self):
        """清理：停止定期保存，执行最后一次保存"""
        logger.info("Cleaning up content deduplicator...")

        await self.url_dedup.stop_auto_save()
        await self.title_dedup.stop_auto_save()
        await self.para_dedup.stop_auto_save()

        self._initialized = False
        logger.info("Content deduplicator cleanup completed")

    def should_crawl(self, url: str) -> bool:
        """
        检查 URL 是否应该抓取

        Returns:
            True=新URL应该抓取，False=已抓取过跳过
        """
        return self.url_dedup.add_if_new(url)

    def should_save_title(self, title: str) -> bool:
        """
        检查标题是否应该保存

        Returns:
            True=新标题应该保存，False=已存在跳过
        """
        if not title or not title.strip():
            return False
        return self.title_dedup.add_if_new(title)

    async def push_paragraph(self, content: str) -> bool:
        """
        段落入队（去重后）

        Returns:
            True=新段落已入队，False=重复段落已丢弃
        """
        if not content or not content.strip():
            return False

        if self.para_dedup.add_if_new(content):
            await self.redis.rpush(self.queue_key, content.encode('utf-8'))
            return True
        return False

    async def push_paragraphs(self, contents: List[str]) -> int:
        """
        批量段落入队

        Returns:
            成功入队的数量
        """
        count = 0
        for content in contents:
            if await self.push_paragraph(content):
                count += 1
        return count

    async def pop_paragraph(self) -> Optional[str]:
        """
        取出一个段落（取出即删除）

        Returns:
            段落内容，队列为空返回 None
        """
        result = await self.redis.lpop(self.queue_key)
        if result:
            return result.decode('utf-8') if isinstance(result, bytes) else result
        return None

    async def pop_paragraphs(self, count: int) -> List[str]:
        """
        批量取出段落（取出即删除）

        Args:
            count: 要取出的数量

        Returns:
            段落列表
        """
        if count <= 0:
            return []

        # 使用 pipeline 批量操作
        pipe = self.redis.pipeline()
        for _ in range(count):
            pipe.lpop(self.queue_key)
        results = await pipe.execute()

        paragraphs = []
        for r in results:
            if r:
                text = r.decode('utf-8') if isinstance(r, bytes) else r
                paragraphs.append(text)

        return paragraphs

    async def get_queue_size(self) -> int:
        """获取段落队列长度"""
        return await self.redis.llen(self.queue_key)

    def get_stats(self) -> dict:
        """获取统计信息"""
        return {
            'url_count': self.url_dedup.get_count(),
            'title_count': self.title_dedup.get_count(),
            'paragraph_count': self.para_dedup.get_count(),
            'initialized': self._initialized
        }
