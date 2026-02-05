# -*- coding: utf-8 -*-
"""
池配置热更新监听器

监听 Redis pool:reload 频道，动态调整缓存池大小。

注意：关键词和图片池已迁移到 Go API 的 DataManager 管理，
此模块仅保留框架，用于未来可能的其他池类型扩展。
"""

import asyncio
import json
from typing import Any, Dict, Optional

from loguru import logger

from config import settings
from core.redis_client import get_redis_client


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
            logger.warning("Redis client not available, pool reloader disabled")
            return

        self._running = True
        self._task = asyncio.create_task(self._listen())
        logger.info(f"Pool reloader started, listening on {settings.channels.pool_reload} channel")

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
            await pubsub.subscribe(settings.channels.pool_reload)

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
                    await pubsub.unsubscribe(settings.channels.pool_reload)
                except Exception:
                    pass

    async def _handle_message(self, data: Any) -> None:
        """处理消息（关键词和图片池已迁移到 Go API）"""
        try:
            if isinstance(data, bytes):
                data = data.decode('utf-8')
            msg: Dict[str, Any] = json.loads(data)

            if msg.get("action") != "reload":
                logger.debug(f"Ignoring non-reload message: {msg.get('action')}")
                return

            sizes = msg.get("sizes", {})
            logger.info(f"Received pool reload message: {sizes}")
            logger.info("Note: Keyword/Image pools are now managed by Go API DataManager")

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
