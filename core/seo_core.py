"""
SEO核心整合模块

整合所有SEO生成器组件，提供统一的模板渲染接口。

主要功能:
- 整合编码器、class生成器、Emoji、链接生成器等
- 提供Jinja2模板函数
- 管理站点配置
- 生成完整HTML页面

架构说明:
- 关键词和图片URL使用全局异步池管理器 (MySQL + Redis)
- 模板渲染使用本地同步缓存 (通过 load_keywords_sync/load_image_urls_sync 预加载)

使用方法:
    >>> from core.seo_core import init_seo_core, get_seo_core
    >>> core = init_seo_core(template_dir="./templates")
    >>> core.load_keywords_sync(keywords_list)  # 预加载关键词到同步缓存
    >>> core.load_image_urls_sync(urls_list)    # 预加载图片URL到同步缓存
    >>> html = core.render_page("template.html", site_config=site_config)
"""
from typing import Dict, Any, Optional, List, Set, Callable
from datetime import datetime
import random
import asyncio
import hashlib
import time as time_module

from jinja2 import Environment, FileSystemLoader, select_autoescape, Template
from markupsafe import Markup

from .encoder import HTMLEntityEncoder
from .class_generator import ClassGenerator
from .emoji import get_emoji_manager
from .link_generator import (LinkGenerator, AnchorGenerator)
from .title_generator import TitleGenerator
from .keyword_group_manager import get_keyword_group
from .keyword_cache_pool import get_keyword_cache_pool
from .image_group_manager import get_image_group
from .content_pool_manager import get_content_pool_manager
from loguru import logger


class SEOCore:
    """
    SEO核心整合类

    整合所有生成器组件，提供统一的渲染接口。
    关键词和图片URL通过全局异步池管理，模板渲染使用本地同步缓存。

    Attributes:
        encoder: HTML实体编码器
        class_gen: CSS类名生成器
        emoji_manager: Emoji管理器
        link_gen: 链接生成器
        anchor_gen: 锚文本生成器
        title_gen: 标题生成器
        jinja_env: Jinja2环境

    Example:
        >>> core = SEOCore(template_dir="./templates")
        >>> core.load_keywords_sync(keywords)  # 预加载关键词
        >>> core.load_image_urls_sync(urls)    # 预加载图片URL
        >>> html = core.render_page("download_site/index.html")
    """

    def __init__(
            self,
            template_dir: str = "./templates",
            encoding_ratio: float = 0.5,
            emoji_file: Optional[str] = None
    ):
        """
        初始化SEO核心

        Args:
            template_dir: 模板目录路径
            encoding_ratio: HTML实体编码混合比例
            emoji_file: Emoji数据文件路径
        """
        # 初始化各组件
        self.encoder = HTMLEntityEncoder(mix_ratio=encoding_ratio)
        self.class_gen = ClassGenerator()
        self.emoji_manager = get_emoji_manager()
        self.link_gen = LinkGenerator()
        self.anchor_gen = AnchorGenerator(encoder=self.encoder)
        self.title_gen = TitleGenerator(emoji_manager=self.emoji_manager)

        # 关键词池现在使用全局异步池，模板中通过 _random_keyword_sync 访问
        self._keyword_cache: List[str] = []  # 用于模板的同步关键词缓存
        self._keyword_cursor: int = 0
        self._keyword_cache_max_size: int = 10000  # 默认值，会被配置覆盖

        # 图片池现在使用全局异步池，模板中通过 _random_image_sync 访问
        self._image_url_cache: List[str] = []  # 用于模板的同步图片URL缓存
        self._image_cursor: int = 0
        self._image_cache_max_size: int = 10000  # 默认值，会被配置覆盖

        # 加载Emoji数据
        if emoji_file:
            self.emoji_manager.load_from_file(emoji_file)

        # 初始化Jinja2环境
        self.template_dir = template_dir
        self.jinja_env = self._create_jinja_env(template_dir)

        # 页面级状态（每次渲染重置）
        self._used_emojis: Set[str] = set()
        self._encoding_count: int = 0
        self._preloaded_content: Optional[str] = None  # 预加载的正文内容

        # 编译后模板缓存（避免重复解析模板内容）
        self._compiled_templates: Dict[str, Template] = {}

        logger.info(f"SEOCore initialized with template_dir: {template_dir}")

    def _create_jinja_env(self, template_dir: str) -> Environment:
        """
        创建Jinja2环境并注册自定义函数

        Args:
            template_dir: 模板目录

        Returns:
            配置好的Jinja2 Environment
        """
        env = Environment(
            loader=FileSystemLoader(template_dir),
            autoescape=select_autoescape(['html', 'xml']),
            trim_blocks=True,
            lstrip_blocks=True
        )

        # 注册模板全局函数
        env.globals.update(self._get_template_functions())

        return env

    def _get_template_functions(self) -> Dict[str, Callable]:
        """
        获取模板中可用的函数

        Returns:
            函数字典
        """
        return {
            # 编码函数
            'encode': self.encoder.encode_text,
            'encode_text': self.encoder.encode_text,

            # Class生成函数
            'cls': self.class_gen.generate,

            # 链接生成函数
            'random_url': self.link_gen.generate,

            # 关键词函数（新命名）
            'random_keyword': self._random_keyword,
            'content': self._content,

            # 关键词函数（旧命名，向后兼容）
            'random_hotspot': self._random_keyword,
            'keyword_with_emoji': self._random_keyword,
            'content_with_pinyin': self._content,

            # 图片函数
            'random_image': self._random_image_sync,

            # 工具函数
            'now': lambda: datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
            'range': range,
            'len': len,
            'str': str,
            'int': int,
            'random_number': lambda min_val, max_val: random.randint(min_val, max_val),
        }

    def _random_keyword(self) -> Markup:
        """
        获取随机关键词（编码后）

        Returns:
            编码后的关键词（Markup安全标记）
        """
        keyword = self._random_keyword_sync()
        if not keyword:
            return Markup("")
        encoded = self.encoder.encode_text(keyword)
        self._encoding_count += len(keyword)
        return Markup(encoded)

    def _content(self) -> str:
        """
        获取正文内容

        Returns:
            正文内容，内容池不可用时返回空字符串
        """
        # 优先使用预加载的内容（由 render_template_content 预先获取）
        if hasattr(self, '_preloaded_content') and self._preloaded_content:
            content = self._preloaded_content
            self._preloaded_content = None  # 使用后清空，避免重复
            return content

        # 降级：尝试同步获取（仅在非异步环境中有效）
        content_pool = get_content_pool_manager()
        if not content_pool:
            return ""

        try:
            loop = asyncio.get_event_loop()
            if not loop.is_running():
                content = loop.run_until_complete(content_pool.get_content())
                return content or ""
            else:
                # 在异步环境中，应该使用预加载的内容
                logger.warning("content() called in async context without preloaded content")
                return ""
        except Exception as e:
            logger.warning(f"Failed to get content from pool: {e}")
            return ""

    def _get_from_local_cache(self, cache: list, cursor_attr: str) -> str:
        """从本地缓存获取随机项（通用方法）"""
        if not cache:
            return ""

        cursor = getattr(self, cursor_attr)
        if cursor >= len(cache):
            random.shuffle(cache)
            setattr(self, cursor_attr, 0)
            cursor = 0

        item = cache[cursor]
        setattr(self, cursor_attr, cursor + 1)
        return item

    def _random_image_sync(self) -> str:
        """
        同步获取随机图片URL（用于模板渲染）

        优先使用全局缓存池，降级到本地缓存。

        Returns:
            图片URL字符串
        """
        # 优先使用全局缓存池
        from .image_cache_pool import get_image_cache_pool
        pool = get_image_cache_pool()
        if pool:
            url = pool.get_url_sync()
            if url:
                return url

        # 降级到本地缓存
        return self._get_from_local_cache(self._image_url_cache, '_image_cursor')

    def load_image_urls_sync(self, urls: List[str]) -> int:
        """
        同步加载图片URL到缓存（用于模板渲染）

        Args:
            urls: 图片URL列表

        Returns:
            加载的URL数量
        """
        self._image_url_cache = urls.copy()
        random.shuffle(self._image_url_cache)
        self._image_cursor = 0
        logger.info(f"Loaded {len(self._image_url_cache)} image URLs for sync access")
        return len(self._image_url_cache)

    def set_image_cache_max_size(self, max_size: int) -> None:
        """设置图片缓存最大数量"""
        self._image_cache_max_size = max_size if max_size > 0 else 10000

    def append_image_url(self, url: str) -> bool:
        """
        追加单个图片 URL 到缓存

        如果缓存已满，随机替换一个旧 URL

        Args:
            url: 图片URL

        Returns:
            是否成功追加
        """
        if len(self._image_url_cache) >= self._image_cache_max_size:
            replace_idx = random.randint(0, len(self._image_url_cache) - 1)
            self._image_url_cache[replace_idx] = url
        else:
            self._image_url_cache.append(url)
        return True

    def _random_keyword_sync(self) -> str:
        """
        同步获取随机关键词（用于模板渲染）

        优先从全局缓存池获取（生产者消费者模型），
        降级到本地缓存（旧逻辑）。

        Returns:
            关键词字符串
        """
        # 优先使用全局缓存池
        pool = get_keyword_cache_pool()
        if pool:
            keyword = pool.get_keyword_sync()
            if keyword:
                return keyword

        # 降级到本地缓存
        return self._get_from_local_cache(self._keyword_cache, '_keyword_cursor')

    def _get_keywords_sync(self, count: int) -> List[str]:
        """
        同步获取多个随机关键词（用于模板渲染）

        优先从全局缓存池获取，降级到本地缓存。

        Args:
            count: 需要的关键词数量

        Returns:
            关键词列表
        """
        # 优先使用全局缓存池
        pool = get_keyword_cache_pool()
        if pool:
            keywords = pool.get_keywords_sync(count)
            if keywords:
                return keywords

        # 降级到本地缓存
        return [
            self._get_from_local_cache(self._keyword_cache, '_keyword_cursor')
            for _ in range(count)
            if self._keyword_cache
        ]

    def load_keywords_sync(self, keywords: List[str]) -> int:
        """
        同步加载关键词到缓存（用于模板渲染）

        Args:
            keywords: 关键词列表

        Returns:
            加载的关键词数量
        """
        self._keyword_cache = keywords.copy()
        random.shuffle(self._keyword_cache)
        self._keyword_cursor = 0
        logger.info(f"Loaded {len(self._keyword_cache)} keywords for sync access")
        return len(self._keyword_cache)

    def set_keyword_cache_max_size(self, max_size: int) -> None:
        """设置关键词缓存最大数量"""
        self._keyword_cache_max_size = max_size if max_size > 0 else 10000

    def append_keyword(self, keyword: str) -> bool:
        """
        追加单个关键词到缓存

        如果缓存已满，随机替换一个旧关键词

        Args:
            keyword: 关键词

        Returns:
            是否成功追加
        """
        if len(self._keyword_cache) >= self._keyword_cache_max_size:
            replace_idx = random.randint(0, len(self._keyword_cache) - 1)
            self._keyword_cache[replace_idx] = keyword
        else:
            self._keyword_cache.append(keyword)
        return True

    def reset_page_state(self) -> None:
        """重置页面级状态（在每次渲染前调用）"""
        self._used_emojis.clear()
        self._encoding_count = 0
        self._preloaded_content = None  # 清空预加载的内容
        self.class_gen.reset()
        self.encoder.reset_count()

    def set_preloaded_content(self, content: str) -> None:
        """
        预加载内容供 content() 使用

        在异步环境中，应该在调用 render_template_content 之前
        预先获取内容并通过此方法设置。

        Args:
            content: 预加载的正文内容
        """
        self._preloaded_content = content

    def _prepare_render_context(
            self,
            keywords: Optional[List[str]],
            site_config: Optional[Dict[str, Any]],
            preserve_preloaded: bool = False,
            **extra_context
    ) -> Dict[str, Any]:
        """
        准备渲染上下文（内部方法）

        Args:
            keywords: 关键词列表
            site_config: 站点配置
            preserve_preloaded: 是否保留预加载的内容
            **extra_context: 额外上下文

        Returns:
            渲染上下文字典
        """
        # 保存预加载的内容
        saved_preloaded_content = None
        if preserve_preloaded:
            saved_preloaded_content = getattr(self, '_preloaded_content', None)

        # 重置页面状态
        self.reset_page_state()

        # 恢复预加载的内容
        if saved_preloaded_content:
            self._preloaded_content = saved_preloaded_content

        # 准备关键词
        if not keywords:
            keywords = self._get_keywords_sync(3)

        # 检查关键词数量是否足够，不足时使用占位符填充
        if len(keywords) < 3:
            logger.warning(f"Insufficient keywords: {len(keywords)}/3, using placeholders")
            while len(keywords) < 3:
                keywords.append(f"关键词{len(keywords) + 1}")

        # 生成标题
        title, self._used_emojis = self.title_gen.generate(
            keywords, self._used_emojis
        )

        # 构建上下文
        context = {
            'title': title,
            'baidu_push_js': site_config.get('baidu_push_js', '') if site_config else '',
            'site_id': site_config.get('id', '') if site_config else '',
            'analytics_code': site_config.get('analytics_code', '') if site_config else '',
        }
        context.update(extra_context)

        return context

    def render_page(
            self,
            template_name: str,
            keywords: Optional[List[str]] = None,
            site_config: Optional[Dict[str, Any]] = None,
            **extra_context
    ) -> str:
        """
        渲染页面

        Args:
            template_name: 模板文件名
            keywords: 目标关键词列表（3个）
            site_config: 站点配置
            **extra_context: 额外的模板上下文

        Returns:
            渲染后的HTML字符串
        """
        context = self._prepare_render_context(keywords, site_config, **extra_context)

        try:
            template = self.jinja_env.get_template(template_name)
            html = template.render(**context)

            logger.debug(
                f"Page rendered: {template_name}, "
                f"encoding_count={self._encoding_count}, "
                f"emojis_used={len(self._used_emojis)}"
            )

            return html

        except Exception as e:
            logger.error(f"Failed to render template {template_name}: {e}")
            raise

    def render_template_content(
            self,
            template_content: str,
            template_name: str = "dynamic_template",
            keywords: Optional[List[str]] = None,
            site_config: Optional[Dict[str, Any]] = None,
            **extra_context
    ) -> str:
        """
        渲染模板内容字符串（从数据库加载的模板）

        Args:
            template_content: 模板HTML内容字符串
            template_name: 模板名称（用于日志）
            keywords: 目标关键词列表（3个）
            site_config: 站点配置
            **extra_context: 额外的模板上下文

        Returns:
            渲染后的HTML字符串
        """
        # 准备上下文计时
        t_ctx_start = time_module.perf_counter()
        context = self._prepare_render_context(
            keywords, site_config, preserve_preloaded=True, **extra_context
        )
        t_ctx_end = time_module.perf_counter()

        try:
            # 模板编译计时
            t_compile_start = time_module.perf_counter()
            cache_key = hashlib.md5(template_content.encode()).hexdigest()
            compile_needed = cache_key not in self._compiled_templates

            if compile_needed:
                self._compiled_templates[cache_key] = self.jinja_env.from_string(template_content)

            template = self._compiled_templates[cache_key]
            t_compile_end = time_module.perf_counter()

            # Jinja2 渲染计时
            t_render_start = time_module.perf_counter()
            html = template.render(**context)
            t_render_end = time_module.perf_counter()

            # 详细渲染耗时日志
            logger.info(
                f"[PERF-CORE] ctx={t_ctx_end-t_ctx_start:.3f}s "
                f"compile={t_compile_end-t_compile_start:.3f}s "
                f"jinja_render={t_render_end-t_render_start:.3f}s "
                f"compile_needed={compile_needed} tpl={template_name}"
            )

            logger.debug(
                f"Page rendered from content: {template_name}, "
                f"encoding_count={self._encoding_count}, "
                f"emojis_used={len(self._used_emojis)}"
            )

            return html

        except Exception as e:
            logger.error(f"Failed to render template content {template_name}: {e}")
            raise

    def _generate_baidu_push_js(self, token: Optional[str]) -> str:
        """
        生成百度推送JS代码

        Args:
            token: 百度推送Token

        Returns:
            JS代码字符串
        """
        if not token:
            return ''

        return f'''<script>
(function(){{
    var bp = document.createElement('script');
    var curProtocol = window.location.protocol.split(':')[0];
    if (curProtocol === 'https') {{
        bp.src = 'https://zz.bdstatic.com/linksubmit/push.js';
    }} else {{
        bp.src = 'http://push.zhanzhang.baidu.com/push.js';
    }}
    var s = document.getElementsByTagName("script")[0];
    s.parentNode.insertBefore(bp, s);
}})();
</script>'''

    def get_stats(self) -> Dict[str, Any]:
        """
        获取当前统计信息

        Returns:
            统计信息字典
        """
        # 获取图片分组统计（如果可用）
        image_group = get_image_group()
        image_stats = image_group.get_stats() if image_group else {'total': 0, 'cursor': 0}

        # 获取关键词分组统计（如果可用）
        keyword_group = get_keyword_group()
        keyword_stats = keyword_group.get_stats() if keyword_group else {'total': 0, 'cursor': 0}

        return {
            'encoding_count': self._encoding_count,
            'emojis_used': len(self._used_emojis),
            'keywords_total': keyword_stats.get('total', 0),
            'images_total': image_stats.get('total', 0),
            'keyword_cache_size': len(self._keyword_cache),
            'image_cache_size': len(self._image_url_cache),
            'keyword_group_stats': keyword_stats,
            'image_group_stats': image_stats,
        }

    def reload_caches(self) -> Dict[str, Dict[str, int]]:
        """
        清空本地缓存

        注意：关键词分组和图片分组需要通过异步API重载:
        - /api/keywords/reload
        - /api/images/urls/reload

        Returns:
            清空统计信息
        """
        return {
            'keyword_cache_cleared': self._clear_keyword_cache(),
            'image_cache_cleared': self._clear_image_cache()
        }

    def _clear_local_cache(self, cache: list, cursor_attr: str) -> int:
        """清空本地缓存（通用方法）"""
        count = len(cache)
        cache.clear()
        setattr(self, cursor_attr, 0)
        return count

    def _clear_keyword_cache(self) -> int:
        """清空本地关键词缓存"""
        return self._clear_local_cache(self._keyword_cache, '_keyword_cursor')

    def _clear_image_cache(self) -> int:
        """清空本地图片URL缓存"""
        return self._clear_local_cache(self._image_url_cache, '_image_cursor')


# 全局SEOCore实例
_core: Optional[SEOCore] = None


def get_seo_core() -> SEOCore:
    """
    获取全局SEOCore实例

    Returns:
        SEOCore实例
    """
    global _core
    if _core is None:
        _core = SEOCore()
    return _core


def init_seo_core(
        template_dir: str = "./templates",
        emoji_file: Optional[str] = None,
        encoding_ratio: float = 0.5
) -> SEOCore:
    """
    初始化全局SEOCore实例

    关键词和图片URL现在通过全局异步分组管理器加载:
    - 关键词: init_keyword_group() + get_keyword_group()
    - 图片: init_image_group() + get_image_group()

    模板渲染需要预加载同步缓存:
    - core.load_keywords_sync(keywords)
    - core.load_image_urls_sync(urls)

    Args:
        template_dir: 模板目录
        emoji_file: Emoji数据文件
        encoding_ratio: 编码混合比例

    Returns:
        初始化后的SEOCore实例
    """
    global _core
    _core = SEOCore(
        template_dir=template_dir,
        encoding_ratio=encoding_ratio,
        emoji_file=emoji_file
    )

    return _core


def render_page(
        template_name: str,
        keywords: Optional[List[str]] = None,
        **context
) -> str:
    """
    快捷函数 - 渲染页面

    Args:
        template_name: 模板名
        keywords: 关键词
        **context: 额外上下文

    Returns:
        HTML字符串
    """
    return get_seo_core().render_page(template_name, keywords, **context)
