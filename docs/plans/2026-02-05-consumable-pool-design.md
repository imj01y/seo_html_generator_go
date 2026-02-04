# ConsumablePool 通用消费型池设计

## 1. 背景与问题

### 1.1 当前问题

项目中 `ObjectPool` 使用环形缓冲区设计，存在数据重复问题：

```go
// object_pool.go - 当前实现
func (p *ObjectPool[T]) Get() T {
    idx := atomic.AddInt64(&p.head, 1) - 1
    return pool[idx % size]  // ⚠️ 取模导致循环重复
}
```

**影响范围**：
- `ClsPool` — CSS 类名会重复
- `URLPool` — 内链 URL 会重复
- `KeywordEmojiPool` — 关键词表情组合会重复
- `NumberPool` — 随机数会重复

**业务影响**：
- SEO 页面生成时，同一个 CSS 类名/URL 可能在不同页面重复出现
- 池大小 800,000，每页用 10 个 cls，渲染 80,000 页后开始循环

### 1.2 期望行为

- 每个数据只能被消费一次
- 消费后数据被移除，不会再出现
- 后台持续生成新数据补充

---

## 2. 设计方案

### 2.1 核心思路

复用项目中已有的 `TitleGenerator` 的 Channel 模式，设计通用泛型池：

```
TitleGenerator (现有)          →    ConsumablePool[T] (通用)
├─ ch chan string                   ├─ ch chan T
├─ generateTitle()                  ├─ generator func() T
├─ Pop() 消费                        ├─ Pop() 消费
├─ fillPool() 补充                   ├─ refill() 补充
└─ refillWorker() 后台              └─ refillLoop() 后台
```

### 2.2 数据结构

```go
// ConsumablePool 通用消费型池（基于 Channel）
type ConsumablePool[T any] struct {
    name      string
    ch        chan T        // Channel 天然 FIFO 消费
    generator func() T      // 生成函数

    // 配置
    size          int
    threshold     float64       // 补充阈值 (0-1)
    numWorkers    int           // 后台 worker 数量
    checkInterval time.Duration // 检查间隔

    // 内存追踪
    memorySizer func(T) int64
    memoryBytes atomic.Int64

    // 统计
    totalGenerated atomic.Int64
    totalConsumed  atomic.Int64
    refillCount    atomic.Int64
    lastRefresh    atomic.Int64

    // 控制
    ctx     context.Context
    cancel  context.CancelFunc
    wg      sync.WaitGroup
    stopped atomic.Bool
}
```

### 2.3 配置结构

```go
type ConsumablePoolConfig struct {
    Name          string
    Size          int             // 池容量
    Threshold     float64         // 补充阈值 (默认 0.4)
    NumWorkers    int             // 后台 worker 数 (默认 1)
    CheckInterval time.Duration   // 检查间隔 (默认 30ms)
    MemorySizer   func(T) int64   // 可选：内存计算函数
}
```

---

## 3. 核心方法

### 3.1 创建池

```go
func NewConsumablePool[T any](cfg ConsumablePoolConfig, generator func() T) *ConsumablePool[T] {
    ctx, cancel := context.WithCancel(context.Background())
    return &ConsumablePool[T]{
        name:          cfg.Name,
        ch:            make(chan T, cfg.Size),
        generator:     generator,
        size:          cfg.Size,
        threshold:     cfg.Threshold,
        numWorkers:    cfg.NumWorkers,
        checkInterval: cfg.CheckInterval,
        memorySizer:   cfg.MemorySizer,
        ctx:           ctx,
        cancel:        cancel,
    }
}
```

### 3.2 Pop() — 消费（核心改动）

```go
func (p *ConsumablePool[T]) Pop() T {
    select {
    case item := <-p.ch:
        // 从 channel 取出，真正消费
        p.totalConsumed.Add(1)
        if p.memorySizer != nil {
            p.memoryBytes.Add(-p.memorySizer(item))
        }
        return item
    default:
        // 池空，同步生成一个返回（降级策略）
        p.totalGenerated.Add(1)
        p.totalConsumed.Add(1)
        return p.generator()
    }
}
```

### 3.3 refill() — 补充

```go
func (p *ConsumablePool[T]) refill() {
    need := p.size - len(p.ch)
    if need <= 0 {
        return
    }

    filled := 0
    var addedMem int64
    for i := 0; i < need; i++ {
        item := p.generator()
        select {
        case p.ch <- item:
            filled++
            if p.memorySizer != nil {
                addedMem += p.memorySizer(item)
            }
        default:
            break // 池满
        }
    }

    p.totalGenerated.Add(int64(filled))
    if addedMem > 0 {
        p.memoryBytes.Add(addedMem)
    }
}
```

### 3.4 refillLoop() — 后台补充

```go
func (p *ConsumablePool[T]) refillLoop() {
    defer p.wg.Done()
    ticker := time.NewTicker(p.checkInterval)
    defer ticker.Stop()

    for {
        select {
        case <-p.ctx.Done():
            return
        case <-ticker.C:
            if p.stopped.Load() {
                return
            }
            thresholdCount := int(float64(p.size) * p.threshold)
            if len(p.ch) < thresholdCount {
                p.refill()
                p.refillCount.Add(1)
                p.lastRefresh.Store(time.Now().UnixNano())
            }
        }
    }
}
```

### 3.5 Start() — 启动

```go
func (p *ConsumablePool[T]) Start() {
    log.Info().Str("pool", p.name).Int("size", p.size).Msg("Starting consumable pool")

    // 初始填充
    p.refill()

    // 启动后台 worker
    for i := 0; i < p.numWorkers; i++ {
        p.wg.Add(1)
        go p.refillLoop()
    }

    log.Info().Str("pool", p.name).Msg("Consumable pool started")
}
```

### 3.6 Stop() — 停止

```go
func (p *ConsumablePool[T]) Stop() {
    if !p.stopped.CompareAndSwap(false, true) {
        return
    }
    p.cancel()
    p.wg.Wait()
    log.Info().Str("pool", p.name).Msg("Consumable pool stopped")
}
```

### 3.7 Stats() — 统计

```go
func (p *ConsumablePool[T]) Stats() map[string]interface{} {
    current := len(p.ch)
    return map[string]interface{}{
        "name":            p.name,
        "size":            p.size,
        "current":         current,
        "available_pct":   float64(current) / float64(p.size) * 100,
        "total_generated": p.totalGenerated.Load(),
        "total_consumed":  p.totalConsumed.Load(),
        "refill_count":    p.refillCount.Load(),
        "memory_bytes":    p.memoryBytes.Load(),
        "status":          p.statusString(),
    }
}
```

### 3.8 其他方法

```go
// Available 当前可用数量
func (p *ConsumablePool[T]) Available() int {
    return len(p.ch)
}

// Capacity 池容量
func (p *ConsumablePool[T]) Capacity() int {
    return p.size
}

// MemoryBytes 内存占用
func (p *ConsumablePool[T]) MemoryBytes() int64 {
    return p.memoryBytes.Load()
}

// Clear 清空池
func (p *ConsumablePool[T]) Clear() {
    for {
        select {
        case <-p.ch:
            // 丢弃
        default:
            p.memoryBytes.Store(0)
            return
        }
    }
}

// UpdateConfig 动态更新配置
func (p *ConsumablePool[T]) UpdateConfig(size int, threshold float64, numWorkers int, checkInterval time.Duration) {
    // 如果 size 变化，需要重建 channel
    // 其他配置可以热更新
}
```

---

## 4. 迁移计划

### 4.1 文件变更

| 操作 | 文件 |
|------|------|
| 新建 | `api/internal/service/consumable_pool.go` |
| 修改 | `api/internal/service/template_funcs.go` |
| 修改 | `api/internal/service/number_pool.go` |
| 修改 | `api/internal/service/title_generator.go` |
| 删除 | `api/internal/service/object_pool.go` (迁移完成后) |

### 4.2 迁移对照表

| 当前 | 迁移后 |
|------|--------|
| `ObjectPool[string]` for ClsPool | `ConsumablePool[string]` |
| `ObjectPool[string]` for URLPool | `ConsumablePool[string]` |
| `ObjectPool[string]` for KeywordEmojiPool | `ConsumablePool[string]` |
| `ObjectPool[int]` for NumberPool | `ConsumablePool[int]` |
| `TitlePool` (独立实现) | `ConsumablePool[string]` |

### 4.3 template_funcs.go 修改

**之前**：
```go
type TemplateFuncsManager struct {
    clsPool          *ObjectPool[string]
    urlPool          *ObjectPool[string]
    keywordEmojiPool *ObjectPool[string]
    // ...
}

func (m *TemplateFuncsManager) Cls(name string) string {
    return m.clsPool.Get() + " " + name  // Get() 会重复
}
```

**之后**：
```go
type TemplateFuncsManager struct {
    clsPool          *ConsumablePool[string]
    urlPool          *ConsumablePool[string]
    keywordEmojiPool *ConsumablePool[string]
    // ...
}

func (m *TemplateFuncsManager) Cls(name string) string {
    return m.clsPool.Pop() + " " + name  // Pop() 真正消费
}
```

### 4.4 TitleGenerator 改造

**之前**：
```go
type TitleGenerator struct {
    pools map[int]*TitlePool  // 独立实现的 TitlePool
    // ...
}

type TitlePool struct {
    ch          chan string
    groupID     int
    memoryBytes atomic.Int64
}
```

**之后**：
```go
type TitleGenerator struct {
    pools       map[int]*ConsumablePool[string]  // 复用通用池
    poolManager *PoolManager
    config      *CachePoolConfig
    mu          sync.RWMutex
}

func (g *TitleGenerator) getOrCreatePool(groupID int) *ConsumablePool[string] {
    g.mu.Lock()
    defer g.mu.Unlock()

    if pool, exists := g.pools[groupID]; exists {
        return pool
    }

    // 使用闭包捕获 groupID
    generator := func() string {
        return g.generateTitle(groupID)
    }

    pool := NewConsumablePool[string](ConsumablePoolConfig{
        Name:          fmt.Sprintf("title_%d", groupID),
        Size:          g.config.TitlePoolSize,
        Threshold:     g.config.TitleThreshold,
        NumWorkers:    g.config.TitleWorkers,
        CheckInterval: g.config.TitleRefillInterval(),
        MemorySizer:   StringMemorySizer,
    }, generator)

    pool.Start()
    g.pools[groupID] = pool
    return pool
}

func (g *TitleGenerator) Pop(groupID int) (string, error) {
    pool := g.getOrCreatePool(groupID)
    return pool.Pop(), nil
}
```

**简化效果**：TitleGenerator 从 ~270 行减少到 ~100 行

---

## 5. 配置参数

### 5.1 默认配置

| 池 | Size | Threshold | NumWorkers | CheckInterval |
|----|------|-----------|------------|---------------|
| ClsPool | 800,000 | 0.4 | 1 | 30ms |
| URLPool | 500,000 | 0.4 | 1 | 30ms |
| KeywordEmojiPool | 800,000 | 0.4 | 1 | 30ms |
| NumberPool (每个范围) | 200,000 | 0.4 | 1 | 50ms |
| TitlePool (每个 groupID) | 800,000 | 0.4 | 1 | 30ms |

### 5.2 NumWorkers 说明

Channel 模式下，`NumWorkers` 通常设为 1 即可：
- Channel 本身是线程安全的
- 单个 worker 足以处理补充任务
- 多个 worker 可能导致竞争

如果生成函数耗时较长（如网络请求），可以增加 worker 数量。

---

## 6. 与 ObjectPool 的对比

| 特性 | ObjectPool (复用型) | ConsumablePool (消费型) |
|------|-------------------|----------------------|
| 底层结构 | 环形缓冲区 `[]T` | Channel `chan T` |
| Get/Pop | 循环读取，会重复 | 真正消费，不重复 |
| 池空处理 | 读旧数据 | 同步生成新数据 |
| 并发模型 | 原子操作 | Channel 天然安全 |
| 内存 | 固定 `size * sizeof(T)` | 动态，按需分配 |

---

## 7. 风险与缓解

### 7.1 池空时的性能

**风险**：如果消费速度 > 生产速度，会频繁触发同步生成

**缓解**：
- 增大池容量
- 降低阈值（提前触发补充）
- 监控 `total_consumed` vs `total_generated`，及时调整配置

### 7.2 内存使用

**风险**：Channel 预分配容量，可能占用更多内存

**缓解**：
- 按需调整池大小
- 监控 `memory_bytes` 指标

### 7.3 迁移兼容性

**风险**：API 方法名变化 (`Get` → `Pop`)

**缓解**：
- 保留 `Get()` 作为 `Pop()` 的别名（可选）
- 或一次性修改所有调用点

---

## 8. 测试计划

### 8.1 单元测试

- [ ] Pop() 真正消费，不重复
- [ ] 池空时降级到同步生成
- [ ] 后台补充正常工作
- [ ] 阈值触发正确
- [ ] Stop() 优雅关闭
- [ ] 内存追踪准确

### 8.2 集成测试

- [ ] 高并发消费场景
- [ ] 长时间运行稳定性
- [ ] 配置热更新

### 8.3 性能测试

- [ ] 对比 ObjectPool 的吞吐量
- [ ] 内存占用对比
- [ ] 延迟分布

---

## 9. 实施步骤

1. **创建 ConsumablePool** — 新建 `consumable_pool.go`
2. **单元测试** — 验证核心功能
3. **迁移 ClsPool** — 修改 template_funcs.go
4. **迁移 URLPool** — 修改 template_funcs.go
5. **迁移 KeywordEmojiPool** — 修改 template_funcs.go
6. **迁移 NumberPool** — 修改 number_pool.go
7. **迁移 TitleGenerator** — 简化代码
8. **集成测试** — 验证整体功能
9. **删除 ObjectPool** — 清理旧代码
10. **文档更新** — 更新 API 文档

---

## 10. 总结

通过引入 `ConsumablePool[T]` 通用消费型池：

1. **解决数据重复问题** — Channel 天然 FIFO，消费即删除
2. **代码复用** — 统一池管理逻辑，减少重复代码约 300 行
3. **易于维护** — 单一实现，改进自动应用到所有池
4. **保持性能** — Channel 高效，池空时降级策略保证可用性
