-- 定时任务表
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '任务名称',
    task_type VARCHAR(50) NOT NULL COMMENT '任务类型: refresh_data, refresh_template, clear_cache, push_urls',
    cron_expr VARCHAR(100) NOT NULL COMMENT 'Cron表达式',
    params JSON COMMENT '任务参数',
    enabled TINYINT(1) DEFAULT 1 COMMENT '是否启用',
    last_run_at DATETIME COMMENT '上次执行时间',
    next_run_at DATETIME COMMENT '下次执行时间',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_enabled (enabled),
    INDEX idx_next_run (next_run_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='定时任务表';

-- 任务执行日志表
CREATE TABLE IF NOT EXISTS task_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id INT NOT NULL COMMENT '任务ID',
    status ENUM('running', 'success', 'failed') NOT NULL DEFAULT 'running',
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    duration_ms INT COMMENT '执行耗时(毫秒)',
    result TEXT COMMENT '执行结果',
    error_msg TEXT COMMENT '错误信息',
    INDEX idx_task_id (task_id),
    INDEX idx_start_time (start_time),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务执行日志';

-- 插入默认任务
INSERT INTO scheduled_tasks (name, task_type, cron_expr, params, enabled) VALUES
('刷新数据池', 'refresh_data', '0 */10 * * * *', '{"pools": ["all"]}', 1),
('刷新模板缓存', 'refresh_template', '0 */30 * * * *', '{}', 1),
('清理过期缓存', 'clear_cache', '0 0 3 * * *', '{"max_age_hours": 24}', 1);
