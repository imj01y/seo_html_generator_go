"""
随机Class生成器模块

生成随机的CSS class名称，格式为:
随机串1(13位) + 空格 + 随机串2(32位) + 空格 + 语义名

示例: "a1b2c3d4e5f6g h7i8j9k0l1m2n3o4p5q6r7s8t9u0v1w2 header"

主要功能:
- cls(name): 生成随机class（模板中使用）
- generate_class(semantic_name): 生成带语义名的随机class

性能优化:
- ClassStringPool: 生成器 + 缓冲区 + 生产者消费者模型
- 预生成随机字符串，避免每次调用都生成
"""
import asyncio
import random
import string
import threading
from typing import Dict, List, Optional, Generator


# ============================================
# ClassStringPool - 随机字符串池（生产者消费者模型）
# ============================================

class ClassStringPool:
    """
    随机字符串池（生成器 + 缓冲区 + 生产者消费者模型）

    使用无限生成器作为数据源，后台任务按需填充缓冲区。
    消费端从缓冲区 O(1) 获取，避免每次调用都生成随机字符串。
    """

    def __init__(
        self,
        part1_length: int = 13,
        part2_length: int = 32,
        chars: str = None,
        pool_size: int = 5000,
        low_watermark_ratio: float = 0.2,
        refill_batch_size: int = 1000,
        check_interval: float = 0.5
    ):
        self.part1_length = part1_length
        self.part2_length = part2_length
        self.chars = chars or (string.ascii_lowercase + string.digits)

        # 池配置
        self._pool_size = pool_size
        self._low_watermark = int(pool_size * low_watermark_ratio)
        self._refill_batch_size = refill_batch_size
        self._check_interval = check_interval

        # 生成器（数据源，不占内存）
        self._generator_part1: Optional[Generator[str, None, None]] = None
        self._generator_part2: Optional[Generator[str, None, None]] = None

        # 缓冲区（按需填充）
        self._buffer_part1: List[str] = []
        self._buffer_part2: List[str] = []
        self._cursor1: int = 0
        self._cursor2: int = 0

        # 线程安全
        self._consume_lock = threading.Lock()

        # 后台任务
        self._refill_task: Optional[asyncio.Task] = None
        self._running = False

        # 统计
        self._total_consumed = 0
        self._total_refilled = 0

    def _create_generator(self, length: int) -> Generator[str, None, None]:
        """
        创建无限随机字符串生成器

        生成器本身不占内存，只在 next() 调用时生成字符串。
        """
        while True:
            yield ''.join(random.choices(self.chars, k=length))

    def _take_from_generator(self, generator: Generator[str, None, None], count: int) -> List[str]:
        """从生成器取指定数量的字符串"""
        return [next(generator) for _ in range(count)]

    async def initialize(self) -> int:
        """初始化池（创建生成器并首次填充缓冲区）"""
        # 创建生成器
        self._generator_part1 = self._create_generator(self.part1_length)
        self._generator_part2 = self._create_generator(self.part2_length)

        # 首次填充缓冲区
        self._buffer_part1 = self._take_from_generator(self._generator_part1, self._pool_size)
        self._buffer_part2 = self._take_from_generator(self._generator_part2, self._pool_size)

        self._cursor1 = 0
        self._cursor2 = 0

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

    def get_part1_sync(self) -> str:
        """同步获取 part1 字符串（O(1) 无阻塞）"""
        with self._consume_lock:
            if self._cursor1 >= len(self._buffer_part1):
                self._cursor1 = 0
            result = self._buffer_part1[self._cursor1]
            self._cursor1 += 1
            self._total_consumed += 1
            return result

    def get_part2_sync(self) -> str:
        """同步获取 part2 字符串（O(1) 无阻塞）"""
        with self._consume_lock:
            if self._cursor2 >= len(self._buffer_part2):
                self._cursor2 = 0
            result = self._buffer_part2[self._cursor2]
            self._cursor2 += 1
            return result

    def _get_remaining_unlocked(self) -> int:
        """内部方法：获取剩余数量（调用者需持有锁）"""
        return min(
            len(self._buffer_part1) - self._cursor1,
            len(self._buffer_part2) - self._cursor2
        )

    def get_remaining(self) -> int:
        """获取剩余可用数量"""
        with self._consume_lock:
            return self._get_remaining_unlocked()

    async def _refill_monitor_loop(self) -> None:
        """后台任务：监控低水位并自动从生成器填充"""
        while self._running:
            try:
                await asyncio.sleep(self._check_interval)
                remaining = self.get_remaining()

                if remaining < self._low_watermark:
                    await self._refill()
            except asyncio.CancelledError:
                break
            except Exception:
                await asyncio.sleep(1)

    async def _refill(self) -> int:
        """从生成器取值补充缓冲区"""
        # 从生成器取新数据
        new_part1 = self._take_from_generator(self._generator_part1, self._refill_batch_size)
        new_part2 = self._take_from_generator(self._generator_part2, self._refill_batch_size)

        with self._consume_lock:
            # 保留未消费的
            remaining1 = self._buffer_part1[self._cursor1:]
            remaining2 = self._buffer_part2[self._cursor2:]

            # 合并到缓冲区
            self._buffer_part1 = remaining1 + new_part1
            self._buffer_part2 = remaining2 + new_part2

            # 重置游标
            self._cursor1 = 0
            self._cursor2 = 0

        self._total_refilled += self._refill_batch_size
        return self._refill_batch_size

    def get_stats(self) -> dict:
        """获取统计信息"""
        with self._consume_lock:
            return {
                "buffer_size": len(self._buffer_part1),
                "remaining": self._get_remaining_unlocked(),
                "low_watermark": self._low_watermark,
                "total_consumed": self._total_consumed,
                "total_refilled": self._total_refilled,
                "running": self._running
            }


# ============================================
# 全局 ClassStringPool 实例管理
# ============================================

_class_string_pool: Optional[ClassStringPool] = None


def get_class_string_pool() -> Optional[ClassStringPool]:
    """获取全局随机字符串池实例"""
    return _class_string_pool


async def init_class_string_pool(**kwargs) -> ClassStringPool:
    """初始化全局随机字符串池"""
    global _class_string_pool
    _class_string_pool = ClassStringPool(**kwargs)
    await _class_string_pool.initialize()
    await _class_string_pool.start()
    return _class_string_pool


async def stop_class_string_pool() -> None:
    """停止全局随机字符串池"""
    global _class_string_pool
    if _class_string_pool:
        await _class_string_pool.stop()
        _class_string_pool = None


# ============================================
# ClassGenerator - 随机Class生成器
# ============================================

class ClassGenerator:
    """
    随机Class生成器

    生成格式为 "随机串1 随机串2 语义名" 的CSS class名称。
    每次调用生成不同的随机串，确保页面间的差异性。

    Attributes:
        part1_length: 随机串1的长度，默认13
        part2_length: 随机串2的长度，默认32
        chars: 用于生成随机串的字符集

    Example:
        >>> generator = ClassGenerator()
        >>> generator.generate("header")
        "q6f7w8037qc22 tz2r6otv8k4pv4u4n8or5alv6nl1lwi1 header"
    """

    def __init__(
        self,
        part1_length: int = 13,
        part2_length: int = 32,
        chars: Optional[str] = None
    ):
        """
        初始化生成器

        Args:
            part1_length: 随机串1的长度
            part2_length: 随机串2的长度
            chars: 用于生成的字符集，默认小写字母+数字
        """
        self.part1_length = part1_length
        self.part2_length = part2_length
        self.chars = chars or (string.ascii_lowercase + string.digits)
        self._cache: Dict[str, str] = {}
        self._use_cache = False

    def _random_string(self, length: int) -> str:
        """生成指定长度的随机字符串"""
        return ''.join(random.choices(self.chars, k=length))

    def generate(self, semantic_name: str) -> str:
        """
        生成随机class名称

        优先使用全局字符串池（O(1)），降级到直接生成。

        Args:
            semantic_name: 语义名称（如 header, footer, content等）

        Returns:
            格式为 "随机串1 随机串2 语义名" 的class名称
        """
        # 如果启用缓存且已存在，返回缓存值
        if self._use_cache:
            if semantic_name in self._cache:
                return self._cache[semantic_name]

        # 优先从池获取预生成的随机字符串
        pool = get_class_string_pool()
        if pool:
            part1 = pool.get_part1_sync()
            part2 = pool.get_part2_sync()
        else:
            # 降级：直接生成（池未初始化时）
            part1 = self._random_string(self.part1_length)
            part2 = self._random_string(self.part2_length)

        result = f"{part1} {part2} {semantic_name}"

        if self._use_cache:
            self._cache[semantic_name] = result

        return result

    def cls(self, name: str) -> str:
        """
        generate的别名，与模板中的使用方式一致

        Args:
            name: 语义名称

        Returns:
            随机class名称
        """
        return self.generate(name)

    def enable_cache(self) -> None:
        """
        启用缓存

        启用后，相同的语义名在同一页面内返回相同的class。
        用于需要在多处引用同一class的场景。
        """
        self._use_cache = True

    def disable_cache(self) -> None:
        """禁用缓存"""
        self._use_cache = False
        self._cache.clear()

    def clear_cache(self) -> None:
        """清空缓存（用于新页面开始时）"""
        self._cache.clear()

    def reset(self) -> None:
        """重置生成器状态（清空缓存）"""
        self._cache.clear()

    def get_cached(self, semantic_name: str) -> Optional[str]:
        """获取缓存的class名称"""
        return self._cache.get(semantic_name)


# 全局生成器实例
_generator = ClassGenerator()


def cls(name: str) -> str:
    """
    快捷函数 - 生成随机class（模板中使用）

    Args:
        name: 语义名称（如 header, footer, content）

    Returns:
        随机class名称

    Example:
        >>> cls("header")
        "a1b2c3d4e5f6g h7i8j9k0l1m2n3o4p5q6r7s8t9u0v1w2 header"
    """
    return _generator.generate(name)


def generate_class(semantic_name: str) -> str:
    """
    快捷函数 - 生成随机class

    Args:
        semantic_name: 语义名称

    Returns:
        随机class名称
    """
    return _generator.generate(semantic_name)


def get_generator() -> ClassGenerator:
    """获取全局生成器实例"""
    return _generator