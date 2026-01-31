"""
SEO HTML生成器核心模块

提供SEO核心功能的模块集合：
- encoder: HTML实体编码器
- class_generator: 随机class生成器
- emoji: Emoji管理器
- link_generator: 内链URL生成器
- title_generator: Title生成器
- seo_core: SEO核心整合类
"""

from .encoder import HTMLEntityEncoder, encode, encode_text
from .class_generator import ClassGenerator, cls, generate_class
from .emoji import EmojiManager, get_emoji_manager, get_random_emoji, get_random_emojis
from .link_generator import LinkGenerator, random_url, random_internal_link, generate_url_list
from .title_generator import TitleGenerator, generate_title
from .seo_core import (
    SEOCore,
    get_seo_core,
    init_seo_core,
    render_page,
)

__all__ = [
    # Encoder
    'HTMLEntityEncoder',
    'encode',
    'encode_text',
    # Class Generator
    'ClassGenerator',
    'cls',
    'generate_class',
    # Emoji
    'EmojiManager',
    'get_emoji_manager',
    'get_random_emoji',
    'get_random_emojis',
    # Link Generator
    'LinkGenerator',
    'random_url',
    'random_internal_link',
    'generate_url_list',
    # Title Generator
    'TitleGenerator',
    'generate_title',
    # SEO Core
    'SEOCore',
    'get_seo_core',
    'init_seo_core',
    'render_page',
]
