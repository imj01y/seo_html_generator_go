# SEO HTML Generator

SEO 站群 HTML 动态生成系统。

支持千万级关键词、图片、文章管理，自动识别搜索引擎蜘蛛并返回 SEO 优化页面，普通用户访问返回 404。内置爬虫框架，支持在线编辑和定时执行，自动将抓取内容处理为可用的标题和段落库。

## 目录

- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [项目结构](#项目结构)
- [架构设计](#架构设计)
- [数据库设计](#数据库设计)
- [本地开发](#本地开发)
- [配置说明](#配置说明)
- [API 文档](#api-文档)
- [WebSocket 实时通信](#websocket-实时通信)
- [模板开发指南](#模板开发指南)
- [爬虫开发指南](#爬虫开发指南)
- [数据导入指南](#数据导入指南)
- [内存池与缓存](#内存池与缓存)
- [生产部署建议](#生产部署建议)
- [常见问题](#常见问题)
- [License](#license)

## 功能特性

### 核心功能
- **动态 HTML 生成** - 基于 Go 模板引擎，每次请求生成不同内容
- **蜘蛛智能识别** - 支持百度、Google、Bing、搜狗、360、头条，可选 DNS 反向验证
- **三层缓存架构** - Nginx Lua 缓存 + Go 内存缓存 + 文件缓存，支持千万级并发
- **千万级数据支持** - 关键词、图片使用 INT UNSIGNED（千万级），标题、正文使用 BIGINT UNSIGNED（亿级）

### 数据管理
- **站群架构** - 站群 → 站点 → 模板的层级管理
- **关键词管理** - 分组管理，批量导入，千万级存储
- **图片管理** - 分组管理，URL 批量导入，千万级存储
- **文章管理** - 原始文章自动拆分为标题库和段落库

### 爬虫系统
- **在线代码编辑** - Monaco Editor，支持语法高亮和智能提示
- **文件树管理** - 支持多文件项目，创建/编辑/删除文件和目录
- **定时任务调度** - Cron 表达式，自动执行
- **实时日志** - WebSocket 推送执行日志
- **队列管理** - 任务队列、失败重试

### 数据处理
- **内容处理器** - 在线编辑处理代码
- **自动拆分** - 原始文章自动拆分为标题和正文
- **数据去重** - 智能去重和清洗

### 管理后台
- **Vue 3 现代化界面** - Element Plus 组件库
- **完整 CRUD** - 站点、关键词、图片、文章、爬虫项目、模板
- **实时监控** - WebSocket 推送日志和状态
- **数据统计仪表板** - ECharts 可视化
- **缓存管理** - 池状态监控、配置调整、缓存清理

## 技术栈

| 分类 | 技术 | 版本 | 说明 |
|------|------|------|------|
| **API 后端** | Go + Gin | 1.24 | 高性能 RESTful API |
| **Python Worker** | Python | 3.11 | 爬虫执行 + 内容处理 |
| **数据库** | MySQL | 8.4 | 千万级数据存储 |
| **缓存** | Redis | 8.0 | 队列 + 会话 + 数据归档 |
| **反向代理** | OpenResty | 1.25 | Nginx + Lua 缓存层 |
| **前端框架** | Vue | 3.4 | 现代化 SPA |
| **前端语言** | TypeScript | 5.x | 类型安全 |
| **UI 组件库** | Element Plus | 2.4 | 企业级 UI |
| **状态管理** | Pinia | 2.1 | Vue 3 状态管理 |
| **构建工具** | Vite | 5.0 | 快速构建 |
| **代码编辑器** | Monaco Editor | 0.45 | VSCode 级编辑体验 |
| **图表库** | ECharts | 5.4 | 数据可视化 |
| **容器化** | Docker + Compose | - | 一键部署 |

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
- 构建 Go API 镜像（约 30MB）
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
| content_worker | - | Python Worker（内部） |
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

### 常用命令

```bash
# 查看服务状态
docker-compose ps

# 查看所有日志
docker-compose logs -f

# 查看特定服务日志
docker-compose logs -f api
docker-compose logs -f content_worker
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
├── api/                           # Go API 后端
│   ├── cmd/
│   │   └── main.go                # 应用入口（初始化所有服务）
│   ├── internal/
│   │   ├── handler/               # HTTP 处理器（23个文件）
│   │   │   ├── router.go          # 路由配置总入口
│   │   │   ├── auth.go            # JWT 认证
│   │   │   ├── page.go            # 页面渲染（核心）
│   │   │   ├── sites.go           # 站点管理
│   │   │   ├── templates.go       # 模板管理
│   │   │   ├── keywords.go        # 关键词管理
│   │   │   ├── images.go          # 图片管理
│   │   │   ├── articles.go        # 文章管理
│   │   │   ├── spiders.go         # 爬虫项目管理
│   │   │   ├── spider_stats.go    # 爬虫统计
│   │   │   ├── processor.go       # 数据处理
│   │   │   ├── cache.go           # 缓存管理
│   │   │   ├── pool.go            # 数据池管理
│   │   │   ├── pool_config.go     # 池配置
│   │   │   ├── dashboard.go       # 仪表板
│   │   │   ├── settings.go        # 系统设置
│   │   │   ├── logs.go            # 日志管理
│   │   │   ├── websocket.go       # WebSocket 处理
│   │   │   ├── compile.go         # 模板编译
│   │   │   ├── content_worker_files.go  # 内容处理代码
│   │   │   └── middleware.go      # 中间件
│   │   ├── model/                 # 数据模型
│   │   │   └── models.go          # 所有模型定义
│   │   ├── repository/            # 数据访问层
│   │   │   └── db.go              # 数据库操作
│   │   └── service/               # 业务逻辑层（35个文件）
│   │       ├── pool_manager.go    # 数据池管理（关键词、图片、标题、正文）
│   │       ├── object_pool.go     # 对象池管理（Cls、URL、Emoji、Number）
│   │       ├── template_funcs.go  # 模板函数注册
│   │       ├── template_renderer.go      # Go Template 渲染
│   │       ├── fast_renderer.go          # 快速渲染优化
│   │       ├── fast_renderer.go   # 快速渲染优化
│   │       ├── template_cache.go  # 模板缓存
│   │       ├── template_analyzer.go      # 模板性能分析
│   │       ├── template_converter.go     # 模板语法转换
│   │       ├── template_validator.go     # 模板验证
│   │       ├── site_cache.go      # 站点配置缓存
│   │       ├── html_cache.go      # HTML 文件缓存
│   │       ├── spider_detector.go # 蜘蛛识别
│   │       ├── scheduler.go       # 任务调度器
│   │       ├── task_handlers.go   # 定时任务处理器
│   │       ├── stats_archiver.go  # 统计数据归档
│   │       ├── pool_reloader.go   # 池配置热重载
│   │       ├── monitor.go         # 性能监控
│   │       ├── alerting.go        # 告警系统
│   │       ├── emoji_manager.go   # Emoji 数据管理
│   │       ├── encoder.go         # HTML 编码（防爬虫）
│   │       ├── number_pool.go     # 随机数生成池
│   │       ├── memory_pool.go     # 内存对象池
│   │       ├── auth.go            # JWT 认证服务
│   │       ├── response.go        # 统一响应格式
│   │       ├── errors.go          # 错误定义
│   │       ├── logger.go          # 日志系统（Zerolog）
│   │       └── recovery.go        # 错误恢复
│   ├── pkg/
│   │   └── config/                # 配置管理
│   │       └── config.go          # 配置加载
│   ├── go.mod
│   └── go.sum
│
├── content_worker/                # Python Worker（爬虫 + 内容处理）
│   ├── main.py                    # Worker 入口
│   ├── config.py                  # 配置加载
│   ├── core/
│   │   ├── processor/             # 文本处理（拆分、去重、优化）
│   │   ├── workers/               # 后台任务
│   │   └── redis_client/          # Redis 连接
│   ├── database/
│   │   └── db.py                  # 数据库操作
│   └── requirements.txt
│
├── web/                           # Vue 3 前端
│   ├── src/
│   │   ├── main.ts                # 入口
│   │   ├── App.vue                # 根组件
│   │   ├── router/
│   │   │   └── index.ts           # 路由配置（20+ 路由）
│   │   ├── stores/
│   │   │   └── user.ts            # 用户状态（Pinia）
│   │   ├── api/                   # API 接口封装（18个文件）
│   │   │   ├── index.ts           # API 入口
│   │   │   ├── shared.ts          # 共享工具
│   │   │   ├── auth.ts            # 认证 API
│   │   │   ├── dashboard.ts       # 仪表板 API
│   │   │   ├── sites.ts           # 站点 API
│   │   │   ├── site-groups.ts     # 站群 API
│   │   │   ├── templates.ts       # 模板 API
│   │   │   ├── keywords.ts        # 关键词 API
│   │   │   ├── images.ts          # 图片 API
│   │   │   ├── articles.ts        # 文章 API
│   │   │   ├── spiderProjects.ts  # 爬虫项目 API
│   │   │   ├── spiders.ts         # 蜘蛛检测 API
│   │   │   ├── processor.ts       # 数据处理 API
│   │   │   ├── cache-pool.ts      # 缓存池 API
│   │   │   ├── pool-config.ts     # 池配置 API
│   │   │   ├── settings.ts        # 设置 API
│   │   │   ├── logs.ts            # 日志 API
│   │   │   └── contentWorker.ts   # 内容处理 API
│   │   ├── views/                 # 页面组件（22个文件）
│   │   │   ├── Dashboard.vue      # 仪表板
│   │   │   ├── Login.vue          # 登录
│   │   │   ├── sites/             # 站点管理
│   │   │   ├── templates/         # 模板管理
│   │   │   ├── keywords/          # 关键词管理
│   │   │   ├── images/            # 图片管理
│   │   │   ├── articles/          # 文章管理
│   │   │   ├── spiders/           # 爬虫管理
│   │   │   │   ├── ProjectList.vue    # 项目列表
│   │   │   │   ├── ProjectEdit.vue    # 项目编辑
│   │   │   │   ├── ProjectCode.vue    # 代码编辑
│   │   │   │   ├── SpiderStats.vue    # 统计
│   │   │   │   └── SpiderLogs.vue     # 日志
│   │   │   ├── processor/         # 数据处理
│   │   │   ├── cache/             # 缓存管理
│   │   │   └── settings/          # 系统设置
│   │   ├── components/            # 公共组件
│   │   │   ├── Layout/
│   │   │   │   └── MainLayout.vue # 主布局
│   │   │   ├── MonacoEditor.vue   # 代码编辑器
│   │   │   ├── LogViewer.vue      # 日志查看
│   │   │   ├── PoolStatusCard.vue # 池状态卡片
│   │   │   ├── ScheduleBuilder.vue    # Cron 构建器
│   │   │   ├── CodeEditorPanel/   # 代码编辑面板
│   │   │   ├── ApiTokenGuide.vue  # API 令牌指南
│   │   │   ├── SpiderGuide.vue    # 爬虫开发指南
│   │   │   └── TemplateGuide.vue  # 模板开发指南
│   │   ├── types/
│   │   │   ├── index.ts           # TypeScript 类型定义
│   │   │   └── generator.ts       # 生成器类型
│   │   └── utils/
│   │       └── request.ts         # HTTP 请求封装
│   ├── package.json
│   └── vite.config.ts
│
├── docker/                        # Docker 配置
│   ├── api.Dockerfile             # Go API 构建（多阶段，约30MB）
│   ├── content_worker.Dockerfile  # Python Worker 构建
│   ├── web.Dockerfile             # Vue 前端构建
│   ├── nginx/
│   │   ├── nginx.conf             # Nginx 主配置
│   │   ├── conf.d/
│   │   │   ├── default.conf       # 页面服务（8009）
│   │   │   └── admin.conf         # 管理后台（8008）
│   │   └── lua/                   # Lua 缓存脚本
│   └── mysql/
│       └── my.cnf                 # MySQL 优化配置
│
├── migrations/                    # 数据库初始化
│   └── 000_init.sql               # 完整数据库结构
│
├── data/                          # 运行时数据（git 忽略）
│   ├── cache/                     # HTML 缓存文件
│   ├── logs/                      # 应用日志
│   ├── emojis.json                # Emoji 数据库
│   └── nginx/                     # Nginx 日志
│
├── .env.example                   # 环境配置示例
├── .env                           # 环境配置（git 忽略）
├── config.yaml                    # 应用配置文件
├── docker-compose.yml             # Docker 编排
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
│      Go API 服务       │       │   Python Worker       │
│      (Gin 框架)        │       │   (content_worker)    │
│  ┌─────────────────┐  │       │  ┌─────────────────┐  │
│  │ 页面渲染        │  │       │  │ 爬虫执行        │  │
│  │ 站点管理        │  │       │  │ 内容处理        │  │
│  │ 关键词/图片管理  │  │       │  │ 定时任务        │  │
│  │ 文章管理        │  │       │  │ 文章拆分        │  │
│  │ 模板管理        │  │       │  └─────────────────┘  │
│  │ 爬虫项目管理    │  │       └───────────────────────┘
│  │ 认证/权限       │  │                   │
│  │ WebSocket       │  │                   │
│  └─────────────────┘  │                   │
└───────────┬───────────┘                   │
            │                               │
            └───────────────┬───────────────┘
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                          数据层                                  │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐  │
│  │     MySQL 8.4    │  │    Redis 8.0     │  │   文件系统     │  │
│  │  - 站点配置      │  │  - 任务队列      │  │  - HTML 缓存   │  │
│  │  - 关键词        │  │  - 会话存储      │  │  - 模板文件    │  │
│  │  - 图片/文章     │  │  - 数据归档      │  │  - 日志        │  │
│  │  - 爬虫项目      │  │  - 热重载通知    │  │  - Emoji 数据  │  │
│  │  - 标题/正文库   │  │                  │  │                │  │
│  └──────────────────┘  └──────────────────┘  └───────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 三层缓存架构

```
┌─────────────────────────────────────────────────┐
│         客户端请求（蜘蛛/用户）                  │
└────────────────┬────────────────────────────────┘
                 │
     ┌───────────▼──────────────┐
     │  第1层：Nginx Lua缓存    │◄─ 文件缓存检查 (data/cache)
     │  - 蜘蛛识别              │   缓存命中 → 返回200
     │  - 文件缓存检查          │   X-Cache: HIT
     │  - 缓存未命中 → 回源     │
     └───────────┬──────────────┘
                 │
     ┌───────────▼──────────────┐
     │  第2层：Go内存缓存        │◄─ HTMLCache、TemplateCache、SiteCache
     │  - HTML 内存缓存（LRU）   │   快速查询
     │  - 模板编译缓存          │
     │  - 站点配置缓存          │
     └───────────┬──────────────┘
                 │
     ┌───────────▼──────────────┐
     │  第3层：数据库查询       │◄─ MySQL
     │  - 获取最新配置          │   关键词、图片、标题、正文
     │  - 生成新页面            │
     │  - 写入缓存              │
     └──────────────────────────┘
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
                      │ 是（已识别蜘蛛）
                      ▼
              ┌────────────────┐
              │  加载站点配置   │
              │  获取模板      │
              └───────┬────────┘
                      │
                      ▼
              ┌────────────────┐
              │  渲染 HTML     │
              │  - 随机关键词   │◄─ 从内存池获取
              │  - 随机图片    │◄─ 从内存池获取
              │  - 随机标题    │◄─ 从数据库获取
              │  - 随机正文    │◄─ 从数据库获取
              │  - 生成内链    │
              │  - HTML 编码   │◄─ 防爬虫
              └───────┬────────┘
                      │
                      ▼
              ┌────────────────┐
              │  保存文件缓存   │
              │  返回响应      │
              │  X-Cache: MISS │
              └────────────────┘
```

## 数据库设计

### 表结构概览

| 表名 | 主键类型 | 预估容量 | 说明 |
|------|----------|----------|------|
| **站群架构** |
| site_groups | INT | 百级 | 站群分组 |
| sites | INT | 千级 | 站点配置 |
| **数据存储** |
| keyword_groups | INT | 百级 | 关键词分组 |
| keywords | INT UNSIGNED | 千万级 | 关键词库 |
| image_groups | INT | 百级 | 图片分组 |
| images | INT UNSIGNED | 千万级 | 图片 URL 库 |
| article_groups | INT | 百级 | 文章分组 |
| original_articles | INT | 万级 | 原始文章 |
| **生成内容** |
| templates | INT | 百级 | 页面模板 |
| titles | BIGINT UNSIGNED | 亿级 | 标题库 |
| contents | BIGINT UNSIGNED | 亿级 | 正文库 |
| **爬虫系统** |
| spider_projects | INT | 百级 | 爬虫项目 |
| spider_project_files | INT | 千级 | 项目文件（文件树） |
| scheduled_tasks | INT | 百级 | 定时任务 |
| task_logs | BIGINT | 百万级 | 任务执行日志 |
| **访问统计** |
| spider_logs | BIGINT | 亿级 | 蜘蛛访问日志 |
| spider_daily_stats | INT | 万级 | 每日统计 |
| spider_hourly_stats | INT | 十万级 | 每小时统计 |
| **系统配置** |
| admins | INT | 十级 | 管理员账户 |
| settings | INT | 十级 | 系统设置 |
| pool_config | INT | 1条 | 缓存池配置 |

### 核心表关系

```
site_groups (站群)
    │
    ├── sites (站点)
    │       └── template_id → templates
    │
    ├── keyword_groups (关键词分组)
    │       └── keywords (关键词表)
    │
    ├── image_groups (图片分组)
    │       └── images (图片表)
    │
    └── article_groups (文章分组)
            └── original_articles (原始文章)
                    │
                    ▼ 处理后
            ┌───────┴───────┐
            │               │
        titles          contents
       (标题库)          (正文库)
```

### 关键索引设计

```sql
-- 关键词表：支持快速随机查询
CREATE INDEX idx_group_status ON keywords(group_id, status);

-- 图片表：支持快速随机查询
CREATE INDEX idx_group_status ON images(group_id, status);

-- 标题/正文表：支持 batch_id 优先新数据
CREATE INDEX idx_template_batch ON titles(template_id, batch_id);
CREATE INDEX idx_template_batch ON contents(template_id, batch_id);

-- 蜘蛛日志：时间范围查询
CREATE INDEX idx_visit_time ON spider_logs(visited_at);
CREATE INDEX idx_domain_time ON spider_logs(domain, visited_at);
```

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
cd content_worker

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

```
.env                    # Docker 部署配置（密码、端口）
├── docker-compose.yml  # 引用 .env 变量
└── config.yaml         # 应用配置（SEO参数、蜘蛛识别等）
```

### .env 环境配置

```bash
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
    dns_verify_enabled: false    # DNS 反向验证（可选）
    supported_spiders:           # 支持的蜘蛛
      - baidu
      - google
      - bing
      - sogou
      - 360
      - toutiao
```

## API 文档

### 认证接口

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | /api/auth/login | 登录，返回 JWT Token |
| POST | /api/auth/logout | 登出 |
| GET | /api/auth/profile | 获取当前用户信息 |

### 站群管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/site-groups | 站群列表 |
| POST | /api/site-groups | 新建站群 |
| PUT | /api/site-groups/:id | 编辑站群 |
| DELETE | /api/site-groups/:id | 删除站群 |

### 站点管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/sites | 站点列表 |
| POST | /api/sites | 新建站点 |
| PUT | /api/sites/:id | 编辑站点 |
| DELETE | /api/sites/:id | 删除站点 |
| POST | /api/sites/batch-create | 批量创建站点 |

### 模板管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/templates | 模板列表 |
| GET | /api/templates/:id | 获取模板详情 |
| POST | /api/templates | 新建模板 |
| PUT | /api/templates/:id | 编辑模板 |
| DELETE | /api/templates/:id | 删除模板 |
| POST | /api/templates/:id/compile | 编译模板 |
| POST | /api/templates/:id/preview | 预览模板 |

### 关键词管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/keywords/groups | 分组列表 |
| POST | /api/keywords/groups | 新建分组 |
| PUT | /api/keywords/groups/:id | 编辑分组 |
| DELETE | /api/keywords/groups/:id | 删除分组 |
| GET | /api/keywords | 关键词列表（分页） |
| POST | /api/keywords/batch-import | 批量导入关键词 |
| DELETE | /api/keywords/batch-delete | 批量删除关键词 |

### 图片管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/images/groups | 分组列表 |
| POST | /api/images/groups | 新建分组 |
| PUT | /api/images/groups/:id | 编辑分组 |
| DELETE | /api/images/groups/:id | 删除分组 |
| GET | /api/images | 图片列表（分页） |
| POST | /api/images/batch-import | 批量导入图片 URL |
| DELETE | /api/images/batch-delete | 批量删除图片 |

### 文章管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/articles/groups | 分组列表 |
| POST | /api/articles/groups | 新建分组 |
| GET | /api/articles | 文章列表（分页） |
| POST | /api/articles | 新建文章 |
| PUT | /api/articles/:id | 编辑文章 |
| DELETE | /api/articles/:id | 删除文章 |
| POST | /api/articles/batch-import | 批量导入文章 |

### 爬虫项目

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/spider-projects | 项目列表 |
| POST | /api/spider-projects | 创建项目 |
| GET | /api/spider-projects/:id | 获取项目详情 |
| PUT | /api/spider-projects/:id | 编辑项目 |
| DELETE | /api/spider-projects/:id | 删除项目 |
| GET | /api/spider-projects/:id/files | 获取文件树 |
| POST | /api/spider-projects/:id/files | 创建文件/目录 |
| PUT | /api/spider-projects/:id/files/:fileId | 更新文件内容 |
| DELETE | /api/spider-projects/:id/files/:fileId | 删除文件/目录 |
| POST | /api/spider-projects/:id/execute | 执行爬虫 |
| POST | /api/spider-projects/:id/stop | 停止执行 |

### 数据处理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/processor/config | 获取处理器配置 |
| PUT | /api/processor/config | 更新处理器配置 |
| POST | /api/processor/execute | 执行数据处理 |
| GET | /api/processor/status | 获取处理状态 |

### 缓存池管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/cache-pool/config | 获取池配置 |
| PUT | /api/cache-pool/config | 更新池配置 |
| GET | /api/cache-pool/status | 获取池状态 |
| POST | /api/cache-pool/reload | 重载池数据 |

### 系统管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/settings | 获取系统设置 |
| PUT | /api/settings | 更新系统设置 |
| GET | /api/settings/cache-stats | 缓存统计 |
| POST | /api/settings/clear-cache | 清除缓存 |
| GET | /api/settings/api-token | 获取 API Token |
| POST | /api/settings/api-token | 生成新 Token |

### 仪表板

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/dashboard/stats | 总体统计 |
| GET | /api/dashboard/spider-stats | 蜘蛛统计 |
| GET | /api/dashboard/daily-stats | 每日统计图表 |
| GET | /api/dashboard/hourly-stats | 每小时统计图表 |

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

## WebSocket 实时通信

系统提供多个 WebSocket 端点用于实时数据推送：

| 端点 | 说明 |
|------|------|
| /ws/spider-logs/:projectId | 爬虫执行日志实时推送 |
| /ws/processor-logs | 数据处理日志实时推送 |
| /ws/pool-status | 缓存池状态实时推送 |
| /ws/system-stats | 系统性能指标实时推送 |

**使用示例（前端）：**

```typescript
import { buildWsUrl, closeWebSocket } from '@/api/shared'

// 建立 WebSocket 连接
const ws = new WebSocket(buildWsUrl(`/ws/spider-logs/${projectId}`))

ws.onmessage = (event) => {
  const data = JSON.parse(event.data)
  console.log('收到日志:', data)
}

ws.onclose = () => {
  console.log('连接关闭')
}

// 关闭连接
closeWebSocket(ws)
```

## 模板开发指南

### 模板引擎

系统使用 **Go Template** 标准模板引擎，支持完整的模板语法和自定义函数。

### 可用模板函数

| 函数名 | 说明 | 示例 |
|--------|------|------|
| `RandomKeyword` | 获取随机关键词 | `{{ RandomKeyword }}` |
| `RandomImage` | 获取随机图片 URL | `{{ RandomImage }}` |
| `RandomTitle` | 获取随机标题 | `{{ RandomTitle }}` |
| `RandomContent` | 获取随机段落 | `{{ RandomContent }}` |
| `GenerateLink` | 生成内链 URL | `{{ GenerateLink "关键词" }}` |
| `Cls` | HTML 编码（防爬虫） | `{{ Cls "文本" }}` |
| `RandomNumber` | 生成随机数 | `{{ RandomNumber 1 100 }}` |
| `RandomEmoji` | 获取随机 Emoji | `{{ RandomEmoji }}` |
| `KeywordWithEmoji` | 关键词+Emoji | `{{ KeywordWithEmoji }}` |
| `Iterate` | 循环 N 次 | `{{ range $i := Iterate 10 }}` |
| `Add` | 加法运算 | `{{ Add 1 2 }}` |
| `Sub` | 减法运算 | `{{ Sub 5 3 }}` |
| `Mul` | 乘法运算 | `{{ Mul 2 3 }}` |
| `Div` | 除法运算 | `{{ Div 6 2 }}` |
| `Mod` | 取模运算 | `{{ Mod 7 3 }}` |
| `Now` | 当前时间 | `{{ Now.Format "2006-01-02" }}` |

### 模板示例

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ .Title }} - {{ RandomKeyword }}</title>
    <meta name="keywords" content="{{ .Keywords }}">
    <meta name="description" content="{{ RandomTitle }}">
</head>
<body>
    <h1>{{ Cls .Title }}</h1>

    <!-- 随机关键词链接区域 -->
    <div class="keywords">
        {{ range $i := Iterate 20 }}
        <a href="{{ GenerateLink (RandomKeyword) }}">{{ KeywordWithEmoji }}</a>
        {{ end }}
    </div>

    <!-- 随机图片 -->
    <div class="images">
        {{ range $i := Iterate 5 }}
        <img src="{{ RandomImage }}" alt="{{ RandomKeyword }}" loading="lazy">
        {{ end }}
    </div>

    <!-- 正文内容 -->
    <article>
        {{ range $i := Iterate 3 }}
        <h2>{{ RandomTitle }}</h2>
        <p>{{ RandomContent }}</p>
        {{ end }}
    </article>

    <!-- 更多链接 -->
    <div class="more-links">
        {{ range $i := Iterate 50 }}
        <a href="{{ GenerateLink (RandomKeyword) }}">{{ RandomKeyword }}</a>
        {{ end }}
    </div>

    <footer>
        <p>更新时间: {{ Now.Format "2006-01-02 15:04:05" }}</p>
        <p>随机数: {{ RandomNumber 1000 9999 }}</p>
    </footer>
</body>
</html>
```

### 模板性能分析

系统提供模板性能分析功能，可以：
- 估算每次渲染的资源消耗
- 推荐合适的池大小配置
- 检测潜在的性能问题

在管理后台「模板管理」页面点击「性能分析」查看。

## 爬虫开发指南

### 爬虫代码结构

```python
from base_spider import BaseSpider, Request

class MySpider(BaseSpider):
    name = "example_spider"
    start_urls = ["https://example.com/articles"]

    def parse(self, response):
        """解析列表页"""
        # 提取文章链接
        links = response.css("a.article-link::attr(href)").getall()
        for link in links:
            yield Request(link, callback=self.parse_article)

        # 翻页
        next_page = response.css("a.next::attr(href)").get()
        if next_page:
            yield Request(next_page, callback=self.parse)

    def parse_article(self, response):
        """解析文章详情"""
        yield {
            "title": response.css("h1::text").get(),
            "content": response.css("div.content").get(),
            "author": response.css("span.author::text").get(),
            "publish_time": response.css("time::attr(datetime)").get(),
        }
```

### 文件树结构

爬虫项目支持多文件结构：

```
spider_project/
├── spider.py          # 主爬虫文件
├── utils/
│   ├── __init__.py
│   └── helper.py      # 工具函数
├── config.py          # 配置文件
└── requirements.txt   # 依赖
```

### 在管理后台使用

1. 进入「爬虫项目」页面
2. 点击「新建项目」
3. 填写项目名称和描述
4. 进入「代码编辑」页面
5. 在 Monaco Editor 中编写代码
6. 使用文件树管理多个文件
7. 点击「测试运行」验证
8. 设置定时任务或手动执行
9. 在「日志」页面查看实时输出

### Cron 表达式

支持标准 Cron 表达式：

```
分 时 日 月 周
*  *  *  *  *

# 示例
0 */2 * * *     # 每2小时执行
0 8 * * *       # 每天8点执行
0 0 * * 1       # 每周一0点执行
*/30 * * * *    # 每30分钟执行
```

## 数据导入指南

### 关键词导入

格式：文本文件，每行一个关键词，UTF-8 编码

```
SEO优化
网站建设
关键词排名
搜索引擎优化
百度SEO
```

支持去重，导入时自动跳过已存在的关键词。

### 图片导入

格式：文本文件，每行一个图片 URL

```
https://example.com/images/1.jpg
https://example.com/images/2.png
https://cdn.example.com/photo/abc.webp
```

### 文章导入

格式：文本文件，使用 `---` 分隔文章

```
这是第一篇文章的标题

这是第一篇文章的正文内容。可以包含多个段落。

这是第二段正文。
---
这是第二篇文章的标题

这是第二篇文章的正文。
---
第三篇文章标题

第三篇内容...
```

导入后可以通过「数据处理」功能自动拆分为标题库和正文库。

## 内存池与缓存

### 数据池架构

```
┌─────────────────────────────────────────────────┐
│                  PoolManager                     │
│  ┌──────────────────┬──────────────────────┐    │
│  │    数据池        │       对象池          │    │
│  │  ┌────────────┐  │  ┌────────────────┐  │    │
│  │  │ Keywords   │  │  │ Cls (编码器)    │  │    │
│  │  │ (50000)    │  │  │ (预生成)        │  │    │
│  │  ├────────────┤  │  ├────────────────┤  │    │
│  │  │ Images     │  │  │ URL (内链)      │  │    │
│  │  │ (50000)    │  │  │ (预生成)        │  │    │
│  │  ├────────────┤  │  ├────────────────┤  │    │
│  │  │ Titles     │  │  │ KeywordEmoji   │  │    │
│  │  │ (5000)     │  │  │ (预生成)        │  │    │
│  │  ├────────────┤  │  ├────────────────┤  │    │
│  │  │ Contents   │  │  │ Number (随机数) │  │    │
│  │  │ (5000)     │  │  │ (预生成)        │  │    │
│  │  └────────────┘  │  └────────────────┘  │    │
│  └──────────────────┴──────────────────────┘    │
└─────────────────────────────────────────────────┘
```

### 池配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| keywords_size | 50000 | 关键词池大小 |
| images_size | 50000 | 图片池大小 |
| titles_size | 5000 | 标题池大小 |
| contents_size | 5000 | 正文池大小 |
| threshold | 1000 | 补充阈值（低于此值触发补充） |
| refill_interval_ms | 1000 | 补充间隔（毫秒） |
| refresh_interval_ms | 300000 | 刷新间隔（毫秒） |

### 热重载

池配置支持热重载，无需重启服务：

1. 在管理后台修改池配置
2. 系统通过 Redis 发布配置变更通知
3. 所有 API 实例接收通知并重载配置
4. 新配置立即生效

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

3. **HTTPS 配置**
   - 建议在 Nginx 前面再加一层负载均衡器处理 SSL

### 资源建议

| 规模 | CPU | 内存 | 磁盘 | 说明 |
|------|-----|------|------|------|
| 小型 | 2核 | 4GB | 50GB | <100万关键词 |
| 中型 | 4核 | 8GB | 200GB | 100-1000万关键词 |
| 大型 | 8核+ | 16GB+ | 500GB+ | >1000万关键词 |

### 数据备份

```bash
# MySQL 备份
docker-compose exec mysql mysqldump -u root -p seo_generator > backup.sql

# Redis 备份
docker-compose exec redis redis-cli -a redis_6yh7uJ BGSAVE

# 恢复数据
docker-compose exec -T mysql mysql -u root -p seo_generator < backup.sql
```

### 监控告警

系统内置监控功能：
- 性能指标采集（10秒采样）
- 内存/CPU 使用率监控
- 池状态监控
- 请求统计

可在「仪表板」页面查看实时数据。

## 常见问题

### 启动问题

**Q: 首次启动很慢？**

A: 首次启动需要构建所有镜像和初始化数据库，查看进度：`docker-compose logs -f`

**Q: 数据库连接失败？**

A: 确保 MySQL 服务健康：`docker-compose ps`，状态应为 healthy

**Q: API 服务无法连接 Redis？**

A: 检查 Redis 密码配置是否正确，确认 Redis 容器正常运行

### 使用问题

**Q: 前端页面 404？**

A: 确保 web 服务已启动：`docker-compose logs web`

**Q: 页面不更新？**

A: 清除缓存：管理后台 → 缓存管理 → 清空缓存

**Q: 蜘蛛访问返回 404？**

A: 检查 User-Agent 是否正确，确认站点域名已配置

**Q: 关键词/图片导入失败？**

A: 检查文件编码（必须 UTF-8），每行一条数据

**Q: 爬虫执行没有日志？**

A: 检查 WebSocket 连接状态，确认 content_worker 服务正常运行

### 性能问题

**Q: 页面渲染慢？**

A:
1. 检查数据池配置，增大池大小
2. 检查数据库查询性能
3. 确认模板缓存已启用

**Q: 内存使用过高？**

A:
1. 减小数据池大小
2. 调整 HTML 缓存大小限制
3. 检查是否有内存泄漏

## License

MIT
