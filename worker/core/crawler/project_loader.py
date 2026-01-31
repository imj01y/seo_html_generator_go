# -*- coding: utf-8 -*-
"""
爬虫项目加载器

支持多文件项目加载，将数据库中的代码文件编译为可导入的模块。
"""

import sys
import types
import importlib.util
from typing import Dict, Any, Optional
from loguru import logger


class ProjectLoader:
    """
    项目加载器

    从数据库加载项目文件，编译为虚拟模块，支持跨文件导入。
    """

    def __init__(self, project_id: int):
        """
        初始化加载器

        Args:
            project_id: 项目ID
        """
        self.project_id = project_id
        self._modules: Dict[str, types.ModuleType] = {}

    async def load(self) -> Dict[str, types.ModuleType]:
        """
        加载项目所有文件

        Returns:
            模块字典 {module_name: module}
        """
        files = await self._load_files_from_db()

        if not files:
            raise ValueError(f"项目 {self.project_id} 没有文件")

        # 创建项目虚拟包
        package_name = f"spider_project_{self.project_id}"
        package = types.ModuleType(package_name)
        package.__path__ = []
        package.__package__ = package_name
        sys.modules[package_name] = package

        # 按依赖顺序编译模块（简单实现：先编译非入口文件）
        for file in files:
            filepath = file['path']  # 路径格式如 /spider.py
            content = file['content']
            filename = filepath.lstrip('/')  # 去掉前导斜杠
            module_name = filename.replace('.py', '')
            full_module_name = f"{package_name}.{module_name}"

            try:
                module = self._compile_module(full_module_name, content, package_name)
                self._modules[module_name] = module

                # 将模块添加到包中
                setattr(package, module_name, module)
                sys.modules[full_module_name] = module

                logger.debug(f"Compiled module: {full_module_name}")
            except Exception as e:
                logger.error(f"Failed to compile {filename}: {e}")
                raise ValueError(f"编译 {filename} 失败: {e}")

        return self._modules

    async def _load_files_from_db(self) -> list:
        """从数据库加载项目文件"""
        from database.db import fetch_all

        sql = """
            SELECT path, content
            FROM spider_project_files
            WHERE project_id = %s AND type = 'file'
            ORDER BY path
        """
        return await fetch_all(sql, (self.project_id,))

    def _compile_module(
        self,
        full_name: str,
        code: str,
        package_name: str
    ) -> types.ModuleType:
        """
        编译代码为模块

        Args:
            full_name: 完整模块名 (如 spider_project_1.utils)
            code: 源代码
            package_name: 包名

        Returns:
            编译后的模块
        """
        # 创建模块对象
        module = types.ModuleType(full_name)
        module.__file__ = f"<{full_name}>"
        module.__package__ = package_name
        module.__loader__ = None

        # 设置内置模块和常用导入
        module.__builtins__ = __builtins__

        # 预导入常用模块，让用户代码可以直接使用
        common_imports = {
            'json': __import__('json'),
            're': __import__('re'),
            'time': __import__('time'),
            'datetime': __import__('datetime'),
            'random': __import__('random'),
            'os': __import__('os'),
        }

        # 尝试导入可选的常用库
        try:
            common_imports['requests'] = __import__('requests')
        except ImportError:
            pass

        try:
            common_imports['httpx'] = __import__('httpx')
        except ImportError:
            pass

        try:
            from parsel import Selector
            common_imports['Selector'] = Selector
        except ImportError:
            pass

        try:
            from loguru import logger as loguru_logger
            common_imports['logger'] = loguru_logger
        except ImportError:
            pass

        # 导入 Request 和 Response 类（用于队列模式）
        try:
            from .request import Request
            from .response import Response
            common_imports['Request'] = Request
            common_imports['Response'] = Response
        except ImportError:
            pass

        # 将常用模块添加到模块命名空间
        module.__dict__.update(common_imports)

        # 自定义导入钩子：支持从同项目其他模块导入
        original_import = __builtins__['__import__']

        def custom_import(name, globals=None, locals=None, fromlist=(), level=0):
            # 处理相对导入 (level > 0) 和项目内绝对导入
            if level > 0 or name.startswith('.'):
                # 相对导入：from .utils import xxx
                actual_name = name.lstrip('.')
                if actual_name in self._modules:
                    return self._modules[actual_name]

            # 检查是否是项目内模块
            if name in self._modules:
                return self._modules[name]

            # 其他情况使用原始导入
            return original_import(name, globals, locals, fromlist, level)

        module.__dict__['__import__'] = custom_import

        # 编译并执行代码
        try:
            compiled = compile(code, f"<{full_name}>", 'exec')
            exec(compiled, module.__dict__)
        except SyntaxError as e:
            raise ValueError(f"语法错误 第 {e.lineno} 行: {e.msg}")

        return module

    def get_module(self, name: str) -> Optional[types.ModuleType]:
        """获取指定名称的模块"""
        return self._modules.get(name)

    def cleanup(self):
        """清理加载的模块"""
        package_name = f"spider_project_{self.project_id}"

        # 从 sys.modules 中移除
        for module_name in list(self._modules.keys()):
            full_name = f"{package_name}.{module_name}"
            sys.modules.pop(full_name, None)

        sys.modules.pop(package_name, None)
        self._modules.clear()
