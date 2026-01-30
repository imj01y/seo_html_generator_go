# -*- coding: utf-8 -*-
"""
Request 类

Scrapy 风格的请求对象，支持回调、元数据透传、重试等功能。
"""

import json
import hashlib
from dataclasses import dataclass, field
from typing import Optional, Dict, Any, Callable, TYPE_CHECKING

if TYPE_CHECKING:
    from .response import Response


@dataclass
class Request:
    """
    HTTP 请求对象

    类似 Scrapy 的 Request，支持回调函数、元数据透传、重试机制等。

    Attributes:
        url: 请求URL
        callback: 回调函数名称（字符串）或函数对象
        method: HTTP方法（GET/POST等）
        headers: 自定义请求头
        body: 请求体（POST数据）
        meta: 透传数据，会传递到 Response.meta
        priority: 优先级（数值越大越优先）
        dont_filter: 是否跳过URL去重
        cookies: 请求cookies
        timeout: 请求超时时间（秒）
        max_retries: 最大重试次数
        retry_count: 当前重试次数（内部使用）
        retry_delay: 基础重试延迟（秒），使用指数退避

    Example:
        # 基本用法
        yield Request('https://example.com', callback=parse_detail)

        # 带元数据
        yield Request(url, callback=parse, meta={'page': 1})

        # 自定义请求参数
        yield Request(
            url,
            callback=parse,
            headers={'Authorization': 'Bearer xxx'},
            timeout=30,
            priority=10,
        )
    """
    url: str
    callback: Optional[Any] = None  # 回调函数或函数名
    method: str = 'GET'
    headers: Optional[Dict[str, str]] = None
    body: Optional[str] = None
    meta: Dict[str, Any] = field(default_factory=dict)
    priority: int = 0
    dont_filter: bool = False
    cookies: Optional[Dict[str, str]] = None
    timeout: Optional[int] = None
    max_retries: int = 3
    retry_count: int = 0
    retry_delay: float = 1.0

    def __post_init__(self):
        """初始化后处理"""
        # 确保 meta 是字典
        if self.meta is None:
            self.meta = {}
        # 确保 headers 是字典或 None
        if self.headers is None:
            self.headers = {}

    @property
    def callback_name(self) -> str:
        """获取回调函数名称"""
        if self.callback is None:
            return 'parse'
        if isinstance(self.callback, str):
            return self.callback
        if callable(self.callback):
            return self.callback.__name__
        return 'parse'

    def fingerprint(self) -> str:
        """
        生成请求指纹，用于去重

        基于 URL、method、body 生成唯一标识。

        Returns:
            str: 请求指纹（MD5 哈希）
        """
        parts = [
            self.url,
            self.method.upper(),
            self.body or '',
        ]
        content = '|'.join(parts)
        return hashlib.md5(content.encode()).hexdigest()

    def copy(self) -> 'Request':
        """创建请求的副本"""
        return Request(
            url=self.url,
            callback=self.callback,
            method=self.method,
            headers=dict(self.headers) if self.headers else None,
            body=self.body,
            meta=dict(self.meta),
            priority=self.priority,
            dont_filter=self.dont_filter,
            cookies=dict(self.cookies) if self.cookies else None,
            timeout=self.timeout,
            max_retries=self.max_retries,
            retry_count=self.retry_count,
            retry_delay=self.retry_delay,
        )

    def replace(self, **kwargs) -> 'Request':
        """创建替换了部分属性的副本"""
        new_request = self.copy()
        for key, value in kwargs.items():
            if hasattr(new_request, key):
                setattr(new_request, key, value)
        return new_request

    @classmethod
    def post(
        cls,
        url: str,
        callback: Optional[Any] = None,
        body: Optional[str] = None,
        json_data: Optional[Dict[str, Any]] = None,
        headers: Optional[Dict[str, str]] = None,
        **kwargs
    ) -> 'Request':
        """
        创建 POST 请求的便捷方法

        Args:
            url: 请求URL
            callback: 回调函数
            body: 原始请求体
            json_data: JSON 数据（会自动序列化并设置 Content-Type）
            headers: 自定义请求头
            **kwargs: 其他 Request 参数

        Returns:
            Request 对象

        Example:
            # 发送 JSON 数据
            yield Request.post(
                'https://api.example.com/data',
                json_data={'key': 'value'},
                callback=self.parse_api,
            )

            # 发送表单数据
            yield Request.post(
                'https://example.com/form',
                body='name=test&value=123',
                callback=self.parse_form,
            )
        """
        request_headers = dict(headers) if headers else {}
        request_body = body

        if json_data is not None:
            request_body = json.dumps(json_data, ensure_ascii=False)
            request_headers['Content-Type'] = 'application/json'

        return cls(
            url=url,
            callback=callback,
            method='POST',
            body=request_body,
            headers=request_headers,
            **kwargs
        )

    @classmethod
    def get(
        cls,
        url: str,
        callback: Optional[Any] = None,
        params: Optional[Dict[str, Any]] = None,
        headers: Optional[Dict[str, str]] = None,
        **kwargs
    ) -> 'Request':
        """
        创建 GET 请求的便捷方法

        Args:
            url: 请求URL
            callback: 回调函数
            params: URL 查询参数（会自动拼接到 URL）
            headers: 自定义请求头
            **kwargs: 其他 Request 参数

        Returns:
            Request 对象

        Example:
            # 基本用法
            yield Request.get(
                'https://example.com/api',
                callback=self.parse_api,
            )

            # 带查询参数
            yield Request.get(
                'https://example.com/search',
                params={'q': 'python', 'page': 1},
                callback=self.parse_search,
            )
        """
        request_url = url

        # 处理查询参数
        if params:
            from urllib.parse import urlencode, urlparse, parse_qs, urlunparse
            parsed = urlparse(url)
            existing_params = parse_qs(parsed.query)
            for key, value in params.items():
                existing_params[key] = [str(value)]
            new_query = urlencode(existing_params, doseq=True)
            request_url = urlunparse((
                parsed.scheme, parsed.netloc, parsed.path,
                parsed.params, new_query, parsed.fragment
            ))

        return cls(
            url=request_url,
            callback=callback,
            method='GET',
            headers=headers,
            **kwargs
        )

    def to_dict(self) -> Dict[str, Any]:
        """
        序列化为字典（用于存储到 Redis）

        注意：callback 存储为函数名字符串
        """
        return {
            'url': self.url,
            'callback': self.callback_name,
            'method': self.method,
            'headers': self.headers,
            'body': self.body,
            'meta': self.meta,
            'priority': self.priority,
            'dont_filter': self.dont_filter,
            'cookies': self.cookies,
            'timeout': self.timeout,
            'max_retries': self.max_retries,
            'retry_count': self.retry_count,
            'retry_delay': self.retry_delay,
        }

    def to_json(self) -> str:
        """序列化为 JSON 字符串"""
        return json.dumps(self.to_dict(), ensure_ascii=False)

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> 'Request':
        """从字典反序列化"""
        return cls(
            url=data['url'],
            callback=data.get('callback'),
            method=data.get('method', 'GET'),
            headers=data.get('headers'),
            body=data.get('body'),
            meta=data.get('meta', {}),
            priority=data.get('priority', 0),
            dont_filter=data.get('dont_filter', False),
            cookies=data.get('cookies'),
            timeout=data.get('timeout'),
            max_retries=data.get('max_retries', 3),
            retry_count=data.get('retry_count', 0),
            retry_delay=data.get('retry_delay', 1.0),
        )

    @classmethod
    def from_json(cls, json_str: str) -> 'Request':
        """从 JSON 字符串反序列化"""
        return cls.from_dict(json.loads(json_str))

    def __repr__(self) -> str:
        return f"<Request {self.method} {self.url[:50]}{'...' if len(self.url) > 50 else ''} callback={self.callback_name}>"

    def __hash__(self) -> int:
        return hash(self.fingerprint())

    def __eq__(self, other) -> bool:
        if not isinstance(other, Request):
            return False
        return self.fingerprint() == other.fingerprint()
