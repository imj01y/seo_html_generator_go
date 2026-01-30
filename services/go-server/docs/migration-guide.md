# 数据库迁移指南

本文档介绍如何使用数据库迁移工具管理 SEO HTML Generator 的数据库结构变更。

## 目录

- [前提条件](#前提条件)
- [快速开始](#快速开始)
- [迁移步骤](#迁移步骤)
- [环境变量](#环境变量)
- [迁移文件格式](#迁移文件格式)
- [常见问题](#常见问题)

## 前提条件

### 系统要求

- Go 1.19 或更高版本（用于构建迁移工具）
- MySQL 5.7 或更高版本
- Git Bash 或 WSL（Windows 用户）
- mysql 和 mysqldump 命令行工具（用于备份和验证）

### 安装依赖

```bash
# 构建迁移工具
cd go-page-server
go build -o bin/migrate ./cmd/migrate
```

## 快速开始

```bash
# 查看迁移状态
DB_PASSWORD=your_password ./scripts/migrate.sh status

# 执行所有待处理的迁移
DB_PASSWORD=your_password ./scripts/migrate.sh up

# 回滚最近一次迁移
DB_PASSWORD=your_password ./scripts/migrate.sh down
```

## 迁移步骤

完整的迁移流程包括以下步骤：

### 1. 备份数据库

**在执行任何迁移之前，务必先备份数据库！**

```bash
# 使用迁移脚本备份
DB_PASSWORD=your_password ./scripts/migrate.sh backup

# 或手动备份
mysqldump -u root -p seo_generator > backup_$(date +%Y%m%d).sql
```

备份文件将保存在 `./backups/` 目录下。

### 2. 检查迁移状态

查看当前数据库的迁移状态：

```bash
DB_PASSWORD=your_password ./scripts/migrate.sh status
```

输出示例：
```
Migration Status
================
Version    Name                                     Status     Executed At
--------------------------------------------------------------------------------
001        rename_baidu_token                       Done       2024-01-15 10:30:00
002        placeholder                              Done       2024-01-15 10:30:01
003        scheduled_tasks                          Pending
--------------------------------------------------------------------------------
Total: 3 migrations (2 done, 1 pending)
```

### 3. 执行迁移

执行所有待处理的迁移：

```bash
# 执行所有待处理迁移
DB_PASSWORD=your_password ./scripts/migrate.sh up

# 执行到指定版本
DB_PASSWORD=your_password ./scripts/migrate.sh up -t 002
```

### 4. 验证迁移

验证数据库结构是否正确：

```bash
DB_PASSWORD=your_password ./scripts/migrate.sh verify
```

验证会检查：
- 必要的表是否存在（sites, schema_migrations, scheduled_tasks, task_logs）
- 关键字段是否存在（如 sites.push_code）
- 迁移记录是否完整

### 5. 回滚（如需要）

如果迁移出现问题，可以回滚：

```bash
# 回滚最近一次迁移
DB_PASSWORD=your_password ./scripts/migrate.sh down

# 回滚到指定版本（不包含该版本）
DB_PASSWORD=your_password ./scripts/migrate.sh down -t 001
```

## 环境变量

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| `DB_HOST` | localhost | 数据库主机地址 |
| `DB_PORT` | 3306 | 数据库端口 |
| `DB_USER` | root | 数据库用户名 |
| `DB_PASSWORD` | (空) | 数据库密码 |
| `DB_NAME` | seo_generator | 数据库名称 |
| `MIGRATIONS_PATH` | ./migrations | 迁移文件目录 |
| `BACKUP_DIR` | ./backups | 备份文件目录 |

### 使用示例

```bash
# 生产环境
export DB_HOST=prod-db.example.com
export DB_PORT=3306
export DB_USER=app_user
export DB_PASSWORD=secure_password
export DB_NAME=seo_prod

./scripts/migrate.sh status
./scripts/migrate.sh up

# 或一行命令
DB_HOST=prod-db.example.com DB_USER=app_user DB_PASSWORD=xxx ./scripts/migrate.sh up
```

## 迁移文件格式

迁移文件位于 `migrations/` 目录，命名格式为 `NNN_description.sql`，其中 NNN 是三位数字版本号。

### 文件结构

```sql
-- migrations/001_rename_baidu_token.sql
-- 简要说明变更内容

-- UP Migration
-- 这里是升级 SQL
ALTER TABLE sites ADD COLUMN new_field VARCHAR(255);

-- DOWN Migration (用于回滚)
-- 注释掉的 SQL 会被解析并执行
-- ALTER TABLE sites DROP COLUMN new_field;
```

### 重要说明

1. **UP Migration** 部分包含升级时执行的 SQL
2. **DOWN Migration** 部分包含回滚时执行的 SQL
3. DOWN 部分的 SQL 通常是注释形式（以 `-- ` 开头），迁移工具会自动去除注释前缀
4. 每个迁移应该是原子性的，并且可逆的

## 迁移文件列表

| 版本 | 文件名 | 说明 |
|------|--------|------|
| 000 | 000_init.sql | 初始化说明（无实际迁移） |
| 001 | 001_rename_baidu_token.sql | 将 baidu_token 重命名为 push_code |
| 002 | 002_placeholder.sql | 占位符迁移 |
| 003 | 003_scheduled_tasks.sql | 创建定时任务相关表 |

## 常见问题

### Q: 迁移失败了怎么办？

1. 查看错误信息，确定失败原因
2. 如果是 SQL 语法错误，修复迁移文件后重新执行
3. 如果数据库状态不一致，可能需要手动修复：
   ```sql
   -- 删除失败的迁移记录
   DELETE FROM schema_migrations WHERE version = '003';
   ```
4. 从备份恢复（如果需要）

### Q: 如何创建新的迁移？

1. 在 `migrations/` 目录创建新文件，版本号递增：
   ```bash
   touch migrations/004_add_new_feature.sql
   ```
2. 按照文件格式编写 UP 和 DOWN SQL
3. 测试迁移：
   ```bash
   ./scripts/migrate.sh up
   ./scripts/migrate.sh down
   ./scripts/migrate.sh up
   ```

### Q: 如何在生产环境安全迁移？

1. **创建数据库快照或备份**
2. 在测试环境验证迁移
3. 选择低峰期执行
4. 准备回滚计划
5. 监控应用日志

### Q: 迁移工具提示 "mysql 客户端未安装"

备份和验证功能需要 mysql 客户端。安装方法：

```bash
# Ubuntu/Debian
sudo apt install mysql-client

# CentOS/RHEL
sudo yum install mysql

# macOS
brew install mysql-client

# Windows (使用 MSYS2 或下载 MySQL Installer)
```

### Q: 如何处理大表的迁移？

对于大表的结构变更：

1. 考虑使用 pt-online-schema-change 或 gh-ost
2. 分批执行数据迁移
3. 在低峰期执行
4. 设置适当的超时时间

### Q: 迁移和回滚都卡住了怎么办？

1. 检查是否有长事务阻塞：
   ```sql
   SHOW PROCESSLIST;
   ```
2. 检查表锁：
   ```sql
   SHOW OPEN TABLES WHERE In_use > 0;
   ```
3. 必要时终止阻塞的查询

## 使用迁移工具（直接调用）

除了使用 Shell 脚本，也可以直接调用迁移工具：

```bash
# 构建工具
go build -o bin/migrate ./cmd/migrate

# 执行迁移
./bin/migrate -dsn "user:password@tcp(localhost:3306)/dbname?parseTime=true" -dir up

# 查看状态
./bin/migrate -dsn "user:password@tcp(localhost:3306)/dbname?parseTime=true" -dir status

# 回滚
./bin/migrate -dsn "user:password@tcp(localhost:3306)/dbname?parseTime=true" -dir down

# 指定迁移文件路径
./bin/migrate -dsn "..." -dir up -path ./migrations
```

## 附录：恢复备份

```bash
# 从 SQL 文件恢复
mysql -u root -p seo_generator < backups/seo_generator_20240115_103000.sql

# 从 gzip 压缩文件恢复
gunzip -c backups/seo_generator_20240115_103000.sql.gz | mysql -u root -p seo_generator
```
