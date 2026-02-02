# 统一对象池配置实现计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将 4 个对象池（标题池、cls类名池、url池、关键词表情池）的配置统一化，支持数据库配置和前端下拉选择。

**Architecture:** 扩展 CachePoolConfig 添加 12 个新字段（4 池 × 3 个新字段），修改 ObjectPool 补充逻辑为"补充到满"，前端用下拉菜单选择池类型进行统一配置。

**Tech Stack:** Go 1.24, Gin, Vue 3, TypeScript, Element Plus, MySQL

---

## Task 1: 数据库迁移

**Files:**
- Create: `migrations/002_unified_pool_config.sql`

**Step 1: 创建迁移文件**

```sql
-- 统一对象池配置字段
-- cls类名池
ALTER TABLE pool_config
ADD COLUMN cls_pool_size INT DEFAULT 800000 AFTER title_threshold,
ADD COLUMN cls_workers INT DEFAULT 20 AFTER cls_pool_size,
ADD COLUMN cls_refill_interval_ms INT DEFAULT 30 AFTER cls_workers,
ADD COLUMN cls_threshold DECIMAL(3,2) DEFAULT 0.40 AFTER cls_refill_interval_ms;

-- url池
ALTER TABLE pool_config
ADD COLUMN url_pool_size INT DEFAULT 500000 AFTER cls_threshold,
ADD COLUMN url_workers INT DEFAULT 16 AFTER url_pool_size,
ADD COLUMN url_refill_interval_ms INT DEFAULT 30 AFTER url_workers,
ADD COLUMN url_threshold DECIMAL(3,2) DEFAULT 0.40 AFTER url_refill_interval_ms;

-- 关键词表情池
ALTER TABLE pool_config
ADD COLUMN keyword_emoji_pool_size INT DEFAULT 800000 AFTER url_threshold,
ADD COLUMN keyword_emoji_workers INT DEFAULT 20 AFTER keyword_emoji_pool_size,
ADD COLUMN keyword_emoji_refill_interval_ms INT DEFAULT 30 AFTER keyword_emoji_workers,
ADD COLUMN keyword_emoji_threshold DECIMAL(3,2) DEFAULT 0.40 AFTER keyword_emoji_refill_interval_ms;

-- 修改标题池 threshold 为浮点数类型
ALTER TABLE pool_config
MODIFY COLUMN title_threshold DECIMAL(3,2) DEFAULT 0.40;

-- 更新现有记录
UPDATE pool_config SET
  cls_pool_size = 800000,
  cls_workers = 20,
  cls_refill_interval_ms = 30,
  cls_threshold = 0.40,
  url_pool_size = 500000,
  url_workers = 16,
  url_refill_interval_ms = 30,
  url_threshold = 0.40,
  keyword_emoji_pool_size = 800000,
  keyword_emoji_workers = 20,
  keyword_emoji_refill_interval_ms = 30,
  keyword_emoji_threshold = 0.40,
  title_threshold = 0.40
WHERE id = 1;
```

**Step 2: Commit**

```bash
git add migrations/002_unified_pool_config.sql
git commit -m "chore(db): 添加统一对象池配置字段迁移"
```

---

## Task 2: 扩展配置结构

**Files:**
- Modify: `api/internal/service/pool_config.go`

**Step 1: 更新 CachePoolConfig 结构体**

添加 12 个新字段，修改 TitleThreshold 为 float64：

```go
// CachePoolConfig holds cache pool configuration
type CachePoolConfig struct {
	ID               int       `db:"id" json:"id"`
	TitlesSize       int       `db:"titles_size" json:"titles_size"`
	ContentsSize     int       `db:"contents_size" json:"contents_size"`
	Threshold        int       `db:"threshold" json:"threshold"`
	RefillIntervalMs int       `db:"refill_interval_ms" json:"refill_interval_ms"`
	KeywordsSize     int       `db:"keywords_size" json:"keywords_size"`
	ImagesSize       int       `db:"images_size" json:"images_size"`
	RefreshIntervalMs int      `db:"refresh_interval_ms" json:"refresh_interval_ms"`

	// 标题池配置
	TitlePoolSize         int     `db:"title_pool_size" json:"title_pool_size"`
	TitleWorkers          int     `db:"title_workers" json:"title_workers"`
	TitleRefillIntervalMs int     `db:"title_refill_interval_ms" json:"title_refill_interval_ms"`
	TitleThreshold        float64 `db:"title_threshold" json:"title_threshold"`

	// cls类名池配置
	ClsPoolSize         int     `db:"cls_pool_size" json:"cls_pool_size"`
	ClsWorkers          int     `db:"cls_workers" json:"cls_workers"`
	ClsRefillIntervalMs int     `db:"cls_refill_interval_ms" json:"cls_refill_interval_ms"`
	ClsThreshold        float64 `db:"cls_threshold" json:"cls_threshold"`

	// url池配置
	UrlPoolSize         int     `db:"url_pool_size" json:"url_pool_size"`
	UrlWorkers          int     `db:"url_workers" json:"url_workers"`
	UrlRefillIntervalMs int     `db:"url_refill_interval_ms" json:"url_refill_interval_ms"`
	UrlThreshold        float64 `db:"url_threshold" json:"url_threshold"`

	// 关键词表情池配置
	KeywordEmojiPoolSize         int     `db:"keyword_emoji_pool_size" json:"keyword_emoji_pool_size"`
	KeywordEmojiWorkers          int     `db:"keyword_emoji_workers" json:"keyword_emoji_workers"`
	KeywordEmojiRefillIntervalMs int     `db:"keyword_emoji_refill_interval_ms" json:"keyword_emoji_refill_interval_ms"`
	KeywordEmojiThreshold        float64 `db:"keyword_emoji_threshold" json:"keyword_emoji_threshold"`

	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
```

**Step 2: 更新 DefaultCachePoolConfig**

```go
func DefaultCachePoolConfig() *CachePoolConfig {
	return &CachePoolConfig{
		ID:                1,
		TitlesSize:        5000,
		ContentsSize:      5000,
		Threshold:         1000,
		RefillIntervalMs:  1000,
		KeywordsSize:      50000,
		ImagesSize:        50000,
		RefreshIntervalMs: 300000,
		// 标题池
		TitlePoolSize:         800000,
		TitleWorkers:          20,
		TitleRefillIntervalMs: 30,
		TitleThreshold:        0.4,
		// cls类名池
		ClsPoolSize:         800000,
		ClsWorkers:          20,
		ClsRefillIntervalMs: 30,
		ClsThreshold:        0.4,
		// url池
		UrlPoolSize:         500000,
		UrlWorkers:          16,
		UrlRefillIntervalMs: 30,
		UrlThreshold:        0.4,
		// 关键词表情池
		KeywordEmojiPoolSize:         800000,
		KeywordEmojiWorkers:          20,
		KeywordEmojiRefillIntervalMs: 30,
		KeywordEmojiThreshold:        0.4,
	}
}
```

**Step 3: 更新 SaveCachePoolConfig**

```go
func SaveCachePoolConfig(ctx context.Context, db *sqlx.DB, config *CachePoolConfig) error {
	query := `
		INSERT INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms,
			keywords_size, images_size, refresh_interval_ms,
			title_pool_size, title_workers, title_refill_interval_ms, title_threshold,
			cls_pool_size, cls_workers, cls_refill_interval_ms, cls_threshold,
			url_pool_size, url_workers, url_refill_interval_ms, url_threshold,
			keyword_emoji_pool_size, keyword_emoji_workers, keyword_emoji_refill_interval_ms, keyword_emoji_threshold)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			titles_size = VALUES(titles_size),
			contents_size = VALUES(contents_size),
			threshold = VALUES(threshold),
			refill_interval_ms = VALUES(refill_interval_ms),
			keywords_size = VALUES(keywords_size),
			images_size = VALUES(images_size),
			refresh_interval_ms = VALUES(refresh_interval_ms),
			title_pool_size = VALUES(title_pool_size),
			title_workers = VALUES(title_workers),
			title_refill_interval_ms = VALUES(title_refill_interval_ms),
			title_threshold = VALUES(title_threshold),
			cls_pool_size = VALUES(cls_pool_size),
			cls_workers = VALUES(cls_workers),
			cls_refill_interval_ms = VALUES(cls_refill_interval_ms),
			cls_threshold = VALUES(cls_threshold),
			url_pool_size = VALUES(url_pool_size),
			url_workers = VALUES(url_workers),
			url_refill_interval_ms = VALUES(url_refill_interval_ms),
			url_threshold = VALUES(url_threshold),
			keyword_emoji_pool_size = VALUES(keyword_emoji_pool_size),
			keyword_emoji_workers = VALUES(keyword_emoji_workers),
			keyword_emoji_refill_interval_ms = VALUES(keyword_emoji_refill_interval_ms),
			keyword_emoji_threshold = VALUES(keyword_emoji_threshold)
	`
	_, err := db.ExecContext(ctx, query,
		config.TitlesSize,
		config.ContentsSize,
		config.Threshold,
		config.RefillIntervalMs,
		config.KeywordsSize,
		config.ImagesSize,
		config.RefreshIntervalMs,
		config.TitlePoolSize,
		config.TitleWorkers,
		config.TitleRefillIntervalMs,
		config.TitleThreshold,
		config.ClsPoolSize,
		config.ClsWorkers,
		config.ClsRefillIntervalMs,
		config.ClsThreshold,
		config.UrlPoolSize,
		config.UrlWorkers,
		config.UrlRefillIntervalMs,
		config.UrlThreshold,
		config.KeywordEmojiPoolSize,
		config.KeywordEmojiWorkers,
		config.KeywordEmojiRefillIntervalMs,
		config.KeywordEmojiThreshold,
	)
	return err
}
```

**Step 4: 添加辅助方法**

```go
// ClsRefillInterval returns the cls refill interval as time.Duration
func (c *CachePoolConfig) ClsRefillInterval() time.Duration {
	return time.Duration(c.ClsRefillIntervalMs) * time.Millisecond
}

// UrlRefillInterval returns the url refill interval as time.Duration
func (c *CachePoolConfig) UrlRefillInterval() time.Duration {
	return time.Duration(c.UrlRefillIntervalMs) * time.Millisecond
}

// KeywordEmojiRefillInterval returns the keyword emoji refill interval as time.Duration
func (c *CachePoolConfig) KeywordEmojiRefillInterval() time.Duration {
	return time.Duration(c.KeywordEmojiRefillIntervalMs) * time.Millisecond
}
```

**Step 5: Commit**

```bash
git add api/internal/service/pool_config.go
git commit -m "feat(pool): 扩展配置结构支持4个对象池"
```

---

## Task 3: 修改 ObjectPool 补充逻辑

**Files:**
- Modify: `api/internal/service/object_pool.go`

**Step 1: 修改 PoolConfig 结构**

将 `LowWatermark` 改为 `Threshold`，移除 `RefillBatch`：

```go
// PoolConfig 池配置
type PoolConfig struct {
	Name          string
	Size          int
	Threshold     float64       // 低于此比例触发补充（0-1）
	NumWorkers    int
	CheckInterval time.Duration
}
```

**Step 2: 修改 ObjectPool 结构体**

将 `lowWatermark` 改为 `threshold`，移除 `refillBatch`：

```go
type ObjectPool[T any] struct {
	name string
	pool []T
	size int64

	head int64
	tail int64

	// 配置
	threshold     float64       // 低于此比例触发补充
	numWorkers    int
	checkInterval time.Duration

	// ... 其他字段保持不变
}
```

**Step 3: 修改 NewObjectPool**

```go
func NewObjectPool[T any](cfg PoolConfig, generator func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		name:          cfg.Name,
		pool:          make([]T, cfg.Size),
		size:          int64(cfg.Size),
		threshold:     cfg.Threshold,
		numWorkers:    cfg.NumWorkers,
		checkInterval: cfg.CheckInterval,
		generator:     generator,
		stopCh:        make(chan struct{}),
	}
}
```

**Step 4: 修改 checkAndRefill 为"补充到满"**

```go
// checkAndRefill 检查并补充到满
func (p *ObjectPool[T]) checkAndRefill() {
	if p.paused.Load() {
		return
	}

	p.mu.RLock()
	size := p.size
	p.mu.RUnlock()

	available := p.Available()
	thresholdCount := int64(float64(size) * p.threshold)

	if available < thresholdCount {
		// 补充到满
		need := size - available
		p.refillToFull(int(need))
		p.refillCount.Add(1)
		p.lastRefresh.Store(time.Now().UnixNano())
	}
}

// refillToFull 补充指定数量到池中
func (p *ObjectPool[T]) refillToFull(need int) {
	var wg sync.WaitGroup
	batchPerWorker := need / p.numWorkers
	remainder := need % p.numWorkers

	for w := 0; w < p.numWorkers; w++ {
		wg.Add(1)
		workerBatch := batchPerWorker
		if w == p.numWorkers-1 {
			workerBatch += remainder
		}
		go func(batch int) {
			defer wg.Done()

			items := make([]T, batch)
			for i := 0; i < batch; i++ {
				items[i] = p.generator()
			}

			p.mu.RLock()
			pool := p.pool
			currentSize := p.size
			p.mu.RUnlock()

			for _, item := range items {
				idx := atomic.AddInt64(&p.tail, 1) - 1
				pool[idx%currentSize] = item
			}

			atomic.AddInt64(&p.totalGenerated, int64(batch))
		}(workerBatch)
	}

	wg.Wait()
}
```

**Step 5: 删除旧的 refillParallel 方法中的 refillBatch 引用**

修改 `prefillParallel` 保持不变（它用于初始填充整个池）。

**Step 6: Commit**

```bash
git add api/internal/service/object_pool.go
git commit -m "refactor(pool): 修改补充逻辑为补充到满"
```

---

## Task 4: 修改 template_funcs.go 从配置初始化池

**Files:**
- Modify: `api/internal/service/template_funcs.go`

**Step 1: 修改 InitPools 接收配置参数**

```go
// InitPools 初始化所有池子（从配置读取）
func (m *TemplateFuncsManager) InitPools(config *CachePoolConfig) {
	// cls池
	m.clsPool = NewObjectPool[string](PoolConfig{
		Name:          "cls",
		Size:          config.ClsPoolSize,
		Threshold:     config.ClsThreshold,
		NumWorkers:    config.ClsWorkers,
		CheckInterval: config.ClsRefillInterval(),
	}, generateRandomCls)

	// url池
	m.urlPool = NewObjectPool[string](PoolConfig{
		Name:          "url",
		Size:          config.UrlPoolSize,
		Threshold:     config.UrlThreshold,
		NumWorkers:    config.UrlWorkers,
		CheckInterval: config.UrlRefillInterval(),
	}, generateRandomURL)

	// number池
	m.numberPool = NewNumberPool()

	// 启动所有池
	m.clsPool.Start()
	m.urlPool.Start()
	m.numberPool.Start()
}

// InitKeywordEmojiPool 初始化带 emoji 的关键词池（从配置读取）
func (m *TemplateFuncsManager) InitKeywordEmojiPool(config *CachePoolConfig) {
	if m.emojiManager == nil || atomic.LoadInt64(&m.rawKeywordLen) == 0 {
		return
	}

	m.keywordEmojiPool = NewObjectPool[string](PoolConfig{
		Name:          "keyword_emoji",
		Size:          config.KeywordEmojiPoolSize,
		Threshold:     config.KeywordEmojiThreshold,
		NumWorkers:    config.KeywordEmojiWorkers,
		CheckInterval: config.KeywordEmojiRefillInterval(),
	}, m.generateKeywordWithEmoji)

	m.keywordEmojiPool.Start()
}
```

**Step 2: Commit**

```bash
git add api/internal/service/template_funcs.go
git commit -m "feat(pool): 从配置初始化对象池"
```

---

## Task 5: 更新调用处传递配置

**Files:**
- Modify: 调用 InitPools 的地方（需要查找）

**Step 1: 查找并更新调用处**

搜索 `InitPools()` 调用，添加配置参数。通常在 main.go 或 app 初始化处。

**Step 2: Commit**

```bash
git add -A
git commit -m "feat(pool): 更新 InitPools 调用传递配置"
```

---

## Task 6: 更新 API Handler

**Files:**
- Modify: `api/internal/handler/pool.go`

**Step 1: 更新 UpdateConfig 请求结构和验证**

添加新字段到请求结构和验证逻辑，参考现有的 title 字段验证。

**Step 2: Commit**

```bash
git add api/internal/handler/pool.go
git commit -m "feat(api): 更新缓存池配置 API 支持所有对象池参数"
```

---

## Task 7: 更新前端类型定义

**Files:**
- Modify: `web/src/api/cache-pool.ts`

**Step 1: 添加新字段到类型定义**

```typescript
export interface CachePoolConfig {
  id?: number
  titles_size: number
  contents_size: number
  threshold: number
  refill_interval_ms: number
  keywords_size: number
  images_size: number
  refresh_interval_ms: number
  // 标题池
  title_pool_size: number
  title_workers: number
  title_refill_interval_ms: number
  title_threshold: number
  // cls类名池
  cls_pool_size: number
  cls_workers: number
  cls_refill_interval_ms: number
  cls_threshold: number
  // url池
  url_pool_size: number
  url_workers: number
  url_refill_interval_ms: number
  url_threshold: number
  // 关键词表情池
  keyword_emoji_pool_size: number
  keyword_emoji_workers: number
  keyword_emoji_refill_interval_ms: number
  keyword_emoji_threshold: number
  updated_at?: string
}
```

**Step 2: Commit**

```bash
git add web/src/api/cache-pool.ts
git commit -m "feat(web): 添加统一对象池配置类型定义"
```

---

## Task 8: 更新前端配置页面

**Files:**
- Modify: `web/src/views/cache/CacheManage.vue`

**Step 1: 添加对象池选择下拉和响应式数据**

在 `<script setup>` 中添加：

```typescript
// 对象池选择
const poolOptions = [
  { label: '标题池', value: 'title' },
  { label: 'cls类名池', value: 'cls' },
  { label: 'url池', value: 'url' },
  { label: '关键词表情池', value: 'keyword_emoji' }
]
const selectedPool = ref('title')

// 当前选中池的配置（计算属性）
const currentPoolConfig = computed({
  get: () => {
    const prefix = selectedPool.value
    return {
      pool_size: cachePoolForm[`${prefix}_pool_size`],
      workers: cachePoolForm[`${prefix}_workers`],
      refill_interval_ms: cachePoolForm[`${prefix}_refill_interval_ms`],
      threshold: cachePoolForm[`${prefix}_threshold`]
    }
  },
  set: (val) => {
    const prefix = selectedPool.value
    cachePoolForm[`${prefix}_pool_size`] = val.pool_size
    cachePoolForm[`${prefix}_workers`] = val.workers
    cachePoolForm[`${prefix}_refill_interval_ms`] = val.refill_interval_ms
    cachePoolForm[`${prefix}_threshold`] = val.threshold
  }
})
```

**Step 2: 更新 cachePoolForm 添加所有新字段**

```typescript
const cachePoolForm = reactive<CachePoolConfig>({
  titles_size: 5000,
  contents_size: 5000,
  threshold: 1000,
  refill_interval_ms: 1000,
  keywords_size: 50000,
  images_size: 50000,
  refresh_interval_ms: 300000,
  // 标题池
  title_pool_size: 800000,
  title_workers: 20,
  title_refill_interval_ms: 30,
  title_threshold: 0.4,
  // cls类名池
  cls_pool_size: 800000,
  cls_workers: 20,
  cls_refill_interval_ms: 30,
  cls_threshold: 0.4,
  // url池
  url_pool_size: 500000,
  url_workers: 16,
  url_refill_interval_ms: 30,
  url_threshold: 0.4,
  // 关键词表情池
  keyword_emoji_pool_size: 800000,
  keyword_emoji_workers: 20,
  keyword_emoji_refill_interval_ms: 30,
  keyword_emoji_threshold: 0.4
})
```

**Step 3: 更新 loadCachePoolConfig 函数加载所有字段**

**Step 4: 替换"标题池配置"卡片为"对象池配置"卡片**

```vue
<!-- 对象池配置 -->
<el-col :xs="24" :lg="12">
  <div class="config-card">
    <div class="card-header">
      <span class="card-title">对象池配置</span>
    </div>
    <div class="card-content">
      <el-form-item label="选择池">
        <el-select v-model="selectedPool" style="width: 100%">
          <el-option
            v-for="opt in poolOptions"
            :key="opt.value"
            :label="opt.label"
            :value="opt.value"
          />
        </el-select>
      </el-form-item>
      <el-form-item label="池大小">
        <el-input-number
          v-model="currentPoolConfig.pool_size"
          :min="100000"
          :max="2000000"
          :step="100000"
        />
        <span class="form-tip">条</span>
      </el-form-item>
      <el-form-item label="生成协程数">
        <el-input-number
          v-model="currentPoolConfig.workers"
          :min="1"
          :max="50"
          :step="1"
        />
        <span class="form-tip">个</span>
      </el-form-item>
      <el-form-item label="生成间隔">
        <el-input-number
          v-model="currentPoolConfig.refill_interval_ms"
          :min="10"
          :max="1000"
          :step="10"
        />
        <span class="form-tip">毫秒</span>
      </el-form-item>
      <el-form-item label="补充阈值">
        <el-input-number
          v-model="currentPoolConfig.threshold"
          :min="0.1"
          :max="0.9"
          :step="0.1"
          :precision="2"
        />
        <span class="form-tip">(0.1-0.9)</span>
      </el-form-item>
    </div>
  </div>
</el-col>
```

**Step 5: Commit**

```bash
git add web/src/views/cache/CacheManage.vue
git commit -m "feat(web): 添加对象池下拉选择配置界面"
```

---

## Task 9: 最终验证

**Step 1: Go 编译检查**

```bash
cd api && go build ./...
```

Expected: 编译成功

**Step 2: 前端编译检查**

```bash
cd web && npm run build
```

Expected: 编译成功

**Step 3: 执行数据库迁移**

```bash
mysql -u root -p seo_generator < migrations/002_unified_pool_config.sql
```

**Step 4: 功能测试**

1. 启动服务
2. 访问缓存管理 → 数据池配置
3. 切换下拉菜单，确认显示不同池的配置
4. 修改配置并保存，确认生效

**Step 5: 最终提交**

```bash
git add -A
git commit -m "feat: 完成统一对象池配置功能

- 4个对象池使用统一配置结构（池大小、协程数、间隔、阈值）
- 前端下拉菜单选择池类型进行配置
- 补充逻辑改为低于阈值时补充到满"
```
