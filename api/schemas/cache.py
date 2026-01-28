# -*- coding: utf-8 -*-
"""缓存相关 Pydantic 模型"""
from pydantic import BaseModel


class CacheWarmupRequest(BaseModel):
    domain: str
    count: int = 100
