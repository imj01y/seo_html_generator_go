"""
蜘蛛检测器模块

检测搜索引擎爬虫，支持User-Agent匹配和DNS反向验证。

主要功能:
- detect_spider(): 检测是否为蜘蛛
- verify_spider_dns(): DNS反向验证
- log_spider_visit(): 记录蜘蛛访问日志

支持的蜘蛛:
- 百度 (Baiduspider)
- 谷歌 (Googlebot)
- 必应 (Bingbot)
- 搜狗 (Sogou)
- 360 (360Spider)
- 神马 (YisouSpider)
- 头条 (Bytespider)
"""
import re
import socket
import asyncio
from typing import Any, Dict, List, NamedTuple, Optional, Tuple
from dataclasses import dataclass
from datetime import datetime
from functools import lru_cache

from loguru import logger


class SpiderInfo(NamedTuple):
    """蜘蛛信息"""
    type: str           # 蜘蛛类型标识
    name: str           # 蜘蛛名称
    dns_domains: List[str]  # DNS验证域名后缀


# 蜘蛛配置
SPIDER_CONFIG: Dict[str, SpiderInfo] = {
    'baidu': SpiderInfo(
        type='baidu',
        name='百度蜘蛛',
        dns_domains=['baidu.com', 'baidu.jp']
    ),
    'google': SpiderInfo(
        type='google',
        name='谷歌蜘蛛',
        dns_domains=['googlebot.com', 'google.com']
    ),
    'bing': SpiderInfo(
        type='bing',
        name='必应蜘蛛',
        dns_domains=['search.msn.com']
    ),
    'sogou': SpiderInfo(
        type='sogou',
        name='搜狗蜘蛛',
        dns_domains=['sogou.com']
    ),
    '360': SpiderInfo(
        type='360',
        name='360蜘蛛',
        dns_domains=['360.cn', 'so.com']
    ),
    'shenma': SpiderInfo(
        type='shenma',
        name='神马蜘蛛',
        dns_domains=['sm.cn']
    ),
    'toutiao': SpiderInfo(
        type='toutiao',
        name='头条蜘蛛',
        dns_domains=['bytedance.com']
    ),
    'yandex': SpiderInfo(
        type='yandex',
        name='Yandex蜘蛛',
        dns_domains=['yandex.ru', 'yandex.com', 'yandex.net']
    ),
}

# User-Agent匹配规则
UA_PATTERNS: List[Tuple[str, re.Pattern]] = [
    ('baidu', re.compile(r'Baiduspider|Baidu-YunGuanCe', re.I)),
    ('google', re.compile(r'Googlebot|Google-InspectionTool|Mediapartners-Google', re.I)),
    ('bing', re.compile(r'bingbot|msnbot|BingPreview', re.I)),
    ('sogou', re.compile(r'Sogou\s*(web\s*)?spider|Sogou\s*inst\s*spider', re.I)),
    ('360', re.compile(r'360Spider|HaosouSpider|360JK', re.I)),
    ('shenma', re.compile(r'YisouSpider|Yisouspider', re.I)),
    ('toutiao', re.compile(r'Bytespider|Bytedance', re.I)),
    ('yandex', re.compile(r'YandexBot|YandexImages|YandexMobileBot', re.I)),
]


@dataclass
class DetectionResult:
    """检测结果"""
    is_spider: bool
    spider_type: Optional[str] = None
    spider_name: Optional[str] = None
    dns_verified: bool = False
    ip: Optional[str] = None
    user_agent: Optional[str] = None


class SpiderDetector:
    """
    蜘蛛检测器

    通过User-Agent和DNS反向验证识别搜索引擎爬虫。

    Attributes:
        enable_dns_verify: 是否启用DNS验证
        dns_verify_types: 需要DNS验证的蜘蛛类型列表
        cache_ttl: DNS缓存时间（秒）

    Example:
        >>> detector = SpiderDetector()
        >>> result = detector.detect("Baiduspider", "220.181.108.95")
        >>> print(result.is_spider, result.spider_name)
        True 百度蜘蛛
    """

    def __init__(
        self,
        enable_dns_verify: bool = True,
        dns_verify_types: Optional[List[str]] = None,
        dns_timeout: float = 2.0
    ):
        """
        初始化检测器

        Args:
            enable_dns_verify: 是否启用DNS反向验证
            dns_verify_types: 需要DNS验证的蜘蛛类型
            dns_timeout: DNS查询超时时间（秒）
        """
        self.enable_dns_verify = enable_dns_verify
        self.dns_verify_types = dns_verify_types or ['baidu', 'google', 'bing']
        self.dns_timeout = dns_timeout
        self._dns_cache: Dict[str, Tuple[str, datetime]] = {}

    def detect(
        self,
        user_agent: str,
        ip: Optional[str] = None
    ) -> DetectionResult:
        """
        检测是否为蜘蛛（同步版本）

        Args:
            user_agent: User-Agent字符串
            ip: 客户端IP地址

        Returns:
            DetectionResult检测结果
        """
        # UA匹配
        spider_type = self._match_user_agent(user_agent)

        if not spider_type:
            return DetectionResult(
                is_spider=False,
                ip=ip,
                user_agent=user_agent
            )

        spider_info = SPIDER_CONFIG.get(spider_type)

        # DNS验证
        dns_verified = False
        if (
            self.enable_dns_verify
            and ip
            and spider_type in self.dns_verify_types
        ):
            dns_verified = self._verify_dns_sync(ip, spider_info.dns_domains)

        return DetectionResult(
            is_spider=True,
            spider_type=spider_type,
            spider_name=spider_info.name if spider_info else spider_type,
            dns_verified=dns_verified,
            ip=ip,
            user_agent=user_agent
        )

    async def detect_async(
        self,
        user_agent: str
    ) -> DetectionResult:
        """
        检测是否为蜘蛛（异步版本，仅UA匹配）

        Args:
            user_agent: User-Agent字符串

        Returns:
            DetectionResult检测结果
        """
        # UA匹配
        spider_type = self._match_user_agent(user_agent)

        if not spider_type:
            return DetectionResult(
                is_spider=False,
                user_agent=user_agent
            )

        spider_info = SPIDER_CONFIG.get(spider_type)

        return DetectionResult(
            is_spider=True,
            spider_type=spider_type,
            spider_name=spider_info.name if spider_info else spider_type,
            user_agent=user_agent
        )

    @lru_cache(maxsize=1000)
    def _match_user_agent(self, user_agent: str) -> Optional[str]:
        """
        匹配User-Agent

        Args:
            user_agent: UA字符串

        Returns:
            匹配的蜘蛛类型或None
        """
        if not user_agent:
            return None

        for spider_type, pattern in UA_PATTERNS:
            if pattern.search(user_agent):
                return spider_type

        return None

    def _get_cached_hostname(self, ip: str) -> Optional[str]:
        """从缓存获取主机名，过期返回None"""
        if ip not in self._dns_cache:
            return None
        hostname, cached_time = self._dns_cache[ip]
        if (datetime.now() - cached_time).total_seconds() >= 3600:
            return None
        return hostname

    def _matches_valid_domain(self, hostname: str, valid_domains: List[str]) -> bool:
        """检查主机名是否匹配有效域名后缀"""
        return any(hostname.endswith(d) for d in valid_domains)

    def _verify_dns_sync(
        self,
        ip: str,
        valid_domains: List[str]
    ) -> bool:
        """
        DNS反向验证（同步）

        Args:
            ip: IP地址
            valid_domains: 有效的域名后缀列表

        Returns:
            是否验证通过
        """
        try:
            # 检查缓存
            cached_hostname = self._get_cached_hostname(ip)
            if cached_hostname:
                return self._matches_valid_domain(cached_hostname, valid_domains)

            # 反向DNS查询
            hostname, _, _ = socket.gethostbyaddr(ip)

            # 缓存结果
            self._dns_cache[ip] = (hostname, datetime.now())

            return self._matches_valid_domain(hostname, valid_domains)

        except (socket.herror, socket.gaierror, socket.timeout) as e:
            logger.debug(f"DNS verification failed for {ip}: {e}")
            return False

    async def _verify_dns_async(
        self,
        ip: str,
        valid_domains: List[str]
    ) -> bool:
        """
        DNS反向验证（异步）

        Args:
            ip: IP地址
            valid_domains: 有效的域名后缀列表

        Returns:
            是否验证通过
        """
        try:
            # 检查缓存
            cached_hostname = self._get_cached_hostname(ip)
            if cached_hostname:
                return self._matches_valid_domain(cached_hostname, valid_domains)

            # 异步反向DNS查询
            loop = asyncio.get_event_loop()
            hostname, _, _ = await asyncio.wait_for(
                loop.run_in_executor(None, socket.gethostbyaddr, ip),
                timeout=self.dns_timeout
            )

            # 缓存结果
            self._dns_cache[ip] = (hostname, datetime.now())

            return self._matches_valid_domain(hostname, valid_domains)

        except (socket.herror, socket.gaierror, socket.timeout, asyncio.TimeoutError) as e:
            logger.debug(f"DNS verification failed for {ip}: {e}")
            return False

    def is_spider(self, user_agent: str) -> bool:
        """
        快速检测是否为蜘蛛（仅UA匹配）

        Args:
            user_agent: UA字符串

        Returns:
            是否为蜘蛛
        """
        return self._match_user_agent(user_agent) is not None

    def get_spider_type(self, user_agent: str) -> Optional[str]:
        """
        获取蜘蛛类型

        Args:
            user_agent: UA字符串

        Returns:
            蜘蛛类型或None
        """
        return self._match_user_agent(user_agent)

    def clear_cache(self) -> None:
        """清空DNS缓存"""
        self._dns_cache.clear()
        self._match_user_agent.cache_clear()

    def get_stats(self) -> Dict[str, Any]:
        """获取缓存统计"""
        return {
            'dns_cache_size': len(self._dns_cache),
            'ua_cache_info': self._match_user_agent.cache_info()._asdict()
        }


# 全局检测器实例
_detector: Optional[SpiderDetector] = None


def get_spider_detector() -> SpiderDetector:
    """获取全局蜘蛛检测器"""
    global _detector
    if _detector is None:
        _detector = SpiderDetector()
    return _detector


def init_spider_detector(
    enable_dns_verify: bool = True,
    dns_verify_types: Optional[List[str]] = None,
    dns_timeout: float = 2.0
) -> SpiderDetector:
    """
    初始化全局蜘蛛检测器

    Args:
        enable_dns_verify: 是否启用DNS验证
        dns_verify_types: 需要DNS验证的类型
        dns_timeout: DNS超时时间

    Returns:
        SpiderDetector实例
    """
    global _detector
    _detector = SpiderDetector(
        enable_dns_verify=enable_dns_verify,
        dns_verify_types=dns_verify_types,
        dns_timeout=dns_timeout
    )
    return _detector


def is_spider(user_agent: str) -> bool:
    """
    快捷函数 - 检测是否为蜘蛛

    Args:
        user_agent: UA字符串

    Returns:
        是否为蜘蛛
    """
    return get_spider_detector().is_spider(user_agent)


def detect_spider(
    user_agent: str,
    ip: Optional[str] = None
) -> DetectionResult:
    """
    快捷函数 - 完整检测

    Args:
        user_agent: UA字符串
        ip: IP地址

    Returns:
        DetectionResult
    """
    return get_spider_detector().detect(user_agent, ip)


async def detect_spider_async(user_agent: str) -> DetectionResult:
    """
    快捷函数 - 异步检测（仅UA匹配）

    Args:
        user_agent: UA字符串

    Returns:
        DetectionResult
    """
    return await get_spider_detector().detect_async(user_agent)
