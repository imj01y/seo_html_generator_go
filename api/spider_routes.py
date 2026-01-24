# -*- coding: utf-8 -*-
"""
爬虫项目管理 API

提供爬虫项目的 CRUD 操作，支持多文件项目管理。
新架构采用纯 Python 风格，入口为 main() 函数。
"""

import json
import asyncio
from typing import Optional, List, Dict, Any
from datetime import datetime

from fastapi import APIRouter, HTTPException, Query, BackgroundTasks
from pydantic import BaseModel
from loguru import logger

from database.db import fetch_one, fetch_all, execute_query, insert, update, delete
from core.redis_client import get_redis_client


router = APIRouter(prefix="/api/spider-projects", tags=["爬虫项目管理"])

# 运行中的测试任务注册表（用于支持立即取消）
_running_test_tasks: Dict[int, asyncio.Task] = {}


def _format_project_row(row: dict) -> dict:
    """格式化项目行数据"""
    return {
        "id": row['id'],
        "name": row['name'],
        "description": row['description'],
        "entry_file": row['entry_file'],
        "entry_function": row['entry_function'],
        "start_url": row['start_url'],
        "config": json.loads(row['config']) if row['config'] else None,
        "concurrency": row['concurrency'],
        "output_group_id": row['output_group_id'],
        "schedule": row['schedule'],
        "enabled": row['enabled'],
        "status": row['status'],
        "last_run_at": row['last_run_at'].isoformat() if row['last_run_at'] else None,
        "last_run_duration": row['last_run_duration'],
        "last_run_items": row['last_run_items'],
        "last_error": row['last_error'],
        "total_runs": row['total_runs'],
        "total_items": row['total_items'],
        "created_at": row['created_at'].isoformat() if row['created_at'] else None,
        "updated_at": row['updated_at'].isoformat() if row['updated_at'] else None,
    }


# ============================================
# 调度规则转换
# ============================================

def schedule_json_to_cron(schedule_json: str) -> Optional[str]:
    """
    将调度 JSON 转换为 Cron 表达式

    Args:
        schedule_json: JSON 格式的调度配置

    Returns:
        Cron 表达式，如果是 none 类型则返回 None

    示例:
        {"type": "interval_minutes", "interval": 30} -> "*/30 * * * *"
        {"type": "daily", "time": "08:30"} -> "30 8 * * *"
        {"type": "weekly", "days": [1, 3, 5], "time": "09:00"} -> "0 9 * * 1,3,5"
    """
    if not schedule_json:
        return None

    try:
        config = json.loads(schedule_json)
    except json.JSONDecodeError:
        # 可能已经是 Cron 格式，直接返回
        return schedule_json if schedule_json.strip() else None

    schedule_type = config.get('type', 'none')

    if schedule_type == 'none':
        return None

    if schedule_type == 'interval_minutes':
        interval = config.get('interval', 30)
        return f"*/{interval} * * * *"

    if schedule_type == 'interval_hours':
        interval = config.get('interval', 1)
        return f"0 */{interval} * * *"

    if schedule_type == 'daily':
        time_str = config.get('time', '08:00')
        hour, minute = time_str.split(':')
        return f"{int(minute)} {int(hour)} * * *"

    if schedule_type == 'weekly':
        time_str = config.get('time', '09:00')
        hour, minute = time_str.split(':')
        days = config.get('days', [])
        if not days:
            return None
        days_str = ','.join(str(d) for d in sorted(days))
        return f"{int(minute)} {int(hour)} * * {days_str}"

    if schedule_type == 'monthly':
        time_str = config.get('time', '10:00')
        hour, minute = time_str.split(':')
        dates = config.get('dates', [])
        if not dates:
            return None
        dates_str = ','.join(str(d) for d in sorted(dates))
        return f"{int(minute)} {int(hour)} {dates_str} * *"

    return None


def schedule_json_to_description(schedule_json: str) -> str:
    """
    将调度 JSON 转换为中文描述

    Args:
        schedule_json: JSON 格式的调度配置

    Returns:
        中文描述文本
    """
    if not schedule_json:
        return "不启用调度"

    try:
        config = json.loads(schedule_json)
    except json.JSONDecodeError:
        return f"Cron: {schedule_json}"

    schedule_type = config.get('type', 'none')
    weekday_names = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

    if schedule_type == 'none':
        return "不启用调度"

    if schedule_type == 'interval_minutes':
        interval = config.get('interval', 30)
        return f"每隔 {interval} 分钟执行"

    if schedule_type == 'interval_hours':
        interval = config.get('interval', 1)
        return f"每隔 {interval} 小时执行"

    if schedule_type == 'daily':
        time_str = config.get('time', '08:00')
        return f"每天 {time_str} 执行"

    if schedule_type == 'weekly':
        time_str = config.get('time', '09:00')
        days = config.get('days', [])
        if not days:
            return "每周（未选择日期）"
        day_names = [weekday_names[d] for d in sorted(days, key=lambda x: (x == 0, x))]
        return f"每{'/'.join(day_names)} {time_str} 执行"

    if schedule_type == 'monthly':
        time_str = config.get('time', '10:00')
        dates = config.get('dates', [])
        if not dates:
            return "每月（未选择日期）"
        date_names = [f"{d}号" for d in sorted(dates)]
        return f"每月 {'/'.join(date_names)} {time_str} 执行"

    return "未知调度类型"


# ============================================
# Pydantic 模型
# ============================================

class ProjectFileCreate(BaseModel):
    """创建项目文件"""
    filename: str
    content: str


class ProjectFileUpdate(BaseModel):
    """更新项目文件"""
    content: str


class ProjectCreate(BaseModel):
    """创建项目"""
    name: str
    description: Optional[str] = None
    entry_file: str = "spider.py"
    entry_function: str = "main"
    start_url: Optional[str] = None
    config: Optional[Dict[str, Any]] = None
    concurrency: int = 3  # 并发数量
    output_group_id: int = 1
    schedule: Optional[str] = None
    enabled: int = 1
    files: Optional[List[ProjectFileCreate]] = None  # 初始文件列表


class ProjectUpdate(BaseModel):
    """更新项目"""
    name: Optional[str] = None
    description: Optional[str] = None
    entry_file: Optional[str] = None
    entry_function: Optional[str] = None
    start_url: Optional[str] = None
    config: Optional[Dict[str, Any]] = None
    concurrency: Optional[int] = None  # 并发数量
    output_group_id: Optional[int] = None
    schedule: Optional[str] = None
    enabled: Optional[int] = None


class ProjectResponse(BaseModel):
    """项目响应"""
    id: int
    name: str
    description: Optional[str]
    entry_file: str
    entry_function: str
    start_url: Optional[str]
    config: Optional[Dict[str, Any]]
    concurrency: int  # 并发数量
    output_group_id: int
    schedule: Optional[str]
    enabled: int
    status: str
    last_run_at: Optional[datetime]
    last_run_duration: Optional[int]
    last_run_items: Optional[int]
    last_error: Optional[str]
    total_runs: int
    total_items: int
    created_at: datetime
    updated_at: datetime


# ============================================
# 默认代码模板
# ============================================

DEFAULT_SPIDER_CODE = '''from loguru import logger
import requests

def main():
    """
    爬虫入口函数

    数据格式（必填字段）：
    - title: 文章标题
    - content: 文章内容

    可选字段：source_url, author, publish_date, summary, cover_image, tags

    点击右上角 [指南] 查看完整文档
    """
    # 示例：API 分页抓取
    for page in range(1, 10):
        try:
            resp = requests.get(f'https://api.example.com/list?page={page}', timeout=10)
            resp.raise_for_status()
            data = resp.json()
        except Exception as e:
            logger.error(f'请求失败: {e}')
            break

        if not data.get('list'):
            break

        for item in data['list']:
            yield {
                'title': item['title'],           # 必填
                'content': item['content'],       # 必填
                'source_url': item.get('url'),    # 可选
                'author': item.get('author'),     # 可选
            }

        logger.info(f'第 {page} 页完成')


# 本地测试（可选）
if __name__ == '__main__':
    for item in main():
        print(f"标题: {item['title']}")
'''


# ============================================
# 项目管理 API
# ============================================

@router.get("")
async def list_projects(
    status: Optional[str] = None,
    enabled: Optional[int] = None,
    search: Optional[str] = None,
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
):
    """获取项目列表"""
    conditions = []
    args = []

    if status:
        conditions.append("status = %s")
        args.append(status)
    if enabled is not None:
        conditions.append("enabled = %s")
        args.append(enabled)
    if search:
        conditions.append("(name LIKE %s OR description LIKE %s)")
        args.extend([f"%{search}%", f"%{search}%"])

    where = " AND ".join(conditions) if conditions else "1=1"

    # 获取总数
    count_sql = f"SELECT COUNT(*) as cnt FROM spider_projects WHERE {where}"
    count_row = await fetch_one(count_sql, args)
    total = count_row['cnt'] if count_row else 0

    # 获取数据
    offset = (page - 1) * page_size
    data_sql = f"""
        SELECT id, name, description, entry_file, entry_function, start_url,
               config, concurrency, output_group_id, schedule, enabled, status,
               last_run_at, last_run_duration, last_run_items, last_error,
               total_runs, total_items, created_at, updated_at
        FROM spider_projects
        WHERE {where}
        ORDER BY id DESC
        LIMIT %s OFFSET %s
    """
    rows = await fetch_all(data_sql, (*args, page_size, offset))

    return {
        "success": True,
        "data": [_format_project_row(row) for row in rows],
        "total": total,
        "page": page,
        "page_size": page_size,
    }


@router.get("/{project_id}")
async def get_project(project_id: int):
    """获取单个项目详情"""
    sql = """
        SELECT id, name, description, entry_file, entry_function, start_url,
               config, concurrency, output_group_id, schedule, enabled, status,
               last_run_at, last_run_duration, last_run_items, last_error,
               total_runs, total_items, created_at, updated_at
        FROM spider_projects
        WHERE id = %s
    """
    row = await fetch_one(sql, (project_id,))

    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    return {"success": True, "data": _format_project_row(row)}


@router.post("")
async def create_project(data: ProjectCreate):
    """创建新项目"""
    # 插入项目
    insert_data = {
        "name": data.name,
        "description": data.description,
        "entry_file": data.entry_file,
        "entry_function": data.entry_function,
        "start_url": data.start_url,
        "config": json.dumps(data.config) if data.config else None,
        "concurrency": data.concurrency,
        "output_group_id": data.output_group_id,
        "schedule": data.schedule,
        "enabled": data.enabled,
    }

    try:
        project_id = await insert("spider_projects", insert_data)

        # 创建默认入口文件
        files_to_create = data.files or []

        # 如果没有提供入口文件，创建默认的
        has_entry_file = any(f.filename == data.entry_file for f in files_to_create)
        if not has_entry_file:
            files_to_create.append(ProjectFileCreate(
                filename=data.entry_file,
                content=DEFAULT_SPIDER_CODE
            ))

        # 创建文件
        for file_data in files_to_create:
            await insert("spider_project_files", {
                "project_id": project_id,
                "filename": file_data.filename,
                "content": file_data.content,
            })

        # 通知调度器（如果有调度配置）
        if data.schedule and data.enabled:
            from core.workers.spider_scheduler import get_scheduler
            scheduler = get_scheduler()
            if scheduler:
                scheduler.update_schedule(project_id, data.schedule, data.enabled)

        logger.info(f"Created spider project: {data.name} (id={project_id})")
        return {"success": True, "id": project_id, "message": "创建成功"}

    except Exception as e:
        logger.error(f"Failed to create spider project: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.put("/{project_id}")
async def update_project(project_id: int, data: ProjectUpdate):
    """更新项目"""
    # 检查项目是否存在
    check_sql = "SELECT id, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(check_sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，无法修改")

    # 构建更新数据
    update_data = {}
    if data.name is not None:
        update_data['name'] = data.name
    if data.description is not None:
        update_data['description'] = data.description
    if data.entry_file is not None:
        update_data['entry_file'] = data.entry_file
    if data.entry_function is not None:
        update_data['entry_function'] = data.entry_function
    if data.start_url is not None:
        update_data['start_url'] = data.start_url
    if data.config is not None:
        update_data['config'] = json.dumps(data.config)
    if data.concurrency is not None:
        update_data['concurrency'] = data.concurrency
    if data.output_group_id is not None:
        update_data['output_group_id'] = data.output_group_id
    if data.schedule is not None:
        update_data['schedule'] = data.schedule
    if data.enabled is not None:
        update_data['enabled'] = data.enabled

    if not update_data:
        return {"success": True, "message": "无需更新"}

    try:
        await update("spider_projects", update_data, "id = %s", (project_id,))

        # 通知调度器更新（如果 schedule 或 enabled 有变化）
        if 'schedule' in update_data or 'enabled' in update_data:
            from core.workers.spider_scheduler import get_scheduler
            scheduler = get_scheduler()
            if scheduler:
                # 获取最新的 schedule 和 enabled 值
                new_schedule = update_data.get('schedule', data.schedule)
                new_enabled = update_data.get('enabled', data.enabled)
                # 如果 enabled 未在更新中，需要查询当前值
                if 'enabled' not in update_data:
                    check = await fetch_one("SELECT enabled FROM spider_projects WHERE id = %s", (project_id,))
                    new_enabled = check['enabled'] if check else 1
                scheduler.update_schedule(project_id, new_schedule, new_enabled)

        logger.info(f"Updated spider project: id={project_id}")
        return {"success": True, "message": "更新成功"}
    except Exception as e:
        logger.error(f"Failed to update spider project: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/{project_id}")
async def delete_project(project_id: int):
    """删除项目"""
    # 检查项目是否存在
    check_sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(check_sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，无法删除")

    try:
        # 通知调度器移除任务
        from core.workers.spider_scheduler import get_scheduler
        scheduler = get_scheduler()
        if scheduler:
            scheduler.remove_schedule(project_id)

        # 删除项目文件
        await delete("spider_project_files", "project_id = %s", (project_id,))
        # 删除项目
        await delete("spider_projects", "id = %s", (project_id,))

        logger.info(f"Deleted spider project: {row['name']} (id={project_id})")
        return {"success": True, "message": "删除成功"}
    except Exception as e:
        logger.error(f"Failed to delete spider project: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/{project_id}/toggle")
async def toggle_project(project_id: int):
    """切换项目启用状态"""
    sql = "SELECT enabled, schedule FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    new_enabled = 0 if row['enabled'] == 1 else 1
    await execute_query(
        "UPDATE spider_projects SET enabled = %s WHERE id = %s",
        (new_enabled, project_id),
        commit=True
    )

    # 通知调度器更新
    from core.workers.spider_scheduler import get_scheduler
    scheduler = get_scheduler()
    if scheduler:
        scheduler.update_schedule(project_id, row['schedule'], new_enabled)

    return {
        "success": True,
        "enabled": new_enabled,
        "message": "已启用" if new_enabled else "已禁用"
    }


# ============================================
# 项目文件管理 API
# ============================================

@router.get("/{project_id}/files")
async def list_project_files(project_id: int):
    """获取项目的所有文件"""
    # 检查项目是否存在
    check_sql = "SELECT id FROM spider_projects WHERE id = %s"
    row = await fetch_one(check_sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    sql = """
        SELECT id, filename, content, created_at, updated_at
        FROM spider_project_files
        WHERE project_id = %s
        ORDER BY filename
    """
    rows = await fetch_all(sql, (project_id,))

    files = [
        {
            "id": row['id'],
            "filename": row['filename'],
            "content": row['content'],
            "created_at": row['created_at'].isoformat() if row['created_at'] else None,
            "updated_at": row['updated_at'].isoformat() if row['updated_at'] else None,
        }
        for row in rows
    ]

    return {"success": True, "data": files}


@router.get("/{project_id}/files/{filename}")
async def get_project_file(project_id: int, filename: str):
    """获取单个文件内容"""
    sql = """
        SELECT id, filename, content, created_at, updated_at
        FROM spider_project_files
        WHERE project_id = %s AND filename = %s
    """
    row = await fetch_one(sql, (project_id, filename))

    if not row:
        raise HTTPException(status_code=404, detail="文件不存在")

    return {
        "success": True,
        "data": {
            "id": row['id'],
            "filename": row['filename'],
            "content": row['content'],
            "created_at": row['created_at'].isoformat() if row['created_at'] else None,
            "updated_at": row['updated_at'].isoformat() if row['updated_at'] else None,
        }
    }


@router.post("/{project_id}/files")
async def create_project_file(project_id: int, data: ProjectFileCreate):
    """创建新文件"""
    # 检查项目是否存在
    check_sql = "SELECT id, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(check_sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，无法添加文件")

    # 检查文件是否已存在
    file_check_sql = "SELECT id FROM spider_project_files WHERE project_id = %s AND filename = %s"
    existing = await fetch_one(file_check_sql, (project_id, data.filename))
    if existing:
        raise HTTPException(status_code=400, detail="文件已存在")

    # 验证文件名
    if not data.filename.endswith('.py'):
        raise HTTPException(status_code=400, detail="文件名必须以 .py 结尾")

    try:
        file_id = await insert("spider_project_files", {
            "project_id": project_id,
            "filename": data.filename,
            "content": data.content,
        })

        logger.info(f"Created file {data.filename} for project {project_id}")
        return {"success": True, "id": file_id, "message": "创建成功"}
    except Exception as e:
        logger.error(f"Failed to create file: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.put("/{project_id}/files/{filename}")
async def update_project_file(project_id: int, filename: str, data: ProjectFileUpdate):
    """更新文件内容"""
    # 检查项目是否存在
    check_sql = "SELECT id, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(check_sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，无法修改文件")

    # 检查文件是否存在
    file_check_sql = "SELECT id FROM spider_project_files WHERE project_id = %s AND filename = %s"
    existing = await fetch_one(file_check_sql, (project_id, filename))
    if not existing:
        raise HTTPException(status_code=404, detail="文件不存在")

    try:
        await update(
            "spider_project_files",
            {"content": data.content},
            "project_id = %s AND filename = %s",
            (project_id, filename)
        )

        logger.info(f"Updated file {filename} for project {project_id}")
        return {"success": True, "message": "更新成功"}
    except Exception as e:
        logger.error(f"Failed to update file: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/{project_id}/files/{filename}")
async def delete_project_file(project_id: int, filename: str):
    """删除文件"""
    # 检查项目是否存在
    check_sql = "SELECT id, status, entry_file FROM spider_projects WHERE id = %s"
    row = await fetch_one(check_sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，无法删除文件")

    # 不能删除入口文件
    if filename == row['entry_file']:
        raise HTTPException(status_code=400, detail="不能删除入口文件")

    # 检查文件是否存在
    file_check_sql = "SELECT id FROM spider_project_files WHERE project_id = %s AND filename = %s"
    existing = await fetch_one(file_check_sql, (project_id, filename))
    if not existing:
        raise HTTPException(status_code=404, detail="文件不存在")

    try:
        await delete("spider_project_files", "project_id = %s AND filename = %s", (project_id, filename))

        logger.info(f"Deleted file {filename} from project {project_id}")
        return {"success": True, "message": "删除成功"}
    except Exception as e:
        logger.error(f"Failed to delete file: {e}")
        raise HTTPException(status_code=500, detail=str(e))


# ============================================
# 项目执行 API
# ============================================

@router.post("/{project_id}/run")
async def run_project(project_id: int, background_tasks: BackgroundTasks):
    """运行项目"""
    from core.crawler.log_manager import log_manager

    # 检查项目是否存在
    sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中")

    # 创建日志会话
    session_id = f"project_{project_id}"
    log_manager.create_session(session_id)
    log_manager.add_log(session_id, "INFO", "任务已加入队列，等待执行...")

    # 更新状态
    await execute_query(
        "UPDATE spider_projects SET status = 'running' WHERE id = %s",
        (project_id,),
        commit=True
    )

    # 在后台执行
    background_tasks.add_task(run_project_task, project_id)

    logger.info(f"Started project: {row['name']} (id={project_id})")
    return {"success": True, "message": "任务已启动"}


@router.post("/{project_id}/test")
async def test_project(project_id: int, max_items: int = 0):
    """测试运行项目（WebSocket 实时推流）

    Args:
        project_id: 项目ID
        max_items: 最大测试条数，0 表示不限制
    """
    from core.crawler.log_manager import log_manager

    # 参数校验：0 表示不限制，否则限制在 1-10000 之间
    if max_items < 0:
        max_items = 0
    elif max_items > 10000:
        max_items = 10000

    # 检查项目是否存在
    sql = "SELECT id, name FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    # 如果已有运行中的测试任务，先取消
    if project_id in _running_test_tasks:
        old_task = _running_test_tasks[project_id]
        if not old_task.done():
            old_task.cancel()
            try:
                await asyncio.wait_for(asyncio.shield(old_task), timeout=1.0)
            except (asyncio.CancelledError, asyncio.TimeoutError):
                pass

    # 创建测试会话（使用不同的 session_id 前缀）
    session_id = f"test_{project_id}"
    log_manager.create_session(session_id)
    limit_text = f"最多 {max_items} 条" if max_items > 0 else "不限制条数"
    log_manager.add_log(session_id, "INFO", f"开始测试运行（{limit_text}）...")

    # 使用 asyncio.create_task 确保在正确的事件循环中运行
    task = asyncio.create_task(test_project_task(project_id, max_items))

    # 注册到全局任务表
    _running_test_tasks[project_id] = task

    # 任务完成时自动清理
    def cleanup(t):
        _running_test_tasks.pop(project_id, None)
    task.add_done_callback(cleanup)

    return {"success": True, "message": "测试已启动", "session_id": session_id}


@router.post("/{project_id}/test/stop")
async def stop_test_project(project_id: int):
    """停止测试运行"""
    from core.crawler.request_queue import RequestQueue
    from core.crawler.log_manager import log_manager

    # 检查项目是否存在
    sql = "SELECT id, name FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    # 设置测试队列状态为停止
    redis_cache = get_redis_client()
    if redis_cache:
        queue = RequestQueue(redis_cache, project_id, is_test=True)
        await queue.stop(clear_queue=True)

    # 立即取消正在运行的测试任务
    if project_id in _running_test_tasks:
        task = _running_test_tasks[project_id]
        if not task.done():
            task.cancel()
            logger.info(f"Cancelled test task for project {project_id}")

    # 添加停止日志
    session_id = f"test_{project_id}"
    log_manager.add_log(session_id, "INFO", "测试已手动停止")

    logger.info(f"Stopped test for project: {row['name']} (id={project_id})")
    return {"success": True, "message": "测试已停止"}


# ============================================
# 后台任务 - 辅助函数
# ============================================

def _create_log_sink(project_id: int, log_func):
    """
    创建 loguru sink 用于捕获爬虫代码中的日志。

    Args:
        project_id: 项目 ID
        log_func: 日志推送函数，签名为 (level: str, message: str) -> None

    Returns:
        tuple: (log_filter, log_sink) 用于 logger.add()
    """
    module_prefix = f"spider_project_{project_id}"

    def log_filter(record):
        """捕获当前项目模块和 crawler 核心模块的日志"""
        name = record["name"]
        file_name = record["file"].name

        # 排除 log_manager.py 避免递归死锁
        if file_name.endswith("log_manager.py"):
            return False

        # 捕获 crawler 核心模块的日志（http_client, queue_consumer 等）
        # 使用 endswith 替代 == 来匹配文件名，因为 Windows 下可能返回绝对路径
        crawler_files = ("http_client.py", "queue_consumer.py", "project_runner.py", "request_queue.py")
        return (
            name.startswith(module_prefix) or
            name.startswith("<spider_project_") or
            file_name.startswith("<spider_project_") or
            any(file_name.endswith(f) for f in crawler_files)
        )

    def log_sink(message):
        """将日志推送到 LogManager"""
        record = message.record
        msg = record["message"]

        # 如果有异常信息（logger.exception），附加堆栈跟踪
        if record["exception"] is not None:
            exc_type, exc_value, exc_tb = record["exception"]
            if exc_tb is not None:
                import traceback
                tb_lines = traceback.format_exception(exc_type, exc_value, exc_tb)
                msg = msg + "\n" + "".join(tb_lines)

        log_func(record["level"].name, msg)

    return log_filter, log_sink


async def _load_project_and_runner(project_id: int, log_func, include_output_group: bool = False, include_concurrency: bool = False, is_test: bool = False, max_items: int = 0):
    """
    加载项目配置并创建运行器。

    Args:
        project_id: 项目 ID
        log_func: 日志推送函数
        include_output_group: 是否在返回结果中包含 output_group_id
        include_concurrency: 是否包含并发配置
        is_test: 是否为测试模式（使用独立的 Redis 队列）
        max_items: 最大数据条数（0 表示不限制）

    Returns:
        tuple: (runner, row) 或 (None, None) 如果项目不存在
    """
    from core.crawler.project_loader import ProjectLoader
    from core.crawler.project_runner import ProjectRunner
    from database.db import get_db_pool

    log_func("INFO", "正在加载项目...")

    # 构建 SQL 查询
    fields = "id, name, entry_file, entry_function, config"
    if include_output_group:
        fields += ", output_group_id"
    if include_concurrency:
        fields += ", concurrency"

    sql = f"SELECT {fields} FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))

    if not row:
        log_func("ERROR", "项目不存在")
        return None, None

    config = json.loads(row['config']) if row['config'] else {}

    # 加载项目文件
    loader = ProjectLoader(project_id)
    modules = await loader.load()
    log_func("INFO", f"已加载 {len(modules)} 个模块")

    # 获取 Redis 和数据库连接
    redis_cache = get_redis_client()
    db_pool = get_db_pool()
    concurrency = row.get('concurrency', 3) if include_concurrency else 3

    # 创建运行器
    runner = ProjectRunner(
        project_id=project_id,
        modules=modules,
        config=config,
        redis=redis_cache if redis_cache else None,
        db_pool=db_pool,
        concurrency=concurrency,
        is_test=is_test,
        max_items=max_items,
    )

    return runner, row


# ============================================
# 后台任务
# ============================================

async def run_project_task(project_id: int):
    """后台执行项目任务"""
    from core.crawler.log_manager import log_manager
    from core.logging import get_log_manager
    import time

    session_id = f"project_{project_id}"
    log_handler_id = None

    def _log(level: str, message: str):
        log_manager.add_log(session_id, level, message)

    start_time = time.time()
    items_count = 0

    try:
        runner, row = await _load_project_and_runner(project_id, _log, include_output_group=True, include_concurrency=True)
        if not runner:
            return

        # 设置日志上下文
        log_mgr = get_log_manager()
        log_mgr.set_context(spider_project_id=project_id)

        # 添加 loguru sink 捕获爬虫代码中的日志
        log_filter, log_sink = _create_log_sink(project_id, _log)
        log_handler_id = logger.add(
            log_sink,
            filter=log_filter,
            format="{message}",
            level="DEBUG",
        )

        # 【新增】清除旧的停止信号，防止重新运行时立即终止
        redis_cache = get_redis_client()
        if redis_cache:
            stop_key = f"spider_project:{project_id}:stop"
            await redis_cache.delete(stop_key)

        _log("INFO", "开始执行 Spider...")

        # 执行并收集数据
        group_id = row['output_group_id']

        async for item in runner.run():
            # 检查停止信号
            redis_cache = get_redis_client()
            if redis_cache:
                stop_key = f"spider_project:{project_id}:stop"
                if await redis_cache.get(stop_key):
                    _log("INFO", "收到停止信号，任务终止")
                    await redis_cache.delete(stop_key)
                    break

            # 根据 type 字段路由到不同的表
            item_type = item.get('type', 'article')

            try:
                if item_type == 'keywords':
                    # ========== 写入关键词表 ==========
                    keywords = item.get('keywords', [])
                    target_group = item.get('group_id', group_id)

                    if keywords:
                        from core.keyword_group_manager import get_keyword_group
                        keyword_manager = get_keyword_group()
                        if keyword_manager:
                            result = await keyword_manager.add_keywords_batch(keywords, target_group)
                            items_count += result.get('added', 0)
                            _log("INFO", f"关键词写入: 新增 {result['added']}, 跳过 {result['skipped']}")
                        else:
                            _log("WARNING", "关键词管理器未初始化，跳过写入")
                    else:
                        _log("WARNING", "keywords 字段为空，已跳过")

                elif item_type == 'images':
                    # ========== 写入图片表 ==========
                    urls = item.get('urls', [])
                    target_group = item.get('group_id', group_id)

                    if urls:
                        from core.image_group_manager import get_image_group
                        image_manager = get_image_group()
                        if image_manager:
                            result = await image_manager.add_urls_batch(urls, target_group)
                            items_count += result.get('added', 0)
                            _log("INFO", f"图片写入: 新增 {result['added']}, 跳过 {result['skipped']}")
                        else:
                            _log("WARNING", "图片管理器未初始化，跳过写入")
                    else:
                        _log("WARNING", "urls 字段为空，已跳过")

                else:
                    # ========== 默认写入文章表 ==========
                    # 验证文章数据格式
                    if not item.get('title') or not item.get('content'):
                        _log("WARNING", f"数据缺少必填字段，已跳过: {item.get('title', '(无标题)')[:30]}")
                        continue

                    # 支持 group_id 覆盖
                    target_group = item.get('group_id', group_id)

                    article_id = await insert("original_articles", {
                        "group_id": target_group,
                        "source_id": project_id,  # 关联爬虫项目ID，便于追溯数据来源
                        "source_url": item.get('source_url'),
                        "title": item['title'][:500],
                        "content": item['content'],
                    })
                    items_count += 1

                    # 推送到待处理队列，供 GeneratorWorker 处理
                    if redis_cache and article_id:
                        try:
                            queue_key = f"pending:articles:{target_group}"
                            await redis_cache.lpush(queue_key, article_id)
                        except Exception as queue_err:
                            _log("WARNING", f"推送到待处理队列失败: {queue_err}")

                if items_count % 10 == 0:
                    _log("INFO", f"已抓取 {items_count} 条数据")

            except Exception as e:
                if 'Duplicate' in str(e):
                    _log("WARNING", f"数据重复，已跳过: {str(item)[:50]}")
                else:
                    _log("ERROR", f"保存数据失败: {e}")

        # 更新统计
        duration = int(time.time() - start_time)
        await execute_query(
            """
            UPDATE spider_projects SET
                status = 'idle',
                last_run_at = NOW(),
                last_run_duration = %s,
                last_run_items = %s,
                last_error = NULL,
                total_runs = total_runs + 1,
                total_items = total_items + %s
            WHERE id = %s
            """,
            (duration, items_count, items_count, project_id),
            commit=True
        )

        _log("INFO", f"任务完成：共 {items_count} 条数据，耗时 {duration} 秒")

    except Exception as e:
        logger.error(f"Project task failed: {e}")
        _log("ERROR", f"任务异常: {str(e)}")

        await execute_query(
            "UPDATE spider_projects SET status = 'error', last_error = %s WHERE id = %s",
            (str(e), project_id),
            commit=True
        )

    finally:
        if log_handler_id is not None:
            try:
                logger.remove(log_handler_id)
            except ValueError:
                pass

        from core.logging import get_log_manager
        log_mgr = get_log_manager()
        log_mgr.clear_context()

        log_manager.close_session(session_id)


async def test_project_task(project_id: int, max_items: int = 0):
    """后台执行测试任务

    Args:
        project_id: 项目ID
        max_items: 最大测试条数，0 表示不限制
    """
    from core.crawler.log_manager import log_manager
    from core.crawler.request_queue import RequestQueue
    import time

    session_id = f"test_{project_id}"
    log_handler_id = None

    def _log(level: str, message: str):
        log_manager.add_log(session_id, level, message)

    start_time = time.time()
    items_count = 0

    try:
        # 测试模式：先清除之前的测试请求队列和去重缓存（使用独立的 test_spider 前缀）
        redis_cache = get_redis_client()
        if redis_cache:
            queue = RequestQueue(redis_cache, project_id, is_test=True)
            await queue.clear()
            _log("DEBUG", "已清除测试队列缓存")

        runner, row = await _load_project_and_runner(project_id, _log, is_test=True, max_items=max_items, include_concurrency=True)
        if not runner:
            return

        # 添加 loguru sink 捕获爬虫代码中的日志
        log_filter, log_sink = _create_log_sink(project_id, _log)
        log_handler_id = logger.add(
            log_sink,
            filter=log_filter,
            format="{message}",
            level="DEBUG",
        )

        limit_text = f"最多 {max_items} 条" if max_items > 0 else "不限制条数"
        _log("INFO", f"开始执行 Spider（测试模式，{limit_text}）...")

        # 执行并收集数据（测试模式不保存到数据库）
        # 限制已在 QueueConsumer 层实现，这里只需收集输出
        async for item in runner.run():
            # 检查停止信号（复用前面创建的 queue 变量）
            if redis_cache:
                state = await queue.get_state()
                if state == RequestQueue.STATE_STOPPED:
                    _log("INFO", "测试完成或已停止")
                    break

            if not item.get('title') or not item.get('content'):
                _log("WARNING", f"数据缺少必填字段，已跳过: {item.get('title', '(无标题)')[:30]}")
                continue

            items_count += 1
            _log("ITEM", json.dumps(item, ensure_ascii=False))
            if max_items > 0:
                _log("INFO", f"[{items_count}/{max_items}] {item['title'][:50]}")
            else:
                _log("INFO", f"[{items_count}] {item['title'][:50]}")

        duration = round(time.time() - start_time, 2)
        _log("INFO", f"测试完成：共 {items_count} 条数据，耗时 {duration} 秒")

    except asyncio.CancelledError:
        # 任务被取消（用户点击停止按钮）
        duration = round(time.time() - start_time, 2)
        _log("INFO", f"测试已手动停止：共 {items_count} 条数据，耗时 {duration} 秒")
        logger.info(f"Test task cancelled for project {project_id}")
        # 不重新抛出，让任务正常结束

    except Exception as e:
        logger.error(f"Test project task failed: {e}")
        _log("ERROR", f"测试异常: {str(e)}")

    finally:
        if log_handler_id is not None:
            try:
                logger.remove(log_handler_id)
            except ValueError:
                pass

        log_manager.close_session(session_id)


# ============================================
# 工具 API
# ============================================

# ============================================
# 任务控制 API（队列模式）
# ============================================

@router.post("/{project_id}/pause")
async def pause_project(project_id: int):
    """暂停项目（保留队列）"""
    from core.crawler.request_queue import RequestQueue

    # 检查项目是否存在
    sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] != 'running':
        return {"success": True, "message": "项目未在运行"}

    # 设置队列状态为暂停
    redis_cache = get_redis_client()
    if redis_cache:
        queue = RequestQueue(redis_cache, project_id)
        await queue.pause()

    logger.info(f"Paused project: {row['name']} (id={project_id})")
    return {"success": True, "message": "已暂停"}


@router.post("/{project_id}/resume")
async def resume_project(project_id: int):
    """恢复项目"""
    from core.crawler.request_queue import RequestQueue

    # 检查项目是否存在
    sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    # 恢复队列
    redis_cache = get_redis_client()
    if redis_cache:
        queue = RequestQueue(redis_cache, project_id)
        await queue.resume()

    logger.info(f"Resumed project: {row['name']} (id={project_id})")
    return {"success": True, "message": "已恢复"}


@router.post("/{project_id}/stop")
async def stop_project(project_id: int, clear_queue: bool = False):
    """
    停止项目

    Args:
        project_id: 项目ID
        clear_queue: 是否清空队列数据
    """
    from core.crawler.request_queue import RequestQueue

    sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    # 设置停止信号
    redis_cache = get_redis_client()
    if redis_cache:
        # 旧方式：设置停止键
        stop_key = f"spider_project:{project_id}:stop"
        await redis_cache.set(stop_key, "1", ex=3600)

        # 新方式：设置队列状态
        queue = RequestQueue(redis_cache, project_id)
        await queue.stop(clear_queue=clear_queue)

    # 更新数据库状态
    await execute_query(
        "UPDATE spider_projects SET status = 'idle', last_error = %s WHERE id = %s",
        ("用户手动停止" + ("（已清空队列）" if clear_queue else ""), project_id),
        commit=True
    )

    logger.info(f"Stopped project: {row['name']} (id={project_id}), clear_queue={clear_queue}")
    return {"success": True, "message": "已停止" + ("并清空队列" if clear_queue else "")}


# ============================================
# 实时统计 API
# ============================================

@router.get("/{project_id}/stats/realtime")
async def get_realtime_stats(project_id: int):
    """获取实时统计数据（从 Redis）"""
    from core.crawler.request_queue import RequestQueue

    # 检查项目是否存在
    sql = "SELECT id, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    redis_cache = get_redis_client()
    if not redis_cache:
        return {
            "success": True,
            "data": {
                "status": row['status'],
                "total": 0,
                "completed": 0,
                "failed": 0,
                "retried": 0,
                "pending": 0,
                "processing": 0,
                "success_rate": 0,
            }
        }

    queue = RequestQueue(redis_cache, project_id)
    stats = await queue.get_stats()
    state = await queue.get_state()

    # 如果数据库状态是 running 但队列状态是 completed，更新数据库
    if row['status'] == 'running' and state == 'completed':
        await execute_query(
            "UPDATE spider_projects SET status = 'idle' WHERE id = %s",
            (project_id,),
            commit=True
        )

    return {
        "success": True,
        "data": {
            "status": state if state != 'idle' else row['status'],
            **stats.to_dict(),
        }
    }


# ============================================
# 历史统计 API（图表数据）
# ============================================

@router.get("/{project_id}/stats/chart")
async def get_chart_stats(
    project_id: int,
    period: str = Query("hour", pattern="^(minute|hour|day|month)$"),
    start: Optional[str] = None,
    end: Optional[str] = None,
    limit: int = Query(100, ge=1, le=1000),
):
    """
    获取历史统计数据（用于图表）

    Args:
        project_id: 项目ID
        period: 周期类型（minute/hour/day/month）
        start: 开始时间（ISO 格式）
        end: 结束时间（ISO 格式）
        limit: 最大记录数
    """
    from datetime import datetime
    from core.workers.stats_worker import SpiderStatsWorker
    from database.db import get_db_pool

    # 检查项目是否存在
    sql = "SELECT id FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    redis_cache = get_redis_client()
    db_pool = get_db_pool()

    if not redis_cache or not db_pool:
        return {"success": True, "data": []}

    # 解析时间
    start_time = datetime.fromisoformat(start) if start else None
    end_time = datetime.fromisoformat(end) if end else None

    # 获取数据
    worker = SpiderStatsWorker(db_pool, redis_cache)
    data = await worker.get_chart_data(
        project_id=project_id,
        period_type=period,
        start=start_time,
        end=end_time,
        limit=limit,
    )

    return {"success": True, "data": data}


# ============================================
# 队列管理 API
# ============================================

@router.post("/{project_id}/queue/clear")
async def clear_queue(project_id: int):
    """清空项目队列"""
    from core.crawler.request_queue import RequestQueue

    # 检查项目是否存在
    sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，请先停止")

    redis_cache = get_redis_client()
    if redis_cache:
        queue = RequestQueue(redis_cache, project_id)
        await queue.clear()

    logger.info(f"Cleared queue for project: {row['name']} (id={project_id})")
    return {"success": True, "message": "队列已清空"}


@router.post("/{project_id}/reset")
async def reset_project(project_id: int):
    """重置项目 - 清空所有队列数据和失败请求记录"""
    from core.crawler.request_queue import RequestQueue
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    # 检查项目是否存在
    sql = "SELECT id, name, status FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    if row['status'] == 'running':
        raise HTTPException(status_code=400, detail="项目正在运行中，请先停止")

    # 清空 Redis 队列
    redis_cache = get_redis_client()
    if redis_cache:
        queue = RequestQueue(redis_cache, project_id)
        await queue.clear()

    # 清空失败请求记录
    failed_count = 0
    db_pool = get_db_pool()
    if db_pool:
        manager = FailedRequestManager(db_pool)
        failed_count = await manager.delete_by_project(project_id)

    # 清理日志会话
    from core.crawler.log_manager import log_manager
    for session_id in [f"project_{project_id}", f"test_{project_id}"]:
        log_manager.force_cleanup_session(session_id)

    logger.info(f"Reset project: {row['name']} (id={project_id}), cleared {failed_count} failed requests")
    return {
        "success": True,
        "message": f"项目已重置，清空了 {failed_count} 条失败记录"
    }


# ============================================
# 失败请求管理 API
# ============================================

@router.get("/{project_id}/failed")
async def list_failed_requests(
    project_id: int,
    page: int = Query(1, ge=1),
    page_size: int = Query(20, ge=1, le=100),
    status: Optional[str] = Query(None, pattern="^(pending|retried|ignored)$"),
):
    """获取失败请求列表"""
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    # 检查项目是否存在
    sql = "SELECT id FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    db_pool = get_db_pool()
    if not db_pool:
        return {"success": True, "data": [], "total": 0}

    manager = FailedRequestManager(db_pool)
    result = await manager.list(
        project_id=project_id,
        page=page,
        page_size=page_size,
        status=status,
    )

    return {"success": True, **result}


@router.get("/{project_id}/failed/stats")
async def get_failed_stats(project_id: int):
    """获取失败请求统计"""
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    # 检查项目是否存在
    sql = "SELECT id FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    db_pool = get_db_pool()
    if not db_pool:
        return {"success": True, "data": {"pending": 0, "retried": 0, "ignored": 0, "total": 0}}

    manager = FailedRequestManager(db_pool)
    stats = await manager.get_stats(project_id)

    return {"success": True, "data": stats}


@router.post("/{project_id}/failed/retry-all")
async def retry_all_failed(project_id: int):
    """重试所有失败请求"""
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    # 检查项目是否存在
    sql = "SELECT id, name FROM spider_projects WHERE id = %s"
    row = await fetch_one(sql, (project_id,))
    if not row:
        raise HTTPException(status_code=404, detail="项目不存在")

    redis_cache = get_redis_client()
    db_pool = get_db_pool()

    if not redis_cache or not db_pool:
        raise HTTPException(status_code=500, detail="Redis 或数据库未初始化")

    manager = FailedRequestManager(db_pool)
    count = await manager.retry_all(project_id, redis_cache)

    logger.info(f"Retried {count} failed requests for project {project_id}")
    return {"success": True, "message": f"已重试 {count} 个失败请求", "count": count}


@router.post("/{project_id}/failed/{failed_id}/retry")
async def retry_one_failed(project_id: int, failed_id: int):
    """重试单个失败请求"""
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    redis_cache = get_redis_client()
    db_pool = get_db_pool()

    if not redis_cache or not db_pool:
        raise HTTPException(status_code=500, detail="Redis 或数据库未初始化")

    manager = FailedRequestManager(db_pool)
    success = await manager.retry_one(failed_id, redis_cache)

    if not success:
        raise HTTPException(status_code=404, detail="失败请求不存在或状态不正确")

    return {"success": True, "message": "已重试"}


@router.post("/{project_id}/failed/{failed_id}/ignore")
async def ignore_failed(project_id: int, failed_id: int):
    """忽略失败请求"""
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    db_pool = get_db_pool()
    if not db_pool:
        raise HTTPException(status_code=500, detail="数据库未初始化")

    manager = FailedRequestManager(db_pool)
    success = await manager.ignore(failed_id)

    if not success:
        raise HTTPException(status_code=404, detail="失败请求不存在")

    return {"success": True, "message": "已忽略"}


@router.delete("/{project_id}/failed/{failed_id}")
async def delete_failed(project_id: int, failed_id: int):
    """删除失败请求记录"""
    from core.crawler.failed_manager import FailedRequestManager
    from database.db import get_db_pool

    db_pool = get_db_pool()
    if not db_pool:
        raise HTTPException(status_code=500, detail="数据库未初始化")

    manager = FailedRequestManager(db_pool)
    success = await manager.delete(failed_id)

    if not success:
        raise HTTPException(status_code=404, detail="失败请求不存在")

    return {"success": True, "message": "已删除"}


# ============================================
# 工具 API
# ============================================

@router.get("/templates")
async def get_code_templates():
    """获取代码模板列表"""
    templates = [
        {
            "name": "api_pagination",
            "display_name": "API 分页抓取",
            "description": "适用于 JSON API 接口的分页抓取",
            "code": DEFAULT_SPIDER_CODE,
        },
        {
            "name": "html_list_detail",
            "display_name": "HTML 列表+详情",
            "description": "适用于 HTML 页面的列表页和详情页抓取",
            "code": '''from loguru import logger
import requests
from parsel import Selector

def main():
    """HTML 列表页 + 详情页抓取"""
    # 抓取列表页
    resp = requests.get('https://example.com/list', timeout=10)
    sel = Selector(resp.text)

    for article in sel.css('.article-item'):
        url = article.css('a::attr(href)').get()
        if not url:
            continue

        # 抓取详情页
        try:
            detail_resp = requests.get(url, timeout=10)
            detail_sel = Selector(detail_resp.text)

            yield {
                'title': detail_sel.css('h1::text').get(),
                'content': detail_sel.css('.content').get(),
                'author': detail_sel.css('.author::text').get(),
                'source_url': url,
            }

            logger.info(f'已抓取: {url}')

        except Exception as e:
            logger.error(f'抓取详情失败: {url} - {e}')


if __name__ == '__main__':
    for item in main():
        print(f"标题: {item['title']}")
''',
        },
        {
            "name": "multi_file",
            "display_name": "多文件项目示例",
            "description": "展示如何组织多文件项目",
            "code": '''# spider.py - 主入口
from loguru import logger
from utils import clean_html
from parsers import ArticleParser

def main():
    """多文件项目示例"""
    import requests

    resp = requests.get('https://example.com', timeout=10)
    parser = ArticleParser(resp.text)

    for article in parser.get_articles():
        yield {
            'title': article['title'],
            'content': clean_html(article['content']),
        }
        logger.info(f"已抓取: {article['title']}")


if __name__ == '__main__':
    for item in main():
        print(item['title'])
''',
            "extra_files": [
                {
                    "filename": "utils.py",
                    "content": '''# utils.py - 工具函数
import re

def clean_html(html):
    """清理 HTML 标签"""
    if not html:
        return ''
    return re.sub(r'<[^>]+>', '', html)
''',
                },
                {
                    "filename": "parsers.py",
                    "content": '''# parsers.py - 解析器
from parsel import Selector

class ArticleParser:
    def __init__(self, html):
        self.sel = Selector(html)

    def get_articles(self):
        for item in self.sel.css('.article'):
            yield {
                'title': item.css('h1::text').get(),
                'content': item.css('.content').get(),
            }
''',
                },
            ],
        },
        {
            "name": "keyword_crawler",
            "display_name": "关键词爬虫",
            "description": "抓取百度下拉词、相关搜索等关键词数据",
            "code": '''from loguru import logger
import httpx
import json
import asyncio

async def fetch_baidu_suggestions(keyword: str) -> list:
    """抓取百度下拉词"""
    url = f"https://suggestion.baidu.com/su?wd={keyword}&cb=window.baidu.sug"
    async with httpx.AsyncClient(timeout=10) as client:
        try:
            resp = await client.get(url, headers={
                "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/131.0"
            })
            text = resp.text
            json_str = text[text.find("(")+1:text.rfind(")")]
            data = json.loads(json_str)
            return data.get("s", [])
        except Exception as e:
            logger.error(f"抓取失败 {keyword}: {e}")
            return []

async def main_async():
    """异步入口"""
    # 种子关键词列表（根据需要修改）
    seed_keywords = ["SEO优化", "网站建设", "Python爬虫"]

    for seed in seed_keywords:
        logger.info(f"正在抓取种子词: {seed}")
        suggestions = await fetch_baidu_suggestions(seed)

        if suggestions:
            # yield 关键词数据，框架自动写入 keywords 表
            yield {
                "type": "keywords",
                "keywords": suggestions,
                "group_id": 1  # 目标分组ID
            }
            logger.info(f"[{seed}] 获取 {len(suggestions)} 个下拉词")

        await asyncio.sleep(1)  # 请求间隔

def main():
    """同步入口（框架调用）"""
    import asyncio
    async def collect():
        items = []
        async for item in main_async():
            items.append(item)
        return items
    return asyncio.run(collect())


if __name__ == "__main__":
    import asyncio
    asyncio.run(main_async())
''',
        },
        {
            "name": "image_crawler",
            "display_name": "图片URL爬虫",
            "description": "从网页抓取图片URL并写入图片库",
            "code": '''from loguru import logger
import httpx
import asyncio
from parsel import Selector

async def fetch_images(url: str) -> list:
    """从页面抓取图片URL"""
    async with httpx.AsyncClient(timeout=15) as client:
        try:
            resp = await client.get(url, headers={
                "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/131.0"
            })
            sel = Selector(resp.text)
            images = []
            for img in sel.css("img"):
                src = img.attrib.get("src") or img.attrib.get("data-src")
                if src and src.startswith("http"):
                    # 过滤小图标
                    if not any(x in src for x in ["icon", "logo", "avatar", "1x1"]):
                        images.append(src)
            return list(set(images))  # 去重
        except Exception as e:
            logger.error(f"抓取失败 {url}: {e}")
            return []

async def main_async():
    """异步入口"""
    # 目标页面列表（根据需要修改）
    target_urls = [
        "https://example.com/gallery1",
        "https://example.com/gallery2",
    ]

    for url in target_urls:
        logger.info(f"正在抓取: {url}")
        images = await fetch_images(url)

        if images:
            # yield 图片数据，框架自动写入 images 表
            yield {
                "type": "images",
                "urls": images,
                "group_id": 1  # 目标分组ID
            }
            logger.info(f"获取 {len(images)} 张图片")

        await asyncio.sleep(2)  # 请求间隔

def main():
    """同步入口（框架调用）"""
    import asyncio
    async def collect():
        items = []
        async for item in main_async():
            items.append(item)
        return items
    return asyncio.run(collect())


if __name__ == "__main__":
    import asyncio
    asyncio.run(main_async())
''',
        },
    ]

    return {"success": True, "data": templates}


# ============================================
# 爬虫统计 API（独立路由）
# ============================================

stats_router = APIRouter(prefix="/api/spider-stats", tags=["爬虫统计"])


@stats_router.get("/overview")
async def get_stats_overview(
    project_id: Optional[int] = Query(None, description="项目ID，不传或0表示全部项目"),
    period: str = Query("day", pattern="^(minute|hour|day|month)$"),
    start: Optional[str] = Query(None, description="开始时间（ISO格式）"),
    end: Optional[str] = Query(None, description="结束时间（ISO格式）"),
):
    """
    获取统计概览

    Args:
        project_id: 项目ID，0或不传表示全部项目
        period: 周期类型（minute/hour/day/month）
        start: 开始时间（ISO 格式）
        end: 结束时间（ISO 格式）
    """
    from core.workers.stats_worker import SpiderStatsWorker
    from database.db import get_db_pool

    redis_cache = get_redis_client()
    db_pool = get_db_pool()

    if not redis_cache or not db_pool:
        return {
            "success": True,
            "data": {
                "total": 0,
                "completed": 0,
                "failed": 0,
                "retried": 0,
                "success_rate": 0,
                "avg_speed": 0,
            }
        }

    # 解析时间
    start_time = datetime.fromisoformat(start) if start else None
    end_time = datetime.fromisoformat(end) if end else None

    # project_id=0 视为全部项目
    pid = None if (project_id is None or project_id == 0) else project_id

    worker = SpiderStatsWorker(db_pool, redis_cache)
    data = await worker.get_overview(
        project_id=pid,
        period_type=period,
        start=start_time,
        end=end_time,
    )

    return {"success": True, "data": data}


@stats_router.get("/chart")
async def get_stats_chart(
    project_id: Optional[int] = Query(None, description="项目ID，不传或0表示全部项目"),
    period: str = Query("hour", pattern="^(minute|hour|day|month)$"),
    start: Optional[str] = Query(None, description="开始时间（ISO格式）"),
    end: Optional[str] = Query(None, description="结束时间（ISO格式）"),
    limit: int = Query(100, ge=1, le=1000),
):
    """
    获取图表数据

    Args:
        project_id: 项目ID，0或不传表示全部项目
        period: 周期类型（minute/hour/day/month）
        start: 开始时间（ISO 格式）
        end: 结束时间（ISO 格式）
        limit: 最大记录数
    """
    from core.workers.stats_worker import SpiderStatsWorker
    from database.db import get_db_pool

    redis_cache = get_redis_client()
    db_pool = get_db_pool()

    if not redis_cache or not db_pool:
        return {"success": True, "data": []}

    # 解析时间
    start_time = datetime.fromisoformat(start) if start else None
    end_time = datetime.fromisoformat(end) if end else None

    worker = SpiderStatsWorker(db_pool, redis_cache)

    # project_id=0 或不传表示全部项目
    if project_id is None or project_id == 0:
        data = await worker.get_all_projects_chart_data(
            period_type=period,
            start=start_time,
            end=end_time,
            limit=limit,
        )
    else:
        data = await worker.get_chart_data(
            project_id=project_id,
            period_type=period,
            start=start_time,
            end=end_time,
            limit=limit,
        )

    return {"success": True, "data": data}


@stats_router.get("/scheduled")
async def get_scheduled_projects():
    """
    获取所有已调度的爬虫项目

    Returns:
        已调度项目列表，包含下次执行时间等信息
    """
    from core.workers.spider_scheduler import get_scheduler

    scheduler = get_scheduler()
    if not scheduler:
        return {"success": True, "data": [], "message": "调度器未启动"}

    jobs = scheduler.get_scheduled_projects()
    return {"success": True, "data": jobs}


@stats_router.get("/by-project")
async def get_stats_by_project(
    period: str = Query("day", pattern="^(minute|hour|day|month)$"),
    start: Optional[str] = Query(None, description="开始时间（ISO格式）"),
    end: Optional[str] = Query(None, description="结束时间（ISO格式）"),
):
    """
    获取各项目统计明细

    Args:
        period: 周期类型（minute/hour/day/month）
        start: 开始时间（ISO 格式）
        end: 结束时间（ISO 格式）
    """
    from core.workers.stats_worker import SpiderStatsWorker
    from database.db import get_db_pool

    redis_cache = get_redis_client()
    db_pool = get_db_pool()

    if not redis_cache or not db_pool:
        return {"success": True, "data": []}

    # 解析时间
    start_time = datetime.fromisoformat(start) if start else None
    end_time = datetime.fromisoformat(end) if end else None

    worker = SpiderStatsWorker(db_pool, redis_cache)
    data = await worker.get_stats_by_project(
        period_type=period,
        start=start_time,
        end=end_time,
    )

    return {"success": True, "data": data}
