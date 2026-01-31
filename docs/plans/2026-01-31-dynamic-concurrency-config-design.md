# 动态并发配置设计

> **Goal:** 用户只需调整一个"并发数"参数，系统自动计算并动态调整 Go 对象池和 Python 数据池的大小，支持预设方案和自定义配置。

**依赖:** 2024-01-29-object-pool-enhancement.md（对象池增强）

---

## 一、设计概述

### 核心思想

将复杂的池配置简化为单一参数控制：

```
用户设置并发数 → 系统自动计算所有池大小 → 热更新生效
```

### 涉及的池

| 层级 | 池名称 | 说明 |
|------|--------|------|
| **Go 对象池** | clsPool | CSS 类名池 |
| | urlPool | URL 池（内链） |
| | keywordEmojiPool | 关键词表情池 |
| **Python 数据池** | KeywordCachePool | 关键词缓存池 |
| | ImageCachePool | 图片缓存池 |

---

## 二、预设等级

| 等级 | 并发数 | 适用场景 |
|------|--------|----------|
| 低 | 50 | 小站点、低配服务器 |
| 中 | 200 | 中等规模站群 |
| 高 | 500 | 大规模站群 |
| 极高 | 1000 | 高性能服务器 |
| 自定义 | 用户输入 | 特殊需求 |

---

## 三、计算公式

### 核心公式

```
池大小 = 并发数 × 单页最大消耗 × 缓冲秒数
```

### 参数说明

| 参数 | 说明 | 来源 |
|------|------|------|
| 并发数 | 用户设置或选择预设 | 管理后台配置 |
| 单页最大消耗 | 所有模板中调用次数最多的值 | 模板分析结果 |
| 缓冲秒数 | 默认 10 秒，可调 5-30 秒 | 高级选项 |

### 示例计算

假设：
- 并发数 = 200
- 单页最大关键词消耗 = 120（来自 landing_page.html）
- 单页最大图片消耗 = 45
- 缓冲秒数 = 10

计算结果：
```
关键词池大小 = 200 × 120 × 10 = 240,000
图片池大小 = 200 × 45 × 10 = 90,000
```

---

## 四、模板分析

### 触发时机

- 上传新模板后异步分析
- 修改模板后异步分析

### 分析规则

| 调用类型 | 处理方式 |
|----------|----------|
| 固定调用 | 直接计数 |
| 动态范围（如 `N 5 20`） | 取最大值 20 |

### 存储位置

在 `templates` 表增加字段：

```sql
ALTER TABLE templates ADD COLUMN keyword_count INT DEFAULT 0 COMMENT '关键词调用次数';
ALTER TABLE templates ADD COLUMN image_count INT DEFAULT 0 COMMENT '图片调用次数';
ALTER TABLE templates ADD COLUMN content_count INT DEFAULT 0 COMMENT '段落调用次数';
ALTER TABLE templates ADD COLUMN cls_count INT DEFAULT 0 COMMENT 'CSS类名调用次数';
ALTER TABLE templates ADD COLUMN url_count INT DEFAULT 0 COMMENT 'URL调用次数';
ALTER TABLE templates ADD COLUMN keyword_emoji_count INT DEFAULT 0 COMMENT '关键词表情调用次数';
ALTER TABLE templates ADD COLUMN analyzed_at DATETIME COMMENT '分析时间';
```

---

## 五、内存预估

### 单条数据大小估算

| 数据类型 | 预估大小 |
|----------|----------|
| 关键词 | 50 字节 |
| 图片 URL | 150 字节 |
| CSS 类名 | 20 字节 |
| 内链 URL | 100 字节 |
| 关键词表情 | 60 字节 |

### 计算公式

```
关键词池内存 = 池大小 × 50 bytes
图片池内存 = 池大小 × 150 bytes
...
总内存 ≈ (所有池内存之和) × 1.2（含 Set 去重开销）
```

### 展示格式

```
并发等级：中 (200)
├── 关键词池：240,000 条 → 约 14 MB
├── 图片池：90,000 条 → 约 16 MB
├── CSS 类名池：... → 约 X MB
├── URL 池：... → 约 X MB
└── 预估总内存：约 XX MB
```

---

## 六、配置存储

### 数据库存储

在 `system_settings` 表存储配置：

| setting_key | setting_value | 说明 |
|-------------|---------------|------|
| pool.concurrency_preset | medium | 预设等级 |
| pool.concurrency_custom | 300 | 自定义并发数（当 preset=custom 时使用） |
| pool.buffer_seconds | 10 | 缓冲秒数 |

### 配置结构

```json
{
  "preset": "medium",
  "custom_concurrency": 300,
  "buffer_seconds": 10
}
```

---

## 七、管理后台 UI

```
┌─────────────────────────────────────────────────────────┐
│  渲染并发配置                                            │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  并发等级:  ○ 低(50)  ○ 中(200)  ● 高(500)  ○ 极高(1000) │
│            ○ 自定义: [____]                             │
│                                                         │
│  ▼ 高级选项                                             │
│  ┌─────────────────────────────────────────────────────┐│
│  │ 缓冲时间: [10] 秒  (5-30)                           ││
│  └─────────────────────────────────────────────────────┘│
│                                                         │
│  ─────────────── 资源预估 ───────────────               │
│                                                         │
│  模板基准 (取自: landing_page.html)                     │
│  ├── 单页关键词: 120 个                                 │
│  ├── 单页图片: 45 个                                    │
│  └── 单页段落: 30 个                                    │
│                                                         │
│  池大小预估                                             │
│  ├── 关键词池: 600,000 条                              │
│  ├── 图片池: 270,000 条                                │
│  ├── CSS 类名池: ...                                   │
│  └── URL 池: ...                                       │
│                                                         │
│  内存预估                                               │
│  ├── Go 对象池: 约 XX MB                               │
│  ├── Python 数据池: 约 XX MB                           │
│  └── 总计: 约 XX MB                                    │
│                                                         │
│                              [取消]  [应用配置]          │
└─────────────────────────────────────────────────────────┘
```

---

## 八、配置生效流程

### 热更新流程

```
用户点击"应用配置"
       ↓
保存配置到 system_settings
       ↓
发送 Redis 消息: pool:reload
       ↓
  ┌────┴────┐
  ↓         ↓
Go 端      Python 端
  ↓         ↓
读取配置   读取配置
  ↓         ↓
计算池大小  计算池大小
  ↓         ↓
调用 Resize 调整缓存池
  ↓         ↓
触发补充    触发补充
```

### Redis 消息格式

```json
{
  "action": "reload",
  "config": {
    "concurrency": 500,
    "buffer_seconds": 10
  }
}
```

---

## 九、API 设计

### 获取当前配置

```
GET /api/settings/pool-config
```

Response:
```json
{
  "preset": "high",
  "concurrency": 500,
  "buffer_seconds": 10,
  "template_stats": {
    "max_keyword": 120,
    "max_image": 45,
    "max_content": 30,
    "max_cls": 80,
    "max_url": 50,
    "max_keyword_emoji": 40,
    "source_template": "landing_page.html"
  },
  "calculated": {
    "keyword_pool_size": 600000,
    "image_pool_size": 225000,
    "cls_pool_size": 400000,
    "url_pool_size": 250000,
    "keyword_emoji_pool_size": 200000
  },
  "memory_estimate": {
    "keyword_pool_mb": 36,
    "image_pool_mb": 40,
    "cls_pool_mb": 10,
    "url_pool_mb": 30,
    "keyword_emoji_pool_mb": 14,
    "total_mb": 156
  }
}
```

### 更新配置

```
PUT /api/settings/pool-config
```

Request:
```json
{
  "preset": "custom",
  "concurrency": 300,
  "buffer_seconds": 15
}
```

### 获取预设列表

```
GET /api/settings/pool-presets
```

Response:
```json
{
  "presets": [
    {"key": "low", "name": "低", "concurrency": 50},
    {"key": "medium", "name": "中", "concurrency": 200},
    {"key": "high", "name": "高", "concurrency": 500},
    {"key": "extreme", "name": "极高", "concurrency": 1000}
  ]
}
```

---

## 十、数据库变更

### templates 表新增字段

```sql
ALTER TABLE templates
  ADD COLUMN keyword_count INT DEFAULT 0 COMMENT '关键词调用次数',
  ADD COLUMN image_count INT DEFAULT 0 COMMENT '图片调用次数',
  ADD COLUMN content_count INT DEFAULT 0 COMMENT '段落调用次数',
  ADD COLUMN cls_count INT DEFAULT 0 COMMENT 'CSS类名调用次数',
  ADD COLUMN url_count INT DEFAULT 0 COMMENT 'URL调用次数',
  ADD COLUMN keyword_emoji_count INT DEFAULT 0 COMMENT '关键词表情调用次数',
  ADD COLUMN analyzed_at DATETIME COMMENT '分析时间';
```

### system_settings 初始数据

```sql
INSERT INTO system_settings (setting_key, setting_value) VALUES
  ('pool.concurrency_preset', 'medium'),
  ('pool.concurrency_custom', '200'),
  ('pool.buffer_seconds', '10');
```

---

## 十一、实现任务清单

### 阶段 1：模板分析器

- [ ] 实现模板函数调用分析器（Go）
- [ ] 处理固定调用和动态范围
- [ ] 模板上传后异步触发分析
- [ ] 保存分析结果到 templates 表

### 阶段 2：配置管理 API

- [ ] 获取当前配置 API
- [ ] 更新配置 API
- [ ] 获取预设列表 API
- [ ] 计算池大小和内存预估逻辑

### 阶段 3：Go 端热更新

- [ ] 监听 Redis pool:reload 消息
- [ ] 读取配置并计算池大小
- [ ] 调用各池的 Resize 方法
- [ ] 触发池补充

### 阶段 4：Python 端热更新

- [ ] 监听 Redis pool:reload 消息
- [ ] 读取配置并计算池大小
- [ ] 调整 KeywordCachePool 和 ImageCachePool
- [ ] 触发池补充

### 阶段 5：前端管理页面

- [ ] 渲染并发配置页面
- [ ] 预设等级选择
- [ ] 自定义并发输入
- [ ] 高级选项（缓冲秒数）
- [ ] 资源预估和内存展示
- [ ] 应用配置按钮

---

## 十二、注意事项

1. **内存安全**：在应用配置前检查内存预估是否超过系统可用内存
2. **平滑过渡**：Resize 时保留现有数据，避免突然清空
3. **模板分析失败**：使用默认值（如单页 100 次调用）
4. **配置回滚**：保留上一次配置，支持快速回滚
