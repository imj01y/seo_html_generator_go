# -*- coding: utf-8 -*-
"""
失败请求管理器

将失败的请求持久化到 MySQL。
注意：查询、重试、忽略等操作由 Go API 处理。
"""

import json
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from aiomysql import Pool

from .request import Request


class FailedRequestManager:
    """
    失败请求管理器

    将失败的请求持久化到 MySQL。

    Example:
        manager = FailedRequestManager(db_pool)
        await manager.save(project_id=1, request=request, error="Connection timeout")
    """

    def __init__(self, db_pool: 'Pool'):
        """
        初始化管理器

        Args:
            db_pool: MySQL 连接池
        """
        self.db_pool = db_pool

    async def save(
        self,
        project_id: int,
        request: Request,
        error_message: str,
    ) -> int:
        """
        保存失败请求

        Args:
            project_id: 项目ID
            request: 请求对象
            error_message: 错误信息

        Returns:
            int: 插入的记录ID
        """
        sql = """
            INSERT INTO spider_failed_requests
            (project_id, url, method, callback, meta, error_message, retry_count, status)
            VALUES (%s, %s, %s, %s, %s, %s, %s, 'pending')
        """
        args = (
            project_id,
            request.url[:2048],  # URL 最大长度
            request.method,
            request.callback_name,
            json.dumps(request.meta, ensure_ascii=False) if request.meta else None,
            error_message[:65535] if error_message else None,  # TEXT 最大长度
            request.retry_count,
        )

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, args)
                await conn.commit()
                return cursor.lastrowid
