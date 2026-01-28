# -*- coding: utf-8 -*-
"""
系统设置 API 路由

包含系统配置、API Token 管理、生成器队列管理、告警等功能。
"""
from datetime import datetime
from typing import Optional

from fastapi import APIRouter, Depends, Query
from loguru import logger

from api.deps import verify_token, get_conf, get_redis_client
from config import Config
from core.keyword_group_manager import get_keyword_group
from core.image_group_manager import get_image_group
from core.content_pool_manager import get_content_pool_manager
from database.db import fetch_one, fetch_all, fetch_value, execute_query, insert

router = APIRouter(tags=["系统设置"])


# ============================================
# 系统配置API
# ============================================

@router.get("/api/settings")
async def get_settings(
    config: Config = Depends(get_conf),
    _: bool = Depends(verify_token)
):
    """获取系统配置（从配置文件）"""
    return {
        "server": {
            "host": config.server.host,
            "port": config.server.port
        },
        "cache": {
            "enabled": config.cache.enabled,
            "max_size_gb": config.cache.max_size_gb,
            "gzip_enabled": config.cache.gzip_enabled
        },
        "seo": {
            "internal_links_count": config.seo.internal_links_count,
            "encoding_mix_ratio": config.seo.encoding_mix_ratio,
            "emoji_count_min": config.seo.emoji_count_min,
            "emoji_count_max": config.seo.emoji_count_max
        },
        "spider_detector": {
            "enabled": config.spider_detector.enabled,
            "dns_verify_enabled": config.spider_detector.dns_verify_enabled,
            "return_404_for_non_spider": config.spider_detector.return_404_for_non_spider
        }
    }


def _convert_setting_value(value: str, stype: str):
    """根据类型转换设置值"""
    if stype == 'number':
        return float(value) if '.' in value else int(value)
    if stype == 'boolean':
        return value.lower() in ('true', '1', 'yes')
    if stype == 'json':
        import json
        return json.loads(value)
    return value


CACHE_DEFAULT_SETTINGS = {
    'keyword_cache_ttl': ('86400', 'number', '关键词缓存过期时间(秒)'),
    'image_cache_ttl': ('86400', 'number', '图片URL缓存过期时间(秒)'),
    'cache_compress_enabled': ('true', 'boolean', '是否启用缓存压缩'),
    'cache_compress_level': ('6', 'number', '压缩级别(1-9)'),
    'keyword_pool_size': ('500000', 'number', '关键词池大小(0=不限制)'),
    'image_pool_size': ('500000', 'number', '图片池大小(0=不限制)'),
    'article_pool_size': ('50000', 'number', '文章池大小(0=不限制)'),
    'file_cache_enabled': ('false', 'boolean', '是否启用文件缓存'),
    'file_cache_dir': ('./html_cache', 'string', '文件缓存目录'),
    'file_cache_max_size_gb': ('50', 'number', '最大缓存大小(GB)'),
    'file_cache_nginx_mode': ('true', 'boolean', 'Nginx直服模式(不压缩)')
}


@router.get("/api/settings/cache")
async def get_cache_settings(_: bool = Depends(verify_token)):
    """获取缓存配置（从数据库）"""
    try:
        settings = await fetch_all(
            "SELECT setting_key, setting_value, setting_type, description FROM system_settings"
        )

        result = {}
        existing_keys = set()

        for s in settings or []:
            key = s['setting_key']
            existing_keys.add(key)
            result[key] = {
                'value': _convert_setting_value(s['setting_value'], s['setting_type']),
                'type': s['setting_type'],
                'description': s['description']
            }

        for key, (default_value, stype, description) in CACHE_DEFAULT_SETTINGS.items():
            if key in existing_keys:
                continue

            await insert('system_settings', {
                'setting_key': key,
                'setting_value': default_value,
                'setting_type': stype,
                'description': description
            })

            result[key] = {
                'value': _convert_setting_value(default_value, stype),
                'type': stype,
                'description': description
            }

        return {"success": True, "settings": result}
    except Exception as e:
        logger.error(f"Failed to get cache settings: {e}")
        return {"success": False, "message": str(e), "settings": {}}


@router.put("/api/settings/cache")
async def update_cache_settings(
    data: dict,
    _: bool = Depends(verify_token)
):
    """更新缓存配置"""
    try:
        updated = 0
        for key, value in data.items():
            existing = await fetch_one(
                "SELECT id FROM system_settings WHERE setting_key = %s",
                (key,)
            )
            if existing:
                await execute_query(
                    "UPDATE system_settings SET setting_value = %s WHERE setting_key = %s",
                    (str(value), key)
                )
                updated += 1
            else:
                stype = 'string'
                if isinstance(value, bool):
                    stype = 'boolean'
                    value = 'true' if value else 'false'
                elif isinstance(value, (int, float)):
                    stype = 'number'
                await insert('system_settings', {
                    'setting_key': key,
                    'setting_value': str(value),
                    'setting_type': stype
                })
                updated += 1

        return {"success": True, "updated": updated}
    except Exception as e:
        logger.error(f"Failed to update cache settings: {e}")
        return {"success": False, "message": str(e)}


@router.post("/api/settings/cache/apply")
async def apply_cache_settings(
    _: bool = Depends(verify_token)
):
    """
    应用缓存配置到运行时

    注意: 文件缓存配置变更需要重启服务才能完全生效。
    """
    try:
        settings = await fetch_all(
            "SELECT setting_key, setting_value, setting_type FROM system_settings "
            "WHERE setting_key LIKE '%cache%' OR setting_key LIKE '%ttl%' OR setting_key LIKE '%pool_size%'"
        )

        config = {}
        for s in settings or []:
            config[s['setting_key']] = _convert_setting_value(s['setting_value'], s['setting_type'])

        applied = []

        file_cache_enabled = config.get('file_cache_enabled', False)
        if file_cache_enabled:
            applied.append("file_cache_enabled=true (需要重启服务生效)")

        pool_configs = [
            ('keyword', get_keyword_group(), config.get('keyword_pool_size', 0)),
            ('image', get_image_group(), config.get('image_pool_size', 0)),
        ]

        for name, group, pool_size in pool_configs:
            if not group:
                continue
            pool_size = int(pool_size)
            await group.reload(max_size=pool_size)
            applied.append(f"{name}_pool reloaded ({group.total_count()} items, limit={pool_size})")

        return {
            "success": True,
            "message": "配置已应用" + ("，部分配置需要重启服务生效" if file_cache_enabled else ""),
            "applied": applied
        }
    except Exception as e:
        logger.error(f"Failed to apply cache settings: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/settings/database")
async def check_database(_: bool = Depends(verify_token)):
    """检查数据库连接"""
    from database import get_db_pool
    pool = get_db_pool()

    if pool is None:
        return {"connected": False, "error": "Pool not initialized"}

    return {
        "connected": True,
        "pool_size": pool.maxsize,
        "free_connections": pool.freesize
    }


# ============================================
# API Token 管理
# ============================================

@router.get("/api/settings/api-token")
async def get_api_token_settings(_: dict = Depends(verify_token)):
    """获取 API Token 设置"""
    try:
        token = await fetch_value("SELECT setting_value FROM system_settings WHERE setting_key = 'api_token'")
        enabled = await fetch_value("SELECT setting_value FROM system_settings WHERE setting_key = 'api_token_enabled'")
        return {
            "success": True,
            "token": token or "",
            "enabled": (enabled or 'true').lower() == 'true'
        }
    except Exception as e:
        logger.error(f"Failed to get API token settings: {e}")
        return {"success": False, "message": str(e)}


@router.put("/api/settings/api-token")
async def update_api_token_settings(data: dict, _: dict = Depends(verify_token)):
    """更新 API Token 设置"""
    try:
        token = data.get('token', '')
        enabled = data.get('enabled', True)

        if token:
            existing = await fetch_value("SELECT id FROM system_settings WHERE setting_key = 'api_token'")
            if existing:
                await execute_query(
                    "UPDATE system_settings SET setting_value = %s WHERE setting_key = 'api_token'",
                    (token,)
                )
            else:
                await insert('system_settings', {
                    'setting_key': 'api_token',
                    'setting_value': token,
                    'description': 'API Token for external access'
                })

        enabled_str = 'true' if enabled else 'false'
        existing_enabled = await fetch_value("SELECT id FROM system_settings WHERE setting_key = 'api_token_enabled'")
        if existing_enabled:
            await execute_query(
                "UPDATE system_settings SET setting_value = %s WHERE setting_key = 'api_token_enabled'",
                (enabled_str,)
            )
        else:
            await insert('system_settings', {
                'setting_key': 'api_token_enabled',
                'setting_value': enabled_str,
                'description': 'Enable API Token authentication'
            })

        return {"success": True, "message": "API Token 设置已更新"}
    except Exception as e:
        logger.error(f"Failed to update API token settings: {e}")
        return {"success": False, "message": str(e)}


@router.post("/api/settings/api-token/generate")
async def generate_api_token(_: dict = Depends(verify_token)):
    """生成新的随机 API Token"""
    import secrets
    token = f"seo_{secrets.token_hex(16)}"
    return {"success": True, "token": token}


# ============================================
# GeneratorWorker 队列管理API
# ============================================

@router.get("/api/generator/queue/stats")
async def get_generator_queue_stats(
    group_id: int = Query(default=1, description="分组ID"),
    _: bool = Depends(verify_token)
):
    """
    获取待处理队列统计信息

    Returns:
        queue_size: 队列中待处理的文章数量
        group_id: 分组ID
    """
    try:
        redis_client = get_redis_client()
        if not redis_client:
            return {"success": False, "message": "Redis not initialized"}
        queue_key = f"pending:articles:{group_id}"
        queue_size = await redis_client.llen(queue_key)
        return {
            "success": True,
            "group_id": group_id,
            "queue_size": queue_size
        }
    except Exception as e:
        logger.error(f"Failed to get queue stats: {e}")
        return {"success": False, "message": str(e)}


@router.post("/api/generator/queue/push")
async def push_articles_to_queue(
    group_id: int = Query(default=1, description="分组ID"),
    limit: int = Query(default=1000, ge=1, le=100000, description="推送数量限制"),
    status: int = Query(default=1, description="文章状态筛选"),
    _: bool = Depends(verify_token)
):
    """
    批量推送文章到待处理队列

    从 original_articles 表获取未处理的文章ID，推送到 Redis 队列。
    GeneratorWorker 会从队列中获取文章ID进行处理。

    Args:
        group_id: 分组ID
        limit: 推送数量限制
        status: 文章状态（1=正常）
    """
    try:
        redis_client = get_redis_client()
        if not redis_client:
            return {"success": False, "message": "Redis not initialized"}

        queue_key = f"pending:articles:{group_id}"
        existing_ids = set()
        existing_items = await redis_client.lrange(queue_key, 0, -1)
        for item in existing_items:
            try:
                existing_ids.add(int(item))
            except (ValueError, TypeError):
                pass

        articles = await fetch_all(
            """
            SELECT id FROM original_articles
            WHERE group_id = %s AND status = %s
            ORDER BY id DESC
            LIMIT %s
            """,
            (group_id, status, limit)
        )

        if not articles:
            return {"success": True, "pushed": 0, "skipped": 0, "message": "没有待处理的文章"}

        new_ids = [a['id'] for a in articles if a['id'] not in existing_ids]
        skipped = len(articles) - len(new_ids)

        if not new_ids:
            return {"success": True, "pushed": 0, "skipped": skipped, "message": "所有文章已在队列中"}

        pipe = redis_client.pipeline()
        for article_id in new_ids:
            pipe.lpush(queue_key, article_id)
        await pipe.execute()

        return {
            "success": True,
            "pushed": len(new_ids),
            "skipped": skipped,
            "message": f"已推送 {len(new_ids)} 篇文章到队列"
        }

    except Exception as e:
        logger.error(f"Failed to push articles to queue: {e}")
        return {"success": False, "message": str(e)}


@router.post("/api/generator/queue/clear")
async def clear_generator_queue(
    group_id: int = Query(default=1, description="分组ID"),
    _: bool = Depends(verify_token)
):
    """
    清空待处理队列

    Args:
        group_id: 分组ID
    """
    try:
        redis_client = get_redis_client()
        if not redis_client:
            return {"success": False, "message": "Redis not initialized"}
        queue_key = f"pending:articles:{group_id}"
        queue_size = await redis_client.llen(queue_key)
        await redis_client.delete(queue_key)
        return {
            "success": True,
            "cleared": queue_size,
            "message": f"已清空队列，共删除 {queue_size} 条"
        }
    except Exception as e:
        logger.error(f"Failed to clear queue: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/generator/worker/status")
async def get_generator_worker_status(_: bool = Depends(verify_token)):
    """获取GeneratorWorker运行状态"""
    from core.initializers import _generator_worker
    return {
        "success": True,
        "running": _generator_worker is not None and _generator_worker._running,
        "worker_initialized": _generator_worker is not None
    }


@router.get("/api/generator/stats")
async def get_generator_stats(
    _: bool = Depends(verify_token)
):
    """
    获取生成器统计信息

    包含 titles 和 contents 表的数据量统计
    """
    try:
        titles_count = await fetch_value("SELECT COUNT(*) FROM titles") or 0
        contents_count = await fetch_value("SELECT COUNT(*) FROM contents") or 0
        articles_count = await fetch_value("SELECT COUNT(*) FROM original_articles WHERE status = 1") or 0

        return {
            "success": True,
            "titles_count": titles_count,
            "contents_count": contents_count,
            "original_articles_count": articles_count
        }
    except Exception as e:
        logger.error(f"Failed to get generator stats: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 告警API
# ============================================

@router.get("/api/alerts/content-pool")
async def get_content_pool_alert(
    _: bool = Depends(verify_token)
):
    """
    获取段落池告警状态

    Returns:
        告警状态，包括级别、消息、池大小等
    """
    content_pool = get_content_pool_manager()
    if not content_pool:
        return {
            "success": False,
            "message": "ContentPoolManager not initialized",
            "alert": {
                "level": "unknown",
                "message": "段落池管理器未初始化",
                "pool_size": 0,
                "used_size": 0,
                "total": 0,
                "updated_at": ""
            }
        }

    alert = await content_pool.get_alert_status()
    return {"success": True, "alert": alert}


@router.post("/api/alerts/content-pool/reset")
async def reset_content_pool(
    _: bool = Depends(verify_token)
):
    """
    重置段落池

    清空已用池，从数据库重新加载所有 ID 到可用池
    """
    content_pool = get_content_pool_manager()
    if not content_pool:
        return {"success": False, "message": "ContentPoolManager not initialized"}

    count = await content_pool.reset_pool()
    return {
        "success": True,
        "message": f"段落池已重置，加载了 {count} 条数据",
        "count": count
    }


# ============================================
# 蜘蛛检测API
# ============================================

@router.get("/api/spiders/config")
async def get_spider_config(_: bool = Depends(verify_token)):
    """获取蜘蛛检测配置"""
    from core.spider_detector import get_spider_detector
    detector = get_spider_detector()
    return {
        "enabled": True,
        "dns_verify_enabled": detector.enable_dns_verify,
        "dns_verify_types": detector.dns_verify_types,
        "dns_timeout": detector.dns_timeout
    }


@router.post("/api/spiders/test")
async def test_spider_detection(
    user_agent: str = Query(..., description="User-Agent字符串"),
    _: bool = Depends(verify_token)
):
    """测试蜘蛛检测"""
    from core.spider_detector import detect_spider_async
    result = await detect_spider_async(user_agent)
    return {
        "is_spider": result.is_spider,
        "spider_type": result.spider_type,
        "spider_name": result.spider_name
    }


@router.get("/api/spiders/logs")
async def get_spider_logs(
    spider_type: Optional[str] = None,
    domain: Optional[str] = None,
    start_date: Optional[str] = None,
    end_date: Optional[str] = None,
    page: int = 1,
    page_size: int = 50,
    _: bool = Depends(verify_token)
):
    """
    获取蜘蛛访问日志

    Args:
        spider_type: 筛选蜘蛛类型 (baidu/google/bing等)
        domain: 筛选域名
        start_date: 开始日期 (YYYY-MM-DD)
        end_date: 结束日期 (YYYY-MM-DD)
        page: 页码
        page_size: 每页数量
    """
    try:
        offset = (page - 1) * page_size

        where_clauses = []
        params = []

        if spider_type:
            where_clauses.append("spider_type = %s")
            params.append(spider_type)

        if domain:
            where_clauses.append("domain = %s")
            params.append(domain)

        if start_date:
            where_clauses.append("DATE(created_at) >= %s")
            params.append(start_date)

        if end_date:
            where_clauses.append("DATE(created_at) <= %s")
            params.append(end_date)

        where_clause = " AND ".join(where_clauses) if where_clauses else "1=1"

        count_sql = f"SELECT COUNT(*) FROM spider_logs WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

        params.extend([page_size, offset])
        list_sql = f"""
            SELECT id, spider_type, ip, ua, domain, path,
                   dns_ok, resp_time, cache_hit, status, created_at
            FROM spider_logs
            WHERE {where_clause}
            ORDER BY created_at DESC
            LIMIT %s OFFSET %s
        """
        items = await fetch_all(list_sql, tuple(params))

        return {
            "items": items or [],
            "total": total,
            "page": page,
            "page_size": page_size
        }
    except Exception as e:
        logger.error(f"Failed to get spider logs: {e}")
        return {"items": [], "total": 0, "page": page, "page_size": page_size}


@router.get("/api/spiders/stats")
async def get_spider_stats(_: bool = Depends(verify_token)):
    """
    获取蜘蛛详细统计信息

    返回按类型、按域名、按状态码等多维度统计
    """
    try:
        total = await fetch_value("SELECT COUNT(*) FROM spider_logs") or 0
        today_total = await fetch_value(
            "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE()"
        ) or 0

        by_type = await fetch_all("""
            SELECT spider_type, COUNT(*) as count
            FROM spider_logs
            GROUP BY spider_type
            ORDER BY count DESC
        """) or []

        by_domain = await fetch_all("""
            SELECT domain, COUNT(*) as count
            FROM spider_logs
            GROUP BY domain
            ORDER BY count DESC
            LIMIT 10
        """) or []

        by_status = await fetch_all("""
            SELECT status, COUNT(*) as count
            FROM spider_logs
            GROUP BY status
            ORDER BY status
        """) or []

        cache_stats = await fetch_all("""
            SELECT cache_hit, COUNT(*) as count
            FROM spider_logs
            GROUP BY cache_hit
        """) or []

        cache_hit_count = 0
        cache_miss_count = 0
        for row in cache_stats:
            if row['cache_hit'] == 1:
                cache_hit_count = row['count']
            else:
                cache_miss_count = row['count']

        hit_rate = 0
        if cache_hit_count + cache_miss_count > 0:
            hit_rate = round(cache_hit_count / (cache_hit_count + cache_miss_count) * 100, 2)

        avg_response = await fetch_value(
            "SELECT AVG(resp_time) FROM spider_logs"
        ) or 0

        return {
            "total": total,
            "today_total": today_total,
            "by_type": [{"type": r['spider_type'], "count": r['count']} for r in by_type],
            "by_domain": [{"domain": r['domain'], "count": r['count']} for r in by_domain],
            "by_status": [{"status": r['status'], "count": r['count']} for r in by_status],
            "cache_hit_rate": hit_rate,
            "cache_hit_count": cache_hit_count,
            "cache_miss_count": cache_miss_count,
            "avg_response_time_ms": round(float(avg_response), 2) if avg_response else 0
        }
    except Exception as e:
        logger.error(f"Failed to get spider stats: {e}")
        return {
            "total": 0,
            "today_total": 0,
            "by_type": [],
            "by_domain": [],
            "by_status": [],
            "cache_hit_rate": 0,
            "cache_hit_count": 0,
            "cache_miss_count": 0,
            "avg_response_time_ms": 0
        }


@router.get("/api/spiders/daily-stats")
async def get_spider_daily_stats(
    days: int = Query(default=7, ge=1, le=30),
    _: bool = Depends(verify_token)
):
    """
    获取每日蜘蛛访问统计

    Args:
        days: 统计天数（1-30天，默认7天）
    """
    try:
        stats = await fetch_all(f"""
            SELECT
                DATE(created_at) as date,
                spider_type,
                COUNT(*) as count,
                SUM(CASE WHEN cache_hit = 1 THEN 1 ELSE 0 END) as cache_hits,
                AVG(resp_time) as avg_response_time
            FROM spider_logs
            WHERE created_at >= DATE_SUB(NOW(), INTERVAL {days} DAY)
            GROUP BY DATE(created_at), spider_type
            ORDER BY date DESC, count DESC
        """)

        result = {}
        if stats:
            for row in stats:
                date_str = row['date'].strftime('%Y-%m-%d') if row['date'] else ''
                if date_str not in result:
                    result[date_str] = {
                        "date": date_str,
                        "total": 0,
                        "by_type": {}
                    }
                result[date_str]['total'] += row['count']
                result[date_str]['by_type'][row['spider_type']] = {
                    "count": row['count'],
                    "cache_hits": row['cache_hits'],
                    "avg_response_time": round(float(row['avg_response_time']), 2) if row['avg_response_time'] else 0
                }

        return {"days": list(result.values())}
    except Exception as e:
        logger.error(f"Failed to get daily spider stats: {e}")
        return {"days": []}


@router.get("/api/spiders/hourly-stats")
async def get_spider_hourly_stats(
    date: Optional[str] = Query(default=None, description="指定日期 YYYY-MM-DD，默认今天"),
    _: bool = Depends(verify_token)
):
    """
    获取按小时的蜘蛛访问统计

    Args:
        date: 指定日期，默认今天
    """
    try:
        date_condition = f"DATE(created_at) = '{date}'" if date else "DATE(created_at) = CURDATE()"

        stats = await fetch_all(f"""
            SELECT
                HOUR(created_at) as hour,
                spider_type,
                COUNT(*) as count
            FROM spider_logs
            WHERE {date_condition}
            GROUP BY HOUR(created_at), spider_type
            ORDER BY hour ASC
        """)

        result = {h: {"hour": h, "total": 0, "by_type": {}} for h in range(24)}

        if stats:
            for row in stats:
                hour = row['hour']
                result[hour]['total'] += row['count']
                result[hour]['by_type'][row['spider_type']] = row['count']

        return {"hours": list(result.values())}
    except Exception as e:
        logger.error(f"Failed to get hourly spider stats: {e}")
        return {"hours": []}


@router.delete("/api/spiders/logs/clear")
async def clear_spider_logs(
    before_days: int = Query(default=30, ge=1, description="清理多少天前的日志"),
    _: bool = Depends(verify_token)
):
    """
    清理旧的蜘蛛日志

    Args:
        before_days: 清理指定天数之前的日志（默认30天前）
    """
    try:
        await execute_query(
            f"DELETE FROM spider_logs WHERE created_at < DATE_SUB(NOW(), INTERVAL %s DAY)",
            (before_days,)
        )
        logger.info(f"Cleared spider logs older than {before_days} days")
        return {"success": True, "message": f"已清理 {before_days} 天前的日志"}
    except Exception as e:
        logger.error(f"Failed to clear spider logs: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 健康检查
# ============================================

@router.get("/health")
async def health_check():
    """健康检查"""
    return {"status": "healthy", "timestamp": datetime.now().isoformat()}


@router.get("/api/health")
async def api_health_check():
    """API健康检查"""
    return {"status": "healthy", "timestamp": datetime.now().isoformat()}
