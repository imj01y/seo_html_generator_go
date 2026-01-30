# Go 完整重构设计方案

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:writing-plans to create detailed implementation plan.

**Goal:** 将 Python 后端完全迁移到 Go，保留 Python 用于爬虫和数据加工的动态代码执行

**Architecture:** Go 主服务处理所有 API 和页面渲染，Python 子进程执行用户编写的爬虫/数据加工代码

**Tech Stack:** Go (Gin), MySQL, Redis, OpenResty (Nginx + Lua), Python (子进程)

---

## 一、整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         管理后台 (Vue)                               │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐     │
│  │  爬虫项目管理    │  │  数据加工管理    │  │  其他管理功能    │     │
│  │  - 编辑代码     │  │  - 编辑代码      │  │  - 关键词/图片   │     │
│  │  - 启动/停止    │  │  - 启动/停止     │  │  - 站点/模板     │     │
│  │  - 查看日志     │  │  - 查看日志      │  │  - 仪表盘等      │     │
│  └────────┬────────┘  └────────┬────────┘  └─────────────────┘     │
└───────────┼────────────────────┼────────────────────────────────────┘
            │                    │
            ▼                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  OpenResty (Nginx + Lua)                            │
│  :8008 管理后台前端 + API 代理                                       │
│  :8009 SEO 页面服务 (Lua 缓存 + Go 回源)                             │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      Go 主服务 (:8080)                               │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  HTTP API Server (145+ 接口)                                 │   │
│  │  - 认证、日志、仪表盘                                         │   │
│  │  - 关键词、图片、文章、站点 CRUD                              │   │
│  │  - 模板、缓存、系统设置                                       │   │
│  │  - 爬虫项目管理、数据加工管理                                 │   │
│  │  - WebSocket 实时日志                                        │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  后台 Workers (goroutines)                                   │   │
│  │  - GeneratorWorker: 消费队列，生成标题/正文                   │   │
│  │  - SpiderScheduler: Cron 定时调度爬虫                        │   │
│  │  - StatsWorker: 聚合统计数据                                  │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │  Python 进程管理器                                            │   │
│  │  - 启动/停止 Python 子进程                                    │   │
│  │  - 实时日志流 (WebSocket)                                     │   │
│  │  - 进程状态监控                                               │   │
│  └─────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
            │
            ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│      MySQL       │  │      Redis       │  │  Python 子进程   │
│   数据持久化      │  │  队列/缓存        │  │  爬虫/数据加工   │
└──────────────────┘  └──────────────────┘  └──────────────────┘
```

---

## 二、数据流设计

```
┌──────────────┐
│ Python 爬虫   │  (用户编写的爬虫代码)
│  - 抓取数据   │
└──────┬───────┘
       │ ① 抓取到数据
       ▼
┌──────────────────────────────────────────────────────────┐
│                    Go 主服务                              │
│                                                          │
│   ② POST /api/articles/add                               │
│      ├─→ 存入数据库 (original_articles 表)               │
│      └─→ 推送到 Redis 队列 (待加工)  ───────┐            │
│                                             │            │
│   ⑤ POST /api/titles/add + /api/contents/add            │
│      └─→ 存入数据库 (titles/contents 表)    │            │
│                                             │            │
└──────────────────────────────────────────────────────────┘
                                              │
                                              ▼
                                  ┌───────────────────┐
                                  │   Redis 队列      │
                                  │  (待加工数据)      │
                                  └─────────┬─────────┘
                                            │ ③ 监听队列
                                            ▼
                                  ┌───────────────────┐
                                  │ Python 数据加工    │
                                  │  (用户编写的代码)   │
                                  │  - 清洗/转换       │
                                  └─────────┬─────────┘
                                            │ ④ 加工完成
                                            └─────────→ ⑤ 调用 Go 接口入库
```

---

## 三、端口规划

| 端口 | 服务 | 用途 |
|------|------|------|
| **8008** | Nginx | 管理后台前端 + API 代理 |
| **8009** | Nginx | SEO 页面服务 (Lua 缓存 + Go 回源) |
| 8080 | Go Server | 内部，所有 API + 页面渲染 |
| 3306 | MySQL | 内部，数据库 |
| 6379 | Redis | 内部，队列/缓存 |

---

## 四、Go 模块划分

```
go-page-server/
├── api/
│   ├── router.go              # 已有，路由入口
│   ├── auth.go                # 新增：认证 (4个接口)
│   ├── keyword.go             # 新增：关键词 CRUD (18个接口)
│   ├── image.go               # 新增：图片 CRUD (17个接口)
│   ├── article.go             # 新增：文章 CRUD (14个接口)
│   ├── site.go                # 新增：站点/站群 CRUD (20个接口)
│   ├── template_crud.go       # 新增：模板 CRUD (7个接口)
│   ├── dashboard.go           # 新增：仪表盘 (3个接口)
│   ├── cache_api.go           # 新增：缓存管理 (6个接口)
│   ├── spider_project.go      # 新增：爬虫项目管理 (28个接口)
│   ├── spider_stats.go        # 新增：爬虫统计 (4个接口)
│   ├── generator.go           # 新增：数据加工管理 (10个接口)
│   ├── generator_queue.go     # 新增：生成器队列 (6个接口)
│   ├── alerts.go              # 新增：告警 (2个接口)
│   ├── spiders.go             # 新增：蜘蛛检测 (7个接口)
│   ├── settings.go            # 新增：系统设置 (6个接口)
│   ├── logs.go                # 新增：日志查询 (3个接口)
│   └── websocket.go           # 新增：WebSocket (3个端点)
│
├── core/
│   ├── ...                    # 已有模块保持不变
│   ├── python_runner.go       # 新增：Python 进程启动/管理
│   ├── process_manager.go     # 新增：进程生命周期管理
│   ├── log_streamer.go        # 新增：实时日志流 (WebSocket)
│   ├── queue_manager.go       # 新增：Redis 队列管理
│   ├── generator_worker.go    # 新增：生成器 Worker
│   ├── spider_scheduler.go    # 新增：爬虫定时调度
│   └── stats_worker.go        # 新增：统计聚合 Worker
│
├── models/
│   ├── ...                    # 已有
│   ├── spider_project.go      # 新增：爬虫项目模型
│   ├── generator.go           # 新增：生成器模型
│   ├── article.go             # 新增：文章模型
│   ├── keyword.go             # 新增：关键词模型
│   └── image.go               # 新增：图片模型
│
└── scripts/
    └── python_wrapper.py      # Python 执行入口包装器
```

---

## 五、API 接口规划 (共 145+ 个)

### 5.1 完全用 Go 实现的接口

| 模块 | 接口数 | 说明 |
|------|--------|------|
| /api/auth | 4 | 登录、登出、profile、改密码 |
| /api/keywords | 18 | 关键词 CRUD + 批量操作 + 分组 |
| /api/images | 17 | 图片 CRUD + 批量操作 + 分组 |
| /api/articles | 14 | 文章 CRUD + 批量操作 + 分组 |
| /api/sites | 14 | 站点 CRUD + 批量操作 |
| /api/site-groups | 6 | 站群管理 |
| /api/templates | 7 | 模板 CRUD |
| /api/dashboard | 3 | 仪表盘统计 |
| /api/cache | 6 | 缓存管理 |
| /api/settings | 6 | 系统设置 |
| /api/logs | 3 | 日志查询 |
| /api/spiders | 7 | 蜘蛛检测配置和日志 |
| /api/alerts | 2 | 告警管理 |
| /api/generator/queue | 4 | 生成器队列 |
| /api/generator/worker | 2 | Worker 状态 |
| /health | 2 | 健康检查 |

### 5.2 Go 管理 + Python 执行的接口

| 模块 | 接口数 | 说明 |
|------|--------|------|
| /api/spider-projects | 28 | 爬虫项目 CRUD + 文件管理 + 运行控制 |
| /api/spider-stats | 4 | 爬虫统计 |
| /api/generators | 10 | 数据加工管理 |

### 5.3 WebSocket 端点

| 端点 | 用途 |
|------|------|
| /api/logs/ws | 系统实时日志 |
| /api/spider-projects/{id}/logs/ws | 爬虫执行日志 |
| /api/spider-projects/{id}/test/logs/ws | 爬虫测试日志 |

---

## 六、Python 进程管理

### 6.1 启动流程

1. 用户点击"启动"
2. Go 从数据库读取代码 (`spider_project_files` 表)
3. Go 写入临时目录 `/tmp/spider_project_{id}/`
4. Go 使用 `exec.Command` 启动 Python
5. Go 捕获 stdout/stderr，实时推送到 WebSocket
6. 记录进程 PID，存入内存 map

### 6.2 停止流程

1. 发送 SIGTERM 信号
2. Python 捕获信号，设置 shutdown_event
3. 用户代码检测 shutdown_event，停止新任务
4. 等待当前任务完成
5. 5秒超时后发送 SIGKILL 强制终止

### 6.3 代码存储

沿用现有设计，代码存在数据库中：
- `spider_projects` 表 - 项目信息
- `spider_project_files` 表 - 代码文件（支持多文件）

---

## 七、后台 Worker

| Worker | 功能 | 实现 |
|--------|------|------|
| GeneratorWorker | 消费 Redis 队列，调用 Python 生成器生成标题/正文 | goroutine |
| SpiderScheduler | 根据 Cron 定时调度爬虫任务 | goroutine + cron 库 |
| StatsWorker | 定期聚合爬虫统计数据到 spider_stats_history 表 | goroutine + ticker |

---

## 八、Docker 部署

### 8.1 服务组成

```yaml
services:
  mysql:       # MySQL 8.4
  redis:       # Redis 7
  go-server:   # Go 主服务 (含 Python 运行环境)
  nginx:       # OpenResty (Nginx + Lua)
```

### 8.2 一键启动

```bash
# 开发环境
cd go-page-server
docker-compose up -d

# 生产环境
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

### 8.3 Nginx 配置

沿用现有 OpenResty 配置：
- `docker/nginx/nginx.conf` - 主配置
- `docker/nginx/conf.d/admin.conf` - 8008 管理后台
- `docker/nginx/conf.d/default.conf` - 8009 SEO 页面服务

修改点：
- upstream 从 `app:8009` (Python) 改为 `go-server:8080` (Go)

---

## 九、数据库表

### 9.1 已有表（保持不变）

- site_groups, sites
- keyword_groups, keywords
- image_groups, images
- article_groups, original_articles
- titles, contents
- templates, content_generators
- admins, system_settings, system_logs
- spider_logs
- spider_projects, spider_project_files
- spider_failed_requests, spider_stats_history

### 9.2 Go 已实现的表

- scheduled_tasks (调度器)

---

## 十、实施优先级

### 第一批：基础 CRUD (简单)

1. auth - 认证
2. templates CRUD - 模板管理
3. logs - 日志查询
4. dashboard - 仪表盘

### 第二批：数据管理 (中等)

5. keywords - 关键词管理
6. images - 图片管理
7. articles - 文章管理
8. sites + site-groups - 站点管理

### 第三批：系统功能 (中等)

9. cache - 缓存管理
10. settings - 系统设置
11. spiders - 蜘蛛检测
12. alerts - 告警

### 第四批：后台 Worker (复杂)

13. generator queue + worker
14. stats worker

### 第五批：Python 进程管理 (高复杂)

15. spider-projects - 爬虫项目管理
16. generators - 数据加工管理
17. WebSocket 实时日志

---

## 十一、风险和注意事项

1. **Python 进程管理复杂度** - 需要处理信号、超时、僵尸进程等
2. **WebSocket 实现** - Gin 需要额外配置支持 WebSocket
3. **前端兼容性** - API 响应格式需与 Python 版本保持一致
4. **数据迁移** - 确保现有数据库数据兼容
5. **并发安全** - Go 中需正确使用 mutex/atomic

---

*文档创建时间: 2026-01-30*
