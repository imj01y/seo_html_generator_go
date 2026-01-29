-- migrations/001_rename_baidu_token.sql
-- 将 baidu_token 字段重命名为 push_code

-- UP Migration
ALTER TABLE sites CHANGE COLUMN baidu_token push_code TEXT COMMENT '推送JS代码';

-- DOWN Migration (用于回滚)
-- ALTER TABLE sites CHANGE COLUMN push_code baidu_token TEXT COMMENT '百度推送token';
