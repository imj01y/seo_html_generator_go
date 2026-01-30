"""
Title生成器模块

生成SEO优化的页面标题，格式为:
关键词1 + Emoji1 + 关键词2 + Emoji2 + 关键词3

示例: "软件下载😀免费工具🔥在线服务"

主要功能:
- generate_title(): 生成页面标题
- generate_title_encoded(): 生成编码后的标题
"""
from typing import List, Tuple, Optional, Set

from .emoji import EmojiManager, get_random_emoji


class TitleGenerator:
    """
    Title生成器

    生成SEO优化的页面标题，采用"关键词+Emoji"交替结构。

    Attributes:
        emoji_manager: Emoji管理器实例
        keywords_count: 标题中的关键词数量
        emojis_count: 标题中的Emoji数量

    Example:
        >>> generator = TitleGenerator()
        >>> title = generator.generate(["软件下载", "免费工具", "在线服务"])
        >>> print(title)  # "软件下载😀免费工具🔥在线服务"
    """

    def __init__(
        self,
        emoji_manager: Optional[EmojiManager] = None,
        keywords_count: int = 3,
        emojis_count: int = 2
    ):
        """
        初始化生成器

        Args:
            emoji_manager: Emoji管理器实例，None时使用全局实例
            keywords_count: 标题中的关键词数量，默认3
            emojis_count: 标题中的Emoji数量，默认2
        """
        self._emoji_manager = emoji_manager
        self.keywords_count = keywords_count
        self.emojis_count = emojis_count

    @property
    def emoji_manager(self) -> EmojiManager:
        """获取Emoji管理器"""
        if self._emoji_manager is None:
            from .emoji import get_emoji_manager
            return get_emoji_manager()
        return self._emoji_manager

    def generate(
        self,
        keywords: List[str],
        used_emojis: Optional[Set[str]] = None
    ) -> Tuple[str, Set[str]]:
        """
        生成页面标题

        格式: 关键词1 + Emoji1 + 关键词2 + Emoji2 + 关键词3

        Args:
            keywords: 关键词列表（至少3个）
            used_emojis: 已使用的Emoji集合（用于避免重复）

        Returns:
            (生成的标题, 更新后的used_emojis集合)

        Raises:
            ValueError: 当关键词数量不足时
        """
        if len(keywords) < self.keywords_count:
            raise ValueError(
                f"需要至少{self.keywords_count}个关键词，"
                f"当前只有{len(keywords)}个"
            )

        used = used_emojis.copy() if used_emojis else set()
        title_parts = []

        # 选择关键词（取前N个或随机选择）
        selected_keywords = keywords[:self.keywords_count]

        # 交替添加关键词和Emoji
        for i, keyword in enumerate(selected_keywords):
            title_parts.append(keyword)

            # 在除最后一个关键词外的每个关键词后添加Emoji
            if i < len(selected_keywords) - 1:
                emoji = self._get_unique_emoji(used)
                used.add(emoji)
                title_parts.append(emoji)

        return ''.join(title_parts), used

    def _get_unique_emoji(self, used: Set[str]) -> str:
        """获取一个未使用过的Emoji"""
        max_attempts = 100
        for _ in range(max_attempts):
            emoji = self.emoji_manager.get_random()
            if emoji not in used:
                return emoji
        # 如果尝试次数过多，返回任意一个
        return self.emoji_manager.get_random()


# 全局生成器实例
_title_generator = TitleGenerator()


def generate_title(
    keywords: List[str],
    used_emojis: Optional[Set[str]] = None
) -> Tuple[str, Set[str]]:
    """
    快捷函数 - 生成页面标题

    Args:
        keywords: 关键词列表（至少3个）
        used_emojis: 已使用的Emoji集合

    Returns:
        (生成的标题, 更新后的used_emojis集合)

    Example:
        >>> title, used = generate_title(["软件下载", "免费工具", "在线服务"])
        >>> print(title)  # "软件下载😀免费工具🔥在线服务"
    """
    return _title_generator.generate(keywords, used_emojis)
