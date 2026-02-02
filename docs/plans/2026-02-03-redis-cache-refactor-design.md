# Redis 缓存重构设计方案

## 概述

将 Go 中的内存缓存重构为 Redis 缓存，实现 Python 生产、Go 消费的架构分离。

## 目标

1. Go 的内存缓存改为 Redis 缓存
2. Python 作为生产者，Go 作为消费者
3. 池大小和生产线程由后台配置管理
4. 标题改为实时拼接生成（关键词+emoji）
5. 安全清理旧的内存缓存代码

## 架构设计

### 新架构图

```
┌─────────────────────────────────────┐
│              Go API                 │
│  ┌─────────────────────────────┐    │
│  │ RedisConsumer (RPOP/SRAND) │    │
│  │ CacheInitializer (持久缓存)│    │
│  └─────────────────────────────┘    │
└──────────────────┬──────────────────┘
                   ▼
┌─────────────────────────────────────┐
│              Redis                  │
│  持久缓存: cache:keywords/images/emojis │
│  消费队列: queue:titles/contents/...   │
└──────────────────┬──────────────────┘
                   ▲
┌──────────────────┴──────────────────┐
│         Python CacheProducer        │
│  监控队列长度，低于阈值时批量补充     │
└─────────────────────────────────────┘
```

## Redis 数据结构设计

### 持久缓存（不消费，随机提取）

| 缓存 | Redis Key | 类型 | 数据来源 | 操作 |
|------|-----------|------|----------|------|
| 关键词 | `cache:keywords:{group_id}` | SET | keywords 表 | SRANDMEMBER / SADD |
| 图片URL | `cache:images:{group_id}` | SET | images 表 | SRANDMEMBER / SADD |
| Emoji | `cache:emojis` | SET | emojis 表 | SRANDMEMBER / SADD |

### 消费型缓存（生产-消费队列）

| 缓存 | Redis Key | 类型 | 生产方式 | 操作 |
|------|-----------|------|----------|------|
| 标题池 | `queue:titles:{group_id}` | LIST | 关键词+emoji 拼接 | RPOP / LPUSH |
| 正文池 | `queue:contents:{group_id}` | LIST | 从 contents 表读取 | RPOP / LPUSH |
| CSS类名池 | `queue:css_classes` | LIST | 随机生成 | RPOP / LPUSH |
| URL池 | `queue:urls` | LIST | 随机生成 | RPOP / LPUSH |
| 关键词表情池 | `queue:keyword_emojis:{group_id}` | LIST | 关键词+emoji 组合 | RPOP / LPUSH |

## Python CacheProducer 设计

### 目录结构

```
content_worker/core/workers/
├── generator_worker.py      # 现有：文章 → contents 表
├── cache_producer.py        # 新增：生产各类缓存队列
└── producers/               # 新增：各池的生产器
    ├── __init__.py
    ├── base.py              # 生产器基类
    ├── title_producer.py    # 标题生产器（关键词+emoji拼接）
    ├── content_producer.py  # 正文生产器（从contents表读取）
    ├── css_producer.py      # CSS类名生产器
    ├── url_producer.py      # URL生产器
    └── keyword_emoji_producer.py  # 关键词表情生产器
```

### CacheProducer 工作流程

```python
class CacheProducer:
    async def run(self):
        # 1. 启动时加载配置（从 cache_pool_config 表）
        # 2. 初始化各生产器
        # 3. 启动监控循环
        while True:
            for pool_type, group_id in self.pools:
                current_len = await redis.llen(f"queue:{pool_type}:{group_id}")
                if current_len < config.min_threshold:
                    items = await producer.produce(config.batch_size)
                    await redis.lpush(f"queue:{pool_type}:{group_id}", *items)
            await asyncio.sleep(config.check_interval_ms / 1000)
```

### 各生产器逻辑

#### TitleProducer（标题生产器）

```python
def produce(self, count: int) -> List[str]:
    """生成标题：关键词1 + emoji1 + 关键词2 + emoji2 + 关键词3"""
    titles = []
    for _ in range(count):
        # 从 Redis Set 随机取 3 个关键词
        keywords = redis.srandmember(f"cache:keywords:{group_id}", 3)
        # 从 Redis Set 随机取 2 个 emoji
        emojis = redis.srandmember("cache:emojis", 2)
        # 拼接
        title = keywords[0] + emojis[0] + keywords[1] + emojis[1] + keywords[2]
        titles.append(title)
    return titles
```

#### ContentProducer（正文生产器）

```python
def produce(self, count: int) -> List[str]:
    """从 contents 表随机读取正文"""
    query = """
        SELECT content FROM contents
        WHERE group_id = %s AND status = 1
        ORDER BY batch_id DESC, RAND()
        LIMIT %s
    """
    return db.fetchall(query, [group_id, count])
```

#### CssProducer（CSS类名生产器）

```python
CHARS = "abcdefghijklmnopqrstuvwxyz0123456789"

def produce(self, count: int) -> List[str]:
    """生成 CSS 类名：13字符 + 空格 + 32字符"""
    results = []
    for _ in range(count):
        part1 = ''.join(random.choices(CHARS, k=13))
        part2 = ''.join(random.choices(CHARS, k=32))
        results.append(f"{part1} {part2}")
    return results
```

#### UrlProducer（URL生产器）

```python
def produce(self, count: int) -> List[str]:
    """生成 URL：60% 随机数格式，40% 日期格式"""
    results = []
    for _ in range(count):
        if random.random() < 0.6:
            num = random.randint(100000000, 999999999)
            results.append(f"/?{num}.html")
        else:
            days_ago = random.randint(0, 29)
            date = (datetime.now() - timedelta(days=days_ago)).strftime("%Y%m%d")
            num = random.randint(10000, 99999)
            results.append(f"/?{date}/{num}.html")
    return results
```

#### KeywordEmojiProducer（关键词表情生产器）

```python
def produce(self, count: int) -> List[str]:
    """关键词 + 随机插入 1-2 个 emoji"""
    results = []
    for _ in range(count):
        keyword = self._next_keyword()  # 轮询取关键词
        runes = list(keyword)

        emoji_count = 1 if random.random() < 0.5 else 2
        used_emojis = set()

        for _ in range(emoji_count):
            emoji = self._get_random_emoji_exclude(used_emojis)
            if emoji:
                used_emojis.add(emoji)
                pos = random.randint(0, len(runes))
                runes.insert(pos, emoji)

        results.append(''.join(runes))
    return results
```

## Go 消费端设计

### RedisConsumer 服务

**文件**: `api/internal/service/redis_consumer.go`

```go
type RedisConsumer struct {
    rdb *redis.Client
}

// 消费型缓存：RPOP 取出（消费即删除）
func (c *RedisConsumer) PopTitle(groupID int) (string, error) {
    return c.rdb.RPop(ctx, fmt.Sprintf("queue:titles:%d", groupID)).Result()
}

func (c *RedisConsumer) PopContent(groupID int) (string, error) {
    return c.rdb.RPop(ctx, fmt.Sprintf("queue:contents:%d", groupID)).Result()
}

func (c *RedisConsumer) PopCssClass() (string, error) {
    return c.rdb.RPop(ctx, "queue:css_classes").Result()
}

func (c *RedisConsumer) PopUrl() (string, error) {
    return c.rdb.RPop(ctx, "queue:urls").Result()
}

func (c *RedisConsumer) PopKeywordEmoji(groupID int) (string, error) {
    return c.rdb.RPop(ctx, fmt.Sprintf("queue:keyword_emojis:%d", groupID)).Result()
}

// 持久缓存：SRANDMEMBER 随机取（不删除）
func (c *RedisConsumer) RandomKeyword(groupID int, count int) ([]string, error) {
    return c.rdb.SRandMemberN(ctx, fmt.Sprintf("cache:keywords:%d", groupID), int64(count)).Result()
}

func (c *RedisConsumer) RandomImage(groupID int) (string, error) {
    return c.rdb.SRandMember(ctx, fmt.Sprintf("cache:images:%d", groupID)).Result()
}

func (c *RedisConsumer) RandomEmoji() (string, error) {
    return c.rdb.SRandMember(ctx, "cache:emojis").Result()
}
```

### CacheInitializer 服务

**文件**: `api/internal/service/cache_initializer.go`

```go
type CacheInitializer struct {
    db  *sqlx.DB
    rdb *redis.Client
}

func (s *CacheInitializer) InitPersistentCaches(ctx context.Context) error {
    // 1. 加载所有分组的关键词到 Redis Set
    groups := s.db.Query("SELECT DISTINCT group_id FROM keywords WHERE status=1")
    for _, gid := range groups {
        keywords := s.db.Query("SELECT keyword FROM keywords WHERE group_id=? AND status=1", gid)
        if len(keywords) > 0 {
            key := fmt.Sprintf("cache:keywords:%d", gid)
            s.rdb.Del(ctx, key)
            s.rdb.SAdd(ctx, key, keywords...)
        }
    }

    // 2. 加载所有分组的图片到 Redis Set
    for _, gid := range groups {
        images := s.db.Query("SELECT url FROM images WHERE group_id=? AND status=1", gid)
        if len(images) > 0 {
            key := fmt.Sprintf("cache:images:%d", gid)
            s.rdb.Del(ctx, key)
            s.rdb.SAdd(ctx, key, images...)
        }
    }

    // 3. 加载 Emoji（全局）
    emojis := s.db.Query("SELECT emoji FROM emojis WHERE status=1")
    if len(emojis) > 0 {
        s.rdb.Del(ctx, "cache:emojis")
        s.rdb.SAdd(ctx, "cache:emojis", emojis...)
    }

    return nil
}
```

### API 写入时同步更新

```go
// 新增关键词时
func (h *KeywordHandler) Create(c *gin.Context) {
    // 1. 写入数据库
    db.Insert("INSERT INTO keywords ...")

    // 2. 同步到 Redis Set
    h.rdb.SAdd(ctx, fmt.Sprintf("cache:keywords:%d", groupID), keyword)
}

// 删除关键词时
func (h *KeywordHandler) Delete(c *gin.Context) {
    // 1. 从数据库删除
    db.Delete("DELETE FROM keywords WHERE id=?", id)

    // 2. 从 Redis Set 移除
    h.rdb.SRem(ctx, fmt.Sprintf("cache:keywords:%d", groupID), keyword)
}
```

## 配置管理

### 数据库表设计

```sql
CREATE TABLE cache_pool_config (
    id INT AUTO_INCREMENT PRIMARY KEY,
    pool_type VARCHAR(50) NOT NULL,      -- titles/contents/css_classes/urls/keyword_emojis
    min_threshold INT DEFAULT 10000,      -- 最小阈值，低于此值触发补充
    max_size INT DEFAULT 100000,          -- 队列最大容量
    batch_size INT DEFAULT 5000,          -- 单次补充数量
    producer_threads INT DEFAULT 4,       -- 生产线程数
    check_interval_ms INT DEFAULT 1000,   -- 检查间隔（毫秒）
    enabled TINYINT DEFAULT 1,            -- 是否启用
    updated_at DATETIME DEFAULT NOW() ON UPDATE NOW(),
    UNIQUE INDEX idx_pool_type (pool_type)
);
```

### 默认配置

| pool_type | min_threshold | max_size | batch_size | producer_threads |
|-----------|---------------|----------|------------|------------------|
| titles | 10000 | 100000 | 5000 | 4 |
| contents | 10000 | 100000 | 5000 | 4 |
| css_classes | 50000 | 500000 | 50000 | 8 |
| urls | 30000 | 300000 | 30000 | 6 |
| keyword_emojis | 50000 | 500000 | 50000 | 8 |

### 后台管理 API

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/cache/pools/status` | 获取所有池状态（长度、配置） |
| GET | `/api/cache/pools/config` | 获取池配置列表 |
| PUT | `/api/cache/pools/config/:type` | 更新单个池配置 |
| POST | `/api/cache/pools/:type/refill` | 手动触发补充 |
| POST | `/api/cache/keywords/:group_id/reload` | 刷新关键词缓存 |
| POST | `/api/cache/images/:group_id/reload` | 刷新图片缓存 |
| POST | `/api/cache/emojis/reload` | 刷新Emoji缓存 |

## 旧代码清理

### Go 端清理清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `api/internal/service/object_pool.go` | **删除整个文件** | 环形缓冲池实现，不再需要 |
| `api/internal/service/pool_manager.go` | **大幅重构** | 删除内存池逻辑，保留 Redis 消费接口 |
| `api/internal/service/template_funcs.go` | **重构** | 删除 clsPool/urlPool/keywordEmojiPool，改为调用 RedisConsumer |
| `api/internal/service/emoji_manager.go` | **重构** | 删除内存存储，改为从 Redis SRANDMEMBER |
| `api/internal/handler/pool.go` | **重构** | 适配新的配置表结构 |

### Python 端清理清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `content_worker/core/title_manager.py` | **删除整个文件** | 三层分层逻辑不再需要 |
| `content_worker/core/workers/generator_worker.py` | **修改** | 删除 save_title 相关逻辑，保留正文处理 |

### 数据库清理

| 表 | 操作 | 说明 |
|---|------|------|
| `titles` | **保留但停用** | 不再写入新数据，历史数据保留 |
| `pool_config` | **删除或重命名** | 被新表 `cache_pool_config` 替代 |

### 清理顺序（安全策略）

1. 先部署新方案（Python CacheProducer + Go RedisConsumer）
2. 验证新方案正常工作（监控队列长度、页面渲染正常）
3. 灰度切换：新旧共存，逐步切流量
4. 确认稳定后，再清理旧代码
5. 最后清理数据库（保留一段时间后再决定是否删除 titles 表数据）

## 文件变更汇总

| 类型 | 新增 | 修改 | 删除 |
|------|------|------|------|
| **Go** | redis_consumer.go, cache_initializer.go | pool_manager.go, template_funcs.go, emoji_manager.go, pool.go | object_pool.go |
| **Python** | cache_producer.py, producers/*.py | generator_worker.py, main.py | title_manager.py |
| **前端** | - | CacheManage.vue | - |
| **数据库** | cache_pool_config 表 | - | pool_config 表 |
