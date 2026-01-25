"""
内链URL生成器模块

生成随机的内链URL，支持两种格式:
- 格式1 (60%): /?数字9位.html  例: /?123456789.html
- 格式2 (40%): /?日期8位/数字5位.html  例: /?20260110/12345.html

主要功能:
- random_url(): 生成随机内链URL
- random_internal_link(): random_url的别名

性能优化:
- URLPool: 生成器 + 缓冲区 + 生产者消费者模型
- 预生成URL，避免每次调用都生成
"""
import asyncio
import random
import threading
from datetime import datetime, timedelta
from typing import Optional, List, Generator


# ============================================
# URLPool - URL池（生产者消费者模型）
# ============================================

class URLPool:
    """
    URL 池（生成器 + 缓冲区 + 生产者消费者模型）

    使用无限生成器作为数据源，后台任务按需填充缓冲区。
    """

    def __init__(
        self,
        format1_ratio: float = 0.6,
        date_range_days: int = 30,
        pool_size: int = 5000,
        low_watermark_ratio: float = 0.2,
        refill_batch_size: int = 1000,
        check_interval: float = 0.5
    ):
        self.format1_ratio = format1_ratio
        self.date_range_days = date_range_days

        # 池配置
        self._pool_size = pool_size
        self._low_watermark = int(pool_size * low_watermark_ratio)
        self._refill_batch_size = refill_batch_size
        self._check_interval = check_interval

        # 生成器（数据源）
        self._generator: Optional[Generator[str, None, None]] = None

        # 缓冲区
        self._buffer: List[str] = []
        self._cursor: int = 0

        # 线程安全
        self._consume_lock = threading.Lock()

        # 后台任务
        self._refill_task: Optional[asyncio.Task] = None
        self._running = False

        # 统计
        self._total_consumed = 0
        self._total_refilled = 0

    def _create_generator(self) -> Generator[str, None, None]:
        """创建无限 URL 生成器"""
        while True:
            if random.random() < self.format1_ratio:
                # 格式1: /?数字9位.html
                number = random.randint(100000000, 999999999)
                yield f"/?{number}.html"
            else:
                # 格式2: /?日期8位/数字5位.html
                days_ago = random.randint(0, self.date_range_days)
                date = datetime.now() - timedelta(days=days_ago)
                date_str = date.strftime("%Y%m%d")
                number = random.randint(10000, 99999)
                yield f"/?{date_str}/{number}.html"

    def _take_from_generator(self, count: int) -> List[str]:
        """从生成器取指定数量的 URL"""
        return [next(self._generator) for _ in range(count)]

    async def initialize(self) -> int:
        """初始化池"""
        self._generator = self._create_generator()
        self._buffer = self._take_from_generator(self._pool_size)
        self._cursor = 0
        return self._pool_size

    async def start(self) -> None:
        """启动后台任务"""
        if self._running:
            return
        self._running = True
        self._refill_task = asyncio.create_task(self._refill_monitor_loop())

    async def stop(self) -> None:
        """停止后台任务"""
        self._running = False
        if self._refill_task:
            self._refill_task.cancel()
            try:
                await self._refill_task
            except asyncio.CancelledError:
                pass

    def get_sync(self) -> str:
        """同步获取 URL（O(1) 无阻塞）"""
        with self._consume_lock:
            if self._cursor >= len(self._buffer):
                self._cursor = 0
            result = self._buffer[self._cursor]
            self._cursor += 1
            self._total_consumed += 1
            return result

    def _get_remaining_unlocked(self) -> int:
        """内部方法：获取剩余数量（调用者需持有锁）"""
        return len(self._buffer) - self._cursor

    def get_remaining(self) -> int:
        """获取剩余可用数量"""
        with self._consume_lock:
            return self._get_remaining_unlocked()

    async def _refill_monitor_loop(self) -> None:
        """后台任务：监控低水位并自动填充"""
        while self._running:
            try:
                await asyncio.sleep(self._check_interval)
                if self.get_remaining() < self._low_watermark:
                    await self._refill()
            except asyncio.CancelledError:
                break
            except Exception:
                await asyncio.sleep(1)

    async def _refill(self) -> int:
        """从生成器取值补充缓冲区"""
        new_urls = self._take_from_generator(self._refill_batch_size)

        with self._consume_lock:
            remaining = self._buffer[self._cursor:]
            self._buffer = remaining + new_urls
            self._cursor = 0

        self._total_refilled += self._refill_batch_size
        return self._refill_batch_size

    def get_stats(self) -> dict:
        """获取统计信息"""
        with self._consume_lock:
            return {
                "buffer_size": len(self._buffer),
                "remaining": self._get_remaining_unlocked(),
                "low_watermark": self._low_watermark,
                "total_consumed": self._total_consumed,
                "total_refilled": self._total_refilled,
                "running": self._running
            }


# ============================================
# 全局 URLPool 实例管理
# ============================================

_url_pool: Optional[URLPool] = None


def get_url_pool() -> Optional[URLPool]:
    """获取全局 URL 池实例"""
    return _url_pool


async def init_url_pool(**kwargs) -> URLPool:
    """初始化全局 URL 池"""
    global _url_pool
    _url_pool = URLPool(**kwargs)
    await _url_pool.initialize()
    await _url_pool.start()
    return _url_pool


async def stop_url_pool() -> None:
    """停止全局 URL 池"""
    global _url_pool
    if _url_pool:
        await _url_pool.stop()
        _url_pool = None


# ============================================
# LinkGenerator - 内链URL生成器
# ============================================

class LinkGenerator:
    """
    内链URL生成器

    生成随机的内链URL，用于SEO优化。
    支持两种URL格式，可配置比例。

    Attributes:
        format1_ratio: 格式1的比例，默认0.6
        date_range_days: 日期格式的天数范围，默认30天

    Example:
        >>> generator = LinkGenerator()
        >>> url = generator.generate()
        >>> print(url)  # "/?123456789.html" 或 "/?20260110/12345.html"
    """

    def __init__(
        self,
        format1_ratio: float = 0.6,
        date_range_days: int = 30
    ):
        """
        初始化生成器

        Args:
            format1_ratio: 格式1(/?数字9位.html)的比例，默认0.6
            date_range_days: 格式2中日期的随机范围天数
        """
        self.format1_ratio = format1_ratio
        self.date_range_days = date_range_days

    def _generate_format1(self) -> str:
        """
        生成格式1的URL: /?数字9位.html

        Returns:
            URL字符串
        """
        number = random.randint(100000000, 999999999)
        return f"/?{number}.html"

    def _generate_format2(self) -> str:
        """
        生成格式2的URL: /?日期8位/数字5位.html

        Returns:
            URL字符串
        """
        # 随机选择最近N天内的日期
        days_ago = random.randint(0, self.date_range_days)
        date = datetime.now() - timedelta(days=days_ago)
        date_str = date.strftime("%Y%m%d")

        number = random.randint(10000, 99999)
        return f"/?{date_str}/{number}.html"

    def generate(self) -> str:
        """
        生成随机内链URL

        优先使用全局 URL 池（O(1)），降级到直接生成。

        Returns:
            随机内链URL
        """
        # 优先从池获取预生成的 URL
        pool = get_url_pool()
        if pool:
            return pool.get_sync()

        # 降级：直接生成（池未初始化时）
        if random.random() < self.format1_ratio:
            return self._generate_format1()
        else:
            return self._generate_format2()

    def random_url(self) -> str:
        """
        generate的别名，与模板中的使用方式一致

        Returns:
            随机内链URL
        """
        return self.generate()

    def generate_batch(self, count: int) -> List[str]:
        """
        批量生成内链URL

        Args:
            count: 生成数量

        Returns:
            URL列表
        """
        return [self.generate() for _ in range(count)]


class AnchorGenerator:
    """
    锚文本生成器

    生成带有HTML实体编码和Emoji的随机锚文本。

    Example:
        >>> from core.encoder import encode
        >>> from core.emoji import get_random_emoji
        >>> generator = AnchorGenerator(keywords=["软件下载", "免费工具"])
        >>> anchor = generator.generate()
    """

    def __init__(
        self,
        keywords: Optional[list] = None,
        encoder=None,
        emoji_manager=None
    ):
        """
        初始化生成器

        Args:
            keywords: 关键词列表
            encoder: HTML编码器实例
            emoji_manager: Emoji管理器实例
        """
        self.keywords = keywords or []
        self._encoder = encoder
        self._emoji_manager = emoji_manager

    def set_keywords(self, keywords: list) -> None:
        """设置关键词列表"""
        self.keywords = keywords

    def generate(self, with_emoji: bool = True) -> str:
        """
        生成随机锚文本

        Args:
            with_emoji: 是否添加Emoji

        Returns:
            编码后的锚文本
        """
        if not self.keywords:
            return ""

        # 随机选择关键词
        keyword = random.choice(self.keywords)

        # 延迟导入避免循环依赖
        if self._encoder is None:
            from .encoder import encode
            encoded = encode(keyword)
        else:
            encoded = self._encoder.encode(keyword)

        # 添加Emoji
        if with_emoji:
            if self._emoji_manager is None:
                from .emoji import get_random_emoji
                emoji = get_random_emoji()
            else:
                emoji = self._emoji_manager.get_random()
            return f"{encoded}{emoji}"

        return encoded

    def random_anchor(self) -> str:
        """
        generate的别名，与模板中的使用方式一致

        Returns:
            随机锚文本
        """
        return self.generate()


# 全局实例
_link_generator = LinkGenerator()
_anchor_generator = AnchorGenerator()


def random_url() -> str:
    """
    快捷函数 - 生成随机内链URL（模板中使用）

    Returns:
        随机内链URL

    Example:
        >>> random_url()
        "/?123456789.html"
    """
    return _link_generator.generate()


def random_internal_link() -> str:
    """
    快捷函数 - random_url的别名

    Returns:
        随机内链URL
    """
    return _link_generator.generate()


def random_anchor() -> str:
    """
    快捷函数 - 生成随机锚文本（模板中使用）

    需要先设置关键词列表。

    Returns:
        编码后的锚文本
    """
    return _anchor_generator.generate()


def set_anchor_keywords(keywords: list) -> None:
    """
    设置锚文本关键词列表

    Args:
        keywords: 关键词列表
    """
    _anchor_generator.set_keywords(keywords)


def get_link_generator() -> LinkGenerator:
    """获取全局链接生成器"""
    return _link_generator


def get_anchor_generator() -> AnchorGenerator:
    """获取全局锚文本生成器"""
    return _anchor_generator


def create_link_generator(
    format1_ratio: float = 0.6,
    date_range_days: int = 30
) -> LinkGenerator:
    """
    创建新的链接生成器实例

    Args:
        format1_ratio: 格式1的比例
        date_range_days: 日期范围

    Returns:
        LinkGenerator实例
    """
    return LinkGenerator(
        format1_ratio=format1_ratio,
        date_range_days=date_range_days
    )


def generate_url_list(count: int = 100) -> List[str]:
    """
    快捷函数 - 批量生成内链URL

    Args:
        count: 生成数量

    Returns:
        URL列表
    """
    return _link_generator.generate_batch(count)
