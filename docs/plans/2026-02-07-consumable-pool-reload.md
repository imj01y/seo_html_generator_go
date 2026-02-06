# 标题/正文缓存重载按钮 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 为标题和正文缓存池添加重载按钮（整体重载 + 分组重载），与关键词/图片缓存的交互方式一致。

**Architecture:** 后端在 TitleGenerator 新增 `ForceReload()` 和 `ReloadGroup()` 方法，PoolManager 新增 `RefreshTitles()`、`RefreshContents()`、`ReloadContentGroup()` 方法并补充 RefreshData 的 titles/contents 分支。前端 PoolStatusCard 让消费型池也显示重载按钮，CacheManage 补充 poolMap 映射。handler 层 dataRefreshHandler 增加 titles/contents 独立 case 支持 group_id 参数。

**Tech Stack:** Go, Vue 3, TypeScript, Element Plus

---

### Task 1: TitleGenerator 新增 ForceReload 和 ReloadGroup 方法

**Files:**
- Modify: `api/internal/service/title_generator.go`

**Step 1: 在 `Reload()` 方法之后（line 244）添加 `ForceReload` 方法**

```go
// ForceReload 强制重载所有标题池（不依赖配置变化）
func (g *TitleGenerator) ForceReload() {
	groupIDs := g.poolManager.GetKeywordGroupIDs()
	if len(groupIDs) == 0 {
		groupIDs = []int{1}
	}

	log.Info().Ints("group_ids", groupIDs).Msg("TitleGenerator: force reloading all pools")

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

	// 4. 重新启动
	g.Start(groupIDs)
}
```

**Step 2: 在 `ForceReload` 之后添加 `ReloadGroup` 方法**

```go
// ReloadGroup 重载指定分组的标题池（清空并重新填充，不重启 worker）
func (g *TitleGenerator) ReloadGroup(groupID int) {
	if g.stopped.Load() {
		return
	}

	pool := g.getOrCreatePool(groupID)

	// 排空 channel 中的旧数据
	drained := 0
	var drainedMem int64
	for {
		select {
		case title := <-pool.ch:
			drained++
			drainedMem += StringMemorySize(title)
		default:
			goto done
		}
	}
done:
	if drainedMem > 0 {
		pool.memoryBytes.Add(-drainedMem)
	}

	// 重新填充
	g.fillPool(groupID, pool)

	log.Info().Int("group_id", groupID).Int("drained", drained).Msg("TitleGenerator: reloaded group")
}
```

**Step 3: Commit**

```bash
git add api/internal/service/title_generator.go
git commit -m "feat(title): add ForceReload and ReloadGroup methods"
```

---

### Task 2: PoolManager 新增正文/标题重载方法 + 补充 RefreshData

**Files:**
- Modify: `api/internal/service/pool_manager.go`

**Step 1: 在 `RefreshData` 方法之前（line 872 附近）添加正文重载方法**

```go
// RefreshContents 重载所有正文缓存池（清空并重新从数据库加载）
func (m *PoolManager) RefreshContents(ctx context.Context) {
	m.mu.RLock()
	pools := make([]*MemoryPool, 0, len(m.contents))
	for _, p := range m.contents {
		pools = append(pools, p)
	}
	m.mu.RUnlock()

	for _, p := range pools {
		p.Clear()
		m.refillPool(p)
	}

	log.Info().Int("groups", len(pools)).Msg("All content pools reloaded")
}

// ReloadContentGroup 重载指定分组的正文缓存池
func (m *PoolManager) ReloadContentGroup(ctx context.Context, groupID int) {
	memPool := m.getOrCreatePool("contents", groupID)
	memPool.Clear()
	m.refillPool(memPool)

	log.Info().Int("group_id", groupID).Msg("Content pool group reloaded")
}
```

**Step 2: 修改 `RefreshData` 方法（line 874-896），在 switch 中补充 titles 和 contents 分支**

将：

```go
	case "all":
		if err := m.poolManager.ReloadAll(ctx); err != nil {
			return fmt.Errorf("reload all pools: %w", err)
		}
		if m.titleGenerator != nil {
			m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
		}
	}
```

改为：

```go
	case "titles":
		if m.titleGenerator != nil {
			m.titleGenerator.ForceReload()
		}
	case "contents":
		m.RefreshContents(ctx)
	case "all":
		if err := m.poolManager.ReloadAll(ctx); err != nil {
			return fmt.Errorf("reload all pools: %w", err)
		}
		if m.titleGenerator != nil {
			m.titleGenerator.SyncGroups(m.GetKeywordGroupIDs())
		}
		m.RefreshContents(ctx)
	}
```

注意 "all" 分支也要加上 `RefreshContents`，确保全量刷新时正文也被重载。

**Step 3: Commit**

```bash
git add api/internal/service/pool_manager.go
git commit -m "feat(pool): add titles/contents reload methods and RefreshData branches"
```

---

### Task 3: dataRefreshHandler 增加 titles/contents 独立 case

**Files:**
- Modify: `api/internal/handler/router.go` (line 718-738)

**Step 1: 修改 switch 语句，为 titles 和 contents 增加独立 case**

将：

```go
		case "emojis":
			deps.PoolManager.ReloadEmojis("data/emojis.json")
		default:
			if err := deps.PoolManager.RefreshData(ctx, req.Pool); err != nil {
				core.FailWithMessage(c, core.ErrInternalServer, err.Error())
				return
			}
```

改为：

```go
		case "titles":
			if req.GroupID != nil {
				if deps.PoolManager.GetTitleGenerator() != nil {
					deps.PoolManager.GetTitleGenerator().ReloadGroup(*req.GroupID)
				}
			} else {
				deps.PoolManager.RefreshData(ctx, "titles")
			}
		case "contents":
			if req.GroupID != nil {
				deps.PoolManager.ReloadContentGroup(ctx, *req.GroupID)
			} else {
				deps.PoolManager.RefreshData(ctx, "contents")
			}
		case "emojis":
			deps.PoolManager.ReloadEmojis("data/emojis.json")
		default:
			if err := deps.PoolManager.RefreshData(ctx, req.Pool); err != nil {
				core.FailWithMessage(c, core.ErrInternalServer, err.Error())
				return
			}
```

**Step 2: 检查 PoolManager 是否有 `GetTitleGenerator()` 方法，如果没有则在 `pool_manager.go` 中添加**

在 pool_manager.go 中搜索 `GetTitleGenerator`，如果不存在则添加：

```go
// GetTitleGenerator 返回 TitleGenerator 实例
func (m *PoolManager) GetTitleGenerator() *TitleGenerator {
	return m.titleGenerator
}
```

**Step 3: Commit**

```bash
git add api/internal/handler/router.go api/internal/service/pool_manager.go
git commit -m "feat(handler): add titles/contents reload with group_id support"
```

---

### Task 4: 前端 PoolStatusCard 消费型池显示重载按钮

**Files:**
- Modify: `web/src/components/PoolStatusCard.vue`

**Step 1: 修改 card-header 部分（line 3-16），让消费型池同时显示状态徽标和重载按钮**

将：

```vue
    <div class="card-header">
      <span class="pool-name">{{ pool.name }}</span>
      <template v-if="pool.pool_type === 'reusable' || pool.pool_type === 'static'">
        <el-button size="small" @click="handleReload">
          重载
        </el-button>
      </template>
      <template v-else>
        <span :class="['status-badge', `status-${pool.status}`]">
          <span class="status-icon">{{ statusIcon }}</span>
          {{ statusText }}
        </span>
      </template>
    </div>
```

改为：

```vue
    <div class="card-header">
      <span class="pool-name">{{ pool.name }}</span>
      <template v-if="pool.pool_type === 'reusable' || pool.pool_type === 'static'">
        <el-button size="small" @click="handleReload">
          重载
        </el-button>
      </template>
      <template v-else>
        <span :class="['status-badge', `status-${pool.status}`]">
          <span class="status-icon">{{ statusIcon }}</span>
          {{ statusText }}
        </span>
        <el-button size="small" @click="handleReload">
          重载
        </el-button>
      </template>
    </div>
```

**Step 2: 在消费型池的分组详情中，为每个分组增加重载按钮（line 54-69）**

将分组详情中的 `consumable-group-item` 的 `group-header` 部分：

```vue
              <div class="group-header">
                <span class="group-name">{{ group.name }}</span>
                <span class="group-util">{{ (group.utilization || 0).toFixed(0) }}%</span>
              </div>
```

改为：

```vue
              <div class="group-header">
                <span class="group-name">{{ group.name }}</span>
                <span class="group-util">{{ (group.utilization || 0).toFixed(0) }}%</span>
                <el-button size="small" link @click="handleReloadGroup(group.id)">
                  重载
                </el-button>
              </div>
```

**Step 3: Commit**

```bash
git add web/src/components/PoolStatusCard.vue
git commit -m "feat(web): add reload buttons for consumable pool cards"
```

---

### Task 5: CacheManage 补充 poolMap 映射

**Files:**
- Modify: `web/src/views/cache/CacheManage.vue`

**Step 1: 修改 `handlePoolReload` 方法（line 693-697），poolMap 增加标题和正文**

将：

```typescript
    const poolMap: Record<string, string> = {
      '关键词': 'keywords',
      '图片': 'images',
      '表情': 'emojis'
    }
```

改为：

```typescript
    const poolMap: Record<string, string> = {
      '关键词': 'keywords',
      '图片': 'images',
      '表情': 'emojis',
      '标题': 'titles',
      '正文': 'contents'
    }
```

**Step 2: 修改 `handlePoolReloadGroup` 方法（line 713-716），poolMap 增加标题和正文**

将：

```typescript
    const poolMap: Record<string, string> = {
      '关键词': 'keywords',
      '图片': 'images'
    }
```

改为：

```typescript
    const poolMap: Record<string, string> = {
      '关键词': 'keywords',
      '图片': 'images',
      '标题': 'titles',
      '正文': 'contents'
    }
```

**Step 3: 在消费型缓存卡片的 PoolStatusCard 组件上绑定 reload 和 reload-group 事件（line 44-48）**

将：

```vue
                  <PoolStatusCard
                    v-for="pool in consumablePoolStats"
                    :key="pool.name"
                    :pool="pool"
                  />
```

改为：

```vue
                  <PoolStatusCard
                    v-for="pool in consumablePoolStats"
                    :key="pool.name"
                    :pool="pool"
                    @reload="handlePoolReload(pool.name)"
                    @reload-group="(groupId: number) => handlePoolReloadGroup(pool.name, groupId)"
                  />
```

**Step 4: Commit**

```bash
git add web/src/views/cache/CacheManage.vue
git commit -m "feat(web): bind reload events for consumable pool cards"
```

---

### Task 6: 构建部署验证

**Step 1: 构建并重启**

```bash
docker compose up -d --build api web
```

**Step 2: 验证后端**

查看 API 日志确认标题和正文重载功能正常：

```bash
docker compose logs api --tail=20
```

**Step 3: 验证前端**

1. 打开缓存管理页面
2. 确认标题卡片和正文卡片的 header 区域同时显示状态徽标和重载按钮
3. 点击标题卡片的重载按钮，确认日志出现 `TitleGenerator: force reloading all pools`
4. 点击正文卡片的重载按钮，确认日志出现 `All content pools reloaded`
5. 展开标题分组详情，点击某个分组的重载按钮，确认日志出现 `TitleGenerator: reloaded group`
6. 展开正文分组详情，点击某个分组的重载按钮，确认日志出现 `Content pool group reloaded`
