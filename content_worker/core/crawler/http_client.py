# -*- coding: utf-8 -*-
"""
异步HTTP客户端

特性：
- 支持 SOCKS5 代理（URI格式）
- 自动重试机制（默认3次）
- 递增延迟重试
"""

import asyncio
from typing import Any, Dict, Optional
from loguru import logger

try:
    import httpx
except ImportError:
    httpx = None
    logger.warning("httpx not installed, run: pip install httpx httpx-socks")


class AsyncHttpClient:
    """
    支持代理和自动重试的异步HTTP客户端

    使用示例：
        client = AsyncHttpClient(proxy_url="socks5://127.0.0.1:1080")
        html = await client.fetch("https://example.com")
    """

    DEFAULT_HEADERS = {
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
        "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
        "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
        "Accept-Encoding": "gzip, deflate, br",
        "Connection": "keep-alive",
        "Cache-Control": "max-age=0",
        "Upgrade-Insecure-Requests": "1",
        "Sec-Fetch-Dest": "document",
        "Sec-Fetch-Mode": "navigate",
        "Sec-Fetch-Site": "none",
        "Sec-Fetch-User": "?1",
        "sec-ch-ua": '"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"',
        "sec-ch-ua-mobile": "?0",
        "sec-ch-ua-platform": '"Windows"',
    }

    def __init__(
        self,
        proxy_url: Optional[str] = None,
        timeout: float = 30.0,
        max_retries: int = 3,
        retry_delay: float = 1.0,
        headers: Optional[Dict[str, str]] = None
    ):
        """
        初始化HTTP客户端

        Args:
            proxy_url: 代理URI，如 socks5://127.0.0.1:1080 或 socks5://user:pass@host:port
            timeout: 请求超时时间（秒）
            max_retries: 最大重试次数（默认3次）
            retry_delay: 重试基础间隔（秒），实际间隔会递增
            headers: 自定义请求头
        """
        self.proxy_url = proxy_url
        self.timeout = timeout
        self.max_retries = max_retries
        self.retry_delay = retry_delay
        self.headers = {**self.DEFAULT_HEADERS, **(headers or {})}
        self.last_error: Optional[str] = None  # 保存最后的错误信息

    async def fetch(
        self,
        url: str,
        method: str = 'GET',
        headers: Optional[Dict[str, str]] = None,
        body: Optional[str] = None,
        json: Optional[Dict[str, Any]] = None,
        **kwargs
    ) -> Optional[bytes]:
        """
        抓取页面内容

        失败会自动重试，重试间隔递增。超过最大重试次数返回 None。

        Args:
            url: 目标URL
            method: HTTP方法（GET/POST/PUT/DELETE等）
            headers: 请求头（与默认请求头合并）
            body: 请求体（原始字符串）
            json: JSON数据（自动序列化，会覆盖body）
            **kwargs: 传递给 httpx.AsyncClient 的额外参数

        Returns:
            页面原始字节内容，失败返回 None
        """
        if httpx is None:
            self.last_error = "httpx not installed"
            logger.error(self.last_error)
            return None

        # 合并请求头
        merged_headers = {**self.headers}
        if headers:
            merged_headers.update(headers)

        # 处理请求体和 Content-Type
        request_content = None
        request_json = None
        if json is not None:
            # JSON 数据：httpx 会自动设置 Content-Type: application/json
            request_json = json
        elif body is not None:
            # 原始请求体
            request_content = body

        self.last_error = None  # 清除上次错误
        last_error = None

        # 从 kwargs 提取 timeout（如果有的话优先使用）
        timeout = kwargs.pop('timeout', self.timeout)

        # 标准化 HTTP 方法
        method = method.upper()

        for attempt in range(self.max_retries):
            try:
                async with httpx.AsyncClient(
                    proxy=self.proxy_url,
                    timeout=timeout,
                    follow_redirects=True,
                    headers=merged_headers,
                    **kwargs
                ) as client:
                    response = await client.request(
                        method=method,
                        url=url,
                        content=request_content,
                        json=request_json,
                    )
                    response.raise_for_status()
                    return response.content

            except asyncio.CancelledError:
                # 任务被取消，设置错误信息并向上传播
                self.last_error = "请求已取消"
                logger.info(f"Request cancelled: {url[:50]}")
                raise

            except httpx.TimeoutException as e:
                last_error = e
                logger.warning(
                    f"Timeout fetching {url} (attempt {attempt + 1}/{self.max_retries})"
                )
            except httpx.HTTPStatusError as e:
                last_error = e
                status = e.response.status_code
                logger.warning(
                    f"HTTP {status} fetching {url} (attempt {attempt + 1}/{self.max_retries})"
                )
                # 4xx 错误不重试
                if 400 <= status < 500:
                    break
            except Exception as e:
                last_error = e
                logger.warning(
                    f"Error fetching {url} (attempt {attempt + 1}/{self.max_retries}): {e}"
                )

            # 重试前等待（递增延迟）
            if attempt < self.max_retries - 1:
                delay = self.retry_delay * (attempt + 1)
                try:
                    await asyncio.sleep(delay)
                except asyncio.CancelledError:
                    # 等待期间被取消
                    self.last_error = "请求已取消"
                    logger.info(f"Retry sleep cancelled: {url[:50]}")
                    raise

        # 保存错误信息供调用方使用
        if last_error:
            if isinstance(last_error, httpx.TimeoutException):
                self.last_error = f"请求超时 (timeout={timeout}s)"
            elif isinstance(last_error, httpx.HTTPStatusError):
                self.last_error = f"HTTP {last_error.response.status_code}"
            else:
                self.last_error = f"{type(last_error).__name__}: {last_error}"
        else:
            self.last_error = "未知错误"

        logger.error(f"Failed to fetch {url} after {self.max_retries} retries: {last_error}")
        return None

    async def fetch_with_encoding(
        self,
        url: str,
        method: str = 'GET',
        encoding: str = 'utf-8',
        body: Optional[str] = None,
        json: Optional[Dict[str, Any]] = None,
        **kwargs
    ) -> Optional[str]:
        """
        抓取页面并指定编码

        Args:
            url: 目标URL
            method: HTTP方法
            encoding: 字符编码
            body: 请求体
            json: JSON数据
            **kwargs: 额外参数

        Returns:
            页面内容
        """
        if httpx is None:
            return None

        # 处理请求体
        request_content = None
        request_json = None
        if json is not None:
            request_json = json
        elif body is not None:
            request_content = body

        # 从 kwargs 提取 timeout（如果有的话优先使用）
        timeout = kwargs.pop('timeout', self.timeout)
        last_error = None

        # 标准化 HTTP 方法
        method = method.upper()

        for attempt in range(self.max_retries):
            try:
                async with httpx.AsyncClient(
                    proxy=self.proxy_url,
                    timeout=timeout,
                    follow_redirects=True,
                    headers=self.headers,
                    **kwargs
                ) as client:
                    response = await client.request(
                        method=method,
                        url=url,
                        content=request_content,
                        json=request_json,
                    )
                    response.raise_for_status()
                    return response.content.decode(encoding, errors='ignore')

            except Exception as e:
                last_error = e
                if attempt < self.max_retries - 1:
                    await asyncio.sleep(self.retry_delay * (attempt + 1))

        logger.error(f"Failed to fetch {url}: {last_error}")
        return None

    async def test_connection(self, test_url: str = "https://httpbin.org/ip") -> bool:
        """
        测试连接（包括代理）

        Args:
            test_url: 测试URL

        Returns:
            是否连接成功
        """
        try:
            result = await self.fetch(test_url)
            return result is not None
        except Exception as e:
            logger.error(f"Connection test failed: {e}")
            return False

    def __repr__(self) -> str:
        proxy_info = f"proxy={self.proxy_url}" if self.proxy_url else "no proxy"
        return f"AsyncHttpClient({proxy_info}, timeout={self.timeout}s, retries={self.max_retries})"
