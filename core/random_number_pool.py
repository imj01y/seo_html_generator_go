"""
随机数池模块

预生成随机数，避免每次调用都生成。采用生产者消费者模型。

架构：
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│ ThreadPoolExecutor │ --> │   Buffer Dict   │ --> │   Consumer      │
│   (多线程生产者)    │     │ {"1-100": [...]}│     │   get_sync()    │
│   后台 refill      │     │ {"1-1000": [...]}│     │   O(1) 获取     │
└─────────────────┘     └─────────────────┘     └─────────────────┘

性能优化：
- 预生成常用范围的随机数（从模板分析得到15个常用范围）
- 使用 ThreadPoolExecutor 多线程并行生成
- 后台任务监控低水位自动 refill
- 未预配置的范围降级到直接生成
"""
import asyncio
import random
import threading
from concurrent.futures import ThreadPoolExecutor
from typing import Dict, List, Optional, Tuple

from loguru import logger


class RandomNumberPool:
    """
    随机数池（生产者消费者模型 + 多线程生产者）

    预生成常用范围的随机数，模板渲染时 O(1) 获取。
    """

    # 从模板分析得到的常用范围（共15个）
    DEFAULT_RANGES: Dict[str, Tuple[int, int]] = {
        "0-9": (0, 9),
        "0-99": (0, 99),
        "1-9": (1, 9),
        "1-10": (1, 10),
        "1-20": (1, 20),
        "1-59": (1, 59),
        "5-10": (5, 10),
        "10-20": (10, 20),
        "10-99": (10, 99),
        "10-100": (10, 100),
        "10-200": (10, 200),
        "30-90": (30, 90),
        "50-200": (50, 200),
        "100-999": (100, 999),
        "10000-99999": (10000, 99999),
    }

    def __init__(
        self,
        ranges: Optional[Dict[str, Tuple[int, int]]] = None,
        pool_size: int = 20000,
        low_watermark_ratio: float = 0.3,
        refill_batch_size: int = 5000,
        check_interval: float = 0.3,
        num_workers: int = 2
    ):
        """
        初始化随机数池

        Args:
            ranges: 预配置的范围字典，格式 {"key": (low, high)}
            pool_size: 每个范围的池大小
            low_watermark_ratio: 低水位比例，触发 refill
            refill_batch_size: 每次 refill 的数量
            check_interval: 检查间隔（秒）
            num_workers: 生产者线程数
        """
        self.ranges = ranges or self.DEFAULT_RANGES
        self._pool_size = pool_size
        self._low_watermark = int(pool_size * low_watermark_ratio)
        self._refill_batch_size = refill_batch_size
        self._check_interval = check_interval
        self._num_workers = num_workers

        # 每个范围独立的缓冲区和游标
        self._buffers: Dict[str, List[int]] = {}
        self._cursors: Dict[str, int] = {}

        # 线程安全
        self._consume_lock = threading.Lock()

        # 多线程生产者
        self._executor: Optional[ThreadPoolExecutor] = None

        # 后台 refill 任务
        self._refill_task: Optional[asyncio.Task] = None
        self._running = False

        # 统计
        self._total_consumed = 0
        self._total_refilled = 0
        self._fallback_count = 0  # 降级生成的次数

    @staticmethod
    def _generate_batch_worker(low: int, high: int, count: int) -> List[int]:
        """
        工作线程：批量生成指定范围的随机数

        Args:
            low: 最小值
            high: 最大值
            count: 生成数量

        Returns:
            随机数列表
        """
        return [random.randint(low, high) for _ in range(count)]

    async def initialize(self) -> int:
        """
        初始化池（多线程并行填充所有范围）

        Returns:
            生成的总随机数数量
        """
        self._executor = ThreadPoolExecutor(
            max_workers=self._num_workers,
            thread_name_prefix="rnd_pool"
        )

        loop = asyncio.get_event_loop()
        tasks = []

        for key, (low, high) in self.ranges.items():
            task = loop.run_in_executor(
                self._executor,
                self._generate_batch_worker,
                low, high, self._pool_size
            )
            tasks.append((key, task))

        # 并行初始化所有范围
        for key, task in tasks:
            self._buffers[key] = await task
            self._cursors[key] = 0

        total = sum(len(buf) for buf in self._buffers.values())
        logger.info(
            f"RandomNumberPool initialized: {len(self.ranges)} ranges, "
            f"{total} numbers, {self._num_workers} workers"
        )
        return total

    async def start(self) -> None:
        """启动后台 refill 任务"""
        if self._running:
            return
        self._running = True
        self._refill_task = asyncio.create_task(self._refill_monitor_loop())

    async def stop(self) -> None:
        """停止后台任务和线程池"""
        self._running = False

        if self._refill_task:
            self._refill_task.cancel()
            try:
                await self._refill_task
            except asyncio.CancelledError:
                pass

        if self._executor:
            self._executor.shutdown(wait=False)
            self._executor = None

        logger.info(
            f"RandomNumberPool stopped: consumed={self._total_consumed}, "
            f"refilled={self._total_refilled}, fallback={self._fallback_count}"
        )

    def get_sync(self, low: int, high: int) -> int:
        """
        同步获取随机数（O(1) 无阻塞）

        Args:
            low: 最小值
            high: 最大值

        Returns:
            随机数
        """
        key = f"{low}-{high}"

        with self._consume_lock:
            if key in self._buffers:
                buf = self._buffers[key]
                cursor = self._cursors[key]

                # 循环使用缓冲区
                if cursor >= len(buf):
                    self._cursors[key] = 0
                    cursor = 0

                result = buf[cursor]
                self._cursors[key] = cursor + 1
                self._total_consumed += 1
                return result

        # 降级：未预配置的范围直接生成
        self._fallback_count += 1
        return random.randint(low, high)

    def _get_remaining(self, key: str) -> int:
        """获取指定范围的剩余数量（调用者需持有锁）"""
        if key not in self._buffers:
            return 0
        return len(self._buffers[key]) - self._cursors[key]

    def get_remaining(self, key: str) -> int:
        """获取指定范围的剩余数量"""
        with self._consume_lock:
            return self._get_remaining(key)

    async def _refill_monitor_loop(self) -> None:
        """后台任务：监控低水位并多线程 refill"""
        while self._running:
            try:
                await asyncio.sleep(self._check_interval)

                for key, (low, high) in self.ranges.items():
                    with self._consume_lock:
                        remaining = self._get_remaining(key)

                    if remaining < self._low_watermark:
                        await self._refill_range(key, low, high)

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.warning(f"RandomNumberPool refill error: {e}")
                await asyncio.sleep(1)

    async def _refill_range(self, key: str, low: int, high: int) -> int:
        """
        多线程 refill 指定范围

        Args:
            key: 范围键
            low: 最小值
            high: 最大值

        Returns:
            新增的随机数数量
        """
        if not self._executor:
            return 0

        loop = asyncio.get_event_loop()
        new_numbers = await loop.run_in_executor(
            self._executor,
            self._generate_batch_worker,
            low, high, self._refill_batch_size
        )

        with self._consume_lock:
            # 保留未消费的
            remaining = self._buffers[key][self._cursors[key]:]
            # 合并新数据
            self._buffers[key] = remaining + new_numbers
            self._cursors[key] = 0

        self._total_refilled += len(new_numbers)
        return len(new_numbers)

    def get_stats(self) -> dict:
        """获取统计信息"""
        with self._consume_lock:
            range_stats = {}
            for key in self.ranges:
                if key in self._buffers:
                    range_stats[key] = {
                        "buffer_size": len(self._buffers[key]),
                        "remaining": self._get_remaining(key)
                    }

            return {
                "ranges_count": len(self.ranges),
                "pool_size": self._pool_size,
                "low_watermark": self._low_watermark,
                "total_consumed": self._total_consumed,
                "total_refilled": self._total_refilled,
                "fallback_count": self._fallback_count,
                "running": self._running,
                "range_stats": range_stats
            }


# ============================================
# 全局 RandomNumberPool 实例管理
# ============================================

_random_number_pool: Optional[RandomNumberPool] = None


def get_random_number_pool() -> Optional[RandomNumberPool]:
    """获取全局随机数池实例"""
    return _random_number_pool


async def init_random_number_pool(**kwargs) -> RandomNumberPool:
    """
    初始化全局随机数池

    Args:
        **kwargs: 传递给 RandomNumberPool 构造函数的参数

    Returns:
        初始化后的 RandomNumberPool 实例
    """
    global _random_number_pool
    _random_number_pool = RandomNumberPool(**kwargs)
    await _random_number_pool.initialize()
    await _random_number_pool.start()
    return _random_number_pool


async def stop_random_number_pool() -> None:
    """停止全局随机数池"""
    global _random_number_pool
    if _random_number_pool:
        await _random_number_pool.stop()
        _random_number_pool = None
