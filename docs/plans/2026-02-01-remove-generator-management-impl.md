# 移除生成器管理功能 - 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 完全移除废弃的生成器管理功能，包括前端、后端和数据库表。

**Architecture:** 按顺序删除前端文件、后端文件、更新路由和菜单配置，最后删除数据库表。

**Tech Stack:** Vue 3, Go/Gin, MySQL

---

## Task 1: 删除前端生成器页面和 API

**Files:**
- Delete: `web/src/views/generators/GeneratorList.vue`
- Delete: `web/src/views/generators/GeneratorEdit.vue`
- Delete: `web/src/api/generators.ts`

**Step 1: 删除文件**

删除以下文件和目录：
- `web/src/views/generators/` 整个目录
- `web/src/api/generators.ts`

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: 删除生成器管理前端页面和API"
```

---

## Task 2: 更新前端路由配置

**Files:**
- Modify: `web/src/router/index.ts`

**Step 1: 移除 generators 路由**

删除以下路由配置（约100-109行）：
```typescript
{
  path: 'generators',
  name: 'Generators',
  component: () => import('@/views/generators/GeneratorList.vue'),
  meta: { title: '生成器管理', icon: 'Cpu' }
},
{
  path: 'generators/edit/:id?',
  name: 'GeneratorEdit',
  component: () => import('@/views/generators/GeneratorEdit.vue'),
  meta: { title: '编辑生成器', icon: 'Cpu', hidden: true }
},
```

**Step 2: Commit**

```bash
git add web/src/router/index.ts
git commit -m "refactor(router): 移除生成器管理路由"
```

---

## Task 3: 更新前端菜单配置

**Files:**
- Modify: `web/src/components/Layout/MainLayout.vue`

**Step 1: 移除菜单项**

在 menuItems 数组中删除：
```typescript
{ path: '/generators', title: '生成器管理', icon: 'Promotion' },
```

**Step 2: 移除 activeMenu 处理**

删除以下代码（约160行）：
```typescript
if (path.startsWith('/generators/')) return '/generators'
```

**Step 3: Commit**

```bash
git add web/src/components/Layout/MainLayout.vue
git commit -m "refactor(menu): 移除生成器管理菜单项"
```

---

## Task 4: 删除后端生成器 API

**Files:**
- Delete: `api/internal/handler/generators.go`

**Step 1: 删除文件**

删除 `api/internal/handler/generators.go`

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: 删除生成器管理后端API"
```

---

## Task 5: 更新后端路由配置

**Files:**
- Modify: `api/internal/handler/router.go`

**Step 1: 移除 generators 路由组**

删除以下代码（约286-299行）：
```go
generatorsHandler := &GeneratorsHandler{}
genRoutes := r.Group("/api/generators")
{
    genRoutes.Use(authMiddleware())
    genRoutes.GET("", generatorsHandler.List)
    genRoutes.POST("", generatorsHandler.Create)
    genRoutes.GET("/templates/list", generatorsHandler.GetTemplates)
    genRoutes.POST("/test", generatorsHandler.Test)
    genRoutes.GET("/:id", generatorsHandler.Get)
    genRoutes.PUT("/:id", generatorsHandler.Update)
    genRoutes.DELETE("/:id", generatorsHandler.Delete)
    genRoutes.POST("/:id/set-default", generatorsHandler.SetDefault)
    genRoutes.POST("/:id/toggle", generatorsHandler.Toggle)
    genRoutes.POST("/:id/reload", generatorsHandler.Reload)
}
```

**Step 2: Commit**

```bash
git add api/internal/handler/router.go
git commit -m "refactor(router): 移除生成器管理API路由"
```

---

## Task 6: 删除 Worker 废弃代码

**Files:**
- Delete: `worker/core/generators/dynamic.py`
- Delete: `worker/core/generators/manager.py`
- Modify: `worker/core/generators/__init__.py`

**Step 1: 删除废弃文件**

删除：
- `worker/core/generators/dynamic.py`
- `worker/core/generators/manager.py`

**Step 2: 更新 __init__.py**

将 `worker/core/generators/__init__.py` 内容改为：
```python
# -*- coding: utf-8 -*-
"""
生成器接口定义

仅保留 IAnnotator 接口供 pinyin_annotator 使用。
"""

from .interface import (
    IAnnotator,
    GeneratorContext,
    GeneratorResult,
    IContentGenerator
)

__all__ = [
    'IAnnotator',
    'GeneratorContext',
    'GeneratorResult',
    'IContentGenerator'
]
```

**Step 3: Commit**

```bash
git add -A
git commit -m "chore: 删除 Worker 废弃的动态生成器代码"
```

---

## Task 7: 删除数据库表

**Step 1: 执行 SQL**

```sql
DROP TABLE IF EXISTS generators;
```

**Step 2: 更新迁移文件（可选）**

如果有迁移文件包含 generators 表创建，添加注释说明已废弃。

**Step 3: Commit**

```bash
git commit -m "chore: 删除 generators 数据库表" --allow-empty
```

---

## Task 8: 验证

**Step 1: 验证前端**

1. 启动开发服务器
2. 确认菜单中没有"生成器管理"
3. 访问 `/generators` 应该 404 或重定向

**Step 2: 验证后端**

1. 确认 Go 编译通过
2. 确认 `/api/generators` 返回 404

**Step 3: 验证 Worker**

1. 确认 Python import 正常
2. 运行 Worker 确认无报错
