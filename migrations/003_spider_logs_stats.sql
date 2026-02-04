-- migrations/003_spider_logs_stats.sql
-- 蜘蛛日志统计预聚合表

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
