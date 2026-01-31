# -*- coding: utf-8 -*-
"""
爬虫项目运行器

执行用户编写的 Spider 类爬虫代码（feapder 风格）。
只支持 Spider 类模式，不支持旧的函数式模式。
"""

import types
import asyncio
import time
from typing import Any, AsyncGenerator, Dict, List, Optional, Tuple, TYPE_CHECKING
from loguru import logger

if TYPE_CHECKING:
    from redis.asyncio import Redis
    from aiomysql import Pool
    from core.workers.logger_protocol import LoggerProtocol

from .spider import Spider
from .request import Request


class ProjectRunner:
    """
    项目运行器 - 只支持 Spider 类模式

    执行继承 Spider 的爬虫类，收集 yield 的数据。
    """

    def __init__(
        self,
        project_id: int,
        modules: Dict[str, types.ModuleType],
        config: Optional[Dict[str, Any]] = None,
        redis: Optional['Redis'] = None,
        db_pool: Optional['Pool'] = None,
        concurrency: int = 3,
        is_test: bool = False,
        max_items: int = 0,
        log: Optional['LoggerProtocol'] = None,
    ):
        """
        初始化运行器

        Args:
            project_id: 项目ID
            modules: 已加载的模块字典
            config: 运行配置
            redis: Redis 客户端（用于队列模式）
            db_pool: MySQL 连接池（用于保存失败请求）
            concurrency: 并发数
            is_test: 是否为测试模式（使用独立的 Redis 队列）
            max_items: 最大数据条数（0 表示不限制）
        """
        self.project_id = project_id
        self.modules = modules
        self.config = config or {}
        self.redis = redis
        self.db_pool = db_pool
        self.concurrency = concurrency
        self.is_test = is_test
        self.max_items = max_items
        self._stop_flag = False
        self._spider_instance: Optional[Spider] = None
        self.log = log

    def _get_spider_class(self) -> type:
        """
        查找模块中的 Spider 子类

        Returns:
            Spider 子类

        Raises:
            ValueError: 未找到 Spider 类
        """
        for module_name, module in self.modules.items():
            for name in dir(module):
                obj = getattr(module, name)
                if (isinstance(obj, type)
                    and issubclass(obj, Spider)
                    and obj is not Spider):
                    return obj

        raise ValueError("未找到 Spider 类，请定义一个继承 Spider 的类")

    def stop(self):
        """设置停止标志"""
        self._stop_flag = True

    def _check_stop(self) -> bool:
        """检查是否应该停止"""
        return self._stop_flag

    def _apply_settings(self, settings: Dict[str, Any]):
        """
        应用自定义配置

        Args:
            settings: 配置字典
        """
        if 'CONCURRENT_REQUESTS' in settings:
            self.concurrency = settings['CONCURRENT_REQUESTS']
        # 可扩展其他配置...

    async def run(self) -> AsyncGenerator[Dict[str, Any], None]:
        """
        运行项目

        Yields:
            yield 的字典数据
        """
        from .request_queue import RequestQueue
        from .queue_consumer import QueueConsumer
        from .failed_manager import FailedRequestManager

        logger.info(f"Starting project {self.project_id}")

        # 获取 Spider 类
        spider_cls = self._get_spider_class()

        # 实例化 Spider
        redis_key = f"project:{self.project_id}:spider"
        spider = spider_cls(redis_key=redis_key)
        self._spider_instance = spider

        # 应用自定义配置
        if hasattr(spider, '__custom_setting__') and spider.__custom_setting__:
            self._apply_settings(spider.__custom_setting__)

        spider_name = spider.name or spider_cls.__name__
        logger.info(f"Spider '{spider_name}' starting")
        if self.log:
            await self.log.info(f"Spider '{spider_name}' 启动，并发数: {self.concurrency}")

        # 初始化队列
        queue = RequestQueue(self.redis, self.project_id, is_test=self.is_test)
        failed_manager = FailedRequestManager(self.db_pool) if self.db_pool else None

        try:
            # 保存生成器（由消费器按需获取请求）
            start_requests_iter = iter(spider.start_requests())

            # 创建消费器（传入生成器，由消费器按需补充请求）
            consumer = QueueConsumer(
                redis=self.redis,
                project_id=self.project_id,
                spider=spider,
                concurrency=self.concurrency,
                stop_callback=self._check_stop,
                is_test=self.is_test,
                max_items=self.max_items,
                start_requests_iter=start_requests_iter,
                log=self.log,
            )

            logger.info(f"Starting queue consumer with {self.concurrency} workers")

            # 消费器负责按需从生成器获取请求并入队
            async for item in consumer.run():
                if isinstance(item, dict):
                    # 检查是否是失败请求通知
                    if item.get('_type') == 'failed':
                        # 保存失败请求
                        if failed_manager:
                            try:
                                await failed_manager.save(
                                    project_id=self.project_id,
                                    request=item['request'],
                                    error_message=item.get('error', 'Unknown error'),
                                )
                            except Exception as e:
                                logger.error(f"Failed to save failed request: {e}")
                    else:
                        # 正常数据
                        yield item

                await asyncio.sleep(0)

        except asyncio.CancelledError:
            # 任务被取消，向上传播
            logger.info(f"Project {self.project_id} runner cancelled")
            spider.close("cancelled")
            raise

        # 关闭回调
        spider.close("finished")
        logger.info(f"Spider '{spider_name}' finished")

    async def test_run(self, max_items: int = 10) -> Dict[str, Any]:
        """
        测试运行，只获取前几条数据

        Args:
            max_items: 最大数据条数

        Returns:
            测试结果 {success, items, logs, duration, error}
        """
        start_time = time.time()
        items = []
        logs = []
        error = None

        # 创建日志捕获器
        log_handler_id = logger.add(
            lambda msg: logs.append({
                "time": msg.record["time"].strftime("%H:%M:%S"),
                "level": msg.record["level"].name,
                "message": msg.record["message"],
            }),
            format="{message}",
            level="DEBUG",
        )

        try:
            count = 0
            async for item in self.run():
                items.append(item)
                count += 1
                if count >= max_items:
                    break

        except Exception as e:
            error = str(e)
            logger.error(f"Test run error: {e}")

        finally:
            # 移除日志捕获器
            logger.remove(log_handler_id)

        return {
            "success": error is None,
            "items": items,
            "logs": logs,
            "duration": round(time.time() - start_time, 3),
            "error": error,
        }


class ProjectCodeValidator:
    """
    项目代码验证器 - 只验证 Spider 类
    """

    def __init__(self, code: str):
        """
        初始化验证器

        Args:
            code: Python 代码
        """
        self.code = code

    def validate(self) -> Tuple[bool, str]:
        """
        验证代码

        必须定义继承 Spider 的类，并实现 start_requests 和 parse 方法。

        Returns:
            (is_valid, error_message)
        """
        # 检查是否有 Spider 子类
        if not ('class ' in self.code and '(Spider)' in self.code):
            return False, "必须定义一个继承 Spider 的类"

        # 检查 start_requests 方法
        if 'def start_requests(self' not in self.code:
            return False, "Spider 类必须实现 start_requests(self) 方法"

        # 检查 parse 方法
        if 'def parse(self' not in self.code:
            return False, "Spider 类必须实现 parse(self, request, response) 方法"

        # 检查语法
        try:
            compile(self.code, '<validation>', 'exec')
        except SyntaxError as e:
            return False, f"语法错误: 第 {e.lineno} 行: {e.msg}"

        return True, ""

    def get_defined_functions(self) -> List[str]:
        """
        获取代码中定义的所有函数名

        Returns:
            函数名列表
        """
        import ast

        try:
            tree = ast.parse(self.code)
            return [
                node.name for node in ast.walk(tree)
                if isinstance(node, ast.FunctionDef)
            ]
        except SyntaxError:
            return []

    def get_spider_class_name(self) -> Optional[str]:
        """
        获取 Spider 子类的名称

        Returns:
            类名，未找到返回 None
        """
        import ast

        try:
            tree = ast.parse(self.code)
            for node in ast.walk(tree):
                if isinstance(node, ast.ClassDef):
                    for base in node.bases:
                        if isinstance(base, ast.Name) and base.id == 'Spider':
                            return node.name
        except SyntaxError:
            pass
        return None
