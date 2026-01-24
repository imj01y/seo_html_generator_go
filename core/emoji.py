"""
Emoji管理器模块

提供Emoji的管理和随机获取功能。

主要功能:
- get_random_emoji(): 随机获取一个Emoji
- get_random_emojis(count): 获取指定数量的不重复Emoji

特征:
- 从JSON文件加载Emoji库（data/emojis.json）
- 支持使用过的Emoji去重
"""
import json
import random
from pathlib import Path
from typing import List, Set, Optional

from loguru import logger


class EmojiManager:
    """
    Emoji管理器

    管理Emoji库，提供随机获取功能，支持去重。

    Attributes:
        emojis: Emoji列表
        used_emojis: 已使用的Emoji集合（用于去重）

    Example:
        >>> manager = EmojiManager()
        >>> emoji = manager.get_random()
        >>> print(emoji)  # "😀" 或其他随机Emoji
    """

    def __init__(self, emojis: Optional[List[str]] = None):
        """
        初始化Emoji管理器

        Args:
            emojis: Emoji列表，None时为空列表
        """
        self.emojis: List[str] = emojis if emojis is not None else []
        self.used_emojis: Set[str] = set()

    def get_random(self, exclude_used: bool = False) -> str:
        """
        随机获取一个Emoji

        Args:
            exclude_used: 是否排除已使用的Emoji

        Returns:
            随机Emoji，库为空时返回空字符串
        """
        if not self.emojis:
            return ""

        if exclude_used and self.used_emojis:
            available = [e for e in self.emojis if e not in self.used_emojis]
            if not available:
                # 所有Emoji都用过了，重置
                self.reset_used()
                available = self.emojis
        else:
            available = self.emojis

        emoji = random.choice(available)
        self.used_emojis.add(emoji)
        return emoji

    def get_random_emoji(self, exclude: Optional[Set[str]] = None) -> str:
        """
        随机获取一个Emoji（支持排除集合）

        Args:
            exclude: 要排除的Emoji集合

        Returns:
            随机Emoji
        """
        if not self.emojis:
            return ""

        if exclude:
            available = [e for e in self.emojis if e not in exclude]
            if available:
                return random.choice(available)
        return random.choice(self.emojis)

    def get_random_list(self, count: int, unique: bool = True) -> List[str]:
        """
        获取指定数量的随机Emoji

        Args:
            count: 需要的Emoji数量
            unique: 是否不重复

        Returns:
            Emoji列表
        """
        if not self.emojis:
            return []

        if unique:
            # 确保不超过可用数量
            count = min(count, len(self.emojis))
            return random.sample(self.emojis, count)
        else:
            return [self.get_random() for _ in range(count)]

    def reset_used(self) -> None:
        """重置已使用的Emoji集合"""
        self.used_emojis.clear()

    def load_from_file(self, file_path: str) -> int:
        """
        从JSON文件加载Emoji

        JSON格式: ["😀", "😃", ...]

        Args:
            file_path: JSON文件路径

        Returns:
            加载的Emoji数量
        """
        path = Path(file_path)
        if not path.exists():
            logger.warning(f"Emoji file not found: {file_path}")
            return 0

        try:
            with open(path, 'r', encoding='utf-8') as f:
                data = json.load(f)
                if isinstance(data, list):
                    self.emojis = data
                elif isinstance(data, dict) and 'emojis' in data:
                    self.emojis = data['emojis']
                else:
                    logger.warning(f"Invalid emoji file format: {file_path}")
                    return 0

            logger.info(f"Loaded {len(self.emojis)} emojis from {file_path}")
            return len(self.emojis)
        except Exception as e:
            logger.error(f"Failed to load emojis from {file_path}: {e}")
            return 0

    def count(self) -> int:
        """返回Emoji库大小"""
        return len(self.emojis)

    def get_all(self) -> List[str]:
        """返回所有Emoji"""
        return self.emojis.copy()


def _get_default_emoji_file() -> Optional[str]:
    """获取默认的emoji文件路径"""
    # 尝试多个可能的路径
    possible_paths = [
        Path(__file__).parent.parent / "data" / "emojis.json",
        Path(__file__).parent.parent.parent / "data" / "emojis.json",
        Path("./data/emojis.json"),
    ]

    for path in possible_paths:
        if path.exists():
            return str(path)
    return None


def _create_default_manager() -> EmojiManager:
    """创建默认的Emoji管理器（从文件加载）"""
    manager = EmojiManager()
    emoji_file = _get_default_emoji_file()
    if emoji_file:
        manager.load_from_file(emoji_file)
    else:
        logger.error("emojis.json not found! Emoji functionality will be limited.")
    return manager


# 全局Emoji管理器实例
_emoji_manager: Optional[EmojiManager] = None


def get_emoji_manager() -> EmojiManager:
    """获取全局Emoji管理器"""
    global _emoji_manager
    if _emoji_manager is None:
        _emoji_manager = _create_default_manager()
    return _emoji_manager


def get_random_emoji(exclude_used: bool = False) -> str:
    """
    快捷函数 - 获取随机Emoji

    Args:
        exclude_used: 是否排除已使用的

    Returns:
        随机Emoji
    """
    return get_emoji_manager().get_random(exclude_used)


def get_random_emojis(count: int, unique: bool = True) -> List[str]:
    """
    快捷函数 - 获取多个随机Emoji

    Args:
        count: 数量
        unique: 是否不重复

    Returns:
        Emoji列表
    """
    return get_emoji_manager().get_random_list(count, unique)


def reset_emoji_usage() -> None:
    """快捷函数 - 重置Emoji使用记录"""
    get_emoji_manager().reset_used()


def load_emojis_from_file(file_path: str) -> int:
    """
    从文件加载Emoji到全局管理器

    Args:
        file_path: 文件路径

    Returns:
        加载的Emoji数量
    """
    return get_emoji_manager().load_from_file(file_path)


def create_emoji_manager(emojis: Optional[List[str]] = None) -> EmojiManager:
    """
    创建新的Emoji管理器实例

    Args:
        emojis: Emoji列表

    Returns:
        EmojiManager实例
    """
    return EmojiManager(emojis)


def init_emoji_manager(file_path: Optional[str] = None) -> EmojiManager:
    """
    初始化全局Emoji管理器

    Args:
        file_path: Emoji文件路径，None时自动查找

    Returns:
        EmojiManager实例
    """
    global _emoji_manager

    _emoji_manager = EmojiManager()
    if file_path:
        _emoji_manager.load_from_file(file_path)
    else:
        default_file = _get_default_emoji_file()
        if default_file:
            _emoji_manager.load_from_file(default_file)

    return _emoji_manager
