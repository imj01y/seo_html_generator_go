# 修复 000_init.sql 数据库结构设计

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 修复 000_init.sql 使其包含完整正确的数据库结构，确保首次 Docker 部署能正常初始化数据库

**Architecture:** 将所有迁移文件内容合并到 000_init.sql，添加缺失的表和优化索引，删除冗余迁移文件

**Tech Stack:** MySQL 8.0, SQL DDL

---

## 问题分析

### 已合并内容
- ✅ 001_add_title_config.sql - title 相关字段已在 pool_config 表中
- ✅ 002_unified_pool_config.sql - cls/url/keyword_emoji 池配置已在 pool_config 表中

### 缺失内容
- ❌ spider_logs_stats 表完全缺失（来自 003_spider_logs_stats.sql）

### 索引优化
- spider_logs 表需要添加复合索引 `(spider_type, domain, created_at)`

---

## 修改方案

### Task 1: 修改 000_init.sql

**Files:**
- Modify: `migrations/000_init.sql`

**Step 1: 在 spider_logs 表后添加 spider_logs_stats 表定义**

位置：第 277 行（spider_logs 表结束）之后

```sql
-- ============================================
-- 蜘蛛日志统计预聚合表
-- ============================================
CREATE TABLE IF NOT EXISTS spider_logs_stats (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    period_type ENUM('minute', 'hour', 'day', 'month') NOT NULL COMMENT '周期类型',
    period_start DATETIME NOT NULL COMMENT '周期开始时间',
    spider_type VARCHAR(20) DEFAULT NULL COMMENT '蜘蛛类型，NULL表示全部汇总',
    total INT UNSIGNED DEFAULT 0 COMMENT '访问次数',
    status_2xx INT UNSIGNED DEFAULT 0 COMMENT '2xx响应数',
    status_3xx INT UNSIGNED DEFAULT 0 COMMENT '3xx响应数',
    status_4xx INT UNSIGNED DEFAULT 0 COMMENT '4xx响应数',
    status_5xx INT UNSIGNED DEFAULT 0 COMMENT '5xx响应数',
    avg_resp_time INT UNSIGNED DEFAULT 0 COMMENT '平均响应时间(ms)',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,

    UNIQUE KEY uk_period_spider (period_type, period_start, spider_type),
    INDEX idx_query (period_type, period_start DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='蜘蛛日志统计表';
```

**Step 2: 优化 spider_logs 表索引**

在现有索引后添加复合索引：

```sql
INDEX idx_type_domain_time (spider_type, domain, created_at)
```

### Task 2: 删除冗余迁移文件

**Files:**
- Delete: `migrations/001_add_title_config.sql`
- Delete: `migrations/002_unified_pool_config.sql`
- Delete: `migrations/003_spider_logs_stats.sql`

---

## 最终结果

migrations 目录只保留：
- `000_init.sql` （完整的数据库初始化脚本）
