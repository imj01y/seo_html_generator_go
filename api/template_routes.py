# -*- coding: utf-8 -*-
"""
模板管理 API 路由

包含模板 CRUD、模板选项等功能。
"""
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query
from loguru import logger

from api.deps import verify_token
from api.schemas import TemplateCreate, TemplateUpdate
from database.db import fetch_one, fetch_all, fetch_value, execute_query, insert

router = APIRouter(prefix="/api/templates", tags=["模板管理"])


@router.get("/")
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
        where_clause = "1=1"
        params = []

        if status is not None:
            where_clause += " AND status = %s"
            params.append(status)

        if site_group_id is not None:
            where_clause += " AND site_group_id = %s"
            params.append(site_group_id)

        total = await fetch_value(
            f"SELECT COUNT(*) FROM templates WHERE {where_clause}",
            tuple(params) if params else None
        ) or 0

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


@router.get("/options")
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


@router.get("/{template_id}")
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


@router.get("/{template_id}/sites")
async def get_template_sites(template_id: int, _: dict = Depends(verify_token)):
    """
    获取使用此模板的站点列表
    """
    try:
        template = await fetch_one(
            "SELECT name FROM templates WHERE id = %s",
            (template_id,)
        )
        if not template:
            raise HTTPException(status_code=404, detail="模板不存在")

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


@router.post("/")
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


@router.put("/{template_id}")
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
        existing = await fetch_one(
            "SELECT id, version FROM templates WHERE id = %s",
            (template_id,)
        )
        if not existing:
            return {"success": False, "message": "模板不存在"}

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


@router.delete("/{template_id}")
async def delete_template(template_id: int, _: dict = Depends(verify_token)):
    """
    删除模板

    如果有站点正在使用此模板，则拒绝删除
    """
    try:
        template = await fetch_one(
            "SELECT name FROM templates WHERE id = %s",
            (template_id,)
        )
        if not template:
            return {"success": False, "message": "模板不存在"}

        sites_count = await fetch_value(
            "SELECT COUNT(*) FROM sites WHERE template = %s AND status = 1",
            (template['name'],)
        )

        if sites_count and sites_count > 0:
            return {
                "success": False,
                "message": f"无法删除：有 {sites_count} 个站点正在使用此模板"
            }

        await execute_query("DELETE FROM templates WHERE id = %s", (template_id,))
        return {"success": True}
    except Exception as e:
        logger.error(f"Failed to delete template: {e}")
        return {"success": False, "message": str(e)}
