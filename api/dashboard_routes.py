# -*- coding: utf-8 -*-
"""
仪表盘 API 路由

提供仪表盘统计数据和蜘蛛访问信息。
"""
from fastapi import APIRouter, Depends
from loguru import logger

from api.deps import verify_token, get_cache
from core.keyword_group_manager import get_keyword_group
from core.image_group_manager import get_image_group
from core.html_cache_manager import HTMLCacheManager
from database.db import fetch_all, fetch_value

router = APIRouter(prefix="/api/dashboard", tags=["仪表盘"])


@router.get("/stats")
async def get_dashboard_stats(
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """获取仪表盘统计数据"""
    keyword_group = get_keyword_group()
    image_group = get_image_group()
    cache_stats = cache.get_stats()

    sites_count = 0
    today_spider_visits = 0
    today_generations = 0
    articles_count = 0

    try:
        sites_count = await fetch_value("SELECT COUNT(*) FROM sites WHERE status = 1") or 0
    except Exception as e:
        logger.warning(f"Failed to get sites count: {e}")

    try:
        today_spider_visits = await fetch_value(
            "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE()"
        ) or 0
    except Exception as e:
        logger.warning(f"Failed to get today spider visits: {e}")

    try:
        today_generations = await fetch_value(
            "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE() AND cache_hit = 0"
        ) or 0
    except Exception as e:
        logger.warning(f"Failed to get today generations: {e}")

    try:
        articles_count = await fetch_value(
            "SELECT COUNT(*) FROM original_articles WHERE status = 1"
        ) or 0
    except Exception as e:
        logger.warning(f"Failed to get articles count: {e}")

    return {
        "sites_count": sites_count,
        "keywords_count": keyword_group.get_stats()['total'] if keyword_group else 0,
        "images_count": image_group.get_stats()['total'] if image_group else 0,
        "articles_count": articles_count,
        "cache_entries": cache_stats.get('total_entries', 0),
        "cache_size_mb": cache_stats.get('total_size_mb', 0),
        "today_generations": today_generations,
        "today_spider_visits": today_spider_visits,
    }


@router.get("/spider-visits")
async def get_spider_visits(_: bool = Depends(verify_token)):
    """获取蜘蛛访问统计"""
    try:
        total = await fetch_value("SELECT COUNT(*) FROM spider_logs") or 0

        type_stats = await fetch_all("""
            SELECT spider_type, COUNT(*) as count
            FROM spider_logs
            GROUP BY spider_type
            ORDER BY count DESC
        """)

        by_type = {}
        if type_stats:
            for row in type_stats:
                by_type[row['spider_type']] = row['count']

        trend = await fetch_all("""
            SELECT DATE(created_at) as date, COUNT(*) as count
            FROM spider_logs
            WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)
            GROUP BY DATE(created_at)
            ORDER BY date ASC
        """)

        trend_data = []
        if trend:
            for row in trend:
                trend_data.append({
                    "date": row['date'].strftime('%Y-%m-%d') if row['date'] else '',
                    "count": row['count']
                })

        return {
            "total": total,
            "by_type": by_type,
            "trend": trend_data
        }
    except Exception as e:
        logger.error(f"Failed to get spider visits: {e}")
        return {
            "total": 0,
            "by_type": {},
            "trend": []
        }


@router.get("/cache-stats")
async def get_cache_stats_api(
    cache: HTMLCacheManager = Depends(get_cache),
    _: bool = Depends(verify_token)
):
    """获取缓存统计"""
    return cache.get_stats()
