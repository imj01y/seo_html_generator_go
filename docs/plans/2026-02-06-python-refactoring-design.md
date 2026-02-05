# Python 代码重构设计文档

## 概述

对 `content_worker` 目录进行重构，目标是提高可维护性。采用渐进式重构，从底层开始。

## 重构范围

- 删除死代码
- 提取硬编码配置
- 拆分超长函数

## 阶段1：删除死代码

### 已删除文件

| 文件 | 行数 | 原因 |
|------|------|------|
| `database/content_writer.py` | 282 | 完全未被导入使用 |
| `core/crawler/log_manager.py` | 259 | 已被 RealtimeLogger 替代 |
| `core/logging.py` | - | 旧日志模块，已废弃 |
| `core/workers/logger_protocol.py` | - | 日志协议，已废弃 |

### 待删除文件

| 文件 | 行数 | 原因 |
|------|------|------|
| `core/auth.py` | ~280 | 认证模块，当前未被调用 |
| `core/lifecycle.py` | ~70 | FastAPI 生命周期，Python 不提供 HTTP API |
| `core/initializers.py` | ~100 | 只被 lifecycle.py 使用 |

### 需要更新的文件

**`core/__init__.py`** - 移除对已删除模块的导入：

```python
# 删除以下行
from .auth import ensure_default_admin
from .initializers import init_components

# 更新 __all__，移除：
# 'ensure_default_admin',
# 'init_components',
```

### 清理统计

- 总计清理：约 **990+ 行** 死代码
- Python 项目当前不对外提供 HTTP API，只是后台 Worker

---

## 阶段2：配置层重构

### 目标

将分散在各处的硬编码值统一到 YAML 配置文件，使用 Dynaconf 管理。

### 新建文件

**`config/settings.yaml`**

```yaml
default:
  redis:
    queues:
      pending_articles: "pending:articles"
      retry_articles: "pending:articles:retry"
      dead_articles: "pending:articles:dead"
    channels:
      pool_reload: "pool:reload"
      crawler_commands: "crawler:commands"
      generator_commands: "generator:commands"

  intervals:
    stats_update: 5
    bloom_save: 300
    heartbeat: 30

  batch:
    default_size: 100
    max_retry: 3
```

**`core/settings.py`**

```python
from dynaconf import Dynaconf

settings = Dynaconf(
    settings_files=["config/settings.yaml"],
    environments=True,
    env_switcher="WORKER_ENV",
    envvar_prefix="WORKER",
)
```

### 使用方式

```python
from core.settings import settings

# 直接访问
queue = settings.redis.queues.pending_articles
interval = settings.intervals.stats_update

# 环境变量覆盖
# WORKER_REDIS__QUEUES__PENDING_ARTICLES=custom:queue
```

### 需要修改的文件

| 文件 | 修改点 |
|------|--------|
| `generator_worker.py` | 替换队列名硬编码（3处） |
| `generator_manager.py` | 替换频道名和间隔（4处） |
| `pool_reloader.py` | 替换频道名（1处） |
| `bloom_dedup.py` | 替换保存间隔（1处） |
| `command_listener.py` | 替换频道名（1处） |

---

## 阶段3：业务逻辑层重构

### 3.1 拆分 `CommandListener.run_project()`

当前 162 行，职责混杂。

**拆分方案**：

```python
async def run_project(self, project_id: int):
    """运行爬虫项目（主入口，只做流程编排）"""
    log = RealtimeLogger(self.rdb, f"spider:logs:project_{project_id}")
    items_count = 0

    try:
        # 更新状态（内联）
        await self.rdb.set(
            f"spider:status:{project_id}",
            json.dumps({"status": "running", "started_at": datetime.now().isoformat()})
        )

        # 加载项目（拆分为独立方法）
        project = await self._load_project(project_id, log)
        if not project:
            return

        # 执行并处理数据（拆分为独立方法）
        items_count = await self._run_and_process(project, log)

        # 更新成功统计（内联）
        await execute_query(
            """
            UPDATE spider_projects SET
                status = 'idle',
                last_run_at = NOW(),
                last_run_items = %s,
                last_error = NULL,
                total_runs = total_runs + 1,
                total_items = total_items + %s
            WHERE id = %s
            """,
            (items_count, items_count, project_id),
            commit=True
        )
        log.info(f"任务完成：共 {items_count} 条数据")

    except asyncio.CancelledError:
        log.info("任务已被取消")
        await execute_query(
            "UPDATE spider_projects SET status = 'idle', last_error = %s WHERE id = %s",
            ("任务被取消", project_id),
            commit=True
        )
    except Exception as e:
        log.error(f"任务异常: {e}")
        await execute_query(
            "UPDATE spider_projects SET status = 'error', last_error = %s WHERE id = %s",
            (str(e), project_id),
            commit=True
        )
    finally:
        await log.end()
        await self.rdb.set(
            f"spider:status:{project_id}",
            json.dumps({"status": "idle"})
        )
        self.running_tasks.pop(project_id, None)


async def _load_project(self, project_id: int, log) -> Optional[dict]:
    """加载项目配置和模块"""
    from core.crawler.project_loader import ProjectLoader

    log.info("正在加载项目...")

    row = await fetch_one(
        "SELECT id, name, entry_file, config, concurrency, output_group_id FROM spider_projects WHERE id = %s",
        (project_id,)
    )
    if not row:
        log.error("项目不存在")
        return None

    config = json.loads(row['config']) if row['config'] else {}

    loader = ProjectLoader(project_id)
    modules = await loader.load()
    log.info(f"已加载 {len(modules)} 个模块")

    # 清除旧的停止信号
    await self.rdb.delete(f"spider_project:{project_id}:stop")

    return {
        "id": project_id,
        "config": config,
        "modules": modules,
        "concurrency": row.get('concurrency', 3),
        "group_id": row['output_group_id'],
    }


async def _run_and_process(self, project: dict, log) -> int:
    """执行爬虫并处理数据"""
    from core.crawler.project_runner import ProjectRunner

    runner = ProjectRunner(
        project_id=project["id"],
        modules=project["modules"],
        config=project["config"],
        redis=self.rdb,
        db_pool=get_db_pool(),
        concurrency=project["concurrency"],
    )

    log.info("开始执行 Spider...")

    items_count = 0
    stop_key = f"spider_project:{project['id']}:stop"

    async for item in runner.run():
        # 检查停止信号
        if await self.rdb.get(stop_key):
            log.info("收到停止信号，任务终止")
            await self.rdb.delete(stop_key)
            break

        # 处理数据项
        count = await self._process_item(item, project["group_id"], project["id"], log)
        items_count += count

        if items_count > 0 and items_count % 10 == 0:
            log.info(f"已抓取 {items_count} 条数据")

    return items_count


async def _process_item(self, item: dict, group_id: int, project_id: int, log) -> int:
    """处理单个数据项（路由到 keywords/images/article）"""
    item_type = item.get('type', 'article')

    try:
        if item_type == 'keywords':
            keywords = item.get('keywords', [])
            target_group = item.get('group_id', group_id)
            if keywords:
                added = await self._batch_insert_keywords(keywords, target_group)
                log.info(f"关键词写入: 新增 {added}, 跳过 {len(keywords) - added}")
                if added > 0:
                    await self._publish_stats(project_id, added)
                return added

        elif item_type == 'images':
            urls = item.get('urls', [])
            target_group = item.get('group_id', group_id)
            if urls:
                added = await self._batch_insert_images(urls, target_group)
                log.info(f"图片写入: 新增 {added}, 跳过 {len(urls) - added}")
                if added > 0:
                    await self._publish_stats(project_id, added)
                return added

        else:
            # article 类型
            if item.get('title') and item.get('content'):
                target_group = item.get('group_id', group_id)
                article_id = await insert("original_articles", {
                    "group_id": target_group,
                    "source_id": project_id,
                    "source_url": item.get('source_url'),
                    "title": item['title'][:500],
                    "content": item['content'],
                })

                await self._publish_stats(project_id, 1)

                if article_id:
                    try:
                        await self.rdb.lpush("pending:articles", article_id)
                    except Exception as queue_err:
                        log.warning(f"推送到待处理队列失败: {queue_err}")

                return 1

    except Exception as e:
        if 'Duplicate' in str(e):
            log.warning("数据重复，已跳过")
        else:
            log.error(f"保存数据失败: {e}")

    return 0
```

### 3.2 复用 `_load_project()` 到 `test_project()`

`test_project()` 与 `run_project()` 有约 25 行重复代码，可复用 `_load_project()` 方法。

### 3.3 `GeneratorManager` 评估

| 指标 | 数值 |
|------|------|
| 总行数 | 410 行 |
| 方法数 | 14 个 |
| 平均方法行数 | ~29 行 |

**结论**：不需要大幅重构，职责虽多但相关，方法行数合理。

---

## 重构优先级

| 阶段 | 内容 | 优先级 |
|------|------|--------|
| 阶段1 | 删除死代码 | 高 |
| 阶段2 | Dynaconf 配置管理 | 中 |
| 阶段3 | 拆分 `run_project()` | 中 |

---

## 不做的事情

以下经过评估后决定不做：

1. **通用工具层抽象**
   - Redis 订阅基类：3 处使用但细节差异大，抽象收益有限
   - 批量写入工具：2 处使用但插入逻辑不同（executemany vs 逐条）

2. **数据访问层拆分**
   - `database/db.py` 虽有 539 行，但职责内聚，拆分反而增加复杂性

3. **`GeneratorManager` 重构**
   - 410 行 / 14 个方法，平均行数合理，不需要拆分
