# -*- coding: utf-8 -*-
"""
生成器管理器

管理所有实现 IContentGenerator 接口的生成器。
支持从数据库加载动态生成器。
"""

from typing import Dict, List, Optional
from loguru import logger

from .interface import (
    IContentGenerator,
    IAnnotator,
    GeneratorContext,
    GeneratorResult
)
from .dynamic import DynamicGenerator, DEFAULT_GENERATOR_CODE


class GeneratorManager:
    """
    生成器管理器

    功能：
    - 注册/注销生成器
    - 从数据库加载动态生成器
    - 切换当前使用的生成器
    - 重新加载单个生成器（热更新）
    """

    def __init__(self, db_pool=None, annotator: Optional[IAnnotator] = None):
        """
        初始化生成器管理器

        Args:
            db_pool: MySQL 连接池（用于加载动态生成器）
            annotator: 拼音标注器实例
        """
        self._generators: Dict[str, IContentGenerator] = {}
        self._current: Optional[str] = None
        self._db_pool = db_pool
        self._annotator = annotator

    async def load_from_db(self) -> int:
        """
        从数据库加载所有启用的生成器

        Returns:
            加载的生成器数量
        """
        if not self._db_pool:
            logger.warning("No database pool, skip loading generators from DB")
            return 0

        try:
            async with self._db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        """
                        SELECT name, display_name, description, code, is_default, version
                        FROM content_generators
                        WHERE enabled = 1
                        ORDER BY is_default DESC, id ASC
                        """
                    )
                    rows = await cursor.fetchall()

            count = 0
            for row in rows:
                name, display_name, description, code, is_default, version = row

                # 创建动态生成器
                generator = DynamicGenerator(
                    name=name,
                    description=display_name or description or name,
                    code=code,
                    annotator=self._annotator,
                    version=version or 1
                )

                # 注册生成器
                self.register(generator)
                count += 1

                # 设置默认生成器
                if is_default and self._current is None:
                    self._current = name

            logger.info(f"Loaded {count} generators from database")
            return count

        except Exception as e:
            logger.error(f"Failed to load generators from database: {e}")
            return 0

    async def reload_one(self, name: str) -> bool:
        """
        重新加载单个生成器（从数据库读取最新代码）

        Args:
            name: 生成器名称

        Returns:
            是否成功
        """
        if not self._db_pool:
            return False

        try:
            async with self._db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        """
                        SELECT display_name, description, code, version
                        FROM content_generators
                        WHERE name = %s AND enabled = 1
                        """,
                        (name,)
                    )
                    row = await cursor.fetchone()

            if not row:
                logger.warning(f"Generator '{name}' not found in database")
                return False

            display_name, description, code, version = row

            # 如果已存在，更新代码
            if name in self._generators:
                existing = self._generators[name]
                if isinstance(existing, DynamicGenerator):
                    existing.recompile(code, version)
                    logger.info(f"Generator '{name}' recompiled (v{version})")
                    return True

            # 否则创建新的
            generator = DynamicGenerator(
                name=name,
                description=display_name or description or name,
                code=code,
                annotator=self._annotator,
                version=version or 1
            )
            self.register(generator)
            return True

        except Exception as e:
            logger.error(f"Failed to reload generator '{name}': {e}")
            return False

    def register(self, generator: IContentGenerator) -> None:
        """
        注册生成器

        Args:
            generator: 生成器实例
        """
        name = generator.name

        # 如果已存在，先卸载
        if name in self._generators:
            self._generators[name].on_unload()

        # 注册新生成器
        generator.on_load()
        self._generators[name] = generator

        # 如果是第一个，设为默认
        if self._current is None:
            self._current = name

        logger.debug(f"Registered generator: {name}")

    def unregister(self, name: str) -> bool:
        """
        注销生成器

        Args:
            name: 生成器名称

        Returns:
            是否成功
        """
        if name not in self._generators:
            return False

        generator = self._generators.pop(name)
        generator.on_unload()

        # 如果注销的是当前生成器，切换到其他
        if self._current == name:
            self._current = next(iter(self._generators), None)

        logger.debug(f"Unregistered generator: {name}")
        return True

    def get(self, name: str) -> Optional[IContentGenerator]:
        """获取指定生成器"""
        return self._generators.get(name)

    def get_current(self) -> Optional[IContentGenerator]:
        """获取当前生成器"""
        if self._current:
            return self._generators.get(self._current)
        return None

    def set_current(self, name: str) -> bool:
        """
        设置当前使用的生成器

        Args:
            name: 生成器名称

        Returns:
            是否成功
        """
        if name not in self._generators:
            logger.warning(f"Generator '{name}' not found")
            return False

        self._current = name
        logger.info(f"Current generator set to: {name}")
        return True

    def list_all(self) -> List[dict]:
        """
        列出所有生成器

        Returns:
            生成器信息列表
        """
        result = []
        for name, gen in self._generators.items():
            info = {
                'name': name,
                'description': gen.description,
                'is_current': name == self._current
            }
            if isinstance(gen, DynamicGenerator):
                info['version'] = gen.version
                info['is_compiled'] = gen.is_compiled
                info['compile_error'] = gen.compile_error
            result.append(info)
        return result

    async def generate(self, ctx: GeneratorContext) -> GeneratorResult:
        """
        使用当前生成器生成正文

        Args:
            ctx: 生成器上下文

        Returns:
            GeneratorResult
        """
        if not self._current:
            return GeneratorResult.error("未设置生成器")

        generator = self._generators.get(self._current)
        if not generator:
            return GeneratorResult.error(f"生成器 '{self._current}' 不存在")

        # 验证上下文
        if not generator.validate_context(ctx):
            return GeneratorResult.error("上下文验证失败（段落不足）")

        return await generator.generate(ctx)

    async def generate_with(self, name: str, ctx: GeneratorContext) -> GeneratorResult:
        """
        使用指定生成器生成正文

        Args:
            name: 生成器名称
            ctx: 生成器上下文

        Returns:
            GeneratorResult
        """
        generator = self._generators.get(name)
        if not generator:
            return GeneratorResult.error(f"生成器 '{name}' 不存在")

        if not generator.validate_context(ctx):
            return GeneratorResult.error("上下文验证失败（段落不足）")

        return await generator.generate(ctx)

    def get_stats(self) -> dict:
        """获取统计信息"""
        return {
            'total': len(self._generators),
            'current': self._current,
            'generators': list(self._generators.keys())
        }


# 全局生成器管理器实例
_generator_manager: Optional[GeneratorManager] = None


async def init_generator_manager(
    db_pool=None,
    annotator: Optional[IAnnotator] = None
) -> GeneratorManager:
    """
    初始化全局生成器管理器

    Args:
        db_pool: MySQL 连接池
        annotator: 拼音标注器

    Returns:
        GeneratorManager 实例
    """
    global _generator_manager
    _generator_manager = GeneratorManager(db_pool, annotator)
    await _generator_manager.load_from_db()
    return _generator_manager


def get_generator_manager() -> Optional[GeneratorManager]:
    """获取全局生成器管理器实例"""
    return _generator_manager
