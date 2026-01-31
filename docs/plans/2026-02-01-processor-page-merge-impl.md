# 数据处理页面合并 - 实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将"数据加工"和"数据加工代码"合并为统一的"数据处理"页面，使用 Tab 切换。

**Architecture:** 创建新的容器组件 `ProcessorPage.vue` 包含 el-tabs，将现有的 `ProcessorManage.vue` 和 `WorkerCodeEditor.vue` 作为 Tab 内容嵌入。修改路由和菜单配置。

**Tech Stack:** Vue 3, Element Plus (el-tabs), TypeScript

---

## Task 1: 修改 ProcessorManage.vue

**Files:**
- Modify: `web/src/views/processor/ProcessorManage.vue`

**Step 1: 去掉页面标题区域，将刷新按钮移到状态卡片区右侧**

将原来的：
```vue
<div class="page-header">
  <h2 class="title">数据加工管理</h2>
  <el-button size="small" @click="loadAll" :loading="loading">
    <el-icon><Refresh /></el-icon>
    刷新
  </el-button>
</div>

<!-- 状态卡片 -->
<el-row :gutter="16" class="status-cards">
```

改为：
```vue
<!-- 状态卡片 -->
<el-row :gutter="16" class="status-cards">
  <el-col :xs="24" class="status-header">
    <el-button size="small" @click="loadAll" :loading="loading">
      <el-icon><Refresh /></el-icon>
      刷新
    </el-button>
  </el-col>
```

**Step 2: 更新样式**

删除 `.page-header` 样式，添加 `.status-header` 样式：
```scss
.status-header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 12px;
}
```

**Step 3: 验证修改**

运行开发服务器，访问 `/processor` 页面，确认：
- 页面标题已移除
- 刷新按钮在状态卡片区右上角

**Step 4: Commit**

```bash
git add web/src/views/processor/ProcessorManage.vue
git commit -m "refactor(processor): 移除页面标题，刷新按钮移至状态卡片区"
```

---

## Task 2: 创建容器组件 ProcessorPage.vue

**Files:**
- Create: `web/src/views/processor/ProcessorPage.vue`

**Step 1: 创建 Tab 容器组件**

```vue
<template>
  <div class="processor-page">
    <el-tabs v-model="activeTab" class="processor-tabs">
      <el-tab-pane label="监控面板" name="monitor">
        <ProcessorManage />
      </el-tab-pane>
      <el-tab-pane label="代码编辑" name="code">
        <WorkerCodeEditor />
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ProcessorManage from './ProcessorManage.vue'
import WorkerCodeEditor from '@/views/worker/WorkerCodeEditor.vue'

const activeTab = ref('monitor')
</script>

<style lang="scss" scoped>
.processor-page {
  height: 100%;

  .processor-tabs {
    height: 100%;

    :deep(.el-tabs__content) {
      height: calc(100% - 40px);
      overflow: auto;
    }

    :deep(.el-tab-pane) {
      height: 100%;
    }
  }
}
</style>
```

**Step 2: 验证组件创建**

确认文件已创建且语法正确。

**Step 3: Commit**

```bash
git add web/src/views/processor/ProcessorPage.vue
git commit -m "feat(processor): 创建 Tab 容器组件"
```

---

## Task 3: 更新路由配置

**Files:**
- Modify: `web/src/router/index.ts`

**Step 1: 修改 /processor 路由指向新组件**

将：
```typescript
{
  path: 'processor',
  name: 'Processor',
  component: () => import('@/views/processor/ProcessorManage.vue'),
  meta: { title: '数据加工', icon: 'Operation' }
},
{
  path: 'worker',
  name: 'WorkerCode',
  component: () => import('@/views/worker/WorkerCodeEditor.vue'),
  meta: { title: '数据加工代码', icon: 'EditPen' }
},
```

改为：
```typescript
{
  path: 'processor',
  name: 'Processor',
  component: () => import('@/views/processor/ProcessorPage.vue'),
  meta: { title: '数据处理', icon: 'Operation' }
},
```

**Step 2: 验证路由配置**

运行开发服务器，访问 `/processor`，确认 Tab 页面正常显示。

**Step 3: Commit**

```bash
git add web/src/router/index.ts
git commit -m "refactor(router): 合并 processor 和 worker 路由"
```

---

## Task 4: 更新菜单配置

**Files:**
- Modify: `web/src/components/Layout/MainLayout.vue`

**Step 1: 修改菜单项**

将 menuItems 中的：
```typescript
{ path: '/processor', title: '数据加工', icon: 'Operation' },
{ path: '/worker', title: '数据加工代码', icon: 'EditPen' },
```

改为：
```typescript
{ path: '/processor', title: '数据处理', icon: 'Operation' },
```

**Step 2: 验证菜单显示**

运行开发服务器，确认：
- 菜单中只有一个"数据处理"项
- 点击可正常进入 Tab 页面

**Step 3: Commit**

```bash
git add web/src/components/Layout/MainLayout.vue
git commit -m "refactor(menu): 合并数据加工菜单项为数据处理"
```

---

## Task 5: 最终验证和提交

**Step 1: 完整功能验证**

1. 访问 `/processor`，确认显示 Tab 页面
2. 切换到"监控面板"，确认：
   - 刷新按钮在右上角
   - 状态卡片正常显示
   - 操作按钮正常工作
3. 切换到"代码编辑"，确认：
   - 文件树正常显示
   - Monaco 编辑器正常加载
   - 日志面板正常显示
4. 访问旧路由 `/worker`，确认重定向到 `/processor` 或 404

**Step 2: 合并提交（可选）**

如果需要，可以将所有改动合并为一个提交：
```bash
git log --oneline -5  # 查看最近提交
```
