# -*- coding: utf-8 -*-
"""模板相关 Pydantic 模型"""
from typing import Optional
from pydantic import BaseModel


class TemplateCreate(BaseModel):
    """创建模板"""
    site_group_id: int = 1  # 所属站群ID
    name: str  # 模板标识名（唯一）
    display_name: str  # 显示名称
    description: Optional[str] = None
    content: str  # HTML模板内容


class TemplateUpdate(BaseModel):
    """更新模板"""
    site_group_id: Optional[int] = None  # 所属站群ID
    display_name: Optional[str] = None
    description: Optional[str] = None
    content: Optional[str] = None
    status: Optional[int] = None  # 1=启用, 0=禁用
