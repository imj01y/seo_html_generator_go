"""
SEO HTML生成器 - 应用入口

启动FastAPI服务，初始化所有组件。

使用方法:
    # 直接运行
    python main.py

    # 使用uvicorn运行
    uvicorn main:app --host 0.0.0.0 --port 8000

    # 开发模式（自动重载）
    uvicorn main:app --reload

    # 测试场景
    from main import create_app
    app = create_app("test_config.yaml")
    client = TestClient(app)
"""
import asyncio
import logging
import os
import sys
from contextlib import asynccontextmanager
from pathlib import Path
from typing import Optional

import uvicorn
from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse
from loguru import logger

# 禁用 websockets 库的 DEBUG 日志（帧级别调试信息，会被截断显示不完整）
logging.getLogger("websockets").setLevel(logging.INFO)

# 添加项目根目录到Python路径
project_root = Path(__file__).parent
if str(project_root) not in sys.path:
    sys.path.insert(0, str(project_root))

# 配置loguru日志
logger.remove()  # 移除默认handler
logger.add(
    sys.stderr,
    format="<green>{time:YYYY-MM-DD HH:mm:ss}</green> | <level>{level: <8}</level> | <cyan>{name}</cyan>:<cyan>{function}</cyan>:<cyan>{line}</cyan> - <level>{message}</level>",
    level="INFO"
)
logger.add(
    "seo_generator.log",
    rotation="10 MB",
    retention="7 days",
    encoding="utf-8",
    format="{time:YYYY-MM-DD HH:mm:ss} | {level: <8} | {name}:{function}:{line} - {message}",
    level="INFO"
)

from config import get_config, reload_config
from core.seo_core import init_seo_core, get_seo_core
from core.spider_detector import init_spider_detector
from core.redis_client import init_redis_client, close_redis_client, get_redis_client
from core.html_cache_manager import init_cache_manager, get_cache_manager
from core.keyword_group_manager import init_keyword_group, get_keyword_group
from core.keyword_cache_pool import init_keyword_cache_pool, stop_keyword_cache_pool, get_keyword_cache_pool
from core.image_cache_pool import init_image_cache_pool, stop_image_cache_pool, get_image_cache_pool
from core.image_group_manager import init_image_group, get_image_group
from core.emoji import get_emoji_manager
from core.auth import ensure_default_admin
from core.title_manager import init_title_manager, get_title_manager
from core.content_manager import init_content_manager, get_content_manager
from core.content_pool_manager import init_content_pool_manager, get_content_pool_manager
from database.db import init_database, init_db_pool, close_db_pool, get_db_pool
from api.routes import router as api_router
from api.generator_routes import router as generator_router
from api.log_routes import router as log_router
from api.spider_routes import router as spider_router, stats_router

# 全局变量：worker 引用（用于清理）
_generator_worker = None
_stats_worker = None
_scheduler_worker = None


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
    global _generator_worker, _stats_worker

    try:
        worker = worker_class(**kwargs)

        if name == 'generator':
            _generator_worker = worker
            await worker.run_forever(group_id=1)
        else:  # 'stats'
            _stats_worker = worker
            await worker.start()

    except asyncio.CancelledError:
        logger.info(f"{name.capitalize()} worker cancelled")
    except Exception as e:
        logger.error(f"{name.capitalize()} worker error: {e}")


async def init_components():
    """初始化所有组件"""
    config = get_config()
    logger.info("Initializing components...")

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

        # 2.1 确保默认管理员存在（从配置文件读取）
        await ensure_default_admin()

    except Exception as e:
        logger.warning(f"Database pool initialization failed (non-critical): {e}")

    # 3. 初始化文件缓存（HTML缓存使用文件系统）
    file_cache_config = await _load_file_cache_config()

    # 3.1 初始化文件缓存
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

    # 3.2 初始化Redis客户端（用于队列、缓存池等功能，不用于HTML缓存）
    redis_client = None
    if hasattr(config, 'redis') and config.redis.enabled:
        try:
            redis_client = await init_redis_client(
                host=config.redis.host,
                port=config.redis.port,
                db=config.redis.db,
                password=config.redis.password or None,
            )
            logger.info("Redis client initialized (for queue operations)")
        except Exception as e:
            logger.warning(f"Redis initialization failed: {e}")

    # 4. 初始化蜘蛛检测器
    init_spider_detector(
        enable_dns_verify=config.spider_detector.dns_verify_enabled,
        dns_verify_types=config.spider_detector.dns_verify_types,
        dns_timeout=config.spider_detector.dns_timeout
    )
    logger.info("Spider detector initialized")

    # 5. 加载Emoji数据（get_emoji_manager 首次调用时自动加载）
    emoji_manager = get_emoji_manager()
    logger.info(f"Emoji manager initialized: {emoji_manager.count()} emojis")

    # 6. 获取Redis客户端和数据库连接（用于后续池初始化）
    redis_client = get_redis_client()
    db_pool = get_db_pool()

    # 7-9. 初始化各分组管理器（关键词、文章、图片）
    if redis_client and db_pool:

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
    else:
        logger.warning("Group managers not initialized (Redis client or DB not ready)")

    # 10. 初始化SEO核心
    templates_dir = Path(project_root) / "templates"
    init_seo_core(
        template_dir=str(templates_dir),
        encoding_ratio=0.5
    )
    logger.info("SEO core initialized")

    # 11. 初始化关键词缓存池（生产者消费者模型）
    # 允许空数据启动，新增数据后会自动加入缓存池
    keyword_group = get_keyword_group()
    if redis_client and keyword_group:
        try:
            await init_keyword_cache_pool(
                keyword_manager=keyword_group,
                redis_client=redis_client,
                cache_size=10000,
                low_watermark_ratio=0.2,
                refill_batch_size=2000,
                check_interval=1.0
            )
            pool = get_keyword_cache_pool()
            if pool:
                stats = pool.get_stats()
                if stats['cache_size'] == 0:
                    logger.warning("Keyword cache pool started empty - will populate when data is added")
                else:
                    logger.info(f"Keyword cache pool initialized: {stats['cache_size']} keywords, low_watermark={stats['low_watermark']}")
        except Exception as e:
            logger.warning(f"Keyword cache pool initialization failed: {e}")

    # 11.1 初始化图片缓存池（生产者消费者模型）
    # 允许空数据启动，新增数据后会自动加入缓存池
    image_group = get_image_group()
    if redis_client and image_group:
        try:
            await init_image_cache_pool(
                image_manager=image_group,
                redis_client=redis_client,
                cache_size=10000,
                low_watermark_ratio=0.2,
                refill_batch_size=2000,
                check_interval=1.0
            )
            pool = get_image_cache_pool()
            if pool:
                stats = pool.get_stats()
                if stats['cache_size'] == 0:
                    logger.warning("Image cache pool started empty - will populate when data is added")
                else:
                    logger.info(f"Image cache pool initialized: {stats['cache_size']} URLs, low_watermark={stats['low_watermark']}")
        except Exception as e:
            logger.warning(f"Image cache pool initialization failed: {e}")

    # 12. 预加载同步缓存到SEOCore（用于模板渲染，作为缓存池的降级方案）
    seo_core = get_seo_core()
    if seo_core:
        image_group = get_image_group()

        # 关键词同步缓存（降级方案，缓存池不可用时使用）
        if keyword_group and keyword_group._loaded:
            keywords = await keyword_group.get_random(1000)
            seo_core.load_keywords_sync(keywords)
            logger.info(f"Preloaded {len(keywords)} keywords to sync cache (fallback)")

        if image_group and image_group._loaded:
            # 从异步分组获取一批图片URL预填充同步缓存
            urls = await image_group.get_random(1000)
            seo_core.load_image_urls_sync(urls)
            logger.info(f"Preloaded {len(urls)} image URLs to sync cache")

    # 13-14. 初始化标题和正文管理器（预热默认分组，其他分组懒加载）
    if redis_client and db_pool:
        # 标题管理器（分层随机抽取）- 预热分组1
        try:
            await init_title_manager(redis_client, db_pool, group_id=1, max_size=500000)
            title_manager = get_title_manager(group_id=1)
            if title_manager:
                stats = title_manager.get_stats()
                logger.info(f"Title manager (group 1) initialized: {stats['total_loaded']} titles loaded")
        except Exception as e:
            logger.warning(f"Title manager initialization failed: {e}")

        # 正文管理器（从contents表读取已处理好的正文）- 预热分组1
        try:
            await init_content_manager(redis_client, db_pool, group_id=1, max_size=50000)
            content_manager = get_content_manager(group_id=1)
            if content_manager:
                stats = content_manager.get_stats()
                logger.info(f"Content manager (group 1) initialized: {stats['total']} contents loaded")
        except Exception as e:
            logger.warning(f"Content manager initialization failed: {e}")

        # 13.1 初始化段落池管理器（一次性消费模式）
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

        # 14. 启动正文生成器后台任务
        try:
            from core.workers.generator_worker import GeneratorWorker
            asyncio.create_task(_start_background_worker(
                GeneratorWorker, 'generator',
                db_pool=db_pool, redis_client=redis_client
            ))
            logger.info("Generator worker started in background")
        except Exception as e:
            logger.warning(f"Generator worker start failed: {e}")

        # 15. 启动爬虫统计归档后台任务
        try:
            from core.workers.stats_worker import SpiderStatsWorker
            asyncio.create_task(_start_background_worker(
                SpiderStatsWorker, 'stats',
                db_pool=db_pool, redis=redis_client
            ))
            logger.info("Spider stats worker started in background")
        except Exception as e:
            logger.warning(f"Spider stats worker start failed: {e}")

        # 16. 启动爬虫定时调度器
        try:
            from core.workers.spider_scheduler import SpiderSchedulerWorker
            global _scheduler_worker
            _scheduler_worker = SpiderSchedulerWorker(db_pool=db_pool, redis=redis_client)
            await _scheduler_worker.start()
            logger.info("Spider scheduler worker started")
        except Exception as e:
            logger.warning(f"Spider scheduler worker start failed: {e}")

    logger.info("All components initialized successfully")


async def _safe_stop(coro, name: str):
    """安全停止组件的辅助函数"""
    try:
        await coro
        logger.info(f"{name} stopped")
    except Exception as e:
        logger.warning(f"Error stopping {name}: {e}")


async def cleanup_components():
    """清理组件"""
    global _generator_worker, _stats_worker, _scheduler_worker
    logger.info("Cleaning up components...")

    # 停止 workers
    if _generator_worker:
        await _safe_stop(_generator_worker.stop(), "Generator worker")
        _generator_worker = None

    if _stats_worker:
        await _safe_stop(_stats_worker.stop(), "Spider stats worker")
        _stats_worker = None

    if _scheduler_worker:
        await _safe_stop(_scheduler_worker.stop(), "Spider scheduler worker")
        _scheduler_worker = None

    # 停止缓存池
    await _safe_stop(stop_keyword_cache_pool(), "Keyword cache pool")
    await _safe_stop(stop_image_cache_pool(), "Image cache pool")

    # 关闭连接
    await _safe_stop(close_redis_client(), "Redis client connection")
    await _safe_stop(close_db_pool(), "Database pool")

    logger.info("Cleanup completed")


@asynccontextmanager
async def lifespan(app: FastAPI):
    """应用生命周期管理"""
    # 启动时初始化
    await init_components()
    yield
    # 关闭时清理
    await cleanup_components()


def create_app(config_path: Optional[str] = None) -> FastAPI:
    """
    应用工厂函数

    创建并配置FastAPI应用实例。支持传入自定义配置文件路径，
    便于测试和多实例场景。

    Args:
        config_path: 可选的配置文件路径。如果提供且文件存在，
                     将加载该配置文件。

    Returns:
        配置好的FastAPI应用实例

    Example:
        # 默认配置
        >>> app = create_app()

        # 自定义配置
        >>> app = create_app("config_prod.yaml")

        # 测试场景
        >>> from fastapi.testclient import TestClient
        >>> app = create_app("test_config.yaml")
        >>> client = TestClient(app)
        >>> response = client.get("/health")
    """
    # 加载配置
    if config_path and Path(config_path).exists():
        reload_config(config_path)

    # 创建应用
    app = FastAPI(
        title="SEO HTML Generator",
        description="SEO站群HTML动态生成系统",
        version="1.0.0",
        lifespan=lifespan
    )

    # 配置CORS中间件
    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    # 挂载API路由
    app.include_router(api_router)
    app.include_router(generator_router)
    app.include_router(log_router)
    app.include_router(spider_router)
    app.include_router(stats_router)

    # 导入日志管理器
    from core.crawler.log_manager import log_manager

    # 日志 WebSocket 端点
    from core.logging import get_log_manager

    @app.websocket("/api/logs/ws")
    async def websocket_logs(websocket: WebSocket):
        """WebSocket 实时日志推送"""
        await websocket.accept()

        log_manager = get_log_manager()
        log_manager.register_websocket(websocket)

        try:
            while True:
                # 保持连接，接收客户端消息（如过滤条件）
                try:
                    data = await asyncio.wait_for(websocket.receive_text(), timeout=30)
                    # 可以处理客户端发来的过滤条件
                except asyncio.TimeoutError:
                    # 发送心跳
                    try:
                        await websocket.send_json({"type": "heartbeat"})
                    except Exception:
                        break
        except WebSocketDisconnect:
            pass
        except Exception as e:
            logger.debug(f"Log WebSocket error: {e}")
        finally:
            log_manager.unregister_websocket(websocket)

    # 项目日志 WebSocket 端点
    @app.websocket("/api/spider-projects/{project_id}/logs/ws")
    async def subscribe_project_logs_ws(websocket: WebSocket, project_id: int):
        """WebSocket 订阅爬虫项目执行日志"""
        logger.info(f"WebSocket log subscription for project {project_id}")
        await _handle_project_logs_ws(websocket, f"project_{project_id}")

    # 测试日志 WebSocket 端点
    @app.websocket("/api/spider-projects/{project_id}/test/logs/ws")
    async def subscribe_test_logs_ws(websocket: WebSocket, project_id: int):
        """WebSocket 订阅爬虫项目测试日志"""
        logger.info(f"WebSocket test log subscription for project {project_id}")
        await _handle_project_logs_ws(websocket, f"test_{project_id}")

    async def _handle_project_logs_ws(websocket: WebSocket, session_id: str):
        """通用的项目日志 WebSocket 处理器"""
        try:
            await websocket.accept()
        except Exception as e:
            logger.error(f"WebSocket accept failed: {e}")
            return

        log_queue = None

        try:
            if not log_manager.has_session(session_id):
                await websocket.send_json({"type": "error", "message": "项目未在执行中"})
                return

            log_queue = log_manager.subscribe(session_id)
            await websocket.send_json({"type": "connected", "session_id": session_id})

            # 如果会话已结束，发送历史日志后直接结束
            if log_manager.is_session_ended(session_id):
                await _drain_log_queue(websocket, log_queue)
                await websocket.send_json({"type": "end"})
                return

            # 持续转发日志
            while True:
                try:
                    entry = await asyncio.wait_for(log_queue.get(), timeout=30)
                    if entry is None:
                        await websocket.send_json({"type": "end"})
                        break
                    await websocket.send_json({
                        "type": "log",
                        "level": entry.level,
                        "message": entry.message
                    })
                except asyncio.TimeoutError:
                    await websocket.send_json({"type": "heartbeat"})

        except WebSocketDisconnect:
            logger.debug(f"WebSocket disconnected for session {session_id}")
        except Exception as e:
            logger.error(f"WebSocket error: {e}")
        finally:
            if log_queue:
                log_manager.unsubscribe(session_id, log_queue)
            try:
                await websocket.close()
            except Exception:
                pass

    async def _drain_log_queue(websocket: WebSocket, log_queue):
        """将队列中所有日志发送到 WebSocket"""
        while True:
            try:
                entry = log_queue.get_nowait()
                if entry is None:
                    break
                await websocket.send_json({
                    "type": "log",
                    "level": entry.level,
                    "message": entry.message
                })
            except asyncio.QueueEmpty:
                break

    # 挂载前端静态文件（admin-panel）
    admin_dist = Path(project_root) / "admin-panel" / "dist"
    if admin_dist.exists():
        # 挂载静态资源
        app.mount("/assets", StaticFiles(directory=str(admin_dist / "assets")), name="admin-assets")

        # Admin 管理后台入口
        @app.get("/admin")
        @app.get("/admin/{path:path}")
        async def serve_admin(path: str = ""):
            """服务 Admin SPA 前端"""
            return FileResponse(str(admin_dist / "index.html"))

        logger.info("Admin panel mounted at /admin")

    logger.info(f"Application created with {len(app.routes)} routes")

    return app


# 默认应用实例（用于uvicorn）
app = create_app()


def main():
    """主函数"""
    # 加载配置
    config_path = os.environ.get("CONFIG_PATH", "config.yaml")
    config = get_config()

    # 如果指定了配置文件，重新创建应用
    if Path(config_path).exists():
        reload_config(config_path)
        config = get_config()

    logger.info(f"Starting SEO HTML Generator on {config.server.host}:{config.server.port}")

    # 启动服务
    uvicorn.run(
        "main:app",
        host=config.server.host,
        port=config.server.port,
        reload=config.server.debug,
        workers=config.server.workers if not config.server.debug else 1,
        log_level="debug"
    )


if __name__ == "__main__":
    main()
