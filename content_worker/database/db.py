"""
数据库连接池管理模块

使用aiomysql实现异步MySQL连接池管理。

主要功能:
- init_db_pool(): 初始化连接池
- close_db_pool(): 关闭连接池
- execute_query(): 执行SQL查询
- fetch_one/fetch_all(): 获取查询结果
"""
from contextlib import asynccontextmanager
from typing import Any, Dict, List, Optional, Tuple

import aiomysql
from loguru import logger

# 全局连接池
_pool: Optional[aiomysql.Pool] = None


async def init_db_pool(
    host: str = "localhost",
    port: int = 3306,
    user: str = "root",
    password: str = "",
    database: str = "seo_generator",
    charset: str = "utf8mb4",
    pool_size: int = 30,
    pool_recycle: int = 1800,
    **kwargs
) -> aiomysql.Pool:
    """
    初始化数据库连接池

    Args:
        host: 数据库主机
        port: 端口
        user: 用户名
        password: 密码
        database: 数据库名
        charset: 字符集
        pool_size: 连接池大小
        pool_recycle: 连接回收时间（秒）

    Returns:
        aiomysql.Pool实例
    """
    global _pool

    if _pool is not None:
        return _pool

    try:
        _pool = await aiomysql.create_pool(
            host=host,
            port=port,
            user=user,
            password=password,
            db=database,
            charset=charset,
            minsize=10,
            maxsize=pool_size,
            pool_recycle=pool_recycle,
            autocommit=True,
            **kwargs
        )
        logger.info(f"Database pool initialized: {host}:{port}/{database}")
        return _pool
    except Exception as e:
        logger.error(f"Failed to initialize database pool: {e}")
        raise


async def close_db_pool() -> None:
    """关闭数据库连接池"""
    global _pool

    if _pool is not None:
        _pool.close()
        await _pool.wait_closed()
        _pool = None
        logger.info("Database pool closed")


def get_db_pool() -> Optional[aiomysql.Pool]:
    """获取数据库连接池"""
    return _pool


@asynccontextmanager
async def get_connection():
    """
    获取数据库连接（上下文管理器）

    Usage:
        async with get_connection() as conn:
            async with conn.cursor() as cur:
                await cur.execute("SELECT * FROM sites")
    """
    global _pool

    if _pool is None:
        raise RuntimeError("Database pool not initialized")

    conn = await _pool.acquire()
    try:
        yield conn
    finally:
        _pool.release(conn)


@asynccontextmanager
async def get_cursor(dict_cursor: bool = True):
    """
    获取数据库游标（上下文管理器）

    Args:
        dict_cursor: 是否使用字典游标

    Usage:
        async with get_cursor() as cur:
            await cur.execute("SELECT * FROM sites")
            result = await cur.fetchall()
    """
    async with get_connection() as conn:
        cursor_class = aiomysql.DictCursor if dict_cursor else aiomysql.Cursor
        async with conn.cursor(cursor_class) as cur:
            yield cur


async def execute_query(
    sql: str,
    args: Optional[Tuple] = None,
    commit: bool = False
) -> int:
    """
    执行SQL查询

    Args:
        sql: SQL语句
        args: 参数元组
        commit: 是否提交事务

    Returns:
        受影响的行数
    """
    async with get_cursor(dict_cursor=False) as cur:
        await cur.execute(sql, args)
        if commit:
            await cur.connection.commit()
        return cur.rowcount


async def fetch_one(
    sql: str,
    args: Optional[Tuple] = None
) -> Optional[Dict[str, Any]]:
    """
    获取单条记录

    Args:
        sql: SQL语句
        args: 参数元组

    Returns:
        记录字典或None
    """
    async with get_cursor() as cur:
        await cur.execute(sql, args)
        return await cur.fetchone()


async def fetch_all(
    sql: str,
    args: Optional[Tuple] = None
) -> List[Dict[str, Any]]:
    """
    获取所有记录

    Args:
        sql: SQL语句
        args: 参数元组

    Returns:
        记录列表
    """
    async with get_cursor() as cur:
        await cur.execute(sql, args)
        return await cur.fetchall()


async def insert(
    table: str,
    data: Dict[str, Any],
    commit: bool = True
) -> int:
    """
    插入单条记录

    Args:
        table: 表名
        data: 数据字典
        commit: 是否提交

    Returns:
        插入的ID
    """
    columns = ', '.join(f'`{k}`' for k in data.keys())
    placeholders = ', '.join(['%s'] * len(data))
    sql = f"INSERT INTO `{table}` ({columns}) VALUES ({placeholders})"

    async with get_cursor() as cur:
        await cur.execute(sql, tuple(data.values()))
        if commit:
            await cur.connection.commit()
        return cur.lastrowid
