# HTML 缓存统计性能优化设计

## 问题

缓存管理页面加载 HTML 缓存统计数据很慢。原因是 `GetStats()` 每次请求都遍历整个缓存目录两次（统计文件数和大小），百万级文件时需要几十秒。

## 解决方案

采用**增量统计**方案：在文件写入/删除时实时更新内存计数器，API 直接读取计数器（O(1)）。

## 数据结构

```go
type CacheStats struct {
    totalFiles  atomic.Int64  // 文件总数
    totalBytes  atomic.Int64  // 总字节数
    initialized atomic.Bool   // 是否完成初始化
    lastScanAt  atomic.Int64  // 上次扫描时间戳
}
```

## 实现要点

### 1. 启动初始化
- 后台 goroutine 异步扫描，不阻塞服务启动
- 使用 `filepath.WalkDir` 单次遍历统计文件数和大小
- 扫描完成后设置 `initialized = true`

### 2. 增量更新
- Set() 写入文件时：`files++`, `bytes += size`
- Delete() 删除文件时：`files--`, `bytes -= size`
- Clear() 清空时：`files = 0`, `bytes = 0`

### 3. API 响应
- `GET /api/cache/stats` 直接返回内存计数器，<1ms
- 返回 `initialized` 字段，前端据此显示"统计中..."

### 4. 手动校验
- `POST /api/cache/stats/recalculate` 用户触发重新计算
- 前端 HTML 缓存卡片添加"重新计算"按钮

## 性能对比

| 场景 | 原方案 | 新方案 |
|------|--------|--------|
| 1万文件 | ~500ms | <1ms |
| 100万文件 | ~30s | <1ms |

## 修改文件

- `api/internal/service/html_cache.go` - 添加 CacheStats，修改 Set/Delete/GetStats
- `api/internal/handler/cache.go` - 添加 Recalculate handler
- `api/cmd/main.go` - 注册新路由
- `web/src/api/settings.ts` - 添加 recalculate API
- `web/src/views/cache/CacheManage.vue` - 添加重新计算按钮
