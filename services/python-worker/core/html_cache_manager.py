"""
HTML缓存管理器模块

提供HTML页面的文件级缓存管理，使用哈希分层目录结构。

主要功能:
- get(): 获取缓存
- set(): 设置缓存
- delete(): 删除缓存
- clear(): 清空缓存

特点:
- 基于文件系统的持久化缓存
- 哈希分层目录结构（支持百万级缓存）
- 无内存索引，路径直接计算
- 异步IO操作
"""
import os
import json
import hashlib
import shutil
from pathlib import Path
from typing import Dict, Optional, Any, List
from datetime import datetime

import aiofiles
from loguru import logger


class HTMLCacheManager:
    """
    HTML缓存管理器

    管理生成的HTML页面缓存，使用哈希分层目录结构。

    目录结构:
        html_cache/
        └── example.com/
            └── a1/                    # hash[0:2]
                └── b2/                # hash[2:4]
                    ├── index.html
                    ├── page.html
                    └── article/123.html

    Attributes:
        cache_dir: 缓存目录
        max_size_gb: 最大缓存大小（GB）

    Example:
        >>> cache = HTMLCacheManager("./html_cache")
        >>> await cache.set("example.com", "/page.html", "<html>...</html>")
        >>> html = await cache.get("example.com", "/page.html")
    """

    def __init__(
        self,
        cache_dir: str = "./html_cache",
        max_size_gb: float = 10.0,
        enable_gzip: bool = True,
        nginx_mode: bool = False
    ):
        """
        初始化缓存管理器

        Args:
            cache_dir: 缓存目录
            max_size_gb: 最大缓存大小（GB）
            enable_gzip: 是否启用gzip压缩（哈希分层模式下忽略）
            nginx_mode: Nginx直服模式（哈希分层模式下忽略）
        """
        self.cache_dir = Path(cache_dir)
        self.max_size_bytes = int(max_size_gb * 1024 * 1024 * 1024)

        # 简化统计（仅保留命中/未命中计数）
        self._stats_hit = 0
        self._stats_miss = 0

        # 确保缓存目录存在
        self.cache_dir.mkdir(parents=True, exist_ok=True)

        # 创建元数据目录
        (self.cache_dir / "_meta").mkdir(parents=True, exist_ok=True)

        logger.info(
            f"HTMLCacheManager initialized: dir={cache_dir}, "
            f"max_size={max_size_gb}GB (hash-layered mode)"
        )

    def _generate_cache_key(self, domain: str, path: str) -> str:
        """生成缓存键"""
        raw_key = f"{domain}:{path}"
        return hashlib.md5(raw_key.encode()).hexdigest()

    def _get_path_hash(self, path: str) -> str:
        """获取路径的哈希值"""
        return hashlib.md5(path.encode()).hexdigest()

    def _normalize_path(self, path: str) -> str:
        """
        规范化URL路径用于文件存储

        Args:
            path: URL路径 (如 "/", "/page.html", "/article/123")

        Returns:
            规范化后的文件路径 (如 "index.html", "page.html", "article/123.html")
        """
        # 移除开头的斜杠
        path = path.lstrip('/')

        # 空路径或根路径转为 index.html
        if not path or path == '/':
            return 'index.html'

        # 如果路径没有扩展名，添加 .html
        if '.' not in path.split('/')[-1]:
            path = f"{path}.html"

        return path

    def _get_cache_path(self, domain: str, path: str) -> Path:
        """
        获取缓存文件路径（哈希分层）

        路径结构: {cache_dir}/{domain}/{hash[0:2]}/{hash[2:4]}/{normalized_path}

        Args:
            domain: 域名
            path: URL路径

        Returns:
            缓存文件的Path对象
        """
        normalized = self._normalize_path(path)
        path_hash = self._get_path_hash(path)

        return self.cache_dir / domain / path_hash[:2] / path_hash[2:4] / normalized

    def _get_meta_path(self, domain: str, path: str) -> Path:
        """
        获取元数据文件路径（哈希分层）

        路径结构: {cache_dir}/_meta/{domain}/{hash[0:2]}/{hash[2:4]}/{cache_key}.json

        Args:
            domain: 域名
            path: URL路径

        Returns:
            元数据文件的Path对象
        """
        cache_key = self._generate_cache_key(domain, path)
        path_hash = self._get_path_hash(path)

        return self.cache_dir / "_meta" / domain / path_hash[:2] / path_hash[2:4] / f"{cache_key}.json"

    async def get(
        self,
        domain: str,
        path: str
    ) -> Optional[str]:
        """
        获取缓存的HTML

        直接读取文件，无需索引。

        Args:
            domain: 域名
            path: 请求路径

        Returns:
            缓存的HTML内容，如果不存在则返回None
        """
        cache_path = self._get_cache_path(domain, path)

        if not cache_path.exists():
            self._stats_miss += 1
            return None

        try:
            async with aiofiles.open(cache_path, 'r', encoding='utf-8') as f:
                html = await f.read()

            self._stats_hit += 1
            return html

        except Exception as e:
            logger.error(f"Failed to read cache: {e}")
            self._stats_miss += 1
            return None

    async def set(
        self,
        domain: str,
        path: str,
        html: str
    ) -> bool:
        """
        设置缓存

        直接写入文件，自动创建目录结构。

        Args:
            domain: 域名
            path: 请求路径
            html: HTML内容

        Returns:
            是否成功
        """
        cache_path = self._get_cache_path(domain, path)
        meta_path = self._get_meta_path(domain, path)

        try:
            # 确保目录存在
            cache_path.parent.mkdir(parents=True, exist_ok=True)
            meta_path.parent.mkdir(parents=True, exist_ok=True)

            # 写入HTML文件
            async with aiofiles.open(cache_path, 'w', encoding='utf-8') as f:
                await f.write(html)

            # 保存元数据
            html_bytes = html.encode('utf-8')
            meta_data = {
                'key': self._generate_cache_key(domain, path),
                'domain': domain,
                'path': path,
                'size': len(html_bytes),
                'created_at': datetime.now().isoformat()
            }
            async with aiofiles.open(meta_path, 'w', encoding='utf-8') as f:
                await f.write(json.dumps(meta_data, ensure_ascii=False))

            return True

        except Exception as e:
            logger.error(f"Failed to set cache: {e}")
            return False

    async def delete(
        self,
        domain: str,
        path: str
    ) -> bool:
        """
        删除缓存

        Args:
            domain: 域名
            path: 请求路径

        Returns:
            是否成功
        """
        cache_path = self._get_cache_path(domain, path)
        meta_path = self._get_meta_path(domain, path)

        try:
            if cache_path.exists():
                cache_path.unlink()
            if meta_path.exists():
                meta_path.unlink()
            return True

        except Exception as e:
            logger.error(f"Failed to delete cache: {e}")
            return False

    def _count_files(self, directory: Path) -> int:
        """统计目录下的文件数量"""
        if not directory.exists():
            return 0
        try:
            return sum(1 for _ in directory.rglob("*.html"))
        except Exception:
            return 0

    def _get_dir_size(self, directory: Path) -> int:
        """获取目录大小（字节）"""
        if not directory.exists():
            return 0
        try:
            return sum(f.stat().st_size for f in directory.rglob("*") if f.is_file())
        except Exception:
            return 0

    async def clear(self, domain: Optional[str] = None) -> int:
        """
        清空缓存

        Args:
            domain: 可选的域名，只清空指定域名

        Returns:
            清空的条目数
        """
        try:
            if domain:
                # 清空指定域名
                domain_dir = self.cache_dir / domain
                meta_dir = self.cache_dir / "_meta" / domain
                count = self._count_files(domain_dir)
                if domain_dir.exists():
                    shutil.rmtree(domain_dir)
                if meta_dir.exists():
                    shutil.rmtree(meta_dir)
            else:
                # 清空全部
                count = self._count_files(self.cache_dir)
                shutil.rmtree(self.cache_dir)
                self.cache_dir.mkdir(parents=True, exist_ok=True)
                (self.cache_dir / "_meta").mkdir(parents=True, exist_ok=True)

            logger.info(f"Cleared {count} cache files")
            return count
        except Exception as e:
            logger.error(f"Failed to clear cache: {e}")
            return 0

    def exists(self, domain: str, path: str) -> bool:
        """
        检查缓存是否存在

        直接检查文件是否存在。

        Args:
            domain: 域名
            path: 请求路径

        Returns:
            缓存是否存在
        """
        cache_path = self._get_cache_path(domain, path)
        return cache_path.exists()

    def get_stats(self) -> Dict[str, Any]:
        """
        获取缓存统计（按需扫描）

        Returns:
            统计信息字典
        """
        total_requests = self._stats_hit + self._stats_miss
        hit_rate = self._stats_hit / max(1, total_requests) * 100

        # 按需统计文件数量和大小
        total_entries = 0
        total_size_bytes = 0
        entries_by_domain = {}

        try:
            for domain_dir in self.cache_dir.iterdir():
                if domain_dir.is_dir() and domain_dir.name != "_meta":
                    domain = domain_dir.name
                    count = self._count_files(domain_dir)
                    size = self._get_dir_size(domain_dir)
                    entries_by_domain[domain] = count
                    total_entries += count
                    total_size_bytes += size
        except Exception as e:
            logger.warning(f"Failed to scan cache stats: {e}")

        return {
            'total_entries': total_entries,
            'total_size_mb': round(total_size_bytes / 1024 / 1024, 2),
            'hit_count': self._stats_hit,
            'miss_count': self._stats_miss,
            'hit_rate': round(hit_rate, 1),
            'entries_by_domain': entries_by_domain,
        }

    def list_entries(
        self,
        domain: Optional[str] = None,
        offset: int = 0,
        limit: int = 100
    ) -> List[Dict[str, Any]]:
        """
        列出缓存条目（按需扫描）

        Args:
            domain: 可选的域名筛选
            offset: 偏移量
            limit: 返回数量

        Returns:
            条目列表
        """
        entries = []

        try:
            if domain:
                # 扫描指定域名的元数据
                meta_dir = self.cache_dir / "_meta" / domain
                if meta_dir.exists():
                    meta_files = list(meta_dir.rglob("*.json"))
            else:
                # 扫描所有元数据
                meta_dir = self.cache_dir / "_meta"
                if meta_dir.exists():
                    meta_files = list(meta_dir.rglob("*.json"))
                else:
                    meta_files = []

            # 按修改时间排序
            meta_files.sort(key=lambda f: f.stat().st_mtime, reverse=True)

            # 分页
            for meta_file in meta_files[offset:offset + limit]:
                try:
                    with open(meta_file, 'r', encoding='utf-8') as f:
                        data = json.load(f)
                    entries.append({
                        'key': data.get('key', ''),
                        'domain': data.get('domain', ''),
                        'path': data.get('path', ''),
                        'size': data.get('size', 0),
                        'created_at': data.get('created_at', ''),
                    })
                except Exception:
                    continue

        except Exception as e:
            logger.warning(f"Failed to list cache entries: {e}")

        return entries


# 全局缓存管理器实例
_cache_manager: Optional[HTMLCacheManager] = None


def get_cache_manager() -> HTMLCacheManager:
    """获取全局缓存管理器"""
    global _cache_manager
    if _cache_manager is None:
        _cache_manager = HTMLCacheManager()
    return _cache_manager


def init_cache_manager(
    cache_dir: str = "./html_cache",
    max_size_gb: float = 10.0,
    enable_gzip: bool = True,
    nginx_mode: bool = False
) -> HTMLCacheManager:
    """
    初始化全局缓存管理器

    Args:
        cache_dir: 缓存目录
        max_size_gb: 最大大小GB
        enable_gzip: 是否启用gzip（哈希分层模式下忽略）
        nginx_mode: Nginx直服模式（哈希分层模式下忽略）

    Returns:
        HTMLCacheManager实例
    """
    global _cache_manager
    _cache_manager = HTMLCacheManager(
        cache_dir=cache_dir,
        max_size_gb=max_size_gb,
        enable_gzip=enable_gzip,
        nginx_mode=nginx_mode
    )
    return _cache_manager
