# -*- coding: utf-8 -*-
"""
Python Worker 命令监听器

监听 Go API 发送的 Redis 命令，执行爬虫任务。
复用现有的 ProjectLoader 和 ProjectRunner。
"""

import asyncio
import json
from datetime import datetime
from typing import Dict, Optional

from loguru import logger

from config import settings
from database.db import fetch_one, insert, execute_query, get_db_pool
from core.redis_client import get_redis_client
from core.realtime_logger import RealtimeContext, send_end, init_realtime_sink


class CommandListener:
    """监听 Go 发来的命令"""

    def __init__(self):
        self.running_tasks: Dict[int, asyncio.Task] = {}
        self.rdb = None

    async def _publish_stats(self, project_id: int, items_count: int):
        """发布实时统计更新到前端"""
        # 更新 Redis 计数
        stats_key = f"spider:{project_id}:stats"
        await self.rdb.hincrby(stats_key, "completed", 1)

        # 发布统计消息（前端 WebSocket 订阅）
        stats_msg = {
            "type": "stats",
            "project_id": project_id,
            "items_count": items_count,
            "timestamp": datetime.now().isoformat()
        }
        await self.rdb.publish(
            f"spider:stats:project_{project_id}",
            json.dumps(stats_msg, ensure_ascii=False)
        )

    async def _batch_insert_keywords(self, keywords: list, group_id: int) -> int:
        """批量插入关键词到数据库（INSERT IGNORE 去重）"""
        if not keywords:
            return 0

        try:
            db_pool = get_db_pool()
            if not db_pool:
                return 0

            async with db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.executemany(
                        "INSERT IGNORE INTO keywords (group_id, keyword) VALUES (%s, %s)",
                        [(group_id, kw) for kw in keywords]
                    )
                    await conn.commit()
                    return cursor.rowcount
        except Exception as e:
            logger.error(f"批量插入关键词失败: {e}")
            return 0

    async def _batch_insert_images(self, urls: list, group_id: int) -> int:
        """批量插入图片URL到数据库（INSERT IGNORE 去重）"""
        if not urls:
            return 0

        try:
            db_pool = get_db_pool()
            if not db_pool:
                return 0

            async with db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.executemany(
                        "INSERT IGNORE INTO images (group_id, url, status) VALUES (%s, %s, 1)",
                        [(group_id, url) for url in urls]
                    )
                    await conn.commit()
                    return cursor.rowcount
        except Exception as e:
            logger.error(f"批量插入图片失败: {e}")
            return 0

    async def start(self):
        """启动监听器"""
        self.rdb = get_redis_client()
        if not self.rdb:
            logger.error("Redis 未初始化，无法启动命令监听器")
            return

        # 初始化全局 Redis sink
        init_realtime_sink()

        logger.info("命令监听器已启动，等待命令...")

        pubsub = self.rdb.pubsub()
        await pubsub.subscribe(
            settings.channels.spider_commands,
            settings.channels.worker_command
        )

        async for message in pubsub.listen():
            if message["type"] == "message":
                try:
                    data = message["data"]
                    if isinstance(data, bytes):
                        data = data.decode('utf-8')

                    # 检查是否是简单字符串命令（如 restart）
                    if data == "restart":
                        await self.handle_restart()
                    else:
                        cmd = json.loads(data)
                        await self.handle_command(cmd)
                except json.JSONDecodeError:
                    # 可能是简单字符串命令
                    if data == "restart":
                        await self.handle_restart()
                except Exception as e:
                    logger.error(f"处理命令失败: {e}")

    async def handle_restart(self):
        """处理重启命令"""
        logger.info("收到重启指令，正在准备退出...")

        # 等待当前任务完成
        for project_id, task in list(self.running_tasks.items()):
            if not task.done():
                logger.info(f"等待项目 {project_id} 任务完成...")
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass

        logger.info("所有任务已完成，退出进程...")
        # 退出进程，Docker 会自动重启
        import sys
        sys.exit(0)

    async def handle_command(self, cmd: dict):
        """处理命令"""
        action = cmd.get("action")
        project_id = cmd.get("project_id")

        logger.info(f"收到命令: {action} for project {project_id}")

        if action == "run":
            # 如果已有运行中的任务，先取消
            if project_id in self.running_tasks:
                old_task = self.running_tasks[project_id]
                if not old_task.done():
                    old_task.cancel()

            task = asyncio.create_task(self.run_project(project_id))
            self.running_tasks[project_id] = task

        elif action == "test":
            max_items = cmd.get("max_items", 0)
            if project_id in self.running_tasks:
                old_task = self.running_tasks[project_id]
                if not old_task.done():
                    old_task.cancel()

            task = asyncio.create_task(self.test_project(project_id, max_items))
            self.running_tasks[project_id] = task

        elif action == "stop":
            await self.stop_project(project_id)

        elif action == "test_stop":
            await self.stop_test(project_id)

        elif action == "pause":
            await self.pause_project(project_id)

        elif action == "resume":
            await self.resume_project(project_id)

    async def run_project(self, project_id: int):
        """运行爬虫项目（主入口，只做流程编排）"""
        channel = f"spider:logs:project_{project_id}"

        async with RealtimeContext(self.rdb, channel) as ctx:
            items_count = 0

            try:
                # 更新状态
                await self.rdb.set(
                    f"spider:status:{project_id}",
                    json.dumps({"status": "running", "started_at": datetime.now().isoformat()})
                )

                # 加载项目
                project = await self._load_project(project_id)
                if not project:
                    return

                # 执行并处理数据
                items_count = await self._run_and_process(project)

                # 更新成功统计
                await execute_query(
                    """
                    UPDATE spider_projects SET
                        status = 'idle',
                        last_run_at = NOW(),
                        last_run_items = %s,
                        last_error = NULL,
                        total_runs = total_runs + 1,
                        total_items = total_items + %s
                    WHERE id = %s
                    """,
                    (items_count, items_count, project_id),
                    commit=True
                )
                logger.info(f"任务完成：共 {items_count} 条数据")

            except asyncio.CancelledError:
                logger.info("任务已被取消")
                await execute_query(
                    "UPDATE spider_projects SET status = 'idle', last_error = %s WHERE id = %s",
                    ("任务被取消", project_id),
                    commit=True
                )

            except Exception as e:
                logger.error(f"任务异常: {str(e)}")
                await execute_query(
                    "UPDATE spider_projects SET status = 'error', last_error = %s WHERE id = %s",
                    (str(e), project_id),
                    commit=True
                )

            finally:
                await self.rdb.set(
                    f"spider:status:{project_id}",
                    json.dumps({"status": "idle"})
                )
                self.running_tasks.pop(project_id, None)

    async def _load_project(self, project_id: int) -> Optional[dict]:
        """加载项目配置和模块"""
        from core.crawler.project_loader import ProjectLoader

        logger.info("正在加载项目...")

        row = await fetch_one(
            "SELECT id, name, entry_file, config, concurrency, output_group_id FROM spider_projects WHERE id = %s",
            (project_id,)
        )
        if not row:
            logger.error("项目不存在")
            return None

        config = json.loads(row['config']) if row['config'] else {}

        loader = ProjectLoader(project_id)
        modules = await loader.load()
        logger.info(f"已加载 {len(modules)} 个模块")

        # 清除旧的停止信号
        await self.rdb.delete(f"spider_project:{project_id}:stop")

        return {
            "id": project_id,
            "config": config,
            "modules": modules,
            "concurrency": row.get('concurrency', 3),
            "group_id": row['output_group_id'],
        }

    async def _run_and_process(self, project: dict) -> int:
        """执行爬虫并处理数据"""
        from core.crawler.project_runner import ProjectRunner

        runner = ProjectRunner(
            project_id=project["id"],
            modules=project["modules"],
            config=project["config"],
            redis=self.rdb,
            db_pool=get_db_pool(),
            concurrency=project["concurrency"],
        )

        logger.info("开始执行 Spider...")

        items_count = 0
        stop_key = f"spider_project:{project['id']}:stop"

        async for item in runner.run():
            # 检查停止信号
            if await self.rdb.get(stop_key):
                logger.info("收到停止信号，任务终止")
                await self.rdb.delete(stop_key)
                break

            # 处理数据项
            count = await self._process_item(item, project["group_id"], project["id"])
            items_count += count

            if items_count > 0 and items_count % 10 == 0:
                logger.info(f"已抓取 {items_count} 条数据")

        return items_count

    async def _process_item(self, item: dict, group_id: int, project_id: int) -> int:
        """处理单个数据项（路由到 keywords/images/article）"""
        item_type = item.get('type', 'article')

        try:
            if item_type == 'keywords':
                keywords = item.get('keywords', [])
                target_group = item.get('group_id', group_id)
                if keywords:
                    added = await self._batch_insert_keywords(keywords, target_group)
                    logger.info(f"关键词写入: 新增 {added}, 跳过 {len(keywords) - added}")
                    if added > 0:
                        await self._publish_stats(project_id, added)
                    return added

            elif item_type == 'images':
                urls = item.get('urls', [])
                target_group = item.get('group_id', group_id)
                if urls:
                    added = await self._batch_insert_images(urls, target_group)
                    logger.info(f"图片写入: 新增 {added}, 跳过 {len(urls) - added}")
                    if added > 0:
                        await self._publish_stats(project_id, added)
                    return added

            else:
                # article 类型
                if item.get('title') and item.get('content'):
                    target_group = item.get('group_id', group_id)
                    article_id = await insert("original_articles", {
                        "group_id": target_group,
                        "source_id": project_id,
                        "source_url": item.get('source_url'),
                        "title": item['title'][:500],
                        "content": item['content'],
                    })

                    await self._publish_stats(project_id, 1)

                    if article_id:
                        try:
                            await self.rdb.lpush(settings.queues.pending, article_id)
                        except Exception as queue_err:
                            logger.warning(f"推送到待处理队列失败: {queue_err}")

                    return 1

        except Exception as e:
            if 'Duplicate' in str(e):
                logger.warning("数据重复，已跳过")
            else:
                logger.error(f"保存数据失败: {e}")

        return 0

    async def test_project(self, project_id: int, max_items: int = 0):
        """测试运行项目"""
        from core.crawler.project_runner import ProjectRunner
        from core.crawler.request_queue import RequestQueue

        channel = f"spider:logs:test_{project_id}"

        async with RealtimeContext(self.rdb, channel) as ctx:
            try:
                limit_text = f"最多 {max_items} 条" if max_items > 0 else "不限制条数"
                logger.info(f"开始测试运行（{limit_text}）...")

                # 清除测试队列
                queue = RequestQueue(self.rdb, project_id, is_test=True)
                await queue.clear()

                # 复用加载逻辑
                project = await self._load_project(project_id)
                if not project:
                    return

                runner = ProjectRunner(
                    project_id=project["id"],
                    modules=project["modules"],
                    config=project["config"],
                    redis=self.rdb,
                    db_pool=get_db_pool(),
                    concurrency=project["concurrency"],
                    is_test=True,
                    max_items=max_items,
                )

                items_count = 0
                async for item in runner.run():
                    # 检查停止信号
                    state = await queue.get_state()
                    if state == RequestQueue.STATE_STOPPED:
                        logger.info("测试已停止")
                        break

                    if not item.get('title') or not item.get('content'):
                        logger.warning("数据缺少必填字段(title 或 content)，已跳过")
                        continue

                    items_count += 1

                    # 输出数据预览
                    title = item.get('title', '(无标题)')[:50]
                    if max_items > 0:
                        logger.info(f"[{items_count}/{max_items}] {title}")
                    else:
                        logger.info(f"[{items_count}] {title}")

                    # 发送数据项供前端展示
                    await ctx.item(item)

                    if max_items > 0 and items_count >= max_items:
                        break

                logger.info(f"测试完成：共 {items_count} 条数据")

            except asyncio.CancelledError:
                logger.info("测试已被取消")

            except Exception as e:
                logger.error(f"测试异常: {str(e)}")

            finally:
                self.running_tasks.pop(project_id, None)

    async def stop_project(self, project_id: int):
        """停止项目"""
        from core.crawler.request_queue import RequestQueue

        stop_key = f"spider_project:{project_id}:stop"
        await self.rdb.set(stop_key, "1", ex=3600)

        queue = RequestQueue(self.rdb, project_id)
        await queue.stop()

        # 取消任务
        if project_id in self.running_tasks:
            task = self.running_tasks[project_id]
            if not task.done():
                task.cancel()

    async def stop_test(self, project_id: int):
        """停止测试"""
        from core.crawler.request_queue import RequestQueue

        queue = RequestQueue(self.rdb, project_id, is_test=True)
        await queue.stop(clear_queue=True)

        # 取消任务
        if project_id in self.running_tasks:
            task = self.running_tasks[project_id]
            if not task.done():
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass

        # 主动发送结束消息，确保前端收到
        await send_end(self.rdb, f"spider:logs:test_{project_id}")

    async def pause_project(self, project_id: int):
        """暂停项目"""
        from core.crawler.request_queue import RequestQueue

        queue = RequestQueue(self.rdb, project_id)
        await queue.pause()

    async def resume_project(self, project_id: int):
        """恢复项目"""
        from core.crawler.request_queue import RequestQueue

        queue = RequestQueue(self.rdb, project_id)
        await queue.resume()

    async def stop(self):
        """停止监听器，取消所有运行中的任务"""
        logger.info("正在停止命令监听器...")

        # 取消所有运行中的任务
        for project_id, task in list(self.running_tasks.items()):
            if not task.done():
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass

        self.running_tasks.clear()
        logger.info("命令监听器已停止")
