# -*- coding: utf-8 -*-
"""
正文生成工作进程

负责从 Redis 待处理队列取出文章ID，从 original_articles 表查询原始数据，
处理后写入 titles 和 contents 表，并将生成的段落 ID 加入可用池。

高性能设计：
- Redis BRPOP 取待处理ID：O(1)
- MySQL 主键查询原文：O(1)
- 无论表多大都是毫秒级响应

重试机制：
- 处理失败的文章放入重试队列
- 超过最大重试次数放入死信队列
"""

import asyncio
import time
from datetime import datetime
from typing import Any, Dict, List, Optional
from loguru import logger

from core.dedup import ContentDeduplicator
from core.processors import PinyinAnnotator, TextCleaner


class GeneratorWorker:
    """
    正文生成工作进程（高性能版）

    工作流程：
    1. 从 Redis 队列 pending:articles 取文章ID（BRPOP，O(1)）
    2. 按主键查询 original_articles 表获取原始数据（O(1)）
    3. 提取标题 → 写入 titles 表
    4. 拆分正文 → 拼音标注 → 写入 contents 表
    5. 失败时放入重试队列，超过重试次数放入死信队列
    """

    # 队列键名
    QUEUE_PENDING = "pending:articles"
    QUEUE_RETRY = "pending:articles:retry"
    QUEUE_DEAD = "pending:articles:dead"

    def __init__(
        self,
        db_pool,
        redis_client,
        dedup: Optional[ContentDeduplicator] = None,
        batch_size: int = 50,
        min_paragraph_length: int = 20,
        retry_max: int = 3,
        on_complete=None,
    ):
        """
        初始化正文生成工作进程

        Args:
            db_pool: MySQL 连接池
            redis_client: Redis 客户端
            dedup: 内容去重器（可选，用于标题去重）
            batch_size: 批量写入大小
            min_paragraph_length: 段落最小长度
            retry_max: 最大重试次数
            on_complete: 任务完成回调
        """
        self.db_pool = db_pool
        self.redis = redis_client
        self.dedup = dedup
        self.batch_size = batch_size
        self.min_paragraph_length = min_paragraph_length
        self.retry_max = retry_max
        self.on_complete = on_complete

        # 文本处理器（延迟初始化）
        self.annotator = None
        self.cleaner = None

        self._running = False
        self._title_buffer: List[Dict[str, Any]] = []
        self._content_buffer: List[Dict[str, Any]] = []

        # 统计
        self._processed_count = 0
        self._failed_count = 0
        self._retried_count = 0
        self._total_processing_time_ms = 0.0  # 累计处理时间（毫秒）

    async def start(self):
        """启动工作进程"""
        if self._running:
            return

        self._running = True

        # 延迟初始化处理器
        if self.annotator is None:
            self.annotator = PinyinAnnotator()
        if self.cleaner is None:
            self.cleaner = TextCleaner(min_length=self.min_paragraph_length)

        # 初始化去重器
        if self.dedup is None:
            self.dedup = ContentDeduplicator(self.redis)
        await self.dedup.init()

        logger.info("Generator worker started")

    async def stop(self):
        """停止工作进程"""
        self._running = False

        # 刷新缓冲区
        await self._flush_title_buffer()
        await self._flush_content_buffer()

        # 清理去重器
        if self.dedup:
            await self.dedup.cleanup()

        logger.info(f"Generator worker stopped (processed: {self._processed_count}, failed: {self._failed_count}, retried: {self._retried_count})")

    # ============================================
    # 核心方法：从 Redis 队列获取待处理文章
    # ============================================

    async def get_article_by_id(self, article_id: int) -> Optional[Dict[str, Any]]:
        """
        按主键查询原始文章（O(1) 性能）

        Args:
            article_id: 文章ID

        Returns:
            {'id': int, 'title': str, 'content': str, 'group_id': int} 或 None
        """
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        """
                        SELECT id, title, content, group_id
                        FROM original_articles
                        WHERE id = %s
                        """,
                        (article_id,)
                    )
                    row = await cursor.fetchone()
                    if row:
                        return {
                            'id': row[0],
                            'title': row[1],
                            'content': row[2],
                            'group_id': row[3]
                        }
            return None

        except Exception as e:
            logger.error(f"Failed to get article {article_id}: {e}")
            return None

    async def pop_article_id(self, timeout: int = 5) -> Optional[int]:
        """
        从 Redis 队列取出待处理文章ID（BRPOP，O(1)）

        优先从主队列取，主队列为空时尝试从重试队列取

        Args:
            timeout: 等待超时（秒）

        Returns:
            文章ID 或 None（超时）
        """
        try:
            # 优先从主队列取
            result = await self.redis.brpop(
                [self.QUEUE_PENDING, self.QUEUE_RETRY],
                timeout=timeout
            )
            if result:
                queue_name, article_id = result
                if isinstance(article_id, bytes):
                    article_id = article_id.decode('utf-8')
                return int(article_id)
            return None

        except asyncio.CancelledError:
            raise
        except Exception as e:
            logger.error(f"Failed to pop article from queue: {e}")
            return None

    async def get_pending_count(self) -> int:
        """获取待处理队列长度"""
        try:
            return await self.redis.llen(self.QUEUE_PENDING)
        except Exception as e:
            logger.error(f"Failed to get queue size: {e}")
            return 0

    # ============================================
    # 重试逻辑
    # ============================================

    async def get_retry_count(self, article_id: int) -> int:
        """获取文章的重试次数"""
        try:
            key = f"processor:retry:{article_id}"
            count = await self.redis.get(key)
            return int(count) if count else 0
        except Exception:
            return 0

    async def incr_retry_count(self, article_id: int) -> int:
        """增加文章的重试次数"""
        try:
            key = f"processor:retry:{article_id}"
            count = await self.redis.incr(key)
            # 设置过期时间（1天）
            await self.redis.expire(key, 86400)
            return count
        except Exception:
            return 0

    async def clear_retry_count(self, article_id: int):
        """清除文章的重试计数"""
        try:
            key = f"processor:retry:{article_id}"
            await self.redis.delete(key)
        except Exception:
            pass

    async def handle_failure(self, article_id: int, error: str):
        """处理失败的文章"""
        retry_count = await self.get_retry_count(article_id)

        if retry_count < self.retry_max:
            # 放入重试队列
            await self.incr_retry_count(article_id)
            await self.redis.lpush(self.QUEUE_RETRY, article_id)
            self._retried_count += 1
            logger.warning(f"Article {article_id} failed (retry {retry_count + 1}/{self.retry_max}): {error}")
        else:
            # 超过重试次数，放入死信队列
            await self.redis.lpush(self.QUEUE_DEAD, article_id)
            await self.clear_retry_count(article_id)
            self._failed_count += 1
            logger.error(f"Article {article_id} moved to dead queue after {self.retry_max} retries: {error}")

    # ============================================
    # 数据写入方法
    # ============================================

    async def get_next_batch_id(self, group_id: int, table: str = 'contents') -> int:
        """获取下一个 batch_id"""
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        f"""
                        SELECT COALESCE(MAX(batch_id), 0) + 1
                        FROM {table}
                        WHERE group_id = %s
                        """,
                        (group_id,)
                    )
                    row = await cursor.fetchone()
                    return row[0] if row else 1

        except Exception as e:
            logger.error(f"Failed to get next batch_id: {e}")
            return 1

    async def save_title(self, title: str, group_id: int) -> bool:
        """
        保存标题到 titles 表

        Args:
            title: 标题文本
            group_id: 分组ID

        Returns:
            是否保存成功
        """
        if not title or not title.strip():
            return False

        # 去重检查
        if self.dedup and not self.dedup.should_save_title(title):
            return False

        # 添加到缓冲区
        self._title_buffer.append({
            'title': title.strip(),
            'group_id': group_id
        })

        # 检查是否需要刷新
        if len(self._title_buffer) >= self.batch_size:
            await self._flush_title_buffer()

        return True

    async def _flush_title_buffer(self):
        """刷新标题缓冲区"""
        if not self._title_buffer:
            return

        try:
            # 按 group_id 分组
            groups: Dict[int, List[str]] = {}
            for item in self._title_buffer:
                gid = item['group_id']
                if gid not in groups:
                    groups[gid] = []
                groups[gid].append(item['title'])

            count = 0
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    for group_id, titles in groups.items():
                        batch_id = await self.get_next_batch_id(group_id, 'titles')
                        values = [(t, group_id, batch_id) for t in titles]

                        # INSERT IGNORE 避免重复
                        await cursor.executemany(
                            """
                            INSERT IGNORE INTO titles (title, group_id, batch_id)
                            VALUES (%s, %s, %s)
                            """,
                            values
                        )
                        count += cursor.rowcount

                await conn.commit()

            self._title_buffer.clear()
            logger.debug(f"Flushed {count} titles to database")

        except Exception as e:
            logger.error(f"Failed to flush title buffer: {e}")

    async def save_content(self, content: str, group_id: int) -> bool:
        """
        保存正文到 contents 表

        Args:
            content: 正文文本（已标注拼音）
            group_id: 分组ID

        Returns:
            是否保存成功
        """
        if not content or not content.strip():
            return False

        # 添加到缓冲区
        self._content_buffer.append({
            'content': content.strip(),
            'group_id': group_id
        })

        # 检查是否需要刷新
        if len(self._content_buffer) >= self.batch_size:
            await self._flush_content_buffer()

        return True

    async def _flush_content_buffer(self):
        """刷新正文缓冲区，并将生成的 ID 加入段落池"""
        if not self._content_buffer:
            return

        try:
            # 按 group_id 分组
            groups: Dict[int, List[str]] = {}
            for item in self._content_buffer:
                gid = item['group_id']
                if gid not in groups:
                    groups[gid] = []
                groups[gid].append(item['content'])

            count = 0
            inserted_ids: Dict[int, List[int]] = {}  # group_id -> [content_ids]

            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    for group_id, contents in groups.items():
                        batch_id = await self.get_next_batch_id(group_id, 'contents')
                        inserted_ids[group_id] = []

                        # 逐条插入以获取 ID
                        for content in contents:
                            await cursor.execute(
                                """
                                INSERT INTO contents (content, group_id, batch_id)
                                VALUES (%s, %s, %s)
                                """,
                                (content, group_id, batch_id)
                            )
                            inserted_ids[group_id].append(cursor.lastrowid)
                            count += 1

                await conn.commit()

            self._content_buffer.clear()
            logger.debug(f"Flushed {count} contents to database")

        except Exception as e:
            logger.error(f"Failed to flush content buffer: {e}")

    # ============================================
    # 文章处理逻辑
    # ============================================

    async def process_article(self, article: Dict[str, Any]) -> bool:
        """
        处理单篇文章

        1. 提取标题 → titles 表
        2. 拆分正文 → 拼音标注 → contents 表

        Args:
            article: {'id': int, 'title': str, 'content': str, 'group_id': int}

        Returns:
            是否处理成功
        """
        start_time = time.perf_counter()

        group_id = article.get('group_id', 1)
        title = article.get('title', '')
        content = article.get('content', '')
        article_id = article.get('id', 0)

        try:
            # 1. 保存标题
            if title:
                await self.save_title(title, group_id)

            # 2. 处理正文：拆分段落 → 清理 → 拼音标注 → 保存
            paragraph_count = 0
            if content:
                # 按换行拆分段落
                paragraphs = content.split('\n') if isinstance(content, str) else []
                # 清理（过滤太短的段落）
                paragraphs = self.cleaner.clean_paragraphs(paragraphs)

                for para in paragraphs:
                    # 拼音标注
                    annotated = self.annotator.annotate(para)
                    # 保存到 contents 表
                    await self.save_content(annotated, group_id)
                    paragraph_count += 1

            self._processed_count += 1

            # 记录处理时间
            elapsed_ms = (time.perf_counter() - start_time) * 1000
            self._total_processing_time_ms += elapsed_ms

            # 清除重试计数（如果有）
            await self.clear_retry_count(article['id'])

            # 更新今日处理量
            await self._update_daily_stats()

            # 每处理 10 篇文章打印一次日志
            if self._processed_count % 10 == 0:
                logger.info(f"已处理 {self._processed_count} 篇文章")

            return True

        except Exception as e:
            logger.error(f"Failed to process article {article_id}: {e}")
            return False

    async def _update_daily_stats(self):
        """更新今日处理量"""
        try:
            today_key = f"processor:processed:{datetime.now().strftime('%Y%m%d')}"
            await self.redis.incr(today_key)
            # 设置过期时间（2天）
            await self.redis.expire(today_key, 172800)
        except Exception:
            pass

    # ============================================
    # 运行方法
    # ============================================

    async def process_with_retry(self, article_id: int) -> bool:
        """
        处理文章，带重试逻辑

        Args:
            article_id: 文章ID

        Returns:
            是否处理成功
        """
        try:
            article = await self.get_article_by_id(article_id)
            if article is None:
                logger.warning(f"Article {article_id} not found in database")
                if self.on_complete:
                    await self.on_complete()
                return False

            success = await self.process_article(article)
            if not success:
                await self.handle_failure(article_id, "Processing failed")
                if self.on_complete:
                    await self.on_complete()
                return False

            if self.on_complete:
                await self.on_complete()
            return True

        except Exception as e:
            await self.handle_failure(article_id, str(e))
            if self.on_complete:
                await self.on_complete()
            return False

    async def run_once(self, count: int = 100) -> int:
        """
        执行一次处理

        Args:
            count: 要处理的文章数量

        Returns:
            成功处理的数量
        """
        await self.start()

        processed = 0
        try:
            for _ in range(count):
                # 从队列取文章ID
                article_id = await self.pop_article_id(timeout=1)
                if article_id is None:
                    # 队列为空，退出
                    break

                if await self.process_with_retry(article_id):
                    processed += 1

            # 刷新缓冲区
            await self._flush_title_buffer()
            await self._flush_content_buffer()

        finally:
            await self.stop()

        return processed

    async def run_forever(self, wait_interval: float = 5.0, stop_event: Optional[asyncio.Event] = None, group_id: int = 1) -> None:
        """
        持续运行（高性能版）

        从 Redis 队列取文章ID，按主键查询后处理。
        无论表多大，都是 O(1) 毫秒级响应。

        Args:
            wait_interval: 队列为空时的等待间隔（秒）
            stop_event: 外部停止事件（可选）
        """
        await self.start()

        try:
            while self._running:
                # 检查外部停止事件
                if stop_event and stop_event.is_set():
                    break

                try:
                    # 从 Redis 队列取文章ID（BRPOP 阻塞等待）
                    article_id = await self.pop_article_id(timeout=int(wait_interval))

                    if article_id is not None:
                        await self.process_with_retry(article_id)
                    else:
                        # 超时，队列为空
                        logger.debug("Waiting for articles (queue empty)")

                except asyncio.CancelledError:
                    break
                except Exception as e:
                    logger.error(f"Processing error: {e}")
                    await asyncio.sleep(1)

        finally:
            await self.stop()

    def get_stats(self) -> Dict[str, Any]:
        """获取统计信息"""
        avg_ms = 0.0
        if self._processed_count > 0:
            avg_ms = self._total_processing_time_ms / self._processed_count

        return {
            'processed': self._processed_count,
            'failed': self._failed_count,
            'retried': self._retried_count,
            'total_processing_time_ms': self._total_processing_time_ms,
            'avg_processing_ms': avg_ms,
            'title_buffer_size': len(self._title_buffer),
            'content_buffer_size': len(self._content_buffer),
            'running': self._running
        }
