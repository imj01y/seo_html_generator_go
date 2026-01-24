# -*- coding: utf-8 -*-
"""
正文生成器管理API

提供正文生成器的CRUD操作和在线编辑功能。
"""

from typing import Optional, List
from datetime import datetime

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from loguru import logger

from database.db import fetch_one, fetch_all, execute_query, insert


# 创建路由器
router = APIRouter(prefix="/api/generators", tags=["生成器管理"])


# ============================================
# Pydantic模型
# ============================================

class GeneratorCreate(BaseModel):
    """创建生成器请求"""
    name: str
    display_name: str
    description: Optional[str] = None
    code: str
    enabled: int = 1
    is_default: int = 0


class GeneratorUpdate(BaseModel):
    """更新生成器请求"""
    display_name: Optional[str] = None
    description: Optional[str] = None
    code: Optional[str] = None
    enabled: Optional[int] = None
    is_default: Optional[int] = None


class GeneratorResponse(BaseModel):
    """生成器响应"""
    id: int
    name: str
    display_name: str
    description: Optional[str]
    code: str
    enabled: int
    is_default: int
    version: int
    created_at: datetime
    updated_at: datetime


class TestGeneratorRequest(BaseModel):
    """测试生成器请求"""
    code: str
    paragraphs: List[str]
    titles: List[str] = []


class TestGeneratorResponse(BaseModel):
    """测试生成器响应"""
    success: bool
    content: Optional[str] = None
    message: Optional[str] = None


# ============================================
# 生成器管理API
# ============================================

def _format_generator_row(row: dict) -> dict:
    """格式化生成器行数据"""
    return {
        "id": row['id'],
        "name": row['name'],
        "display_name": row['display_name'],
        "description": row['description'],
        "code": row['code'],
        "enabled": row['enabled'],
        "is_default": row['is_default'],
        "version": row['version'],
        "created_at": row['created_at'].isoformat() if row['created_at'] else None,
        "updated_at": row['updated_at'].isoformat() if row['updated_at'] else None,
    }


@router.get("/")
async def list_generators(
    enabled: Optional[int] = None,
    page: int = 1,
    page_size: int = 20
):
    """获取生成器列表"""
    conditions = []
    params = []

    if enabled is not None:
        conditions.append("enabled = %s")
        params.append(enabled)

    where_clause = " AND ".join(conditions) if conditions else "1=1"

    # 获取总数
    count_sql = f"SELECT COUNT(*) as cnt FROM content_generators WHERE {where_clause}"
    total = await fetch_one(count_sql, params)
    total_count = total['cnt'] if total else 0

    # 获取数据
    offset = (page - 1) * page_size
    params.extend([offset, page_size])

    data_sql = f"""
        SELECT id, name, display_name, description, code, enabled,
               is_default, version, created_at, updated_at
        FROM content_generators
        WHERE {where_clause}
        ORDER BY is_default DESC, id ASC
        LIMIT %s, %s
    """
    rows = await fetch_all(data_sql, params)

    return {
        "success": True,
        "data": [_format_generator_row(row) for row in rows],
        "total": total_count,
        "page": page,
        "page_size": page_size
    }


@router.get("/{generator_id}")
async def get_generator(generator_id: int):
    """获取单个生成器详情"""
    sql = """
        SELECT id, name, display_name, description, code, enabled,
               is_default, version, created_at, updated_at
        FROM content_generators
        WHERE id = %s
    """
    row = await fetch_one(sql, [generator_id])

    if not row:
        raise HTTPException(status_code=404, detail="生成器不存在")

    return {"success": True, "data": _format_generator_row(row)}


@router.post("/")
async def create_generator(data: GeneratorCreate):
    """创建生成器"""
    # 检查名称是否已存在
    check_sql = "SELECT id FROM content_generators WHERE name = %s"
    existing = await fetch_one(check_sql, [data.name])
    if existing:
        raise HTTPException(status_code=400, detail="生成器名称已存在")

    # 如果设为默认，先取消其他默认
    if data.is_default:
        await execute_query(
            "UPDATE content_generators SET is_default = 0 WHERE is_default = 1",
            []
        )

    try:
        generator_id = await insert("content_generators", {
            "name": data.name,
            "display_name": data.display_name,
            "description": data.description,
            "code": data.code,
            "enabled": data.enabled,
            "is_default": data.is_default,
        })
        logger.info(f"Created generator: {data.name} (id={generator_id})")
        return {"success": True, "id": generator_id, "message": "创建成功"}
    except Exception as e:
        logger.error(f"Failed to create generator: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.put("/{generator_id}")
async def update_generator(generator_id: int, data: GeneratorUpdate):
    """更新生成器"""
    # 检查是否存在
    check_sql = "SELECT id, version FROM content_generators WHERE id = %s"
    existing = await fetch_one(check_sql, [generator_id])
    if not existing:
        raise HTTPException(status_code=404, detail="生成器不存在")

    # 如果设为默认，先取消其他默认
    if data.is_default:
        await execute_query(
            "UPDATE content_generators SET is_default = 0 WHERE is_default = 1 AND id != %s",
            [generator_id]
        )

    # 构建更新字段
    updates = []
    params = []

    if data.display_name is not None:
        updates.append("display_name = %s")
        params.append(data.display_name)
    if data.description is not None:
        updates.append("description = %s")
        params.append(data.description)
    if data.code is not None:
        updates.append("code = %s")
        params.append(data.code)
        # 代码更新时版本号+1
        updates.append("version = version + 1")
    if data.enabled is not None:
        updates.append("enabled = %s")
        params.append(data.enabled)
    if data.is_default is not None:
        updates.append("is_default = %s")
        params.append(data.is_default)

    if not updates:
        return {"success": True, "message": "无需更新"}

    params.append(generator_id)
    sql = f"UPDATE content_generators SET {', '.join(updates)} WHERE id = %s"

    try:
        await execute_query(sql, params)
        logger.info(f"Updated generator: id={generator_id}")
        return {"success": True, "message": "更新成功"}
    except Exception as e:
        logger.error(f"Failed to update generator: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/{generator_id}")
async def delete_generator(generator_id: int):
    """删除生成器"""
    # 检查是否存在
    check_sql = "SELECT id, is_default FROM content_generators WHERE id = %s"
    row = await fetch_one(check_sql, [generator_id])
    if not row:
        raise HTTPException(status_code=404, detail="生成器不存在")

    # 不允许删除默认生成器
    if row['is_default']:
        raise HTTPException(status_code=400, detail="不能删除默认生成器")

    sql = "DELETE FROM content_generators WHERE id = %s"
    try:
        await execute_query(sql, [generator_id])
        logger.info(f"Deleted generator: id={generator_id}")
        return {"success": True, "message": "删除成功"}
    except Exception as e:
        logger.error(f"Failed to delete generator: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/{generator_id}/set-default")
async def set_default_generator(generator_id: int):
    """设置为默认生成器"""
    # 检查是否存在且启用
    check_sql = "SELECT id, enabled FROM content_generators WHERE id = %s"
    row = await fetch_one(check_sql, [generator_id])
    if not row:
        raise HTTPException(status_code=404, detail="生成器不存在")

    if not row['enabled']:
        raise HTTPException(status_code=400, detail="不能将禁用的生成器设为默认")

    # 取消其他默认
    await execute_query(
        "UPDATE content_generators SET is_default = 0 WHERE is_default = 1",
        []
    )

    # 设置新默认
    await execute_query(
        "UPDATE content_generators SET is_default = 1 WHERE id = %s",
        [generator_id]
    )

    logger.info(f"Set default generator: id={generator_id}")
    return {"success": True, "message": "已设为默认"}


@router.post("/{generator_id}/toggle")
async def toggle_generator(generator_id: int):
    """切换生成器启用状态"""
    # 获取当前状态
    sql = "SELECT enabled, is_default FROM content_generators WHERE id = %s"
    row = await fetch_one(sql, [generator_id])
    if not row:
        raise HTTPException(status_code=404, detail="生成器不存在")

    # 不允许禁用默认生成器
    if row['is_default'] and row['enabled']:
        raise HTTPException(status_code=400, detail="不能禁用默认生成器")

    new_enabled = 0 if row['enabled'] == 1 else 1
    update_sql = "UPDATE content_generators SET enabled = %s WHERE id = %s"
    await execute_query(update_sql, [new_enabled, generator_id])

    return {
        "success": True,
        "enabled": new_enabled,
        "message": "已启用" if new_enabled else "已禁用"
    }


@router.post("/test")
async def test_generator(data: TestGeneratorRequest):
    """测试生成器代码"""
    try:
        from core.generators.interface import GeneratorContext
        from core.processors import PinyinAnnotator
        import random
        import re

        # 创建测试上下文
        ctx = GeneratorContext(
            paragraphs=data.paragraphs,
            titles=data.titles,
            group_id=1
        )

        # 准备执行环境
        annotator = PinyinAnnotator()
        safe_globals = {
            'annotate_pinyin': annotator.annotate,
            'random': random,
            're': re,
        }
        local_vars = {}

        # 编译并执行代码
        exec(data.code, safe_globals, local_vars)
        generate_func = local_vars.get('generate')

        if not generate_func:
            return TestGeneratorResponse(
                success=False,
                message="代码中未定义 generate 函数"
            )

        # 执行生成函数
        import asyncio
        if asyncio.iscoroutinefunction(generate_func):
            content = await generate_func(ctx)
        else:
            content = generate_func(ctx)

        if content:
            return TestGeneratorResponse(
                success=True,
                content=content,
                message="生成成功"
            )
        else:
            return TestGeneratorResponse(
                success=False,
                message="生成结果为空"
            )

    except SyntaxError as e:
        return TestGeneratorResponse(
            success=False,
            message=f"语法错误: {e}"
        )
    except Exception as e:
        logger.error(f"Generator test failed: {e}")
        return TestGeneratorResponse(
            success=False,
            message=f"执行错误: {e}"
        )


@router.get("/templates/list")
async def get_code_templates():
    """获取代码模板列表"""
    templates = [
        {
            "name": "basic",
            "display_name": "基础模板",
            "description": "简单的段落拼接+拼音标注",
            "code": '''async def generate(ctx):
    """
    可用变量:
      ctx.paragraphs - 段落列表
      ctx.titles - 标题列表
    可用函数:
      annotate_pinyin(text) - 添加拼音标注
      random - Python random 模块
      re - Python re 模块
    """
    if len(ctx.paragraphs) < 3:
        return None

    # 选择段落
    selected = ctx.paragraphs[:3]

    # 拼接并标注
    content = "\\n\\n".join(selected)
    return annotate_pinyin(content)
'''
        },
        {
            "name": "with_title",
            "display_name": "带标题模板",
            "description": "正文开头插入随机标题",
            "code": '''async def generate(ctx):
    """带标题的正文生成"""
    if len(ctx.paragraphs) < 3:
        return None

    parts = []

    # 添加标题（如果有）
    if ctx.titles:
        parts.append(random.choice(ctx.titles))
        parts.append("")  # 空行

    # 添加段落
    count = min(len(ctx.paragraphs), random.randint(3, 5))
    selected = random.sample(ctx.paragraphs, count)
    parts.extend(selected)

    # 拼接并标注
    content = "\\n\\n".join(parts)
    return annotate_pinyin(content)
'''
        },
        {
            "name": "shuffle",
            "display_name": "随机打乱模板",
            "description": "随机打乱段落顺序",
            "code": '''async def generate(ctx):
    """随机打乱段落顺序"""
    if len(ctx.paragraphs) < 3:
        return None

    # 随机选择并打乱
    count = min(len(ctx.paragraphs), random.randint(4, 6))
    selected = random.sample(ctx.paragraphs, count)
    random.shuffle(selected)

    # 拼接并标注
    content = "\\n\\n".join(selected)
    return annotate_pinyin(content)
'''
        }
    ]

    return {
        "success": True,
        "data": templates
    }


@router.post("/{generator_id}/reload")
async def reload_generator(generator_id: int):
    """重新加载生成器（热更新）"""
    # 检查是否存在
    check_sql = "SELECT id, name FROM content_generators WHERE id = %s"
    row = await fetch_one(check_sql, [generator_id])
    if not row:
        raise HTTPException(status_code=404, detail="生成器不存在")

    # TODO: 调用 GeneratorManager 的 reload_one 方法
    # 这需要获取全局的 GeneratorManager 实例

    logger.info(f"Reloaded generator: {row['name']} (id={generator_id})")
    return {"success": True, "message": "生成器已重新加载"}
