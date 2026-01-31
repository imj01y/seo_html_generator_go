# 异步模板预热 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 服务启动后异步预热所有模板编译缓存，消除首次访问的延迟

**Architecture:** 在 main.go 中 pageHandler 创建后启动 goroutine，遍历所有已缓存的模板，调用 TemplateRenderer.Render() 触发编译。预热不阻塞服务启动，请求在预热完成前到达时走正常编译流程。

**Tech Stack:** Go, sync.Map, goroutine

---

## Task 1: 新增 TemplateCache.Range 方法

**Files:**
- Modify: `api/internal/service/template_cache.go:309` (文件末尾)

**Step 1: 添加 Range 方法**

在 `template_cache.go` 文件末尾（第 309 行后）添加：

```go
// Range 遍历所有缓存的模板
// 回调函数返回 false 时停止遍历
func (tc *TemplateCache) Range(fn func(tmpl *models.Template) bool) {
	tc.cache.Range(func(key, value interface{}) bool {
		if tmpl, ok := value.(*models.Template); ok && tmpl != nil {
			return fn(tmpl)
		}
		return true // 跳过 nil 值，继续遍历
	})
}
```

**Step 2: 验证编译**

Run: `cd api && go build ./...`
Expected: 编译成功，无错误

**Step 3: Commit**

```bash
git add api/internal/service/template_cache.go
git commit -m "feat(template_cache): add Range method for template iteration"
```

---

## Task 2: 在 main.go 中添加异步预热逻辑

**Files:**
- Modify: `api/cmd/main.go:262` (pageHandler 创建后)

**Step 1: 添加异步预热 goroutine**

在 `pageHandler := api.NewPageHandler(...)` 之后（约第 262 行后），添加：

```go
	// === 异步模板预热 ===
	go func() {
		log.Info().Msg("Starting async template warmup...")
		warmupStart := time.Now()
		warmupCount := 0

		templateCache.Range(func(tmpl *models.Template) bool {
			// 构造最小化渲染数据
			dummyData := &core.RenderData{
				Title:  "warmup",
				SiteID: 1,
			}
			// 触发模板编译和快速渲染器初始化
			_, err := pageHandler.GetTemplateRenderer().Render(
				tmpl.Content, tmpl.Name, dummyData)
			if err != nil {
				log.Warn().
					Err(err).
					Str("template", tmpl.Name).
					Msg("Template warmup failed")
			} else {
				warmupCount++
			}
			return true // 继续遍历
		})

		log.Info().
			Int("count", warmupCount).
			Dur("duration", time.Since(warmupStart)).
			Msg("Async template warmup completed")
	}()
```

**Step 2: 确认导入**

确保 `api/cmd/main.go` 已导入 `core` 包（应该已有）：
```go
core "seo-generator/api/internal/service"
```

**Step 3: 验证编译**

Run: `cd api && go build ./cmd/main.go`
Expected: 编译成功，无错误

**Step 4: Commit**

```bash
git add api/cmd/main.go
git commit -m "feat(startup): add async template warmup on service start"
```

---

## Task 3: 本地测试验证

**Step 1: 启动服务观察日志**

Run: `cd api && go run ./cmd/main.go`

Expected 日志输出（顺序）：
```
... All templates loaded into cache
... Starting async template warmup...
... Server starting addr=0.0.0.0:8080
... Async template warmup completed count=N duration=XXXms
```

关键验证点：
- "Server starting" 在 "warmup completed" 之前出现（证明异步不阻塞）
- warmup count > 0（证明有模板被预热）

**Step 2: 测试首次请求速度**

重启服务后，立即发起请求：

Run: `curl -w "Time: %{time_total}s\n" "http://127.0.0.1:8080/page?ua=Baiduspider&path=/test.html&domain=example.com"`

对比预热前后的 `resp_time`（在蜘蛛日志中查看）

**Step 3: Final Commit**

```bash
git add -A
git commit -m "feat: async template warmup for faster first request

- Add TemplateCache.Range() method for iteration
- Start warmup goroutine after pageHandler creation
- Warmup runs in background, does not block server start
- Requests during warmup use normal compilation path"
```

---

## 验收标准

1. 服务启动日志显示 "Async template warmup completed"
2. 预热不阻塞服务启动（Server starting 先于 warmup completed）
3. 首次请求响应时间显著降低（< 100ms vs 之前的 > 500ms）
