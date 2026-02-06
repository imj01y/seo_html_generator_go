# Nginx 端口 .env 统一配置 + 多实例支持

## 目标

实现所有服务端口通过 `.env` 单文件配置，支持同一台机器上运行多套独立环境。

## 架构

将 Nginx 配置文件改为 `.template` 模板，用 `${VAR}` 占位符替代硬编码端口。容器启动前通过 `entrypoint.sh`（sed 替换）生成实际配置文件。去掉所有 `container_name` 硬编码，让 Docker Compose 按项目名自动隔离。

## 环境变量（.env）

```env
PAGE_PORT=8009       # Nginx 页面服务端口
ADMIN_PORT=8008      # Nginx 管理后台端口
API_PORT=8080        # Go API 端口
PPROF_PORT=6060      # pprof 调试端口
```

## 变更清单

### 1. Nginx 模板化

| 原文件 | 改为 | 变更 |
|--------|------|------|
| `docker/nginx/nginx.conf` | `nginx.conf.template` | `server api:8080` → `server api:${API_PORT}`，顶部加 `env API_PORT;` |
| `docker/nginx/conf.d/default.conf` | `default.conf.template` | `listen 8009` → `listen ${PAGE_PORT}`，`api:8080` → `api:${API_PORT}` |
| `docker/nginx/conf.d/admin.conf` | `admin.conf.template` | `listen 8008` → `listen ${ADMIN_PORT}`，`api:8080` → `api:${API_PORT}` |
| `docker/nginx/lua/cache_handler.lua` | 原地修改 | `connect(server_ip, 8080)` → `connect(server_ip, tonumber(os.getenv("API_PORT") or "8080"))` |

### 2. entrypoint.sh（新建）

路径：`docker/nginx/entrypoint.sh`

- 用 `sed` 替换模板中的 `${PAGE_PORT}`、`${ADMIN_PORT}`、`${API_PORT}`
- 生成 `nginx.conf` 和 `conf.d/*.conf`
- 原样复制用户自定义 `.conf` 文件
- `exec openresty -g 'daemon off;'` 启动

### 3. docker-compose.yml

- Nginx 服务：加 `entrypoint`、`environment`，调整 volume 挂载（模板挂到 `/etc/nginx/templates/`）
- Nginx 端口映射：`"${PAGE_PORT:-8009}:${PAGE_PORT:-8009}"`（两侧同步）
- Nginx healthcheck：`http://localhost:${PAGE_PORT}/nginx-health`
- API pprof 端口：`"${PPROF_PORT:-6060}:6060"`
- **去掉所有 `container_name`**（6 个服务）

### 4. .env / .env.example

- 新增 `PPROF_PORT=6060`

### 5. .gitattributes

- 加 `*.sh text eol=lf`（防 Windows CRLF 问题）

## 多实例使用方式

```bash
# 实例 1
cp -r seo_html_generator instance_a
cd instance_a
# 编辑 .env: PAGE_PORT=8009, ADMIN_PORT=8008
docker compose up -d

# 实例 2
cp -r seo_html_generator instance_b
cd instance_b
# 编辑 .env: PAGE_PORT=9009, ADMIN_PORT=9008
docker compose up -d
```

Docker Compose 以目录名作为项目前缀，容器、网络、volume 自动隔离。

## 限制

- 改端口需要 `docker compose up -d` 重启（Nginx 本质限制）
- 同机多实例需放不同目录（项目名隔离）
