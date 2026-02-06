# Nginx 端口 .env 统一配置 + 多实例支持

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现所有服务端口通过 `.env` 单文件配置，支持同一台机器上运行多套独立环境。

**Architecture:** 将 Nginx 配置文件改为 `.template` 模板，用 `${VAR}` 占位符替代硬编码端口。新建 `entrypoint.sh`，容器启动时用 `sed` 从环境变量生成实际配置。去掉所有 `container_name` 实现多实例隔离。pprof 端口也纳入 `.env`。

**Tech Stack:** Docker Compose, OpenResty/Nginx, Lua, sed

---

### Task 1: 创建 entrypoint.sh 和 .gitattributes

**Files:**
- Create: `docker/nginx/entrypoint.sh`
- Create: `.gitattributes`

**Step 1: 创建 entrypoint.sh**

```bash
#!/bin/sh
set -e

PAGE_PORT=${PAGE_PORT:-8009}
ADMIN_PORT=${ADMIN_PORT:-8008}
API_PORT=${API_PORT:-8080}

# 从模板生成 nginx.conf
sed "s/\${PAGE_PORT}/$PAGE_PORT/g; s/\${ADMIN_PORT}/$ADMIN_PORT/g; s/\${API_PORT}/$API_PORT/g" \
  /etc/nginx/templates/nginx.conf.template > /usr/local/openresty/nginx/conf/nginx.conf

# 从模板生成 conf.d/*.conf
for f in /etc/nginx/templates/conf.d/*.template; do
  [ -f "$f" ] || continue
  filename=$(basename "$f" .template)
  sed "s/\${PAGE_PORT}/$PAGE_PORT/g; s/\${ADMIN_PORT}/$ADMIN_PORT/g; s/\${API_PORT}/$API_PORT/g" \
    "$f" > /etc/nginx/conf.d/"$filename"
done

# 复制用户自定义 .conf 文件（非模板，原样复制）
for f in /etc/nginx/templates/conf.d/*.conf; do
  [ -f "$f" ] || continue
  cp "$f" /etc/nginx/conf.d/
done

exec openresty -g 'daemon off;'
```

**Step 2: 创建 .gitattributes**

```
*.sh text eol=lf
```

**Step 3: 提交**

```bash
git add docker/nginx/entrypoint.sh .gitattributes
git commit -m "feat: add nginx entrypoint.sh for template-based port config"
```

---

### Task 2: 转换 Nginx 配置为模板

**Files:**
- Rename+Modify: `docker/nginx/nginx.conf` → `docker/nginx/nginx.conf.template`
- Rename+Modify: `docker/nginx/conf.d/default.conf` → `docker/nginx/conf.d/default.conf.template`
- Rename+Modify: `docker/nginx/conf.d/admin.conf` → `docker/nginx/conf.d/admin.conf.template`

**Step 4: 转换 nginx.conf → nginx.conf.template**

重命名文件，然后做以下修改：

1. 在文件顶部（`user nobody;` 之前）加一行：
```nginx
env API_PORT;
```

2. 第 93 行 `server api:8080;` 替换为：
```nginx
        server api:${API_PORT};
```

**Step 5: 转换 default.conf → default.conf.template**

重命名文件，然后做以下替换（全部替换）：

- `listen 8009` → `listen ${PAGE_PORT}`（2 处：第 6、7 行）
- `api:8080` → `api:${API_PORT}`（5 处：第 143、166、182、211、258 行）

**Step 6: 转换 admin.conf → admin.conf.template**

重命名文件，然后做以下替换：

- 第 2 行注释 `管理后台 - 独立端口 8008` → `管理后台 - 独立端口 ${ADMIN_PORT}`
- `listen 8008` → `listen ${ADMIN_PORT}`（1 处：第 6 行）
- `api:8080` → `api:${API_PORT}`（4 处：第 47、72、86、100 行）

**Step 7: 验证模板文件中不再有硬编码端口**

```bash
grep -rn "8080\|8008\|8009" docker/nginx/nginx.conf.template docker/nginx/conf.d/*.template
```

预期：无输出（所有端口已被 `${VAR}` 替换）。

注意：`docker/nginx/conf.d/*.example` 文件中可能包含端口引用，这些不需要修改。

**Step 8: 提交**

```bash
git add docker/nginx/nginx.conf.template docker/nginx/conf.d/default.conf.template docker/nginx/conf.d/admin.conf.template
git rm docker/nginx/nginx.conf docker/nginx/conf.d/default.conf docker/nginx/conf.d/admin.conf
git commit -m "feat: convert nginx configs to templates with port variables"
```

---

### Task 3: 修改 cache_handler.lua

**Files:**
- Modify: `docker/nginx/lua/cache_handler.lua:102`

**Step 9: 替换硬编码端口为环境变量读取**

将第 102 行：
```lua
        local ok, err = httpc:connect(server_ip, 8080)
```

替换为：
```lua
        local ok, err = httpc:connect(server_ip, tonumber(os.getenv("API_PORT") or "8080"))
```

**Step 10: 提交**

```bash
git add docker/nginx/lua/cache_handler.lua
git commit -m "feat: read API_PORT from env var in cache_handler.lua"
```

---

### Task 4: 更新 docker-compose.yml

**Files:**
- Modify: `docker-compose.yml`

**Step 11: 修改 docker-compose.yml**

需要修改的内容：

1. **去掉所有 container_name**（6 处）：
   - 删除 `container_name: seo-generator-web`（第 11 行）
   - 删除 `container_name: seo-generator-nginx`（第 22 行）
   - 删除 `container_name: seo-generator-api`（第 57 行）
   - 删除 `container_name: seo-generator-worker`（第 109 行）
   - 删除 `container_name: seo-generator-mysql`（第 147 行）
   - 删除 `container_name: seo-generator-redis`（第 183 行）

2. **Nginx 服务改造**：

   添加 entrypoint 和 environment：
   ```yaml
   nginx:
       image: openresty/openresty:1.25.3.1-alpine
       entrypoint: ["/bin/sh", "/etc/nginx/entrypoint.sh"]
       restart: unless-stopped
       environment:
         - PAGE_PORT=${PAGE_PORT:-8009}
         - ADMIN_PORT=${ADMIN_PORT:-8008}
         - API_PORT=${API_PORT:-8080}
   ```

   端口映射改为两侧同步：
   ```yaml
       ports:
         - "${PAGE_PORT:-8009}:${PAGE_PORT:-8009}"   # 页面服务
         - "${ADMIN_PORT:-8008}:${ADMIN_PORT:-8008}"  # 管理后台
   ```

   volumes 调整（模板挂到 templates 目录，entrypoint 单独挂载，lua 保持不变）：
   ```yaml
       volumes:
         - ./config.yaml:/app/config.yaml:ro
         - ./docker/nginx/entrypoint.sh:/etc/nginx/entrypoint.sh:ro
         - ./docker/nginx/nginx.conf.template:/etc/nginx/templates/nginx.conf.template:ro
         - ./docker/nginx/conf.d:/etc/nginx/templates/conf.d:ro
         - ./docker/nginx/lua:/etc/nginx/lua:ro
         - ./docker/nginx/ssl:/etc/nginx/ssl:ro
         - ./data/cache:/data/cache:ro
         - web_dist:/app/admin-panel/dist:ro
         - ./data/logs/nginx:/var/log/nginx
   ```

   healthcheck 使用环境变量：
   ```yaml
       healthcheck:
         test: ["CMD", "sh", "-c", "wget -q --spider http://localhost:${PAGE_PORT}/nginx-health"]
         interval: 30s
         timeout: 10s
         retries: 3
         start_period: 10s
   ```

3. **API 服务 pprof 端口可配**：

   ```yaml
       ports:
         - "${PPROF_PORT:-6060}:6060"  # pprof for CPU profiling (临时调试用)
   ```

**Step 12: 验证 docker-compose.yml 语法**

```bash
cd /项目根目录 && docker compose config --quiet
```

预期：无错误输出。

**Step 13: 提交**

```bash
git add docker-compose.yml
git commit -m "feat: make all ports configurable via .env, remove container_name for multi-instance support"
```

---

### Task 5: 更新 .env 和 .env.example

**Files:**
- Modify: `.env`
- Modify: `.env.example`

**Step 14: 更新 .env 和 .env.example**

在"服务端口配置"部分添加 pprof 端口，两个文件都加：

```env
# ============================================
# 服务端口配置
# ============================================
# Nginx 页面服务端口
PAGE_PORT=8009
# Nginx 管理后台端口
ADMIN_PORT=8008
# Go API 内部端口（一般不需要修改）
API_PORT=8080
# pprof 调试端口（一般不需要修改）
PPROF_PORT=6060
```

**Step 15: 提交**

```bash
git add .env .env.example
git commit -m "feat: add PPROF_PORT to .env config"
```

---

### Task 6: 更新 Go API pprof 端口读取

**Files:**
- Modify: `api/cmd/main.go:39-44`

**Step 16: pprof 从环境变量读取端口**

将第 39-44 行：
```go
	// Start pprof server for CPU profiling (port 6060)
	go func() {
		log.Info().Msg("Starting pprof server on :6060")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Error().Err(err).Msg("pprof server failed")
		}
	}()
```

替换为：
```go
	// Start pprof server for CPU profiling
	go func() {
		pprofPort := os.Getenv("PPROF_PORT")
		if pprofPort == "" {
			pprofPort = "6060"
		}
		log.Info().Str("port", pprofPort).Msg("Starting pprof server")
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			log.Error().Err(err).Msg("pprof server failed")
		}
	}()
```

**Step 17: 在 docker-compose.yml api 服务 environment 中添加 PPROF_PORT**

在 api 服务的 environment 列表末尾添加：
```yaml
      - PPROF_PORT=${PPROF_PORT:-6060}
```

**Step 18: 验证编译**

```bash
cd api && go build ./...
```

预期：编译成功。

**Step 19: 提交**

```bash
git add api/cmd/main.go docker-compose.yml
git commit -m "feat: make pprof port configurable via PPROF_PORT env var"
```

---

### Task 7: 部署验证

**Step 20: 重建所有服务**

```bash
cd /项目根目录 && docker compose up -d --build
```

**Step 21: 验证 Nginx 配置生成**

```bash
docker compose exec nginx cat /usr/local/openresty/nginx/conf/nginx.conf | grep "server api:"
docker compose exec nginx cat /etc/nginx/conf.d/default.conf | grep "listen"
docker compose exec nginx cat /etc/nginx/conf.d/admin.conf | grep "listen"
```

预期：
- `server api:8080;`
- `listen 8009 default_server;`
- `listen 8008;`

**Step 22: 验证服务健康**

```bash
docker compose ps
```

预期：所有服务 healthy/running。

**Step 23: 验证管理后台和页面服务可访问**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8008/
curl -s -o /dev/null -w "%{http_code}" http://localhost:8009/nginx-health
```

预期：200。

**Step 24: 验证无 container_name 硬编码**

```bash
docker compose ps --format "{{.Name}}"
```

预期：容器名带有项目前缀（如 `seo_html_generator-api-1`），而非 `seo-generator-api`。
