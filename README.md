# SEO HTML Generator

SEO 站群 HTML 动态生成系统。

支持千万级关键词、图片、文章管理，自动识别搜索引擎蜘蛛并返回 SEO 优化页面，普通用户访问返回 404。内置爬虫框架，支持在线编辑和定时执行，自动将抓取内容处理为可用的标题和段落库。

## 目录

- [功能特性](#功能特性)
- [技术栈](#技术栈)
- [快速开始](#快速开始)
- [本地开发](#本地开发)
- [项目结构](#项目结构)
- [架构设计](#架构设计)
- [API 概览](#api-概览)
- [配置说明](#配置说明)
- [模板开发指南](#模板开发指南)
- [爬虫开发指南](#爬虫开发指南)
- [数据导入指南](#数据导入指南)
- [生产部署建议](#生产部署建议)
- [监控与运维](#监控与运维)
- [数据备份与恢复](#数据备份与恢复)
- [常见问题](#常见问题)
- [贡献指南](#贡献指南)
- [License](#license)

## 演示截图

### 仪表板
![Dashboard](docs/screenshots/dashboard.png)

### 站点管理
![Sites](docs/screenshots/sites.png)

### 关键词管理
![Keywords](docs/screenshots/keywords.png)

### 爬虫项目（代码编辑器）
![Spider Editor](docs/screenshots/spider-editor.png)

> 注：截图需要在 `docs/screenshots/` 目录下添加对应图片

## 功能特性

### 核心功能
- **动态 HTML 生成** - 基于 Jinja2 模板，每次请求生成不同内容
- **蜘蛛智能识别** - 支持百度、Google、Bing、搜狗、360、头条，可选 DNS 反向验证
- **三层缓存架构** - 文件缓存 + 内存缓存池 + 数据库，支持千万级并发
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
| **后端框架** | FastAPI | 0.128 |
| **数据库** | MySQL | 8.0 |
| **数据库驱动** | aiomysql | 异步 |
| **缓存** | Redis | 7.0 |
| **模板引擎** | Jinja2 | - |
| **配置管理** | Dynaconf | 3.2 |
| **任务调度** | APScheduler | - |
| **日志** | Loguru | - |
| **前端框架** | Vue | 3.4 |
| **前端语言** | TypeScript | - |
| **UI 组件库** | Element Plus | 2.4 |
| **状态管理** | Pinia | 2.1 |
| **构建工具** | Vite | 5.0 |
| **代码编辑器** | Monaco Editor | 0.45 |
| **图表库** | ECharts | 5.4 |
| **容器化** | Docker + Compose | - |

## 快速开始

### Docker 一键部署（推荐）

```bash
# 克隆项目后，进入项目目录
cd seo_html_generator

# 一键启动（首次会自动构建镜像）
docker-compose up -d --build
```

### 服务说明

| 服务 | 端口 | 说明 |
|------|------|------|
| nginx | 8009 | 反向代理（页面服务 + API） |
| nginx | 8008 | 管理后台 |
| mysql | 3306 | MySQL 8.0 数据库 |
| redis | 6379 | Redis 缓存 |

### 访问地址

| 功能 | 地址 |
|------|------|
| Admin 管理后台 | http://localhost:8008 |
| API 健康检查 | http://localhost:8009/api/health |
| API 文档 | http://localhost:8009/docs |
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

# 查看应用日志
docker-compose logs -f app

# 查看所有日志
docker-compose logs -f

# 重启服务
docker-compose restart

# 停止服务
docker-compose down

# 停止服务并删除数据卷（清除所有数据）
docker-compose down -v

# 重新构建并启动
docker-compose up -d --build

# 强制重建所有容器（推荐）
docker-compose up -d --build  --force-recreate

# 重启docker中nginx服务
docker restart seo-generator-nginx

```

### 数据持久化

以下目录/数据会被持久化：

| 类型 | 位置 |
|------|------|
| MySQL 数据 | Docker Volume: `mysql_data` |
| Redis 数据 | Docker Volume: `redis_data` |
| 应用日志 | `./logs/` |
| 缓存文件 | `./cache/` |
| 数据文件 | `./data/` |
| 模板文件 | `./templates/` |

### 自定义配置

#### 环境变量

可在 `docker-compose.yml` 中修改以下环境变量：

```yaml
environment:
  # 数据库配置
  - DB_HOST=mysql
  - DB_PORT=3306
  - DB_USER=root
  - DB_PASSWORD=mysql_6yh7uJ
  - DB_NAME=seo_generator

  # Redis 配置
  - REDIS_ENABLED=true
  - REDIS_HOST=redis
  - REDIS_PORT=6379
  - REDIS_PASSWORD=redis_6yh7uJ
  - REDIS_DB=0
```

#### 修改 MySQL 密码

1. 修改 `docker-compose.yml` 中的 MySQL 环境变量
2. 同步修改 app 服务的 `DB_PASSWORD` 环境变量
3. 重新启动服务

#### 修改 Redis 密码

1. 修改 `docker-compose.yml` 中 redis 服务的 `--requirepass` 参数
2. 同步修改 app 服务的 `REDIS_PASSWORD` 环境变量
3. 重新启动服务

## 本地开发

### 环境要求

- Python >= 3.9
- Node.js >= 18
- MySQL >= 8.0
- Redis >= 6.0

### 后端启动

```bash
# 1. 创建虚拟环境
python -m venv .venv
source .venv/bin/activate  # Linux/Mac
.venv\Scripts\activate     # Windows

# 2. 安装依赖
pip install -r requirements.txt

# 3. 配置数据库和 Redis
# 编辑 config.yaml

# 4. 启动服务
python main.py

# 或开发模式（自动重载）
uvicorn main:app --reload
```

### 前端启动

```bash
cd admin-panel

# 安装依赖
npm install

# 开发模式
npm run dev

# 构建生产版本
npm run build
```

### 配置环境切换

项目使用 Dynaconf 管理配置，支持 development（默认）和 production 环境。

**方式一：环境变量**

```bash
# Linux/Mac
ENV_FOR_DYNACONF=production python main.py

# Windows CMD
set ENV_FOR_DYNACONF=production && python main.py

# Windows PowerShell
$env:ENV_FOR_DYNACONF="production"; python main.py
```

**方式二：.env 文件**

在项目根目录创建 .env：

```
ENV_FOR_DYNACONF=production
```

**环境配置差异**

| 配置项 | development | production |
|--------|-------------|------------|
| server.host | 127.0.0.1 | 0.0.0.0 |
| server.debug | true | false |
| server.workers | 1 | 4 |
| 数据库配置 | config.yaml 固定值 | 从环境变量读取 |

## 项目结构

```
seo_html_generator/
├── main.py                     # 应用入口，初始化所有组件
├── config.py                   # Dynaconf 配置加载
├── config.yaml                 # 配置文件（开发/生产环境）
├── requirements.txt            # Python 依赖
├── Dockerfile                  # Docker 构建文件
├── docker-compose.yml          # Docker 编排
│
├── api/                        # API 路由层
│   ├── routes.py               # 核心 API：认证、站点、页面服务
│   ├── spider_routes.py        # 爬虫项目 CRUD、执行、日志
│   ├── generator_routes.py     # 正文生成器管理
│   └── log_routes.py           # 系统日志查看
│
├── core/                       # 核心业务逻辑
│   ├── seo_core.py             # SEO 渲染核心
│   │                           # - Jinja2 模板环境
│   │                           # - 注册模板函数（随机关键词、图片、链接等）
│   │                           # - render_page() 主渲染方法
│   │
│   ├── spider_detector.py      # 蜘蛛识别
│   │                           # - User-Agent 正则匹配
│   │                           # - DNS 反向验证（可选）
│   │                           # - 支持 6 种主流搜索引擎
│   │
│   ├── html_cache_manager.py   # HTML 文件缓存
│   │                           # - gzip 压缩存储
│   │                           # - 按蜘蛛类型分目录
│   │                           # - 支持 Nginx 直接服务
│   │
│   ├── redis_client.py         # Redis 连接管理
│   ├── auth.py                 # JWT 认证
│   ├── logging.py              # Loguru 日志配置
│   │
│   ├── 关键词管理/
│   │   ├── keyword_group_manager.py   # 异步分组管理（MySQL）
│   │   └── keyword_cache_pool.py      # 内存缓存池（生产者-消费者）
│   │
│   ├── 图片管理/
│   │   ├── image_group_manager.py     # 异步分组管理
│   │   └── image_cache_pool.py        # 内存缓存池
│   │
│   ├── 文章管理/
│   │   ├── title_manager.py           # 标题库（分层随机）
│   │   ├── content_manager.py         # 正文管理
│   │   └── content_pool_manager.py    # 段落池（一次性消费模式）
│   │
│   ├── 生成组件/
│   │   ├── encoder.py                 # HTML 实体编码
│   │   ├── class_generator.py         # CSS 类名生成
│   │   ├── emoji.py                   # Emoji 管理
│   │   ├── link_generator.py          # 链接生成
│   │   └── title_generator.py         # 标题生成（带 Emoji）
│   │
│   ├── processors/             # 文本处理
│   │   ├── cleaner.py          # 文本清洗
│   │   └── pinyin_annotator.py # 拼音标注
│   │
│   ├── dedup/                  # 去重模块（BloomFilter）
│   │
│   ├── crawler/                # 爬虫框架
│   │   ├── spider.py           # 爬虫基类
│   │   ├── request_queue.py    # 请求队列
│   │   ├── project_runner.py   # 项目执行器
│   │   └── log_manager.py      # 爬虫日志
│   │
│   └── workers/                # 后台工作进程
│       ├── generator_worker.py # 正文生成 Worker
│       │                       # - 监听 pending:articles 队列
│       │                       # - 提取标题写入 titles 表
│       │                       # - 拆分正文写入 contents 表
│       ├── stats_worker.py     # 统计 Worker
│       └── spider_scheduler.py # 爬虫调度 Worker
│
├── database/                   # 数据层
│   ├── db.py                   # 异步连接池 + 查询接口
│   ├── schema.sql              # 数据库初始化脚本
│   └── content_writer.py       # 内容批量写入
│
├── templates/                  # HTML 模板（Jinja2）
│   └── download_site/          # 示例模板
│
├── admin-panel/                # Vue 3 前端
│   ├── src/
│   │   ├── main.ts             # 入口
│   │   ├── App.vue             # 根组件
│   │   ├── router/             # 路由配置
│   │   ├── stores/             # Pinia 状态管理
│   │   ├── api/                # API 接口封装
│   │   ├── views/              # 页面组件
│   │   │   ├── Dashboard.vue   # 仪表板
│   │   │   ├── sites/          # 站点管理
│   │   │   ├── keywords/       # 关键词管理
│   │   │   ├── images/         # 图片管理
│   │   │   ├── articles/       # 文章管理
│   │   │   ├── spiders/        # 爬虫项目
│   │   │   ├── generators/     # 生成器
│   │   │   └── settings/       # 系统设置
│   │   └── components/         # 公共组件
│   └── vite.config.ts          # Vite 配置
│
├── data/                       # 数据文件
├── cache/                      # 缓存文件
├── logs/                       # 日志文件
└── docker/                     # Docker 配置
    └── mysql/my.cnf            # MySQL 优化配置
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
│                      Nginx（可选反向代理）                        │
│                     - 静态文件服务                               │
│                     - HTML 缓存直服                              │
│                     - 负载均衡                                   │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                       FastAPI 应用层                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                      API 路由层                          │    │
│  │  routes.py │ spider_routes.py │ generator_routes.py     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                            │                                     │
│  ┌─────────────┬───────────┼───────────┬─────────────────────┐  │
│  │             │           │           │                     │  │
│  ▼             ▼           ▼           ▼                     ▼  │
│ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────────┐│
│ │蜘蛛识别 │ │SEO 核心 │ │HTML 缓存│ │认证模块 │ │ 后台 Workers ││
│ │Detector │ │ 渲染    │ │ 管理    │ │  JWT   │ │Generator/Stats││
│ └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────────────┘│
│                │             │                        │          │
│  ┌─────────────┼─────────────┼────────────────────────┘          │
│  │             │             │                                   │
│  ▼             ▼             ▼                                   │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │                        缓存层                                │ │
│ │  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │ │
│ │  │ 内存缓存池  │  │ Redis 缓存  │  │ 文件缓存（gzip）    │  │ │
│ │  │ 关键词/图片 │  │ 队列/段落池 │  │ HTML 页面           │  │ │
│ │  └─────────────┘  └─────────────┘  └─────────────────────┘  │ │
│ └─────────────────────────────────────────────────────────────┘ │
└───────────────────────────┬─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                         数据层                                   │
│  ┌──────────────────────────────┐  ┌─────────────────────────┐  │
│  │           MySQL              │  │        文件系统          │  │
│  │  - sites（站点）             │  │  - templates/（模板）    │  │
│  │  - keywords（关键词）        │  │  - cache/（HTML缓存）    │  │
│  │  - images（图片）            │  │  - logs/（日志）         │  │
│  │  - titles（标题库）          │  │  - data/（数据文件）     │  │
│  │  - contents（段落库）        │  │                         │  │
│  │  - spider_projects（爬虫）   │  │                         │  │
│  └──────────────────────────────┘  └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### HTML 动态生成流程

```
┌──────────────┐
│  请求到达    │
│ GET /path    │
└──────┬───────┘
       │
       ▼
┌──────────────┐     是      ┌──────────────┐
│  蜘蛛识别    │────────────▶│  继续处理    │
│ User-Agent   │             └──────┬───────┘
└──────┬───────┘                    │
       │ 否                         │
       ▼                            ▼
┌──────────────┐            ┌──────────────┐     命中     ┌──────────────┐
│  返回 404    │            │  查询缓存    │─────────────▶│  返回缓存    │
└──────────────┘            │ HTML 文件    │              │  (gzip)      │
                            └──────┬───────┘              └──────────────┘
                                   │ 未命中
                                   ▼
                            ┌──────────────┐
                            │  加载站点    │
                            │  配置信息    │
                            └──────┬───────┘
                                   │
                                   ▼
                            ┌──────────────┐
                            │  获取数据    │
                            │ - 关键词     │◀─── 缓存池优先
                            │ - 图片 URL   │◀─── 缓存池优先
                            │ - 标题       │◀─── 标题库
                            │ - 正文段落   │◀─── 段落池
                            └──────┬───────┘
                                   │
                                   ▼
                            ┌──────────────┐
                            │ Jinja2 渲染  │
                            │ - 随机关键词 │
                            │ - 随机图片   │
                            │ - 随机链接   │
                            │ - HTML 编码  │
                            └──────┬───────┘
                                   │
                                   ▼
                            ┌──────────────┐
                            │  存入缓存    │
                            │  gzip 压缩   │
                            └──────┬───────┘
                                   │
                                   ▼
                            ┌──────────────┐
                            │  返回响应    │
                            └──────────────┘
```

### 三层缓存架构

```
                    请求
                      │
                      ▼
        ┌─────────────────────────────┐
        │      L1: 文件缓存           │  ◀── 最快，持久化
        │   cache/{spider}/{domain}/  │      支持 Nginx 直服
        │      gzip 压缩存储          │
        └─────────────┬───────────────┘
                      │ 未命中
                      ▼
        ┌─────────────────────────────┐
        │   L2: 内存缓存池            │  ◀── O(1) 读取
        │   KeywordCachePool          │      预填充 10000 条
        │   ImageCachePool            │      后台异步补充
        └─────────────┬───────────────┘
                      │ 池空/降级
                      ▼
        ┌─────────────────────────────┐
        │   L3: MySQL 数据库          │  ◀── 最终数据源
        │   批量查询（10-100条/次）    │      千万级存储
        └─────────────────────────────┘
```

### 爬虫数据处理流水线

```
┌───────────────┐
│  爬虫项目     │
│ spider_project│
└───────┬───────┘
        │ 执行
        ▼
┌───────────────┐
│  抓取网页     │
│  提取内容     │
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ original_     │  ◀── 存储原始文章
│ articles      │      status=pending
└───────┬───────┘
        │
        ▼
┌───────────────┐
│ Generator     │  ◀── 后台 Worker
│ Worker        │      监听 Redis 队列
└───────┬───────┘
        │
        ├──────────────────┐
        ▼                  ▼
┌───────────────┐  ┌───────────────┐
│   titles      │  │   contents    │
│   标题库      │  │   段落库      │
│  (BIGINT ID)  │  │  (BIGINT ID)  │
└───────┬───────┘  └───────┬───────┘
        │                  │
        │                  ▼
        │          ┌───────────────┐
        │          │ ContentPool   │  ◀── 段落池（Redis SET）
        │          │ 一次性消费    │      用完自动轮转
        │          └───────────────┘
        │                  │
        └────────┬─────────┘
                 │
                 ▼
          ┌───────────────┐
          │   页面生成    │
          │   随机使用    │
          └───────────────┘
```

### 数据库核心表

| 表名 | 说明 | 主键类型 | 预估容量 |
|------|------|----------|----------|
| sites | 站点配置 | INT | 千级 |
| site_groups | 站群分组 | INT | 百级 |
| keyword_groups | 关键词分组 | INT | 百级 |
| keywords | 关键词 | UNSIGNED INT | 千万级 |
| image_groups | 图片分组 | INT | 百级 |
| images | 图片 URL | UNSIGNED INT | 千万级 |
| article_groups | 文章分组 | INT | 百级 |
| original_articles | 原始文章 | BIGINT | 百万级 |
| titles | 标题库 | BIGINT | 亿级 |
| contents | 段落库 | BIGINT | 亿级 |
| spider_projects | 爬虫项目 | INT | 百级 |
| admins | 管理员 | INT | 十级 |

## API 概览

完整 API 文档请访问 http://localhost:8009/docs

### 认证

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | /api/auth/login | 登录，返回 JWT |
| POST | /api/auth/logout | 登出 |
| GET | /api/auth/profile | 获取当前用户信息 |
| POST | /api/auth/change-password | 修改密码 |

### 站点管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/sites | 站点列表（分页） |
| POST | /api/sites | 新建站点 |
| PUT | /api/sites/{id} | 编辑站点 |
| DELETE | /api/sites/{id} | 删除站点 |
| GET | /api/sites/{id}/preview | 预览生成的 HTML |

### 关键词管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/keywords/groups | 分组列表 |
| POST | /api/keywords/groups | 新建分组 |
| GET | /api/keywords/{group_id} | 分组内关键词 |
| POST | /api/keywords/batch-import | 批量导入（支持大文件） |
| DELETE | /api/keywords/{id} | 删除关键词 |

### 图片管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/images/groups | 分组列表 |
| POST | /api/images/groups | 新建分组 |
| POST | /api/images/batch-import | 批量导入 URL |

### 文章管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/articles | 文章列表 |
| POST | /api/articles/upload | 上传文章文件 |
| GET | /api/articles/stats | 统计信息（标题数、段落数） |

### 爬虫项目

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/spider-projects | 项目列表 |
| POST | /api/spider-projects | 创建项目 |
| PUT | /api/spider-projects/{id} | 编辑项目代码 |
| POST | /api/spider-projects/{id}/test | 测试运行 |
| POST | /api/spider-projects/{id}/execute | 正式执行 |
| GET | /api/spider-projects/{id}/logs | 执行日志 |

### 系统管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /api/health | 健康检查 |
| GET | /api/settings | 获取系统配置 |
| PUT | /api/settings | 更新配置 |
| GET | /api/cache/stats | 缓存统计 |
| POST | /api/cache/clear | 清空缓存 |
| GET | /api/stats/dashboard | 仪表板数据 |

### 页面服务

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | /page | 动态生成 SEO 页面 |

**参数说明：**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| ua | string | 是 | User-Agent 字符串（如 Baiduspider） |
| path | string | 是 | 请求路径（如 /test.html） |
| domain | string | 是 | 站点域名（如 example.com） |

**示例请求：**
```
GET /page?ua=Baiduspider&path=/test.html&domain=example.com
```

### API Token 外部调用

外部系统可通过 API Token 调用数据新增接口，无需登录管理后台。

#### 配置 API Token

1. 登录管理后台，进入「系统设置」页面
2. 在「API Token」卡片中点击「生成新 Token」
3. 点击「保存」按钮保存 Token
4. 确保启用开关为开启状态

#### 调用方式

支持两种 Header 传递方式：

**方式一：X-API-Token Header（推荐）**

```bash
curl -X POST "http://localhost:8009/api/keywords/batch" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: seo_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6" \
  -d '{"group_id": 1, "keywords": ["关键词1", "关键词2"]}'
```

**方式二：Authorization Header**

```bash
curl -X POST "http://localhost:8009/api/articles/add" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer seo_a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6" \
  -d '{"group_id": 1, "title": "文章标题", "content": "文章内容"}'
```

#### 支持的接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/keywords/add` | POST | 添加单个关键词 |
| `/api/keywords/batch` | POST | 批量添加关键词（最多 10 万条/次） |
| `/api/articles/add` | POST | 添加单篇文章 |
| `/api/articles/batch` | POST | 批量添加文章（最多 1000 条/次） |
| `/api/images/urls/add` | POST | 添加单个图片 URL |
| `/api/images/urls/batch` | POST | 批量添加图片 URL（最多 10 万条/次） |

#### 请求参数示例

**添加关键词**

```json
// POST /api/keywords/add
{"group_id": 1, "keyword": "关键词"}

// POST /api/keywords/batch
{"group_id": 1, "keywords": ["关键词1", "关键词2", "关键词3"]}
```

**添加文章**

```json
// POST /api/articles/add
{"group_id": 1, "title": "文章标题", "content": "文章正文内容"}

// POST /api/articles/batch
{
  "articles": [
    {"group_id": 1, "title": "标题1", "content": "内容1"},
    {"group_id": 1, "title": "标题2", "content": "内容2"}
  ]
}
```

**添加图片 URL**

```json
// POST /api/images/urls/add
{"group_id": 1, "url": "https://example.com/image.jpg"}

// POST /api/images/urls/batch
{"group_id": 1, "urls": ["https://example.com/1.jpg", "https://example.com/2.jpg"]}
```

#### 响应格式

**成功响应**

```json
// 单条添加
{"success": true, "id": 123}

// 批量添加
{"success": true, "added": 10, "failed": 0}
```

**错误响应**

```json
// 缺少 Token (HTTP 401)
{"detail": "Missing API token"}

// Token 无效 (HTTP 401)
{"detail": "Invalid API token"}

// API Token 未启用 (HTTP 403)
{"detail": "API Token authentication is disabled"}
```

## 配置说明

### 配置文件结构

config.yaml 使用 Dynaconf 管理，支持环境覆盖：

```yaml
default:           # 通用配置
development:       # 开发环境覆盖
production:        # 生产环境覆盖
```

### 主要配置项

**服务器配置**

```yaml
server:
  host: "127.0.0.1"     # 监听地址
  port: 8000            # 端口
  workers: 1            # Worker 进程数
  debug: true           # 调试模式
```

**数据库配置**

```yaml
database:
  host: "localhost"
  port: 3306
  user: "seo_generator"
  password: "your_password"
  database: "seo_generator"
  pool_size: 5          # 连接池大小
```

**Redis 配置**

```yaml
redis:
  enabled: true
  host: "localhost"
  port: 6379
  password: "your_password"
  db: 0
```

**蜘蛛识别配置**

```yaml
spider_detector:
  enabled: true
  return_404_for_non_spider: true    # 普通用户返回 404
  dns_verify_enabled: false          # DNS 反向验证
  dns_timeout: 2.0
```

**SEO 生成配置**

```yaml
seo:
  internal_links_count: 3856         # 内链数量
  encoding_mix_ratio: 0.5            # HTML 编码混合比例
  emoji_count_min: 10                # Emoji 最少数量
  emoji_count_max: 20                # Emoji 最多数量
```

### 环境变量覆盖

生产环境可通过环境变量覆盖配置（前缀 SEO_）：

```bash
export SEO_DATABASE__HOST=db.example.com
export SEO_DATABASE__PASSWORD=prod_password
export SEO_REDIS__PASSWORD=prod_redis_pass
```

## 模板开发指南

### 模板位置

模板文件存放在 `templates/` 目录下，每个站点可以选择不同的模板。

### 可用模板变量

| 变量名 | 类型 | 说明 |
|--------|------|------|
| site | dict | 站点配置信息 |
| title | str | 生成的页面标题 |
| keywords | list | 关键词列表 |
| description | str | 页面描述 |
| content | str | 正文内容（HTML） |
| request_path | str | 请求路径 |

### 可用模板函数

| 函数名 | 说明 | 示例 |
|--------|------|------|
| `_random_keyword_sync()` | 获取随机关键词 | `{{ _random_keyword_sync() }}` |
| `_random_image_sync()` | 获取随机图片 URL | `<img src="{{ _random_image_sync() }}">` |
| `_generate_link(keyword)` | 生成内链 | `{{ _generate_link(kw) }}` |
| `_generate_class()` | 生成随机 CSS 类名 | `<div class="{{ _generate_class() }}">` |
| `_encode_html(text)` | HTML 实体编码 | `{{ _encode_html(text) }}` |
| `_random_emoji()` | 获取随机 Emoji | `{{ _random_emoji() }}` |

### 模板示例

templates/my_template/index.html

```html
<!DOCTYPE html>
<html>
<head>
    <title>{{ title }}</title>
    <meta name="keywords" content="{{ keywords|join(',') }}">
    <meta name="description" content="{{ description }}">
</head>
<body>
    <h1>{{ title }}</h1>

    <!-- 随机关键词链接 -->
    {% for i in range(10) %}
    <a href="{{ _generate_link(_random_keyword_sync()) }}">
        {{ _random_keyword_sync() }}
    </a>
    {% endfor %}

    <!-- 随机图片 -->
    <img src="{{ _random_image_sync() }}" alt="{{ _random_keyword_sync() }}">

    <!-- 正文内容 -->
    <div class="{{ _generate_class() }}">
        {{ content|safe }}
    </div>
</body>
</html>
```

### 创建新模板

1. 在 `templates/` 下创建目录
2. 创建 `index.html` 主模板
3. 在管理后台"站点管理"中选择该模板

## 爬虫开发指南

### 爬虫项目结构

每个爬虫项目是一段 Python 代码，继承自 `BaseSpider` 基类。

### 基类 API

```python
class BaseSpider:
    # 配置项
    name = "spider_name"           # 爬虫名称
    start_urls = []                # 起始 URL 列表
    allowed_domains = []           # 允许的域名
    max_concurrent = 5             # 最大并发数
    request_delay = 1.0            # 请求延迟（秒）

    # 回调方法
    def parse(self, response):
        """解析响应，返回文章或新请求"""
        pass

    def parse_article(self, response):
        """解析文章页"""
        pass
```

### 示例爬虫

```python
class MySpider(BaseSpider):
    name = "example_spider"
    start_urls = ["https://example.com/articles"]
    allowed_domains = ["example.com"]
    max_concurrent = 3
    request_delay = 2.0

    def parse(self, response):
        # 提取文章链接
        links = response.css("a.article-link::attr(href)").getall()
        for link in links:
            yield Request(link, callback=self.parse_article)

        # 提取下一页
        next_page = response.css("a.next::attr(href)").get()
        if next_page:
            yield Request(next_page, callback=self.parse)

    def parse_article(self, response):
        # 提取文章内容
        yield {
            "title": response.css("h1::text").get(),
            "content": response.css("div.content").get(),
            "source_url": response.url,
        }
```

### Response 对象方法

| 方法 | 说明 |
|------|------|
| `css(selector)` | CSS 选择器 |
| `xpath(query)` | XPath 查询 |
| `json()` | 解析 JSON 响应 |
| `text` | 响应文本 |
| `url` | 当前 URL |

### 选择器方法

| 方法 | 说明 |
|------|------|
| `.get()` | 获取第一个匹配 |
| `.getall()` | 获取所有匹配 |
| `::text` | 提取文本 |
| `::attr(name)` | 提取属性 |

### 在管理后台使用

1. 进入"爬虫项目"页面
2. 点击"新建项目"
3. 在 Monaco Editor 中编写代码
4. 点击"测试运行"验证
5. 设置定时任务或手动执行

## 数据导入指南

### 关键词导入

**格式要求：**
- 文本文件，每行一个关键词
- UTF-8 编码
- 支持 .txt 文件

**示例文件：**

```
SEO优化
网站建设
关键词排名
搜索引擎优化
```

**导入方式：**
1. 管理后台 → 关键词管理 → 选择分组 → 批量导入
2. 支持拖拽上传
3. 大文件（>10MB）会自动分批处理

### 图片导入

**格式要求：**
- 文本文件，每行一个图片 URL
- UTF-8 编码
- URL 必须是完整的 http/https 地址

**示例文件：**

```
https://example.com/images/1.jpg
https://example.com/images/2.png
https://cdn.example.com/photo.webp
```

**注意事项：**
- 系统不会下载图片，只存储 URL
- 确保图片 URL 长期有效
- 建议使用 CDN 地址

### 文章导入

**格式要求：**
- 文本文件，使用分隔符区分文章
- 默认分隔符：`---`（三个短横线独占一行）
- UTF-8 编码

**示例文件：**

```
这是第一篇文章的标题

这是第一篇文章的正文内容，可以有多个段落。

这是第二段正文。
---
这是第二篇文章的标题

这是第二篇文章的正文。
---
```

**导入后处理：**
1. 原始文章存入 `original_articles` 表
2. Generator Worker 自动处理
3. 提取标题 → `titles` 表
4. 拆分段落 → `contents` 表
5. 段落 ID 加入可用池

### 导入性能建议

| 数据量 | 建议方式 |
|--------|----------|
| < 1万 | 直接在后台导入 |
| 1万 - 100万 | 分批导入，每批 10 万 |
| > 100万 | 使用命令行工具或直接 SQL |

### 命令行批量导入（大数据量）

```bash
# 关键词批量导入
python scripts/import_keywords.py --file keywords.txt --group-id 1

# 图片批量导入
python scripts/import_images.py --file images.txt --group-id 1
```

## 生产部署建议

### 安全加固

**1. 修改默认密码**

```bash
# 修改 Admin 密码
在管理后台 → 系统设置 → 修改密码

# 修改 MySQL 密码
修改 docker-compose.yml 中的 MYSQL_PASSWORD
同步修改 app 服务的 DB_PASSWORD

# 修改 Redis 密码
修改 docker-compose.yml 中的 --requirepass
同步修改 app 服务的 REDIS_PASSWORD
```

**2. 防火墙配置**

```bash
# 只开放必要端口
ufw allow 8008/tcp  # 管理后台
ufw allow 8009/tcp  # 页面服务和API
# 不要对外开放 3306、6379
```

**3. 修改 JWT 密钥**

在 config.yaml 中修改：

```yaml
auth:
  secret_key: "your-random-secret-key-here"
```

### Nginx 反向代理

```nginx
server {
    listen 80;
    server_name example.com;

    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    # 静态文件缓存直服（可选，提升性能）
    location /cache/ {
        alias /path/to/seo_html_generator/cache/;
        gzip_static on;
        expires 7d;
    }

    # API 和动态页面
    location / {
        proxy_pass http://127.0.0.1:8009;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 生产环境配置

```bash
# 使用生产环境配置启动
ENV_FOR_DYNACONF=production docker-compose up -d
```

**生产环境 config.yaml 建议：**

```yaml
production:
  server:
    host: "0.0.0.0"
    port: 8009
    workers: 4              # 根据 CPU 核数调整
    debug: false

  database:
    pool_size: 10           # 增加连接池

  redis:
    enabled: true
```

### 资源建议

| 规模 | CPU | 内存 | 磁盘 |
|------|-----|------|------|
| 小型（<100万关键词） | 2核 | 4GB | 50GB |
| 中型（100-1000万） | 4核 | 8GB | 200GB |
| 大型（>1000万） | 8核+ | 16GB+ | 500GB+ |

## 监控与运维

### 日志位置

| 日志类型 | 位置 |
|----------|------|
| 应用日志 | `logs/app.log` |
| 访问日志 | `logs/access.log` |
| 错误日志 | `logs/error.log` |
| 爬虫日志 | `logs/spider/` |

### 查看日志

```bash
# Docker 环境
docker-compose logs -f app

# 实时查看应用日志
tail -f logs/app.log

# 查看错误日志
tail -100 logs/error.log

# 查看特定爬虫日志
tail -f logs/spider/spider_name.log
```

### 健康检查

```bash
# 应用健康检查
curl http://localhost:8009/api/health
# 返回: {"status": "ok"}
```

### 常用排查命令

```bash
# 查看服务状态
docker-compose ps

# 查看资源使用
docker stats

# 进入容器调试
docker-compose exec app bash

# 查看 MySQL 连接数
docker-compose exec mysql mysql -u root -p -e "SHOW PROCESSLIST"

# 查看 Redis 状态
docker-compose exec redis redis-cli -a your_password INFO
```

### 性能监控

管理后台提供以下监控数据：

- **仪表板**：关键词/图片/文章数量统计
- **缓存统计**：缓存命中率、缓存大小
- **系统状态**：CPU、内存、磁盘使用率

### 告警建议

建议监控以下指标：
- 磁盘使用率 > 80%
- 内存使用率 > 90%
- 应用健康检查失败
- 错误日志出现频率

## 数据备份与恢复

### MySQL 备份

**手动备份：**

```bash
# 备份整个数据库
docker-compose exec mysql mysqldump -u root -p seo_generator > backup_$(date +%Y%m%d).sql

# 备份特定表
docker-compose exec mysql mysqldump -u root -p seo_generator keywords images > data_backup.sql
```

**定时备份脚本：**

```bash
#!/bin/bash
# backup.sh
BACKUP_DIR="/path/to/backups"
DATE=$(date +%Y%m%d_%H%M%S)

# MySQL 备份
docker-compose exec -T mysql mysqldump -u root -pYOUR_PASSWORD seo_generator | gzip > $BACKUP_DIR/mysql_$DATE.sql.gz

# 保留最近 7 天
find $BACKUP_DIR -name "mysql_*.sql.gz" -mtime +7 -delete
```

**添加定时任务：**

```bash
# 每天凌晨 3 点备份
0 3 * * * /path/to/backup.sh
```

### Redis 备份

```bash
# 触发 RDB 快照
docker-compose exec redis redis-cli -a your_password BGSAVE

# 备份 RDB 文件
cp /var/lib/docker/volumes/seo_html_generator_redis_data/_data/dump.rdb ./backup/
```

### 文件备份

```bash
# 备份模板
tar -czf templates_backup.tar.gz templates/

# 备份配置
cp config.yaml config.yaml.backup

# 备份缓存（可选，可重新生成）
tar -czf cache_backup.tar.gz cache/
```

### 数据恢复

**MySQL 恢复：**

```bash
# 从备份恢复
gunzip < backup_20240124.sql.gz | docker-compose exec -T mysql mysql -u root -pYOUR_PASSWORD seo_generator

# 或者先解压再导入
gunzip backup_20240124.sql.gz
docker-compose exec -T mysql mysql -u root -pYOUR_PASSWORD seo_generator < backup_20240124.sql
```

**Redis 恢复：**

```bash
# 停止 Redis
docker-compose stop redis

# 替换 RDB 文件
cp backup/dump.rdb /var/lib/docker/volumes/seo_html_generator_redis_data/_data/

# 启动 Redis
docker-compose start redis
```

### 完整恢复流程

1. 停止服务：`docker-compose down`
2. 恢复 MySQL 数据
3. 恢复 Redis 数据（如有）
4. 恢复配置文件
5. 启动服务：`docker-compose up -d`
6. 验证：访问健康检查接口

## 常见问题

### 启动问题

**Q: 首次启动很慢？**

A: 首次启动需要：
- 构建 Docker 镜像（下载依赖、编译前端）
- 初始化数据库
- 等待健康检查通过

查看进度：docker-compose logs -f

**Q: 数据库连接失败？**

A: 检查 MySQL 服务状态：

```bash
docker-compose ps
# 确保 mysql 状态为 healthy
```

**Q: Redis 连接失败？**

A: 确保密码一致：
- docker-compose.yml 中 redis 的 --requirepass
- app 服务的 REDIS_PASSWORD 环境变量

### 使用问题

**Q: 前端页面 404？**

A: 确保前端已构建：
- Docker 部署：重新 docker-compose up -d --build
- 本地开发：cd admin-panel && npm run build

**Q: 页面不更新？**

A: 清除 HTML 缓存：
- 管理后台：缓存管理 → 清空缓存
- 或删除 cache/ 目录

**Q: 关键词/图片导入失败？**

A: 检查文件格式：
- 关键词：每行一个，UTF-8 编码
- 图片：每行一个 URL，UTF-8 编码

### 性能问题

**Q: 页面生成慢？**

A: 检查缓存池状态，确保 Redis 正常运行。可在日志中查看缓存命中率。

**Q: 内存占用高？**

A: 正常现象。系统会预加载关键词/图片 ID 到内存（千万级约占 80-100MB）。

## 贡献指南

### 开发流程

1. Fork 项目
2. 创建特性分支：git checkout -b feature/your-feature
3. 提交更改：git commit -m "Add your feature"
4. 推送分支：git push origin feature/your-feature
5. 创建 Pull Request

### 代码规范

**后端**
- 使用 Black 格式化代码
- 使用 isort 排序导入
- 遵循 PEP 8

**前端**
- 使用 ESLint + Prettier
- 组件使用 PascalCase 命名
- 使用 TypeScript 类型注解

### 提交规范

使用语义化提交信息：

```
feat: 新功能
fix: Bug 修复
docs: 文档更新
style: 代码格式
refactor: 重构
test: 测试
chore: 构建/工具
```

## License

MIT
