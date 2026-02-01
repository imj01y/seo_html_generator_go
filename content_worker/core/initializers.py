# -*- coding: utf-8 -*-
"""
组件初始化模块

提供所有系统组件的初始化逻辑，包括数据库、Redis、缓存池等。
"""
import asyncio
from pathlib import Path
from typing import Optional

from loguru import logger

from config import get_config
from database.db import init_database, init_db_pool, get_db_pool
from core.redis_client import init_redis_client, get_redis_client
from core.spider_detector import init_spider_detector
from core.auth import ensure_default_admin
from core.title_manager import init_title_manager, get_title_manager
from core.content_manager import init_content_manager, get_content_manager


# 全局变量：worker 引用（用于清理）
_generator_worker = None
_scheduler_worker = None


def get_generator_worker():
    """获取生成器 worker 引用"""
    return _generator_worker


async def _start_background_worker(worker_class, name: str, **kwargs):
    """启动后台 worker 的通用函数"""
    global _generator_worker

    try:
        worker = worker_class(**kwargs)

        if name == 'generator':
            _generator_worker = worker
            await worker.run_forever(group_id=1)

    except asyncio.CancelledError:
        logger.info(f"{name.capitalize()} worker cancelled")
    except Exception as e:
        logger.error(f"{name.capitalize()} worker error: {e}")


async def init_database_components(config, project_root: Path):
    """
    初始化数据库相关组件

    Args:
        config: 配置对象
        project_root: 项目根目录
    """
    # 1. 初始化数据库（创建数据库和表）
    try:
        await init_database(
            host=config.database.host,
            port=config.database.port,
            user=config.database.user,
            password=config.database.password,
            database=config.database.database,
            charset=config.database.charset,
            schema_file=str(project_root / "database" / "schema.sql")
        )
    except Exception as e:
        logger.warning(f"Database schema initialization failed: {e}")

    # 2. 初始化数据库连接池
    try:
        await init_db_pool(
            host=config.database.host,
            port=config.database.port,
            user=config.database.user,
            password=config.database.password,
            database=config.database.database,
            charset=config.database.charset,
            pool_size=config.database.pool_size
        )
        logger.info("Database pool initialized")

        # 确保默认管理员存在
        await ensure_default_admin()

    except Exception as e:
        logger.warning(f"Database pool initialization failed (non-critical): {e}")


async def init_cache_components(config):
    """
    初始化缓存相关组件（HTML 文件缓存由 Go API 管理）

    Args:
        config: 配置对象
    """
    # 初始化 Redis 客户端
    if hasattr(config, 'redis') and config.redis.enabled:
        try:
            await init_redis_client(
                host=config.redis.host,
                port=config.redis.port,
                db=config.redis.db,
                password=config.redis.password or None,
            )
            logger.info("Redis client initialized (for queue operations)")
        except Exception as e:
            logger.warning(f"Redis initialization failed: {e}")


async def init_pool_components():
    """初始化各类缓存池组件（已简化）"""
    redis_client = get_redis_client()
    db_pool = get_db_pool()

    if not (redis_client and db_pool):
        logger.warning("Pool components not initialized (Redis client or DB not ready)")
        return

    logger.info("Pool components initialized")


async def init_content_components():
    """初始化内容相关组件（标题、正文管理器）"""
    redis_client = get_redis_client()
    db_pool = get_db_pool()

    if not (redis_client and db_pool):
        return

    # 标题管理器
    try:
        await init_title_manager(redis_client, db_pool, group_id=1, max_size=500000)
        title_manager = get_title_manager(group_id=1)
        if title_manager:
            stats = title_manager.get_stats()
            logger.info(f"Title manager (group 1) initialized: {stats['total_loaded']} titles loaded")
    except Exception as e:
        logger.warning(f"Title manager initialization failed: {e}")

    # 正文管理器
    try:
        await init_content_manager(redis_client, db_pool, group_id=1, max_size=50000)
        content_manager = get_content_manager(group_id=1)
        if content_manager:
            stats = content_manager.get_stats()
            logger.info(f"Content manager (group 1) initialized: {stats['total']} contents loaded")
    except Exception as e:
        logger.warning(f"Content manager initialization failed: {e}")


async def init_background_workers():
    """初始化后台工作线程"""
    global _scheduler_worker

    redis_client = get_redis_client()
    db_pool = get_db_pool()

    if not (redis_client and db_pool):
        return

    # 正文生成器后台任务
    try:
        from core.workers.generator_worker import GeneratorWorker
        asyncio.create_task(_start_background_worker(
            GeneratorWorker, 'generator',
            db_pool=db_pool, redis_client=redis_client
        ))
        logger.info("Generator worker started in background")
    except Exception as e:
        logger.warning(f"Generator worker start failed: {e}")

    # 注意：爬虫统计归档已迁移到 Go API 的 StatsArchiver 服务

    # 爬虫定时调度器
    try:
        from core.workers.spider_scheduler import SpiderSchedulerWorker
        _scheduler_worker = SpiderSchedulerWorker(db_pool=db_pool, redis=redis_client)
        await _scheduler_worker.start()
        logger.info("Spider scheduler worker started")
    except Exception as e:
        logger.warning(f"Spider scheduler worker start failed: {e}")


async def init_components(project_root: Optional[Path] = None):
    """
    初始化所有组件

    Args:
        project_root: 项目根目录路径，默认为当前文件所在目录的父目录
    """
    if project_root is None:
        project_root = Path(__file__).parent.parent

    config = get_config()
    logger.info("Initializing components...")

    # 1-2. 数据库组件
    await init_database_components(config, project_root)

    # 3. 缓存组件
    await init_cache_components(config)

    # 4. 蜘蛛检测器
    init_spider_detector(
        enable_dns_verify=config.spider_detector.dns_verify_enabled,
        dns_verify_types=config.spider_detector.dns_verify_types,
        dns_timeout=config.spider_detector.dns_timeout
    )
    logger.info("Spider detector initialized")

    # 5. 分组管理器
    await init_pool_components()

    # 6. 内容组件（标题、正文管理器）
    await init_content_components()

    # 7. 后台工作线程
    await init_background_workers()

    logger.info("All components initialized successfully")
