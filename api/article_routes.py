# -*- coding: utf-8 -*-
"""
文章管理 API 路由

包含文章分组管理、文章 CRUD、批量操作等功能。
"""
from typing import Optional
from collections import defaultdict

from fastapi import APIRouter, Depends, HTTPException, Query
from loguru import logger

from api.deps import verify_token, verify_token_or_api_token, get_redis_client
from api.schemas import (
    GroupCreate, GroupUpdate,
    ArticleCreate, ArticleBatchCreate, ArticleUpdate,
    BatchIds, BatchStatusUpdate, BatchMoveGroup, DeleteAllRequest
)
from database.db import fetch_one, fetch_all, fetch_value, execute_query, insert

router = APIRouter(prefix="/api/articles", tags=["文章管理"])


# ============================================
# 文章分组管理API
# ============================================

@router.get("/groups")
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
        return {"groups": [{"id": 1, "site_group_id": 1, "name": "默认文章分组", "description": "系统默认文章分组", "is_default": 1}]}


@router.post("/groups")
async def create_article_group(data: GroupCreate, _: bool = Depends(verify_token)):
    """创建文章分组"""
    try:
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


@router.delete("/groups/{group_id}")
async def delete_article_group(group_id: int, _: bool = Depends(verify_token)):
    """删除文章分组（软删除，标记为inactive）"""
    try:
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


@router.put("/groups/{group_id}")
async def update_article_group(group_id: int, data: GroupUpdate, _: bool = Depends(verify_token)):
    """更新文章分组"""
    try:
        group = await fetch_one(
            "SELECT id, is_default FROM article_groups WHERE id = %s AND status = 1",
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


# ============================================
# 文章列表和CRUD
# ============================================

@router.get("/list")
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

        where_clause = "group_id = %s AND status = 1"
        params = [group_id]

        if search:
            where_clause += " AND (title LIKE %s OR content LIKE %s)"
            params.extend([f"%{search}%", f"%{search}%"])

        count_sql = f"SELECT COUNT(*) FROM original_articles WHERE {where_clause}"
        total = await fetch_value(count_sql, tuple(params)) or 0

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


@router.get("/{article_id}")
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


@router.put("/{article_id}")
async def update_article(article_id: int, data: ArticleUpdate, _: bool = Depends(verify_token)):
    """更新文章"""
    try:
        existing = await fetch_one("SELECT id FROM original_articles WHERE id = %s", (article_id,))
        if not existing:
            return {"success": False, "message": "文章不存在"}

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


@router.delete("/{article_id}")
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


# ============================================
# 批量操作
# ============================================

@router.delete("/batch/delete")
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


@router.delete("/delete-all")
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


@router.put("/batch/status")
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


@router.put("/batch/move")
async def batch_move_articles(data: BatchMoveGroup, _: bool = Depends(verify_token)):
    """批量移动文章到其他分组"""
    if not data.ids:
        return {"success": False, "message": "ID列表不能为空", "moved": 0}

    try:
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


# ============================================
# 添加
# ============================================

@router.post("/add")
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
        article_id = await insert('original_articles', {
            'group_id': data.group_id,
            'title': data.title,
            'content': data.content,
            'status': 1
        })

        if article_id:
            redis_client = get_redis_client()
            if redis_client:
                queue_key = f"pending:articles:{data.group_id}"
                await redis_client.lpush(queue_key, article_id)
            return {"success": True, "id": article_id}

        return {"success": False, "message": "文章已存在或添加失败"}
    except Exception as e:
        logger.error(f"Failed to add article: {e}")
        return {"success": False, "message": str(e)}


@router.post("/batch")
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

        redis_client = get_redis_client()
        if new_ids and redis_client:
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
