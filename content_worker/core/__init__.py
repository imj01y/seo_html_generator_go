"""
Content Worker 核心模块

提供内容处理的核心功能：
- redis_client: Redis 客户端

注意：
- 模板渲染功能已迁移到 Go API (api/internal/service/)
- 标题管理器已迁移到 Go API (api/internal/service/title_generator.go)
- 蜘蛛检测器已迁移到 Go API (api/internal/service/spider_detector.go)
- 正文管理器已迁移到 Go API (api/internal/service/pool_manager.go)
- 认证模块已迁移到 Go API
"""

from .redis_client import init_redis_client, get_redis_client

__all__ = [
    # Redis
    'init_redis_client',
    'get_redis_client',
]
