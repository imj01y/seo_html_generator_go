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
    is_default TINYINT DEFAULT 0 COMMENT '是否默认站群',
    status TINYINT DEFAULT 1 COMMENT '状态: 1=启用, 0=禁用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_status (status),
    INDEX idx_default (is_default)
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
    status TINYINT DEFAULT 1 COMMENT '状态: 1=可用, 0=已使用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group_batch (group_id, batch_id),
    INDEX idx_group_status (group_id, status),
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
    status TINYINT DEFAULT 1 COMMENT '状态: 1=可用, 0=已使用',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group_batch (group_id, batch_id),
    INDEX idx_group_status (group_id, status)
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
-- 缓存池配置表
-- ============================================
CREATE TABLE IF NOT EXISTS pool_config (
    id INT AUTO_INCREMENT PRIMARY KEY,
    -- 标题池配置
    title_pool_size INT NOT NULL DEFAULT 100000 COMMENT '标题池大小',
    title_workers INT NOT NULL DEFAULT 4 COMMENT '标题池工作线程数',
    title_refill_interval_ms INT NOT NULL DEFAULT 200 COMMENT '标题池补充间隔(毫秒)',
    title_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.30 COMMENT '标题池补充阈值(0-1)',
    -- 正文池配置
    content_pool_size INT NOT NULL DEFAULT 500000 COMMENT '正文池大小',
    content_workers INT NOT NULL DEFAULT 10 COMMENT '正文池工作线程数',
    content_refill_interval_ms INT NOT NULL DEFAULT 50 COMMENT '正文池补充间隔(毫秒)',
    content_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.40 COMMENT '正文池补充阈值(0-1)',
    -- cls类名池配置
    cls_pool_size INT NOT NULL DEFAULT 100000 COMMENT 'cls池大小',
    cls_workers INT NOT NULL DEFAULT 4 COMMENT 'cls池工作线程数',
    cls_refill_interval_ms INT NOT NULL DEFAULT 200 COMMENT 'cls池补充间隔(毫秒)',
    cls_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.30 COMMENT 'cls池补充阈值(0-1)',
    -- url池配置
    url_pool_size INT NOT NULL DEFAULT 100000 COMMENT 'url池大小',
    url_workers INT NOT NULL DEFAULT 4 COMMENT 'url池工作线程数',
    url_refill_interval_ms INT NOT NULL DEFAULT 200 COMMENT 'url池补充间隔(毫秒)',
    url_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.30 COMMENT 'url池补充阈值(0-1)',
    -- 关键词表情池配置
    keyword_emoji_pool_size INT NOT NULL DEFAULT 50000 COMMENT '关键词表情池大小',
    keyword_emoji_workers INT NOT NULL DEFAULT 2 COMMENT '关键词表情池工作线程数',
    keyword_emoji_refill_interval_ms INT NOT NULL DEFAULT 200 COMMENT '关键词表情池补充间隔(毫秒)',
    keyword_emoji_threshold DECIMAL(3,2) NOT NULL DEFAULT 0.30 COMMENT '关键词表情池补充阈值(0-1)',
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='缓存池配置表';

-- 默认缓存池配置
INSERT INTO pool_config (id, title_pool_size, title_workers, title_refill_interval_ms, title_threshold, content_pool_size, content_workers, content_refill_interval_ms, content_threshold, cls_pool_size, cls_workers, cls_refill_interval_ms, cls_threshold, url_pool_size, url_workers, url_refill_interval_ms, url_threshold, keyword_emoji_pool_size, keyword_emoji_workers, keyword_emoji_refill_interval_ms, keyword_emoji_threshold) VALUES
(1, 100000, 4, 200, 0.30, 500000, 10, 50, 0.40, 100000, 4, 200, 0.30, 100000, 4, 200, 0.30, 50000, 2, 200, 0.30)
ON DUPLICATE KEY UPDATE id = id;

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
    INDEX idx_time (created_at),
    INDEX idx_type_domain_time (spider_type, domain, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='蜘蛛日志';

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

-- ============================================
-- 初始数据
-- ============================================

-- 默认管理员 (密码: admin_6yh7uJ)
INSERT INTO admins (username, password) VALUES
('admin', '$2b$12$wsNjHaxHbiLgtCe5RxxxUOYSqLBA78ncnfJq/zZ1pWpwghnOYtS16')
ON DUPLICATE KEY UPDATE username = username;

-- 默认站群
INSERT INTO site_groups (id, name, description, is_default) VALUES
(1, '默认站群', '系统默认站群', 1)
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
INSERT INTO templates (site_group_id, name, display_name, description, content, status) VALUES
(1, 'download_site', '下载站模板', '适用于软件下载类站点的SEO模板',
'<!DOCTYPE html>
<html lang="zh-CN">

<head>
    <!-- ========== 1. 百度适配Meta标签 ========== -->
    <meta name="applicable-device" content="pc" />
    <meta http-equiv="Cache-Control" content="no-transform" />
    <meta http-equiv="Cache-Control" content="no-siteapp" />
    <meta http-equiv="Content-Type" content="text/html; charset=utf-8" />

    <!-- ========== 2. Title (关键词+Emoji混合编码) ========== -->
    <title>{{ title }}</title>

    <!-- ========== 3. Mobile Agent适配 ========== -->
    <meta http-equiv="mobile-agent" content="format=xhtml; url={{ random_url() }}" />
    <meta http-equiv="mobile-agent" content="format=html5; url={{ random_url() }}" />

    <!-- ========== 4. SEO基础标签（留空） ========== -->
    <meta name="keywords" content="" />
    <meta name="description" content="" />

    <!-- ========== 5. CSS引用 ========== -->
    <link href="http://www.imansoft.cn/static/css/3673.css" type="text/css" rel="stylesheet" />

    <!-- ========== 6. 页面配置JS变量 ========== -->
    <script>
    var _pageinfo = {
        id: "{{ random_url() }}",
        path: "down",
        categroyId: ''{{ random_number(100,999) }}'',
        rootId: ''{{ random_number(1,9) }}'',
        commendid: ''0'',
        catalogname: ''{{ random_keyword() }}'',
        softname: ''{{ random_keyword() }}'',
        softver: ''{{ random_keyword() }}'',
        system: ''{{ random_keyword() }}'',
        softlicence: "{{ random_keyword() }}",
        body: "1"
    }
    </script>
    <script type="text/javascript" src="/static/js/jquerymin.js"></script>
    <script type="text/javascript" src="/static/js/maininfo.js"></script>

    <!-- ========== 7. OG协议标签 ========== -->
    <meta property="og:type" content="soft" />
    <meta property="og:description" content="" />
    <meta property="og:image" content="{{ random_image() }}" />
    <meta property="og:title" content="{{ random_keyword() }}" />

    <!-- ========== 8. OG软件扩展标签 (og:soft:*) ========== -->
    <meta property="og:soft:operating_system" content="{{ random_keyword() }}" />
    <meta property="og:soft:language" content="{{ random_keyword() }}" />
    <meta property="og:soft:license" content="{{ random_keyword() }}" />
    <meta property="og:soft:url" content="{{ random_url() }}" />
    <meta property="og:release_date" content="{{ now() }}" />

    <!-- ========== 9. 字节跳动时间戳标签 ========== -->
    <meta property="bytedance:published_time" content="{{ now() }}" />
    <meta property="bytedance:lrDate_time" content="{{ now() }}" />
    <meta property="bytedance:updated_time" content="{{ now() }}" />

    <!-- ========== 10. 统计代码 ========== -->
    {{ analytics_code or '''' }}
</head>

<body id="downpage">
    <!-- ========== 区域1: 顶部导航条 (topper) ========== -->
    <div class="{{ cls(''topper'') }}">
        <div class="{{ cls(''box'') }}">
            <span class="{{ cls(''fl'') }}">{{ random_keyword() }}</span>
            {% for i in range(4) %}
            <a href="{{ random_url() }}">{{ random_keyword() }}</a>|
            {% endfor %}
        </div>
    </div>

    <!-- ========== 区域2: 头部 (header) ========== -->
    <div class="{{ cls(''header'') }}">
        <div class="{{ cls(''box'') }}">
            <!-- Logo -->
            <div class="{{ cls(''logo'') }}">
                <a href="{{ random_url() }}"><b>{{ site_id }}</b>.com</a>
            </div>

            <!-- 导航菜单 -->
            <div class="{{ cls(''menu'') }}">
                <a href="{{ random_url() }}" class="{{ cls(''menuGame'') }}"><i></i>网络游戏</a>
                <a href="{{ random_url() }}" class="{{ cls(''menuGame'') }}"><i></i>单机游戏</a>
                <a href="{{ random_url() }}" class="{{ cls(''menuApp'') }}"><i></i>手机应用</a>
                <a href="{{ random_url() }}" class="{{ cls(''menuTopic'') }}"><i></i>电脑软件</a>
                <a href="{{ random_url() }}" class="{{ cls(''menuNew'') }}"><i></i>专题</a>
                <a href="{{ random_url() }}" class="{{ cls(''menuGame'') }}"><i></i>热门排行榜</a>
            </div>

            <!-- 搜索框 -->
            <div class="{{ cls(''searBox'') }}">
                <div class="{{ cls(''search'') }}">
                    <input type="text" class="{{ cls(''txt_search'') }}" id="words" onkeypress="if(event.keyCode==13) {search_submit(''words'');return false;}" value="" />
                    <input type="button" class="{{ cls(''btn_search'') }}" onclick="search_submit(''words'')" />
                </div>
                <!-- 热搜词 -->
                <div class="{{ cls(''hot_word'') }}">
                    {% for i in range(9) %}
                    <a href="{{ random_url() }}" class="{{ cls(''bdcs-hot-item'') }}" target="_blank">{{ random_keyword() }}</a>
                    {% endfor %}
                </div>
            </div>
        </div>
    </div>

    <!-- ========== 区域3: 面包屑导航 (breadcrumb) ========== -->
    <div class="{{ cls(''box'') }}">
        <p class="{{ cls(''pos'') }}">当前位置：
            <a href="{{ random_url() }}">{{ random_keyword_emoji() }}</a>→
            <a href="{{ random_url() }}">{{ random_keyword() }}</a>→
            <a href="{{ random_url() }}">{{ random_keyword() }}</a>
            {{ random_keyword_emoji() }} 安卓版
        </p>
    </div>

    <!-- ========== 区域4: 主内容区域 ========== -->
    <div class="{{ cls(''box'') }}">
        <!-- ===== 4.1 左侧信息栏 (infoSub) ===== -->
        <div class="{{ cls(''infoSub'') }}">
            <!-- 应用图标和H1标题 -->
            <div class="{{ cls(''subAppicon'') }}">
                <img src="{{ random_image() }}" alt="{{ title }}" />
                <h1>{{ title }}</h1>

                <!-- 下载按钮 -->
                <ul id="dbtns">
                    <li class="{{ cls(''az'') }}" id="azbtn">
                        <a href="{{ random_url() }}" rel="nofollow" class="{{ cls(''down_counter'') }}" id="{{ random_url() }}" target="_blank">安卓版下载</a>
                    </li>
                    <li id="pgbtn" class="{{ cls(''appstore'') }}">
                        <a target="_blank" id="{{ random_url() }}" href="{{ random_url() }}">电脑版下载</a>
                    </li>
                </ul>
            </div>

            <!-- 点赞/踩按钮 -->
            <ul class="{{ cls(''aztop'') }}">
                <li id="showding" onclick="javascript:sEval({{ random_url() }},1,''showding'',''showcai'',0)">
                    <em class="{{ cls(''showDinNum'') }}">{{ random_number(1, 20) }}</em>
                </li>
                <li id="showcai" onclick="javascript:sEval({{ random_url() }},0,''showding'',''showcai'',0)">
                    <em class="{{ cls(''showDinNum'') }}">{{ random_number(1, 10) }}</em>
                </li>
            </ul>

            <!-- 相关推荐列表 -->
            <div class="{{ cls(''subList'') }}">
                <div class="{{ cls(''tit'') }}" id="anchorLike">相关推荐</div>
                <ul class="{{ cls(''zolSub'') }}">
                    {% for i in range(50) %}
                    <li>
                        <a href="{{ random_url() }}" title="{{ random_keyword() }}">
                            <img src="{{ random_image() }}" alt="{{ random_keyword() }}" />
                            <section>
                                <h3>{{ random_keyword() }}</h3>
                                <span class="{{ cls(''star'') }}" title="热度评级：6/10">
                                    <span style="width:calc(40%*2)"></span>
                                </span>
                                <p>v{{ random_number(1,9) }}.{{ random_number(0,9) }}.{{ random_number(0,99) }}</p>
                            </section>
                        </a>
                    </li>
                    {% endfor %}
                </ul>
            </div>
        </div>

        <!-- ===== 4.1.5 右侧边栏 (infoRight) ===== -->
        <div class="{{ cls(''infoRight'') }}">
            <dl class="{{ cls(''kbox'') }}">
                <dt>相关游戏</dt>
                <dd>
                    {% for i in range(18) %}
                    <a href="{{ random_url() }}">
                        <img alt="{{ random_keyword() }}" src="{{ random_image() }}" />
                        <i>{{ random_keyword() }}</i>
                    </a>
                    {% endfor %}
                </dd>
            </dl>

            <!-- 热门排行 (移动到infoRight内) -->
            <div class="{{ cls(''tit'') }}">热门冒险解谜</div>
            <ul class="{{ cls(''topList'') }}">
                {% for i in range(700) %}
                <li>
                    <i>{{ now() }}</i>
                    <b class=''on''>{{ random_number(100, 999) }}</b>
                    <a href="{{ random_url() }}" target="_blank">
                        <img src="{{ random_image() }}" alt="{{ random_keyword() }}" />
                    </a>
                    <p><a href="{{ random_url() }}" target="_blank">{{ random_keyword() }}</a></p>
                </li>
                {% endfor %}
            </ul>
        </div>

        <!-- ===== 4.2 主信息区域 (infoMain) ===== -->
        <div class="{{ cls(''infoMain'') }}">
            <!-- 软件信息表格 -->
            <table cellpadding="0" cellspacing="0">
                <tr>
                    <td><i>分类：</i>{{ random_keyword() }} / {{ random_keyword() }}</td>
                    <td><i>大小：</i></td>
                    <td><i>授权：</i>{{ random_keyword() }}</td>
                </tr>
                <tr>
                    <td><i>语言：</i>{{ random_keyword() }}</td>
                    <td><i>更新：</i>{{ now() }}</td>
                    <td><i>等级：</i>
                        <span class="{{ cls(''star'') }}">
                            <span style="width:calc(40%*2)"></span>
                        </span>
                    </td>
                </tr>
                <tr>
                    <td><i>平台：</i>{{ random_keyword() }}</td>
                    <td><i>厂商：</i>
                        <a href="{{ random_url() }}" target="_blank">{{ random_keyword() }}</a>
                    </td>
                    <td class="{{ cls(''siteurl'') }}"><i>官网：</i>暂无</td>
                </tr>
                <tr>
                    <td class="{{ cls(''qx'') }}"><i>权限：</i><b>查看</b>
                        <div class="{{ cls(''qxstr'') }}">允许程序访问网络.</div>
                    </td>
                    <td class="{{ cls(''beian'') }}"><i>备案：</i>湘ICP备2023018554号-3A</td>
                    <td></td>
                </tr>
                <tr>
                    <td colspan="3"><i>标签：</i>
                        <span class="{{ cls(''tipWord'') }}">
                            {% for i in range(3) %}
                            <a href="{{ random_url() }}" target="_blank">{{ random_keyword() }}</a>
                            {% endfor %}
                        </span>
                    </td>
                </tr>
            </table>

            <!-- Tab导航 -->
            <dl class="{{ cls(''queTit'') }}">
                <dd class="{{ cls(''on'') }}"><i class="{{ cls(''queTop'') }}"></i>详情</dd>
                <dd><i class="{{ cls(''queInfo'') }}"></i>介绍</dd>
                <dd><i class="{{ cls(''queLike'') }}"></i>猜你喜欢</dd>
                <dd><i class="{{ cls(''queHistory'') }}"></i>相关版本</dd>
            </dl>

            <!-- 截图区域 -->
            <div class="{{ cls(''appWrap'') }}" id="div_screenshots">
                <h3><i class="{{ cls(''iInfo'') }}"></i>截图</h3>
                <div class="{{ cls(''infopic'') }}" id="div_screenshots2">
                    <div class="{{ cls(''picbox'') }}">
                        <ul class="{{ cls(''piclist'') }}">
                            {% for i in range(4) %}
                            <li>
                                <a href="{{ random_image() }}" data-lightbox="s1" data-text="{{ random_keyword() }}">
                                    <img src="{{ random_image() }}" alt="{{ random_keyword() }}" />
                                </a>
                            </li>
                            {% endfor %}
                        </ul>
                    </div>
                    <div class="{{ cls(''gn_prev'') }}"></div>
                    <div class="{{ cls(''gn_next'') }}"></div>
                </div>
            </div>

            <!-- 内容详情区域 -->
            <div class="{{ cls(''appWrap'') }}">
                <h3 id="anchorInfo"><i class="{{ cls(''iInfo'') }}"></i>内容详情</h3>
                <div class="{{ cls(''txtIntro'') }}">
                    <!-- 热点新闻标题 -->
                    <p id="news_1">{{ random_keyword() }}</p>
                    <p id="news_2">{{ random_keyword() }}</p>
                    <p id="news_3">{{ random_keyword() }}</p>

                    <!-- 厂商新闻 -->
                    <p style="text-indent:2em;">
                        <i>厂商新闻</i>{{ random_keyword() }} 时间：{{ now() }}
                    </p>

                    <!-- 文章正文（含拼音标注） -->
                    <ul class="{{ cls(''intem'') }}">
                        <ul class="{{ cls(''intem'') }}">
                            <li>编辑：admin</li>
                        </ul>
                        <p align="center">{{ now() }}
                        <div class="{{ cls(''left_zw'') }}">
                            <!-- 拼音标注段落 -->
                            <p>　{{ content() }}</p>

                            <table border="0" cellspacing="0" cellpadding="0" align="left" class="{{ cls(''adInContent'') }}">
                                <tr>
                                    <td>
                                        <!--画中画广告start-->
                                        <!--画中画广告end-->
                                    </td>
                                </tr>
                            </table>

                            <!-- 编辑署名 -->
                            <div class="{{ cls(''adEditor'') }}">
                                <div class="{{ cls(''left_name'') }} right">
                                    <span>编辑：站点编辑</span>
                                </div>
                            </div>
                            <div id="function_code_page"></div>
                        </div>
                        <img src="{{ random_image() }}" alt="" />
                        </p>
                    </ul>
                </div>
            </div>

            <div class="{{ cls(''appWrap'') }}"></div>

            <!-- 厂商其他下载 -->
            <div class="{{ cls(''company'') }}" id="company">
                <p class="{{ cls(''introTit'') }}">厂商其他下载</p>
                <p class="{{ cls(''sys'') }}">
                    <span>安卓应用</span>
                    <span>安卓手游</span>
                    <span>苹果应用</span>
                    <span>苹果手游</span>
                    <span>电脑</span>
                    <a href="{{ random_url() }}" target="_blank">更多+</a>
                </p>
                <ul class="{{ cls(''clearfix'') }}"></ul>
                <ul class="{{ cls(''clearfix'') }}">
                    {% for i in range(350) %}
                    <li>
                        <a href="{{ random_url() }}" target="_blank">
                            <img alt="{{ random_keyword() }}" src="{{ random_image() }}" />
                            {{ random_keyword() }}
                        </a>
                    </li>
                    {% endfor %}
                </ul>
            </div>
        </div>

        <!-- ========== 区域4.5: AppBox 应用详情盒子 ========== -->
        <div class="{{ cls(''AppBox'') }}">
            <!-- App-details: 应用图标和标题 -->
            <div class="{{ cls(''App-details'') }}">
                <img class="{{ cls(''icon'') }}" height="51px" width="51px" alt="{{ random_keyword() }}" src="{{ random_image() }}" />
                <div class="{{ cls(''App-details-det'') }}">
                    <div class="{{ cls(''app-details-appname'') }}">
                        <h2>{{ random_keyword() }}</h2>
                    </div>
                    <div class="{{ cls(''app-details-appdes'') }}">
                        <h3>「活动」首次登录送{{ random_number(50, 200) }}元红包</h3>
                    </div>
                    <div class="{{ cls(''app-details-appinfo'') }}">
                        <div>{{ random_keyword() }}</div>
                        <div>{{ random_keyword() }}</div>
                    </div>
                </div>
            </div>

            <!-- App-display-a: 下载按钮 -->
            <div class="{{ cls(''App-display-a'') }}">
                <div class="{{ cls(''download_btn_col'') }}">
                    <a class="{{ cls(''normal-down-btn'') }}" href="{{ random_url() }}">下载APK</a>
                    <a class="{{ cls(''high-speed-down-btn'') }}" href="{{ random_url() }}">高速下载</a>
                </div>
                <div class="{{ cls(''b-text'') }}">
                    <span class="{{ cls(''span'') }}">下载安装你想要的应用 更方便 更快捷 发现更多</span>
                </div>
            </div>

            <!-- App_Hots: 点赞和评论数 -->
            <div class="{{ cls(''App_Hots'') }}">
                <div>
                    <div class="{{ cls(''fr'') }}">
                        <img alt="喜欢" src="/static/images/icon_03.png" />
                        <span>{{ random_number(30, 90) }}%好评({{ random_number(10, 100) }}人)</span>
                    </div>
                </div>
                <div>
                    <div class="{{ cls(''fl'') }}">
                        <img alt="评论" src="/static/images/icon_04.png" />
                        <span>{{ random_number(10, 200) }}</span>
                    </div>
                </div>
            </div>

            <!-- App_ImgBox: 截图图片行 -->
            <div class="{{ cls(''img'') }}">
                <div class="{{ cls(''App_ImgBox'') }}">
                    {% for i in range(5) %}
                    <img alt="{{ random_keyword() }}截图{{ i }}" src="{{ random_image() }}" />
                    {% endfor %}
                </div>
            </div>

            <!-- App_Category: 详细信息区域 -->
            <div class="{{ cls(''App_Category'') }}">
                <!-- 详细信息 -->
                <div class="{{ cls(''App_SpreadBox'') }}">
                    <div class="{{ cls(''App_SpreadTitle'') }}">
                        <span>详细信息</span>
                        <div class="{{ cls(''icon'') }}"></div>
                    </div>
                    <ul class="{{ cls(''App_SpreadContent'') }}">
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">软件大小:</span>
                            <span class="{{ cls(''fl'') }}">{{ random_number(10, 200) }}.{{ random_number(10, 99) }}MB</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">最后更新:</span>
                            <span class="{{ cls(''fl'') }}">{{ now() }}</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">最新版本:</span>
                            <span class="{{ cls(''fl'') }}">V{{ random_number(1, 20) }}.{{ random_number(0, 99) }}.{{ random_number(0, 99) }}</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">文件格式:</span>
                            <span class="{{ cls(''fl'') }}">apk</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">应用分类:</span>
                            <span class="{{ cls(''fl'') }}">ios-Android</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">使用语言:</span>
                            <span class="{{ cls(''fl'') }}">中文</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">:</span>
                            <span class="{{ cls(''fl'') }}">需要联网</span>
                        </li>
                        <li class="{{ cls(''3ce690'') }}">
                            <span class="{{ cls(''fl'') }}">系统要求:</span>
                            <span class="{{ cls(''fl'') }}">{{ random_number(5, 10) }}.{{ random_number(0, 9) }}以上</span>
                        </li>
                    </ul>
                </div>

                <!-- 简介 -->
                <div class="{{ cls(''App_SpreadBox'') }}">
                    <div class="{{ cls(''App_SpreadTitle'') }}">
                        <span>简介</span>
                        <div class="{{ cls(''icon'') }}"></div>
                    </div>
                    <div class="{{ cls(''App_Introduce'') }}">
                        <span><a href=''{{ random_url() }}''>{{ random_keyword() }}</a></span>
                    </div>
                </div>

                <!-- 更新日志 -->
                <div class="{{ cls(''App_SpreadBox'') }}">
                    <div class="{{ cls(''App_SpreadTitle'') }}">
                        <span>更新</span>
                        <div class="{{ cls(''icon'') }}"></div>
                    </div>
                    <div class="{{ cls(''App_Update'') }}">
                        <span>V{{ random_number(10, 20) }}.{{ random_number(0, 99) }}.{{ random_number(0, 9) }}</span>
                        <div class="{{ cls(''get_more'') }}">更多历史版本</div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <!-- ========== 区域5.5: 相关攻略 (StrategyList) ========== -->
    <div class="{{ cls(''App_SpreadBox'') }}">
        <div class="{{ cls(''App_SpreadTitle'') }}">
            <span>相关攻略</span>
            <div class="{{ cls(''icon'') }}"></div>
        </div>
        <div class="{{ cls(''App_SpreadContent'') }}">
            <ul>
                {% for i in range(22) %}
                <li class="{{ cls(''StrategyList_Content'') }}">
                    <div class="{{ cls(''StrategyList_ContentTitle ml0'') }}">
                        <a href="{{ random_url() }}">
                            <div>{{ random_keyword() }}</div>
                        </a>
                    </div>
                    <div class="{{ cls(''StrategyList_TimeBox'') }}">
                        <span style="line-height: 40px;">{{ random_number(1, 9) }}</span>
                    </div>
                </li>
                {% endfor %}
            </ul>
        </div>
    </div>

    <!-- ========== 区域5.6: 评论区域 ========== -->
    <div class="{{ cls(''App_SpreadBox'') }}">
        <div class="{{ cls(''App_SpreadTitle'') }}">
            <span>评论</span>
            <div class="{{ cls(''icon'') }}"></div>
        </div>
        <div class="{{ cls(''App-details-comment'') }}">
            <ul>
                {% for i in range(10) %}
                <li class="{{ cls(''3ce690'') }}">
                    <table>
                        <tr>
                            <td style="width:230px;">{{ random_keyword() }}</td>
                            <td>{{ random_number(1, 59) }}分钟前</td>
                        </tr>
                        <tr>
                            <td colspan="2" style="color:#44423C; text-align:justify; text-justify:distribute-all-lines;word-wrap:break-word;word-break:break-all;">{{ random_keyword() }}</td>
                        </tr>
                    </table>
                </li>
                {% endfor %}
            </ul>
            <a href="{{ random_url() }}">
                <div class="{{ cls(''link_more'') }}">{{ random_keyword() }}</div>
            </a>
        </div>
    </div>

    <!-- 隐藏输入字段 -->
    <input data-page-role="{{ cls('''') }}" id="SpreadBox_Open" type="hidden" value="{{ random_image() }}" />
    <input data-aria-hidden="{{ cls('''') }}" id="SpreadBox_Close" type="hidden" value="{{ random_image() }}" />

    <!-- ========== 区域6: Footer (页脚) ========== -->
    <div class="{{ cls(''footer'') }}">
        <div class="{{ cls(''wrap'') }}">
            <div class="{{ cls(''bottomText'') }}">
                <a href="{{ random_url() }}" rel="nofollow">关于我们</a>|
                <a href="{{ random_url() }}" rel="nofollow">意见反馈</a>|
                <a href="{{ random_url() }}" rel="nofollow">版权声明</a>|
                <a href="{{ random_url() }}" rel="nofollow">合作伙伴</a>|
                <a href="{{ random_url() }}" rel="nofollow">友情连接</a>|
                <a href="{{ random_url() }}" rel="nofollow">联系我们</a>|
                <a href="{{ random_url() }}">网站地图</a>
            </div>
            <span>copyright 2022-2024 3673安卓网.All Right Reserved</span>
        </div>
    </div>

    <!-- ========== 区域7: 底部脚本 ========== -->
    <script>
    var detail = {
        ''sid'': {{ random_number(10000, 99999) }},
        ''sname'': "{{ random_keyword() }}",
        ''cname'': "冒险解谜",
        ''crid'': {{ random_number(1, 10) }},
        ''cid'': {{ random_number(10, 99) }}
    };
    var _webInfo = {};
    _webInfo = {
        Username: "{{ random_keyword() }}",
        Type: "0",
        DateTime: "{{ now() }}",
        Id: "{{ random_number(10000, 99999) }}"
    };
    </script>

    <!-- 二维码 -->
    <div id="qrcode1" style="display:none;" title="{{ random_url() }}">
        <canvas width="110" height="110" style="display: none;"></canvas>
        <img src="{{ random_image() }}" style="display: block;" />
    </div>

    <!-- ========== 用户反馈表单 (feedBack) ========== -->
    <div class="{{ cls(''feedBack'') }}">
        <div class="{{ cls(''feBaBox'') }}">
            <div class="{{ cls(''feBaClose'') }}"><i class="{{ cls(''ico'') }}"></i></div>
            <div class="{{ cls(''feHead'') }}">用户反馈</div>
            <div class="{{ cls(''feBack'') }}">
                <p>反馈原因</p>
                <div class="{{ cls(''info'') }}">
                    <div class="{{ cls(''checkbox'') }}">
                        <input type="checkbox" id="checkbox1" name="fklb" value="有色情、暴力、反动等不良内容" />
                        <label for="checkbox1">有色情、暴力、反动等不良内容</label>
                    </div>
                    <div class="{{ cls(''checkbox'') }}">
                        <input type="checkbox" id="checkbox2" name="fklb" value="有抄袭、侵权嫌疑" />
                        <label for="checkbox2">有抄袭、侵权嫌疑</label>
                    </div>
                    <div class="{{ cls(''checkbox'') }}">
                        <input type="checkbox" id="checkbox3" name="fklb" value="广告很多、含有不良插件" />
                        <label for="checkbox3">广告很多、含有不良插件</label>
                    </div>
                    <div class="{{ cls(''checkbox'') }}">
                        <input type="checkbox" id="checkbox4" name="fklb" value="无法正常安装或进入游戏" />
                        <label for="checkbox4">无法正常安装或进入游戏</label>
                    </div>
                </div>
                <p>其他原因</p>
                <textarea name="remake" placeholder="请输入补充说明"></textarea>
            </div>
            <div class="{{ cls(''h20'') }}"></div>
            <div class="{{ cls(''telBox'') }}">
                <span>联系方式</span>
                <input type="tel" name="tel" placeholder="请输入邮箱" />
            </div>
            <div class="{{ cls(''feSubmit'') }}">
                <input type="button" class="{{ cls(''submit'') }}" name="submit" value="提交反馈" />
            </div>
        </div>
    </div>

    <!-- 脚本引用 -->
    <script type="text/javascript" src="/static/js/comment.js"></script>
    <script type="text/javascript" src="/static/js/footer.js"></script>

    <!-- 结构化数据 -->
    <script type="application/ld+json">{
"@context": "http://zhanzhang.baidu.com/contexts/cambrian.jsonld",
"@id": "{{ random_url() }}",
"appid": "否",
"title": "{{ random_keyword() }}",
"images": ["{{ random_image() }}"],
"description": "{{ random_keyword() }}",
"upDate": "{{ now() }}",
"data": {
"WebPage": {
"pcUrl": "{{ random_url() }}",
"wapUrl": "{{ random_url() }}",
"fromSrc": "3676安卓网"
}
}
}</script>

    <!-- ========== 图片灯箱 (lightbox) ========== -->
    <div id="lightboxOverlay" class="{{ cls(''lightboxOverlay'') }}" style="display: none;"></div>
    <div id="lightbox" class="{{ cls(''lightbox'') }}" style="display: none;">
        <div class="{{ cls(''lb-outerContainer'') }}">
            <div class="{{ cls(''lb-container'') }}">
                <img class="{{ cls(''lb-image'') }}" src="{{ random_image() }}" />
                <div class="{{ cls(''lb-nav'') }}">
                    <a class="{{ cls(''lb-prev'') }}" href="{{ random_url() }}"></a>
                    <a class="{{ cls(''lb-next'') }}" href="{{ random_url() }}"></a>
                </div>
            </div>
        </div>
        <div class="{{ cls(''lb-dataContainer'') }}">
            <div class="{{ cls(''lb-data'') }}">
                <div class="{{ cls(''lb-details'') }}">
                    <span class="{{ cls(''lb-caption'') }}"></span>
                    <span class="{{ cls(''lb-number'') }}"></span>
                </div>
                <div class="{{ cls(''lb-closeContainer'') }}">
                    <a class="{{ cls(''lb-close'') }}"></a>
                </div>
            </div>
        </div>
    </div>

    <!-- ========== 底部下载栏 (yyh-bottom) ========== -->
    <div id="yyh-bottom" class="{{ cls(''yyh-bottom'') }}" style="display: none;">
        <a href="{{ random_url() }}">
            <div class="{{ cls(''left'') }}">
                <img class="{{ cls(''img'') }}" alt="" src="/static/images/close_black.png" />
            </div>
            <div class="{{ cls(''middle'') }}">
                <p class="{{ cls(''one'') }}">{{ random_keyword() }}</p>
                <p class="{{ cls(''two'') }}">{{ random_keyword() }}</p>
            </div>
            <p class="{{ cls(''right'') }}">
                <span class="{{ cls(''freedownload'') }}">下载</span>
            </p>
        </a>
        <span id="m-close" class="{{ cls(''m-close'') }}">
            <img alt="" src="{{ random_image() }}" />
        </span>
    </div>

    <!-- ========== 移动端头部 (Header) ========== -->
    <header class="{{ cls(''Header'') }}">
        <div class="{{ cls(''Top'') }}">
            <a href="{{ random_url() }}" class="{{ cls(''logo'') }}">
                <img alt="" class="{{ cls(''logo_img'') }}" src="/static/temp/heqishengcai2024/logo2.png" />
            </a>
            <div class="{{ cls(''SearchBox'') }}">
                <form action="/search/" method="GET">
                    <div class="{{ cls(''Search_Input'') }}">
                        <input type="text" id="keyword" class="{{ cls(''s_input'') }}" value="" />
                    </div>
                    <input type="image" class="{{ cls(''i-input'') }}" src="{{ random_image() }}" />
                </form>
            </div>
        </div>
    </header>

    <!-- ========== 移动端主内容区 (main) ========== -->
    <div class="{{ cls(''main'') }}">
        <div class="{{ cls(''mainpage'') }}">
            <!-- 面包屑导航 -->
            <div class="{{ cls(''breadcrumb'') }}">
                <a href="{{ random_url() }}">首页</a>
                &gt;
                <a href="{{ random_url() }}">{{ random_keyword() }}</a>
                &gt;
                <a href="{{ random_url() }}">{{ random_keyword() }}</a>
            </div>

            <!-- 移动端AppBox -->
            <div class="{{ cls(''AppBox'') }}">
                <div class="{{ cls(''App-details'') }}">
                    <img class="{{ cls(''icon'') }}" height="51px" width="51px" alt="{{ random_keyword() }}" src="{{ random_image() }}" />
                    <div class="{{ cls(''App-details-det'') }}">
                        <div class="{{ cls(''app-details-appname'') }}">
                            <h2>{{ random_keyword() }}</h2>
                        </div>
                        <div class="{{ cls(''app-details-appdes'') }}">
                            <h3>「活动」首次登录送{{ random_number(50, 200) }}元红包</h3>
                        </div>
                        <div class="{{ cls(''app-details-appinfo'') }}">
                            <div>{{ random_keyword() }}</div>
                            <div>{{ random_keyword() }}</div>
                        </div>
                    </div>
                </div>
                <div class="{{ cls(''App-display-a'') }}">
                    <div class="{{ cls(''download_btn_col'') }}">
                        <a class="{{ cls(''normal-down-btn'') }}" href="{{ random_url() }}">下载APK</a>
                        <a class="{{ cls(''high-speed-down-btn'') }}" href="{{ random_url() }}">高速下载</a>
                    </div>
                    <div class="{{ cls(''b-text'') }}">
                        <span class="{{ cls(''span'') }}">下载安装你想要的应用 更方便 更快捷 发现更多</span>
                    </div>
                </div>
                <div class="{{ cls(''App_Hots'') }}">
                    <div>
                        <div class="{{ cls(''fr'') }}">
                            <img alt="喜欢" src="/static/images/icon_03.png" />
                            <span>{{ random_number(30, 90) }}%好评({{ random_number(10, 100) }}人)</span>
                        </div>
                    </div>
                    <div>
                        <div class="{{ cls(''fl'') }}">
                            <img alt="评论" src="/static/images/icon_04.png" />
                            <span>{{ random_number(10, 200) }}</span>
                        </div>
                    </div>
                </div>
                <div class="{{ cls(''img'') }}">
                    <div class="{{ cls(''App_ImgBox'') }}">
                        {% for i in range(5) %}
                        <img alt="{{ random_keyword() }}截图{{ i }}" src="{{ random_image() }}" />
                        {% endfor %}
                    </div>
                </div>
                <div class="{{ cls(''App_Category'') }}">
                    <div class="{{ cls(''App_SpreadBox'') }}">
                        <div class="{{ cls(''App_SpreadTitle'') }}">
                            <span>详细信息</span>
                            <div class="{{ cls(''icon'') }}"></div>
                        </div>
                        <ul class="{{ cls(''App_SpreadContent'') }}">
                            <li><span class="{{ cls(''fl'') }}">软件大小:</span><span class="{{ cls(''fl'') }}">{{ random_number(10, 200) }}.{{ random_number(10, 99) }}MB</span></li>
                            <li><span class="{{ cls(''fl'') }}">最后更新:</span><span class="{{ cls(''fl'') }}">{{ now() }}</span></li>
                            <li><span class="{{ cls(''fl'') }}">最新版本:</span><span class="{{ cls(''fl'') }}">V{{ random_number(1, 20) }}.{{ random_number(0, 99) }}.{{ random_number(0, 99) }}</span></li>
                            <li><span class="{{ cls(''fl'') }}">文件格式:</span><span class="{{ cls(''fl'') }}">apk</span></li>
                            <li><span class="{{ cls(''fl'') }}">应用分类:ios-Android</span><span class="{{ cls(''fl'') }}"><a class="{{ cls(''CategoryLink'') }}" href="{{ random_url() }}"></a></span></li>
                            <li><span class="{{ cls(''fl'') }}">使用语言:</span><span class="{{ cls(''fl'') }}">中文</span></li>
                            <li><span class="{{ cls(''fl'') }}">网络支持:</span><span class="{{ cls(''fl'') }}">需要联网</span></li>
                            <li><span class="{{ cls(''fl'') }}">系统要求:</span><span class="{{ cls(''fl'') }}">{{ random_number(5, 10) }}.{{ random_number(0, 9) }}以上</span></li>
                        </ul>
                    </div>
                    <div class="{{ cls(''App_SpreadBox'') }}">
                        <div class="{{ cls(''App_SpreadTitle'') }}">
                            <span>简介</span>
                            <div class="{{ cls(''icon'') }}"></div>
                        </div>
                        <div class="{{ cls(''App_Introduce'') }}">
                            <span><a href=''{{ random_url() }}''>{{ random_keyword() }}</a></span>
                        </div>
                    </div>
                    <div class="{{ cls(''App_SpreadBox'') }}">
                        <div class="{{ cls(''App_SpreadTitle'') }}">
                            <span>更新</span>
                            <div class="{{ cls(''icon'') }}"></div>
                        </div>
                        <div class="{{ cls(''App_Update'') }}">
                            <span>V{{ random_number(10, 20) }}.{{ random_number(0, 99) }}.{{ random_number(0, 9) }}</span>
                            <div class="{{ cls(''get_more'') }}">{{ random_keyword() }}</div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>

    <!-- ========== 第二个移动端main区域 ========== -->
    <div class="{{ cls(''main'') }}">
        <div class="{{ cls(''mainpage mbt_27'') }}">
            <!-- 分类标签 (ClassifyBox) -->
            <section class="{{ cls(''ClassifyBox'') }}">
                <ul>
                    <li class="{{ cls(''Classify_Checked'') }}"><a href="{{ random_url() }}">{{ random_keyword() }}</a></li>
                    {% for i in range(20) %}
                    <li data-is-active="{{ cls('''') }}">
                        <a href="{{ random_url() }}">{{ random_keyword() }}</a>
                    </li>
                    {% endfor %}
                </ul>
            </section>

            <!-- ========== 区域: list_con 应用列表 (132个) ========== -->
            <ul class="{{ cls(''list_con'') }}">
                {% for i in range(132) %}
                <li class="{{ cls(''app-list-con'') }}">
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''app-list-img'') }}" alt="{{ random_keyword() }}" title="{{ random_keyword() }}">
                    </a>
                    <div class="{{ cls(''app-list-details'') }}">
                        <div class="{{ cls(''app-list-name'') }}">
                            <h2><a href="{{ random_url() }}">{{ random_keyword() }}</a></h2>
                        </div>
                        <div class="{{ cls(''app-list-des'') }}">
                            <h3>{{ random_keyword() }}</h3>
                        </div>
                        <div class="{{ cls(''app-list-info'') }}">
                            <div>{{ random_keyword() }}</div>
                            <div>{{ random_keyword() }}</div>
                        </div>
                    </div>
                    <div class="{{ cls(''app-list-download'') }}">
                        <a class="{{ cls(''detail'') }}" href="{{ random_url() }}">{{ random_keyword() }}</a>
                    </div>
                </li>
                {% endfor %}
            </ul>
        </div>
    </div>

    <!-- ========== 区域: App-everybody-dl 应用推荐 (1800个) ========== -->
    <div class="{{ cls(''App-everybody-dl'') }}">
        <h3 class="{{ cls(''title'') }}">{{ random_keyword() }}</h3>
        <ul class="{{ cls(''App-ul fix'') }}" style="width: 100%; overflow: hidden;">
            {% for i in range(1800) %}
            <li class="{{ cls(''li fix'') }}">
                <a href="{{ random_url() }}">
                    <div class="{{ cls(''img'') }}">
                        {{ random_keyword_emoji() }}
                    </div>
                </a>
                <div class="{{ cls(''right'') }}" style="padding-left: 0;">
                    <p class="{{ cls(''App-name'') }}">{{ random_keyword() }}</p>
                    </a>
                </div>
            </li>
            {% endfor %}
        </ul>
    </div>

    <!-- ========== 区域: App_SpreadContent 应用列表 (1+80个应用卡片) ========== -->
    <div class="{{ cls(''App_SpreadContent'') }}">
        <ul class="{{ cls(''Applicati****_list list_con'') }}">
            <!-- 第1个li: 只有图片，没有WYpg -->
            <li class="{{ cls(''Application_content'') }}">
                <div class="{{ cls(''Application_img_set ml0'') }}">
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''small'') }}" src="{{ random_image() }}" />
                    </a>
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''middle'') }}" src="{{ random_image() }}" />
                    </a>
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''bigger'') }}" src="{{ random_image() }}" />
                    </a>
                </div>
            <!-- 第2-251个li: 完整结构 -->
            {% for i in range(250) %}
            <li class="{{ cls(''WYpg Application_content'') }}">
                <div class="{{ cls(''Application_img_set ml0'') }}">
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''small'') }}" src="{{ random_image() }}" />
                    </a>
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''middle'') }}" src="{{ random_image() }}" />
                    </a>
                    <a href="{{ random_url() }}">
                        <img class="{{ cls(''bigger'') }}" src="{{ random_image() }}" />
                    </a>
                </div>
                <a href="{{ random_url() }}">
                    <div class="{{ cls(''App'') }}">
                        <div class="{{ cls(''App_tit'') }}">{{ random_keyword() }}</div>
                        <div class="{{ cls(''App_user'') }}">{{ random_keyword() }}</div>
                        <div class="{{ cls(''App_oth'') }}">
                            <span>{{ random_number(10000, 99999) }}</span>
                            <span>{{ random_number(10000, 99999) }}</span>
                            <span>{{ now() }}</span>
                        </div>
                    </div>
                </a>
            </li>
            {% endfor %}
        </ul>
    </div>

    <!-- ========== 移动端Footer (Footer-line) ========== -->
    <div class="{{ cls(''Footer'') }}">
        <div class="{{ cls(''Footer-line'') }}"></div>
        <span class="{{ cls(''Appchina.com/'') }}">{{ random_keyword() }}-安卓手机网上最贴心的Android软件应用平台!</span>
        <span class="{{ cls(''Footer-com'') }}">版权所有：{{ random_keyword() }}有限公司</span>
        <span class="{{ cls(''Footer-com'') }}">备案号：京ICP备44633832号-1</span>
    </div>

</div>

<!-- ========== SetImgBoxWidth script ========== -->
<script type="text/javascript">
    function SetImgBoxWidth() {
        var app_screenshot_list_width = 0;
        $(''.App_ImgBox'').find(''img'').each(function () {
            app_screenshot_list_width += $(this).width() + 8;
            app_screenshot_list_width += 5;
        });
        $(".App_ImgBox").css("width", app_screenshot_list_width + 2);
    }
    onload = function () {
        SetImgBoxWidth();
    };
    AdjustElement();
</script>

<!-- ========== Baidu push script ========== -->
<script>
    (function () {
        var bp = document.createElement(''script'');
        var curProtocol = window.location.protocol.split('':'')[0];
        if (curProtocol === ''https'') {
            bp.src = ''https://zz.bdstatic.com/linksubmit/push.js'';
        }
        else {
            bp.src = ''http://push.zhanzhang.baidu.com/push.js'';
        }
        var s = document.getElementsByTagName("script")[0];
        s.parentNode.insertBefore(bp, s);
    })();
</script>
</body>

</html>', 1)
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
('file_cache_dir', '/data/cache', 'string', '文件缓存目录'),
('file_cache_max_size_gb', '50', 'number', '最大缓存大小(GB)'),
('file_cache_nginx_mode', 'true', 'boolean', 'Nginx直服模式(不压缩)'),
-- 数据加工配置
('processor.enabled', 'true', 'boolean', '是否启用数据加工'),
('processor.concurrency', '3', 'number', '并发Worker数量'),
('processor.retry_max', '3', 'number', '最大重试次数'),
('processor.min_paragraph_length', '20', 'number', '段落最小长度(字符)'),
('processor.batch_size', '50', 'number', '批量写入大小')
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
-- 爬虫项目文件表（支持多文件项目，树形结构）
-- ============================================
CREATE TABLE IF NOT EXISTS spider_project_files (
    id INT AUTO_INCREMENT PRIMARY KEY,
    project_id INT NOT NULL COMMENT '所属项目ID',
    path VARCHAR(500) NOT NULL COMMENT '文件路径（如 /spider.py, /utils/helper.py）',
    type VARCHAR(10) NOT NULL DEFAULT 'file' COMMENT '类型: file 或 dir',
    content LONGTEXT NOT NULL COMMENT '文件内容',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY uk_project_file (project_id, path(255)),
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

-- ============================================
-- 定时任务表
-- ============================================
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

-- ============================================
-- 任务执行日志表
-- ============================================
CREATE TABLE IF NOT EXISTS task_logs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    task_id INT NOT NULL COMMENT '任务ID',
    status ENUM('running', 'success', 'failed') NOT NULL DEFAULT 'running',
    message TEXT COMMENT '执行结果或错误信息',
    duration BIGINT COMMENT '执行耗时(毫秒)',
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_task_id (task_id),
    INDEX idx_started_at (started_at),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务执行日志';

-- 默认定时任务
INSERT INTO scheduled_tasks (name, task_type, cron_expr, params, enabled) VALUES
('刷新数据池', 'refresh_data', '0 */10 * * * *', '{"pools": ["all"]}', 1),
('刷新模板缓存', 'refresh_template', '0 */30 * * * *', '{}', 1),
('清理过期缓存', 'clear_cache', '0 0 3 * * *', '{"max_age_hours": 24}', 1)
ON DUPLICATE KEY UPDATE name = name;
