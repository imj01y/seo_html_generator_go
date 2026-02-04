# Go API 重构指南

## 概述

本文档说明 2026-02 重构后的代码架构和最佳实践。

## 新架构

### 分层架构

```
Handler 层 (HTTP 请求处理)
    ↓
Service 层 (业务逻辑)
    ↓
Repository 层 (数据访问)
    ↓
Database (MySQL)
```

### 目录结构

```
api/
├── cmd/
│   └── main.go              # 应用入口
├── internal/
│   ├── di/                  # 依赖注入容器
│   │   └── container.go
│   ├── handler/             # HTTP 处理器
│   │   ├── error_middleware.go
│   │   └── *.go
│   ├── model/               # 数据模型
│   ├── repository/          # 数据访问层
│   │   ├── interfaces.go    # 接口定义
│   │   ├── filters.go       # 查询过滤器
│   │   └── *_repo.go        # 实现
│   ├── service/             # 业务逻辑层
│   │   ├── pool/            # 池管理（重构后）
│   │   └── *.go
│   └── testing/             # 测试工具
├── pkg/
│   └── config/              # 配置管理
└── test/
    └── integration/         # 集成测试
```

## 依赖注入

所有依赖通过 DI 容器管理：

```go
// 创建容器
container := di.NewContainer(db, cfg)
defer container.Close()

// 获取 Repository（懒加载单例）
repo := container.GetKeywordRepository()

// 设置 Redis（可选）
container.SetRedis(redisClient)
```

### 可用的 Repository

- `GetSiteRepository()` - 站点
- `GetKeywordRepository()` - 关键词
- `GetImageRepository()` - 图片
- `GetArticleRepository()` - 文章
- `GetTitleRepository()` - 标题
- `GetContentRepository()` - 正文

## Repository 模式

所有数据库访问统一通过 Repository 接口：

```go
type KeywordRepository interface {
    Create(ctx context.Context, keyword *model.Keyword) error
    GetByID(ctx context.Context, id uint) (*model.Keyword, error)
    List(ctx context.Context, filter KeywordFilter) ([]*model.Keyword, int64, error)
    // ...
}
```

### 查询过滤器

使用类型安全的过滤器：

```go
filter := repository.KeywordFilter{
    GroupID:    &groupID,
    Status:     &status,
    Pagination: repository.NewPagination(page, pageSize),
}
keywords, total, err := repo.List(ctx, filter)
```

## 错误处理

### AppError 类型

统一使用 AppError 包装所有错误：

```go
// 创建错误
if err != nil {
    return core.NewDatabaseError(err)
}

// 验证错误
return core.NewValidationError("invalid input")

// 未找到错误
return core.NewNotFoundError("keyword not found")
```

### 错误中间件

HTTP 层自动将 AppError 转换为正确的状态码：

| 错误类型 | HTTP 状态码 |
|---------|------------|
| ValidationError | 422 |
| NotFoundError | 404 |
| UnauthorizedError | 401 |
| ForbiddenError | 403 |
| DatabaseError | 500 |
| PoolExhaustedError | 503 |

## 池管理（重构后）

### UpdateBatcher

解决 channel 消息丢失问题：

```go
// 配置
batcherConfig := pool.BatcherConfig{
    MaxBatch:      100,           // 最大批量大小
    FlushInterval: 5 * time.Second, // 刷新间隔
}

// 使用
batcher := pool.NewUpdateBatcher(db, batcherConfig)
batcher.Add(pool.UpdateTask{Table: "keywords", ID: id})
```

### 连接池优化

```go
db.SetMaxOpenConns(maxConns)
db.SetMaxIdleConns(maxConns / 5)  // 20% 空闲连接
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(2 * time.Minute)
```

## 测试

### 单元测试

使用 mock 数据库：

```go
db, mock, cleanup := testutil.NewMockDB(t)
defer cleanup()

repo := repository.NewKeywordRepository(db)

// 设置期望
mock.ExpectQuery("SELECT").WillReturnRows(...)

// 执行测试
result, err := repo.GetByID(ctx, 1)
```

### 集成测试

使用 `integration` build tag：

```bash
# 运行集成测试（需要真实数据库）
go test -tags=integration ./test/integration/... -v
```

## 迁移指南

### 从旧代码迁移

**旧代码:**
```go
db := database.GetDB()
rows, err := db.Query("SELECT * FROM keywords WHERE id = ?", id)
// 手动处理错误和扫描...
```

**新代码:**
```go
repo := container.GetKeywordRepository()
keyword, err := repo.GetByID(ctx, id)
if err != nil {
    return core.NewDatabaseError(err)
}
```

## 最佳实践

1. **使用 Repository** - 永远不要在 Handler 或 Service 中直接写 SQL
2. **错误传播** - 使用 AppError 包装所有错误
3. **依赖注入** - 通过参数传递依赖，不使用全局变量
4. **测试优先** - 先写测试再写实现
5. **接口抽象** - 定义接口以支持 Mock 测试
6. **Context 传递** - 所有数据库操作使用 context

## 向后兼容

旧代码通过兼容层继续工作：

- `api/internal/service/pool_manager.go` 委托到新的池实现
- 全局 `database.GetDB()` 仍然可用（但不推荐）

## 性能考虑

- **批量操作** - 使用 UpdateBatcher 减少数据库压力
- **连接池** - 优化的空闲连接设置减少内存占用
- **懒加载** - Repository 单例懒加载，减少启动时间
