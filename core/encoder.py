"""
HTML实体编码器模块

提供中文文本到HTML实体编码的转换功能，支持十进制和十六进制混合编码。

主要功能:
- encode(): 编码单个字符或文本
- encode_text(): 编码整个文本（保留ASCII字符）

特征:
- 十进制(&#数字;)和十六进制(&#x数字;)1:1混合编码
- 仅编码非ASCII字符（中文、Emoji等）
- 保留HTML标签、英文、数字、标点

依赖:
- random: 用于随机选择编码方式
"""
import random
from typing import Optional


class HTMLEntityEncoder:
    """
    HTML实体编码器

    将中文字符转换为HTML实体编码，支持十进制(&#数字;)和十六进制(&#x数字;)
    两种格式的1:1混合编码。

    Attributes:
        mix_ratio: 十六进制编码占比，默认0.5（1:1混合）
        _encode_count: 已编码字符计数

    Example:
        >>> encoder = HTMLEntityEncoder(mix_ratio=0.5)
        >>> result = encoder.encode_text("测试")
        >>> # 可能输出: "&#27979;&#x8bd5;" 或 "&#x6d4b;&#35797;"
    """

    def __init__(self, mix_ratio: float = 0.5):
        """
        初始化编码器

        Args:
            mix_ratio: 十六进制编码占比，0.5表示十进制:十六进制=1:1
        """
        self.mix_ratio = mix_ratio
        self._encode_count = 0

    def encode_char(self, char: str) -> str:
        """
        编码单个字符为HTML实体

        Args:
            char: 单个字符

        Returns:
            HTML实体编码字符串
        """
        code = ord(char)

        # 随机选择十进制或十六进制编码
        if random.random() < self.mix_ratio:
            return f"&#x{code:x};"  # 十六进制
        else:
            return f"&#{code};"  # 十进制

    def should_encode(self, char: str) -> bool:
        """
        判断字符是否需要编码

        规则:
        - ASCII字符(0-127)不编码，保留原样
        - 非ASCII字符（中文、Emoji等）需要编码

        Args:
            char: 单个字符

        Returns:
            True: 需要编码, False: 不需要编码
        """
        return ord(char) > 127

    def encode_text(self, text: str) -> str:
        """
        编码文本

        对文本中的非ASCII字符进行HTML实体编码，
        保留ASCII字符（英文、数字、标点、HTML标签）原样。

        Args:
            text: 待编码的文本

        Returns:
            编码后的文本
        """
        if not text:
            return ""

        result = []
        for char in text:
            if self.should_encode(char):
                result.append(self.encode_char(char))
                self._encode_count += 1
            else:
                result.append(char)

        return ''.join(result)

    def encode(self, text: str) -> str:
        """
        encode_text的别名，与模板中的使用方式一致

        Args:
            text: 待编码的文本

        Returns:
            编码后的文本
        """
        return self.encode_text(text)

    def get_encode_count(self) -> int:
        """获取已编码字符数量"""
        return self._encode_count

    def reset_count(self) -> None:
        """重置编码计数器"""
        self._encode_count = 0


# 全局编码器实例
_encoder = HTMLEntityEncoder()


def encode(text: str) -> str:
    """
    快捷函数 - 编码文本（模板中使用）

    Args:
        text: 待编码的文本

    Returns:
        编码后的文本

    Example:
        >>> encode("测试文本")
        "&#27979;&#x8bd5;&#25991;&#x672c;"
    """
    return _encoder.encode_text(text)


def encode_text(text: str) -> str:
    """
    快捷函数 - 编码文本

    Args:
        text: 待编码的文本

    Returns:
        编码后的文本
    """
    return _encoder.encode_text(text)


def encode_char(char: str) -> str:
    """
    快捷函数 - 编码单个字符

    Args:
        char: 单个字符

    Returns:
        HTML实体编码
    """
    return _encoder.encode_char(char)
