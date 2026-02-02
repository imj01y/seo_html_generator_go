-- 统一对象池配置迁移
-- 将 4 个对象池（标题池、cls类名池、url池、关键词表情池）的配置字段统一化

-- 1. 修改 title_threshold 为 DECIMAL(3,2) 类型
ALTER TABLE pool_config
MODIFY COLUMN title_threshold DECIMAL(3,2) DEFAULT 0.40;

-- 2. 添加 cls 类名池配置字段
ALTER TABLE pool_config
ADD COLUMN cls_pool_size INT DEFAULT 800000 AFTER title_threshold,
ADD COLUMN cls_workers INT DEFAULT 20 AFTER cls_pool_size,
ADD COLUMN cls_refill_interval_ms INT DEFAULT 30 AFTER cls_workers,
ADD COLUMN cls_threshold DECIMAL(3,2) DEFAULT 0.40 AFTER cls_refill_interval_ms;

-- 3. 添加 url 池配置字段
ALTER TABLE pool_config
ADD COLUMN url_pool_size INT DEFAULT 500000 AFTER cls_threshold,
ADD COLUMN url_workers INT DEFAULT 16 AFTER url_pool_size,
ADD COLUMN url_refill_interval_ms INT DEFAULT 30 AFTER url_workers,
ADD COLUMN url_threshold DECIMAL(3,2) DEFAULT 0.40 AFTER url_refill_interval_ms;

-- 4. 添加关键词表情池配置字段
ALTER TABLE pool_config
ADD COLUMN keyword_emoji_pool_size INT DEFAULT 800000 AFTER url_threshold,
ADD COLUMN keyword_emoji_workers INT DEFAULT 20 AFTER keyword_emoji_pool_size,
ADD COLUMN keyword_emoji_refill_interval_ms INT DEFAULT 30 AFTER keyword_emoji_workers,
ADD COLUMN keyword_emoji_threshold DECIMAL(3,2) DEFAULT 0.40 AFTER keyword_emoji_refill_interval_ms;

-- 5. 更新现有记录（id=1）设置默认值
UPDATE pool_config SET
  title_threshold = 0.40,
  cls_pool_size = 800000,
  cls_workers = 20,
  cls_refill_interval_ms = 30,
  cls_threshold = 0.40,
  url_pool_size = 500000,
  url_workers = 16,
  url_refill_interval_ms = 30,
  url_threshold = 0.40,
  keyword_emoji_pool_size = 800000,
  keyword_emoji_workers = 20,
  keyword_emoji_refill_interval_ms = 30,
  keyword_emoji_threshold = 0.40
WHERE id = 1;
