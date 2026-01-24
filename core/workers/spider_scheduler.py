# -*- coding: utf-8 -*-
"""
爬虫调度 Worker

基于 APScheduler 实现定时调度，读取项目的 schedule JSON 配置并自动执行。
"""

import json
import asyncio
from typing import Optional, Dict, Any, TYPE_CHECKING

from apscheduler.schedulers.asyncio import AsyncIOScheduler
from apscheduler.triggers.interval import IntervalTrigger
from apscheduler.triggers.cron import CronTrigger
from loguru import logger

if TYPE_CHECKING:
    from aiomysql import Pool
    from redis.asyncio import Redis


# 全局调度器实例（单例）
_scheduler_instance: Optional['SpiderSchedulerWorker'] = None


def get_scheduler() -> Optional['SpiderSchedulerWorker']:
    """获取全局调度器实例"""
    return _scheduler_instance


class SpiderSchedulerWorker:
    """
    爬虫定时调度 Worker

    功能：
    - 启动时加载所有 enabled=1 且 schedule 不为空的项目
    - 解析 JSON 配置，注册 APScheduler 任务
    - 监听项目变更（新增/修改/删除）自动更新调度
    - 执行时调用已有的 run_project_task() 函数
    """

    def __init__(self, db_pool: 'Pool', redis: 'Redis'):
        """
        初始化调度器

        Args:
            db_pool: MySQL 连接池
            redis: Redis 客户端
        """
        global _scheduler_instance

        self.db_pool = db_pool
        self.redis = redis
        self.scheduler = AsyncIOScheduler(timezone='Asia/Shanghai')
        self._running = False

        # 注册为全局实例
        _scheduler_instance = self

    async def start(self) -> None:
        """启动调度器"""
        self._running = True

        # 加载所有调度配置
        await self._load_all_schedules()

        # 启动 APScheduler
        self.scheduler.start()

        job_count = len(self.scheduler.get_jobs())
        logger.info(f"Spider scheduler started with {job_count} scheduled jobs")

    async def stop(self) -> None:
        """停止调度器"""
        global _scheduler_instance

        self._running = False
        self.scheduler.shutdown(wait=False)
        _scheduler_instance = None

        logger.info("Spider scheduler stopped")

    async def _load_all_schedules(self) -> None:
        """加载所有项目的调度配置"""
        sql = """
            SELECT id, name, schedule
            FROM spider_projects
            WHERE enabled = 1 AND schedule IS NOT NULL AND schedule != ''
        """

        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql)
                rows = await cursor.fetchall()

        loaded_count = 0
        for row in rows:
            project_id, name, schedule_json = row
            try:
                if self._add_job(project_id, schedule_json):
                    loaded_count += 1
                    logger.debug(f"Loaded schedule for project '{name}' (id={project_id})")
            except Exception as e:
                logger.warning(f"Failed to load schedule for project {project_id}: {e}")

        logger.info(f"Loaded {loaded_count} scheduled projects")

    def _add_job(self, project_id: int, schedule_json: str) -> bool:
        """
        解析 JSON 并添加调度任务

        Args:
            project_id: 项目 ID
            schedule_json: 调度配置 JSON

        Returns:
            是否成功添加任务
        """
        if not schedule_json:
            return False

        try:
            config = json.loads(schedule_json)
        except json.JSONDecodeError:
            logger.warning(f"Invalid schedule JSON for project {project_id}")
            return False

        schedule_type = config.get('type', 'none')
        if schedule_type == 'none':
            return False

        trigger = self._create_trigger(config)
        if not trigger:
            return False

        job_id = f"spider_{project_id}"

        # 移除可能存在的旧任务
        existing_job = self.scheduler.get_job(job_id)
        if existing_job:
            self.scheduler.remove_job(job_id)

        # 添加新任务
        self.scheduler.add_job(
            self._execute_project,
            trigger=trigger,
            id=job_id,
            args=[project_id],
            name=f"Spider Project {project_id}",
            replace_existing=True,
            max_instances=1,  # 防止同一项目并发执行
            coalesce=True,    # 错过的执行合并为一次
        )

        return True

    def _create_trigger(self, config: Dict[str, Any]):
        """
        根据配置类型创建 APScheduler Trigger

        Args:
            config: 调度配置字典

        Returns:
            APScheduler Trigger 实例，或 None
        """
        schedule_type = config.get('type', 'none')

        if schedule_type == 'interval_minutes':
            interval = config.get('interval', 30)
            return IntervalTrigger(minutes=interval)

        if schedule_type == 'interval_hours':
            interval = config.get('interval', 1)
            return IntervalTrigger(hours=interval)

        if schedule_type == 'daily':
            time_str = config.get('time', '08:00')
            hour, minute = map(int, time_str.split(':'))
            return CronTrigger(hour=hour, minute=minute)

        if schedule_type == 'weekly':
            time_str = config.get('time', '09:00')
            hour, minute = map(int, time_str.split(':'))
            days = config.get('days', [])
            if not days:
                return None
            # APScheduler 使用 0=Monday, 6=Sunday
            # 前端 JSON 使用 0=Sunday, 1=Monday, ..., 6=Saturday
            # 转换：前端 0 -> APScheduler 6, 前端 1 -> APScheduler 0, ...
            ap_days = [(d - 1) % 7 if d > 0 else 6 for d in days]
            day_of_week = ','.join(str(d) for d in sorted(set(ap_days)))
            return CronTrigger(day_of_week=day_of_week, hour=hour, minute=minute)

        if schedule_type == 'monthly':
            time_str = config.get('time', '10:00')
            hour, minute = map(int, time_str.split(':'))
            dates = config.get('dates', [])
            if not dates:
                return None
            day = ','.join(str(d) for d in sorted(dates))
            return CronTrigger(day=day, hour=hour, minute=minute)

        return None

    async def _execute_project(self, project_id: int) -> None:
        """
        执行爬虫项目

        Args:
            project_id: 项目 ID
        """
        from database.db import fetch_one, execute_query

        # 检查项目状态
        sql = "SELECT id, name, status, enabled FROM spider_projects WHERE id = %s"
        async with self.db_pool.acquire() as conn:
            async with conn.cursor() as cursor:
                await cursor.execute(sql, (project_id,))
                row = await cursor.fetchone()

        if not row:
            logger.warning(f"Scheduled project {project_id} not found, removing from scheduler")
            self.remove_schedule(project_id)
            return

        project_id, name, status, enabled = row

        # 检查是否启用
        if not enabled:
            logger.debug(f"Scheduled project '{name}' is disabled, skipping")
            return

        # 检查是否已在运行
        if status == 'running':
            logger.warning(f"Scheduled project '{name}' is already running, skipping")
            return

        logger.info(f"Scheduled execution started: '{name}' (id={project_id})")

        try:
            # 导入并执行任务
            from api.spider_routes import run_project_task
            from core.crawler.log_manager import log_manager

            # 创建日志会话
            session_id = f"project_{project_id}"
            log_manager.create_session(session_id)
            log_manager.add_log(session_id, "INFO", "定时任务触发，开始执行...")

            # 更新状态为运行中
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        "UPDATE spider_projects SET status = 'running' WHERE id = %s",
                        (project_id,)
                    )
                    await conn.commit()

            # 执行任务
            await run_project_task(project_id)

            logger.info(f"Scheduled execution completed: '{name}' (id={project_id})")

        except Exception as e:
            logger.error(f"Scheduled execution failed for project {project_id}: {e}")

            # 更新状态为错误
            try:
                async with self.db_pool.acquire() as conn:
                    async with conn.cursor() as cursor:
                        await cursor.execute(
                            "UPDATE spider_projects SET status = 'error', last_error = %s WHERE id = %s",
                            (str(e), project_id)
                        )
                        await conn.commit()
            except Exception:
                pass

    def update_schedule(self, project_id: int, schedule_json: Optional[str], enabled: int = 1) -> None:
        """
        项目配置变更时更新调度

        Args:
            project_id: 项目 ID
            schedule_json: 新的调度配置 JSON（None 或空字符串表示删除调度）
            enabled: 项目是否启用
        """
        job_id = f"spider_{project_id}"

        # 先移除旧任务
        existing_job = self.scheduler.get_job(job_id)
        if existing_job:
            self.scheduler.remove_job(job_id)
            logger.debug(f"Removed old schedule for project {project_id}")

        # 如果有新配置且项目启用，添加新任务
        if schedule_json and enabled:
            if self._add_job(project_id, schedule_json):
                logger.info(f"Updated schedule for project {project_id}")
            else:
                logger.debug(f"No valid schedule config for project {project_id}")

    def remove_schedule(self, project_id: int) -> None:
        """
        删除项目调度

        Args:
            project_id: 项目 ID
        """
        job_id = f"spider_{project_id}"
        existing_job = self.scheduler.get_job(job_id)
        if existing_job:
            self.scheduler.remove_job(job_id)
            logger.info(f"Removed schedule for project {project_id}")

    def get_scheduled_projects(self) -> list:
        """
        获取所有已调度的项目信息

        Returns:
            已调度项目列表
        """
        jobs = self.scheduler.get_jobs()
        result = []

        for job in jobs:
            if job.id.startswith('spider_'):
                project_id = int(job.id.replace('spider_', ''))
                next_run = job.next_run_time
                result.append({
                    'project_id': project_id,
                    'job_id': job.id,
                    'next_run': next_run.isoformat() if next_run else None,
                    'trigger': str(job.trigger),
                })

        return result
