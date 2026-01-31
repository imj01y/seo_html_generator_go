-- 004_template_stats.sql
-- 为 templates 表添加函数调用统计字段

ALTER TABLE templates
  ADD COLUMN cls_count INT UNSIGNED DEFAULT 0 COMMENT 'cls() 调用次数',
  ADD COLUMN url_count INT UNSIGNED DEFAULT 0 COMMENT 'random_url() 调用次数',
  ADD COLUMN keyword_emoji_count INT UNSIGNED DEFAULT 0 COMMENT 'keyword_with_emoji() 调用次数',
  ADD COLUMN keyword_count INT UNSIGNED DEFAULT 0 COMMENT 'random_keyword() 调用次数',
  ADD COLUMN image_count INT UNSIGNED DEFAULT 0 COMMENT 'random_image() 调用次数',
  ADD COLUMN title_count INT UNSIGNED DEFAULT 0 COMMENT 'random_title() 调用次数',
  ADD COLUMN content_count INT UNSIGNED DEFAULT 0 COMMENT 'random_content() 调用次数',
  ADD COLUMN analyzed_at DATETIME DEFAULT NULL COMMENT '最后分析时间',
  ADD INDEX idx_analyzed_at (analyzed_at);

-- 添加池配置相关的系统设置
INSERT INTO system_settings (setting_key, setting_value, setting_type, description) VALUES
  ('pool.concurrency_preset', 'medium', 'string', '并发预设: low/medium/high/extreme/custom'),
  ('pool.concurrency_custom', '200', 'number', '自定义并发数'),
  ('pool.buffer_seconds', '10', 'number', '缓冲秒数 (5-30)')
ON DUPLICATE KEY UPDATE
  setting_value = VALUES(setting_value),
  setting_type = VALUES(setting_type),
  description = VALUES(description);
