-- =============================================================================
-- Go Page Server 测试数据初始化脚本
-- 用于 Docker 一键部署环境
-- =============================================================================

SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

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
-- 关键词表
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
-- 图片表
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
-- 正文库表
-- ============================================
CREATE TABLE IF NOT EXISTS contents (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    group_id INT NOT NULL DEFAULT 1 COMMENT '所属分组ID',
    content MEDIUMTEXT NOT NULL COMMENT '已生成的完整正文',
    batch_id INT DEFAULT 0 COMMENT '批次号',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_group_batch (group_id, batch_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='正文库';

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
    version INT DEFAULT 1 COMMENT '版本号',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_site_group (site_group_id),
    INDEX idx_status (status),
    UNIQUE INDEX idx_site_group_name (site_group_id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模板表';

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
-- 测试数据：站群
-- ============================================
INSERT INTO site_groups (id, name, description) VALUES
(1, '默认站群', '系统默认站群')
ON DUPLICATE KEY UPDATE name = name;

-- ============================================
-- 测试数据：关键词分组 + 关键词
-- ============================================
INSERT INTO keyword_groups (id, site_group_id, name, description, is_default) VALUES
(1, 1, '默认关键词分组', '系统默认关键词分组', 1)
ON DUPLICATE KEY UPDATE name = name;

INSERT INTO keywords (group_id, keyword) VALUES
(1, '免费下载'),
(1, '软件下载'),
(1, '最新版本'),
(1, '官方下载'),
(1, '绿色软件'),
(1, '破解版下载'),
(1, '安装教程'),
(1, '使用指南'),
(1, '常见问题'),
(1, '软件评测');

-- ============================================
-- 测试数据：图片分组 + 图片
-- ============================================
INSERT INTO image_groups (id, site_group_id, name, description, is_default) VALUES
(1, 1, '默认图片分组', '系统默认图片分组', 1)
ON DUPLICATE KEY UPDATE name = name;

INSERT INTO images (group_id, url) VALUES
(1, 'https://picsum.photos/800/600?random=1'),
(1, 'https://picsum.photos/800/600?random=2'),
(1, 'https://picsum.photos/800/600?random=3'),
(1, 'https://picsum.photos/800/600?random=4'),
(1, 'https://picsum.photos/800/600?random=5'),
(1, 'https://picsum.photos/640/480?random=6'),
(1, 'https://picsum.photos/640/480?random=7'),
(1, 'https://picsum.photos/640/480?random=8'),
(1, 'https://picsum.photos/640/480?random=9'),
(1, 'https://picsum.photos/640/480?random=10');

-- ============================================
-- 测试数据：文章分组 + 正文
-- ============================================
INSERT INTO article_groups (id, site_group_id, name, description, is_default) VALUES
(1, 1, '默认文章分组', '系统默认文章分组', 1)
ON DUPLICATE KEY UPDATE name = name;

INSERT INTO contents (group_id, content) VALUES
(1, '<p>这是一款非常实用的软件，拥有强大的功能和简洁的界面设计。无论您是专业用户还是普通用户，都能轻松上手使用。</p><p>软件支持多种文件格式，处理速度快，占用系统资源少。定期更新确保您始终使用最新版本。</p>'),
(1, '<p>本软件经过严格测试，安全无毒，请放心下载使用。安装过程简单快捷，只需几分钟即可完成。</p><p>如有任何问题，欢迎联系我们的技术支持团队，我们将竭诚为您服务。</p>'),
(1, '<p>功能特点：界面简洁直观，操作便捷高效。支持批量处理，大大提高工作效率。兼容主流操作系统。</p><p>更新日志：修复已知问题，优化性能表现，新增实用功能，提升用户体验。</p>'),
(1, '<p>使用教程：第一步，下载安装包到本地。第二步，双击运行安装程序。第三步，按照提示完成安装。第四步，启动软件开始使用。</p>'),
(1, '<p>常见问题解答：如果遇到安装失败，请检查系统权限设置。如果软件运行缓慢，请尝试清理缓存文件。如需更多帮助，请查阅官方文档。</p>');

-- ============================================
-- 测试数据：模板（Jinja2 格式）
-- ============================================
INSERT INTO templates (site_group_id, name, display_name, description, content) VALUES
(1, 'download_site', '下载站模板', '适用于软件下载类站点的SEO模板', '<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ title }} - {{ site_name }}</title>
    <meta name="keywords" content="{{ keyword }}">
    <meta name="description" content="{{ title }}，免费下载，安全无毒">
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 800px; margin: 0 auto; background: #fff; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
        .content { line-height: 1.8; color: #555; }
        .download-btn { display: inline-block; background: #007bff; color: #fff; padding: 12px 30px; border-radius: 5px; text-decoration: none; margin: 20px 0; }
        .download-btn:hover { background: #0056b3; }
        .links { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; }
        .links a { color: #007bff; margin-right: 15px; text-decoration: none; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; color: #999; font-size: 12px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>{{ title }}</h1>
        <div class="content">
            {{ content }}
        </div>
        <a href="{{ random_url() }}" class="download-btn">立即下载</a>
        <div class="links">
            <strong>相关推荐：</strong>
            {% for i in range(5) %}
            <a href="{{ random_url() }}">{{ random_keyword() }}</a>
            {% endfor %}
        </div>
        <div class="footer">
            <p>&copy; {{ site_name }} | {{ icp_number }}</p>
        </div>
    </div>
</body>
</html>')
ON DUPLICATE KEY UPDATE content = VALUES(content);

-- ============================================
-- 测试数据：站点
-- ============================================
INSERT INTO sites (site_group_id, domain, name, template, keyword_group_id, image_group_id, article_group_id, icp_number) VALUES
(1, 'test.example.com', '测试站点', 'download_site', 1, 1, 1, '京ICP备12345678号'),
(1, 'demo.example.com', '演示站点', 'download_site', 1, 1, 1, '京ICP备87654321号')
ON DUPLICATE KEY UPDATE domain = domain;
