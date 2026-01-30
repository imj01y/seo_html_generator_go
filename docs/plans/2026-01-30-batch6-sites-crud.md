# Batch 6: 站点管理模块实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现站点管理的 14 个 API 接口，包括站点 CRUD、批量操作、站群管理

**Architecture:** Go Gin 框架，沿用已有认证中间件和响应格式

**Tech Stack:** Go 1.22+, Gin, sqlx, MySQL

---

## API 接口列表 (14个)

### 站点管理 (5个)
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/sites | 获取站点列表（分页） |
| POST | /api/sites | 创建站点 |
| GET | /api/sites/:id | 获取站点详情 |
| PUT | /api/sites/:id | 更新站点 |
| DELETE | /api/sites/:id | 删除站点 |

### 站点批量操作 (2个)
| 方法 | 路径 | 功能 |
|------|------|------|
| DELETE | /api/sites/batch/delete | 批量删除 |
| PUT | /api/sites/batch/status | 批量更新状态 |

### 站群管理 (6个)
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/site-groups | 获取站群列表 |
| GET | /api/site-groups/:id | 获取站群详情 |
| GET | /api/site-groups/:id/options | 获取站群资源选项 |
| POST | /api/site-groups | 创建站群 |
| PUT | /api/site-groups/:id | 更新站群 |
| DELETE | /api/site-groups/:id | 删除站群 |

### 分组选项 (1个)
| 方法 | 路径 | 功能 |
|------|------|------|
| GET | /api/groups/options | 获取所有分组选项 |

---

## Task 1-5: 创建完整的站点模块

创建 `go-page-server/api/sites.go`，包含所有结构体和方法。

---

## Task 6: 注册站点路由

在 router.go 中添加站点相关路由。

---

## Task 7: 添加站点 API 测试

创建 `go-page-server/api/sites_test.go`。

---

## Task 8: 集成测试

运行所有测试，验证编译。

---

*文档创建时间: 2026-01-30*
