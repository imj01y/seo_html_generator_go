# 爬虫项目抓取类型与分组绑定 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 爬虫项目增加 `crawl_type` 字段，前端按类型联动选择对应分组，后端校验 yield 的 type 与项目配置一致。

**Architecture:** 在 `spider_projects` 表新增 `crawl_type` 列（article/keywords/images），前端根据选择的类型动态加载对应分组列表（文章/关键词/图片分组），Python Worker 在处理 yield 数据时校验 type 是否匹配项目 crawl_type。

**Tech Stack:** MySQL (ALTER TABLE), Go (gin/sqlx), Vue 3 (Element Plus), Python (loguru)

---

### Task 1: 数据库 — 添加 crawl_type 字段

**Files:**
- Modify: `migrations/000_init.sql` (末尾追加迁移 SQL)

**Step 1: 在 init.sql 末尾追加 ALTER TABLE**

```sql
-- 2026-02-08: 爬虫项目增加抓取类型字段
ALTER TABLE spider_projects
ADD COLUMN crawl_type VARCHAR(20) NOT NULL DEFAULT 'article' COMMENT '抓取类型: article/keywords/images' AFTER concurrency;

-- 存量数据默认为 article
UPDATE spider_projects SET crawl_type = 'article' WHERE crawl_type = '' OR crawl_type = 'article';
```

**Step 2: 在 Docker 容器中执行迁移**

```bash
docker compose exec mysql mysql -uroot -p<password> seo_html_generator -e "
ALTER TABLE spider_projects ADD COLUMN crawl_type VARCHAR(20) NOT NULL DEFAULT 'article' COMMENT '抓取类型: article/keywords/images' AFTER concurrency;
"
```

**Step 3: Commit**

```bash
git add migrations/000_init.sql
git commit -m "feat: add crawl_type column to spider_projects table"
```

---

### Task 2: Go Model — 添加 CrawlType 字段

**Files:**
- Modify: `api/internal/model/spider_models.go:9-31` (SpiderProject struct)
- Modify: `api/internal/model/spider_models.go:45-57` (SpiderProjectCreate struct)
- Modify: `api/internal/model/spider_models.go:60-71` (SpiderProjectUpdate struct)

**Step 1: SpiderProject 添加 CrawlType 字段**

在第 19 行 `OutputGroupID` 前面插入：

```go
CrawlType       string          `db:"crawl_type" json:"crawl_type"`
```

**Step 2: SpiderProjectCreate 添加 CrawlType 字段**

在第 53 行 `OutputGroupID` 前面插入：

```go
CrawlType     string                 `json:"crawl_type"`
```

**Step 3: SpiderProjectUpdate 添加 CrawlType 字段**

在第 68 行 `OutputGroupID` 前面插入：

```go
CrawlType     *string                `json:"crawl_type"`
```

**Step 4: Commit**

```bash
git add api/internal/model/spider_models.go
git commit -m "feat: add CrawlType field to spider project models"
```

---

### Task 3: Go Handler — Create/Update/List 支持 crawl_type

**Files:**
- Modify: `api/internal/handler/spider_projects.go`

**Step 1: List 查询添加 crawl_type（第 111 行）**

将 `config, concurrency, output_group_id` 改为 `config, concurrency, crawl_type, output_group_id`。

同样修改 Get 方法中的 SELECT（约第 157 行），也加上 `crawl_type`。

**Step 2: Create 方法添加 crawl_type 默认值和 INSERT（第 197-226 行）**

在第 202 行后添加：

```go
if req.CrawlType == "" {
    req.CrawlType = "article"
}
```

修改第 221-223 行 INSERT 语句，在 `concurrency` 后加 `crawl_type`：

```go
result, err := tx.Exec(`
    INSERT INTO spider_projects
    (name, description, entry_file, entry_function, start_url, config,
     concurrency, crawl_type, output_group_id, schedule, enabled)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, req.Name, req.Description, req.EntryFile, req.EntryFunction,
    req.StartURL, configJSON, req.Concurrency, req.CrawlType, req.OutputGroupID,
    req.Schedule, req.Enabled)
```

**Step 3: Update 方法添加 crawl_type 更新（第 352 行附近）**

在 `OutputGroupID` 更新逻辑前添加：

```go
if req.CrawlType != nil {
    updates = append(updates, "crawl_type = ?")
    args = append(args, *req.CrawlType)
}
```

**Step 4: Commit**

```bash
git add api/internal/handler/spider_projects.go
git commit -m "feat: support crawl_type in spider project CRUD"
```

---

### Task 4: TypeScript 类型 — 添加 crawl_type

**Files:**
- Modify: `web/src/api/spiderProjects.ts:16-39` (SpiderProject)
- Modify: `web/src/api/spiderProjects.ts:51-63` (ProjectCreate)
- Modify: `web/src/api/spiderProjects.ts:65-76` (ProjectUpdate)

**Step 1: SpiderProject interface 添加字段**

在第 25 行 `output_group_id` 前插入：

```typescript
crawl_type: 'article' | 'keywords' | 'images'
```

**Step 2: ProjectCreate interface 添加字段**

在第 59 行 `output_group_id` 前插入：

```typescript
crawl_type?: 'article' | 'keywords' | 'images'
```

**Step 3: ProjectUpdate interface 添加字段**

在第 73 行 `output_group_id` 前插入：

```typescript
crawl_type?: 'article' | 'keywords' | 'images'
```

**Step 4: Commit**

```bash
git add web/src/api/spiderProjects.ts
git commit -m "feat: add crawl_type to spider project TypeScript types"
```

---

### Task 5: 前端 ProjectEdit.vue — 抓取类型联动分组选择

**Files:**
- Modify: `web/src/views/spiders/ProjectEdit.vue`

**Step 1: 修改 import（第 216 行）**

新增导入：

```typescript
import { getKeywordGroups } from '@/api/keywords'
import { getImageGroups } from '@/api/images'
```

**Step 2: 修改 form reactive（第 243-251 行）**

将 `output_group_id: 1` 改为默认不选中，并添加 `crawl_type`：

```typescript
const form = reactive({
  name: '',
  description: '',
  entry_file: 'spider.py',
  concurrency: 3,
  crawl_type: '' as string,
  output_group_id: null as number | null,
  schedule: '',
  enabled: 1
})
```

**Step 3: 修改分组变量（第 259 行附近）**

将 `articleGroups` 改为通用的 `outputGroups`：

```typescript
const outputGroups = ref<{ id: number; name: string }[]>([])
```

**Step 4: 添加抓取类型选项和联动逻辑**

```typescript
const crawlTypeOptions = [
  { value: 'article', label: '文章' },
  { value: 'keywords', label: '关键词' },
  { value: 'images', label: '图片' },
]

// 切换抓取类型时重新加载分组列表
async function onCrawlTypeChange() {
  form.output_group_id = null
  outputGroups.value = []
  onFormChange()
  await loadGroupsByType(form.crawl_type)
}

async function loadGroupsByType(type: string) {
  try {
    if (type === 'article') {
      const groups = await getArticleGroups()
      outputGroups.value = groups.map(g => ({ id: g.id, name: g.name }))
    } else if (type === 'keywords') {
      const groups = await getKeywordGroups()
      outputGroups.value = groups.map(g => ({ id: g.id, name: g.name }))
    } else if (type === 'images') {
      const groups = await getImageGroups()
      outputGroups.value = groups.map(g => ({ id: g.id, name: g.name }))
    }
  } catch {
    outputGroups.value = []
  }
}
```

**Step 5: 修改模板 — 替换输出分组下拉为两级联动（第 68-77 行）**

替换原来的"输出分组" form-item 为：

```vue
<el-form-item label="抓取类型" required>
  <el-select
    v-model="form.crawl_type"
    placeholder="请选择抓取类型"
    style="width: 100%"
    @change="onCrawlTypeChange"
  >
    <el-option
      v-for="opt in crawlTypeOptions"
      :key="opt.value"
      :label="opt.label"
      :value="opt.value"
    />
  </el-select>
</el-form-item>
<el-form-item label="输出分组" required>
  <el-select
    v-model="form.output_group_id"
    placeholder="请先选择抓取类型"
    style="width: 100%"
    :disabled="!form.crawl_type"
    @change="onFormChange"
  >
    <el-option
      v-for="group in outputGroups"
      :key="group.id"
      :label="group.name"
      :value="group.id"
    />
  </el-select>
</el-form-item>
```

**Step 6: 修改保存逻辑 — 传递 crawl_type**

在 `updateProject` 调用处（约第 385-392 行和第 676-683 行），添加 `crawl_type: form.crawl_type`。

在 `createProject` 调用处，同样添加 `crawl_type: form.crawl_type`。

保存前验证：

```typescript
if (!form.crawl_type) {
  ElMessage.warning('请选择抓取类型')
  return
}
if (!form.output_group_id) {
  ElMessage.warning('请选择输出分组')
  return
}
```

**Step 7: 修改加载逻辑 — 编辑时回显（第 486-494 行）**

```typescript
form.crawl_type = project.crawl_type || 'article'
form.output_group_id = project.output_group_id
// 根据类型加载对应分组
await loadGroupsByType(form.crawl_type)
```

**Step 8: 修改 onMounted — 去掉固定加载 articleGroups（第 891-898 行）**

删除 onMounted 中的 `getArticleGroups()` 调用（分组加载已移到 `loadGroupsByType` 中由 `loadProject` 触发）。
新建项目时不需要预加载分组，因为用户必须先选类型。

**Step 9: Commit**

```bash
git add web/src/views/spiders/ProjectEdit.vue
git commit -m "feat: crawl type dropdown with dynamic group selection in ProjectEdit"
```

---

### Task 6: 前端 ProjectList.vue — 配置弹窗同步改造

**Files:**
- Modify: `web/src/views/spiders/ProjectList.vue`

**Step 1: 修改 import（第 248 行）**

新增导入：

```typescript
import { getKeywordGroups } from '@/api/keywords'
import { getImageGroups } from '@/api/images'
```

**Step 2: 修改 configForm（第 294-301 行）**

```typescript
const configForm = ref({
  name: '',
  description: '',
  entry_file: '',
  concurrency: 3,
  crawl_type: '' as string,
  output_group_id: null as number | null,
  schedule: ''
})
```

**Step 3: 将 articleGroups 改为 outputGroups（第 304 行）**

```typescript
const outputGroups = ref<{ id: number; name: string }[]>([])
```

**Step 4: 替换 loadArticleGroups 为 loadGroupsByType（第 307-314 行）**

```typescript
async function loadGroupsByType(type: string) {
  try {
    if (type === 'article') {
      const groups = await getArticleGroups()
      outputGroups.value = groups.map(g => ({ id: g.id, name: g.name }))
    } else if (type === 'keywords') {
      const groups = await getKeywordGroups()
      outputGroups.value = groups.map(g => ({ id: g.id, name: g.name }))
    } else if (type === 'images') {
      const groups = await getImageGroups()
      outputGroups.value = groups.map(g => ({ id: g.id, name: g.name }))
    }
  } catch {
    outputGroups.value = []
  }
}

async function onCrawlTypeChange() {
  configForm.value.output_group_id = null
  outputGroups.value = []
  await loadGroupsByType(configForm.value.crawl_type)
}
```

**Step 5: 修改模板 — 配置弹窗中添加抓取类型（第 203-213 行）**

在"输出分组"前添加"抓取类型"下拉，并把分组的 `articleGroups` 改为 `outputGroups`，添加 `disabled` 和 `placeholder`。

**Step 6: 修改 handleConfig — 回显时加载分组（第 370-389 行）**

```typescript
configForm.value = {
  name: row.name,
  description: row.description || '',
  entry_file: row.entry_file,
  concurrency: row.concurrency,
  crawl_type: row.crawl_type || 'article',
  output_group_id: row.output_group_id,
  schedule: row.schedule || ''
}
await loadGroupsByType(configForm.value.crawl_type)
```

**Step 7: 修改 handleSaveConfig — 传递 crawl_type（第 402-408 行）**

添加 `crawl_type: configForm.value.crawl_type`，并添加保存前验证。

**Step 8: Commit**

```bash
git add web/src/views/spiders/ProjectList.vue
git commit -m "feat: crawl type selection in ProjectList config dialog"
```

---

### Task 7: Python Worker — 加载 crawl_type 并校验 yield type

**Files:**
- Modify: `content_worker/core/workers/command_listener.py`

**Step 1: 修改 _load_project — 查询 crawl_type（第 290-313 行）**

SQL 中添加 `crawl_type` 字段：

```python
row = await fetch_one(
    "SELECT id, name, entry_file, config, concurrency, crawl_type, output_group_id FROM spider_projects WHERE id = %s",
    (project_id,)
)
```

返回字典中添加：

```python
return {
    "id": project_id,
    "config": config,
    "modules": modules,
    "concurrency": row.get('concurrency', 3),
    "crawl_type": row.get('crawl_type', 'article'),
    "group_id": row['output_group_id'],
}
```

**Step 2: 修改 _run_and_process — 传递 crawl_type（第 341 行）**

```python
count = await self._process_item(item, project["group_id"], project["id"], project["crawl_type"])
```

**Step 3: 修改 _process_item — 增加 crawl_type 参数并校验（第 349 行起）**

```python
async def _process_item(self, item: dict, group_id: int, project_id: int, crawl_type: str = 'article') -> int:
    """处理单个数据项（路由到 keywords/images/article）"""
    item_type = item.get('type', 'article')

    # 校验 yield 的 type 与项目配置的 crawl_type 是否一致
    if item_type != crawl_type:
        logger.warning(f"数据类型不匹配: yield type='{item_type}', 项目配置 crawl_type='{crawl_type}'，已跳过")
        return 0

    try:
        # ... 原有的分支逻辑不变 ...
```

**Step 4: 修改 test_project — 同样校验 crawl_type（约第 437 行）**

在 `_load_project` 返回中已包含 `crawl_type`，在测试循环中添加校验：

```python
crawl_type = project["crawl_type"]

async for item in runner.run():
    # ... 检查停止信号 ...

    item_type = item.get('type', 'article')

    # 校验类型匹配
    if item_type != crawl_type:
        logger.warning(f"数据类型不匹配: yield type='{item_type}', 项目配置 crawl_type='{crawl_type}'，已跳过")
        continue

    # ... 原有的按类型验证字段逻辑 ...
```

**Step 5: Commit**

```bash
git add content_worker/core/workers/command_listener.py
git commit -m "feat: validate yield type matches project crawl_type"
```

---

## 改动影响总结

| 文件 | 改动量 | 风险 |
|------|--------|------|
| `migrations/000_init.sql` | +3 行 | 低，ALTER TABLE 加列 |
| `spider_models.go` | +3 行 | 低，纯加字段 |
| `spider_projects.go` | ~15 行 | 低，CRUD 加字段 |
| `spiderProjects.ts` | +3 行 | 低，纯类型定义 |
| `ProjectEdit.vue` | ~60 行改动 | 中，UI 联动逻辑 |
| `ProjectList.vue` | ~40 行改动 | 中，配置弹窗改动 |
| `command_listener.py` | ~10 行改动 | 低，加校验 |

**存量兼容：** crawl_type 默认值 'article'，现有项目不受影响。
