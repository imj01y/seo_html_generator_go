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
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
import warnings

import aiomysql
from loguru import logger

# 全局连接池
_pool: Optional[aiomysql.Pool] = None


async def init_database(
    host: str = "localhost",
    port: int = 3306,
    user: str = "root",
    password: str = "",
    database: str = "seo_generator",
    charset: str = "utf8mb4",
    schema_file: str = "./database/schema.sql"
) -> None:
    """
    初始化数据库（创建数据库和表）

    首次启动时自动创建数据库和执行schema.sql。
    如果表已存在则跳过（使用 IF NOT EXISTS）。

    Args:
        host: 数据库主机
        port: 端口
        user: 用户名
        password: 密码
        database: 数据库名
        charset: 字符集
        schema_file: schema.sql 文件路径
    """
    schema_path = Path(schema_file)
    if not schema_path.exists():
        logger.warning(f"Schema file not found: {schema_file}, skipping database init")
        return

    conn = None
    try:
        # 1. 先连接不指定数据库
        conn = await aiomysql.connect(
            host=host,
            port=port,
            user=user,
            password=password,
            charset=charset,
            autocommit=True
        )

        async with conn.cursor() as cur:
            # 2. 创建数据库（如果不存在），抑制 "database exists" 警告
            with warnings.catch_warnings():
                warnings.simplefilter("ignore")
                await cur.execute(
                    f"CREATE DATABASE IF NOT EXISTS `{database}` "
                    f"DEFAULT CHARACTER SET utf8mb4 DEFAULT COLLATE utf8mb4_unicode_ci"
                )
            logger.info(f"Database '{database}' ensured")

            # 3. 切换到该数据库
            await cur.execute(f"USE `{database}`")

            # 4. 读取并执行 schema.sql
            schema_content = schema_path.read_text(encoding='utf-8')

            # 按分号分割语句（正确处理字符串中的分号）
            statements = []
            current_stmt = []
            in_string = False  # 跟踪是否在单引号字符串内

            def should_skip_line(line: str) -> bool:
                """检查是否应跳过该行"""
                upper_line = line.upper()
                return (
                    not line or
                    line.startswith('--') or
                    upper_line.startswith('USE ') or
                    'CREATE DATABASE' in upper_line
                )

            def count_quotes(line: str) -> int:
                """计算行中未转义的单引号数量"""
                return line.replace("''", "").count("'")

            for line in schema_content.split('\n'):
                stripped = line.strip()

                # 跳过注释和空行（仅当不在字符串内时）
                if not in_string and should_skip_line(stripped):
                    continue

                current_stmt.append(line)

                # 计算这行中单引号的数量（排除转义的 ''）
                if count_quotes(line) % 2 == 1:
                    in_string = not in_string

                # 只有在字符串外且行末是分号时才分割语句
                if not in_string and stripped.endswith(';'):
                    stmt = '\n'.join(current_stmt).strip()
                    if stmt:
                        statements.append(stmt)
                    current_stmt = []

            # 执行所有语句（抑制 IF NOT EXISTS 警告）
            executed = 0
            with warnings.catch_warnings():
                warnings.simplefilter("ignore")
                for stmt in statements:
                    try:
                        await cur.execute(stmt)
                        executed += 1
                    except Exception as e:
                        # 忽略重复键错误（ON DUPLICATE KEY）
                        if 'Duplicate' not in str(e):
                            logger.debug(f"SQL warning: {e}")

            logger.info(f"Database initialized: executed {executed} statements")

            # 5. 加载默认模板内容（从 HTML 文件）
            await _load_default_templates(cur, schema_path.parent)

    except Exception as e:
        logger.error(f"Failed to initialize database: {e}")
        raise
    finally:
        if conn:
            conn.close()


async def _load_default_templates(cur: aiomysql.Cursor, base_path: Path) -> None:
    """
    从 templates 目录加载默认模板内容

    Args:
        cur: 数据库游标
        base_path: database 目录路径
    """
    templates_dir = Path(base_path) / "templates"
    if not templates_dir.exists():
        logger.debug(f"Templates directory not found: {templates_dir}")
        return

    # 模板名称到文件的映射
    template_files = {
        "download_site": "download_site.html",
    }

    for template_name, filename in template_files.items():
        template_file = templates_dir / filename
        if not template_file.exists():
            logger.debug(f"Template file not found: {template_file}")
            continue

        try:
            content = template_file.read_text(encoding='utf-8')

            # 更新模板内容（仅当内容为空时）
            await cur.execute(
                "UPDATE templates SET content = %s WHERE name = %s AND (content IS NULL OR content = '')",
                (content, template_name)
            )

            if cur.rowcount > 0:
                logger.info(f"Loaded template '{template_name}' from {filename}")
            else:
                logger.debug(f"Template '{template_name}' already has content, skipped")

        except Exception as e:
            logger.warning(f"Failed to load template '{template_name}': {e}")


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


async def execute_many(
    sql: str,
    args_list: List[Tuple],
    commit: bool = True
) -> int:
    """
    批量执行SQL

    Args:
        sql: SQL语句
        args_list: 参数列表
        commit: 是否提交事务

    Returns:
        受影响的行数
    """
    async with get_cursor(dict_cursor=False) as cur:
        await cur.executemany(sql, args_list)
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


async def fetch_value(
    sql: str,
    args: Optional[Tuple] = None
) -> Any:
    """
    获取单个值

    Args:
        sql: SQL语句
        args: 参数元组

    Returns:
        单个值或None
    """
    async with get_cursor(dict_cursor=False) as cur:
        await cur.execute(sql, args)
        row = await cur.fetchone()
        return row[0] if row else None


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


async def update(
    table: str,
    data: Dict[str, Any],
    where: str,
    where_args: Tuple,
    commit: bool = True
) -> int:
    """
    更新记录

    Args:
        table: 表名
        data: 更新的数据字典
        where: WHERE条件
        where_args: WHERE参数
        commit: 是否提交

    Returns:
        受影响的行数
    """
    set_clause = ', '.join(f'`{k}` = %s' for k in data.keys())
    sql = f"UPDATE `{table}` SET {set_clause} WHERE {where}"
    args = tuple(data.values()) + where_args

    async with get_cursor() as cur:
        await cur.execute(sql, args)
        if commit:
            await cur.connection.commit()
        return cur.rowcount


async def delete(
    table: str,
    where: str,
    where_args: Tuple,
    commit: bool = True
) -> int:
    """
    删除记录

    Args:
        table: 表名
        where: WHERE条件
        where_args: WHERE参数
        commit: 是否提交

    Returns:
        受影响的行数
    """
    sql = f"DELETE FROM `{table}` WHERE {where}"

    async with get_cursor() as cur:
        await cur.execute(sql, where_args)
        if commit:
            await cur.connection.commit()
        return cur.rowcount


async def check_connection() -> Dict[str, Any]:
    """
    检查数据库连接状态

    Returns:
        连接状态信息
    """
    global _pool

    if _pool is None:
        return {
            "connected": False,
            "error": "Pool not initialized"
        }

    try:
        async with get_cursor() as cur:
            await cur.execute("SELECT 1")
            result = await cur.fetchone()

        return {
            "connected": True,
            "pool_size": _pool.maxsize,
            "free_connections": _pool.freesize,
            "used_connections": _pool.maxsize - _pool.freesize
        }
    except Exception as e:
        return {
            "connected": False,
            "error": str(e)
        }


async def init_from_config(config) -> aiomysql.Pool:
    """
    从配置对象初始化连接池

    Args:
        config: DatabaseConfig实例或Dynaconf配置对象

    Returns:
        aiomysql.Pool实例
    """
    return await init_db_pool(
        host=getattr(config, 'host', 'localhost'),
        port=getattr(config, 'port', 3306),
        user=getattr(config, 'user', 'root'),
        password=getattr(config, 'password', ''),
        database=getattr(config, 'database', 'seo_generator'),
        charset=getattr(config, 'charset', 'utf8mb4'),
        pool_size=getattr(config, 'pool_size', 5),
        pool_recycle=getattr(config, 'pool_recycle', 3600)
    )
