# -*- coding: utf-8 -*-
"""图片相关 Pydantic 模型"""
from typing import List
from pydantic import BaseModel


class ImageUrlCreate(BaseModel):
    """添加单个图片URL"""
    group_id: int = 1
    url: str


class ImageUrlBatchCreate(BaseModel):
    """批量添加图片URL"""
    group_id: int = 1
    urls: List[str]  # 最多100000条
