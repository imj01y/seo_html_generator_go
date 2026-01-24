"""
Redis客户端管理器（用于队列等非HTML缓存功能）

提供轻量级的Redis客户端连接管理，用于：
- RequestQueue - 爬虫项目队列
- KeywordCachePool - 关键词缓存池
- ImageCachePool - 图片缓存池
- TitleManager - 标题管理
- ContentManager - 正文管理
- ContentPoolManager - 段落池管理
- GeneratorWorker - 正文生成器
- SpiderStatsWorker - 爬虫统计

注意：HTML缓存已迁移到文件缓存（HTMLCacheManager）
"""
from typing import Optional

import redis.asyncio as aioredis
from loguru import logger


_redis_client: Optional[aioredis.Redis] = None


async def init_redis_client(
    host: str = 'localhost',
    port: int = 6379,
    db: int = 0,
    password: Optional[str] = None
) -> aioredis.Redis:
    """
    初始化Redis客户端

    Args:
        host: Redis服务器地址
        port: Redis端口
        db: Redis数据库编号
        password: Redis密码（可选）

    Returns:
        aioredis.Redis: 初始化后的Redis客户端实例

    Raises:
        Exception: 连接失败时抛出异常
    """
    global _redis_client

    _redis_client = aioredis.Redis(
        host=host,
        port=port,
        db=db,
        password=password if password else None,
        decode_responses=False,
        socket_timeout=10.0,
        socket_connect_timeout=5.0,
        protocol=3,  # 使用 RESP3 协议，支持更多数据类型
    )

    # 测试连接
    await _redis_client.ping()

    logger.info(f"Redis client initialized: {host}:{port}/{db}")
    return _redis_client


def get_redis_client() -> Optional[aioredis.Redis]:
    """
    获取Redis客户端

    Returns:
        Optional[aioredis.Redis]: Redis客户端实例，未初始化返回None
    """
    return _redis_client


async def close_redis_client() -> None:
    """
    关闭Redis连接

    在应用关闭时调用，释放Redis连接资源。
    """
    global _redis_client

    if _redis_client:
        try:
            await _redis_client.close()
            logger.info("Redis client connection closed")
        except Exception as e:
            logger.error(f"Error closing Redis connection: {e}")

    _redis_client = None
