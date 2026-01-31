# 移除生成器管理功能设计

## 背景

"生成器管理"功能允许在后台管理界面创建和编辑动态生成器代码，存储在数据库中。但经检查发现：

1. 后端的动态生成器代码（`worker/core/generators/dynamic.py` 和 `manager.py`）从未被实际调用
2. 这是一个废弃的功能
3. 现在可以通过"数据处理"页面直接编辑 Worker 代码

因此决定完全移除该功能。

## 设计目标

完全移除生成器管理功能，包括前端界面、后端 API、相关废弃代码和数据库表。

## 变更清单

### 前端删除

| 文件/配置 | 操作 |
|-----------|------|
| `web/src/views/generators/GeneratorList.vue` | 删除 |
| `web/src/views/generators/GeneratorEdit.vue` | 删除 |
| `web/src/views/generators/` 目录 | 删除 |
| `web/src/api/generators.ts` | 删除 |
| `web/src/router/index.ts` | 移除 generators 相关路由 |
| `web/src/components/Layout/MainLayout.vue` | 移除"生成器管理"菜单项 |

### 后端删除

| 文件/配置 | 操作 |
|-----------|------|
| `api/internal/handler/generators.go` | 删除 |
| `api/internal/handler/router.go` | 移除 generators 路由组（286-299行） |
| `worker/core/generators/dynamic.py` | 删除 |
| `worker/core/generators/manager.py` | 删除 |
| `worker/core/generators/__init__.py` | 更新，移除废弃导出 |

### 数据库

| 表 | 操作 |
|-----|------|
| `generators` | 删除 |

### 保留

| 文件 | 原因 |
|------|------|
| `worker/core/generators/interface.py` | `IAnnotator` 接口被 `pinyin_annotator.py` 使用 |
| `api/internal/handler/generator_queue.go` | 文章处理队列功能，与生成器管理无关 |

## 菜单变化

```
变更前                    变更后
├─ 数据抓取              ├─ 数据抓取
├─ 抓取统计              ├─ 抓取统计
├─ 数据处理              ├─ 数据处理
├─ 生成器管理       →    （删除）
├─ 蜘蛛日志              ├─ 蜘蛛日志
```
