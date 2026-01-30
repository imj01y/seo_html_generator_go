# 项目目录重构设计

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 重构项目目录结构，使其清晰、统一配置、一键部署

**Architecture:** 单仓库多服务架构，三个服务（api/worker/web）代码分离，配置集中在根目录，Docker 相关文件集中在 docker/ 目录

**Tech Stack:** Go, Python, Vue 3, Docker Compose, Nginx, MySQL, Redis

---

## 目标结构

```
seo-generator/
├── api/                    # Go 后端
│   ├── cmd/
│   │   └── main.go
│   ├── internal/
│   │   ├── handler/
│   │   ├── service/
│   │   ├── model/
│   │   └── middleware/
│   ├── pkg/
│   │   ├── cache/
│   │   ├── render/
│   │   └── detector/
│   ├── go.mod
│   └── go.sum
├── worker/                 # Python Worker
│   ├── core/
│   │   ├── crawler/
│   │   ├── generators/
│   │   └── processors/
│   ├── database/
│   ├── main.py
│   └── requirements.txt
├── web/                    # Vue 前端
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── views/
│   │   ├── router/
│   │   ├── stores/
│   │   └── main.ts
│   ├── public/
│   ├── package.json
│   └── vite.config.ts
├── docker/                 # Docker 构建配置
│   ├── api.Dockerfile
│   ├── worker.Dockerfile
│   ├── web.Dockerfile
│   ├── nginx/
│   │   ├── nginx.conf
│   │   └── conf.d/
│   │       └── default.conf
│   └── mysql/
│       └── my.cnf
├── migrations/             # 数据库迁移脚本
├── scripts/                # 工具脚本
├── docs/                   # 文档
├── docker-compose.yml      # 一键部署入口
├── config.example.yaml     # 配置示例（入库）
├── .gitignore
├── README.md
└── LICENSE
```

## 运行时目录（不入库）

```
├── config.yaml             # 实际配置
├── data/
│   ├── logs/               # 日志
│   └── cache/              # HTML 缓存
```

---

## 文件映射关系

### 当前 → 目标

| 当前路径 | 目标路径 |
|---------|---------|
| `services/go-server/` | `api/` |
| `services/go-server/main.go` | `api/cmd/main.go` |
| `services/go-server/api/` | `api/internal/handler/` |
| `services/go-server/core/` | `api/internal/service/` + `api/pkg/` |
| `services/go-server/models/` | `api/internal/model/` |
| `services/go-server/handlers/` | `api/internal/handler/` (合并) |
| `services/go-server/database/` | `api/internal/repository/` |
| `services/go-server/config/` | `api/pkg/config/` |
| `services/go-server/migrations/` | `migrations/` |
| `services/python-worker/` | `worker/` |
| `services/admin-panel/` | `web/` |
| `deploy/docker-compose.yml` | `docker-compose.yml` |
| `deploy/docker/nginx/` | `docker/nginx/` |
| `deploy/docker/mysql/` | `docker/mysql/` |
| `deploy/Dockerfile` | `docker/api.Dockerfile` |
| `services/python-worker/Dockerfile` | `docker/worker.Dockerfile` |
| `shared/config/config.yaml` | `config.yaml` |
| `shared/database/schema.sql` | `migrations/000_init.sql` |
| `html_cache/` | `data/cache/` (运行时) |
| `logs/` | `data/logs/` (运行时) |
| `static/` | `web/public/static/` 或 `api/static/` |

### 需要合并 spiders.yaml 到 config.yaml

将 `services/go-server/config/spiders.yaml` 内容合并到主配置文件。

---

## 需要删除的目录/文件

- `services/` (移动后删除)
- `shared/` (移动后删除)
- `deploy/` (移动后删除)
- `go-page-server/` (空目录)
- `services/go-server/docker-compose.yml` (重复)
- `services/go-server/docker-compose.go-server.yml` (重复)

---

## 实施任务

### Task 1: 创建目标目录结构

**Files:**
- Create: `api/cmd/`
- Create: `api/internal/handler/`
- Create: `api/internal/service/`
- Create: `api/internal/model/`
- Create: `api/internal/middleware/`
- Create: `api/internal/repository/`
- Create: `api/pkg/`
- Create: `docker/`
- Create: `migrations/`
- Create: `scripts/`

**Step 1:** 创建所有目标目录
```bash
mkdir -p api/cmd api/internal/{handler,service,model,middleware,repository} api/pkg
mkdir -p docker/nginx/conf.d docker/mysql
mkdir -p migrations scripts
```

**Step 2:** 验证目录创建成功

---

### Task 2: 迁移 Go 后端代码

**Step 1:** 移动 main.go 到 api/cmd/
```bash
cp services/go-server/main.go api/cmd/main.go
```

**Step 2:** 移动 handler 代码
```bash
cp -r services/go-server/api/* api/internal/handler/
cp -r services/go-server/handlers/* api/internal/handler/
```

**Step 3:** 移动 core 代码到 service 和 pkg
- 业务逻辑 → `api/internal/service/`
- 工具类 → `api/pkg/`

**Step 4:** 移动 models
```bash
cp -r services/go-server/models/* api/internal/model/
```

**Step 5:** 移动 database
```bash
cp -r services/go-server/database/* api/internal/repository/
```

**Step 6:** 复制 go.mod/go.sum 并更新 module 路径
```bash
cp services/go-server/go.mod api/
cp services/go-server/go.sum api/
```

**Step 7:** 更新所有 Go import 路径

---

### Task 3: 迁移 Python Worker 代码

**Step 1:** 移动 worker 代码
```bash
cp -r services/python-worker/* worker/
```

**Step 2:** 删除 worker 中的 Dockerfile（将集中到 docker/）

---

### Task 4: 迁移 Vue 前端代码

**Step 1:** 移动前端代码
```bash
cp -r services/admin-panel/* web/
```

---

### Task 5: 迁移 Docker 配置

**Step 1:** 移动 Dockerfile 文件
```bash
cp deploy/Dockerfile docker/api.Dockerfile
cp services/python-worker/Dockerfile docker/worker.Dockerfile
# 创建 web.Dockerfile（如需要）
```

**Step 2:** 移动 nginx 配置
```bash
cp -r deploy/docker/nginx/* docker/nginx/
```

**Step 3:** 移动 mysql 配置
```bash
cp -r deploy/docker/mysql/* docker/mysql/
```

**Step 4:** 更新 docker-compose.yml 到根目录并修改路径
```bash
cp deploy/docker-compose.yml ./docker-compose.yml
# 更新所有 build context 和 volume 路径
```

---

### Task 6: 迁移配置文件

**Step 1:** 移动主配置
```bash
cp shared/config/config.yaml ./config.yaml
cp shared/config/config.yaml ./config.example.yaml
```

**Step 2:** 合并 spiders.yaml 到 config.yaml

**Step 3:** 移动数据库 schema
```bash
cp shared/database/schema.sql migrations/000_init.sql
cp services/go-server/migrations/* migrations/
```

---

### Task 7: 更新 .gitignore

添加：
```gitignore
# 运行时配置
config.yaml

# 运行时数据
data/

# 旧目录（如果残留）
services/
shared/
deploy/
go-page-server/
```

---

### Task 8: 清理旧目录

**Step 1:** 删除已迁移的旧目录
```bash
rm -rf services/ shared/ deploy/ go-page-server/
```

**Step 2:** 删除其他不需要的文件

---

### Task 9: 更新 Go import 路径

**Step 1:** 更新 go.mod 中的 module 名称

**Step 2:** 批量替换所有 Go 文件中的 import 路径

**Step 3:** 运行 `go mod tidy` 验证

---

### Task 10: 更新 docker-compose.yml 路径

更新所有 build context 和 volume 映射：
- `context: ./api`
- `dockerfile: ../docker/api.Dockerfile`
- volumes 映射到 `./data/logs`, `./data/cache`

---

### Task 11: 测试构建

**Step 1:** 测试 Go 编译
```bash
cd api && go build ./cmd/main.go
```

**Step 2:** 测试 Docker 构建
```bash
docker-compose build
```

---

### Task 12: 提交变更

```bash
git add -A
git commit -m "refactor: 重构项目为清晰的单仓库多服务架构"
```

---

## 一键部署使用方式

```bash
# 1. 克隆项目
git clone <repo>
cd seo-generator

# 2. 复制配置
cp config.example.yaml config.yaml
# 编辑 config.yaml 配置数据库、Redis 等

# 3. 启动
docker-compose up -d

# 4. 访问
# 管理后台: http://localhost:8008
# 页面服务: http://localhost:8009
```
