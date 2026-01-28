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

from config import reload_config, get_config
from core.lifecycle import lifespan
# 核心路由（页面服务）
from api.routes import router as page_router
# 拆分后的 API 路由模块
from api.auth_routes import router as auth_router
from api.dashboard_routes import router as dashboard_router
from api.site_routes import router as site_router
from api.template_routes import router as template_router
from api.keyword_routes import router as keyword_router
from api.image_routes import router as image_router
from api.article_routes import router as article_router
from api.cache_routes import router as cache_router
from api.settings_routes import router as settings_router
# 其他已有路由
from api.generator_routes import router as generator_router
from api.log_routes import router as log_router
from api.spider_routes import router as spider_router, stats_router

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
    # 核心页面服务路由
    app.include_router(page_router)
    # 拆分后的 API 路由模块
    app.include_router(auth_router)
    app.include_router(dashboard_router)
    app.include_router(site_router)
    app.include_router(template_router)
    app.include_router(keyword_router)
    app.include_router(image_router)
    app.include_router(article_router)
    app.include_router(cache_router)
    app.include_router(settings_router)
    # 其他已有路由
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
