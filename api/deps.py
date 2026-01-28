# -*- coding: utf-8 -*-
"""
API 依赖注入模块

统一管理所有 API 路由使用的依赖注入函数。
"""
import threading
from typing import Optional

from cachetools import TTLCache
from fastapi import HTTPException, Header, Depends
from loguru import logger

from config import get_config, Config
from core.seo_core import get_seo_core, SEOCore
from core.redis_client import get_redis_client as get_redis
from core.html_cache_manager import get_cache_manager, HTMLCacheManager
from core.keyword_group_manager import get_keyword_group, AsyncKeywordGroupManager
from core.image_group_manager import get_image_group, AsyncImageGroupManager
from core.auth import verify_token as verify_jwt_token
from database.db import fetch_one, fetch_value, get_db_pool


# ============================================
# 核心组件依赖
# ============================================

def get_seo() -> SEOCore:
    """获取SEO核心实例"""
    return get_seo_core()


def get_cache() -> HTMLCacheManager:
    """
    获取HTML缓存管理器（文件缓存）

    Returns:
        HTMLCacheManager 实例

    Raises:
        HTTPException: 缓存未初始化时抛出500错误
    """
    cache = get_cache_manager()
    if cache is None:
        raise HTTPException(
            status_code=500,
            detail="File cache not initialized."
        )
    return cache


def get_redis_client():
    """
    获取Redis客户端（用于队列等非HTML缓存操作）

    Returns:
        Redis客户端实例，如果Redis未初始化则返回None
    """
    return get_redis()


def get_conf() -> Config:
    """获取配置"""
    return get_config()


def get_image_group_dep() -> AsyncImageGroupManager:
    """获取图片分组实例"""
    group = get_image_group()
    if group is None:
        raise HTTPException(status_code=500, detail="Image group not initialized")
    return group


def get_keyword_group_dep() -> AsyncKeywordGroupManager:
    """获取关键词分组实例"""
    group = get_keyword_group()
    if group is None:
        raise HTTPException(status_code=500, detail="Keyword group not initialized")
    return group


# ============================================
# 认证依赖
# ============================================

async def verify_token(authorization: Optional[str] = Header(None)) -> dict:
    """
    验证JWT Token

    Returns:
        Token 中的用户信息
    """
    if not authorization:
        raise HTTPException(status_code=401, detail="Missing authorization header")

    # 支持 "Bearer token" 和 "token" 两种格式
    token = authorization
    if authorization.startswith("Bearer "):
        token = authorization[7:]

    # 验证 JWT
    payload = verify_jwt_token(token)
    if not payload:
        raise HTTPException(status_code=401, detail="Invalid or expired token")

    return payload


async def verify_api_token(
    authorization: Optional[str] = Header(None),
    x_api_token: Optional[str] = Header(None, alias="X-API-Token")
) -> dict:
    """
    验证 API Token（用于外部系统调用）

    支持两种方式传递 Token:
    1. X-API-Token Header
    2. Authorization Header (Bearer token 或直接 token)
    """
    token = x_api_token or (authorization[7:] if authorization and authorization.startswith("Bearer ") else authorization)

    if not token:
        raise HTTPException(status_code=401, detail="Missing API token")

    # 检查是否启用
    enabled = await fetch_value("SELECT setting_value FROM system_settings WHERE setting_key = 'api_token_enabled'")
    if enabled and enabled.lower() != 'true':
        raise HTTPException(status_code=403, detail="API Token authentication is disabled")

    # 验证 token
    stored_token = await fetch_value("SELECT setting_value FROM system_settings WHERE setting_key = 'api_token'")
    if not stored_token or token != stored_token:
        raise HTTPException(status_code=401, detail="Invalid API token")

    return {"type": "api_token"}


async def verify_token_or_api_token(
    authorization: Optional[str] = Header(None),
    x_api_token: Optional[str] = Header(None, alias="X-API-Token")
) -> dict:
    """
    双重认证：支持 JWT Token 或 API Token

    优先检查 X-API-Token Header，其次检查 Authorization Header
    以 seo_ 开头的 token 走 API Token 验证，否则走 JWT 验证
    """
    if x_api_token:
        return await verify_api_token(authorization, x_api_token)

    if not authorization:
        raise HTTPException(status_code=401, detail="Missing authorization")

    token = authorization[7:] if authorization.startswith("Bearer ") else authorization

    # 以 seo_ 开头的走 API Token 验证
    if token.startswith("seo_"):
        return await verify_api_token(authorization, token)

    # 否则走 JWT 验证
    payload = verify_jwt_token(token)
    if not payload:
        raise HTTPException(status_code=401, detail="Invalid or expired token")

    return {"type": "jwt", "payload": payload}


# ============================================
# 站点配置缓存
# ============================================

# 站点配置内存缓存（最多 500 个站点，TTL 5 分钟）
_site_config_cache: TTLCache = TTLCache(maxsize=500, ttl=300)
_site_config_lock = threading.Lock()


async def get_site_config_by_domain(domain: str) -> Optional[dict]:
    """
    根据域名获取站点配置

    Args:
        domain: 站点域名

    Returns:
        站点配置字典，包含 template, icp_number, baidu_push_js 等字段
        如果未找到或数据库未初始化则返回 None
    """
    # 检查数据库连接池是否已初始化
    if get_db_pool() is None:
        logger.debug(f"Database not initialized, skipping site config for {domain}")
        return None

    try:
        site = await fetch_one(
            "SELECT * FROM sites WHERE domain = %s AND status = 1 LIMIT 1",
            (domain,)
        )
        if site:
            logger.debug(f"Site config loaded for {domain}: template={site.get('template')}")
        return site
    except Exception as e:
        logger.warning(f"Failed to fetch site config for {domain}: {e}")
        return None


async def get_site_config_cached(domain: str) -> Optional[dict]:
    """
    带内存缓存的站点配置获取

    Args:
        domain: 站点域名

    Returns:
        站点配置字典或 None
    """
    # 先检查缓存
    with _site_config_lock:
        if domain in _site_config_cache:
            return _site_config_cache[domain]

    # 数据库查询
    site = await get_site_config_by_domain(domain)

    # 写入缓存（包括 None 值，避免缓存穿透）
    if site is not None:
        with _site_config_lock:
            _site_config_cache[domain] = site

    return site


def invalidate_site_config_cache(domain: Optional[str] = None):
    """
    清除站点配置缓存

    Args:
        domain: 指定域名，None 则清除所有
    """
    with _site_config_lock:
        if domain:
            _site_config_cache.pop(domain, None)
        else:
            _site_config_cache.clear()
