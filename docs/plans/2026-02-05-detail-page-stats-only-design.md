# 详情页统计优化设计

## 背景

当前爬虫统计（total/completed/failed/retried）包含所有请求类型（列表页、详情页、初始请求等）。用户只关心详情页的抓取情况，其他页面的统计数据没有实际意义。

## 需求

- 只统计 `callback='parse_detail'` 的详情页请求
- 移除对列表页、初始请求等其他请求的统计
- 前端显示保持不变（标签文字不改）

## 设计方案

### 修改范围

仅修改 `content_worker/core/crawler/request_queue.py`，Go API 和前端无需改动。

### 修改点

| 方法 | 行号 | 当前逻辑 | 修改后 |
|------|------|----------|--------|
| `push()` | 134 | 无条件 `hincrby('total', 1)` | 仅当 `callback_name == 'parse_detail'` 时执行 |
| `complete(success=True)` | 218 | 无条件 `hincrby('completed', 1)` | 同上 |
| `complete(success=False)` | 221 | 无条件 `hincrby('failed', 1)` | 同上 |
| `retry()` | 250 | 无条件 `hincrby('retried', 1)` | 同上 |
| `recover_timeout()` | 368, 372 | 无条件更新 retried/failed | 同上 |

### 实现方式

在每个统计更新点添加条件判断：

```python
if request.callback_name == 'parse_detail':
    await self.redis.hincrby(self._key_stats, 'xxx', 1)
```

### 数据流

```
Request 入队 (push)
    ↓
callback_name == 'parse_detail'?
    ↓ Yes: hincrby total
    ↓ No: 不统计

Request 完成 (complete)
    ↓
callback_name == 'parse_detail'?
    ↓ Yes: hincrby completed/failed
    ↓ No: 不统计

Request 重试 (retry/recover_timeout)
    ↓
callback_name == 'parse_detail'?
    ↓ Yes: hincrby retried
    ↓ No: 不统计
```

## 影响范围

- Python 爬虫端：`request_queue.py`（1 个文件，5 处修改）
- Go API 端：无需改动（Redis 数据格式不变）
- 前端：无需改动
- 数据库：无需改动

## 边界情况

1. **初始请求**：`start_requests` 产生的请求通常 callback 是 `parse` 或 None，不会被统计
2. **列表页请求**：callback 通常是 `parse_list` 或其他名称，不会被统计
3. **已有数据**：修改后新产生的统计只包含详情页，历史数据不受影响（但含义会混淆，建议在项目重置时清理）
