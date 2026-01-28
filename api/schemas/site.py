# -*- coding: utf-8 -*-
"""站点相关 Pydantic 模型"""
from typing import Optional
from pydantic import BaseModel


class SiteCreate(BaseModel):
    site_group_id: int = 1  # 所属站群ID
    domain: str
    name: str
    template: str = "download_site"
    keyword_group_id: Optional[int] = None  # 绑定的关键词分组ID
    image_group_id: Optional[int] = None    # 绑定的图片分组ID
    article_group_id: Optional[int] = None  # 绑定的文章分组ID
    icp_number: Optional[str] = None
    baidu_token: Optional[str] = None
    analytics: Optional[str] = None


class SiteUpdate(BaseModel):
    site_group_id: Optional[int] = None  # 所属站点分组ID
    name: Optional[str] = None
    template: Optional[str] = None
    status: Optional[int] = None  # 1=启用, 0=禁用
    keyword_group_id: Optional[int] = None  # 绑定的关键词分组ID
    image_group_id: Optional[int] = None    # 绑定的图片分组ID
    article_group_id: Optional[int] = None  # 绑定的文章分组ID
    icp_number: Optional[str] = None
    baidu_token: Optional[str] = None
    analytics: Optional[str] = None


class SiteGroupCreate(BaseModel):
    """创建站群请求"""
    name: str
    description: Optional[str] = None


class SiteGroupUpdate(BaseModel):
    """更新站群请求"""
    name: Optional[str] = None
    description: Optional[str] = None
    status: Optional[int] = None
    is_default: Optional[int] = None
