-- migrations/002_spider_files_tree.sql
-- 爬虫项目文件表支持树形结构迁移

-- 1. 重命名 filename 为 path，修改长度
ALTER TABLE spider_project_files
  CHANGE COLUMN filename path VARCHAR(500) NOT NULL COMMENT '文件路径（如 /spider.py, /lib/utils.py）';

-- 2. 为现有数据添加 / 前缀
UPDATE spider_project_files
  SET path = CONCAT('/', path)
  WHERE path NOT LIKE '/%';

-- 3. 添加 type 字段区分文件和目录
ALTER TABLE spider_project_files
  ADD COLUMN type ENUM('file', 'dir') NOT NULL DEFAULT 'file' COMMENT '类型：file=文件, dir=目录' AFTER path;

-- 4. 更新唯一索引
ALTER TABLE spider_project_files
  DROP INDEX uk_project_file,
  ADD UNIQUE INDEX uk_project_path (project_id, path);
