# -*- coding: utf-8 -*-
"""
系统日志 API

提供日志的 WebSocket 实时推送和历史查询功能。
"""

from typing import Optional, List
from datetime import datetime

from fastapi import APIRouter, Query
from pydantic import BaseModel
from loguru import logger

from database.db import fetch_all, fetch_one
from core.logging import get_log_manager

router = APIRouter(prefix="/api/logs", tags=["系统日志"])


# ============================================
# Pydantic 模型
# ============================================

class LogEntry(BaseModel):
    """日志条目"""
    id: int
    level: str
    module: Optional[str]
    spider_project_id: Optional[int]
    message: str
    extra: Optional[dict]
    created_at: datetime


class LogHistoryResponse(BaseModel):
    """日志历史响应"""
    success: bool
    logs: List[LogEntry]
    total: int
    page: int
    page_size: int


# ============================================
# HTTP API
# ============================================

@router.get("/history")
async def get_log_history(
    level: Optional[str] = Query(None, description="日志级别过滤"),
    module: Optional[str] = Query(None, description="模块名称过滤"),
    spider_project_id: Optional[int] = Query(None, description="爬虫项目ID过滤"),
    search: Optional[str] = Query(None, description="日志内容搜索"),
    page: int = Query(1, ge=1, description="页码"),
    page_size: int = Query(100, ge=1, le=1000, description="每页数量"),
):
    """
    查询历史日志

    支持按级别、模块、爬虫项目ID过滤，以及日志内容搜索。
    """
    conditions = []
    args = []

    if level:
        conditions.append("level = %s")
        args.append(level)
    if module:
        conditions.append("module LIKE %s")
        args.append(f"%{module}%")
    if spider_project_id:
        conditions.append("spider_project_id = %s")
        args.append(spider_project_id)
    if search:
        conditions.append("message LIKE %s")
        args.append(f"%{search}%")

    where = " AND ".join(conditions) if conditions else "1=1"

    # 获取总数
    count_sql = f"SELECT COUNT(*) as cnt FROM system_logs WHERE {where}"
    count_row = await fetch_one(count_sql, args)
    total = count_row['cnt'] if count_row else 0

    # 获取数据
    offset = (page - 1) * page_size
    data_sql = f"""
        SELECT id, level, module, spider_project_id, message, extra, created_at
        FROM system_logs
        WHERE {where}
        ORDER BY created_at DESC
        LIMIT %s OFFSET %s
    """
    rows = await fetch_all(data_sql, (*args, page_size, offset))

    logs = [
        {
            "id": row['id'],
            "level": row['level'],
            "module": row['module'],
            "spider_project_id": row['spider_project_id'],
            "message": row['message'],
            "extra": row['extra'],
            "created_at": row['created_at'].isoformat() if row['created_at'] else None,
        }
        for row in rows
    ]

    return {
        "success": True,
        "logs": logs,
        "total": total,
        "page": page,
        "page_size": page_size,
    }


@router.get("/stats")
async def get_log_stats():
    """
    获取日志统计信息
    """
    # 各级别日志数量
    level_sql = """
        SELECT level, COUNT(*) as cnt
        FROM system_logs
        GROUP BY level
    """
    level_rows = await fetch_all(level_sql, [])
    level_stats = {row['level']: row['cnt'] for row in level_rows}

    # 今日日志数量
    today_sql = """
        SELECT COUNT(*) as cnt
        FROM system_logs
        WHERE DATE(created_at) = CURDATE()
    """
    today_row = await fetch_one(today_sql, [])
    today_count = today_row['cnt'] if today_row else 0

    # 最近错误
    recent_errors_sql = """
        SELECT id, module, message, created_at
        FROM system_logs
        WHERE level IN ('ERROR', 'CRITICAL')
        ORDER BY created_at DESC
        LIMIT 10
    """
    recent_errors = await fetch_all(recent_errors_sql, [])

    return {
        "success": True,
        "data": {
            "level_stats": level_stats,
            "today_count": today_count,
            "recent_errors": recent_errors,
            "websocket_clients": get_log_manager().websocket_count,
        }
    }


@router.delete("/clear")
async def clear_old_logs(
    days: int = Query(30, ge=1, le=365, description="保留天数"),
):
    """
    清理旧日志

    删除指定天数之前的日志。
    """
    from database.db import execute_query

    sql = "DELETE FROM system_logs WHERE created_at < DATE_SUB(NOW(), INTERVAL %s DAY)"
    deleted = await execute_query(sql, (days,), commit=True)

    logger.info(f"Cleared {deleted} old logs (older than {days} days)")

    return {
        "success": True,
        "deleted": deleted,
        "message": f"已清理 {deleted} 条 {days} 天前的日志"
    }


# ============================================
# WebSocket API（在 main.py 中注册）
# ============================================
# WebSocket 端点需要在 main.py 中注册，因为需要访问 app 实例
