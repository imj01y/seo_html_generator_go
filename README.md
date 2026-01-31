# SEO HTML Generator

SEO 站群 HTML 动态生成系统。

支持千万级关键词、图片、文章管理，自动识别搜索引擎蜘蛛并返回 SEO 优化页面，普通用户访问返回 404。内置爬虫框架，支持在线编辑和定时执行，自动将抓取内容处理为可用的标题和段落库。

## 目录

- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [项目结构](#项目结构)
- [架构设计](#架构设计)
- [本地开发](#本地开发)
- [配置说明](#配置说明)
- [API 概览](#api-概览)
- [模板开发指南](#模板开发指南)
- [爬虫开发指南](#爬虫开发指南)
- [数据导入指南](#数据导入指南)
- [生产部署建议](#生产部署建议)
- [常见问题](#常见问题)
- [License](#license)

## 功能特性

### 核心功能
- **动态 HTML 生成** - 基于 Go 模板引擎，每次请求生成不同内容
- **蜘蛛智能识别** - 支持百度、Google、Bing、搜狗、360、头条，可选 DNS 反向验证
- **三层缓存架构** - Nginx Lua 缓存 + 文件缓存 + 数据库，支持千万级并发
- **千万级数据支持** - 关键词、图片、文章均支持千万级存储和快速随机读取

### 爬虫系统
- **在线代码编辑** - Monaco Editor，支持语法高亮
- **定时任务调度** - Cron 表达式，自动执行
- **自动内容处理** - 原始文章自动拆分为标题库和段落库

### 管理后台
- **Vue 3 现代化界面** - Element Plus 组件库
- **完整 CRUD** - 站点、关键词、图片、文章、爬虫项目
- **实时日志查看** - WebSocket 推送
- **数据统计仪表板** - ECharts 可视化

## 技术栈

| 分类 | 技术 | 版本 |
|------|------|------|
| **API 后端** | Go + Gin | 1.24 |
| **Python Worker** | Python | 3.11 |
| **数据库** | MySQL | 8.4 |
| **缓存** | Redis | 8.0 |
| **反向代理** | OpenResty (Nginx + Lua) | 1.25 |
| **前端框架** | Vue | 3.4 |
| **前端语言** | TypeScript | - |
| **UI 组件库** | Element Plus | 2.4 |
| **状态管理** | Pinia | 2.1 |
| **构建工具** | Vite | 5.0 |
| **代码编辑器** | Monaco Editor | 0.45 |
| **图表库** | ECharts | 5.4 |
| **容器化** | Docker + Compose | - |

## 快速开始

### Docker 一键部署

```bash
# 克隆项目
git clone https://github.com/your-repo/seo_html_generator.git
cd seo_html_generator

# 创建环境配置文件（可选，使用默认配置可跳过）
cp .env.example .env

# 一键启动（首次会自动构建镜像）
docker-compose up -d
```

首次启动需要等待：
- 构建 Go API 镜像
- 构建 Vue 前端镜像
- 构建 Python Worker 镜像
- 初始化 MySQL 数据库
- 等待健康检查通过

查看启动进度：`docker-compose logs -f`

### 服务说明

| 服务 | 端口 | 说明 |
|------|------|------|
| nginx | 8009 | 页面服务（蜘蛛访问） |
| nginx | 8008 | 管理后台 |
| api | 8080 | Go API（内部） |
| mysql | 3306 | MySQL 数据库 |
| redis | 6379 | Redis 缓存 |

### 访问地址

| 功能 | 地址 |
|------|------|
| Admin 管理后台 | http://localhost:8008 |
| API 健康检查 | http://localhost:8009/api/health |
| 页面服务 | http://localhost:8009/page?ua=Baiduspider&path=/test.html&domain=example.com |

### 默认账号密码

| 服务 | 用户名 | 密码 |
|------|--------|------|
| Admin 后台 | admin | admin_6yh7uJ |
| MySQL | root | mysql_6yh7uJ |
| Redis | - | redis_6yh7uJ |

### 自定义配置

所有配置统一在 `.env` 文件中管理：

```bash
# 创建配置文件
cp .env.example .env

# 编辑配置
vim .env
```

**.env 配置项说明：**

```bash
# 数据库配置
MYSQL_ROOT_PASSWORD=mysql_6yh7uJ  # MySQL root 密码
DB_PASSWORD=mysql_6yh7uJ          # 应用连接密码（与上面保持一致）
DB_NAME=seo_generator             # 数据库名

# Redis 配置
REDIS_PASSWORD=redis_6yh7uJ       # Redis 密码

# 端口配置
PAGE_PORT=8009                    # 页面服务端口
ADMIN_PORT=8008                   # 管理后台端口
```

**修改配置后重启：**

```bash
docker-compose down && docker-compose up -d
```

### 常用命令

```bash
# 查看服务状态
docker-compose ps

# 查看所有日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f api
docker-compose logs -f worker
docker-compose logs -f nginx

# 重启服务
docker-compose restart

# 停止服务
docker-compose down

# 停止服务并删除数据卷（清除所有数据）
docker-compose down -v

# 重新构建并启动
docker-compose up -d --build

# 强制重建所有容器
docker-compose up -d --build --force-recreate
```

## 项目结构

```
seo_html_generator/
├── api/                        # Go API 后端
│   ├── cmd/
│   │   └── main.go             # 应用入口
│   ├── internal/
│   │   ├── handler/            # HTTP 处理器
│   │   │   ├── articles.go     # 文章管理
│   │   │   ├── auth.go         # 认证
│   │   │   ├── cache.go        # 缓存管理
│   │   │   ├── dashboard.go    # 仪表板
│   │   │   ├── images.go       # 图片管理
│   │   │   ├── keywords.go     # 关键词管理
│   │   │   ├── page.go         # 页面渲染
│   │   │   ├── settings.go     # 系统设置
│   │   │   ├── sites.go        # 站点管理
│   │   │   ├── spiders.go      # 爬虫管理
│   │   │   └── templates.go    # 模板管理
│   │   ├── middleware/         # 中间件
│   │   ├── model/              # 数据模型
│   │   ├── repository/         # 数据访问层
│   │   └── service/            # 业务逻辑层
│   ├── pkg/
│   │   ├── config/             # 配置管理
│   │   ├── database/           # 数据库连接
│   │   └── redis/              # Redis 连接
│   ├── templates/              # HTML 模板
│   ├── go.mod
│   └── go.sum
│
├── worker/                     # Python Worker（爬虫 + 内容生成）
│   ├── main.py                 # Worker 入口
│   ├── config.py               # 配置加载
│   ├── core/
│   │   ├── crawler/            # 爬虫框架
│   │   ├── processors/         # 文本处理
│   │   └── workers/            # 后台任务
│   ├── database/               # 数据库操作
│   └── requirements.txt
│
├── web/                        # Vue 3 前端
│   ├── src/
│   │   ├── main.ts             # 入口
│   │   ├── App.vue             # 根组件
│   │   ├── router/             # 路由配置
│   │   ├── stores/             # Pinia 状态管理
│   │   ├── api/                # API 接口封装
│   │   ├── views/              # 页面组件
│   │   └── components/         # 公共组件
│   ├── package.json
│   └── vite.config.ts
│
├── docker/                     # Docker 配置
│   ├── api.Dockerfile          # Go API 构建
│   ├── worker.Dockerfile       # Python Worker 构建
│   ├── web.Dockerfile          # Vue 前端构建
│   ├── nginx/
│   │   ├── nginx.conf          # Nginx 主配置
│   │   ├── conf.d/
│   │   │   ├── default.conf    # 页面服务配置
│   │   │   └── admin.conf      # 管理后台配置
│   │   └── lua/                # Lua 缓存脚本
│   └── mysql/
│       └── my.cnf              # MySQL 配置
│
├── migrations/                 # 数据库迁移
│   └── 000_init.sql            # 初始化脚本
│
├── data/                       # 运行时数据（git 忽略）
│   ├── cache/                  # HTML 缓存
│   └── logs/                   # 日志文件
│
├── .env.example                # 环境配置示例
├── .env                        # 环境配置（git 忽略）
├── config.yaml                 # 应用配置文件
├── docker-compose.yml          # Docker 编排
└── README.md
```

## 架构设计

### 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         客户端请求                               │
│                    （蜘蛛 / 普通用户）                            │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                   OpenResty (Nginx + Lua)                       │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  8009: 页面服务                  8008: 管理后台          │    │
│  │  - Lua 缓存层（优先读取文件缓存）  - Vue SPA 静态文件      │    │
│  │  - 蜘蛛识别                      - API 反向代理          │    │
│  │  - 未命中回源 Go API                                     │    │
│  └─────────────────────────────────────────────────────────┘    │
└───────────────────────────┬─────────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            ▼                               ▼
┌───────────────────────┐       ┌───────────────────────┐
│      Go API 服务       │       │    Python Worker      │
│      (Gin 框架)        │       │    (异步任务)          │
│  ┌─────────────────┐  │       │  ┌─────────────────┐  │
│  │ 页面渲染        │  │       │  │ 爬虫执行        │  │
│  │ 站点管理        │  │       │  │ 内容处理        │  │
│  │ 关键词/图片管理  │  │       │  │ 定时任务        │  │
│  │ 文章管理        │  │       │  │ 统计任务        │  │
│  │ 认证/权限       │  │       │  └─────────────────┘  │
│  └─────────────────┘  │       └───────────────────────┘
└───────────┬───────────┘                   │
            │                               │
            └───────────────┬───────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                          数据层                                  │
│  ┌──────────────────┐  ┌──────────────┐  ┌─────────────────┐   │
│  │     MySQL 8.4    │  │  Redis 8.0   │  │   文件系统       │   │
│  │  - 站点配置      │  │  - 缓存      │  │  - HTML 缓存     │   │
│  │  - 关键词        │  │  - 队列      │  │  - 模板文件      │   │
│  │  - 图片/文章     │  │  - 会话      │  │  - 日志          │   │
│  │  - 爬虫项目      │  │              │  │                  │   │
│  └──────────────────┘  └──────────────┘  └─────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### 请求处理流程

```
                    请求到达
                       │
                       ▼
              ┌────────────────┐
              │  Nginx 接收    │
              │  (8009端口)    │
              └───────┬────────┘
                      │
                      ▼
              ┌────────────────┐     是     ┌────────────────┐
              │  Lua 层检查    │──────────▶│  返回缓存文件   │
              │  文件缓存存在？ │            │  X-Cache: HIT  │
              └───────┬────────┘            └────────────────┘
                      │ 否
                      ▼
              ┌────────────────┐
              │  回源 Go API   │
              │  /page 端点    │
              └───────┬────────┘
                      │
                      ▼
              ┌────────────────┐     否     ┌────────────────┐
              │   蜘蛛识别     │──────────▶│   返回 404     │
              │  User-Agent    │            └────────────────┘
              └───────┬────────┘
                      │ 是
                      ▼
              ┌────────────────┐
              │  加载站点配置   │
              │  获取模板      │
              └───────┬────────┘
                      │
                      ▼
              ┌────────────────┐
              │  渲染 HTML     │
              │  - 随机关键词   │
              │  - 随机图片    │
              │  - 随机段落    │
              └───────┬────────┘
                      │
                      ▼
              ┌────────────────┐
              │  保存文件缓存   │
              │  返回响应      │
              │  X-Cache: MISS │
              └────────────────┘
```

### 数据库核心表

| 表名 | 说明 | 主键类型 | 预估容量 |
|------|------|----------|----------|
| sites | 站点配置 | INT | 千级 |
| keywords | 关键词 | BIGINT | 千万级 |
| images | 图片 URL | BIGINT | 千万级 |
| titles | 标题库 | BIGINT | 亿级 |
| contents | 段落库 | BIGINT | 亿级 |
| spider_projects | 爬虫项目 | INT | 百级 |
| admins | 管理员 | INT | 十级 |

## 本地开发

### 环境要求

- Go >= 1.24
- Python >= 3.11
- Node.js >= 20
- MySQL >= 8.0
- Redis >= 6.0

### Go API 开发

```bash
cd api

# 安装依赖
go mod download

# 启动服务（开发模式）
go run cmd/main.go

# 或使用 air 热重载
air
```

### Python Worker 开发

```bash
cd worker

# 创建虚拟环境
python -m venv .venv
source .venv/bin/activate  # Linux/Mac
.venv\Scripts\activate     # Windows

# 安装依赖
pip install -r requirements.txt

# 启动 Worker
python main.py
```

### Vue 前端开发

```bash
cd web

# 安装依赖
npm install

# 开发模式（热重载）
npm run dev

# 构建生产版本
npm run build
```

### 配置数据库

本地开发需要启动 MySQL 和 Redis：

```bash
# 只启动依赖服务
docker-compose up -d mysql redis

# 导入初始数据
mysql -u root -p seo_generator < migrations/000_init.sql
```

## 配置说明

### 配置架构

项目采用分层配置架构：

```
.env                    # Docker 部署配置（密码、端口）
├── docker-compose.yml  # 引用 .env 变量
└── config.yaml         # 应用配置（SEO参数、蜘蛛识别等）
    ├── default         # 默认配置
    ├── development     # 本地开发配置
    └── production      # 生产环境配置（引用环境变量）
```

### .env 环境配置

Docker 部署时，所有敏感配置统一在 `.env` 文件中管理：

```bash
# .env 文件示例

# ============================================
# 数据库配置
# ============================================
MYSQL_ROOT_PASSWORD=mysql_6yh7uJ    # MySQL root 密码
DB_HOST=mysql                        # 数据库主机
DB_PORT=3306                         # 数据库端口
DB_USER=root                         # 数据库用户
DB_PASSWORD=mysql_6yh7uJ             # 数据库密码
DB_NAME=seo_generator                # 数据库名

# ============================================
# Redis 配置
# ============================================
REDIS_HOST=redis                     # Redis 主机
REDIS_PORT=6379                      # Redis 端口
REDIS_PASSWORD=redis_6yh7uJ          # Redis 密码
REDIS_DB=0                           # Redis 数据库

# ============================================
# 服务端口配置
# ============================================
PAGE_PORT=8009                       # 页面服务端口
ADMIN_PORT=8008                      # 管理后台端口
API_PORT=8080                        # Go API 内部端口

# ============================================
# 环境标识
# ============================================
ENV_FOR_DYNACONF=production          # 环境标识
GIN_MODE=release                     # Gin 运行模式
TZ=Asia/Shanghai                     # 时区
```

### config.yaml 应用配置

应用级配置在 `config.yaml` 中管理：

```yaml
default:
  # 缓存配置
  cache:
    enabled: true
    dir: "/data/cache"           # 缓存目录
    ttl_hours: 24                # 缓存过期时间
    max_size_gb: 10.0            # 最大缓存大小

  # SEO 生成配置
  seo:
    internal_links_count: 3856   # 内链数量
    encoding_mix_ratio: 0.5      # HTML 编码比例

  # 蜘蛛识别配置
  spider_detector:
    enabled: true
    return_404_for_non_spider: true
    dns_verify_enabled: false

development:
  # 本地开发使用固定配置
  database:
    host: "localhost"
    password: "mysql_6yh7uJ"

production:
  # 生产环境从环境变量读取
  database:
    host: "@format {env[DB_HOST]}"
    password: "@format {env[DB_PASSWORD]}"
```

### 修改配置

**修改数据库/Redis 密码：**

```bash
# 1. 编辑 .env 文件
vim .env

# 2. 修改密码（确保 MYSQL_ROOT_PASSWORD 和 DB_PASSWORD 一致）
MYSQL_ROOT_PASSWORD=新密码
DB_PASSWORD=新密码
REDIS_PASSWORD=新密码

# 3. 重启服务
docker-compose down && docker-compose up -d
```

**修改服务端口：**

```bash
# 修改 .env 中的端口配置
PAGE_PORT=9009    # 页面服务改为 9009
ADMIN_PORT=9008   # 管理后台改为 9008

# 重启服务
docker-compose down && docker-compose up -d
```

**注意：** 首次部署后修改 MySQL 密码需要先删除数据卷：

```bash
docker-compose down -v  # 删除数据卷
docker-compose up -d    # 重新初始化
```

## API 概览

### 认证

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | /api/auth/login | 登录 |
| POST | /api/auth/logout | 登出 |
| GET | /api/auth/profile | 获取当前用户 |

### 站点管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/sites | 站点列表 |
| POST | /api/sites | 新建站点 |
| PUT | /api/sites/:id | 编辑站点 |
| DELETE | /api/sites/:id | 删除站点 |

### 关键词管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/keywords/groups | 分组列表 |
| POST | /api/keywords/groups | 新建分组 |
| POST | /api/keywords/batch-import | 批量导入 |

### 图片管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/images/groups | 分组列表 |
| POST | /api/images/batch-import | 批量导入 |

### 爬虫项目

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/spider-projects | 项目列表 |
| POST | /api/spider-projects | 创建项目 |
| POST | /api/spider-projects/:id/execute | 执行爬虫 |
| GET | /api/spider-projects/:id/logs/ws | 日志 WebSocket |

### 页面服务

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /page | 动态生成 SEO 页面 |

**参数：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ua | string | 是 | User-Agent |
| path | string | 是 | 请求路径 |
| domain | string | 是 | 站点域名 |

**示例：**
```
GET /page?ua=Baiduspider&path=/test.html&domain=example.com
```

## 模板开发指南

### 模板位置

模板文件存放在 `api/templates/` 目录下。

### 可用模板函数

| 函数名 | 说明 |
|--------|------|
| `RandomKeyword` | 获取随机关键词 |
| `RandomImage` | 获取随机图片 URL |
| `RandomTitle` | 获取随机标题 |
| `RandomContent` | 获取随机段落 |
| `GenerateLink` | 生成内链 |

### 模板示例

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }}</title>
    <meta name="keywords" content="{{ .Keywords }}">
</head>
<body>
    <h1>{{ .Title }}</h1>

    <!-- 随机关键词链接 -->
    {{ range $i := Iterate 10 }}
    <a href="{{ GenerateLink (RandomKeyword) }}">{{ RandomKeyword }}</a>
    {{ end }}

    <!-- 随机图片 -->
    <img src="{{ RandomImage }}" alt="{{ RandomKeyword }}">

    <!-- 正文内容 -->
    <div>{{ .Content | safeHTML }}</div>
</body>
</html>
```

## 爬虫开发指南

### 爬虫代码结构

```python
class MySpider(BaseSpider):
    name = "example_spider"
    start_urls = ["https://example.com/articles"]

    def parse(self, response):
        # 提取文章链接
        links = response.css("a.article-link::attr(href)").getall()
        for link in links:
            yield Request(link, callback=self.parse_article)

    def parse_article(self, response):
        yield {
            "title": response.css("h1::text").get(),
            "content": response.css("div.content").get(),
        }
```

### 在管理后台使用

1. 进入「爬虫项目」页面
2. 点击「新建项目」
3. 在 Monaco Editor 中编写代码
4. 点击「测试运行」验证
5. 设置定时任务或手动执行

## 数据导入指南

### 关键词导入

格式：文本文件，每行一个关键词，UTF-8 编码

```
SEO优化
网站建设
关键词排名
```

### 图片导入

格式：文本文件，每行一个图片 URL

```
https://example.com/images/1.jpg
https://example.com/images/2.png
```

### 文章导入

格式：文本文件，使用 `---` 分隔文章

```
这是第一篇文章的标题

这是第一篇文章的正文内容。
---
这是第二篇文章的标题

这是第二篇文章的正文。
```

## 生产部署建议

### 安全加固

1. **修改默认密码**
   - Admin 后台密码
   - MySQL 密码
   - Redis 密码
   - JWT 密钥

2. **防火墙配置**
   ```bash
   # 只开放必要端口
   ufw allow 8008/tcp  # 管理后台
   ufw allow 8009/tcp  # 页面服务
   # 不要对外开放 3306、6379
   ```

### 资源建议

| 规模 | CPU | 内存 | 磁盘 |
|------|-----|------|------|
| 小型（<100万关键词） | 2核 | 4GB | 50GB |
| 中型（100-1000万） | 4核 | 8GB | 200GB |
| 大型（>1000万） | 8核+ | 16GB+ | 500GB+ |

### 数据备份

```bash
# MySQL 备份
docker-compose exec mysql mysqldump -u root -p seo_generator > backup.sql

# Redis 备份
docker-compose exec redis redis-cli -a redis_6yh7uJ BGSAVE
```

## 常见问题

### 启动问题

**Q: 首次启动很慢？**

A: 首次启动需要构建所有镜像和初始化数据库，查看进度：`docker-compose logs -f`

**Q: 数据库连接失败？**

A: 确保 MySQL 服务健康：`docker-compose ps`，状态应为 healthy

### 使用问题

**Q: 前端页面 404？**

A: 确保 web 服务已启动：`docker-compose logs web`

**Q: 页面不更新？**

A: 清除缓存：管理后台 → 缓存管理 → 清空缓存

## License

MIT
