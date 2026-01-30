# -*- coding: utf-8 -*-
"""
Python Worker 入口

启动命令监听器，处理来自 Go API 的爬虫和生成器命令。
"""

import asyncio
import sys
import os

# 添加当前目录到 Python 路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from loguru import logger


async def main():
    """主入口"""
    logger.info("Python Worker 启动中...")

    # 初始化数据库连接
    from database.db import init_db_pool
    await init_db_pool()
    logger.info("数据库连接池已初始化")

    # 初始化 Redis 连接
    from core.redis_client import init_redis
    await init_redis()
    logger.info("Redis 连接已初始化")

    # 启动命令监听器
    from core.workers.command_listener import CommandListener
    listener = CommandListener()

    logger.info("命令监听器启动，等待命令...")
    await listener.start()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Worker 已停止")
    except Exception as e:
        logger.error(f"Worker 异常退出: {e}")
        sys.exit(1)
