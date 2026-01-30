-- SEO HTML Generator Database Schema
-- 站群分组架构版本 - 引入站群(site_groups)作为顶层管理单元

-- 设置客户端字符集（确保中文正确解析）
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

CREATE DATABASE IF NOT EXISTS seo_generator
    DEFAULT CHARACTER SET utf8mb4
    DEFAULT COLLATE utf8mb4_unicode_ci;

USE seo_generator;

-- ============================================
-- 站群表（顶层管理单元）
-- ============================================
CREATE TABLE IF NOT EXISTS site_groups (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE COMMENT '站群名称',
    description VARCHAR(500) DEFAULT NULL COMMENT '站群描述',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站群表';

-- ============================================
-- 站点表
-- ============================================
CREATE TABLE IF NOT EXISTS sites (
    id INT AUTO_INCREMENT PRIMARY KEY,
    site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID',
    domain VARCHAR(100) NOT NULL UNIQUE COMMENT '域名',
    name VARCHAR(100) NOT NULL COMMENT '站点名称',
    template VARCHAR(50) DEFAULT 'download_site' COMMENT '模板名',
    keyword_group_id INT DEFAULT NULL COMMENT '绑定的关键词分组ID',
    image_group_id INT DEFAULT NULL COMMENT '绑定的图片分组ID',
    article_group_id INT DEFAULT NULL COMMENT '绑定的文章分组ID',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    icp_number VARCHAR(50) DEFAULT NULL COMMENT 'ICP备案号',
    baidu_token VARCHAR(100) DEFAULT NULL COMMENT '百度推送Token',
    analytics TEXT DEFAULT NULL COMMENT '统计代码',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_site_group (site_group_id),
    INDEX idx_status (status),
    INDEX idx_keyword_group (keyword_group_id),
    INDEX idx_image_group (image_group_id),
    INDEX idx_article_group (article_group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站点表';

-- ============================================
-- 关键词分组表
-- ============================================
CREATE TABLE IF NOT EXISTS keyword_groups (
    id INT AUTO_INCREMENT PRIMARY KEY,
    site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID',
    name VARCHAR(100) NOT NULL COMMENT '分组名称',
    description VARCHAR(255) DEFAULT NULL COMMENT '描述',
    is_default TINYINT DEFAULT 0 COMMENT '是否默认分组',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_site_group (site_group_id),
    INDEX idx_default (is_default),
    UNIQUE INDEX idx_site_group_name (site_group_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关键词分组';

-- ============================================
-- 关键词表 (支持千万级)
-- ============================================
CREATE TABLE IF NOT EXISTS keywords (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL COMMENT '所属分组ID',
    keyword VARCHAR(500) NOT NULL COMMENT '关键词',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=有效, 0=无效',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group (group_id),
    INDEX idx_group_status (group_id, status),
    UNIQUE INDEX idx_group_kw (group_id, keyword(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='关键词表';

-- ============================================
-- 图片分组表
-- ============================================
CREATE TABLE IF NOT EXISTS image_groups (
    id INT AUTO_INCREMENT PRIMARY KEY,
    site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID',
    name VARCHAR(100) NOT NULL COMMENT '分组名称',
    description VARCHAR(255) DEFAULT NULL COMMENT '描述',
    is_default TINYINT DEFAULT 0 COMMENT '是否默认分组',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_site_group (site_group_id),
    INDEX idx_default (is_default),
    UNIQUE INDEX idx_site_group_name (site_group_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='图片分组';

-- ============================================
-- 图片表 (支持千万级)
-- ============================================
CREATE TABLE IF NOT EXISTS images (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL COMMENT '所属分组ID',
    url VARCHAR(1000) NOT NULL COMMENT '图片URL',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=有效, 0=无效',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group (group_id),
    INDEX idx_group_status (group_id, status),
    UNIQUE INDEX idx_group_url (group_id, url(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='图片表';

-- ============================================
-- 文章分组表
-- ============================================
CREATE TABLE IF NOT EXISTS article_groups (
    id INT AUTO_INCREMENT PRIMARY KEY,
    site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID',
    name VARCHAR(100) NOT NULL COMMENT '分组名称',
    description VARCHAR(255) DEFAULT NULL COMMENT '描述',
    is_default TINYINT DEFAULT 0 COMMENT '是否默认分组',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_site_group (site_group_id),
    INDEX idx_default (is_default),
    UNIQUE INDEX idx_site_group_name (site_group_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='文章分组';

-- ============================================
-- 原始文章表 (爬虫抓取 + 手工上传，支持亿级)
-- ============================================
CREATE TABLE IF NOT EXISTS original_articles (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL DEFAULT 1 COMMENT '所属分组ID',
    source_id INT NULL COMMENT '数据源ID，关联spider_projects表，手工上传为NULL',
    source_url VARCHAR(500) NULL COMMENT '来源URL，爬虫抓取的原始页面URL',
    title VARCHAR(500) NOT NULL COMMENT '标题',
    content MEDIUMTEXT NOT NULL COMMENT '正文',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=可用, 0=已删除',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_group (group_id),
    INDEX idx_group_status (group_id, status),
    INDEX idx_source_id (source_id),
    UNIQUE INDEX idx_group_title (group_id, title(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='原始文章表（爬虫抓取 + 手工上传）';

-- ============================================
-- 标题库表（数据生产项目写入，支持亿级）
-- ============================================
CREATE TABLE IF NOT EXISTS titles (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL DEFAULT 1 COMMENT '所属分组ID',
    title VARCHAR(500) NOT NULL COMMENT '标题文本',
    batch_id INT DEFAULT 0 COMMENT '批次号（用于优先最新）',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group_batch (group_id, batch_id),
    UNIQUE INDEX idx_group_title (group_id, title(191))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='标题库';

-- ============================================
-- 正文库表（数据生产项目写入，已处理好的完整正文，支持亿级）
-- ============================================
CREATE TABLE IF NOT EXISTS contents (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL DEFAULT 1 COMMENT '所属分组ID',
    content MEDIUMTEXT NOT NULL COMMENT '已生成的完整正文（含拼音标注）',
    batch_id INT DEFAULT 0 COMMENT '批次号（用于优先最新）',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group_batch (group_id, batch_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='正文库（已处理好的完整正文）';

-- ============================================
-- 模板表
-- ============================================
CREATE TABLE IF NOT EXISTS templates (
    id INT AUTO_INCREMENT PRIMARY KEY,
    site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID',
    name VARCHAR(100) NOT NULL COMMENT '模板标识名',
    display_name VARCHAR(100) NOT NULL COMMENT '显示名称',
    description VARCHAR(500) DEFAULT NULL COMMENT '模板描述',
    content MEDIUMTEXT NOT NULL COMMENT 'HTML模板内容',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    version INT DEFAULT 1 COMMENT '版本号（每次保存+1）',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_site_group (site_group_id),
    INDEX idx_status (status),
    UNIQUE INDEX idx_site_group_name (site_group_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模板表';

-- ============================================
-- 管理员表
-- ============================================
CREATE TABLE IF NOT EXISTS admins (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE COMMENT '用户名',
    password VARCHAR(255) NOT NULL COMMENT '密码哈希',
    last_login DATETIME DEFAULT NULL COMMENT '最后登录',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员表';

-- ============================================
-- 系统设置表
-- ============================================
CREATE TABLE IF NOT EXISTS system_settings (
    id INT AUTO_INCREMENT PRIMARY KEY,
    setting_key VARCHAR(100) NOT NULL UNIQUE COMMENT '设置键名',
    setting_value TEXT NOT NULL COMMENT '设置值',
    setting_type ENUM('string', 'number', 'boolean', 'json') DEFAULT 'string' COMMENT '值类型',
    description VARCHAR(255) DEFAULT NULL COMMENT '设置描述',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统设置表';

-- ============================================
-- 蜘蛛日志表
-- ============================================
CREATE TABLE IF NOT EXISTS spider_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    spider_type VARCHAR(20) NOT NULL COMMENT '蜘蛛类型',
    ip VARCHAR(45) NOT NULL COMMENT 'IP地址',
    ua VARCHAR(500) NOT NULL COMMENT 'User-Agent',
    domain VARCHAR(100) NOT NULL COMMENT '访问域名',
    path VARCHAR(500) NOT NULL COMMENT '访问路径',
    dns_ok TINYINT DEFAULT 0 COMMENT 'DNS验证通过',
    resp_time INT DEFAULT 0 COMMENT '响应时间(ms)',
    cache_hit TINYINT DEFAULT 0 COMMENT '缓存命中',
    status INT DEFAULT 200 COMMENT 'HTTP状态码',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_type (spider_type),
    INDEX idx_domain (domain),
    INDEX idx_time (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='蜘蛛日志';

-- ============================================
-- 初始数据
-- ============================================

-- 默认管理员 (密码: admin_6yh7uJ)
INSERT INTO admins (username, password) VALUES
('admin', '$2b$12$wsNjHaxHbiLgtCe5RxxxUOYSqLBA78ncnfJq/zZ1pWpwghnOYtS16')
ON DUPLICATE KEY UPDATE username = username;

-- 默认站群
INSERT INTO site_groups (id, name, description) VALUES
(1, '默认站群', '系统默认站群')
ON DUPLICATE KEY UPDATE name = name;

-- 默认关键词分组（属于默认站群）
INSERT INTO keyword_groups (site_group_id, name, description, is_default) VALUES
(1, '默认关键词分组', '系统默认关键词分组', 1)
ON DUPLICATE KEY UPDATE name = name;

-- 默认图片分组（属于默认站群）
INSERT INTO image_groups (site_group_id, name, description, is_default) VALUES
(1, '默认图片分组', '系统默认图片分组', 1)
ON DUPLICATE KEY UPDATE name = name;

-- 默认文章分组（属于默认站群）
INSERT INTO article_groups (site_group_id, name, description, is_default) VALUES
(1, '默认文章分组', '系统默认文章分组', 1)
ON DUPLICATE KEY UPDATE name = name;

-- 默认模板（属于默认站群）
-- 注意：模板内容从 database/templates/download_site.html 文件加载
INSERT INTO templates (site_group_id, name, display_name, description, content, status) VALUES
(1, 'download_site', '下载站模板', '适用于软件下载类站点的SEO模板', '', 1)
ON DUPLICATE KEY UPDATE name = name;

-- 示例站点（属于默认站群，绑定默认分组）
INSERT INTO sites (site_group_id, domain, name, template, keyword_group_id, image_group_id, article_group_id, icp_number) VALUES
(1, 'example.com', '示例站点', 'download_site', 1, 1, 1, '京ICP备xxxxxxxx号')
ON DUPLICATE KEY UPDATE domain = domain;

-- 默认系统设置（缓存配置）
INSERT INTO system_settings (setting_key, setting_value, setting_type, description) VALUES
('keyword_cache_ttl', '86400', 'number', '关键词缓存过期时间(秒)'),
('image_cache_ttl', '86400', 'number', '图片URL缓存过期时间(秒)'),
('cache_compress_enabled', 'true', 'boolean', '是否启用缓存压缩'),
('cache_compress_level', '6', 'number', '压缩级别(1-9)'),
('encoding_mix_ratio', '0.5', 'number', 'HTML实体编码混合比例(0-1)'),
('log_retention_days', '30', 'number', '日志保留天数'),
('keyword_pool_size', '500000', 'number', '关键词池大小(0=不限制)'),
('image_pool_size', '500000', 'number', '图片池大小(0=不限制)'),
('article_pool_size', '50000', 'number', '文章池大小(0=不限制)'),
-- 文件缓存配置
('file_cache_enabled', 'false', 'boolean', '是否启用文件缓存'),
('file_cache_dir', './html_cache', 'string', '文件缓存目录'),
('file_cache_max_size_gb', '50', 'number', '最大缓存大小(GB)'),
('file_cache_nginx_mode', 'true', 'boolean', 'Nginx直服模式(不压缩)')
ON DUPLICATE KEY UPDATE setting_key = setting_key;

-- ============================================
-- 正文生成器代码表（支持在线编辑）
-- ============================================
CREATE TABLE IF NOT EXISTS content_generators (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE COMMENT '生成器标识',
    display_name VARCHAR(100) NOT NULL COMMENT '显示名称',
    description TEXT COMMENT '生成器描述',
    code TEXT NOT NULL COMMENT 'Python代码',
    enabled TINYINT DEFAULT 1 COMMENT '是否启用',
    is_default TINYINT DEFAULT 0 COMMENT '是否默认',
    version INT DEFAULT 1 COMMENT '版本号',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_enabled (enabled),
    INDEX idx_default (is_default)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='正文生成器';

-- 默认正文生成器
INSERT INTO content_generators (name, display_name, description, code, is_default) VALUES
('default', '默认生成器', '将段落拼接并添加拼音标注', '
async def generate(ctx):
    """
    可用变量:
      ctx.paragraphs - 段落列表
      ctx.titles - 标题列表
    可用函数:
      annotate_pinyin(text) - 添加拼音标注
      random - Python random 模块
      re - Python re 模块
    """
    if len(ctx.paragraphs) < 3:
        return None

    # 随机选择3-5个段落
    count = min(len(ctx.paragraphs), random.randint(3, 5))
    selected = random.sample(ctx.paragraphs, count)

    # 拼接段落
    content = "\\n\\n".join(selected)

    # 添加拼音标注
    return annotate_pinyin(content)
', 1)
ON DUPLICATE KEY UPDATE name = name;

-- ============================================
-- 系统日志表（用于存储 ERROR 级别以上日志）
-- ============================================
CREATE TABLE IF NOT EXISTS system_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    level VARCHAR(10) NOT NULL COMMENT '日志级别 DEBUG/INFO/WARNING/ERROR',
    module VARCHAR(100) COMMENT '模块名称',
    spider_project_id INT COMMENT '关联的爬虫项目ID',
    message TEXT NOT NULL COMMENT '日志内容',
    extra JSON COMMENT '额外信息',
    created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3),

    INDEX idx_level (level),
    INDEX idx_module (module),
    INDEX idx_spider_project (spider_project_id),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统日志表';

-- ============================================
-- 爬虫项目表
-- ============================================
CREATE TABLE IF NOT EXISTS spider_projects (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '项目名称',
    description TEXT COMMENT '项目描述',

    -- 入口配置
    entry_file VARCHAR(100) DEFAULT 'spider.py' COMMENT '入口文件名',
    entry_function VARCHAR(50) DEFAULT 'main' COMMENT '入口函数名',
    start_url TEXT COMMENT '起始URL（可为空，由代码决定）',

    -- 运行配置
    config JSON COMMENT '配置参数 {proxy, timeout, headers, custom...}',
    concurrency INT NOT NULL DEFAULT 3 COMMENT '并发数量',

    -- 输出目标
    output_group_id INT DEFAULT 1 COMMENT '数据写入的文章分组ID',

    -- 调度配置
    schedule VARCHAR(50) DEFAULT NULL COMMENT 'Cron表达式',
    enabled TINYINT DEFAULT 1 COMMENT '是否启用',

    -- 运行状态
    status ENUM('idle', 'running', 'error') DEFAULT 'idle' COMMENT '运行状态',
    last_run_at DATETIME COMMENT '最后运行时间',
    last_run_duration INT COMMENT '最后运行耗时(秒)',
    last_run_items INT COMMENT '最后运行抓取数量',
    last_error TEXT COMMENT '最后错误信息',
    total_runs INT DEFAULT 0 COMMENT '累计运行次数',
    total_items INT DEFAULT 0 COMMENT '累计抓取数量',

    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    INDEX idx_status (status),
    INDEX idx_enabled (enabled),
    INDEX idx_output_group (output_group_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='爬虫项目表';

-- ============================================
-- 爬虫项目文件表（支持多文件项目）
-- ============================================
CREATE TABLE IF NOT EXISTS spider_project_files (
    id INT AUTO_INCREMENT PRIMARY KEY,
    project_id INT NOT NULL COMMENT '所属项目ID',
    filename VARCHAR(100) NOT NULL COMMENT '文件名（如 spider.py, utils.py）',
    content LONGTEXT NOT NULL COMMENT '文件内容',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uk_project_file (project_id, filename),
    INDEX idx_project_id (project_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='爬虫项目文件表';

-- ============================================
-- 失败请求表（队列模式使用）
-- ============================================
CREATE TABLE IF NOT EXISTS spider_failed_requests (
    id INT AUTO_INCREMENT PRIMARY KEY,
    project_id INT NOT NULL COMMENT '项目ID',
    url VARCHAR(2048) NOT NULL COMMENT '请求URL',
    method VARCHAR(10) DEFAULT 'GET' COMMENT 'HTTP方法',
    callback VARCHAR(100) COMMENT '回调函数名',
    meta JSON COMMENT '透传元数据',
    error_message TEXT COMMENT '错误信息',
    retry_count INT DEFAULT 0 COMMENT '已重试次数',
    failed_at DATETIME DEFAULT CURRENT_TIMESTAMP COMMENT '失败时间',
    status ENUM('pending', 'retried', 'ignored') DEFAULT 'pending' COMMENT '状态',
    INDEX idx_project_status (project_id, status),
    INDEX idx_project_failed_at (project_id, failed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='爬虫失败请求表';

-- ============================================
-- 统计历史表（用于图表展示）
-- ============================================
CREATE TABLE IF NOT EXISTS spider_stats_history (
    id INT AUTO_INCREMENT PRIMARY KEY,
    project_id INT NOT NULL COMMENT '项目ID',
    period_type ENUM('minute', 'hour', 'day', 'month') NOT NULL COMMENT '周期类型',
    period_start DATETIME NOT NULL COMMENT '周期开始时间',
    total INT DEFAULT 0 COMMENT '总请求数',
    completed INT DEFAULT 0 COMMENT '成功数',
    failed INT DEFAULT 0 COMMENT '失败数',
    retried INT DEFAULT 0 COMMENT '重试次数',
    avg_speed DECIMAL(10,2) COMMENT '平均速度（条/分钟）',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_project_period (project_id, period_type, period_start),
    INDEX idx_query (project_id, period_type, period_start DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='爬虫统计历史表';

-- ============================================
-- 数据库迁移脚本（从旧版本升级）
-- 如果是从旧版本升级，请执行以下SQL：
-- ============================================
/*
-- 1. 创建站群表
CREATE TABLE IF NOT EXISTS site_groups (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE COMMENT '站群名称',
    description VARCHAR(500) DEFAULT NULL COMMENT '站群描述',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='站群表';

-- 2. 插入默认站群
INSERT INTO site_groups (id, name, description) VALUES (1, '默认站群', '系统默认站群');

-- 3. 各表添加site_group_id字段
ALTER TABLE keyword_groups
ADD COLUMN site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID' AFTER id,
ADD INDEX idx_site_group (site_group_id),
DROP INDEX name,
ADD UNIQUE INDEX idx_site_group_name (site_group_id, name);

ALTER TABLE image_groups
ADD COLUMN site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID' AFTER id,
ADD INDEX idx_site_group (site_group_id),
DROP INDEX name,
ADD UNIQUE INDEX idx_site_group_name (site_group_id, name);

ALTER TABLE article_groups
ADD COLUMN site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID' AFTER id,
ADD INDEX idx_site_group (site_group_id),
DROP INDEX name,
ADD UNIQUE INDEX idx_site_group_name (site_group_id, name);

ALTER TABLE templates
ADD COLUMN site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID' AFTER id,
ADD INDEX idx_site_group (site_group_id),
DROP INDEX name,
ADD UNIQUE INDEX idx_site_group_name (site_group_id, name);

ALTER TABLE sites
ADD COLUMN site_group_id INT NOT NULL DEFAULT 1 COMMENT '所属站群ID' AFTER id,
ADD COLUMN article_group_id INT DEFAULT NULL COMMENT '绑定的文章分组ID' AFTER image_group_id,
ADD INDEX idx_site_group (site_group_id),
ADD INDEX idx_article_group (article_group_id);

-- 4. articles表添加group_id字段
ALTER TABLE articles
ADD COLUMN group_id INT NOT NULL DEFAULT 1 COMMENT '所属分组ID' AFTER id,
ADD INDEX idx_group (group_id),
ADD INDEX idx_group_status (group_id, status);

-- 5. 更新现有数据绑定到默认站群和默认分组
UPDATE sites SET article_group_id = (SELECT id FROM article_groups WHERE is_default = 1 LIMIT 1) WHERE article_group_id IS NULL;

-- 6. articles表添加唯一索引（防止同分组内文章标题重复）
-- 注意：如果有重复数据需要先处理
ALTER TABLE articles ADD UNIQUE INDEX idx_group_title (group_id, title(255));
*/
