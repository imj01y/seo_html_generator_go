# -*- coding: utf-8 -*-
"""通用 Pydantic 模型"""
from typing import List, Optional
from pydantic import BaseModel


class GroupCreate(BaseModel):
    """创建分组请求"""
    site_group_id: int = 1  # 所属站群ID
    name: str
    description: Optional[str] = None
    is_default: bool = False


class GroupUpdate(BaseModel):
    """更新分组请求"""
    name: Optional[str] = None
    description: Optional[str] = None
    is_default: Optional[int] = None


class BatchIds(BaseModel):
    """批量操作ID列表"""
    ids: List[int]


class BatchStatusUpdate(BaseModel):
    """批量状态更新"""
    ids: List[int]
    status: int  # 1=启用, 0=禁用


class BatchMoveGroup(BaseModel):
    """批量移动分组"""
    ids: List[int]
    group_id: int


class DeleteAllRequest(BaseModel):
    """删除全部请求"""
    group_id: Optional[int] = None  # 为空表示删除所有分组
    confirm: bool = False  # 必须为True才执行
