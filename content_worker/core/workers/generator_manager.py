# -*- coding: utf-8 -*-
"""
数据加工管理器

管理多个 GeneratorWorker 协程，从数据库加载配置，监听命令。
"""

import asyncio
import json
import time
from datetime import datetime
from typing import Dict, List, Optional

from loguru import logger

from database.db import fetch_one, fetch_all, get_db_pool
from core.redis_client import get_redis_client
from core.workers.generator_worker import GeneratorWorker


class ProcessorLogger:
    """数据处理日志发布到 Redis，Go 订阅后推送给前端"""

    CHANNEL = "processor:logs"

    # 日志级别到 loguru 方法的映射
    _LEVEL_METHODS = {
        "INFO": logger.info,
        "WARNING": logger.warning,
        "ERROR": logger.error,
        "DEBUG": logger.debug,
    }

    def __init__(self, rdb):
        self.rdb = rdb

    async def _log(self, level: str, message: str):
        """统一的日志处理：发布到 Redis 并记录到本地"""
        # 发布到 Redis
        if self.rdb:
            log_data = {
                "type": "log",
                "level": level,
                "message": message,
                "timestamp": datetime.now().isoformat()
            }
            try:
                await self.rdb.publish(self.CHANNEL, json.dumps(log_data, ensure_ascii=False))
            except Exception:
                pass

        # 记录到本地日志
        log_method = self._LEVEL_METHODS.get(level, logger.info)
        log_method(message)

    async def info(self, msg: str):
        await self._log("INFO", msg)

    async def warning(self, msg: str):
        await self._log("WARNING", msg)

    async def error(self, msg: str):
        await self._log("ERROR", msg)

    async def debug(self, msg: str):
        await self._log("DEBUG", msg)


class GeneratorManager:
    """
    数据加工管理器

    负责：
    1. 从数据库加载配置
    2. 创建和管理多个 GeneratorWorker 协程
    3. 监听 processor:commands 频道接收命令
    4. 更新运行状态到 Redis
    """

    def __init__(self):
        self.workers: List[asyncio.Task] = []
        self.worker_instances: List[GeneratorWorker] = []
        self.config: Dict = {}
        self.running = False
        self.rdb = None
        self.db_pool = None
        self._stop_event = asyncio.Event()
        self._stats_task: Optional[asyncio.Task] = None
        # 速度计算
        self._last_processed_count = 0
        self._last_stats_time = None
        # 日志发布器
        self.log: Optional[ProcessorLogger] = None
        # 最后错误信息
        self._last_error: Optional[str] = None

    async def load_config(self) -> Dict:
        """从数据库加载配置"""
        config = {
            'enabled': True,
            'concurrency': 3,
            'retry_max': 3,
            'min_paragraph_length': 20,
            'batch_size': 50,
        }

        try:
            rows = await fetch_all(
                "SELECT setting_key, setting_value FROM system_settings WHERE setting_key LIKE 'processor.%'"
            )

            for row in rows:
                key = row['setting_key']
                value = row['setting_value']

                if key == 'processor.enabled':
                    config['enabled'] = value.lower() in ('true', '1', 'yes')
                elif key == 'processor.concurrency':
                    config['concurrency'] = int(value) if value.isdigit() else 3
                elif key == 'processor.retry_max':
                    config['retry_max'] = int(value) if value.isdigit() else 3
                elif key == 'processor.min_paragraph_length':
                    config['min_paragraph_length'] = int(value) if value.isdigit() else 20
                elif key == 'processor.batch_size':
                    config['batch_size'] = int(value) if value.isdigit() else 50

        except Exception as e:
            logger.warning(f"加载配置失败，使用默认值: {e}")

        return config

    async def start(self):
        """启动管理器"""
        self.rdb = get_redis_client()
        self.db_pool = get_db_pool()

        if not self.rdb:
            logger.error("Redis 未初始化，无法启动数据加工管理器")
            return

        # 初始化日志发布器
        self.log = ProcessorLogger(self.rdb)

        if not self.db_pool:
            await self.log.error("数据库未初始化，无法启动数据加工管理器")
            return

        # 加载配置
        self.config = await self.load_config()
        await self.log.info(f"数据加工管理器配置: {self.config}")

        if not self.config.get('enabled', True):
            await self.log.info("数据加工已禁用，不启动 Worker")
            # 仍然监听命令，以便可以通过命令启动
            await self.listen_commands()
            return

        # 启动 Workers
        await self._start_workers()

        # 启动统计更新任务
        self._stats_task = asyncio.create_task(self._update_stats_loop())

        # 监听命令
        await self.listen_commands()

    async def _start_workers(self):
        """启动 Worker 协程"""
        if self.running:
            await self.log.warning("Workers 已在运行中")
            return

        concurrency = self.config.get('concurrency', 3)
        await self.log.info(f"启动 {concurrency} 个数据加工 Worker...")

        self.running = True
        self._stop_event.clear()

        for i in range(concurrency):
            worker = GeneratorWorker(
                db_pool=self.db_pool,
                redis_client=self.rdb,
                batch_size=self.config.get('batch_size', 50),
                min_paragraph_length=self.config.get('min_paragraph_length', 20),
                retry_max=self.config.get('retry_max', 3),
                log=self.log,
                on_complete=self._publish_realtime_status,
            )
            self.worker_instances.append(worker)

            task = asyncio.create_task(
                self._run_worker(worker, i),
                name=f"generator_worker_{i}"
            )
            self.workers.append(task)

        # 更新状态
        await self._update_status()
        await self.log.info(f"已启动 {len(self.workers)} 个数据加工 Worker")

    async def _run_worker(self, worker: GeneratorWorker, index: int):
        """运行单个 Worker"""
        try:
            await worker.start()
            await worker.run_forever(stop_event=self._stop_event)
        except asyncio.CancelledError:
            logger.info(f"Worker {index} 被取消")
        except Exception as e:
            logger.error(f"Worker {index} 异常: {e}")
            self._last_error = str(e)
        finally:
            await worker.stop()

    async def stop(self):
        """停止所有 Worker"""
        if not self.running:
            return

        await self.log.info("正在停止数据加工 Workers...")
        self.running = False
        self._stop_event.set()

        # 取消所有任务
        for task in self.workers:
            if not task.done():
                task.cancel()

        # 等待任务完成
        if self.workers:
            await asyncio.gather(*self.workers, return_exceptions=True)

        self.workers.clear()
        self.worker_instances.clear()

        # 停止统计任务
        if self._stats_task and not self._stats_task.done():
            self._stats_task.cancel()
            try:
                await self._stats_task
            except asyncio.CancelledError:
                pass

        # 更新状态
        await self._update_status()
        await self.log.info("数据加工 Workers 已停止")

    async def reload_config(self):
        """重新加载配置"""
        old_config = self.config.copy()
        self.config = await self.load_config()

        await self.log.info(f"配置已重新加载: {self.config}")

        # 如果并发数改变，重启 Workers
        if old_config.get('concurrency') != self.config.get('concurrency'):
            await self.log.info("并发数已改变，重启 Workers...")
            await self.stop()
            if self.config.get('enabled', True):
                await self._start_workers()

        # 更新 Worker 配置
        for worker in self.worker_instances:
            worker.batch_size = self.config.get('batch_size', 50)
            worker.min_paragraph_length = self.config.get('min_paragraph_length', 20)
            worker.retry_max = self.config.get('retry_max', 3)

    async def listen_commands(self):
        """监听命令频道"""
        if not self.rdb:
            return

        logger.info("数据加工管理器开始监听命令...")

        pubsub = self.rdb.pubsub()
        await pubsub.subscribe("processor:commands")

        try:
            async for message in pubsub.listen():
                if message["type"] == "message":
                    try:
                        data = message["data"]
                        if isinstance(data, bytes):
                            data = data.decode('utf-8')
                        cmd = json.loads(data)
                        await self.handle_command(cmd)
                    except Exception as e:
                        logger.error(f"处理命令失败: {e}")
        except asyncio.CancelledError:
            pass
        finally:
            await pubsub.unsubscribe("processor:commands")

    async def handle_command(self, cmd: dict):
        """处理命令"""
        action = cmd.get("action")
        await self.log.info(f"收到数据加工命令: {action}")

        if action == "start":
            if not self.running:
                self.config = await self.load_config()
                await self._start_workers()
                if not self._stats_task or self._stats_task.done():
                    self._stats_task = asyncio.create_task(self._update_stats_loop())
            else:
                await self.log.info("Workers 已在运行中")

        elif action == "stop":
            await self.stop()

        elif action == "reload_config":
            await self.reload_config()

    async def _update_status(self):
        """更新运行状态到 Redis"""
        if not self.rdb:
            return

        status = {
            "running": "true" if self.running else "false",
            "workers": str(len(self.workers)),
            "updated_at": datetime.now().isoformat(),
        }

        try:
            await self.rdb.hset("processor:status", mapping=status)
        except Exception as e:
            logger.error(f"更新状态失败: {e}")

    async def _publish_realtime_status(self):
        """发布实时状态到 Redis 频道"""
        if not self.rdb:
            return

        try:
            # 查询队列长度
            queue_pending = await self.rdb.llen("pending:articles")
            queue_retry = await self.rdb.llen("pending:articles:retry")
            queue_dead = await self.rdb.llen("pending:articles:dead")

            # 汇总所有 Worker 统计
            total_processed = 0
            total_failed = 0
            for worker in self.worker_instances:
                stats = worker.get_stats()
                total_processed += stats.get('processed', 0)
                total_failed += stats.get('failed', 0)

            # 计算处理速度
            current_time = time.time()
            time_elapsed = current_time - self._last_stats_time if self._last_stats_time else 1
            processed_delta = total_processed - self._last_processed_count
            speed = processed_delta / time_elapsed if time_elapsed > 0 else 0.0

            # 获取今日处理量
            today_key = f"processor:processed:{datetime.now().strftime('%Y%m%d')}"
            today_count = await self.rdb.get(today_key)
            processed_today = int(today_count) if today_count else 0

            # 组装状态数据
            status = {
                "running": self.running,
                "workers": len(self.workers),
                "queue_pending": queue_pending,
                "queue_retry": queue_retry,
                "queue_dead": queue_dead,
                "processed_total": total_processed,
                "processed_today": processed_today,
                "speed": round(speed, 2),
                "last_error": self._last_error
            }

            # 发布到 Redis 频道
            await self.rdb.publish(
                "processor:status:realtime",
                json.dumps(status, ensure_ascii=False)
            )

        except Exception as e:
            logger.error(f"发布实时状态失败: {e}")

    async def _update_stats_loop(self):
        """定期更新统计信息到 Redis"""
        self._last_stats_time = time.time()
        self._last_processed_count = 0

        while self.running:
            try:
                current_time = time.time()

                # 汇总所有 Worker 的统计
                total_processed = 0
                total_failed = 0
                total_retried = 0
                total_processing_time_ms = 0.0

                for worker in self.worker_instances:
                    stats = worker.get_stats()
                    total_processed += stats.get('processed', 0)
                    total_failed += stats.get('failed', 0)
                    total_retried += stats.get('retried', 0)
                    total_processing_time_ms += stats.get('total_processing_time_ms', 0.0)

                # 计算处理速度（条/秒）
                time_elapsed = current_time - self._last_stats_time
                processed_delta = total_processed - self._last_processed_count
                speed = processed_delta / time_elapsed if time_elapsed > 0 else 0.0

                # 更新上次记录
                self._last_stats_time = current_time
                self._last_processed_count = total_processed

                # 计算平均处理时间
                avg_processing_ms = 0.0
                if total_processed > 0:
                    avg_processing_ms = total_processing_time_ms / total_processed

                # 获取今日处理量（从 Redis）
                today_key = f"processor:processed:{datetime.now().strftime('%Y%m%d')}"
                today_count = await self.rdb.get(today_key)
                processed_today = int(today_count) if today_count else 0

                # 1. 更新 processor:status（状态信息）
                status = {
                    "running": "true" if self.running else "false",
                    "workers": str(len(self.workers)),
                    "processed_total": str(total_processed),
                    "processed_today": str(processed_today),
                    "speed": f"{speed:.2f}",
                    "updated_at": datetime.now().isoformat(),
                }
                await self.rdb.hset("processor:status", mapping=status)

                # 2. 更新 processor:stats（统计信息）
                stats_data = {
                    "total_processed": str(total_processed),
                    "total_failed": str(total_failed),
                    "total_retried": str(total_retried),
                    "avg_processing_ms": f"{avg_processing_ms:.2f}",
                    "updated_at": datetime.now().isoformat(),
                }
                await self.rdb.hset("processor:stats", mapping=stats_data)

                # 每 5 秒更新一次
                await asyncio.sleep(5)

            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"更新统计失败: {e}")
                await asyncio.sleep(5)

    def get_status(self) -> Dict:
        """获取当前状态"""
        return {
            'running': self.running,
            'workers': len(self.workers),
            'config': self.config,
        }
