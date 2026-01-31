# -*- coding: utf-8 -*-
"""
失败请求管理器

管理失败的请求（MySQL 持久化），支持查询、重试、忽略等操作。
"""

import json
from dataclasses import dataclass
from typing import Optional, Dict, Any, List, TYPE_CHECKING
from datetime import datetime
from loguru import logger

if TYPE_CHECKING:
    from aiomysql import Pool
    from redis.asyncio import Redis

from .request import Request
from .request_queue import RequestQueue


@dataclass
class FailedRequest:
    """失败请求记录"""
    id: int
    project_id: int
    url: str
    method: str
    callback: str
    meta: Dict[str, Any]
    error_message: str
    retry_count: int
    failed_at: datetime
    status: str  # pending, retried, ignored

    def to_dict(self) -> Dict[str, Any]:
        return {
            'id': self.id,
            'project_id': self.project_id,
            'url': self.url,
            'method': self.method,
            'callback': self.callback,
            'meta': self.meta,
            'error_message': self.error_message,
            'retry_count': self.retry_count,
            'failed_at': self.failed_at.isoformat() if self.failed_at else None,
            'status': self.status,
        }


class FailedRequestManager:
    """
    失败请求管理器

    将失败的请求持久化到 MySQL，支持：
    - 保存失败请求
    - 分页查询
    - 重试单个/全部
    - 忽略/删除

    Example:
        manager = FailedRequestManager(db_pool)

        # 保存失败请求
        await manager.save(project_id=1, request=request, error="Connection timeout")

        # 查询失败请求
        result = await manager.list(project_id=1, page=1, page_size=20)

        # 重试单个
        await manager.retry_one(id=1, redis=redis_client)

        # 重试所有
        await manager.retry_all(project_id=1, redis=redis_client)
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

    async def list(
        self,
        project_id: int,
        page: int = 1,
        page_size: int = 20,
        status: Optional[str] = None,
    ) -> Dict[str, Any]:
        """
        分页查询失败请求

        Args:
            project_id: 项目ID
            page: 页码
            page_size: 每页数量
            status: 状态过滤（pending, retried, ignored）

        Returns:
            dict: {total, data: [FailedRequest, ...]}
        """
        conditions = ["project_id = %s"]
        args = [project_id]

        if status:
            conditions.append("status = %s")
            args.append(status)

        where = " AND ".join(conditions)

        # 查询总数
        count_sql = f"SELECT COUNT(*) as cnt FROM spider_failed_requests WHERE {where}"
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(count_sql, args)
                row = await cursor.fetchone()
                total = row[0] if row else 0

        # 查询数据
        offset = (page - 1) * page_size
        data_sql = f"""
            SELECT id, project_id, url, method, callback, meta, error_message,
                   retry_count, failed_at, status
            FROM spider_failed_requests
            WHERE {where}
            ORDER BY failed_at DESC
            LIMIT %s OFFSET %s
        """
        args.extend([page_size, offset])

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(data_sql, args)
                rows = await cursor.fetchall()

        data = []
        for row in rows:
            meta = json.loads(row[5]) if row[5] else {}
            data.append(FailedRequest(
                id=row[0],
                project_id=row[1],
                url=row[2],
                method=row[3],
                callback=row[4],
                meta=meta,
                error_message=row[6],
                retry_count=row[7],
                failed_at=row[8],
                status=row[9],
            ).to_dict())

        return {
            'total': total,
            'data': data,
            'page': page,
            'page_size': page_size,
        }

    async def get_one(self, id: int) -> Optional[FailedRequest]:
        """
        获取单个失败请求

        Args:
            id: 记录ID

        Returns:
            FailedRequest 或 None
        """
        sql = """
            SELECT id, project_id, url, method, callback, meta, error_message,
                   retry_count, failed_at, status
            FROM spider_failed_requests
            WHERE id = %s
        """
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (id,))
                row = await cursor.fetchone()

        if not row:
            return None

        meta = json.loads(row[5]) if row[5] else {}
        return FailedRequest(
            id=row[0],
            project_id=row[1],
            url=row[2],
            method=row[3],
            callback=row[4],
            meta=meta,
            error_message=row[6],
            retry_count=row[7],
            failed_at=row[8],
            status=row[9],
        )

    async def retry_one(self, id: int, redis: 'Redis') -> bool:
        """
        重试单个失败请求

        Args:
            id: 记录ID
            redis: Redis 客户端

        Returns:
            bool: 是否成功
        """
        # 获取失败请求
        failed = await self.get_one(id)
        if not failed:
            return False

        if failed.status != 'pending':
            logger.warning(f"Failed request {id} is not pending, status={failed.status}")
            return False

        # 创建 Request 对象
        request = Request(
            url=failed.url,
            method=failed.method,
            callback=failed.callback,
            meta=failed.meta,
            retry_count=0,  # 重置重试计数
            dont_filter=True,  # 跳过去重
        )

        # 加入队列
        queue = RequestQueue(redis, failed.project_id)
        await queue.push(request)

        # 更新状态
        await self._update_status(id, 'retried')

        logger.info(f"Retried failed request {id}: {failed.url[:50]}")
        return True

    async def retry_all(self, project_id: int, redis: 'Redis') -> int:
        """
        重试所有失败请求

        Args:
            project_id: 项目ID
            redis: Redis 客户端

        Returns:
            int: 重试的数量
        """
        # 获取所有 pending 状态的失败请求
        sql = """
            SELECT id, url, method, callback, meta
            FROM spider_failed_requests
            WHERE project_id = %s AND status = 'pending'
        """
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (project_id,))
                rows = await cursor.fetchall()

        if not rows:
            return 0

        queue = RequestQueue(redis, project_id)
        count = 0

        for row in rows:
            id, url, method, callback, meta_str = row
            meta = json.loads(meta_str) if meta_str else {}

            request = Request(
                url=url,
                method=method,
                callback=callback,
                meta=meta,
                retry_count=0,
                dont_filter=True,
            )

            await queue.push(request)
            await self._update_status(id, 'retried')
            count += 1

        logger.info(f"Retried {count} failed requests for project {project_id}")
        return count

    async def ignore(self, id: int) -> bool:
        """
        忽略失败请求

        Args:
            id: 记录ID

        Returns:
            bool: 是否成功
        """
        result = await self._update_status(id, 'ignored')
        if result:
            logger.info(f"Ignored failed request {id}")
        return result

    async def delete(self, id: int) -> bool:
        """
        删除失败请求记录

        Args:
            id: 记录ID

        Returns:
            bool: 是否成功
        """
        sql = "DELETE FROM spider_failed_requests WHERE id = %s"
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (id,))
                await conn.commit()
                return cursor.rowcount > 0

    async def delete_by_project(self, project_id: int, status: Optional[str] = None) -> int:
        """
        删除项目的失败请求

        Args:
            project_id: 项目ID
            status: 可选状态过滤

        Returns:
            int: 删除的数量
        """
        if status:
            sql = "DELETE FROM spider_failed_requests WHERE project_id = %s AND status = %s"
            args = (project_id, status)
        else:
            sql = "DELETE FROM spider_failed_requests WHERE project_id = %s"
            args = (project_id,)

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, args)
                await conn.commit()
                return cursor.rowcount

    async def _update_status(self, id: int, status: str) -> bool:
        """更新状态"""
        sql = "UPDATE spider_failed_requests SET status = %s WHERE id = %s"
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (status, id))
                await conn.commit()
                return cursor.rowcount > 0

    async def get_stats(self, project_id: int) -> Dict[str, int]:
        """
        获取项目失败请求统计

        Args:
            project_id: 项目ID

        Returns:
            dict: {pending, retried, ignored, total}
        """
        sql = """
            SELECT status, COUNT(*) as cnt
            FROM spider_failed_requests
            WHERE project_id = %s
            GROUP BY status
        """
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (project_id,))
                rows = await cursor.fetchall()

        stats = {'pending': 0, 'retried': 0, 'ignored': 0, 'total': 0}
        for row in rows:
            status, count = row
            if status in stats:
                stats[status] = count
            stats['total'] += count

        return stats
