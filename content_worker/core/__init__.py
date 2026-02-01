"""
Content Worker 核心模块

提供内容处理的核心功能：
- redis_client: Redis 客户端
- spider_detector: 蜘蛛检测器
- auth: 认证模块
- title_manager: 标题管理器
- content_manager: 正文管理器
- initializers: 组件初始化

注意：模板渲染功能已迁移到 Go API (api/internal/service/)
"""

from .redis_client import init_redis_client, get_redis_client
from .spider_detector import init_spider_detector, get_spider_detector
from .auth import ensure_default_admin
from .title_manager import init_title_manager, get_title_manager
from .content_manager import init_content_manager, get_content_manager
from .initializers import init_components
from .pool_filler import PoolFiller, PoolFillerManager

__all__ = [
    # Redis
    'init_redis_client',
    'get_redis_client',
    # Spider Detector
    'init_spider_detector',
    'get_spider_detector',
    # Auth
    'ensure_default_admin',
    # Title Manager
    'init_title_manager',
    'get_title_manager',
    # Content Manager
    'init_content_manager',
    'get_content_manager',
    # Initializers
    'init_components',
    # Pool Filler
    'PoolFiller',
    'PoolFillerManager',
]
