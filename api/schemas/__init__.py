# -*- coding: utf-8 -*-
"""
API Pydantic 模型模块

按领域组织所有请求和响应模型。
"""
from .auth import LoginRequest, LoginResponse, PasswordChangeRequest
from .site import (
    SiteCreate, SiteUpdate,
    SiteGroupCreate, SiteGroupUpdate
)
from .keyword import KeywordCreate, KeywordBatchCreate
from .image import ImageUrlCreate, ImageUrlBatchCreate
from .article import ArticleCreate, ArticleBatchCreate, ArticleUpdate
from .template import TemplateCreate, TemplateUpdate
from .cache import CacheWarmupRequest
from .common import (
    GroupCreate, GroupUpdate,
    BatchIds, BatchStatusUpdate, BatchMoveGroup, DeleteAllRequest
)

__all__ = [
    # Auth
    'LoginRequest', 'LoginResponse', 'PasswordChangeRequest',
    # Site
    'SiteCreate', 'SiteUpdate', 'SiteGroupCreate', 'SiteGroupUpdate',
    # Keyword
    'KeywordCreate', 'KeywordBatchCreate',
    # Image
    'ImageUrlCreate', 'ImageUrlBatchCreate',
    # Article
    'ArticleCreate', 'ArticleBatchCreate', 'ArticleUpdate',
    # Template
    'TemplateCreate', 'TemplateUpdate',
    # Cache
    'CacheWarmupRequest',
    # Common
    'GroupCreate', 'GroupUpdate', 'BatchIds', 'BatchStatusUpdate',
    'BatchMoveGroup', 'DeleteAllRequest',
]
