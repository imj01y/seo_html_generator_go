"""
随机Class生成器模块

生成随机的CSS class名称，格式为:
随机串1(13位) + 空格 + 随机串2(32位) + 空格 + 语义名

示例: "a1b2c3d4e5f6g h7i8j9k0l1m2n3o4p5q6r7s8t9u0v1w2 header"

主要功能:
- cls(name): 生成随机class（模板中使用）
- generate_class(semantic_name): 生成带语义名的随机class
"""
import random
import string
from typing import Dict, Optional


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

        Args:
            semantic_name: 语义名称（如 header, footer, content等）

        Returns:
            格式为 "随机串1 随机串2 语义名" 的class名称
        """
        # 如果启用缓存且已存在，返回缓存值
        if self._use_cache:
            if semantic_name in self._cache:
                return self._cache[semantic_name]

        result = f"{self._random_string(self.part1_length)} {self._random_string(self.part2_length)} {semantic_name}"

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