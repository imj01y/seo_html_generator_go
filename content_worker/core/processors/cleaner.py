# -*- coding: utf-8 -*-
"""
文本清洗器

提供文本清洗功能，去除广告、特殊字符、HTML标签等。
"""

import re
import html
from typing import List, Optional


class TextCleaner:
    """
    文本清洗器

    功能：
    - 去除HTML标签
    - 去除特殊字符
    - 去除多余空白
    - 过滤广告文本
    - 过滤短文本
    """

    # 广告关键词（匹配到则过滤整段）
    AD_KEYWORDS = [
        '广告', '推广', '点击查看', '立即购买', '免费领取',
        '加微信', '加QQ', '扫码', '二维码', '客服电话',
        '版权所有', 'Copyright', '备案号', 'ICP备',
        '联系我们', '关于我们', '友情链接', '网站地图'
    ]

    # 需要清除的HTML标签
    HTML_TAG_PATTERN = re.compile(r'<[^>]+>')

    # 特殊字符
    SPECIAL_CHARS_PATTERN = re.compile(r'[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]')

    # 多个空白字符
    MULTI_SPACE_PATTERN = re.compile(r'[ \t]+')
    MULTI_NEWLINE_PATTERN = re.compile(r'\n{3,}')

    def __init__(
        self,
        min_length: int = 10,
        max_length: int = 5000,
        remove_html: bool = True,
        filter_ads: bool = True,
        ad_keywords: Optional[List[str]] = None
    ):
        """
        初始化文本清洗器

        Args:
            min_length: 最小文本长度（过滤短文本）
            max_length: 最大文本长度（截断长文本）
            remove_html: 是否去除HTML标签
            filter_ads: 是否过滤广告文本
            ad_keywords: 自定义广告关键词列表
        """
        self.min_length = min_length
        self.max_length = max_length
        self.remove_html = remove_html
        self.filter_ads = filter_ads
        self.ad_keywords = ad_keywords or self.AD_KEYWORDS

    def clean(self, text: str) -> str:
        """
        清洗单个文本

        Args:
            text: 原始文本

        Returns:
            清洗后的文本
        """
        if not text:
            return ""

        # 1. HTML实体解码
        text = html.unescape(text)

        # 2. 去除HTML标签
        if self.remove_html:
            text = self.HTML_TAG_PATTERN.sub('', text)

        # 3. 去除特殊字符
        text = self.SPECIAL_CHARS_PATTERN.sub('', text)

        # 4. 规范化空白
        text = self.MULTI_SPACE_PATTERN.sub(' ', text)
        text = self.MULTI_NEWLINE_PATTERN.sub('\n\n', text)

        # 5. 去除首尾空白
        text = text.strip()

        # 6. 截断长文本
        if self.max_length and len(text) > self.max_length:
            text = text[:self.max_length]

        return text

    def clean_paragraph(self, text: str) -> Optional[str]:
        """
        清洗段落文本

        会额外检查长度和广告关键词。

        Args:
            text: 段落文本

        Returns:
            清洗后的文本，或 None（被过滤）
        """
        text = self.clean(text)

        # 检查长度
        if len(text) < self.min_length:
            return None

        # 检查广告关键词
        if self.filter_ads and self._contains_ad(text):
            return None

        return text

    def clean_paragraphs(self, texts: List[str]) -> List[str]:
        """
        批量清洗段落

        Args:
            texts: 段落列表

        Returns:
            清洗后的段落列表（已过滤无效段落）
        """
        return [
            cleaned for text in texts
            if (cleaned := self.clean_paragraph(text))
        ]

    def _contains_ad(self, text: str) -> bool:
        """检查是否包含广告关键词"""
        text_lower = text.lower()
        return any(keyword.lower() in text_lower for keyword in self.ad_keywords)
