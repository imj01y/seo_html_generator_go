# -*- coding: utf-8 -*-
"""
拼音标注器

为汉字添加拼音标注。
"""

import re
from abc import ABC, abstractmethod
from typing import List, Optional
from loguru import logger


class IAnnotator(ABC):
    """拼音标注器接口"""

    @abstractmethod
    def annotate(self, text: str) -> str:
        """对文本添加拼音标注"""
        pass

    def annotate_batch(self, texts: List[str]) -> List[str]:
        """批量标注"""
        return [self.annotate(text) for text in texts]

try:
    from pypinyin import pinyin, Style
except ImportError:
    pinyin = None
    Style = None
    logger.warning("pypinyin not installed, run: pip install pypinyin")


class PinyinAnnotator(IAnnotator):
    """
    拼音标注器

    将汉字转换为带拼音标注的格式：
    "汉字" -> "汉(han)字(zi)"

    特性：
    - 只标注汉字，保留其他字符
    - 支持批量标注
    - 可选择是否标注标点符号
    """

    # 汉字匹配正则
    CHINESE_PATTERN = re.compile(r'[\u4e00-\u9fff]')

    # 中文标点
    CHINESE_PUNCTUATION = '，。！？；：""''（）【】、'

    def __init__(
        self,
        annotate_punctuation: bool = True,
        style: Optional[int] = None
    ):
        """
        初始化拼音标注器

        Args:
            annotate_punctuation: 是否标注中文标点
            style: pypinyin 风格（默认 NORMAL）
        """
        self.annotate_punctuation = annotate_punctuation
        self.style = style or (Style.NORMAL if Style else None)

        if pinyin is None:
            logger.warning("pypinyin not available, annotations will be skipped")

    def annotate(self, text: str) -> str:
        """
        为文本添加拼音标注

        Args:
            text: 原始文本

        Returns:
            带拼音标注的文本，如 "汉(han)字(zi)"
        """
        if not text:
            return ""

        if pinyin is None:
            return text

        result = []
        for char in text:
            result.append(char)

            # 检查是否需要标注
            if self.CHINESE_PATTERN.match(char):
                # 汉字标注
                try:
                    py = pinyin(char, style=self.style, heteronym=False)
                    if py and py[0]:
                        result.append(f"({py[0][0]})")
                except Exception:
                    pass
            elif self.annotate_punctuation and char in self.CHINESE_PUNCTUATION:
                # 中文标点标注（使用符号名称）
                punctuation_names = {
                    '，': 'dou',
                    '。': 'ju',
                    '！': 'tan',
                    '？': 'wen',
                    '；': 'fen',
                    '：': 'mao',
                    '"': 'yin',
                    '"': 'yin',
                    ''': 'yin',
                    ''': 'yin',
                    '（': 'kuo',
                    '）': 'kuo',
                    '【': 'kuo',
                    '】': 'kuo',
                    '、': 'dun',
                }
                if char in punctuation_names:
                    result.append(f"({punctuation_names[char]})")

        return ''.join(result)


# 全局拼音标注器实例
_annotator: Optional[PinyinAnnotator] = None


def get_pinyin_annotator() -> PinyinAnnotator:
    """获取全局拼音标注器实例"""
    global _annotator
    if _annotator is None:
        _annotator = PinyinAnnotator()
    return _annotator


def annotate_pinyin(text: str) -> str:
    """
    便捷函数：添加拼音标注

    Args:
        text: 原始文本

    Returns:
        带拼音标注的文本
    """
    return get_pinyin_annotator().annotate(text)
