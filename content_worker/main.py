# -*- coding: utf-8 -*-
"""
Python Worker 入口

启动命令监听器和数据加工管理器：
- CommandListener: 处理来自 Go API 的爬虫命令
- GeneratorManager: 自动处理爬虫抓取的文章数据
"""

import asyncio
import signal
import sys
import os

# 添加当前目录到 Python 路径
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from loguru import logger


async def init_components():
    """初始化所有组件"""
    from config import get_config
    config = get_config()

    # 1. 初始化数据库连接池
    logger.info("初始化数据库连接...")
    from database.db import init_db_pool

    db_config = {
        'host': os.environ.get('DB_HOST', getattr(config.database, 'host', 'localhost')),
        'port': int(os.environ.get('DB_PORT', getattr(config.database, 'port', 3306))),
        'user': os.environ.get('DB_USER', getattr(config.database, 'user', 'root')),
        'password': os.environ.get('DB_PASSWORD', getattr(config.database, 'password', '')),
        'database': os.environ.get('DB_NAME', getattr(config.database, 'database', 'seo_generator')),
    }

    await init_db_pool(**db_config)
    logger.info(f"数据库连接池已初始化: {db_config['host']}:{db_config['port']}")

    # 2. 初始化 Redis 连接
    redis_enabled = os.environ.get('REDIS_ENABLED', 'true').lower() == 'true'
    if redis_enabled:
        logger.info("初始化 Redis 连接...")
        from core.redis_client import init_redis_client

        redis_config = {
            'host': os.environ.get('REDIS_HOST', getattr(config.redis, 'host', 'localhost')),
            'port': int(os.environ.get('REDIS_PORT', getattr(config.redis, 'port', 6379))),
            'db': int(os.environ.get('REDIS_DB', getattr(config.redis, 'db', 0))),
            'password': os.environ.get('REDIS_PASSWORD', getattr(config.redis, 'password', None)),
        }

        await init_redis_client(**redis_config)
        logger.info(f"Redis 连接已初始化: {redis_config['host']}:{redis_config['port']}")
    else:
        logger.warning("Redis 已禁用")

    return config


async def cleanup():
    """清理资源"""
    logger.info("正在关闭连接...")

    try:
        from core.redis_client import close_redis_client
        await close_redis_client()
    except Exception as e:
        logger.warning(f"关闭 Redis 连接时出错: {e}")

    try:
        from database.db import close_db_pool
        await close_db_pool()
    except Exception as e:
        logger.warning(f"关闭数据库连接池时出错: {e}")

    logger.info("连接已关闭")


async def main():
    """主入口"""
    logger.info("=" * 50)
    logger.info("SEO Generator Python Worker 启动中...")
    logger.info("=" * 50)

    # 初始化组件
    try:
        config = await init_components()
    except Exception as e:
        logger.error(f"初始化组件失败: {e}")
        sys.exit(1)

    # 启动命令监听器（爬虫命令）
    from core.workers.command_listener import CommandListener
    listener = CommandListener()

    # 启动数据加工管理器
    from core.workers.generator_manager import GeneratorManager
    generator = GeneratorManager()

    # 启动池配置热更新监听器
    from core.pool_reloader import start_pool_reloader, stop_pool_reloader
    pool_reloader = await start_pool_reloader()

    # 初始化缓存池填充器
    from core.pool_filler import PoolFillerManager
    from database.db import get_db_pool
    from core.redis_client import get_redis_client

    pool_filler_manager = PoolFillerManager(
        redis_client=get_redis_client(),
        db_pool=get_db_pool(),
    )

    # 获取活跃的分组 ID
    async def get_active_group_ids():
        async with get_db_pool().acquire() as conn:
            async with conn.cursor() as cur:
                # 查询有数据的分组
                await cur.execute("""
                    SELECT DISTINCT group_id FROM (
                        SELECT group_id FROM titles WHERE status = 1
                        UNION
                        SELECT group_id FROM contents WHERE status = 1
                    ) t
                """)
                rows = await cur.fetchall()
                return [row[0] for row in rows] if rows else [1]

    group_ids = await get_active_group_ids()
    logger.info(f"发现 {len(group_ids)} 个活跃分组: {group_ids}")

    # 为每个分组添加填充器
    for gid in group_ids:
        pool_filler_manager.add_group(gid)

    # 启动时立即填充
    logger.info("初始化缓存池...")
    await pool_filler_manager.fill_all_now()

    # 启动填充循环
    await pool_filler_manager.start(check_interval=5.0)

    # 设置信号处理
    shutdown_event = asyncio.Event()

    def signal_handler():
        logger.info("收到关闭信号...")
        shutdown_event.set()

    # 注册信号处理器（仅在非 Windows 系统上）
    if sys.platform != 'win32':
        loop = asyncio.get_event_loop()
        for sig in (signal.SIGTERM, signal.SIGINT):
            loop.add_signal_handler(sig, signal_handler)

    try:
        logger.info("启动 Worker 服务...")
        logger.info("  - 命令监听器: 处理爬虫命令")
        logger.info("  - 数据加工器: 自动处理文章数据")
        logger.info("  - 池配置监听器: 动态调整缓存池大小")

        # 创建任务
        listener_task = asyncio.create_task(listener.start(), name="command_listener")
        generator_task = asyncio.create_task(generator.start(), name="generator_manager")
        shutdown_task = asyncio.create_task(shutdown_event.wait(), name="shutdown_wait")

        # 等待任一任务完成
        done, pending = await asyncio.wait(
            [listener_task, generator_task, shutdown_task],
            return_when=asyncio.FIRST_COMPLETED
        )

        # 取消未完成的任务
        for task in pending:
            task.cancel()
            try:
                await task
            except asyncio.CancelledError:
                pass

    except Exception as e:
        logger.error(f"Worker 运行异常: {e}")
    finally:
        await stop_pool_reloader()
        await pool_filler_manager.stop()
        await generator.stop()
        await listener.stop()
        await cleanup()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("Worker 已停止")
    except Exception as e:
        logger.error(f"Worker 异常退出: {e}")
        sys.exit(1)
