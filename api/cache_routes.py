# -*- coding: utf-8 -*-
"""
缓存管理 API 路由

包含缓存统计、清理、预热等功能。
"""
from typing import Optional

from fastapi import APIRouter, Depends
from loguru import logger

from api.deps import verify_token, get_cache, get_seo
from api.schemas import CacheWarmupRequest
from core.seo_core import SEOCore
from core.html_cache_manager import HTMLCacheManager
from core.keyword_cache_pool import get_keyword_cache_pool
from core.image_cache_pool import get_image_cache_pool

router = APIRouter(prefix="/api/cache", tags=["缓存管理"])


@router.get("/stats")
async def cache_stats(
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """获取缓存统计"""
    return cache.get_stats()


@router.get("/pools/stats")
async def get_cache_pools_stats(_: bool = Depends(verify_token)):
    """获取关键词和图片缓存池统计"""
    keyword_pool = get_keyword_cache_pool()
    image_pool = get_image_cache_pool()

    return {
        "keyword_pool": keyword_pool.get_stats() if keyword_pool else None,
        "image_pool": image_pool.get_stats() if image_pool else None,
    }


@router.post("/clear")
async def clear_cache(
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """清空全部缓存"""
    count = await cache.clear()
    return {"success": True, "cleared": count}


@router.post("/clear/{domain}")
async def clear_domain_cache(
    domain: str,
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """清空指定域名缓存"""
    count = await cache.clear(domain)
    return {"success": True, "cleared": count}


@router.get("/entries")
async def list_cache_entries(
    domain: Optional[str] = None,
    offset: int = 0,
    limit: int = 50,
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """列出缓存条目"""
    if domain:
        entries = cache.list_entries(domain=domain, offset=offset, limit=limit)
        return {
            "items": [{"domain": domain, "count": len(entries), "paths": [e['path'] for e in entries]}],
            "offset": offset,
            "limit": limit
        }
    stats = cache.get_stats()
    return {
        "items": [],
        "total_entries": stats.get('total_entries', 0),
        "total_size_mb": stats.get('total_size_mb', 0),
        "offset": offset,
        "limit": limit
    }


@router.post("/warmup")
async def warmup_cache(
    request: CacheWarmupRequest,
    seo: SEOCore = Depends(get_seo),
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """缓存预热"""
    # TODO: 实现异步批量生成
    return {"success": True, "message": "Warmup task started"}
