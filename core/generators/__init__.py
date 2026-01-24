# -*- coding: utf-8 -*-
"""
正文生成器模块

提供正文生成器接口定义、动态代码生成器、生成器管理器。
"""

from .interface import (
    GeneratorContext,
    GeneratorResult,
    IAnnotator,
    IContentGenerator
)
from .dynamic import DynamicGenerator
from .manager import GeneratorManager

__all__ = [
    'GeneratorContext',
    'GeneratorResult',
    'IAnnotator',
    'IContentGenerator',
    'DynamicGenerator',
    'GeneratorManager'
]
