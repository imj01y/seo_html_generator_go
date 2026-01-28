# -*- coding: utf-8 -*-
"""
图片管理 API 路由

包含图片分组管理、图片 URL CRUD、批量操作、缓存管理等功能。
"""
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query
from loguru import logger

from api.deps import verify_token, verify_token_or_api_token, get_image_group_dep
from api.schemas import (
    GroupCreate, GroupUpdate,
    ImageUrlCreate, ImageUrlBatchCreate,
    BatchIds, BatchStatusUpdate, BatchMoveGroup, DeleteAllRequest
)
from core.image_group_manager import AsyncImageGroupManager
from database.db import fetch_one, fetch_all, fetch_value, execute_query, insert

router = APIRouter(prefix="/api/images", tags=["图片管理"])


# ============================================
# 图片分组管理API
# ============================================

@router.get("/groups")
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


@router.post("/groups")
async def create_image_group(data: GroupCreate, _: bool = Depends(verify_token)):
    """创建图片分组"""
    try:
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


@router.delete("/groups/{group_id}")
async def delete_image_group(group_id: int, _: bool = Depends(verify_token)):
    """删除图片分组（软删除，标记为inactive）"""
    try:
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE image_group_id = %s",
            (group_id,)
        )
        if sites_count and sites_count > 0:
            return {"success": False, "message": f"无法删除：有 {sites_count} 个站点正在使用此分组"}

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


@router.put("/groups/{group_id}")
async def update_image_group(group_id: int, data: GroupUpdate, _: bool = Depends(verify_token)):
    """更新图片分组"""
    try:
        group = await fetch_one(
            "SELECT id, is_default FROM image_groups WHERE id = %s AND status = 1",
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


# ============================================
# 图片URL列表和CRUD
# ============================================

@router.get("/urls/list")
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

        where_clause = "group_id = %s AND status = 1"
        params = [group_id]

        if search:
            where_clause += " AND url LIKE %s"
            params.append(f"%{search}%")

        count_sql = f"SELECT COUNT(*) FROM images WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

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


@router.put("/urls/{image_id}")
async def update_image_url(image_id: int, data: dict, _: bool = Depends(verify_token)):
    """更新图片URL"""
    try:
        existing = await fetch_one("SELECT id FROM images WHERE id = %s", (image_id,))
        if not existing:
            return {"success": False, "message": "图片不存在"}

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


@router.delete("/urls/{image_id}")
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


# ============================================
# 批量操作
# ============================================

@router.delete("/batch")
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


@router.delete("/delete-all")
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


@router.put("/batch/status")
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


@router.put("/batch/move")
async def batch_move_images(data: BatchMoveGroup, _: bool = Depends(verify_token)):
    """批量移动图片URL到其他分组"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "moved": 0}

    try:
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
# 添加
# ============================================

@router.post("/urls/add")
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


@router.post("/urls/batch")
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


# ============================================
# 统计和缓存管理
# ============================================

@router.get("/urls/stats")
async def get_image_url_stats(
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """获取图片URL分组统计信息"""
    return group.get_stats()


@router.get("/urls/random")
async def get_random_image_urls(
    count: int = Query(default=10, ge=1, le=100),
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """获取随机图片URL"""
    urls = await group.get_random(count)
    return {"urls": urls, "count": len(urls)}


@router.post("/urls/reload")
async def reload_image_urls(
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """重新加载图片URL ID列表"""
    count = await group.reload()
    return {"success": True, "total": count}


@router.post("/cache/clear")
async def clear_image_cache(
    group: AsyncImageGroupManager = Depends(get_image_group_dep),
    _: bool = Depends(verify_token)
):
    """清理图片Redis缓存"""
    try:
        redis_client = group.redis
        group_id = group.group_id

        hash_key = f"images:pool:{group_id}"
        new_ids_key = f"images:new_ids:{group_id}"
        url_hash_key = f"images:url_hashes:{group_id}"

        cleared = 0
        for key in [hash_key, new_ids_key, url_hash_key]:
            if await redis_client.exists(key):
                await redis_client.delete(key)
                cleared += 1

        group._pool_start = 0
        group._pool_end = 0
        group._cursor = 0

        return {"success": True, "cleared": cleared, "message": f"已清理图片缓存"}
    except Exception as e:
        logger.error(f"Failed to clear image cache: {e}")
        return {"success": False, "message": str(e)}
