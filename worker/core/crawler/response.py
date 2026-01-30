# -*- coding: utf-8 -*-
"""
Response 类

Scrapy 风格的响应对象，支持 CSS/XPath 选择器、JSON 解析等。
"""

import json
from typing import Optional, Dict, Any, List, TYPE_CHECKING
from urllib.parse import urljoin

if TYPE_CHECKING:
    from .request import Request

# 尝试导入 parsel 用于 CSS/XPath 选择
try:
    from parsel import Selector, SelectorList
    HAS_PARSEL = True
except ImportError:
    HAS_PARSEL = False
    Selector = None
    SelectorList = None


class Response:
    """
    HTTP 响应对象

    类似 Scrapy 的 Response，提供 CSS/XPath 选择器等便捷方法。

    Attributes:
        url: 响应URL（可能与请求URL不同，如重定向后）
        body: 响应体（字节）
        status: HTTP状态码
        headers: 响应头
        request: 对应的 Request 对象

    Example:
        # 使用 CSS 选择器
        title = response.css('h1::text').get()
        links = response.css('a::attr(href)').getall()

        # 使用 XPath
        content = response.xpath('//div[@class="content"]/text()').get()

        # 解析 JSON
        data = response.json()

        # 获取透传数据
        page = response.meta.get('page', 1)
    """

    def __init__(
        self,
        url: str,
        body: bytes,
        status: int = 200,
        headers: Optional[Dict[str, str]] = None,
        request: Optional['Request'] = None,
        encoding: str = 'utf-8',
    ):
        """
        初始化响应对象

        Args:
            url: 响应URL
            body: 响应体（字节）
            status: HTTP状态码
            headers: 响应头
            request: 对应的请求对象
            encoding: 文本编码
        """
        self.url = url
        self._body = body
        self.status = status
        self.headers = headers or {}
        self.request = request
        self._encoding = encoding
        self._selector: Optional[Any] = None
        self._text: Optional[str] = None

    @property
    def body(self) -> bytes:
        """响应体（字节）"""
        return self._body

    @property
    def text(self) -> str:
        """响应体（文本）"""
        if self._text is None:
            self._text = self._body.decode(self._encoding, errors='replace')
        return self._text

    @property
    def encoding(self) -> str:
        """响应编码"""
        return self._encoding

    @property
    def meta(self) -> Dict[str, Any]:
        """获取请求透传的元数据"""
        if self.request:
            return self.request.meta
        return {}

    def _get_selector(self) -> Any:
        """获取 Selector 对象（懒加载）"""
        if self._selector is None:
            if not HAS_PARSEL:
                raise ImportError(
                    "parsel 未安装，请运行: pip install parsel\n"
                    "或使用 response.text 配合其他解析库"
                )
            self._selector = Selector(text=self.text)
        return self._selector

    def css(self, query: str) -> Any:
        """
        CSS 选择器查询

        Args:
            query: CSS 选择器表达式

        Returns:
            SelectorList: 匹配的选择器列表

        Example:
            # 获取单个值
            title = response.css('h1::text').get()

            # 获取多个值
            links = response.css('a::attr(href)').getall()

            # 链式调用
            items = response.css('.item')
            for item in items:
                title = item.css('.title::text').get()
        """
        return self._get_selector().css(query)

    def xpath(self, query: str, **kwargs) -> Any:
        """
        XPath 选择器查询

        Args:
            query: XPath 表达式
            **kwargs: 命名空间等额外参数

        Returns:
            SelectorList: 匹配的选择器列表

        Example:
            # 获取文本
            title = response.xpath('//h1/text()').get()

            # 获取属性
            links = response.xpath('//a/@href').getall()
        """
        return self._get_selector().xpath(query, **kwargs)

    def json(self) -> Any:
        """
        解析 JSON 响应

        Returns:
            解析后的 JSON 数据

        Raises:
            json.JSONDecodeError: JSON 解析失败

        Example:
            data = response.json()
            for item in data['list']:
                yield item
        """
        return json.loads(self.text)

    def urljoin(self, url: str) -> str:
        """
        合并相对URL

        Args:
            url: 相对URL或绝对URL

        Returns:
            str: 完整URL

        Example:
            # 相对路径
            full_url = response.urljoin('/page/2')

            # 已经是绝对路径则原样返回
            full_url = response.urljoin('https://other.com/path')
        """
        return urljoin(self.url, url)

    def follow(self, url: str, callback=None, **kwargs) -> 'Request':
        """
        创建跟随链接的新请求

        Args:
            url: 相对或绝对URL
            callback: 回调函数
            **kwargs: 传递给 Request 的其他参数

        Returns:
            Request: 新的请求对象

        Example:
            # 跟随链接
            yield response.follow('/next', callback=self.parse)

            # 带元数据
            yield response.follow(url, callback=parse, meta={'page': 2})
        """
        from .request import Request

        full_url = self.urljoin(url)
        # 默认继承当前请求的 meta
        meta = dict(self.meta)
        if 'meta' in kwargs:
            meta.update(kwargs.pop('meta'))

        return Request(
            url=full_url,
            callback=callback,
            meta=meta,
            **kwargs
        )

    def follow_all(self, urls: List[str], callback=None, **kwargs) -> List['Request']:
        """
        批量创建跟随链接的请求

        Args:
            urls: URL列表
            callback: 回调函数
            **kwargs: 传递给 Request 的其他参数

        Returns:
            List[Request]: 请求对象列表
        """
        return [self.follow(url, callback=callback, **kwargs) for url in urls]

    @classmethod
    def from_request(
        cls,
        request: 'Request',
        body: bytes,
        status: int = 200,
        headers: Optional[Dict[str, str]] = None,
        url: Optional[str] = None,
        encoding: str = 'utf-8',
    ) -> 'Response':
        """
        从 Request 创建 Response

        Args:
            request: 请求对象
            body: 响应体
            status: HTTP状态码
            headers: 响应头
            url: 响应URL（默认使用请求URL）
            encoding: 文本编码

        Returns:
            Response: 响应对象
        """
        return cls(
            url=url or request.url,
            body=body,
            status=status,
            headers=headers,
            request=request,
            encoding=encoding,
        )

    def __repr__(self) -> str:
        return f"<Response [{self.status}] {self.url[:50]}{'...' if len(self.url) > 50 else ''}>"

    def __len__(self) -> int:
        """响应体长度"""
        return len(self._body)

    def __bool__(self) -> bool:
        """响应是否有效"""
        return 200 <= self.status < 300
