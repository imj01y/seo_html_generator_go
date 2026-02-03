"""
Content Worker 核心模块

提供内容处理的核心功能：
- redis_client: Redis 客户端
- spider_detector: 蜘蛛检测器
- auth: 认证模块
- content_manager: 正文管理器
- initializers: 组件初始化

注意：模板渲染功能和标题管理器已迁移到 Go API (api/internal/service/)
"""

from .redis_client import init_redis_client, get_redis_client
from .spider_detector import init_spider_detector, get_spider_detector
from .auth import ensure_default_admin
from .content_manager import init_content_manager, get_content_manager
from .initializers import init_components

__all__ = [
    # Redis
    'init_redis_client',
    'get_redis_client',
    # Spider Detector
    'init_spider_detector',
    'get_spider_detector',
    # Auth
    'ensure_default_admin',
    # Content Manager
    'init_content_manager',
    'get_content_manager',
    # Initializers
    'init_components',
]
