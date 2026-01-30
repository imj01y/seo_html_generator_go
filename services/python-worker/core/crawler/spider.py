# -*- coding: utf-8 -*-
"""
Spider 基类

feapder 风格的爬虫基类，用户继承此类实现爬虫逻辑。
"""

from typing import Dict, Any, Generator, Optional

from .request import Request
from .response import Response


class Spider:
    """
    爬虫基类 - feapder 风格

    用户继承此类，实现 start_requests 和 parse 方法。

    Example:
        class MySpider(Spider):
            name = "example"

            __custom_setting__ = dict(
                CONCURRENT_REQUESTS=3,
                DOWNLOAD_DELAY=1,
            )

            def start_requests(self):
                yield Request("https://example.com")

            def parse(self, request, response):
                yield {"title": response.xpath("//title/text()").extract_first()}

        if __name__ == "__main__":
            MySpider(redis_key="project:spider").start()
    """

    # 爬虫名称
    name: Optional[str] = None

    # 自定义配置（feapder 风格）
    __custom_setting__: Dict[str, Any] = {}

    def __init__(self, redis_key: str = None, **kwargs):
        """
        初始化 Spider

        Args:
            redis_key: Redis 队列键名（feapder 兼容）
            **kwargs: 其他参数会设置为实例属性
        """
        self.redis_key = redis_key
        for key, value in kwargs.items():
            setattr(self, key, value)

    def start_requests(self) -> Generator[Request, None, None]:
        """
        生成初始请求 - 子类必须实现

        Yields:
            Request: 初始请求对象

        Example:
            def start_requests(self):
                for page in range(1, 10):
                    yield Request(f"https://example.com/page/{page}")
        """
        raise NotImplementedError(f"{self.__class__.__name__}.start_requests() 未实现")

    def parse(self, request: Request, response: Response) -> Generator[Any, None, None]:
        """
        默认回调方法 - 子类必须实现

        Args:
            request: 当前请求对象
            response: 响应对象

        Yields:
            dict: 数据字典
            Request: 新的请求对象

        Example:
            def parse(self, request, response):
                yield {"title": response.css("h1::text").get()}

                for url in response.css("a::attr(href)").getall():
                    yield Request(url, callback=self.parse_detail)
        """
        raise NotImplementedError(f"{self.__class__.__name__}.parse() 未实现")

    def start(self):
        """
        启动爬虫（feapder 兼容）

        注意：此方法在独立运行时使用，在框架内运行时由 ProjectRunner 调用。
        """
        pass

    def close(self, reason: str = "finished"):
        """
        爬虫关闭时调用（可选重写）

        Args:
            reason: 关闭原因
        """
        pass

    def download_midware(self, request: Request) -> Optional[Request]:
        """
        下载中间件（可选重写）

        在请求发送前调用，可以修改请求或返回 None 跳过请求。

        Args:
            request: 请求对象

        Returns:
            Request: 修改后的请求，返回 None 则跳过该请求
        """
        return request

    def validate(self, request: Request, response: Response) -> bool:
        """
        响应验证（可选重写）

        验证响应是否有效，返回 False 会触发重试。

        Args:
            request: 请求对象
            response: 响应对象

        Returns:
            bool: 响应是否有效
        """
        return True

    def exception_request(self, request: Request, response: Response, e: Exception):
        """
        请求异常回调（可选重写）

        Args:
            request: 请求对象
            response: 响应对象（可能为 None）
            e: 异常对象
        """
        pass

    def failed_request(self, request: Request, response: Response, e: Exception):
        """
        请求失败回调（超过重试次数后调用，可选重写）

        Args:
            request: 请求对象
            response: 响应对象（可能为 None）
            e: 异常对象
        """
        pass
