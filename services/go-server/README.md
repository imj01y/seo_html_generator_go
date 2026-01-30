# Go Page Server

高性能 /page 接口的 Go 实现，使用 **quicktemplate** 编译时渲染，获得 6x 性能提升。

## 核心特性

- **quicktemplate 编译时渲染**: 模板编译为原生 Go 代码，渲染时间从 15ms 降至 2.5ms
- **Jinja2 语法兼容**: 自动转换 Jinja2 模板语法，用户无感知
- **在线编译 API**: 支持在线验证、预览、编译模板
- **热重载**: 编译后自动重载服务（Unix/Docker 环境）
- **4 层模板验证**: Jinja2 语法 → 函数白名单 → qtc 编译 → go build

## 项目结构

```
go-page-server/
├── main.go                         # 入口，Gin路由，生命周期管理
├── go.mod                          # Go 模块定义
├── Dockerfile                      # Docker 构建文件
├── docker-compose.go-server.yml    # Docker Compose 配置
├── nginx/
│   └── go-server.conf              # Nginx 配置示例
├── config/
│   └── config.go                   # 配置加载
├── database/
│   └── db.go                       # MySQL 连接池
├── core/
│   ├── spider_detector.go          # 蜘蛛检测
│   ├── encoder.go                  # HTML 实体编码
│   ├── template_converter.go       # Jinja2 → Go 模板语法转换
│   ├── template_funcs.go           # 模板函数实现
│   ├── template_renderer.go        # 模板渲染引擎
│   ├── quicktemplate_renderer.go   # quicktemplate 渲染器
│   ├── jinja2_to_quicktemplate.go  # Jinja2 → quicktemplate 转换器
│   ├── template_validator.go       # 模板验证器（4层检测）
│   ├── reload.go                   # 热重载管理器
│   ├── data_manager.go             # 关键词/图片/正文数据管理
│   ├── site_cache.go               # 站点配置缓存
│   └── html_cache.go               # HTML 文件缓存
├── handlers/
│   ├── page.go                     # /page 接口处理函数
│   └── compile.go                  # 模板编译 API
├── templates/
│   └── page.qtpl                   # quicktemplate 模板
└── models/
    └── models.go                   # 数据模型定义
```

## 构建

```bash
# 安装依赖
go mod tidy

# 编译
go build -o go-page-server

# 或使用交叉编译
# Linux
GOOS=linux GOARCH=amd64 go build -o go-page-server-linux

# Windows
GOOS=windows GOARCH=amd64 go build -o go-page-server.exe
```

## 运行

确保 MySQL 数据库已启动，且 Python 项目的 config.yaml 配置正确。

```bash
# 从 go-page-server 目录运行
./go-page-server

# 或指定配置文件路径（默认读取父目录的 config.yaml）
./go-page-server
```

服务器默认监听端口 **8001**（可在配置中修改）。

## API 端点

### 页面渲染

| 端点 | 说明 |
|------|------|
| `GET /page?ua=...&path=...&domain=...` | 页面生成接口 |
| `GET /health` | 健康检查 |
| `GET /stats` | 统计信息 |

### 模板编译 API

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/template/validate` | POST | 验证 Jinja2 模板语法 |
| `/api/template/preview` | POST | 预览转换后的 quicktemplate |
| `/api/template/compile` | POST | 编译并部署模板 |
| `/api/template/compile/status` | GET | 检查编译环境状态 |

#### 验证模板

```bash
curl -X POST http://localhost:8080/api/template/validate \
  -H "Content-Type: application/json" \
  -d '{"content": "<div>{{ title }}</div>"}'
```

响应:
```json
{
  "valid": true,
  "errors": [],
  "warnings": []
}
```

#### 预览转换

```bash
curl -X POST http://localhost:8080/api/template/preview \
  -H "Content-Type: application/json" \
  -d '{"content": "{% for i in range(10) %}{{ random_url() }}{% endfor %}"}'
```

响应:
```json
{
  "valid": true,
  "quicktemplate": "{% for i := 0; i < 10; i++ %}{%s p.RandomURL() %}{% endfor %}",
  "warnings": []
}
```

#### 编译模板

```bash
curl -X POST http://localhost:8080/api/template/compile \
  -H "Content-Type: application/json" \
  -d '{"template_id": 1}'
```

响应:
```json
{
  "success": true,
  "message": "编译成功，耗时 2.5s，服务正在重载"
}
```

## Nginx 配置

混合架构部署，Go 处理 /page，Python 处理其他 API：

```nginx
upstream go_backend {
    server 127.0.0.1:8001;
    keepalive 32;
}

upstream python_backend {
    server 127.0.0.1:8000;
    keepalive 16;
}

server {
    listen 80;
    server_name example.com;

    # /page 路由到 Go 服务
    location /page {
        proxy_pass http://go_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    # 其他路由到 Python 服务
    location / {
        proxy_pass http://python_backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }
}
```

## 性能对比

### quicktemplate vs strings.Replacer

| 渲染方式 | 平均耗时 | 提升倍数 |
|----------|----------|----------|
| strings.Replacer | ~15ms | 基准 |
| quicktemplate | ~2.5ms | **6x** |

### 整体性能对比

| 指标 | Python | Go (quicktemplate) |
|------|--------|-----|
| 渲染时间 | ~330ms | ~40-60ms |
| 缓存命中 | 2-6ms | 0.4-1.5ms |
| 吞吐量 (单核) | ~10 QPS | ~80-150 QPS |
| 吞吐量 (8核) | ~530 QPS | ~1600-2600 QPS |

## 配置说明

Go 服务读取 Python 项目的 `config.yaml`，支持以下配置项：

```yaml
default:
  server:
    host: "127.0.0.1"
    port: 8001  # Go 服务端口
    debug: false

  database:
    host: "localhost"
    port: 3306
    user: "root"
    password: "your_password"
    database: "seo_html_generator"

  cache:
    enabled: true
    max_size_gb: 10.0

  spider_detector:
    enabled: true
    return_404_for_non_spider: true
```

## 技术栈

- **Web 框架**: Gin
- **模板引擎**: quicktemplate (编译时渲染)
- **数据库**: sqlx + go-sql-driver/mysql
- **日志**: zerolog
- **缓存**: go-cache (内存) + 文件系统

## Docker 部署

### 构建镜像

```bash
cd go-page-server
docker build -t seo-generator-go .
```

### Docker Compose 部署

将 `docker-compose.go-server.yml` 合并到主 docker-compose.yml：

```bash
# 启动服务
docker-compose up -d go-server

# 查看日志
docker-compose logs -f go-server
```

### Nginx 配置

将 `nginx/go-server.conf` 内容添加到 Nginx 配置：

```nginx
# Go 服务上游
upstream go_backend {
    server go-server:8080;
    keepalive 32;
}

# /page 路由到 Go 服务
location /page {
    proxy_pass http://go_backend;
    proxy_http_version 1.1;
    # ... 详见 nginx/go-server.conf
}
```

## quicktemplate 模板开发

### 支持的 Jinja2 语法

| Jinja2 语法 | quicktemplate 转换 |
|-------------|-------------------|
| `{{ var }}` | `{%s p.Var %}` |
| `{{ func() }}` | `{%s p.Func() %}` |
| `{% for i in range(n) %}...{% endfor %}` | `{% for i := 0; i < n; i++ %}...{% endfor %}` |
| `{{ var or 'default' }}` | `{% if p.Var != "" %}{%s p.Var %}{% endif %}` |
| `{{ cls('name') }}` | `{%s p.Cls("name") %}` |

### 支持的模板函数

| 函数 | 说明 |
|------|------|
| `cls(name)` | 获取随机 CSS 类名 |
| `random_url()` | 生成随机 URL |
| `random_keyword()` | 获取随机关键词 |
| `keyword_with_emoji()` | 获取带 emoji 的关键词 |
| `random_image()` | 获取随机图片 URL |
| `random_hotspot()` | 获取随机热点词 |
| `random_number(min, max)` | 生成指定范围的随机数 |
| `now()` | 获取当前时间字符串 |

### 运行测试

```bash
cd go-page-server

# 运行所有测试
go test ./...

# 运行特定测试
go test ./core -v -run TestJinja2ToQuickTemplate
go test ./handlers -v -run TestValidateTemplateEndpoint

# 运行性能基准测试
go test ./core -bench=. -benchmem
```

## 注意事项

1. Go 服务与 Python 服务共享同一个 MySQL 数据库
2. HTML 缓存目录也是共享的（`html_cache/`）
3. 模板语法从 Jinja2 自动转换为 quicktemplate
4. 关键词在加载时预编码，避免每次渲染时重复编码
5. 编译模板需要容器内安装 `qtc` 和 Go 工具链
6. Windows 环境下编译后需手动重启服务，Docker/Linux 环境支持自动重载
