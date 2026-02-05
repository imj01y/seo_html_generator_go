# -*- coding: utf-8 -*-
"""
爬虫模块

提供 feapder 风格的爬虫框架。
用户只需继承 Spider 类，实现 start_requests 和 parse 方法即可。

Example:
    from core.crawler import Spider, Request

    class MySpider(Spider):
        name = "example"

        def start_requests(self):
            yield Request("https://example.com")

        def parse(self, request, response):
            yield {"title": response.xpath("//title/text()").extract_first()}

    # 迁移到 feapder 只需改导入语句：
    # from core.crawler import Spider, Request
    # -> import feapder
    # Spider -> feapder.Spider
    # Request -> feapder.Request
"""

from .spider import Spider
from .request import Request
from .response import Response
from .http_client import AsyncHttpClient
from .project_loader import ProjectLoader
from .project_runner import ProjectRunner
from .request_queue import RequestQueue, QueueStats
from .queue_consumer import QueueConsumer
from .failed_manager import FailedRequestManager

__all__ = [
    # 核心类（用户使用）
    'Spider',
    'Request',
    'Response',
    # 项目管理
    'ProjectLoader',
    'ProjectRunner',
    # HTTP
    'AsyncHttpClient',
    # 队列模式
    'RequestQueue',
    'QueueStats',
    'QueueConsumer',
    'FailedRequestManager',
]
