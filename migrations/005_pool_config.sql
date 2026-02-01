-- migrations/005_pool_config.sql
-- 缓存池配置表

CREATE TABLE IF NOT EXISTS pool_config (
    id INT PRIMARY KEY DEFAULT 1,
    titles_size INT NOT NULL DEFAULT 5000 COMMENT '标题池大小',
    contents_size INT NOT NULL DEFAULT 5000 COMMENT '正文池大小',
    threshold INT NOT NULL DEFAULT 1000 COMMENT '补充阈值',
    refill_interval_ms INT NOT NULL DEFAULT 1000 COMMENT '检查间隔(毫秒)',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT chk_id CHECK (id = 1)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='缓存池配置（单行）';

-- 插入默认配置
INSERT IGNORE INTO pool_config (id, titles_size, contents_size, threshold, refill_interval_ms)
VALUES (1, 5000, 5000, 1000, 1000);
