# TitleGenerator 关键词分组同步 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 TitleGenerator 不响应关键词分组变化的问题——当关键词池新增/删除分组后，TitleGenerator 同步创建/移除对应的标题池和 worker。

**Architecture:** 在 TitleGenerator 上新增 `SyncGroups(groupIDs []int)` 方法，比较新旧分组差异，为新分组启动池和 worker，移除不再存在的分组。在 `PoolManager.ReloadKeywordGroup()` 和 `RefreshData("keywords"/"all")` 之后调用 `SyncGroups`，传入最新的关键词分组 ID 列表。同时修复 `Reload()` 方法在重启时也使用最新分组。

**Tech Stack:** Go

---

### Task 1: TitleGenerator 新增 SyncGroups 方法

**Files:**
- Modify: `api/internal/service/title_generator.go` (在 `Reload()` 方法之后，约 line 246)

**Step 1: 添加 SyncGroups 方法**

在 `title_generator.go` 的 `Reload()` 方法之后添加：

```go
// SyncGroups 同步分组：为新增的关键词分组创建标题池和 worker，移除已删除的分组
func (g *TitleGenerator) SyncGroups(groupIDs []int) {
	if g.stopped.Load() {
		return
	}

	// 构建目标分组集合
	target := make(map[int]struct{}, len(groupIDs))
	for _, gid := range groupIDs {
		target[gid] = struct{}{}
	}

	g.mu.RLock()
	// 找出需要新增的分组
	var toAdd []int
	for _, gid := range groupIDs {
		if _, exists := g.pools[gid]; !exists {
			toAdd = append(toAdd, gid)
		}
	}
	g.mu.RUnlock()

	// 为新分组创建池和启动 worker
	for _, gid := range toAdd {
		pool := g.getOrCreatePool(gid)
		g.fillPool(gid, pool)
		for i := 0; i < g.config.TitleWorkers; i++ {
			g.wg.Add(1)
			go g.refillWorker(gid, pool)
		}
		log.Info().Int("group_id", gid).Msg("TitleGenerator: added new group")
	}
}
```

说明：
- 只处理新增分组（创建池 + 启动 worker），不处理删除（删除分组的 worker 会因关键词为空而自动跳过填充，池会逐渐被消费为空，不需要主动清理）
- `getOrCreatePool` 内部有锁保护，线程安全
- `refillWorker` 内部检查 `g.ctx.Done()` 和 `g.stopped`，生命周期由 TitleGenerator 整体管理

**Step 2: Commit**

```bash
git add api/internal/service/title_generator.go
git commit -m "feat(title): add SyncGroups method for dynamic group addition"
```

---

### Task 2: PoolManager 在关键词变更后同步 TitleGenerator

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 修改 ReloadKeywordGroup 方法（line 467-469）**

将：

```go
func (m *PoolManager) ReloadKeywordGroup(ctx context.Context, groupID int) error {
	return m.poolManager.GetKeywordPool().ReloadGroup(ctx, groupID)
}
```

改为：

```go
func (m *PoolManager) ReloadKeywordGroup(ctx context.Context, groupID int) error {
	if err := m.poolManager.GetKeywordPool().ReloadGroup(ctx, groupID); err != nil {
		return err
	}
	// 同步 TitleGenerator 分组
	if m.titleGenerator != nil {
		m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
	}
	return nil
}
```

**Step 2: 修改 RefreshData 方法中 "keywords" 和 "all" 分支（line 864-878）**

将 "keywords" 分支（line 865-869）从：

```go
	case "keywords":
		groupIDs, _ := m.discoverKeywordGroups(ctx)
		if err := m.poolManager.GetKeywordPool().Reload(ctx, groupIDs); err != nil {
			return fmt.Errorf("reload keywords: %w", err)
		}
```

改为：

```go
	case "keywords":
		groupIDs, _ := m.discoverKeywordGroups(ctx)
		if err := m.poolManager.GetKeywordPool().Reload(ctx, groupIDs); err != nil {
			return fmt.Errorf("reload keywords: %w", err)
		}
		if m.titleGenerator != nil {
			m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
		}
```

将 "all" 分支（line 875-878）从：

```go
	case "all":
		if err := m.poolManager.ReloadAll(ctx); err != nil {
			return fmt.Errorf("reload all pools: %w", err)
		}
```

改为：

```go
	case "all":
		if err := m.poolManager.ReloadAll(ctx); err != nil {
			return fmt.Errorf("reload all pools: %w", err)
		}
		if m.titleGenerator != nil {
			m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
		}
```

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): sync TitleGenerator groups after keyword reload"
```

---

### Task 3: 修复 Reload 方法使用最新分组

**Files:**
- Modify: `api/internal/service/title_generator.go` (`Reload` 方法, line 206-246)

**Step 1: 修改 Reload 方法，重启时从 poolManager 获取最新分组**

需要让 `Reload` 能获取最新关键词分组。由于 `TitleGenerator` 已持有 `poolManager *PoolManager` 引用，可以直接调用。

将 `Reload` 方法（line 206-246）从：

```go
func (g *TitleGenerator) Reload(config *CachePoolConfig) {
	g.mu.Lock()
	oldConfig := g.config
	g.config = config
	needRestart := config.TitlePoolSize != oldConfig.TitlePoolSize

	// 获取当前所有 groupID（在持有锁时）
	var groupIDs []int
	if needRestart {
		groupIDs = make([]int, 0, len(g.pools))
		for gid := range g.pools {
			groupIDs = append(groupIDs, gid)
		}
	}
	g.mu.Unlock()

	if needRestart && len(groupIDs) > 0 {
		log.Info().
			Int("old_size", oldConfig.TitlePoolSize).
			Int("new_size", config.TitlePoolSize).
			Ints("group_ids", groupIDs).
			Msg("Title pool size changed, restarting workers")

		// 1. 停止旧 worker
		g.stopped.Store(true)
		g.cancel()
		g.wg.Wait()

		// 2. 重置状态
		g.stopped.Store(false)
		g.ctx, g.cancel = context.WithCancel(context.Background())

		// 3. 清空旧池
		g.mu.Lock()
		g.pools = make(map[int]*TitlePool)
		g.mu.Unlock()

		// 4. 重新启动（会创建新池并启动 worker）
		g.Start(groupIDs)
	}
}
```

改为：

```go
func (g *TitleGenerator) Reload(config *CachePoolConfig) {
	g.mu.Lock()
	oldConfig := g.config
	g.config = config
	needRestart := config.TitlePoolSize != oldConfig.TitlePoolSize
	g.mu.Unlock()

	if needRestart {
		// 从关键词池获取最新分组（而非复用旧分组）
		groupIDs := g.poolManager.GetKeywordGroupIDs()
		if len(groupIDs) == 0 {
			groupIDs = []int{1}
		}

		log.Info().
			Int("old_size", oldConfig.TitlePoolSize).
			Int("new_size", config.TitlePoolSize).
			Ints("group_ids", groupIDs).
			Msg("Title pool size changed, restarting workers")

		// 1. 停止旧 worker
		g.stopped.Store(true)
		g.cancel()
		g.wg.Wait()

		// 2. 重置状态
		g.stopped.Store(false)
		g.ctx, g.cancel = context.WithCancel(context.Background())

		// 3. 清空旧池
		g.mu.Lock()
		g.pools = make(map[int]*TitlePool)
		g.mu.Unlock()

		// 4. 重新启动（使用最新分组）
		g.Start(groupIDs)
	}
}
```

关键改动：
- 删除在锁内收集旧 `groupIDs` 的代码
- 改为从 `g.poolManager.GetKeywordGroupIDs()` 获取最新分组
- 加上与 `Start()` 初始化逻辑一致的空列表保护 `if len == 0 → [1]`

**Step 2: Commit**

```bash
git add api/internal/service/title_generator.go
git commit -m "fix(title): use latest keyword groups on Reload instead of stale pool keys"
```

---

### Task 4: 验证部署

**Step 1: 构建并重启**

```bash
docker compose up -d --build api
```

**Step 2: 验证日志**

查看 API 启动日志确认 TitleGenerator 启动时分组数与关键词分组数一致：

```bash
docker compose logs api --tail=20 | grep -i "TitleGenerator\|keyword_groups"
```

预期：`Starting TitleGenerator group_ids=[1 2]`（两个分组，因为两个关键词分组都有数据）

**Step 3: 验证前端**

打开缓存管理页面，确认标题卡片下方出现"分组详情 (2 个分组)"折叠面板。

**Step 4: 测试热更新**

1. 在管理后台上传新关键词到某个新分组
2. 检查 API 日志是否出现 `TitleGenerator: added new group`
3. 刷新缓存管理页面，确认分组数量更新
