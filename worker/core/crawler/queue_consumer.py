# -*- coding: utf-8 -*-
"""
队列消费器

并发消费 Redis 队列中的 Request，执行 HTTP 请求并调用 Spider 回调方法。
只支持 Spider 类模式（feapder 风格）。
"""

import asyncio
import json
from typing import Optional, Dict, Any, AsyncGenerator, Callable, Iterator, TYPE_CHECKING
from loguru import logger

if TYPE_CHECKING:
    from redis.asyncio import Redis

from .spider import Spider
from .request import Request
from .response import Response
from .request_queue import RequestQueue
from .http_client import AsyncHttpClient


class QueueConsumer:
    """
    队列消费器 - 只支持 Spider 模式

    从 Redis 队列中取出 Request，发起 HTTP 请求，调用 Spider 回调方法处理响应。
    支持并发消费、自动重试等。

    Example:
        consumer = QueueConsumer(
            redis=redis_client,
            project_id=1,
            spider=spider_instance,
            concurrency=3,
        )

        async for result in consumer.run():
            if isinstance(result, dict):
                save_to_db(result)
    """

    def __init__(
        self,
        redis: 'Redis',
        project_id: int,
        spider: Spider,
        concurrency: int = 3,
        http_client: Optional[AsyncHttpClient] = None,
        stop_callback: Optional[Callable[[], bool]] = None,
        is_test: bool = False,
        max_items: int = 0,
        start_requests_iter: Optional[Iterator] = None,
    ):
        """
        初始化消费器

        Args:
            redis: Redis 客户端
            project_id: 项目ID
            spider: Spider 实例
            concurrency: 并发数
            http_client: HTTP 客户端（可选，默认创建新的）
            stop_callback: 停止检查回调（返回 True 则停止）
            is_test: 是否为测试模式（使用独立的 Redis 队列）
            max_items: 最大数据条数（0 表示不限制）
            start_requests_iter: 初始请求生成器（用于按需补充请求）
        """
        self.redis = redis
        self.project_id = project_id
        self.spider = spider
        self.concurrency = concurrency
        self.stop_callback = stop_callback
        self.max_items = max_items
        self.start_requests_iter = start_requests_iter

        # 队列管理器
        self.queue = RequestQueue(redis, project_id, is_test=is_test)

        # HTTP 客户端
        self.http_client = http_client or AsyncHttpClient()

        # 运行状态
        self._running = False

    def _get_callback(self, callback_name: str) -> Callable:
        """
        获取 Spider 的回调方法

        Args:
            callback_name: 回调方法名

        Returns:
            回调方法

        Raises:
            ValueError: Spider 没有该方法
        """
        if hasattr(self.spider, callback_name):
            return getattr(self.spider, callback_name)
        raise ValueError(f"Spider '{self.spider.__class__.__name__}' 没有方法: {callback_name}")

    async def _check_item_limit(self) -> bool:
        """检查是否已达到数据限制（基于已入队的请求数）"""
        if self.max_items <= 0:
            return False
        # 改用 queued_count 检查，而不是 item_count
        queued = await self.queue.get_queued_count()
        return queued >= self.max_items

    async def _feed_requests(self) -> bool:
        """
        从 start_requests 生成器按需补充请求（翻页逻辑）

        Returns:
            bool: True 表示还有更多请求可补充，False 表示生成器已耗尽或达到限制
        """
        if self.start_requests_iter is None:
            return False

        # 如果已达限制，不再补充（停止翻页）
        if await self._check_item_limit():
            logger.info("Item limit reached, stop feeding more requests (no more pagination)")
            self.start_requests_iter = None
            return False

        # 检查队列是否需要补充
        # pending 数量小于并发数的2倍时，补充请求
        pending = await self.queue.get_pending_count()
        if pending >= self.concurrency * 2:
            return True  # 队列充足，暂不补充，但生成器还有

        # 从生成器获取请求并入队
        try:
            request = next(self.start_requests_iter)
            if callable(request.callback):
                request.callback = request.callback.__name__
            # start_requests 产出的请求跳过去重，确保列表页每次都能抓取
            request.dont_filter = True
            await self.queue.push(request)
            logger.info(f"Fed request from start_requests: {request.url[:80]}")
            return True
        except StopIteration:
            # 生成器已耗尽（所有列表页都已入队）
            logger.info("start_requests exhausted")
            self.start_requests_iter = None
            return False

    async def _fetch_request(self, request: Request) -> Optional[Response]:
        """
        发起 HTTP 请求

        Args:
            request: 请求对象

        Returns:
            Response 对象，失败返回 None
        """
        try:
            # 调用 Spider 的下载中间件（如果有）
            request = self.spider.download_midware(request)
            if request is None:
                logger.debug(f"Request skipped by download_midware")
                return None

            # 合并请求参数
            headers = dict(self.http_client.headers)
            if request.headers:
                headers.update(request.headers)

            timeout = request.timeout or self.http_client.timeout

            # 处理 JSON 请求体
            request_body = request.body
            request_json = None
            if request_body and request.headers.get('Content-Type') == 'application/json':
                try:
                    request_json = json.loads(request_body)
                    request_body = None  # 使用 json 参数代替 body
                except (json.JSONDecodeError, TypeError):
                    pass  # 保持原始 body

            logger.info(f"Fetching [{request.method}]: {request.url[:80]}")
            body = await self.http_client.fetch(
                url=request.url,
                method=request.method,
                headers=headers,
                body=request_body,
                json=request_json,
                timeout=timeout,
            )

            if body is None:
                logger.info(f"Fetch failed: {request.url[:80]}")
                return None

            response = Response.from_request(
                request=request,
                body=body,
                status=200,
            )

            # 调用 Spider 的响应验证（如果有）
            if not self.spider.validate(request, response):
                logger.warning(f"Response validation failed for {request.url[:50]}")
                return None

            return response

        except asyncio.CancelledError:
            # 任务被取消，向上传播
            logger.info(f"Fetch cancelled: {request.url[:50]}")
            raise

        except Exception as e:
            logger.error(f"Fetch error for {request.url[:50]}: {e}")
            # 调用 Spider 的异常回调
            self.spider.exception_request(request, None, e)
            return None

    async def _process_request(self, request: Request) -> AsyncGenerator[Any, None]:
        """
        处理单个请求

        1. 发起 HTTP 请求
        2. 调用回调方法（feapder 风格：callback(request, response)）
        3. 处理回调返回的数据（dict 或 Request）

        Args:
            request: 请求对象

        Yields:
            dict 数据或 Request 对象
        """
        # 计算重试延迟（指数退避）
        if request.retry_count > 0:
            delay = request.retry_delay * (2 ** (request.retry_count - 1))
            logger.debug(f"Retry delay {delay}s for {request.url[:50]}")
            try:
                await asyncio.sleep(delay)
            except asyncio.CancelledError:
                logger.info(f"Retry sleep cancelled for {request.url[:50]}")
                raise

        # 发起请求
        response = await self._fetch_request(request)

        if response is None:
            # 请求失败，尝试重试
            can_retry = await self.queue.retry(request)
            if not can_retry:
                # 超过重试次数，标记失败
                await self.queue.complete(request, success=False)
                # 调用 Spider 的失败回调
                error_msg = self.http_client.last_error or 'Unknown error'
                self.spider.failed_request(request, None, Exception(error_msg))
                # 通知调用方保存到 failed 表
                yield {
                    '_type': 'failed',
                    'request': request,
                    'error': error_msg,
                }
            return

        # 获取回调方法
        callback_name = request.callback_name
        callback = self._get_callback(callback_name)

        # 调用回调方法（feapder 风格：parse(self, request, response)）
        try:
            result = callback(request, response)

            # 处理生成器结果
            if result is not None:
                if hasattr(result, '__iter__') or hasattr(result, '__next__'):
                    for item in result:
                        # 检查停止信号
                        if self.stop_callback and self.stop_callback():
                            logger.info("Stop signal received during callback iteration")
                            return
                        if isinstance(item, Request):
                            # 先递增计数，再检查限制
                            if self.max_items > 0:
                                queued = await self.queue.incr_queued_count()
                                if queued > self.max_items:
                                    logger.debug(f"Queued limit reached ({self.max_items}), breaking loop")
                                    break
                            # 如果 callback 是方法对象，转为方法名
                            if callable(item.callback):
                                item.callback = item.callback.__name__
                            # 新请求入队
                            await self.queue.push(item)
                            yield item
                        elif isinstance(item, dict):
                            # 检查是否已达到限制
                            if self.max_items > 0:
                                current = await self.queue.incr_item_count()
                                if current > self.max_items:
                                    # 超出限制，停止消费
                                    logger.info(f"Item limit reached ({self.max_items}), stopping")
                                    self._running = False
                                    await self.queue.set_state(RequestQueue.STATE_STOPPED)
                                    return
                            # 数据
                            yield item
                        else:
                            logger.warning(f"Invalid callback result type: {type(item)}")
                elif isinstance(result, Request):
                    # 先递增计数，再检查限制
                    can_queue = True
                    if self.max_items > 0:
                        queued = await self.queue.incr_queued_count()
                        if queued > self.max_items:
                            logger.debug(f"Queued limit reached ({self.max_items}), skipping: {result.url[:50]}")
                            can_queue = False
                    if can_queue:
                        if callable(result.callback):
                            result.callback = result.callback.__name__
                        await self.queue.push(result)
                        yield result
                elif isinstance(result, dict):
                    # 检查是否已达到限制
                    if self.max_items > 0:
                        current = await self.queue.incr_item_count()
                        if current > self.max_items:
                            logger.info(f"Item limit reached ({self.max_items}), stopping")
                            self._running = False
                            await self.queue.set_state(RequestQueue.STATE_STOPPED)
                            return
                    yield result

            # 标记请求完成
            await self.queue.complete(request, success=True)

        except asyncio.CancelledError:
            # 任务被取消，向上传播
            logger.info(f"Request processing cancelled: {request.url}")
            raise

        except Exception as e:
            logger.exception(f"Callback error for {request.url}")
            # 调用 Spider 的异常回调
            self.spider.exception_request(request, response, e)
            # 回调出错，尝试重试
            can_retry = await self.queue.retry(request)
            if not can_retry:
                await self.queue.complete(request, success=False)
                self.spider.failed_request(request, response, e)
                yield {
                    '_type': 'failed',
                    'request': request,
                    'error': str(e),
                }

    async def _worker(self, worker_id: int, output_queue: asyncio.Queue) -> None:
        """
        工作协程

        从队列取请求，处理后将结果放入输出队列。

        Args:
            worker_id: 工作者ID
            output_queue: 输出队列
        """
        logger.info(f"Worker {worker_id} started")

        try:
            while self._running:
                # 检查停止信号
                if self.stop_callback and self.stop_callback():
                    logger.info(f"Worker {worker_id} received stop signal")
                    break

                # 检查队列状态
                state = await self.queue.get_state()
                if state == RequestQueue.STATE_PAUSED:
                    await asyncio.sleep(1)
                    continue
                if state == RequestQueue.STATE_STOPPED:
                    logger.info(f"Worker {worker_id} exiting: state is STOPPED")
                    break

                # 从队列取请求
                request = await self.queue.pop()
                if request is None:
                    # 队列可能为空或暂停
                    await asyncio.sleep(0.1)
                    continue

                # pop 之后再次检查停止信号
                if self.stop_callback and self.stop_callback():
                    # 放回队列，然后退出
                    await self.queue.push(request)
                    logger.info(f"Worker {worker_id} returning request and stopping")
                    break

                logger.info(f"Worker {worker_id} processing: {request.url[:80]}")

                # 处理请求
                try:
                    async for item in self._process_request(request):
                        await output_queue.put(item)
                except asyncio.CancelledError:
                    # 任务被取消，放回请求并退出
                    await self.queue.push(request)
                    logger.info(f"Worker {worker_id} cancelled, returning request")
                    raise
                except Exception as e:
                    logger.error(f"Worker {worker_id} error: {e}")

        except asyncio.CancelledError:
            logger.info(f"Worker {worker_id} cancelled")
            raise

        logger.info(f"Worker {worker_id} stopped")

    async def run(self) -> AsyncGenerator[Dict[str, Any], None]:
        """
        运行消费器

        启动多个工作协程并发消费队列。

        Yields:
            dict 数据或特殊消息（如 failed 请求信息）
        """
        self._running = True
        workers = []

        try:
            # 设置队列状态
            await self.queue.set_state(RequestQueue.STATE_RUNNING)

            # 恢复超时请求
            recovered = await self.queue.recover_timeout()
            if recovered:
                logger.info(f"Recovered {recovered} timeout requests")

            # 输出队列
            output_queue: asyncio.Queue = asyncio.Queue()

            # 启动工作协程（Worker 立即待命）
            workers = [
                asyncio.create_task(self._worker(i, output_queue))
                for i in range(self.concurrency)
            ]

            logger.info(f"Started {self.concurrency} workers (standing by)")

            # 监控循环
            empty_count = 0
            while self._running:
                # 检查停止信号
                if self.stop_callback and self.stop_callback():
                    break

                # 检查队列状态
                state = await self.queue.get_state()
                if state == RequestQueue.STATE_STOPPED:
                    break

                # 【核心】按需补充初始请求（翻页）
                # 当队列快空时，从生成器获取更多请求
                has_more = await self._feed_requests()

                # 检查是否完成：队列空 + 生成器耗尽
                if await self.queue.is_empty():
                    if not has_more:
                        empty_count += 1
                        if empty_count >= 3:  # 连续3次检测为空，认为完成
                            logger.info(f"Queue empty and no more requests, stopping")
                            break
                    else:
                        empty_count = 0
                else:
                    empty_count = 0

                # 收集输出
                while not output_queue.empty():
                    try:
                        item = output_queue.get_nowait()
                        yield item
                    except asyncio.QueueEmpty:
                        break

                await asyncio.sleep(0.1)

            # 收集剩余输出
            while not output_queue.empty():
                try:
                    item = output_queue.get_nowait()
                    yield item
                except asyncio.QueueEmpty:
                    break

            # 检查是否正常完成
            if await self.queue.is_empty():
                await self.queue.set_state(RequestQueue.STATE_COMPLETED)
            else:
                state = await self.queue.get_state()
                if state != RequestQueue.STATE_STOPPED:
                    await self.queue.set_state(RequestQueue.STATE_STOPPED)

        except asyncio.CancelledError:
            logger.info(f"Consumer cancelled for project {self.project_id}")
            raise

        finally:
            # 停止工作协程
            self._running = False
            for worker in workers:
                if not worker.done():
                    worker.cancel()

            # 等待工作协程结束
            if workers:
                await asyncio.gather(*workers, return_exceptions=True)

            logger.info(f"Consumer finished for project {self.project_id}")

    async def stop(self) -> None:
        """停止消费器"""
        self._running = False
        await self.queue.stop(clear_queue=False)

    async def get_stats(self) -> Dict[str, Any]:
        """获取统计信息"""
        stats = await self.queue.get_stats()
        state = await self.queue.get_state()
        return {
            **stats.to_dict(),
            'status': state,
        }
