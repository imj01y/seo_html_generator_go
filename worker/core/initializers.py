# -*- coding: utf-8 -*-
"""
组件初始化模块

提供所有系统组件的初始化逻辑，包括数据库、Redis、缓存池等。
"""
import asyncio
from pathlib import Path
from typing import Optional

from loguru import logger

from config import get_config
from database.db import init_database, init_db_pool, get_db_pool
from core.redis_client import init_redis_client, get_redis_client
from core.html_cache_manager import init_cache_manager
from core.spider_detector import init_spider_detector
from core.seo_core import init_seo_core, get_seo_core
from core.emoji import get_emoji_manager
from core.auth import ensure_default_admin
from core.keyword_group_manager import init_keyword_group, get_keyword_group
from core.image_group_manager import init_image_group, get_image_group
from core.keyword_cache_pool import init_keyword_cache_pool, get_keyword_cache_pool
from core.image_cache_pool import init_image_cache_pool, get_image_cache_pool
from core.class_generator import init_class_string_pool, get_class_string_pool
from core.link_generator import init_url_pool, get_url_pool
from core.random_number_pool import init_random_number_pool, get_random_number_pool
from core.title_manager import init_title_manager, get_title_manager
from core.content_manager import init_content_manager, get_content_manager
from core.content_pool_manager import init_content_pool_manager


# 全局变量：worker 引用（用于清理）
_generator_worker = None
_scheduler_worker = None


def get_generator_worker():
    """获取生成器 worker 引用"""
    return _generator_worker


def _parse_setting_value(value: str, setting_type: str):
    """解析配置值为对应类型"""
    if setting_type == 'boolean':
        return value.lower() in ('true', '1', 'yes')
    if setting_type == 'number':
        return float(value) if '.' in value else int(value)
    return value


async def _load_file_cache_config() -> dict:
    """从数据库加载文件缓存配置"""
    if not get_db_pool():
        return {}

    try:
        from database.db import fetch_all
        settings = await fetch_all(
            "SELECT setting_key, setting_value, setting_type FROM system_settings "
            "WHERE setting_key LIKE 'file_cache%'"
        )
        return {
            s['setting_key']: _parse_setting_value(s['setting_value'], s['setting_type'])
            for s in (settings or [])
        }
    except Exception as e:
        logger.warning(f"Failed to load file cache settings from database: {e}")
        return {}


async def _start_background_worker(worker_class, name: str, **kwargs):
    """启动后台 worker 的通用函数"""
    global _generator_worker

    try:
        worker = worker_class(**kwargs)

        if name == 'generator':
            _generator_worker = worker
            await worker.run_forever(group_id=1)

    except asyncio.CancelledError:
        logger.info(f"{name.capitalize()} worker cancelled")
    except Exception as e:
        logger.error(f"{name.capitalize()} worker error: {e}")


async def init_database_components(config, project_root: Path):
    """
    初始化数据库相关组件

    Args:
        config: 配置对象
        project_root: 项目根目录
    """
    # 1. 初始化数据库（创建数据库和表）
    try:
        await init_database(
            host=config.database.host,
            port=config.database.port,
            user=config.database.user,
            password=config.database.password,
            database=config.database.database,
            charset=config.database.charset,
            schema_file=str(project_root / "database" / "schema.sql")
        )
    except Exception as e:
        logger.warning(f"Database schema initialization failed: {e}")

    # 2. 初始化数据库连接池
    try:
        await init_db_pool(
            host=config.database.host,
            port=config.database.port,
            user=config.database.user,
            password=config.database.password,
            database=config.database.database,
            charset=config.database.charset,
            pool_size=config.database.pool_size
        )
        logger.info("Database pool initialized")

        # 确保默认管理员存在
        await ensure_default_admin()

    except Exception as e:
        logger.warning(f"Database pool initialization failed (non-critical): {e}")


async def init_cache_components(config):
    """
    初始化缓存相关组件

    Args:
        config: 配置对象
    """
    # 加载文件缓存配置
    file_cache_config = await _load_file_cache_config()

    # 初始化文件缓存
    try:
        init_cache_manager(
            cache_dir=file_cache_config.get('file_cache_dir', './html_cache'),
            max_size_gb=file_cache_config.get('file_cache_max_size_gb', 50),
            enable_gzip=not file_cache_config.get('file_cache_nginx_mode', True),
            nginx_mode=file_cache_config.get('file_cache_nginx_mode', True)
        )
        logger.info(f"File HTML cache initialized: dir={file_cache_config.get('file_cache_dir', './html_cache')}, "
                   f"nginx_mode={file_cache_config.get('file_cache_nginx_mode', True)}")
    except Exception as e:
        logger.error(f"Failed to initialize file cache: {e}")

    # 初始化Redis客户端
    if hasattr(config, 'redis') and config.redis.enabled:
        try:
            await init_redis_client(
                host=config.redis.host,
                port=config.redis.port,
                db=config.redis.db,
                password=config.redis.password or None,
            )
            logger.info("Redis client initialized (for queue operations)")
        except Exception as e:
            logger.warning(f"Redis initialization failed: {e}")


async def init_pool_components():
    """初始化各类缓存池组件"""
    redis_client = get_redis_client()
    db_pool = get_db_pool()

    if not (redis_client and db_pool):
        logger.warning("Pool components not initialized (Redis client or DB not ready)")
        return

    # 关键词分组
    try:
        await init_keyword_group(db_pool)
        logger.info("Keyword group initialized from MySQL")
    except Exception as e:
        logger.warning(f"Keyword group initialization failed: {e}")

    # 图片分组
    try:
        await init_image_group(db_pool)
        logger.info("Image group initialized from MySQL")
    except Exception as e:
        logger.warning(f"Image group initialization failed: {e}")


async def init_seo_components(project_root: Path):
    """
    初始化 SEO 核心组件

    Args:
        project_root: 项目根目录
    """
    # 初始化 SEO 核心
    templates_dir = Path(project_root) / "templates"
    init_seo_core(
        template_dir=str(templates_dir),
        encoding_ratio=0.5
    )
    logger.info("SEO core initialized")

    # 初始化随机字符串池
    try:
        await init_class_string_pool(
            pool_size=50000,
            low_watermark_ratio=0.3,
            refill_batch_size=10000,
            check_interval=0.3,
            num_workers=4
        )
        pool = get_class_string_pool()
        if pool:
            stats = pool.get_stats()
            logger.info(f"Class string pool initialized: {stats['buffer_size']} strings, workers=4")
    except Exception as e:
        logger.warning(f"Class string pool initialization failed: {e}")

    # 初始化 URL 池
    try:
        await init_url_pool(
            pool_size=20000,
            low_watermark_ratio=0.3,
            refill_batch_size=5000,
            check_interval=0.3,
            num_workers=2
        )
        pool = get_url_pool()
        if pool:
            stats = pool.get_stats()
            logger.info(f"URL pool initialized: {stats['buffer_size']} URLs, workers=2")
    except Exception as e:
        logger.warning(f"URL pool initialization failed: {e}")

    # 初始化随机数池
    try:
        await init_random_number_pool(
            ranges={
                "0-9": (0, 9),
                "0-99": (0, 99),
                "1-9": (1, 9),
                "1-10": (1, 10),
                "1-20": (1, 20),
                "1-59": (1, 59),
                "5-10": (5, 10),
                "10-20": (10, 20),
                "10-99": (10, 99),
                "10-100": (10, 100),
                "10-200": (10, 200),
                "30-90": (30, 90),
                "50-200": (50, 200),
                "100-999": (100, 999),
                "10000-99999": (10000, 99999),
            },
            pool_size=20000,
            low_watermark_ratio=0.3,
            refill_batch_size=5000,
            check_interval=0.3,
            num_workers=2
        )
        pool = get_random_number_pool()
        if pool:
            stats = pool.get_stats()
            logger.info(f"Random number pool initialized: {stats['ranges_count']} ranges, workers=2")
    except Exception as e:
        logger.warning(f"Random number pool initialization failed: {e}")


async def init_keyword_image_cache_pools():
    """初始化关键词和图片缓存池"""
    redis_client = get_redis_client()
    keyword_group = get_keyword_group()
    image_group = get_image_group()
    seo_core = get_seo_core()

    # 关键词缓存池
    if redis_client and keyword_group:
        try:
            await init_keyword_cache_pool(
                keyword_manager=keyword_group,
                redis_client=redis_client,
                cache_size=10000,
                low_watermark_ratio=0.2,
                refill_batch_size=2000,
                check_interval=1.0,
                encoder=seo_core.encoder if seo_core else None
            )
            pool = get_keyword_cache_pool()
            if pool:
                stats = pool.get_stats()
                if stats['cache_size'] == 0:
                    logger.warning("Keyword cache pool started empty - will populate when data is added")
                else:
                    logger.info(f"Keyword cache pool initialized: {stats['cache_size']} pre-encoded keywords")
        except Exception as e:
            logger.warning(f"Keyword cache pool initialization failed: {e}")

    # 图片缓存池
    if redis_client and image_group:
        try:
            await init_image_cache_pool(
                image_manager=image_group,
                redis_client=redis_client,
                cache_size=30000,
                low_watermark_ratio=0.3,
                refill_batch_size=5000,
                check_interval=0.5
            )
            pool = get_image_cache_pool()
            if pool:
                stats = pool.get_stats()
                if stats['cache_size'] == 0:
                    logger.warning("Image cache pool started empty - will populate when data is added")
                else:
                    logger.info(f"Image cache pool initialized: {stats['cache_size']} URLs")
        except Exception as e:
            logger.warning(f"Image cache pool initialization failed: {e}")


async def init_content_components():
    """初始化内容相关组件（标题、正文管理器）"""
    redis_client = get_redis_client()
    db_pool = get_db_pool()

    if not (redis_client and db_pool):
        return

    # 标题管理器
    try:
        await init_title_manager(redis_client, db_pool, group_id=1, max_size=500000)
        title_manager = get_title_manager(group_id=1)
        if title_manager:
            stats = title_manager.get_stats()
            logger.info(f"Title manager (group 1) initialized: {stats['total_loaded']} titles loaded")
    except Exception as e:
        logger.warning(f"Title manager initialization failed: {e}")

    # 正文管理器
    try:
        await init_content_manager(redis_client, db_pool, group_id=1, max_size=50000)
        content_manager = get_content_manager(group_id=1)
        if content_manager:
            stats = content_manager.get_stats()
            logger.info(f"Content manager (group 1) initialized: {stats['total']} contents loaded")
    except Exception as e:
        logger.warning(f"Content manager initialization failed: {e}")

    # 段落池管理器
    try:
        content_pool = await init_content_pool_manager(
            redis_client, db_pool, group_id=1, auto_initialize=True
        )
        if content_pool:
            stats = await content_pool.get_pool_stats()
            logger.info(
                f"Content pool manager initialized: "
                f"{stats['pool_size']} available, {stats['used_size']} used"
            )
    except Exception as e:
        logger.warning(f"Content pool manager initialization failed: {e}")


async def init_background_workers():
    """初始化后台工作线程"""
    global _scheduler_worker

    redis_client = get_redis_client()
    db_pool = get_db_pool()

    if not (redis_client and db_pool):
        return

    # 正文生成器后台任务
    try:
        from core.workers.generator_worker import GeneratorWorker
        asyncio.create_task(_start_background_worker(
            GeneratorWorker, 'generator',
            db_pool=db_pool, redis_client=redis_client
        ))
        logger.info("Generator worker started in background")
    except Exception as e:
        logger.warning(f"Generator worker start failed: {e}")

    # 注意：爬虫统计归档已迁移到 Go API 的 StatsArchiver 服务

    # 爬虫定时调度器
    try:
        from core.workers.spider_scheduler import SpiderSchedulerWorker
        _scheduler_worker = SpiderSchedulerWorker(db_pool=db_pool, redis=redis_client)
        await _scheduler_worker.start()
        logger.info("Spider scheduler worker started")
    except Exception as e:
        logger.warning(f"Spider scheduler worker start failed: {e}")


async def preload_sync_caches():
    """预加载同步缓存（作为缓存池的降级方案）"""
    seo_core = get_seo_core()
    keyword_group = get_keyword_group()
    image_group = get_image_group()

    if not seo_core:
        return

    # 关键词同步缓存
    if keyword_group and keyword_group._loaded:
        keywords = await keyword_group.get_random(1000)
        seo_core.load_keywords_sync(keywords)
        logger.info(f"Preloaded {len(keywords)} keywords to sync cache (fallback)")

    # 图片同步缓存
    if image_group and image_group._loaded:
        urls = await image_group.get_random(1000)
        seo_core.load_image_urls_sync(urls)
        logger.info(f"Preloaded {len(urls)} image URLs to sync cache")


async def init_components(project_root: Optional[Path] = None):
    """
    初始化所有组件

    Args:
        project_root: 项目根目录路径，默认为当前文件所在目录的父目录
    """
    if project_root is None:
        project_root = Path(__file__).parent.parent

    config = get_config()
    logger.info("Initializing components...")

    # 1-2. 数据库组件
    await init_database_components(config, project_root)

    # 3. 缓存组件
    await init_cache_components(config)

    # 4. 蜘蛛检测器
    init_spider_detector(
        enable_dns_verify=config.spider_detector.dns_verify_enabled,
        dns_verify_types=config.spider_detector.dns_verify_types,
        dns_timeout=config.spider_detector.dns_timeout
    )
    logger.info("Spider detector initialized")

    # 5. Emoji 管理器
    emoji_manager = get_emoji_manager()
    logger.info(f"Emoji manager initialized: {emoji_manager.count()} emojis")

    # 6-9. 分组管理器
    await init_pool_components()

    # 10. SEO 核心组件
    await init_seo_components(project_root)

    # 11. 关键词和图片缓存池
    await init_keyword_image_cache_pools()

    # 12. 预加载同步缓存
    await preload_sync_caches()

    # 13-14. 内容组件
    await init_content_components()

    # 15-16. 后台工作线程
    await init_background_workers()

    logger.info("All components initialized successfully")
