# -*- coding: utf-8 -*-
"""
站点和站群管理 API 路由

包含站点 CRUD、批量操作、站群管理等功能。
"""
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query
from loguru import logger

from api.deps import verify_token
from api.schemas import (
    SiteCreate, SiteUpdate,
    SiteGroupCreate, SiteGroupUpdate,
    BatchIds, BatchStatusUpdate
)
from database.db import fetch_one, fetch_all, fetch_value, execute_query, insert

router = APIRouter(tags=["站点管理"])


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
        where_clause = "1=1"
        params = []

        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        total = await fetch_value(f"SELECT COUNT(*) FROM sites WHERE {where_clause}", tuple(params) if params else None)

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
        existing = await fetch_one("SELECT id FROM sites WHERE id = %s", (site_id,))
        if not existing:
            return {"success": False, "message": "站点不存在"}

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
        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE site_group_id = %s",
            (group_id,)
        )
        if sites_count and sites_count > 0:
            return {"success": False, "message": f"无法删除：有 {sites_count} 个站点属于此站群"}

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
