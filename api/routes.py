# -*- coding: utf-8 -*-
"""
FastAPI 核心路由模块

仅保留页面服务路由（供蜘蛛访问），其他 API 路由已拆分到独立模块。

路由拆分:
- api/auth_routes.py: 认证相关 (/api/auth/*)
- api/dashboard_routes.py: 仪表盘数据 (/api/dashboard/*)
- api/site_routes.py: 站点和站群管理 (/api/sites/*, /api/site-groups/*)
- api/template_routes.py: 模板管理 (/api/templates/*)
- api/keyword_routes.py: 关键词管理 (/api/keywords/*)
- api/image_routes.py: 图片管理 (/api/images/*)
- api/article_routes.py: 文章管理 (/api/articles/*)
- api/cache_routes.py: 缓存管理 (/api/cache/*)
- api/settings_routes.py: 系统设置、蜘蛛检测、生成器队列等
"""
import time
import asyncio
import re
from typing import List
from datetime import datetime

from fastapi import APIRouter, HTTPException, Depends, Query, Request, BackgroundTasks
from fastapi.responses import HTMLResponse
from loguru import logger

from api.deps import (
    get_seo, get_cache, get_conf, get_site_config_cached
)
from config import Config
from core.seo_core import SEOCore
from core.spider_detector import detect_spider_async
from core.html_cache_manager import HTMLCacheManager
from core.title_manager import get_random_titles
from core.content_manager import get_random_content
from core.content_pool_manager import get_content_pool_manager, get_or_create_content_pool_manager
from database.db import fetch_one, insert, get_db_pool

# 创建路由器
router = APIRouter()


# ============================================
# 辅助函数
# ============================================

def _build_article_content(titles: List[str], content: str) -> str:
    """
    组装文章内容（标题 + 正文）

    Args:
        titles: 随机标题列表（需要4个）
        content: 正文内容

    Returns:
        组装好的文章内容
    """
    if not titles or len(titles) < 4:
        return content or ''

    now_str = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    return f"""{titles[0]}

{titles[1]}

{titles[2]}

厂商新闻：{titles[3]} 时间：{now_str}

编辑：admin
{now_str}

　{content}

admin】"""


def get_client_ip(request: Request) -> str:
    """
    获取客户端真实IP

    优先从 X-Forwarded-For 或 X-Real-IP 头获取（用于反向代理场景）
    """
    forwarded_for = request.headers.get('X-Forwarded-For')
    if forwarded_for:
        return forwarded_for.split(',')[0].strip()

    real_ip = request.headers.get('X-Real-IP')
    if real_ip:
        return real_ip

    if request.client:
        return request.client.host

    return '0.0.0.0'


async def log_spider_visit(
    spider_type: str,
    ip: str,
    ua: str,
    domain: str,
    path: str,
    dns_ok: bool,
    resp_time: int,
    cache_hit: bool,
    status: int
):
    """
    异步记录蜘蛛访问日志到数据库

    使用异步方式记录，不阻塞主请求流程
    """
    if get_db_pool() is None:
        logger.debug("Database not initialized, skipping spider log")
        return

    try:
        await insert('spider_logs', {
            'spider_type': spider_type or 'unknown',
            'ip': ip,
            'ua': ua[:500] if ua else '',
            'domain': domain,
            'path': path[:500] if path else '',
            'dns_ok': 1 if dns_ok else 0,
            'resp_time': resp_time,
            'cache_hit': 1 if cache_hit else 0,
            'status': status
        })
        logger.debug(f"Spider log recorded: {spider_type} -> {domain}{path}")
    except Exception as e:
        logger.warning(f"Failed to log spider visit: {e}")


# ============================================
# 页面服务路由（供蜘蛛访问）
# ============================================

@router.get("/page", response_class=HTMLResponse)
async def serve_page(
    request: Request,
    background_tasks: BackgroundTasks,
    ua: str = Query(..., description="User-Agent字符串"),
    path: str = Query(..., description="请求路径"),
    domain: str = Query(..., description="站点域名"),
    seo: SEOCore = Depends(get_seo),
    cache: HTMLCacheManager = Depends(get_cache),
    config: Config = Depends(get_conf)
):
    """
    页面服务入口

    根据传入的UA、路径和域名生成SEO页面。

    Args:
        request: FastAPI Request对象
        background_tasks: 后台任务
        ua: User-Agent字符串
        path: 请求路径
        domain: 站点域名
    """
    import time as time_module
    t0 = time_module.perf_counter()
    start_time = time.time()
    client_ip = get_client_ip(request)
    cache_hit = False

    # 蜘蛛检测
    detection = await detect_spider_async(ua)
    t1 = time_module.perf_counter()

    # 非蜘蛛处理
    if not detection.is_spider:
        if config.spider_detector.return_404_for_non_spider:
            raise HTTPException(status_code=404, detail="Not Found")
        return HTMLResponse(content="<html><body>Hello</body></html>")

    # 尝试从缓存获取
    cached_html = await cache.get(domain, path)
    t2 = time_module.perf_counter()
    if cached_html:
        elapsed = (time.time() - start_time) * 1000
        cache_hit = True
        logger.debug(f"Cache hit: {domain}/{path} ({elapsed:.2f}ms)")
        logger.info(
            f"[PERF] spider={t1-t0:.3f}s cache={t2-t1:.3f}s "
            f"total={t2-t0:.3f}s path={path} (cache_hit)"
        )

        background_tasks.add_task(
            log_spider_visit,
            spider_type=detection.spider_type,
            ip=client_ip,
            ua=ua,
            domain=domain,
            path=path,
            dns_ok=getattr(detection, 'dns_verified', False),
            resp_time=int(elapsed),
            cache_hit=True,
            status=200
        )

        return HTMLResponse(content=cached_html)

    # 生成页面
    try:
        site_config = await get_site_config_cached(domain)
        t3 = time_module.perf_counter()

        if site_config is None:
            logger.warning(f"Domain not registered: {domain}")
            raise HTTPException(status_code=403, detail="Domain not registered")

        template_name = site_config.get('template', 'download_site')
        site_group_id = site_config.get('site_group_id', 1)
        article_group_id = site_config.get('article_group_id') or 1

        # 并行获取：模板、标题、正文
        template_task = fetch_one(
            "SELECT name, content FROM templates WHERE name = %s AND site_group_id = %s AND status = 1",
            (template_name, site_group_id)
        )
        titles_task = get_random_titles(4, group_id=article_group_id)
        content_task = get_random_content(group_id=article_group_id)

        template_data, random_titles, random_content = await asyncio.gather(
            template_task, titles_task, content_task
        )
        t4 = time_module.perf_counter()

        # 如果站群内找不到模板，回退到默认站群
        if not template_data:
            template_data = await fetch_one(
                "SELECT name, content FROM templates WHERE name = %s AND site_group_id = 1 AND status = 1",
                (template_name,)
            )

        if not template_data or not template_data.get('content'):
            logger.error(f"Template not found or empty: {template_name} (site_group_id={site_group_id})")
            raise HTTPException(status_code=500, detail=f"Template '{template_name}' not found")

        article_content = _build_article_content(random_titles, random_content)

        # 预加载内容
        content_pool = get_content_pool_manager(group_id=article_group_id)
        if not content_pool:
            content_pool = await get_or_create_content_pool_manager(group_id=article_group_id)
        if content_pool:
            try:
                preloaded_content = await content_pool.get_content()
                if preloaded_content:
                    seo.set_preloaded_content(preloaded_content)
            except Exception as e:
                logger.warning(f"Failed to preload content from pool: {e}")

        # 模板统计日志
        tpl_content = template_data['content']
        tpl_size = len(tpl_content)
        kw_calls = len(re.findall(r'\{\{[^}]*(?:random_keyword|random_hotspot|keyword_with_emoji)\s*\(\s*\)', tpl_content))
        img_calls = len(re.findall(r'\{\{[^}]*random_image\s*\(\s*\)', tpl_content))
        encode_calls = len(re.findall(r'\{\{[^}]*(?:encode|encode_text)\s*\(', tpl_content))
        content_calls = len(re.findall(r'\{\{[^}]*(?:content|content_with_pinyin)\s*\(\s*\)', tpl_content))
        cls_calls = len(re.findall(r'\{\{[^}]*cls\s*\(\s*\)', tpl_content))
        for_loops = len(re.findall(r'\{%\s*for\s+', tpl_content))
        logger.info(
            f"[PERF-TPL] size={tpl_size} kw={kw_calls} img={img_calls} "
            f"enc={encode_calls} content={content_calls} cls={cls_calls} "
            f"for_loops={for_loops} tpl={template_name}"
        )

        # 渲染模板
        html = seo.render_template_content(
            template_content=tpl_content,
            template_name=template_name,
            site_config=site_config,
            article_content=article_content
        )
        t5 = time_module.perf_counter()

        # 缓存结果
        background_tasks.add_task(cache.set, domain, path, html)

        elapsed = (time.time() - start_time) * 1000
        logger.info(
            f"Page generated: {domain}/{path} "
            f"spider={detection.spider_type} ({elapsed:.2f}ms)"
        )
        logger.info(
            f"[PERF] spider={t1-t0:.3f}s cache={t2-t1:.3f}s "
            f"site={t3-t2:.3f}s fetch={t4-t3:.3f}s render={t5-t4:.3f}s "
            f"total={t5-t0:.3f}s path={path}"
        )

        background_tasks.add_task(
            log_spider_visit,
            spider_type=detection.spider_type,
            ip=client_ip,
            ua=ua,
            domain=domain,
            path=path,
            dns_ok=getattr(detection, 'dns_verified', False),
            resp_time=int(elapsed),
            cache_hit=False,
            status=200
        )

        return HTMLResponse(content=html)

    except ValueError as e:
        error_msg = str(e)
        logger.warning(f"Data not ready: {error_msg}")
        elapsed = (time.time() - start_time) * 1000

        background_tasks.add_task(
            log_spider_visit,
            spider_type=detection.spider_type,
            ip=client_ip,
            ua=ua,
            domain=domain,
            path=path,
            dns_ok=getattr(detection, 'dns_verified', False),
            resp_time=int(elapsed),
            cache_hit=False,
            status=503
        )

        if "关键词" in error_msg:
            raise HTTPException(
                status_code=503,
                detail="Service temporarily unavailable: No keyword data loaded. Please add keywords first."
            )
        raise HTTPException(status_code=503, detail=error_msg)

    except Exception as e:
        logger.error(f"Failed to generate page: {e}")
        elapsed = (time.time() - start_time) * 1000

        background_tasks.add_task(
            log_spider_visit,
            spider_type=detection.spider_type,
            ip=client_ip,
            ua=ua,
            domain=domain,
            path=path,
            dns_ok=getattr(detection, 'dns_verified', False),
            resp_time=int(elapsed),
            cache_hit=False,
            status=500
        )

        raise HTTPException(status_code=500, detail="Internal Server Error")
