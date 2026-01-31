# -*- coding: utf-8 -*-
"""
池配置热更新监听器

监听 Redis pool:reload 频道，动态调整缓存池大小。
"""

import asyncio
import json
from typing import Optional

from loguru import logger

from core.redis_client import get_redis_client
from core.keyword_cache_pool import get_keyword_cache_pool
from core.image_cache_pool import get_image_cache_pool


class PoolReloader:
    """池配置热更新监听器"""

    def __init__(self):
        self._running = False
        self._task: Optional[asyncio.Task] = None
        self._redis = None

    async def start(self):
        """启动监听"""
        if self._running:
            return

        self._redis = get_redis_client()
        if not self._redis:
            logger.error("Redis client not available, cannot start pool reloader")
            return

        self._running = True
        self._task = asyncio.create_task(self._listen())
        logger.info("Pool reloader started, listening on pool:reload channel")

    async def stop(self):
        """停止监听"""
        self._running = False
        if self._task:
            self._task.cancel()
            try:
                await self._task
            except asyncio.CancelledError:
                pass
            self._task = None
        logger.info("Pool reloader stopped")

    async def _listen(self):
        """监听 Redis 消息"""
        pubsub = None
        try:
            pubsub = self._redis.pubsub()
            await pubsub.subscribe("pool:reload")

            async for message in pubsub.listen():
                if not self._running:
                    break
                if message["type"] == "message":
                    await self._handle_message(message["data"])

        except asyncio.CancelledError:
            pass
        except Exception as e:
            logger.error(f"Pool reloader listen error: {e}")
        finally:
            if pubsub:
                try:
                    await pubsub.unsubscribe("pool:reload")
                except Exception:
                    pass

    async def _handle_message(self, data):
        """处理消息"""
        try:
            if isinstance(data, bytes):
                data = data.decode('utf-8')
            msg = json.loads(data)

            if msg.get("action") != "reload":
                logger.debug(f"Ignoring non-reload message: {msg.get('action')}")
                return

            sizes = msg.get("sizes", {})
            keyword_size = sizes.get("keyword_pool_size")
            image_size = sizes.get("image_pool_size")

            logger.info(f"Received pool reload: keyword={keyword_size}, image={image_size}")

            # Resize pools
            keyword_pool = get_keyword_cache_pool()
            image_pool = get_image_cache_pool()

            if keyword_pool and keyword_size and isinstance(keyword_size, int) and keyword_size > 0:
                await keyword_pool.resize(keyword_size)

            if image_pool and image_size and isinstance(image_size, int) and image_size > 0:
                await image_pool.resize(image_size)

            logger.info("Pool sizes updated successfully")

        except json.JSONDecodeError as e:
            logger.error(f"Failed to parse pool reload message: {e}")
        except Exception as e:
            logger.error(f"Failed to handle pool reload message: {e}")


# Global instance
_pool_reloader: Optional[PoolReloader] = None


def get_pool_reloader() -> Optional[PoolReloader]:
    """获取全局池重载器实例"""
    return _pool_reloader


async def start_pool_reloader() -> PoolReloader:
    """启动全局池重载器"""
    global _pool_reloader
    _pool_reloader = PoolReloader()
    await _pool_reloader.start()
    return _pool_reloader


async def stop_pool_reloader():
    """停止全局池重载器"""
    global _pool_reloader
    if _pool_reloader:
        await _pool_reloader.stop()
        _pool_reloader = None
