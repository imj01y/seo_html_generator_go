# -*- coding: utf-8 -*-
"""
动态代码生成器

从数据库加载代码，实现 IContentGenerator 接口。
支持在线编辑，保存后立即生效。
"""

import asyncio
from typing import Optional, Callable, Any, Dict
from loguru import logger

from .interface import (
    IContentGenerator,
    IAnnotator,
    GeneratorContext,
    GeneratorResult
)


class DynamicGenerator(IContentGenerator):
    """
    从数据库加载的动态生成器

    特性：
    - 代码存储在数据库中
    - 支持在线编辑
    - 保存后重新编译即可生效
    - 沙箱执行，限制可用模块
    """

    # 允许在生成器代码中使用的模块
    ALLOWED_MODULES = {
        'random': __import__('random'),
        're': __import__('re'),
        'datetime': __import__('datetime'),
        'json': __import__('json'),
        'html': __import__('html'),
    }

    def __init__(self, name: str, description: str, code: str,
                 annotator: Optional[IAnnotator] = None,
                 version: int = 1):
        """
        初始化动态生成器

        Args:
            name: 生成器唯一标识
            description: 生成器描述
            code: Python 代码（必须包含 generate 函数）
            annotator: 拼音标注器实例
            version: 代码版本号
        """
        self._name = name
        self._description = description
        self._code = code
        self._annotator = annotator
        self._version = version
        self._func: Optional[Callable] = None
        self._is_async: bool = False
        self._compile_error: Optional[str] = None

        self._compile()

    @property
    def name(self) -> str:
        return self._name

    @property
    def description(self) -> str:
        return self._description

    @property
    def version(self) -> int:
        return self._version

    @property
    def code(self) -> str:
        return self._code

    @property
    def is_compiled(self) -> bool:
        """是否编译成功"""
        return self._func is not None

    @property
    def compile_error(self) -> Optional[str]:
        """编译错误信息"""
        return self._compile_error

    def _create_safe_globals(self) -> Dict[str, Any]:
        """创建安全的全局命名空间"""
        safe_globals = {
            '__builtins__': {
                # 安全的内置函数
                'len': len,
                'str': str,
                'int': int,
                'float': float,
                'bool': bool,
                'list': list,
                'dict': dict,
                'set': set,
                'tuple': tuple,
                'range': range,
                'enumerate': enumerate,
                'zip': zip,
                'map': map,
                'filter': filter,
                'sorted': sorted,
                'reversed': reversed,
                'min': min,
                'max': max,
                'sum': sum,
                'any': any,
                'all': all,
                'abs': abs,
                'round': round,
                'isinstance': isinstance,
                'hasattr': hasattr,
                'getattr': getattr,
                'setattr': setattr,
                'print': logger.debug,  # 重定向 print 到日志
                'None': None,
                'True': True,
                'False': False,
            }
        }

        # 添加允许的模块
        safe_globals.update(self.ALLOWED_MODULES)

        # 添加拼音标注函数
        if self._annotator:
            safe_globals['annotate_pinyin'] = self._annotator.annotate
        else:
            safe_globals['annotate_pinyin'] = lambda x: x  # 无标注器时返回原文

        return safe_globals

    def _compile(self):
        """编译代码为可执行函数"""
        self._func = None
        self._compile_error = None

        if not self._code or not self._code.strip():
            self._compile_error = "代码为空"
            return

        try:
            safe_globals = self._create_safe_globals()
            local_vars = {}

            # 执行代码，提取 generate 函数
            exec(self._code, safe_globals, local_vars)

            func = local_vars.get('generate')
            if func is None:
                self._compile_error = "代码中未定义 generate 函数"
                return

            if not callable(func):
                self._compile_error = "generate 不是可调用对象"
                return

            self._func = func
            self._is_async = asyncio.iscoroutinefunction(func)

            logger.debug(f"Generator '{self._name}' compiled successfully (async={self._is_async})")

        except SyntaxError as e:
            self._compile_error = f"语法错误: {e}"
            logger.error(f"Generator '{self._name}' compile failed: {self._compile_error}")
        except Exception as e:
            self._compile_error = f"编译错误: {e}"
            logger.error(f"Generator '{self._name}' compile failed: {self._compile_error}")

    def recompile(self, code: str, version: int = None):
        """
        重新编译代码

        Args:
            code: 新代码
            version: 新版本号
        """
        self._code = code
        if version is not None:
            self._version = version
        self._compile()

    async def generate(self, ctx: GeneratorContext) -> GeneratorResult:
        """生成正文"""
        if not self._func:
            error_msg = self._compile_error or "生成器未编译"
            return GeneratorResult.error(error_msg)

        try:
            if self._is_async:
                content = await self._func(ctx)
            else:
                # 在线程池中执行同步函数，避免阻塞
                loop = asyncio.get_event_loop()
                content = await loop.run_in_executor(None, self._func, ctx)

            if content is None:
                return GeneratorResult.error("生成结果为空")

            if not isinstance(content, str):
                content = str(content)

            return GeneratorResult.ok(
                content=content,
                generator=self._name,
                version=self._version
            )

        except Exception as e:
            error_msg = f"生成失败: {e}"
            logger.error(f"Generator '{self._name}' error: {e}")
            return GeneratorResult.error(error_msg)

    def generate_sync(self, ctx: GeneratorContext) -> GeneratorResult:
        """同步生成（仅支持同步函数）"""
        if not self._func:
            error_msg = self._compile_error or "生成器未编译"
            return GeneratorResult.error(error_msg)

        if self._is_async:
            return GeneratorResult.error("此生成器仅支持异步调用")

        try:
            content = self._func(ctx)

            if content is None:
                return GeneratorResult.error("生成结果为空")

            if not isinstance(content, str):
                content = str(content)

            return GeneratorResult.ok(
                content=content,
                generator=self._name,
                version=self._version
            )

        except Exception as e:
            error_msg = f"生成失败: {e}"
            logger.error(f"Generator '{self._name}' error: {e}")
            return GeneratorResult.error(error_msg)

    def on_load(self) -> None:
        """加载回调"""
        logger.info(f"Dynamic generator '{self._name}' v{self._version} loaded")

    def on_unload(self) -> None:
        """卸载回调"""
        logger.info(f"Dynamic generator '{self._name}' unloaded")
        self._func = None


# 默认生成器代码模板
DEFAULT_GENERATOR_CODE = '''
async def generate(ctx):
    """
    正文生成器

    可用变量:
      ctx.paragraphs - 段落列表
      ctx.titles - 标题列表（用于正文开头）
      ctx.group_id - 分组ID

    可用函数:
      annotate_pinyin(text) - 添加拼音标注
      random - Python random 模块
      re - Python re 模块

    返回:
      生成的正文内容（字符串），返回 None 表示生成失败
    """
    # 检查段落数量
    if len(ctx.paragraphs) < 3:
        return None

    parts = []

    # 添加随机标题（正文开头）
    if ctx.titles:
        header_titles = ctx.titles[:3]
        parts.extend(header_titles)
        parts.append("")  # 空行

    # 添加段落
    for para in ctx.paragraphs[:3]:
        # 对段落添加拼音标注
        annotated = annotate_pinyin(para)
        parts.append(annotated)

    # 组合正文
    content = "\\n\\n".join(parts)
    return content
'''
