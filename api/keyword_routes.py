# -*- coding: utf-8 -*-
"""
关键词管理 API 路由

包含关键词分组管理、关键词 CRUD、批量操作、上传、缓存管理等功能。
"""
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query, UploadFile, File, Form
from loguru import logger

from api.deps import verify_token, verify_token_or_api_token, get_keyword_group_dep
from api.schemas import (
    GroupCreate, GroupUpdate,
    KeywordCreate, KeywordBatchCreate,
    BatchIds, BatchStatusUpdate, BatchMoveGroup, DeleteAllRequest
)
from core.keyword_group_manager import AsyncKeywordGroupManager
from database.db import fetch_one, fetch_all, fetch_value, execute_query, insert

router = APIRouter(prefix="/api/keywords", tags=["关键词管理"])


# ============================================
# 关键词分组管理API
# ============================================

@router.get("/groups")
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


@router.post("/groups")
async def create_keyword_group(data: GroupCreate, _: bool = Depends(verify_token)):
    """创建关键词分组"""
    try:
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


@router.delete("/groups/{group_id}")
async def delete_keyword_group(group_id: int, _: bool = Depends(verify_token)):
    """删除关键词分组（软删除，标记为inactive）"""
    try:
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE keyword_group_id = %s",
            (group_id,)
        )
        if sites_count and sites_count > 0:
            return {"success": False, "message": f"无法删除：有 {sites_count} 个站点正在使用此分组"}

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


@router.put("/groups/{group_id}")
async def update_keyword_group(group_id: int, data: GroupUpdate, _: bool = Depends(verify_token)):
    """更新关键词分组"""
    try:
        group = await fetch_one(
            "SELECT id, is_default FROM keyword_groups WHERE id = %s AND status = 1",
            (group_id,)
        )
        if not group:
            return {"success": False, "message": "分组不存在"}

        updates = []
        params = []

        if data.name is not None:
            updates.append("name = %s")
            params.append(data.name)

        if data.description is not None:
            updates.append("description = %s")
            params.append(data.description)

        if data.is_default is not None:
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


# ============================================
# 关键词列表和CRUD
# ============================================

@router.get("/list")
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

        where_clause = "group_id = %s AND status = 1"
        params = [group_id]

        if search:
            where_clause += " AND keyword LIKE %s"
            params.append(f"%{search}%")

        count_sql = f"SELECT COUNT(*) FROM keywords WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

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


@router.put("/{keyword_id}")
async def update_keyword(keyword_id: int, data: dict, _: bool = Depends(verify_token)):
    """更新关键词"""
    try:
        existing = await fetch_one("SELECT id FROM keywords WHERE id = %s", (keyword_id,))
        if not existing:
            return {"success": False, "message": "关键词不存在"}

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


@router.delete("/{keyword_id}")
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


# ============================================
# 批量操作
# ============================================

@router.delete("/batch")
async def batch_delete_keywords(data: BatchIds, _: bool = Depends(verify_token)):
    """批量删除关键词（软删除）"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "deleted": 0}

    try:
        placeholders = ','.join(['%s'] * len(data.ids))
        await execute_query(
            f"UPDATE keywords SET status = 0 WHERE id IN ({placeholders})",
            tuple(data.ids)
        )
        return {"success": True, "deleted": len(data.ids)}
    except Exception as e:
        logger.error(f"Failed to batch delete keywords: {e}")
        return {"success": False, "message": str(e), "deleted": 0}


@router.delete("/delete-all")
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


@router.put("/batch/status")
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


@router.put("/batch/move")
async def batch_move_keywords(data: BatchMoveGroup, _: bool = Depends(verify_token)):
    """批量移动关键词到其他分组"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "moved": 0}

    try:
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
# 添加和上传
# ============================================

@router.post("/add")
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


@router.post("/batch")
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


@router.post("/upload")
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
    if not file.filename or not file.filename.endswith('.txt'):
        raise HTTPException(status_code=400, detail="只支持 .txt 格式文件")

    content = await file.read()

    try:
        text = content.decode('utf-8')
    except UnicodeDecodeError:
        try:
            text = content.decode('gbk')
        except UnicodeDecodeError:
            text = content.decode('utf-8', errors='ignore')

    keywords = [line.strip() for line in text.splitlines() if line.strip()]

    if not keywords:
        raise HTTPException(status_code=400, detail="文件中没有有效的关键词")

    if len(keywords) > 500000:
        raise HTTPException(status_code=400, detail="单次最多上传 500000 个关键词")

    result = await group.add_keywords_batch(keywords, group_id)

    return {
        "success": True,
        "message": f"成功添加 {result['added']} 个关键词，跳过 {result['skipped']} 个重复",
        "total": len(keywords),
        "added": result['added'],
        "skipped": result['skipped']
    }


# ============================================
# 统计和缓存管理
# ============================================

@router.get("/stats")
async def get_keyword_stats(
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """获取关键词分组统计信息"""
    return group.get_stats()


@router.get("/random")
async def get_random_keywords(
    count: int = Query(default=10, ge=1, le=100),
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """获取随机关键词"""
    keywords = await group.get_random(count)
    return {"keywords": keywords, "count": len(keywords)}


@router.post("/reload")
async def reload_keywords(
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """重新加载关键词ID列表"""
    count = await group.reload()
    return {"success": True, "total": count}


@router.post("/cache/clear")
async def clear_keyword_cache(
    group: AsyncKeywordGroupManager = Depends(get_keyword_group_dep),
    _: bool = Depends(verify_token)
):
    """清理关键词Redis缓存"""
    try:
        redis_client = group.redis

        cleared = 0
        cursor = 0
        while True:
            cursor, keys = await redis_client.scan(cursor, match="keyword:*", count=1000)
            if keys:
                await redis_client.delete(*keys)
                cleared += len(keys)
            if cursor == 0:
                break

        group._cursor = 0

        return {"success": True, "cleared": cleared, "message": f"已清理 {cleared} 个关键词缓存"}
    except Exception as e:
        logger.error(f"Failed to clear keyword cache: {e}")
        return {"success": False, "message": str(e)}
