-- 添加标题生成配置字段到 pool_config 表
ALTER TABLE pool_config
ADD COLUMN title_pool_size INT DEFAULT 5000 AFTER refresh_interval_ms,
ADD COLUMN title_workers INT DEFAULT 2 AFTER title_pool_size,
ADD COLUMN title_refill_interval_ms INT DEFAULT 500 AFTER title_workers,
ADD COLUMN title_threshold INT DEFAULT 1000 AFTER title_refill_interval_ms;

-- 更新现有记录的默认值
UPDATE pool_config SET
  title_pool_size = 5000,
  title_workers = 2,
  title_refill_interval_ms = 500,
  title_threshold = 1000
WHERE id = 1;
