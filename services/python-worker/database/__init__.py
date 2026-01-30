"""
数据库模块

提供数据库连接和操作功能：
- db: 数据库连接池管理
- models: 数据模型定义
"""

from .db import (
    get_db_pool,
    init_db_pool,
    init_database,
    close_db_pool,
    execute_query,
    execute_many,
    fetch_one,
    fetch_all,
)

__all__ = [
    'get_db_pool',
    'init_db_pool',
    'init_database',
    'close_db_pool',
    'execute_query',
    'execute_many',
    'fetch_one',
    'fetch_all',
]
