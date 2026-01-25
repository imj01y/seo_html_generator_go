"""
FastAPI路由模块

定义所有API端点，包括页面服务和管理后台API。

路由分组:
- /: 页面服务（蜘蛛访问）
- /api/auth/*: 认证相关
- /api/dashboard/*: 仪表盘数据
- /api/sites/*: 站点管理
- /api/keywords/*: 关键词管理
- /api/images/*: 图片分组管理
- /api/groups/*: 分组选项
- /api/cache/*: 缓存管理
- /api/spiders/*: 蜘蛛检测
- /api/settings/*: 系统配置
"""
import time
import asyncio
import threading
import re
from typing import Optional, List
from datetime import datetime

from cachetools import TTLCache

from fastapi import APIRouter, HTTPException, Depends, Header, Query, Request, BackgroundTasks, UploadFile, File, Form
from fastapi.responses import HTMLResponse
from pydantic import BaseModel
from loguru import logger

from core.seo_core import get_seo_core, SEOCore
from core.spider_detector import (get_spider_detector, detect_spider_async)
from core.redis_client import get_redis_client as get_redis
from core.html_cache_manager import get_cache_manager, HTMLCacheManager
from core.keyword_group_manager import get_keyword_group, AsyncKeywordGroupManager
from core.image_group_manager import get_image_group, AsyncImageGroupManager
from core.title_manager import get_title_manager, get_random_titles
from core.content_manager import get_content_manager, get_random_content
from core.content_pool_manager import get_content_pool_manager, get_or_create_content_pool_manager
from core.auth import (
    authenticate_admin, create_access_token, verify_token as verify_jwt_token,
    update_admin_password, get_admin_by_username
)
from config import get_config, Config
from database.db import (
    fetch_one, fetch_all, fetch_value, execute_query, insert, get_db_pool
)

# 创建路由器（类似Flask的Blueprint）
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


# ============================================
# Pydantic模型
# ============================================

class LoginRequest(BaseModel):
    username: str
    password: str


class LoginResponse(BaseModel):
    success: bool
    token: Optional[str] = None
    message: Optional[str] = None


class SiteCreate(BaseModel):
    site_group_id: int = 1  # 所属站群ID
    domain: str
    name: str
    template: str = "download_site"
    keyword_group_id: Optional[int] = None  # 绑定的关键词分组ID
    image_group_id: Optional[int] = None    # 绑定的图片分组ID
    article_group_id: Optional[int] = None  # 绑定的文章分组ID
    icp_number: Optional[str] = None
    baidu_token: Optional[str] = None
    analytics: Optional[str] = None


class SiteUpdate(BaseModel):
    site_group_id: Optional[int] = None  # 所属站点分组ID
    name: Optional[str] = None
    template: Optional[str] = None
    status: Optional[int] = None  # 1=启用, 0=禁用
    keyword_group_id: Optional[int] = None  # 绑定的关键词分组ID
    image_group_id: Optional[int] = None    # 绑定的图片分组ID
    article_group_id: Optional[int] = None  # 绑定的文章分组ID
    icp_number: Optional[str] = None
    baidu_token: Optional[str] = None
    analytics: Optional[str] = None


class KeywordsImport(BaseModel):
    keywords: List[str]


class CacheWarmupRequest(BaseModel):
    domain: str
    count: int = 100


class GroupCreate(BaseModel):
    """创建分组请求"""
    site_group_id: int = 1  # 所属站群ID
    name: str
    description: Optional[str] = None
    is_default: bool = False


class GroupUpdate(BaseModel):
    """更新分组请求"""
    name: Optional[str] = None
    description: Optional[str] = None
    is_default: Optional[int] = None


class SiteGroupCreate(BaseModel):
    """创建站群请求"""
    name: str
    description: Optional[str] = None


class SiteGroupUpdate(BaseModel):
    """更新站群请求"""
    name: Optional[str] = None
    description: Optional[str] = None
    status: Optional[int] = None


class ArticleCreate(BaseModel):
    """添加单篇文章"""
    group_id: int = 1
    title: str
    content: str


class ArticleBatchCreate(BaseModel):
    """批量添加文章"""
    articles: List[ArticleCreate]  # 最多1000条


class ArticleUpdate(BaseModel):
    """更新文章"""
    group_id: Optional[int] = None
    title: Optional[str] = None
    content: Optional[str] = None
    status: Optional[int] = None


class ImageUrlCreate(BaseModel):
    """添加单个图片URL"""
    group_id: int = 1
    url: str


class ImageUrlBatchCreate(BaseModel):
    """批量添加图片URL"""
    group_id: int = 1
    urls: List[str]  # 最多100000条


class KeywordCreate(BaseModel):
    """添加单个关键词"""
    group_id: int = 1
    keyword: str


class KeywordBatchCreate(BaseModel):
    """批量添加关键词"""
    group_id: int = 1
    keywords: List[str]  # 最多100000条


class PasswordChangeRequest(BaseModel):
    """修改密码请求"""
    old_password: str
    new_password: str


class TemplateCreate(BaseModel):
    """创建模板"""
    site_group_id: int = 1  # 所属站群ID
    name: str  # 模板标识名（唯一）
    display_name: str  # 显示名称
    description: Optional[str] = None
    content: str  # HTML模板内容


class TemplateUpdate(BaseModel):
    """更新模板"""
    site_group_id: Optional[int] = None  # 所属站群ID
    display_name: Optional[str] = None
    description: Optional[str] = None
    content: Optional[str] = None
    status: Optional[int] = None  # 1=启用, 0=禁用


class BatchIds(BaseModel):
    """批量操作ID列表"""
    ids: List[int]


class BatchStatusUpdate(BaseModel):
    """批量状态更新"""
    ids: List[int]
    status: int  # 1=启用, 0=禁用


class BatchMoveGroup(BaseModel):
    """批量移动分组"""
    ids: List[int]
    group_id: int


class DeleteAllRequest(BaseModel):
    """删除全部请求"""
    group_id: Optional[int] = None  # 为空表示删除所有分组
    confirm: bool = False  # 必须为True才执行


# ============================================
# 依赖注入
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


# 站点配置内存缓存（最多 500 个站点，TTL 5 分钟）
_site_config_cache: TTLCache = TTLCache(maxsize=500, ttl=300)
_site_config_lock = threading.Lock()


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


# ============================================
# 页面服务路由（供蜘蛛访问）
# ============================================

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
    # 检查数据库是否已初始化
    if get_db_pool() is None:
        logger.debug("Database not initialized, skipping spider log")
        return

    try:
        await insert('spider_logs', {
            'spider_type': spider_type or 'unknown',
            'ip': ip,
            'ua': ua[:500] if ua else '',  # 限制长度
            'domain': domain,
            'path': path[:500] if path else '',  # 限制长度
            'dns_ok': 1 if dns_ok else 0,
            'resp_time': resp_time,
            'cache_hit': 1 if cache_hit else 0,
            'status': status
        })
        logger.debug(f"Spider log recorded: {spider_type} -> {domain}{path}")
    except Exception as e:
        logger.warning(f"Failed to log spider visit: {e}")


def get_client_ip(request: Request) -> str:
    """
    获取客户端真实IP

    优先从 X-Forwarded-For 或 X-Real-IP 头获取（用于反向代理场景）
    """
    # 尝试从代理头获取
    forwarded_for = request.headers.get('X-Forwarded-For')
    if forwarded_for:
        # X-Forwarded-For 可能包含多个IP，取第一个（原始客户端IP）
        return forwarded_for.split(',')[0].strip()

    real_ip = request.headers.get('X-Real-IP')
    if real_ip:
        return real_ip

    # 直接连接的客户端IP
    if request.client:
        return request.client.host

    return '0.0.0.0'


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
    status_code = 200

    # 蜘蛛检测
    detection = await detect_spider_async(ua)
    t1 = time_module.perf_counter()

    # 非蜘蛛处理
    if not detection.is_spider:
        if config.spider_detector.return_404_for_non_spider:
            raise HTTPException(status_code=404, detail="Not Found")
        # 可选：返回简单页面或重定向
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

        # 后台记录蜘蛛日志
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
        # 从缓存或数据库获取站点配置
        site_config = await get_site_config_cached(domain)
        t3 = time_module.perf_counter()

        # 检查站点是否存在于数据库中
        if site_config is None:
            logger.warning(f"Domain not registered: {domain}")
            raise HTTPException(status_code=403, detail="Domain not registered")

        # 提取配置参数
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

        # 如果站群内找不到模板，回退到默认站群（兼容旧数据）
        if not template_data:
            template_data = await fetch_one(
                "SELECT name, content FROM templates WHERE name = %s AND site_group_id = 1 AND status = 1",
                (template_name,)
            )

        if not template_data or not template_data.get('content'):
            logger.error(f"Template not found or empty: {template_name} (site_group_id={site_group_id})")
            raise HTTPException(status_code=500, detail=f"Template '{template_name}' not found")

        # 组装 article_content
        article_content = _build_article_content(random_titles, random_content)

        # 预加载内容供模板中的 content_with_pinyin() 使用
        # 优先从 ContentPoolManager 获取（一次性消费模式）- 按分组过滤（支持懒加载）
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
        kw_calls = len(re.findall(r'\{\{\s*random_keyword\s*\(\s*\)\s*\}\}', tpl_content))
        img_calls = len(re.findall(r'\{\{\s*random_image\s*\(\s*\)\s*\}\}', tpl_content))
        encode_calls = len(re.findall(r'\{\{\s*encode\s*\(', tpl_content))
        logger.info(
            f"[PERF-TPL] size={tpl_size} kw_calls={kw_calls} "
            f"img_calls={img_calls} encode_calls={encode_calls} tpl={template_name}"
        )

        # 使用模板内容渲染页面
        html = seo.render_template_content(
            template_content=tpl_content,
            template_name=template_name,
            site_config=site_config,
            article_content=article_content
        )
        t5 = time_module.perf_counter()
        # 缓存结果（后台执行，不阻塞响应）
        background_tasks.add_task(cache.set, domain, path, html)

        elapsed = (time.time() - start_time) * 1000
        logger.info(
            f"Page generated: {domain}/{path} "
            f"spider={detection.spider_type} ({elapsed:.2f}ms)"
        )
        # 详细耗时日志
        logger.info(
            f"[PERF] spider={t1-t0:.3f}s cache={t2-t1:.3f}s "
            f"site={t3-t2:.3f}s fetch={t4-t3:.3f}s render={t5-t4:.3f}s "
            f"total={t5-t0:.3f}s path={path}"
        )

        # 后台记录蜘蛛日志
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
        # 关键词不足等数据问题，返回友好错误信息
        error_msg = str(e)
        logger.warning(f"Data not ready: {error_msg}")
        elapsed = (time.time() - start_time) * 1000

        # 记录错误日志
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

        # 记录错误日志
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


# ============================================
# 认证API
# ============================================

@router.post("/api/auth/login", response_model=LoginResponse)
async def login(request: LoginRequest):
    """
    管理员登录

    从数据库验证用户名和密码，成功后返回 JWT Token。
    """
    # 从数据库验证
    admin = await authenticate_admin(request.username, request.password)

    if not admin:
        return LoginResponse(success=False, message="用户名或密码错误")

    # 生成 JWT Token
    token = create_access_token(data={
        "sub": admin['username'],
        "admin_id": admin['id'],
        "role": admin['role']
    })

    return LoginResponse(
        success=True,
        token=token,
        message="登录成功"
    )


@router.post("/api/auth/logout")
async def logout(token_data: dict = Depends(verify_token)):
    """退出登录"""
    return {"success": True}


@router.get("/api/auth/profile")
async def get_profile(token_data: dict = Depends(verify_token)):
    """获取当前用户信息"""
    username = token_data.get('sub', 'unknown')
    admin = await get_admin_by_username(username)

    if admin:
        return {
            "id": admin['id'],
            "username": admin['username'],
            "role": "admin",  # role字段已从数据库移除，默认为admin
            "last_login": admin['last_login'].isoformat() if admin['last_login'] else None
        }

    return {
        "username": username,
        "role": token_data.get('role', 'admin'),
        "last_login": None
    }


@router.post("/api/auth/change-password")
async def change_password(
    request: PasswordChangeRequest,
    token_data: dict = Depends(verify_token)
):
    """
    修改密码

    需要验证旧密码，然后设置新密码。
    """
    username = token_data.get('sub')
    admin_id = token_data.get('admin_id')

    if not username or not admin_id:
        raise HTTPException(status_code=401, detail="Invalid token data")

    # 验证旧密码
    admin = await authenticate_admin(username, request.old_password)
    if not admin:
        return {"success": False, "message": "旧密码错误"}

    # 验证新密码长度
    if len(request.new_password) < 6:
        return {"success": False, "message": "新密码长度至少6位"}

    # 更新密码
    success = await update_admin_password(admin_id, request.new_password)

    if success:
        return {"success": True, "message": "密码修改成功"}
    else:
        return {"success": False, "message": "密码修改失败"}


# ============================================
# 仪表盘API
# ============================================

@router.get("/api/dashboard/stats")
async def get_dashboard_stats(
        cache: HTMLCacheManager = Depends(get_cache),
        _: bool = Depends(verify_token)
):
    """获取仪表盘统计数据"""
    keyword_group = get_keyword_group()
    image_group = get_image_group()
    cache_stats = cache.get_stats()

    # 从数据库获取站点数量
    sites_count = 0
    today_spider_visits = 0
    today_generations = 0

    try:
        sites_count = await fetch_value("SELECT COUNT(*) FROM sites WHERE status = 1") or 0
    except Exception as e:
        logger.warning(f"Failed to get sites count: {e}")

    # 今日蜘蛛访问统计
    try:
        today_spider_visits = await fetch_value(
            "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE()"
        ) or 0
    except Exception as e:
        logger.warning(f"Failed to get today spider visits: {e}")

    # 今日页面生成数（未命中缓存的请求）
    try:
        today_generations = await fetch_value(
            "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE() AND cache_hit = 0"
        ) or 0
    except Exception as e:
        logger.warning(f"Failed to get today generations: {e}")

    # 文章总数
    articles_count = 0
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


@router.get("/api/dashboard/spider-visits")
async def get_spider_visits(_: bool = Depends(verify_token)):
    """获取蜘蛛访问统计"""
    try:
        # 总访问数
        total = await fetch_value("SELECT COUNT(*) FROM spider_logs") or 0

        # 按蜘蛛类型统计
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

        # 最近7天趋势
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


@router.get("/api/dashboard/cache-stats")
async def get_cache_stats_api(
        cache: HTMLCacheManager = Depends(get_cache),
        _: bool = Depends(verify_token)
):
    """获取缓存统计"""
    return cache.get_stats()


# ============================================
# 站点管理API
# ============================================

@router.get("/api/sites")
async def list_sites(
        page: int = 1,
        page_size: int = 20,
        site_group_id: Optional[int] = Query(default=None, description="按站群ID过滤"),
        _: dict = Depends(verify_token)
):
    """获取站点列表"""
    try:
        # 构建查询条件
        where_clause = "1=1"
        params = []

        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        # 获取总数
        total = await fetch_value(f"SELECT COUNT(*) FROM sites WHERE {where_clause}", tuple(params) if params else None)

        # 分页查询
        offset = (page - 1) * page_size
        params.extend([page_size, offset])
        items = await fetch_all(
            f"""SELECT id, site_group_id, domain, name, template, keyword_group_id, image_group_id,
                      article_group_id, status, icp_number, baidu_token, analytics, created_at, updated_at
               FROM sites
               WHERE {where_clause}
               ORDER BY id DESC
               LIMIT %s OFFSET %s""",
            tuple(params)
        )

        return {
            "items": items or [],
            "total": total or 0,
            "page": page,
            "page_size": page_size
        }
    except Exception as e:
        logger.error(f"Failed to list sites: {e}")
        return {"items": [], "total": 0, "page": page, "page_size": page_size}


@router.post("/api/sites")
async def create_site(
        site: SiteCreate,
        _: dict = Depends(verify_token)
):
    """创建站点"""
    try:
        # 如果未指定分组，使用默认分组
        keyword_group_id = site.keyword_group_id
        image_group_id = site.image_group_id
        article_group_id = site.article_group_id

        if keyword_group_id is None:
            keyword_group_id = await fetch_value(
                "SELECT id FROM keyword_groups WHERE is_default = 1 LIMIT 1"
            )
        if image_group_id is None:
            image_group_id = await fetch_value(
                "SELECT id FROM image_groups WHERE is_default = 1 LIMIT 1"
            )
        if article_group_id is None:
            article_group_id = await fetch_value(
                "SELECT id FROM article_groups WHERE is_default = 1 LIMIT 1"
            )

        site_id = await insert('sites', {
            'site_group_id': site.site_group_id,
            'domain': site.domain,
            'name': site.name,
            'template': site.template,
            'keyword_group_id': keyword_group_id,
            'image_group_id': image_group_id,
            'article_group_id': article_group_id,
            'icp_number': site.icp_number,
            'baidu_token': site.baidu_token,
            'analytics': site.analytics
        })
        return {"success": True, "id": site_id}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "域名已存在"}
        logger.error(f"Failed to create site: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/sites/{site_id}")
async def get_site(site_id: int, _: dict = Depends(verify_token)):
    """获取站点详情"""
    try:
        site = await fetch_one(
            "SELECT * FROM sites WHERE id = %s",
            (site_id,)
        )
        if not site:
            raise HTTPException(status_code=404, detail="站点不存在")
        return site
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to get site: {e}")
        raise HTTPException(status_code=500, detail="获取站点失败")


@router.put("/api/sites/{site_id}")
async def update_site(
        site_id: int,
        site: SiteUpdate,
        _: dict = Depends(verify_token)
):
    """更新站点"""
    try:
        # 检查站点是否存在
        existing = await fetch_one("SELECT id FROM sites WHERE id = %s", (site_id,))
        if not existing:
            return {"success": False, "message": "站点不存在"}

        # 构建更新字段
        update_fields = []
        update_values = []

        if site.site_group_id is not None:
            update_fields.append("site_group_id = %s")
            update_values.append(site.site_group_id)
        if site.name is not None:
            update_fields.append("name = %s")
            update_values.append(site.name)
        if site.template is not None:
            update_fields.append("template = %s")
            update_values.append(site.template)
        if site.status is not None:
            update_fields.append("status = %s")
            update_values.append(site.status)
        if site.icp_number is not None:
            update_fields.append("icp_number = %s")
            update_values.append(site.icp_number)
        if site.baidu_token is not None:
            update_fields.append("baidu_token = %s")
            update_values.append(site.baidu_token)
        if site.keyword_group_id is not None:
            update_fields.append("keyword_group_id = %s")
            update_values.append(site.keyword_group_id)
        if site.image_group_id is not None:
            update_fields.append("image_group_id = %s")
            update_values.append(site.image_group_id)
        if site.article_group_id is not None:
            update_fields.append("article_group_id = %s")
            update_values.append(site.article_group_id)
        if site.analytics is not None:
            update_fields.append("analytics = %s")
            update_values.append(site.analytics)

        if not update_fields:
            return {"success": True, "message": "没有需要更新的字段"}

        update_values.append(site_id)
        sql = f"UPDATE sites SET {', '.join(update_fields)} WHERE id = %s"
        await execute_query(sql, tuple(update_values))

        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to update site: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/sites/{site_id}")
async def delete_site(site_id: int, _: dict = Depends(verify_token)):
    """删除站点"""
    try:
        await execute_query("DELETE FROM sites WHERE id = %s", (site_id,))
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete site: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 站点批量操作API
# ============================================

@router.delete("/api/sites/batch/delete")
async def batch_delete_sites(data: BatchIds, _: dict = Depends(verify_token)):
    """批量删除站点"""
    if not data.ids:
        return {"success": False, "message": "请选择要删除的站点", "deleted": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"DELETE FROM sites WHERE id IN ({placeholders})",
            tuple(data.ids)
        )
        return {"success": True, "deleted": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch delete sites: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.put("/api/sites/batch/status")
async def batch_update_site_status(data: BatchStatusUpdate, _: dict = Depends(verify_token)):
    """批量更新站点状态"""
    if not data.ids:
        return {"success": False, "message": "请选择要更新的站点", "updated": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE sites SET status = %s WHERE id IN ({placeholders})",
            (data.status, *data.ids)
        )
        return {"success": True, "updated": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch update site status: {e}")
        return {"success": False, "message": str(e), "updated": 0}


# ============================================
# 站群管理API
# ============================================

@router.get("/api/site-groups")
async def list_site_groups(_: dict = Depends(verify_token)):
    """获取所有站群列表"""
    try:
        groups = await fetch_all("""
            SELECT sg.*,
                   (SELECT COUNT(*) FROM sites WHERE site_group_id = sg.id) as sites_count,
                   (SELECT COUNT(*) FROM keyword_groups WHERE site_group_id = sg.id AND status = 1) as keyword_groups_count,
                   (SELECT COUNT(*) FROM image_groups WHERE site_group_id = sg.id AND status = 1) as image_groups_count,
                   (SELECT COUNT(*) FROM article_groups WHERE site_group_id = sg.id AND status = 1) as article_groups_count,
                   (SELECT COUNT(*) FROM templates WHERE site_group_id = sg.id AND status = 1) as templates_count
            FROM site_groups sg
            WHERE sg.status = 1
            ORDER BY sg.id
        """)
        return {"items": groups or [], "total": len(groups) if groups else 0}
    except Exception as e:
        logger.error(f"Failed to fetch site groups: {e}")
        return {"items": [], "total": 0, "error": str(e)}


@router.get("/api/site-groups/{group_id}")
async def get_site_group(group_id: int, _: dict = Depends(verify_token)):
    """获取单个站群详情（含统计信息）"""
    try:
        group = await fetch_one(
            "SELECT * FROM site_groups WHERE id = %s AND status = 1",
            (group_id,)
        )
        if not group:
            return {"success": False, "message": "站群不存在"}

        # 获取统计信息
        stats = {
            "sites_count": await fetch_value(
                "SELECT COUNT(*) FROM sites WHERE site_group_id = %s", (group_id,)
            ) or 0,
            "keyword_groups_count": await fetch_value(
                "SELECT COUNT(*) FROM keyword_groups WHERE site_group_id = %s AND status = 1", (group_id,)
            ) or 0,
            "image_groups_count": await fetch_value(
                "SELECT COUNT(*) FROM image_groups WHERE site_group_id = %s AND status = 1", (group_id,)
            ) or 0,
            "article_groups_count": await fetch_value(
                "SELECT COUNT(*) FROM article_groups WHERE site_group_id = %s AND status = 1", (group_id,)
            ) or 0,
            "templates_count": await fetch_value(
                "SELECT COUNT(*) FROM templates WHERE site_group_id = %s AND status = 1", (group_id,)
            ) or 0
        }

        return {**group, "stats": stats}
    except Exception as e:
        logger.error(f"Failed to fetch site group: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/site-groups/{group_id}/options")
async def get_site_group_options(group_id: int, _: dict = Depends(verify_token)):
    """获取站群下的所有资源选项（用于站点配置）"""
    try:
        keyword_groups = await fetch_all("""
            SELECT id, name, is_default
            FROM keyword_groups
            WHERE site_group_id = %s AND status = 1
            ORDER BY is_default DESC, name
        """, (group_id,))

        image_groups = await fetch_all("""
            SELECT id, name, is_default
            FROM image_groups
            WHERE site_group_id = %s AND status = 1
            ORDER BY is_default DESC, name
        """, (group_id,))

        article_groups = await fetch_all("""
            SELECT id, name, is_default
            FROM article_groups
            WHERE site_group_id = %s AND status = 1
            ORDER BY is_default DESC, name
        """, (group_id,))

        templates = await fetch_all("""
            SELECT id, name, display_name
            FROM templates
            WHERE site_group_id = %s AND status = 1
            ORDER BY name
        """, (group_id,))

        return {
            "keyword_groups": keyword_groups or [],
            "image_groups": image_groups or [],
            "article_groups": article_groups or [],
            "templates": templates or []
        }
    except Exception as e:
        logger.error(f"Failed to fetch site group options: {e}")
        return {"keyword_groups": [], "image_groups": [], "article_groups": [], "templates": []}


@router.post("/api/site-groups")
async def create_site_group(data: SiteGroupCreate, _: dict = Depends(verify_token)):
    """创建站群"""
    try:
        group_id = await insert('site_groups', {
            'name': data.name,
            'description': data.description
        })
        return {"success": True, "id": group_id}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "站群名称已存在"}
        logger.error(f"Failed to create site group: {e}")
        return {"success": False, "message": str(e)}


@router.put("/api/site-groups/{group_id}")
async def update_site_group(group_id: int, data: SiteGroupUpdate, _: dict = Depends(verify_token)):
    """更新站群"""
    try:
        update_fields = []
        update_values = []

        if data.name is not None:
            update_fields.append("name = %s")
            update_values.append(data.name)
        if data.description is not None:
            update_fields.append("description = %s")
            update_values.append(data.description)
        if data.status is not None:
            update_fields.append("status = %s")
            update_values.append(data.status)

        if not update_fields:
            return {"success": True, "message": "没有需要更新的字段"}

        update_values.append(group_id)
        sql = f"UPDATE site_groups SET {', '.join(update_fields)} WHERE id = %s"
        await execute_query(sql, tuple(update_values))

        return {"success": True}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "站群名称已存在"}
        logger.error(f"Failed to update site group: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/site-groups/{group_id}")
async def delete_site_group(group_id: int, _: dict = Depends(verify_token)):
    """删除站群（软删除）"""
    try:
        # 检查是否有站点使用此站群
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE site_group_id = %s",
            (group_id,)
        )
        if sites_count and sites_count > 0:
            return {"success": False, "message": f"无法删除：有 {sites_count} 个站点属于此站群"}

        # 检查是否为默认站群（ID=1）
        if group_id == 1:
            return {"success": False, "message": "不能删除默认站群"}

        await execute_query(
            "UPDATE site_groups SET status = 0 WHERE id = %s",
            (group_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete site group: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 分组选项API（用于站点绑定）
# ============================================

@router.get("/api/groups/options")
async def get_group_options(_: dict = Depends(verify_token)):
    """
    获取所有分组选项（用于站点绑定下拉列表）

    返回关键词分组和图片分组的选项列表
    """
    try:
        keyword_groups = await fetch_all("""
            SELECT id, name, is_default
            FROM keyword_groups
            WHERE status = 1
            ORDER BY is_default DESC, name
        """)

        image_groups = await fetch_all("""
            SELECT id, name, is_default
            FROM image_groups
            WHERE status = 1
            ORDER BY is_default DESC, name
        """)

        return {
            "keyword_groups": keyword_groups or [],
            "image_groups": image_groups or []
        }
    except Exception as e:
        logger.error(f"Failed to get group options: {e}")
        return {"keyword_groups": [], "image_groups": []}


# ============================================
# 模板管理API
# ============================================

@router.get("/api/templates")
async def list_templates(
    page: int = 1,
    page_size: int = 20,
    status: Optional[int] = None,
    site_group_id: Optional[int] = Query(default=None, description="按站群ID过滤"),
    _: dict = Depends(verify_token)
):
    """
    获取模板列表（不含content字段，提高性能）

    Args:
        page: 页码
        page_size: 每页数量
        status: 状态筛选 (1=启用, 0=禁用)
        site_group_id: 站群ID过滤
    """
    try:
        # 构建查询条件
        where_clause = "1=1"
        params = []

        if status is not None:
            where_clause += " AND status = %s"
            params.append(status)

        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        # 获取总数
        total = await fetch_value(
            f"SELECT COUNT(*) FROM templates WHERE {where_clause}",
            tuple(params) if params else None
        ) or 0

        # 分页查询（不含content字段）
        offset = (page - 1) * page_size
        params.extend([page_size, offset])

        items = await fetch_all(
            f"""SELECT id, site_group_id, name, display_name, description, status, version,
                       created_at, updated_at,
                       (SELECT COUNT(*) FROM sites WHERE sites.template = templates.name) as sites_count
                FROM templates
                WHERE {where_clause}
                ORDER BY id DESC
                LIMIT %s OFFSET %s""",
            tuple(params)
        )

        return {
            "items": items or [],
            "total": total,
            "page": page,
            "page_size": page_size
        }
    except Exception as e:
        logger.error(f"Failed to list templates: {e}")
        return {"items": [], "total": 0, "page": page, "page_size": page_size}


@router.get("/api/templates/options")
async def get_template_options(
    site_group_id: Optional[int] = Query(None, description="按站群ID过滤"),
    _: dict = Depends(verify_token)
):
    """
    获取模板下拉选项（用于站点绑定）

    只返回启用状态的模板，可按站群过滤
    """
    try:
        if site_group_id:
            # 优先返回当前站群的模板，再返回默认站群的模板（兼容旧数据）
            items = await fetch_all(
                """SELECT id, name, display_name
                   FROM templates
                   WHERE status = 1 AND (site_group_id = %s OR site_group_id = 1)
                   ORDER BY site_group_id DESC, name""",
                (site_group_id,)
            )
        else:
            items = await fetch_all(
                """SELECT id, name, display_name
                   FROM templates
                   WHERE status = 1
                   ORDER BY name"""
            )
        return {"options": items or []}
    except Exception as e:
        logger.error(f"Failed to get template options: {e}")
        return {"options": []}


@router.get("/api/templates/{template_id}")
async def get_template(template_id: int, _: dict = Depends(verify_token)):
    """
    获取模板详情（含content字段）

    用于编辑页面加载完整模板内容
    """
    try:
        template = await fetch_one(
            """SELECT id, site_group_id, name, display_name, description, content, status,
                      version, created_at, updated_at
               FROM templates WHERE id = %s""",
            (template_id,)
        )
        if not template:
            raise HTTPException(status_code=404, detail="模板不存在")
        return template
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to get template: {e}")
        raise HTTPException(status_code=500, detail="获取模板失败")


@router.get("/api/templates/{template_id}/sites")
async def get_template_sites(template_id: int, _: dict = Depends(verify_token)):
    """
    获取使用此模板的站点列表
    """
    try:
        # 先获取模板名称
        template = await fetch_one(
            "SELECT name FROM templates WHERE id = %s",
            (template_id,)
        )
        if not template:
            raise HTTPException(status_code=404, detail="模板不存在")

        # 查询使用此模板的站点
        sites = await fetch_all(
            """SELECT id, domain, name, status, created_at
               FROM sites
               WHERE template = %s
               ORDER BY id DESC""",
            (template['name'],)
        )
        return {"sites": sites or [], "template_name": template['name']}
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to get template sites: {e}")
        return {"sites": [], "error": str(e)}


@router.post("/api/templates")
async def create_template(
    data: TemplateCreate,
    _: dict = Depends(verify_token)
):
    """
    创建新模板
    """
    try:
        template_id = await insert('templates', {
            'site_group_id': data.site_group_id,
            'name': data.name,
            'display_name': data.display_name,
            'description': data.description,
            'content': data.content,
            'status': 1,
            'version': 1
        })
        return {"success": True, "id": template_id}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "该站群内模板标识名已存在"}
        logger.error(f"Failed to create template: {e}")
        return {"success": False, "message": str(e)}


@router.put("/api/templates/{template_id}")
async def update_template(
    template_id: int,
    data: TemplateUpdate,
    _: dict = Depends(verify_token)
):
    """
    更新模板

    每次保存content时自动递增version
    """
    try:
        # 检查模板是否存在
        existing = await fetch_one(
            "SELECT id, version FROM templates WHERE id = %s",
            (template_id,)
        )
        if not existing:
            return {"success": False, "message": "模板不存在"}

        # 构建更新字段
        update_fields = []
        update_values = []

        if data.site_group_id is not None:
            update_fields.append("site_group_id = %s")
            update_values.append(data.site_group_id)
        if data.display_name is not None:
            update_fields.append("display_name = %s")
            update_values.append(data.display_name)
        if data.description is not None:
            update_fields.append("description = %s")
            update_values.append(data.description)
        if data.content is not None:
            update_fields.append("content = %s")
            update_values.append(data.content)
            # 更新内容时递增版本号
            update_fields.append("version = version + 1")
        if data.status is not None:
            update_fields.append("status = %s")
            update_values.append(data.status)

        if not update_fields:
            return {"success": True, "message": "没有需要更新的字段"}

        update_values.append(template_id)
        sql = f"UPDATE templates SET {', '.join(update_fields)} WHERE id = %s"
        await execute_query(sql, tuple(update_values))

        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to update template: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/templates/{template_id}")
async def delete_template(template_id: int, _: dict = Depends(verify_token)):
    """
    删除模板

    如果有站点正在使用此模板，则拒绝删除
    """
    try:
        # 获取模板名称
        template = await fetch_one(
            "SELECT name FROM templates WHERE id = %s",
            (template_id,)
        )
        if not template:
            return {"success": False, "message": "模板不存在"}

        # 检查是否有站点使用此模板
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE template = %s AND status = 1",
            (template['name'],)
        )

        if sites_count and sites_count > 0:
            return {
                "success": False,
                "message": f"无法删除：有 {sites_count} 个站点正在使用此模板"
            }

        # 删除模板
        await execute_query("DELETE FROM templates WHERE id = %s", (template_id,))
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete template: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 关键词分组管理API（数据库查询）
# ============================================

@router.get("/api/keywords/groups")
async def list_keyword_groups(
    site_group_id: Optional[int] = Query(default=None, description="按站群ID过滤"),
    _: bool = Depends(verify_token)
):
    """获取所有关键词分组列表"""
    try:
        where_clause = "status = 1"
        params = []
        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        sql = f"""
            SELECT id, site_group_id, name, description, is_default,
                   status, created_at
            FROM keyword_groups
            WHERE {where_clause}
            ORDER BY is_default DESC, name
        """
        groups = await fetch_all(sql, tuple(params) if params else None)
        return {"groups": groups or []}
    except Exception as e:
        logger.error(f"Failed to fetch keyword groups from database: {e}")
        return {"groups": [], "error": str(e)}


@router.post("/api/keywords/groups")
async def create_keyword_group(data: GroupCreate, _: bool = Depends(verify_token)):
    """创建关键词分组"""
    try:
        # 如果设为默认分组，先取消其他默认分组
        if data.is_default:
            await execute_query("UPDATE keyword_groups SET is_default = 0 WHERE is_default = 1")

        group_id = await insert('keyword_groups', {
            'site_group_id': data.site_group_id,
            'name': data.name,
            'description': data.description,
            'is_default': 1 if data.is_default else 0
        })
        return {"success": True, "id": group_id}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "分组名称已存在"}
        logger.error(f"Failed to create keyword group: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/keywords/groups/{group_id}")
async def delete_keyword_group(group_id: int, _: bool = Depends(verify_token)):
    """删除关键词分组（软删除，标记为inactive）"""
    try:
        # 检查是否有站点使用此分组
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE keyword_group_id = %s",
            (group_id,)
        )
        if sites_count and sites_count > 0:
            return {"success": False, "message": f"无法删除：有 {sites_count} 个站点正在使用此分组"}

        # 检查是否为默认分组
        group = await fetch_one("SELECT is_default FROM keyword_groups WHERE id = %s", (group_id,))
        if group and group.get('is_default'):
            return {"success": False, "message": "不能删除默认分组"}

        await execute_query(
            "UPDATE keyword_groups SET status = 0 WHERE id = %s",
            (group_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete keyword group: {e}")
        return {"success": False, "message": str(e)}


@router.put("/api/keywords/groups/{group_id}")
async def update_keyword_group(group_id: int, data: GroupUpdate, _: bool = Depends(verify_token)):
    """更新关键词分组"""
    try:
        # 检查分组是否存在
        group = await fetch_one(
            "SELECT id, is_default FROM keyword_groups WHERE id = %s AND status = 1",
            (group_id,)
        )
        if not group:
            return {"success": False, "message": "分组不存在"}

        # 构建更新字段
        updates = []
        params = []

        if data.name is not None:
            updates.append("name = %s")
            params.append(data.name)

        if data.description is not None:
            updates.append("description = %s")
            params.append(data.description)

        if data.is_default is not None:
            # 如果设为默认，先取消其他默认分组
            if data.is_default == 1:
                await execute_query("UPDATE keyword_groups SET is_default = 0 WHERE is_default = 1")
            updates.append("is_default = %s")
            params.append(data.is_default)

        if not updates:
            return {"success": True, "message": "无需更新"}

        params.append(group_id)
        sql = f"UPDATE keyword_groups SET {', '.join(updates)} WHERE id = %s"
        await execute_query(sql, tuple(params))

        return {"success": True}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "分组名称已存在"}
        logger.error(f"Failed to update keyword group: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/keywords/list")
async def list_keywords(
    group_id: int = Query(default=1),
    page: int = Query(default=1, ge=1),
    page_size: int = Query(default=20, ge=1, le=100),
    search: Optional[str] = None,
    _: bool = Depends(verify_token)
):
    """分页获取关键词列表"""
    try:
        offset = (page - 1) * page_size

        # 构建查询条件（status: 1=有效，0=无效）
        where_clause = "group_id = %s AND status = 1"
        params = [group_id]

        if search:
            where_clause += " AND keyword LIKE %s"
            params.append(f"%{search}%")

        # 获取总数
        count_sql = f"SELECT COUNT(*) FROM keywords WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

        # 获取列表
        params.extend([page_size, offset])
        list_sql = f"""
            SELECT id, group_id, keyword, status, created_at
            FROM keywords
            WHERE {where_clause}
            ORDER BY id DESC
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
        logger.error(f"Failed to list keywords: {e}")
        return {"items": [], "total": 0, "page": page, "page_size": page_size}


@router.put("/api/keywords/{keyword_id}")
async def update_keyword(keyword_id: int, data: dict, _: bool = Depends(verify_token)):
    """更新关键词"""
    try:
        # 检查关键词是否存在
        existing = await fetch_one("SELECT id FROM keywords WHERE id = %s", (keyword_id,))
        if not existing:
            return {"success": False, "message": "关键词不存在"}

        # 构建更新字段
        updates = []
        params = []

        if 'keyword' in data and data['keyword']:
            updates.append("keyword = %s")
            params.append(data['keyword'])
        if 'group_id' in data and data['group_id']:
            updates.append("group_id = %s")
            params.append(data['group_id'])
        if 'status' in data and data['status'] is not None:
            updates.append("status = %s")
            params.append(data['status'])

        if not updates:
            return {"success": False, "message": "没有要更新的字段"}

        params.append(keyword_id)
        await execute_query(
            f"UPDATE keywords SET {', '.join(updates)} WHERE id = %s",
            tuple(params)
        )
        return {"success": True}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "关键词已存在"}
        logger.error(f"Failed to update keyword: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/keywords/{keyword_id}")
async def delete_keyword(keyword_id: int, _: bool = Depends(verify_token)):
    """删除关键词（软删除，标记status=0）"""
    try:
        await execute_query(
            "UPDATE keywords SET status = 0 WHERE id = %s",
            (keyword_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete keyword: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/keywords/batch")
async def batch_delete_keywords(data: BatchIds, _: bool = Depends(verify_token)):
    """批量删除关键词（软删除）"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "deleted": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        result = await execute_query(
            f"UPDATE keywords SET status = 0 WHERE id IN ({placeholders})",
            tuple(data.ids)
        )
        return {"success": True, "deleted": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch delete keywords: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.delete("/api/keywords/delete-all")
async def delete_all_keywords(data: DeleteAllRequest, _: bool = Depends(verify_token)):
    """删除全部关键词（软删除）"""
    if not data.confirm:
        return {"success": False, "message": "请确认删除操作", "deleted": 0}

    try:
        if data.group_id:
            result = await execute_query(
                "UPDATE keywords SET status = 0 WHERE group_id = %s AND status = 1",
                (data.group_id,)
            )
        else:
            result = await execute_query(
                "UPDATE keywords SET status = 0 WHERE status = 1"
            )
        deleted_count = result.rowcount if hasattr(result, 'rowcount') else 0
        return {"success": True, "deleted": deleted_count}
    except Exception as e:
        logger.error(f"Failed to delete all keywords: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.put("/api/keywords/batch/status")
async def batch_update_keyword_status(data: BatchStatusUpdate, _: bool = Depends(verify_token)):
    """批量更新关键词状态"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "updated": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE keywords SET status = %s WHERE id IN ({placeholders})",
            (data.status, *data.ids)
        )
        return {"success": True, "updated": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch update keyword status: {e}")
        return {"success": False, "message": str(e), "updated": 0}


@router.put("/api/keywords/batch/move")
async def batch_move_keywords(data: BatchMoveGroup, _: bool = Depends(verify_token)):
    """批量移动关键词到其他分组"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "moved": 0}

    try:
        # 验证目标分组存在
        group = await fetch_one(
            "SELECT id FROM keyword_groups WHERE id = %s AND status = 1",
            (data.group_id,)
        )
        if not group:
            return {"success": False, "message": "目标分组不存在", "moved": 0}

        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE keywords SET group_id = %s WHERE id IN ({placeholders})",
            (data.group_id, *data.ids)
        )
        return {"success": True, "moved": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch move keywords: {e}")
        return {"success": False, "message": str(e), "moved": 0}


# ============================================
# 图片分组管理API（MySQL+Redis模式）
# ============================================

@router.get("/api/images/groups")
async def list_image_groups(
    site_group_id: Optional[int] = Query(default=None, description="按站群ID过滤"),
    _: bool = Depends(verify_token)
):
    """获取图片分组列表"""
    try:
        where_clause = "status = 1"
        params = []
        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        sql = f"""
            SELECT id, site_group_id, name, description, is_default, status, created_at
            FROM image_groups
            WHERE {where_clause}
            ORDER BY is_default DESC, name
        """
        groups = await fetch_all(sql, tuple(params) if params else None)
        return {"groups": groups or []}
    except Exception as e:
        logger.error(f"Failed to fetch image groups: {e}")
        return {"groups": [], "error": str(e)}


@router.post("/api/images/groups")
async def create_image_group(data: GroupCreate, _: bool = Depends(verify_token)):
    """创建图片分组"""
    try:
        # 如果设为默认分组，先取消其他默认分组
        if data.is_default:
            await execute_query("UPDATE image_groups SET is_default = 0 WHERE is_default = 1")

        group_id = await insert('image_groups', {
            'site_group_id': data.site_group_id,
            'name': data.name,
            'description': data.description,
            'is_default': 1 if data.is_default else 0
        })
        return {"success": True, "id": group_id}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "分组名称已存在"}
        logger.error(f"Failed to create image group: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/images/groups/{group_id}")
async def delete_image_group(group_id: int, _: bool = Depends(verify_token)):
    """删除图片分组（软删除，标记为inactive）"""
    try:
        # 检查是否有站点使用此分组
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE image_group_id = %s",
            (group_id,)
        )
        if sites_count and sites_count > 0:
            return {"success": False, "message": f"无法删除：有 {sites_count} 个站点正在使用此分组"}

        # 检查是否为默认分组
        group = await fetch_one("SELECT is_default FROM image_groups WHERE id = %s", (group_id,))
        if group and group.get('is_default'):
            return {"success": False, "message": "不能删除默认分组"}

        await execute_query(
            "UPDATE image_groups SET status = 0 WHERE id = %s",
            (group_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete image group: {e}")
        return {"success": False, "message": str(e)}


@router.put("/api/images/groups/{group_id}")
async def update_image_group(group_id: int, data: GroupUpdate, _: bool = Depends(verify_token)):
    """更新图片分组"""
    try:
        # 检查分组是否存在
        group = await fetch_one(
            "SELECT id, is_default FROM image_groups WHERE id = %s AND status = 1",
            (group_id,)
        )
        if not group:
            return {"success": False, "message": "分组不存在"}

        # 构建更新字段
        updates = []
        params = []

        if data.name is not None:
            updates.append("name = %s")
            params.append(data.name)

        if data.description is not None:
            updates.append("description = %s")
            params.append(data.description)

        if data.is_default is not None:
            # 如果设为默认，先取消其他默认分组
            if data.is_default == 1:
                await execute_query("UPDATE image_groups SET is_default = 0 WHERE is_default = 1")
            updates.append("is_default = %s")
            params.append(data.is_default)

        if not updates:
            return {"success": True, "message": "无需更新"}

        params.append(group_id)
        sql = f"UPDATE image_groups SET {', '.join(updates)} WHERE id = %s"
        await execute_query(sql, tuple(params))

        return {"success": True}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "分组名称已存在"}
        logger.error(f"Failed to update image group: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/images/urls/list")
async def list_image_urls(
    group_id: int = Query(default=1),
    page: int = Query(default=1, ge=1),
    page_size: int = Query(default=20, ge=1, le=100),
    search: Optional[str] = None,
    _: bool = Depends(verify_token)
):
    """分页获取图片URL列表"""
    try:
        offset = (page - 1) * page_size

        # 构建查询条件（status: 1=有效，0=无效）
        where_clause = "group_id = %s AND status = 1"
        params = [group_id]

        if search:
            where_clause += " AND url LIKE %s"
            params.append(f"%{search}%")

        # 获取总数
        count_sql = f"SELECT COUNT(*) FROM images WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

        # 获取列表
        params.extend([page_size, offset])
        list_sql = f"""
            SELECT id, group_id, url, status, created_at
            FROM images
            WHERE {where_clause}
            ORDER BY id DESC
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
        logger.error(f"Failed to list image urls: {e}")
        return {"items": [], "total": 0, "page": page, "page_size": page_size}


@router.put("/api/images/urls/{image_id}")
async def update_image_url(image_id: int, data: dict, _: bool = Depends(verify_token)):
    """更新图片URL"""
    try:
        # 检查图片是否存在
        existing = await fetch_one("SELECT id FROM images WHERE id = %s", (image_id,))
        if not existing:
            return {"success": False, "message": "图片不存在"}

        # 构建更新字段
        updates = []
        params = []

        if 'url' in data and data['url']:
            updates.append("url = %s")
            params.append(data['url'])
        if 'group_id' in data and data['group_id']:
            updates.append("group_id = %s")
            params.append(data['group_id'])
        if 'status' in data and data['status'] is not None:
            updates.append("status = %s")
            params.append(data['status'])

        if not updates:
            return {"success": False, "message": "没有要更新的字段"}

        params.append(image_id)
        await execute_query(
            f"UPDATE images SET {', '.join(updates)} WHERE id = %s",
            tuple(params)
        )
        return {"success": True}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "图片URL已存在"}
        logger.error(f"Failed to update image url: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/images/urls/{image_id}")
async def delete_image_url(image_id: int, _: bool = Depends(verify_token)):
    """删除图片URL（软删除，标记status=0）"""
    try:
        await execute_query(
            "UPDATE images SET status = 0 WHERE id = %s",
            (image_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete image url: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/images/batch")
async def batch_delete_images(data: BatchIds, _: bool = Depends(verify_token)):
    """批量删除图片URL（软删除）"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "deleted": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE images SET status = 0 WHERE id IN ({placeholders})",
            tuple(data.ids)
        )
        return {"success": True, "deleted": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch delete images: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.delete("/api/images/delete-all")
async def delete_all_images(data: DeleteAllRequest, _: bool = Depends(verify_token)):
    """删除全部图片URL（软删除）"""
    if not data.confirm:
        return {"success": False, "message": "请确认删除操作", "deleted": 0}

    try:
        if data.group_id:
            result = await execute_query(
                "UPDATE images SET status = 0 WHERE group_id = %s AND status = 1",
                (data.group_id,)
            )
        else:
            result = await execute_query(
                "UPDATE images SET status = 0 WHERE status = 1"
            )
        deleted_count = result.rowcount if hasattr(result, 'rowcount') else 0
        return {"success": True, "deleted": deleted_count}
    except Exception as e:
        logger.error(f"Failed to delete all images: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.put("/api/images/batch/status")
async def batch_update_image_status(data: BatchStatusUpdate, _: bool = Depends(verify_token)):
    """批量更新图片URL状态"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "updated": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE images SET status = %s WHERE id IN ({placeholders})",
            (data.status, *data.ids)
        )
        return {"success": True, "updated": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch update image status: {e}")
        return {"success": False, "message": str(e), "updated": 0}


@router.put("/api/images/batch/move")
async def batch_move_images(data: BatchMoveGroup, _: bool = Depends(verify_token)):
    """批量移动图片URL到其他分组"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "moved": 0}

    try:
        # 验证目标分组存在
        group = await fetch_one(
            "SELECT id FROM image_groups WHERE id = %s AND status = 1",
            (data.group_id,)
        )
        if not group:
            return {"success": False, "message": "目标分组不存在", "moved": 0}

        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE images SET group_id = %s WHERE id IN ({placeholders})",
            (data.group_id, *data.ids)
        )
        return {"success": True, "moved": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch move images: {e}")
        return {"success": False, "message": str(e), "moved": 0}


# ============================================
# 文章管理API（新增）
# ============================================

@router.get("/api/articles/groups")
async def list_article_groups(
    site_group_id: Optional[int] = Query(default=None, description="按站群ID过滤"),
    _: bool = Depends(verify_token)
):
    """获取文章分组列表"""
    try:
        where_clause = "status = 1"
        params = []
        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        sql = f"""
            SELECT id, site_group_id, name, description, is_default, status, created_at
            FROM article_groups
            WHERE {where_clause}
            ORDER BY is_default DESC, name
        """
        groups = await fetch_all(sql, tuple(params) if params else None)
        return {"groups": groups or []}
    except Exception as e:
        logger.error(f"Failed to fetch article groups: {e}")
        # 如果表不存在，返回默认分组
        return {"groups": [{"id": 1, "site_group_id": 1, "name": "默认文章分组", "description": "系统默认文章分组", "is_default": 1}]}


@router.post("/api/articles/groups")
async def create_article_group(data: GroupCreate, _: bool = Depends(verify_token)):
    """创建文章分组"""
    try:
        # 如果设为默认分组，先取消其他默认分组
        if data.is_default:
            await execute_query("UPDATE article_groups SET is_default = 0 WHERE is_default = 1")

        group_id = await insert('article_groups', {
            'site_group_id': data.site_group_id,
            'name': data.name,
            'description': data.description,
            'is_default': 1 if data.is_default else 0
        })
        return {"success": True, "id": group_id}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "分组名称已存在"}
        logger.error(f"Failed to create article group: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/articles/groups/{group_id}")
async def delete_article_group(group_id: int, _: bool = Depends(verify_token)):
    """删除文章分组（软删除，标记为inactive）"""
    try:
        # 检查是否为默认分组
        group = await fetch_one("SELECT is_default FROM article_groups WHERE id = %s", (group_id,))
        if group and group.get('is_default'):
            return {"success": False, "message": "不能删除默认分组"}

        await execute_query(
            "UPDATE article_groups SET status = 0 WHERE id = %s",
            (group_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete article group: {e}")
        return {"success": False, "message": str(e)}


@router.put("/api/articles/groups/{group_id}")
async def update_article_group(group_id: int, data: GroupUpdate, _: bool = Depends(verify_token)):
    """更新文章分组"""
    try:
        # 检查分组是否存在
        group = await fetch_one(
            "SELECT id, is_default FROM article_groups WHERE id = %s AND status = 1",
            (group_id,)
        )
        if not group:
            return {"success": False, "message": "分组不存在"}

        # 构建更新字段
        updates = []
        params = []

        if data.name is not None:
            updates.append("name = %s")
            params.append(data.name)

        if data.description is not None:
            updates.append("description = %s")
            params.append(data.description)

        if data.is_default is not None:
            # 如果设为默认，先取消其他默认分组
            if data.is_default == 1:
                await execute_query("UPDATE article_groups SET is_default = 0 WHERE is_default = 1")
            updates.append("is_default = %s")
            params.append(data.is_default)

        if not updates:
            return {"success": True, "message": "无需更新"}

        params.append(group_id)
        sql = f"UPDATE article_groups SET {', '.join(updates)} WHERE id = %s"
        await execute_query(sql, tuple(params))

        return {"success": True}
    except Exception as e:
        if "Duplicate" in str(e):
            return {"success": False, "message": "分组名称已存在"}
        logger.error(f"Failed to update article group: {e}")
        return {"success": False, "message": str(e)}


@router.get("/api/articles/list")
async def list_articles(
    group_id: int = Query(default=1),
    page: int = Query(default=1, ge=1),
    page_size: int = Query(default=20, ge=1, le=100),
    search: Optional[str] = None,
    _: bool = Depends(verify_token)
):
    """分页获取文章列表"""
    try:
        offset = (page - 1) * page_size

        # 构建查询条件（按分组过滤，status: 1=可用, 0=已删除）
        where_clause = "group_id = %s AND status = 1"
        params = [group_id]

        if search:
            where_clause += " AND (title LIKE %s OR content LIKE %s)"
            params.extend([f"%{search}%", f"%{search}%"])

        # 获取总数
        count_sql = f"SELECT COUNT(*) FROM original_articles WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

        # 获取列表
        params.extend([page_size, offset])
        list_sql = f"""
            SELECT id, group_id, title, LEFT(content, 200) as content, status, created_at, updated_at
            FROM original_articles
            WHERE {where_clause}
            ORDER BY id DESC
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
        logger.error(f"Failed to list articles: {e}")
        return {"items": [], "total": 0, "page": page, "page_size": page_size}


@router.get("/api/articles/{article_id}")
async def get_article(article_id: int, _: bool = Depends(verify_token)):
    """获取单篇文章"""
    try:
        article = await fetch_one(
            "SELECT id, group_id, title, content, status, source_url, created_at, updated_at FROM original_articles WHERE id = %s",
            (article_id,)
        )
        if not article:
            raise HTTPException(status_code=404, detail="文章不存在")
        return article
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to get article: {e}")
        raise HTTPException(status_code=500, detail="获取文章失败")


@router.put("/api/articles/{article_id}")
async def update_article(article_id: int, data: ArticleUpdate, _: bool = Depends(verify_token)):
    """更新文章"""
    try:
        # 检查文章是否存在
        existing = await fetch_one("SELECT id FROM original_articles WHERE id = %s", (article_id,))
        if not existing:
            return {"success": False, "message": "文章不存在"}

        # 构建更新字段
        updates = []
        params = []

        if data.group_id is not None:
            updates.append("group_id = %s")
            params.append(data.group_id)
        if data.title is not None:
            updates.append("title = %s")
            params.append(data.title)
        if data.content is not None:
            updates.append("content = %s")
            params.append(data.content)
        if data.status is not None:
            updates.append("status = %s")
            params.append(data.status)

        if not updates:
            return {"success": False, "message": "没有要更新的字段"}

        updates.append("updated_at = NOW()")
        params.append(article_id)

        await execute_query(
            f"UPDATE original_articles SET {', '.join(updates)} WHERE id = %s",
            tuple(params)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to update article: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/articles/{article_id}")
async def delete_article(article_id: int, _: bool = Depends(verify_token)):
    """删除文章（软删除，标记为archived）"""
    try:
        await execute_query(
            "UPDATE original_articles SET status = 0 WHERE id = %s",
            (article_id,)
        )
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete article: {e}")
        return {"success": False, "message": str(e)}


@router.delete("/api/articles/batch/delete")
async def batch_delete_articles(data: BatchIds, _: bool = Depends(verify_token)):
    """批量删除文章（软删除）"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "deleted": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE original_articles SET status = 0 WHERE id IN ({placeholders})",
            tuple(data.ids)
        )
        return {"success": True, "deleted": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch delete articles: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.delete("/api/articles/delete-all")
async def delete_all_articles(data: DeleteAllRequest, _: bool = Depends(verify_token)):
    """删除全部文章（软删除）"""
    if not data.confirm:
        return {"success": False, "message": "请确认删除操作", "deleted": 0}

    try:
        if data.group_id:
            result = await execute_query(
                "UPDATE original_articles SET status = 0 WHERE group_id = %s AND status = 1",
                (data.group_id,)
            )
        else:
            result = await execute_query(
                "UPDATE original_articles SET status = 0 WHERE status = 1"
            )
        deleted_count = result.rowcount if hasattr(result, 'rowcount') else 0
        return {"success": True, "deleted": deleted_count}
    except Exception as e:
        logger.error(f"Failed to delete all articles: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.put("/api/articles/batch/status")
async def batch_update_article_status(data: BatchStatusUpdate, _: bool = Depends(verify_token)):
    """批量更新文章状态"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "updated": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE original_articles SET status = %s WHERE id IN ({placeholders})",
            (data.status, *data.ids)
        )
        return {"success": True, "updated": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch update article status: {e}")
        return {"success": False, "message": str(e), "updated": 0}


@router.put("/api/articles/batch/move")
async def batch_move_articles(data: BatchMoveGroup, _: bool = Depends(verify_token)):
    """批量移动文章到其他分组"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "moved": 0}

    try:
        # 验证目标分组存在
        group = await fetch_one(
            "SELECT id FROM article_groups WHERE id = %s AND status = 1",
            (data.group_id,)
        )
        if not group:
            return {"success": False, "message": "目标分组不存在", "moved": 0}

        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE original_articles SET group_id = %s WHERE id IN ({placeholders})",
            (data.group_id, *data.ids)
        )
        return {"success": True, "moved": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch move articles: {e}")
        return {"success": False, "message": str(e), "moved": 0}


@router.post("/api/articles/add")
async def add_article(
    data: ArticleCreate,
    _: dict = Depends(verify_token_or_api_token)
):
    """
    添加单篇文章

    1. 插入MySQL
    2. 推送ID到Redis队列等待处理
    """
    try:
        # 使用 INSERT IGNORE 防止重复
        article_id = await insert('original_articles', {
            'group_id': data.group_id,
            'title': data.title,
            'content': data.content,
            'status': 1
        })

        if article_id:
            # 推送到待处理队列
            redis_client = get_redis_client()
            if redis_client:
                queue_key = f"pending:articles:{data.group_id}"
                await redis_client.lpush(queue_key, article_id)
            return {"success": True, "id": article_id}

        return {"success": False, "message": "文章已存在或添加失败"}
    except Exception as e:
        logger.error(f"Failed to add article: {e}")
        return {"success": False, "message": str(e)}


@router.post("/api/articles/batch")
async def add_articles_batch(
    data: ArticleBatchCreate,
    _: dict = Depends(verify_token_or_api_token)
):
    """
    批量添加文章（每次最多1000条）
    """
    if len(data.articles) > 1000:
        raise HTTPException(status_code=400, detail="Maximum 1000 articles per batch")

    if not data.articles:
        return {"success": True, "added": 0, "failed": 0}

    try:
        added = 0
        failed = 0
        new_ids = []

        # 逐条插入（使用 INSERT IGNORE 跳过重复）
        for article in data.articles:
            try:
                article_id = await insert('original_articles', {
                    'group_id': article.group_id,
                    'title': article.title,
                    'content': article.content,
                    'status': 1
                })
                if article_id:
                    new_ids.append((article_id, article.group_id))
                    added += 1
                else:
                    failed += 1
            except Exception:
                failed += 1

        # 批量推送到待处理队列
        redis_client = get_redis_client()
        if new_ids and redis_client:
            # 按 group_id 分组推送
            from collections import defaultdict
            groups = defaultdict(list)
            for aid, gid in new_ids:
                groups[gid].append(aid)

            pipe = redis_client.pipeline()
            for gid, ids in groups.items():
                queue_key = f"pending:articles:{gid}"
                for aid in ids:
                    pipe.lpush(queue_key, aid)
            await pipe.execute()

        return {"success": True, "added": added, "failed": failed}
    except Exception as e:
        logger.error(f"Failed to batch add articles: {e}")
        return {"success": False, "added": 0, "failed": len(data.articles), "message": str(e)}


# ============================================
# 图片URL管理API（新增）
# ============================================

@router.post("/api/images/urls/add")
async def add_image_url(
    data: ImageUrlCreate,
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: dict = Depends(verify_token_or_api_token)
):
    """
    添加单个图片URL

    1. 插入MySQL（唯一索引自动去重）
    2. 写入Redis缓存
    3. 追加ID到内存列表
    """
    image_id = await group.add_url(url=data.url, group_id=data.group_id)

    if image_id:
        return {"success": True, "id": image_id}
    return {"success": False, "message": "URL already exists or failed to add"}


@router.post("/api/images/urls/batch")
async def add_image_urls_batch(
    data: ImageUrlBatchCreate,
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: dict = Depends(verify_token_or_api_token)
):
    """
    批量添加图片URL（每次最多100000条）

    使用 INSERT IGNORE 跳过重复URL
    """
    if len(data.urls) > 100000:
        raise HTTPException(status_code=400, detail="Maximum 100000 URLs per batch")

    result = await group.add_urls_batch(urls=data.urls, group_id=data.group_id)
    return {"success": True, **result}


@router.get("/api/images/urls/stats")
async def get_image_url_stats(
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """获取图片URL分组统计信息"""
    return group.get_stats()


@router.get("/api/images/urls/random")
async def get_random_image_urls(
    count: int = Query(default=10, ge=1, le=100),
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """获取随机图片URL"""
    urls = await group.get_random(count)
    return {"urls": urls, "count": len(urls)}


@router.post("/api/images/urls/reload")
async def reload_image_urls(
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """重新加载图片URL ID列表"""
    count = await group.reload()
    return {"success": True, "total": count}


@router.post("/api/images/cache/clear")
async def clear_image_cache(
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """清理图片Redis缓存"""
    try:
        redis_client = group.redis
        group_id = group.group_id

        # 删除图片缓存 hash
        hash_key = f"images:pool:{group_id}"
        new_ids_key = f"images:new_ids:{group_id}"
        url_hash_key = f"images:url_hashes:{group_id}"

        cleared = 0
        for key in [hash_key, new_ids_key, url_hash_key]:
            if await redis_client.exists(key):
                await redis_client.delete(key)
                cleared += 1

        # 重置内存状态
        group._pool_start = 0
        group._pool_end = 0
        group._cursor = 0

        return {"success": True, "cleared": cleared, "message": f"已清理图片缓存"}
    except Exception as e:
        logger.error(f"Failed to clear image cache: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 关键词管理API（MySQL + Redis架构）
# ============================================

@router.post("/api/keywords/add")
async def add_keyword(
    data: KeywordCreate,
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: dict = Depends(verify_token_or_api_token)
):
    """
    添加单个关键词

    1. 插入MySQL（唯一索引自动去重）
    2. 写入Redis缓存
    3. 追加ID到内存列表
    """
    keyword_id = await group.add_keyword(keyword=data.keyword, group_id=data.group_id)

    if keyword_id:
        return {"success": True, "id": keyword_id}
    return {"success": False, "message": "Keyword already exists or failed to add"}


@router.post("/api/keywords/batch")
async def add_keywords_batch(
    data: KeywordBatchCreate,
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: dict = Depends(verify_token_or_api_token)
):
    """
    批量添加关键词（每次最多100000条）

    使用 INSERT IGNORE 跳过重复关键词
    """
    if len(data.keywords) > 100000:
        raise HTTPException(status_code=400, detail="Maximum 100000 keywords per batch")

    result = await group.add_keywords_batch(keywords=data.keywords, group_id=data.group_id)
    return {"success": True, **result}


@router.post("/api/keywords/upload")
async def upload_keywords_file(
    file: UploadFile = File(...),
    group_id: int = Form(default=1),
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """
    上传 TXT 文件批量添加关键词

    - 文件格式：TXT（一行一个关键词）
    - 自动过滤空行和空格
    - 自动去重（数据库唯一索引）
    """
    # 验证文件类型
    if not file.filename or not file.filename.endswith('.txt'):
        raise HTTPException(status_code=400, detail="只支持 .txt 格式文件")

    # 读取文件内容
    content = await file.read()

    # 解码（支持 UTF-8 和 GBK）
    try:
        text = content.decode('utf-8')
    except UnicodeDecodeError:
        try:
            text = content.decode('gbk')
        except UnicodeDecodeError:
            text = content.decode('utf-8', errors='ignore')

    # 解析关键词
    keywords = [line.strip() for line in text.splitlines() if line.strip()]

    if not keywords:
        raise HTTPException(status_code=400, detail="文件中没有有效的关键词")

    if len(keywords) > 500000:
        raise HTTPException(status_code=400, detail="单次最多上传 500000 个关键词")

    # 调用批量添加
    result = await group.add_keywords_batch(keywords, group_id)

    return {
        "success": True,
        "message": f"成功添加 {result['added']} 个关键词，跳过 {result['skipped']} 个重复",
        "total": len(keywords),
        "added": result['added'],
        "skipped": result['skipped']
    }


@router.get("/api/keywords/stats")
async def get_keyword_stats(
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """获取关键词分组统计信息"""
    return group.get_stats()


@router.get("/api/keywords/random")
async def get_random_keywords(
    count: int = Query(default=10, ge=1, le=100),
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """获取随机关键词"""
    keywords = await group.get_random(count)
    return {"keywords": keywords, "count": len(keywords)}


@router.post("/api/keywords/reload")
async def reload_keywords(
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """重新加载关键词ID列表"""
    count = await group.reload()
    return {"success": True, "total": count}


@router.post("/api/keywords/cache/clear")
async def clear_keyword_cache(
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """清理关键词Redis缓存"""
    try:
        redis_client = group.redis

        # 使用 SCAN 查找所有 keyword:* 键并删除
        cleared = 0
        cursor = 0
        while True:
            cursor, keys = await redis_client.scan(cursor, match="keyword:*", count=1000)
            if keys:
                await redis_client.delete(*keys)
                cleared += len(keys)
            if cursor == 0:
                break

        # 重置内存状态
        group._cursor = 0

        return {"success": True, "cleared": cleared, "message": f"已清理 {cleared} 个关键词缓存"}
    except Exception as e:
        logger.error(f"Failed to clear keyword cache: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 缓存管理API
# ============================================

@router.get("/api/cache/stats")
async def cache_stats(
        cache: HTMLCacheManager = Depends(get_cache),
        _: bool = Depends(verify_token)
):
    """获取缓存统计"""
    return cache.get_stats()


@router.get("/api/cache-pools/stats")
async def get_cache_pools_stats(_: bool = Depends(verify_token)):
    """获取关键词和图片缓存池统计"""
    from core.keyword_cache_pool import get_keyword_cache_pool
    from core.image_cache_pool import get_image_cache_pool

    keyword_pool = get_keyword_cache_pool()
    image_pool = get_image_cache_pool()

    return {
        "keyword_pool": keyword_pool.get_stats() if keyword_pool else None,
        "image_pool": image_pool.get_stats() if image_pool else None,
    }


@router.post("/api/cache/clear")
async def clear_cache(
        cache: HTMLCacheManager = Depends(get_cache),
        _: bool = Depends(verify_token)
):
    """清空全部缓存"""
    count = await cache.clear()
    return {"success": True, "cleared": count}


@router.post("/api/cache/clear/{domain}")
async def clear_domain_cache(
        domain: str,
        cache: HTMLCacheManager = Depends(get_cache),
        _: bool = Depends(verify_token)
):
    """清空指定域名缓存"""
    count = await cache.clear(domain)
    return {"success": True, "cleared": count}


@router.get("/api/cache/entries")
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
    # 无域名参数时返回总体统计
    stats = cache.get_stats()
    return {
        "items": [],
        "total_entries": stats.get('total_entries', 0),
        "total_size_mb": stats.get('total_size_mb', 0),
        "offset": offset,
        "limit": limit
    }


@router.post("/api/cache/warmup")
async def warmup_cache(
        request: CacheWarmupRequest,
        seo: SEOCore = Depends(get_seo),
        cache: HTMLCacheManager = Depends(get_cache),
        _: bool = Depends(verify_token)
):
    """缓存预热"""
    # TODO: 实现异步批量生成
    return {"success": True, "message": "Warmup task started"}


# ============================================
# 蜘蛛检测API
# ============================================

@router.get("/api/spiders/config")
async def get_spider_config(_: bool = Depends(verify_token)):
    """获取蜘蛛检测配置"""
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

        # 构建查询条件
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

        # 获取总数
        count_sql = f"SELECT COUNT(*) FROM spider_logs WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

        # 获取列表
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
        # 总计
        total = await fetch_value("SELECT COUNT(*) FROM spider_logs") or 0
        today_total = await fetch_value(
            "SELECT COUNT(*) FROM spider_logs WHERE DATE(created_at) = CURDATE()"
        ) or 0

        # 按蜘蛛类型统计
        by_type = await fetch_all("""
            SELECT spider_type, COUNT(*) as count
            FROM spider_logs
            GROUP BY spider_type
            ORDER BY count DESC
        """) or []

        # 按域名统计（Top 10）
        by_domain = await fetch_all("""
            SELECT domain, COUNT(*) as count
            FROM spider_logs
            GROUP BY domain
            ORDER BY count DESC
            LIMIT 10
        """) or []

        # 按状态码统计
        by_status = await fetch_all("""
            SELECT status, COUNT(*) as count
            FROM spider_logs
            GROUP BY status
            ORDER BY status
        """) or []

        # 缓存命中率
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

        # 平均响应时间
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

        # 整理数据结构
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
        # 如果没有指定日期，使用今天
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

        # 整理数据结构，确保 0-23 小时都有数据
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
        result = await execute_query(
            f"DELETE FROM spider_logs WHERE created_at < DATE_SUB(NOW(), INTERVAL %s DAY)",
            (before_days,)
        )
        logger.info(f"Cleared spider logs older than {before_days} days")
        return {"success": True, "message": f"已清理 {before_days} 天前的日志"}
    except Exception as e:
        logger.error(f"Failed to clear spider logs: {e}")
        return {"success": False, "message": str(e)}


# ============================================
# 系统设置API
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


# 缓存配置默认值
CACHE_DEFAULT_SETTINGS = {
    'keyword_cache_ttl': ('86400', 'number', '关键词缓存过期时间(秒)'),
    'image_cache_ttl': ('86400', 'number', '图片URL缓存过期时间(秒)'),
    'cache_compress_enabled': ('true', 'boolean', '是否启用缓存压缩'),
    'cache_compress_level': ('6', 'number', '压缩级别(1-9)'),
    'keyword_pool_size': ('500000', 'number', '关键词池大小(0=不限制)'),
    'image_pool_size': ('500000', 'number', '图片池大小(0=不限制)'),
    'article_pool_size': ('50000', 'number', '文章池大小(0=不限制)'),
    # 文件缓存配置
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

        # 自动创建缺失的设置项
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
            # 检查设置是否存在
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
                # 新增设置
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

        # 文件缓存配置变更提示
        file_cache_enabled = config.get('file_cache_enabled', False)
        if file_cache_enabled:
            applied.append("file_cache_enabled=true (需要重启服务生效)")

        # 定义需要重载的分组及其配置
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

        # 更新 token
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

        # 更新 enabled 状态
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

        # 获取队列中已有的文章ID（避免重复推送）
        queue_key = f"pending:articles:{group_id}"
        existing_ids = set()
        existing_items = await redis_client.lrange(queue_key, 0, -1)
        for item in existing_items:
            try:
                existing_ids.add(int(item))
            except (ValueError, TypeError):
                pass

        # 获取需要推送的文章ID
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

        # 过滤已存在的ID
        new_ids = [a['id'] for a in articles if a['id'] not in existing_ids]
        skipped = len(articles) - len(new_ids)

        if not new_ids:
            return {"success": True, "pushed": 0, "skipped": skipped, "message": "所有文章已在队列中"}

        # 批量推送到队列
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
    from main import _generator_worker
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
        # 获取标题数量
        titles_count = await fetch_value("SELECT COUNT(*) FROM titles") or 0

        # 获取正文数量
        contents_count = await fetch_value("SELECT COUNT(*) FROM contents") or 0

        # 获取原始文章数量
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
