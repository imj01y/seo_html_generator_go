"""
内链URL生成器模块

生成随机的内链URL，支持两种格式:
- 格式1 (60%): /?数字9位.html  例: /?123456789.html
- 格式2 (40%): /?日期8位/数字5位.html  例: /?20260110/12345.html

主要功能:
- random_url(): 生成随机内链URL
- random_internal_link(): random_url的别名
"""
import random
from datetime import datetime, timedelta
from typing import Optional, List


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

        根据配置的比例随机选择格式1或格式2。

        Returns:
            随机内链URL
        """
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
