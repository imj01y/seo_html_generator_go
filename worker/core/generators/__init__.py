# -*- coding: utf-8 -*-
"""
生成器接口定义

仅保留 IAnnotator 接口供 pinyin_annotator 使用。
"""

from .interface import (
    IAnnotator,
    GeneratorContext,
    GeneratorResult,
    IContentGenerator
)

__all__ = [
    'IAnnotator',
    'GeneratorContext',
    'GeneratorResult',
    'IContentGenerator'
]
