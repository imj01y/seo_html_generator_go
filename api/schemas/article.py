# -*- coding: utf-8 -*-
"""文章相关 Pydantic 模型"""
from typing import List, Optional
from pydantic import BaseModel


class ArticleCreate(BaseModel):
    """添加单篇文章"""
    group_id: int = 1
    title: str
    content: str


class ArticleBatchCreate(BaseModel):
    """批量添加文章"""
    articles: List[ArticleCreate]  # 最多1000条


class ArticleUpdate(BaseModel):
    """更新文章"""
    group_id: Optional[int] = None
    title: Optional[str] = None
    content: Optional[str] = None
    status: Optional[int] = None
