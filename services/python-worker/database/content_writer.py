# -*- coding: utf-8 -*-
"""
内容批量写入器

提供 titles 和 contents 表的批量写入功能。
"""

from typing import List, Optional
from loguru import logger


class ContentWriter:
    """
    内容批量写入器

    功能：
    - 批量写入标题（titles 表）
    - 批量写入正文（contents 表）
    - 自动管理 batch_id
    """

    def __init__(self, db_pool):
        """
        初始化写入器

        Args:
            db_pool: MySQL 连接池
        """
        self.db_pool = db_pool

    async def get_next_batch_id(self, table: str, group_id: int) -> int:
        """
        获取下一个 batch_id

        Args:
            table: 表名（titles 或 contents）
            group_id: 分组ID

        Returns:
            下一个 batch_id
        """
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        f"""
                        SELECT COALESCE(MAX(batch_id), 0) + 1
                        FROM {table}
                        WHERE group_id = %s
                        """,
                        (group_id,)
                    )
                    row = await cursor.fetchone()
                    return row[0] if row else 1

        except Exception as e:
            logger.error(f"Failed to get next batch_id for {table}: {e}")
            return 1

    async def write_title(self, title: str, group_id: int = 1) -> bool:
        """
        写入单个标题

        Args:
            title: 标题内容
            group_id: 分组ID

        Returns:
            是否成功
        """
        if not title or not title.strip():
            return False

        try:
            batch_id = await self.get_next_batch_id('titles', group_id)

            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        """
                        INSERT IGNORE INTO titles (title, group_id, batch_id)
                        VALUES (%s, %s, %s)
                        """,
                        (title.strip(), group_id, batch_id)
                    )
                await conn.commit()
            return True

        except Exception as e:
            logger.error(f"Failed to write title: {e}")
            return False

    async def write_titles_batch(
        self,
        titles: List[str],
        group_id: int = 1
    ) -> int:
        """
        批量写入标题

        使用 INSERT IGNORE 避免重复。

        Args:
            titles: 标题列表
            group_id: 分组ID

        Returns:
            成功写入的数量
        """
        if not titles:
            return 0

        # 过滤空标题
        valid_titles = [t.strip() for t in titles if t and t.strip()]
        if not valid_titles:
            return 0

        try:
            batch_id = await self.get_next_batch_id('titles', group_id)
            values = [(t, group_id, batch_id) for t in valid_titles]

            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.executemany(
                        """
                        INSERT IGNORE INTO titles (title, group_id, batch_id)
                        VALUES (%s, %s, %s)
                        """,
                        values
                    )
                    affected = cursor.rowcount
                await conn.commit()

            logger.debug(f"Wrote {affected} titles (batch_id={batch_id})")
            return affected

        except Exception as e:
            logger.error(f"Failed to batch write titles: {e}")
            return 0

    async def write_content(
        self,
        content: str,
        group_id: int = 1,
        batch_id: Optional[int] = None
    ) -> bool:
        """
        写入单条正文

        Args:
            content: 正文内容
            group_id: 分组ID
            batch_id: 批次ID（可选，不传则自动获取）

        Returns:
            是否成功
        """
        if not content or not content.strip():
            return False

        try:
            if batch_id is None:
                batch_id = await self.get_next_batch_id('contents', group_id)

            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.execute(
                        """
                        INSERT INTO contents (content, group_id, batch_id)
                        VALUES (%s, %s, %s)
                        """,
                        (content, group_id, batch_id)
                    )
                await conn.commit()
            return True

        except Exception as e:
            logger.error(f"Failed to write content: {e}")
            return False

    async def write_contents_batch(
        self,
        contents: List[str],
        group_id: int = 1
    ) -> int:
        """
        批量写入正文

        Args:
            contents: 正文列表
            group_id: 分组ID

        Returns:
            成功写入的数量
        """
        if not contents:
            return 0

        # 过滤空内容
        valid_contents = [c for c in contents if c and c.strip()]
        if not valid_contents:
            return 0

        try:
            batch_id = await self.get_next_batch_id('contents', group_id)
            values = [(c, group_id, batch_id) for c in valid_contents]

            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    await cursor.executemany(
                        """
                        INSERT INTO contents (content, group_id, batch_id)
                        VALUES (%s, %s, %s)
                        """,
                        values
                    )
                    affected = cursor.rowcount
                await conn.commit()

            logger.debug(f"Wrote {affected} contents (batch_id={batch_id})")
            return affected

        except Exception as e:
            logger.error(f"Failed to batch write contents: {e}")
            return 0

    async def write_contents_with_items(
        self,
        items: List[dict]
    ) -> int:
        """
        批量写入正文（支持不同分组）

        Args:
            items: [{'content': str, 'group_id': int}, ...]

        Returns:
            成功写入的数量
        """
        if not items:
            return 0

        # 按 group_id 分组
        groups = {}
        for item in items:
            content = item.get('content')
            group_id = item.get('group_id', 1)

            if content and content.strip():
                if group_id not in groups:
                    groups[group_id] = []
                groups[group_id].append(content)

        total = 0
        for group_id, contents in groups.items():
            count = await self.write_contents_batch(contents, group_id)
            total += count

        return total

    async def get_stats(self) -> dict:
        """获取统计信息"""
        try:
            async with self.db_pool.acquire() as conn:
                async with conn.cursor() as cursor:
                    # 标题数量
                    await cursor.execute("SELECT COUNT(*) FROM titles")
                    titles_count = (await cursor.fetchone())[0]

                    # 正文数量
                    await cursor.execute("SELECT COUNT(*) FROM contents")
                    contents_count = (await cursor.fetchone())[0]

            return {
                'titles_count': titles_count,
                'contents_count': contents_count
            }

        except Exception as e:
            logger.error(f"Failed to get stats: {e}")
            return {'titles_count': 0, 'contents_count': 0}
