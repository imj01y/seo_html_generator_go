# -*- coding: utf-8 -*-
"""
正文生成器接口定义

所有生成器必须实现 IContentGenerator 接口。
"""

from abc import ABC, abstractmethod
from typing import List, Optional
from dataclasses import dataclass, field


@dataclass
class GeneratorContext:
    """
    生成器输入上下文

    Attributes:
        paragraphs: 段落列表（从段落队列取出）
        titles: 标题列表（从 titles 表随机抽取，用于正文开头填充）
        group_id: 当前分组ID
        extra: 额外参数（扩展用）
    """
    paragraphs: List[str]
    titles: List[str] = field(default_factory=list)
    group_id: int = 1
    extra: dict = field(default_factory=dict)

    def __post_init__(self):
        """确保列表不为 None"""
        if self.paragraphs is None:
            self.paragraphs = []
        if self.titles is None:
            self.titles = []
        if self.extra is None:
            self.extra = {}


@dataclass
class GeneratorResult:
    """
    生成器输出结果

    Attributes:
        content: 生成的正文内容（已含拼音标注）
        success: 是否成功
        message: 错误信息（失败时）
        metadata: 额外元数据
    """
    content: str
    success: bool = True
    message: str = ""
    metadata: dict = field(default_factory=dict)

    @classmethod
    def error(cls, message: str) -> 'GeneratorResult':
        """创建错误结果的便捷方法"""
        return cls(content="", success=False, message=message)

    @classmethod
    def ok(cls, content: str, **metadata) -> 'GeneratorResult':
        """创建成功结果的便捷方法"""
        return cls(content=content, success=True, metadata=metadata)


class IAnnotator(ABC):
    """
    拼音标注器接口

    所有拼音标注器必须实现此接口。
    """

    @abstractmethod
    def annotate(self, text: str) -> str:
        """
        对文本添加拼音标注

        Args:
            text: 原始文本

        Returns:
            带拼音标注的文本，如 "汉(han)字(zi)"
        """
        pass

    def annotate_batch(self, texts: List[str]) -> List[str]:
        """
        批量标注（默认实现，子类可重写优化）

        Args:
            texts: 文本列表

        Returns:
            标注后的文本列表
        """
        return [self.annotate(text) for text in texts]


class IContentGenerator(ABC):
    """
    正文生成器接口

    所有生成器必须实现此接口。
    支持同步和异步两种生成方式。
    """

    @property
    @abstractmethod
    def name(self) -> str:
        """
        生成器唯一标识

        用于在管理器中注册和查找。
        """
        pass

    @property
    @abstractmethod
    def description(self) -> str:
        """
        生成器描述

        用于在 UI 中显示。
        """
        pass

    @abstractmethod
    async def generate(self, ctx: GeneratorContext) -> GeneratorResult:
        """
        生成正文（异步）

        Args:
            ctx: 生成器上下文，包含段落、标题等信息

        Returns:
            GeneratorResult 包含生成的正文或错误信息
        """
        pass

    def generate_sync(self, ctx: GeneratorContext) -> GeneratorResult:
        """
        生成正文（同步，可选实现）

        默认抛出未实现异常，子类可重写。
        """
        raise NotImplementedError("Sync generation not supported")

    def validate_context(self, ctx: GeneratorContext) -> bool:
        """
        验证上下文是否有效

        Args:
            ctx: 生成器上下文

        Returns:
            True=有效，False=无效
        """
        return len(ctx.paragraphs) > 0

    def on_load(self) -> None:
        """
        加载回调

        生成器被注册时调用，可用于初始化资源。
        """
        pass

    def on_unload(self) -> None:
        """
        卸载回调

        生成器被移除时调用，可用于清理资源。
        """
        pass

    def get_required_paragraphs(self) -> int:
        """
        获取所需的最少段落数量

        Returns:
            最少段落数量，默认 1
        """
        return 1

    def get_required_titles(self) -> int:
        """
        获取所需的最少标题数量

        Returns:
            最少标题数量，默认 0
        """
        return 0
