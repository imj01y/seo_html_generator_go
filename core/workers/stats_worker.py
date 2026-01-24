# -*- coding: utf-8 -*-
"""
统计归档 Worker

定时将 Redis 中的实时统计数据归档到 MySQL，支持分钟/小时/天/月维度的聚合。
"""

import asyncio
from datetime import datetime, timedelta
from typing import Optional, Dict, Any, List, TYPE_CHECKING
from loguru import logger

if TYPE_CHECKING:
    from aiomysql import Pool
    from redis.asyncio import Redis


class SpiderStatsWorker:
    """
    爬虫统计归档 Worker

    定时任务：
    - 每分钟：Redis 实时数据 -> MySQL 分钟表
    - 每小时：聚合分钟数据 -> MySQL 小时表
    - 每天：聚合小时数据 -> MySQL 天表
    - 每月：聚合天数据 -> MySQL 月表

    数据保留：
    - 分钟数据保留 7 天
    - 小时数据保留 30 天
    - 天数据保留 1 年
    - 月数据永久保留
    """

    # 周期类型
    PERIOD_MINUTE = 'minute'
    PERIOD_HOUR = 'hour'
    PERIOD_DAY = 'day'
    PERIOD_MONTH = 'month'

    # 数据保留时间（天）
    RETENTION_MINUTE = 7
    RETENTION_HOUR = 30
    RETENTION_DAY = 365

    def __init__(self, db_pool: 'Pool', redis: 'Redis'):
        """
        初始化 Worker

        Args:
            db_pool: MySQL 连接池
            redis: Redis 客户端
        """
        self.db_pool = db_pool
        self.redis = redis
        self._running = False
        self._last_minute_run = None
        self._last_hour_run = None
        self._last_day_run = None
        self._last_month_run = None

    async def start(self) -> None:
        """启动 Worker"""
        self._running = True
        logger.info("Spider stats worker started")

        while self._running:
            try:
                now = datetime.now()

                # 每分钟执行
                if self._should_run_minute(now):
                    await self._archive_minute_stats()
                    self._last_minute_run = now

                # 每小时执行
                if self._should_run_hour(now):
                    await self._aggregate_hour_stats()
                    await self._cleanup_old_data(self.PERIOD_MINUTE, self.RETENTION_MINUTE)
                    self._last_hour_run = now

                # 每天执行
                if self._should_run_day(now):
                    await self._aggregate_day_stats()
                    await self._cleanup_old_data(self.PERIOD_HOUR, self.RETENTION_HOUR)
                    self._last_day_run = now

                # 每月执行
                if self._should_run_month(now):
                    await self._aggregate_month_stats()
                    await self._cleanup_old_data(self.PERIOD_DAY, self.RETENTION_DAY)
                    self._last_month_run = now

            except Exception as e:
                logger.error(f"Stats worker error: {e}")

            # 每 10 秒检查一次
            await asyncio.sleep(10)

    async def stop(self) -> None:
        """停止 Worker"""
        self._running = False
        logger.info("Spider stats worker stopped")

    def _should_run_minute(self, now: datetime) -> bool:
        """是否应该执行分钟归档"""
        if self._last_minute_run is None:
            return True
        return (now - self._last_minute_run).total_seconds() >= 60

    def _should_run_hour(self, now: datetime) -> bool:
        """是否应该执行小时聚合"""
        if self._last_hour_run is None:
            return now.minute == 0
        return (now - self._last_hour_run).total_seconds() >= 3600

    def _should_run_day(self, now: datetime) -> bool:
        """是否应该执行天聚合"""
        if self._last_day_run is None:
            return now.hour == 0 and now.minute < 10
        return (now - self._last_day_run).total_seconds() >= 86400

    def _should_run_month(self, now: datetime) -> bool:
        """是否应该执行月聚合"""
        if self._last_month_run is None:
            return now.day == 1 and now.hour == 0
        # 检查是否跨月
        return now.month != self._last_month_run.month

    async def _get_active_projects(self) -> List[int]:
        """获取所有活跃的项目ID（有 Redis 统计数据的）"""
        cursor = 0
        project_ids = set()

        while True:
            cursor, keys = await self.redis.scan(cursor, match="spider:*:stats", count=100)
            for key in keys:
                key_str = key.decode() if isinstance(key, bytes) else key
                # spider:{project_id}:stats
                parts = key_str.split(':')
                if len(parts) >= 2:
                    try:
                        project_id = int(parts[1])
                        project_ids.add(project_id)
                    except ValueError:
                        pass
            if cursor == 0:
                break

        return list(project_ids)

    async def _get_redis_stats(self, project_id: int) -> Dict[str, int]:
        """获取项目的 Redis 实时统计"""
        key = f"spider:{project_id}:stats"
        stats_data = await self.redis.hgetall(key)

        def _get_int(field: str) -> int:
            val = stats_data.get(field) or stats_data.get(field.encode())
            if val is None:
                return 0
            if isinstance(val, bytes):
                val = val.decode()
            return int(val)

        return {
            'total': _get_int('total'),
            'completed': _get_int('completed'),
            'failed': _get_int('failed'),
            'retried': _get_int('retried'),
        }

    async def _get_last_archived_stats(self, project_id: int) -> Dict[str, int]:
        """从 Redis 获取上次归档时的累计值快照"""
        key = f"spider:{project_id}:stats:archived"
        data = await self.redis.hgetall(key)

        def _get_int(field: str) -> int:
            val = data.get(field) or data.get(field.encode())
            if val is None:
                return 0
            if isinstance(val, bytes):
                val = val.decode()
            return int(val)

        return {
            'total': _get_int('total'),
            'completed': _get_int('completed'),
            'failed': _get_int('failed'),
            'retried': _get_int('retried'),
        }

    async def _save_last_archived_stats(self, project_id: int, stats: Dict[str, int]) -> None:
        """保存当前累计值作为下次计算增量的基准"""
        key = f"spider:{project_id}:stats:archived"
        await self.redis.hset(key, mapping={
            'total': stats['total'],
            'completed': stats['completed'],
            'failed': stats['failed'],
            'retried': stats['retried'],
        })

    async def _archive_minute_stats(self) -> None:
        """归档分钟统计数据（保存增量而非累计值）"""
        project_ids = await self._get_active_projects()
        if not project_ids:
            return

        now = datetime.now()
        period_start = now.replace(second=0, microsecond=0)

        for project_id in project_ids:
            try:
                # 获取当前 Redis 累计值
                current = await self._get_redis_stats(project_id)
                if current['total'] == 0:
                    continue

                # 获取上次归档时的累计值
                last = await self._get_last_archived_stats(project_id)

                # 计算增量
                delta = {
                    'total': max(0, current['total'] - last['total']),
                    'completed': max(0, current['completed'] - last['completed']),
                    'failed': max(0, current['failed'] - last['failed']),
                    'retried': max(0, current['retried'] - last['retried']),
                }

                # 有变化才保存
                if delta['total'] > 0 or delta['completed'] > 0 or delta['failed'] > 0:
                    # 增量完成数作为该分钟的速度
                    avg_speed = delta['completed']

                    await self._save_stats(
                        project_id=project_id,
                        period_type=self.PERIOD_MINUTE,
                        period_start=period_start,
                        total=delta['total'],
                        completed=delta['completed'],
                        failed=delta['failed'],
                        retried=delta['retried'],
                        avg_speed=avg_speed,
                    )

                # 更新归档基准值（无论有没有变化都更新，避免负增量）
                await self._save_last_archived_stats(project_id, current)
            except Exception as e:
                logger.error(f"Error archiving minute stats for project {project_id}: {e}")

        logger.debug(f"Archived minute stats for {len(project_ids)} projects")

    async def _aggregate_hour_stats(self) -> None:
        """聚合小时统计数据"""
        now = datetime.now()
        hour_start = now.replace(minute=0, second=0, microsecond=0) - timedelta(hours=1)
        hour_end = hour_start + timedelta(hours=1)

        await self._aggregate_period(
            source_type=self.PERIOD_MINUTE,
            target_type=self.PERIOD_HOUR,
            period_start=hour_start,
            start_time=hour_start,
            end_time=hour_end,
        )
        logger.debug(f"Aggregated hour stats for {hour_start}")

    async def _aggregate_day_stats(self) -> None:
        """聚合天统计数据"""
        now = datetime.now()
        day_start = now.replace(hour=0, minute=0, second=0, microsecond=0) - timedelta(days=1)
        day_end = day_start + timedelta(days=1)

        await self._aggregate_period(
            source_type=self.PERIOD_HOUR,
            target_type=self.PERIOD_DAY,
            period_start=day_start,
            start_time=day_start,
            end_time=day_end,
        )
        logger.debug(f"Aggregated day stats for {day_start}")

    async def _aggregate_month_stats(self) -> None:
        """聚合月统计数据"""
        now = datetime.now()
        # 上个月的第一天
        first_day = now.replace(day=1, hour=0, minute=0, second=0, microsecond=0)
        month_start = (first_day - timedelta(days=1)).replace(day=1)
        month_end = first_day

        await self._aggregate_period(
            source_type=self.PERIOD_DAY,
            target_type=self.PERIOD_MONTH,
            period_start=month_start,
            start_time=month_start,
            end_time=month_end,
        )
        logger.debug(f"Aggregated month stats for {month_start}")

    async def _aggregate_period(
        self,
        source_type: str,
        target_type: str,
        period_start: datetime,
        start_time: datetime,
        end_time: datetime,
    ) -> None:
        """聚合某个时间段的统计数据"""
        sql = """
            SELECT project_id,
                   SUM(total) as total,
                   SUM(completed) as completed,
                   SUM(failed) as failed,
                   SUM(retried) as retried,
                   SUM(avg_speed) as avg_speed
            FROM spider_stats_history
            WHERE period_type = %s
              AND period_start >= %s
              AND period_start < %s
            GROUP BY project_id
        """

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (source_type, start_time, end_time))
                rows = await cursor.fetchall()

        for row in rows:
            project_id, total, completed, failed, retried, avg_speed = row
            if total and total > 0:
                await self._save_stats(
                    project_id=project_id,
                    period_type=target_type,
                    period_start=period_start,
                    total=int(total),
                    completed=int(completed or 0),
                    failed=int(failed or 0),
                    retried=int(retried or 0),
                    avg_speed=float(avg_speed or 0),
                )

    async def _save_stats(
        self,
        project_id: int,
        period_type: str,
        period_start: datetime,
        total: int,
        completed: int,
        failed: int,
        retried: int,
        avg_speed: float,
    ) -> None:
        """保存统计数据（使用 UPSERT）"""
        sql = """
            INSERT INTO spider_stats_history
            (project_id, period_type, period_start, total, completed, failed, retried, avg_speed)
            VALUES (%s, %s, %s, %s, %s, %s, %s, %s)
            ON DUPLICATE KEY UPDATE
                total = VALUES(total),
                completed = VALUES(completed),
                failed = VALUES(failed),
                retried = VALUES(retried),
                avg_speed = VALUES(avg_speed)
        """
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (
                    project_id, period_type, period_start,
                    total, completed, failed, retried, avg_speed
                ))
                await conn.commit()

    async def _cleanup_old_data(self, period_type: str, retention_days: int) -> None:
        """清理过期数据"""
        cutoff = datetime.now() - timedelta(days=retention_days)
        sql = """
            DELETE FROM spider_stats_history
            WHERE period_type = %s AND period_start < %s
        """
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (period_type, cutoff))
                deleted = cursor.rowcount
                await conn.commit()

        if deleted > 0:
            logger.info(f"Cleaned up {deleted} old {period_type} stats records")

    async def get_chart_data(
        self,
        project_id: int,
        period_type: str = PERIOD_HOUR,
        start: Optional[datetime] = None,
        end: Optional[datetime] = None,
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        """
        获取图表数据

        Args:
            project_id: 项目ID
            period_type: 周期类型（minute/hour/day/month）
            start: 开始时间
            end: 结束时间
            limit: 最大记录数

        Returns:
            统计数据列表
        """
        conditions = ["project_id = %s", "period_type = %s"]
        args = [project_id, period_type]

        if start:
            conditions.append("period_start >= %s")
            args.append(start)
        if end:
            conditions.append("period_start <= %s")
            args.append(end)

        where = " AND ".join(conditions)
        sql = f"""
            SELECT period_start, total, completed, failed, retried, avg_speed
            FROM spider_stats_history
            WHERE {where}
            ORDER BY period_start DESC
            LIMIT %s
        """
        args.append(limit)

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, args)
                rows = await cursor.fetchall()

        result = []
        for row in rows:
            period_start, total, completed, failed, retried, avg_speed = row
            total_done = (completed or 0) + (failed or 0)
            success_rate = round((completed or 0) / total_done * 100, 2) if total_done > 0 else 0

            result.append({
                'time': period_start.isoformat() if period_start else None,
                'total': total or 0,
                'completed': completed or 0,
                'failed': failed or 0,
                'retried': retried or 0,
                'avg_speed': float(avg_speed or 0),
                'success_rate': success_rate,
            })

        # 返回时间正序
        result.reverse()
        return result

    async def get_overview(
        self,
        project_id: Optional[int] = None,
        period_type: str = PERIOD_DAY,
        start: Optional[datetime] = None,
        end: Optional[datetime] = None,
    ) -> Dict[str, Any]:
        """
        获取统计概览（支持单项目或全部项目）

        Args:
            project_id: 项目ID，None 表示全部项目
            period_type: 周期类型（minute/hour/day/month）
            start: 开始时间
            end: 结束时间

        Returns:
            统计概览数据
        """
        conditions = ["period_type = %s"]
        args = [period_type]

        if project_id is not None:
            conditions.append("project_id = %s")
            args.append(project_id)
        if start:
            conditions.append("period_start >= %s")
            args.append(start)
        if end:
            conditions.append("period_start <= %s")
            args.append(end)

        where = " AND ".join(conditions)
        sql = f"""
            SELECT
                COALESCE(SUM(total), 0) as total,
                COALESCE(SUM(completed), 0) as completed,
                COALESCE(SUM(failed), 0) as failed,
                COALESCE(SUM(retried), 0) as retried,
                COALESCE(SUM(avg_speed), 0) as avg_speed
            FROM spider_stats_history
            WHERE {where}
        """

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, args)
                row = await cursor.fetchone()

        if not row:
            return {
                'total': 0,
                'completed': 0,
                'failed': 0,
                'retried': 0,
                'success_rate': 0,
                'avg_speed': 0,
            }

        total, completed, failed, retried, avg_speed = row
        total_done = (completed or 0) + (failed or 0)
        success_rate = round((completed or 0) / total_done * 100, 2) if total_done > 0 else 0

        return {
            'total': int(total or 0),
            'completed': int(completed or 0),
            'failed': int(failed or 0),
            'retried': int(retried or 0),
            'success_rate': success_rate,
            'avg_speed': round(float(avg_speed or 0), 2),
        }

    async def get_all_projects_chart_data(
        self,
        period_type: str = PERIOD_HOUR,
        start: Optional[datetime] = None,
        end: Optional[datetime] = None,
        limit: int = 100,
    ) -> List[Dict[str, Any]]:
        """
        获取全部项目汇总的图表数据

        Args:
            period_type: 周期类型（minute/hour/day/month）
            start: 开始时间
            end: 结束时间
            limit: 最大记录数

        Returns:
            统计数据列表
        """
        conditions = ["period_type = %s"]
        args = [period_type]

        if start:
            conditions.append("period_start >= %s")
            args.append(start)
        if end:
            conditions.append("period_start <= %s")
            args.append(end)

        where = " AND ".join(conditions)
        sql = f"""
            SELECT
                period_start,
                SUM(total) as total,
                SUM(completed) as completed,
                SUM(failed) as failed,
                SUM(retried) as retried,
                SUM(avg_speed) as avg_speed
            FROM spider_stats_history
            WHERE {where}
            GROUP BY period_start
            ORDER BY period_start DESC
            LIMIT %s
        """
        args.append(limit)

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, args)
                rows = await cursor.fetchall()

        result = []
        for row in rows:
            period_start, total, completed, failed, retried, avg_speed = row
            total_done = (completed or 0) + (failed or 0)
            success_rate = round((completed or 0) / total_done * 100, 2) if total_done > 0 else 0

            result.append({
                'time': period_start.isoformat() if period_start else None,
                'total': int(total or 0),
                'completed': int(completed or 0),
                'failed': int(failed or 0),
                'retried': int(retried or 0),
                'avg_speed': float(avg_speed or 0),
                'success_rate': success_rate,
            })

        # 返回时间正序
        result.reverse()
        return result

    async def get_stats_by_project(
        self,
        period_type: str = PERIOD_DAY,
        start: Optional[datetime] = None,
        end: Optional[datetime] = None,
    ) -> List[Dict[str, Any]]:
        """
        获取各项目的统计数据列表

        Args:
            period_type: 周期类型（minute/hour/day/month）
            start: 开始时间
            end: 结束时间

        Returns:
            各项目统计数据列表
        """
        conditions = ["h.period_type = %s"]
        args = [period_type]

        if start:
            conditions.append("h.period_start >= %s")
            args.append(start)
        if end:
            conditions.append("h.period_start <= %s")
            args.append(end)

        where = " AND ".join(conditions)
        sql = f"""
            SELECT
                h.project_id,
                p.name as project_name,
                SUM(h.total) as total,
                SUM(h.completed) as completed,
                SUM(h.failed) as failed,
                SUM(h.retried) as retried,
                SUM(h.avg_speed) as avg_speed
            FROM spider_stats_history h
            LEFT JOIN spider_projects p ON h.project_id = p.id
            WHERE {where}
            GROUP BY h.project_id, p.name
            ORDER BY total DESC
        """

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, args)
                rows = await cursor.fetchall()

        result = []
        for row in rows:
            project_id, project_name, total, completed, failed, retried, avg_speed = row
            total_done = (completed or 0) + (failed or 0)
            success_rate = round((completed or 0) / total_done * 100, 2) if total_done > 0 else 0

            result.append({
                'project_id': project_id,
                'project_name': project_name or f'项目 {project_id}',
                'total': int(total or 0),
                'completed': int(completed or 0),
                'failed': int(failed or 0),
                'retried': int(retried or 0),
                'success_rate': success_rate,
                'avg_speed': round(float(avg_speed or 0), 2),
            })

        return result
