# -*- coding: utf-8 -*-
"""
应用生命周期管理模块

提供 FastAPI 应用的启动和关闭生命周期管理。
"""
from contextlib import asynccontextmanager

from fastapi import FastAPI
from loguru import logger

from core.redis_client import close_redis_client
from database.db import close_db_pool
from core.keyword_cache_pool import stop_keyword_cache_pool
from core.image_cache_pool import stop_image_cache_pool
from core.class_generator import stop_class_string_pool
from core.link_generator import stop_url_pool
from core.random_number_pool import stop_random_number_pool


async def _safe_stop(coro, name: str):
    """
    安全停止组件的辅助函数

    Args:
        coro: 要执行的协程
        name: 组件名称（用于日志）
    """
    try:
        await coro
        logger.info(f"{name} stopped")
    except Exception as e:
        logger.warning(f"Error stopping {name}: {e}")


async def cleanup_components():
    """清理所有组件"""
    # 导入 workers 引用
    from core.initializers import _generator_worker, _stats_worker, _scheduler_worker

    logger.info("Cleaning up components...")

    # 停止 workers
    if _generator_worker:
        await _safe_stop(_generator_worker.stop(), "Generator worker")

    if _stats_worker:
        await _safe_stop(_stats_worker.stop(), "Spider stats worker")

    if _scheduler_worker:
        await _safe_stop(_scheduler_worker.stop(), "Spider scheduler worker")

    # 停止缓存池
    await _safe_stop(stop_keyword_cache_pool(), "Keyword cache pool")
    await _safe_stop(stop_image_cache_pool(), "Image cache pool")
    await _safe_stop(stop_class_string_pool(), "Class string pool")
    await _safe_stop(stop_url_pool(), "URL pool")
    await _safe_stop(stop_random_number_pool(), "Random number pool")

    # 关闭连接
    await _safe_stop(close_redis_client(), "Redis client connection")
    await _safe_stop(close_db_pool(), "Database pool")

    logger.info("Cleanup completed")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """
    应用生命周期管理

    FastAPI 的 lifespan 上下文管理器，处理应用启动和关闭时的初始化和清理工作。

    Args:
        app: FastAPI 应用实例
    """
    from core.initializers import init_components

    # 启动时初始化
    await init_components()
    yield
    # 关闭时清理
    await cleanup_components()
