"""
数据库模块

提供数据库连接和操作功能：
- db: 数据库连接池管理
"""

from .db import (
    get_db_pool,
    init_db_pool,
    close_db_pool,
    execute_query,
    fetch_one,
    fetch_all,
    insert,
)

__all__ = [
    'get_db_pool',
    'init_db_pool',
    'close_db_pool',
    'execute_query',
    'fetch_one',
    'fetch_all',
    'insert',
]
