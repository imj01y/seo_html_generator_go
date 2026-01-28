# -*- coding: utf-8 -*-
"""关键词相关 Pydantic 模型"""
from typing import List
from pydantic import BaseModel


class KeywordCreate(BaseModel):
    """添加单个关键词"""
    group_id: int = 1
    keyword: str


class KeywordBatchCreate(BaseModel):
    """批量添加关键词"""
    group_id: int = 1
    keywords: List[str]  # 最多100000条
