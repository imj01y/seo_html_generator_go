# 数据库迁移实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 一键迁移脚本，包含数据库变更和服务切换验证。

**Architecture:** SQL 迁移文件 + Go 迁移工具 + 回滚支持

**Tech Stack:** Go, MySQL

**依赖:** 所有其他阶段完成后执行

---

## Task 1: 创建迁移文件目录结构

**Files:**
- Create: `go-page-server/migrations/` 目录
- Create: `go-page-server/migrations/000_init.sql`

**Step 1: 创建初始迁移文件**

```sql
-- migrations/000_init.sql
-- 初始数据库结构（现有表）

-- 此文件记录现有数据库结构，仅供参考
-- 实际迁移从 001 开始

-- sites 表（现有）
-- templates 表（现有）
-- keywords 表（现有）
-- images 表（现有）
-- titles 表（现有）
-- contents 表（现有）
```

**Step 2: Commit**

```bash
git add go-page-server/migrations/000_init.sql
git commit -m "docs: add initial migration reference"
```

---

## Task 2: 创建 baidu_token 到 push_code 迁移

**Files:**
- Create: `go-page-server/migrations/001_rename_baidu_token.sql`

**Step 1: 创建迁移文件**

```sql
-- migrations/001_rename_baidu_token.sql
-- 将 baidu_token 字段重命名为 push_code

-- UP Migration
ALTER TABLE sites CHANGE COLUMN baidu_token push_code TEXT COMMENT '推送JS代码';

-- DOWN Migration (用于回滚)
-- ALTER TABLE sites CHANGE COLUMN push_code baidu_token TEXT COMMENT '百度推送token';
```

**Step 2: Commit**

```bash
git add go-page-server/migrations/001_rename_baidu_token.sql
git commit -m "feat: add migration for baidu_token to push_code"
```

---

## Task 3: 创建定时任务表迁移

**Files:**
- Create: `go-page-server/migrations/002_scheduled_tasks.sql`

**Step 1: 创建迁移文件**

```sql
-- migrations/002_scheduled_tasks.sql
-- 定时任务相关表

-- UP Migration

-- 定时任务表
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL COMMENT '任务名称',
    task_type VARCHAR(50) NOT NULL COMMENT '任务类型',
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

-- DOWN Migration (用于回滚)
-- DROP TABLE IF EXISTS task_logs;
-- DROP TABLE IF EXISTS scheduled_tasks;
```

**Step 2: Commit**

```bash
git add go-page-server/migrations/002_scheduled_tasks.sql
git commit -m "feat: add scheduled tasks migration"
```

---

## Task 4: 创建迁移工具

**Files:**
- Create: `go-page-server/cmd/migrate/main.go`

**Step 1: 创建迁移工具**

```go
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var (
	dsn       = flag.String("dsn", "", "Database connection string")
	direction = flag.String("dir", "up", "Migration direction: up or down")
	target    = flag.String("target", "", "Target migration (e.g., 002)")
)

func main() {
	flag.Parse()

	if *dsn == "" {
		fmt.Println("Error: -dsn is required")
		os.Exit(1)
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// 确保迁移记录表存在
	if err := ensureMigrationTable(db); err != nil {
		fmt.Printf("Error creating migration table: %v\n", err)
		os.Exit(1)
	}

	// 获取迁移文件
	migrations, err := getMigrations()
	if err != nil {
		fmt.Printf("Error reading migrations: %v\n", err)
		os.Exit(1)
	}

	// 获取已执行的迁移
	executed, err := getExecutedMigrations(db)
	if err != nil {
		fmt.Printf("Error getting executed migrations: %v\n", err)
		os.Exit(1)
	}

	switch *direction {
	case "up":
		if err := migrateUp(db, migrations, executed, *target); err != nil {
			fmt.Printf("Error during migration: %v\n", err)
			os.Exit(1)
		}
	case "down":
		if err := migrateDown(db, migrations, executed, *target); err != nil {
			fmt.Printf("Error during rollback: %v\n", err)
			os.Exit(1)
		}
	case "status":
		showStatus(migrations, executed)
	default:
		fmt.Printf("Unknown direction: %s\n", *direction)
		os.Exit(1)
	}
}

func ensureMigrationTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(50) PRIMARY KEY,
			executed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func getMigrations() ([]string, error) {
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		return nil, err
	}

	var migrations []string
	for _, f := range files {
		name := filepath.Base(f)
		if !strings.HasPrefix(name, "000") { // 跳过初始参考文件
			migrations = append(migrations, name)
		}
	}

	sort.Strings(migrations)
	return migrations, nil
}

func getExecutedMigrations(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	executed := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			continue
		}
		executed[version] = true
	}

	return executed, nil
}

func migrateUp(db *sql.DB, migrations []string, executed map[string]bool, target string) error {
	for _, m := range migrations {
		version := strings.TrimSuffix(m, ".sql")

		if executed[version] {
			continue
		}

		if target != "" && version > target {
			break
		}

		fmt.Printf("Executing migration: %s\n", m)

		content, err := os.ReadFile(filepath.Join("migrations", m))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", m, err)
		}

		// 提取 UP 部分
		sql := extractUpSQL(string(content))
		if sql == "" {
			continue
		}

		// 执行迁移
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("failed to execute %s: %w", m, err)
		}

		// 记录迁移
		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", m, err)
		}

		fmt.Printf("Completed: %s\n", m)
	}

	fmt.Println("Migration completed successfully")
	return nil
}

func migrateDown(db *sql.DB, migrations []string, executed map[string]bool, target string) error {
	// 反向遍历
	for i := len(migrations) - 1; i >= 0; i-- {
		m := migrations[i]
		version := strings.TrimSuffix(m, ".sql")

		if !executed[version] {
			continue
		}

		if target != "" && version <= target {
			break
		}

		fmt.Printf("Rolling back migration: %s\n", m)

		content, err := os.ReadFile(filepath.Join("migrations", m))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", m, err)
		}

		// 提取 DOWN 部分
		sql := extractDownSQL(string(content))
		if sql == "" {
			fmt.Printf("Warning: No DOWN migration found for %s\n", m)
			continue
		}

		// 执行回滚
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("failed to rollback %s: %w", m, err)
		}

		// 删除迁移记录
		if _, err := db.Exec("DELETE FROM schema_migrations WHERE version = ?", version); err != nil {
			return fmt.Errorf("failed to remove migration record %s: %w", m, err)
		}

		fmt.Printf("Rolled back: %s\n", m)
	}

	fmt.Println("Rollback completed successfully")
	return nil
}

func extractUpSQL(content string) string {
	// 查找 UP Migration 注释后的 SQL
	lines := strings.Split(content, "\n")
	var sql strings.Builder
	inUp := false

	for _, line := range lines {
		if strings.Contains(line, "UP Migration") {
			inUp = true
			continue
		}
		if strings.Contains(line, "DOWN Migration") {
			break
		}
		if inUp && !strings.HasPrefix(strings.TrimSpace(line), "--") {
			sql.WriteString(line)
			sql.WriteString("\n")
		}
	}

	return strings.TrimSpace(sql.String())
}

func extractDownSQL(content string) string {
	// 查找 DOWN Migration 注释后的 SQL（去掉注释符号）
	lines := strings.Split(content, "\n")
	var sql strings.Builder
	inDown := false

	for _, line := range lines {
		if strings.Contains(line, "DOWN Migration") {
			inDown = true
			continue
		}
		if inDown {
			// 去掉行首的 "-- " 注释
			trimmed := strings.TrimPrefix(line, "-- ")
			if trimmed != line { // 确实有注释
				sql.WriteString(trimmed)
				sql.WriteString("\n")
			}
		}
	}

	return strings.TrimSpace(sql.String())
}

func showStatus(migrations []string, executed map[string]bool) {
	fmt.Println("Migration Status:")
	fmt.Println("-----------------")

	for _, m := range migrations {
		version := strings.TrimSuffix(m, ".sql")
		status := "pending"
		if executed[version] {
			status = "executed"
		}
		fmt.Printf("[%s] %s\n", status, m)
	}
}
```

**Step 2: Commit**

```bash
git add go-page-server/cmd/migrate/main.go
git commit -m "feat: add database migration tool"
```

---

## Task 5: 创建迁移脚本

**Files:**
- Create: `go-page-server/scripts/migrate.sh`

**Step 1: 创建迁移脚本**

```bash
#!/bin/bash

# 数据库迁移脚本

set -e

# 配置
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASS="${DB_PASS:-}"
DB_NAME="${DB_NAME:-seo_generator}"

DSN="${DB_USER}:${DB_PASS}@tcp(${DB_HOST}:${DB_PORT})/${DB_NAME}?multiStatements=true"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 帮助信息
show_help() {
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  up [target]     Run pending migrations (optional: up to target version)"
    echo "  down [target]   Rollback migrations (optional: down to target version)"
    echo "  status          Show migration status"
    echo "  backup          Backup database before migration"
    echo "  verify          Verify migration completed successfully"
    echo ""
    echo "Environment Variables:"
    echo "  DB_HOST         Database host (default: localhost)"
    echo "  DB_PORT         Database port (default: 3306)"
    echo "  DB_USER         Database user (default: root)"
    echo "  DB_PASS         Database password"
    echo "  DB_NAME         Database name (default: seo_generator)"
}

# 备份数据库
backup_database() {
    echo -e "${YELLOW}Backing up database...${NC}"

    BACKUP_FILE="backup_${DB_NAME}_$(date +%Y%m%d_%H%M%S).sql"

    mysqldump -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" \
        --single-transaction --routines --triggers \
        "$DB_NAME" > "$BACKUP_FILE"

    echo -e "${GREEN}Backup saved to: $BACKUP_FILE${NC}"
}

# 运行迁移
run_migrate() {
    local direction=$1
    local target=$2

    cd "$(dirname "$0")/.."

    if [ -n "$target" ]; then
        go run ./cmd/migrate -dsn="$DSN" -dir="$direction" -target="$target"
    else
        go run ./cmd/migrate -dsn="$DSN" -dir="$direction"
    fi
}

# 验证迁移
verify_migration() {
    echo -e "${YELLOW}Verifying migration...${NC}"

    # 检查 schema_migrations 表
    RESULT=$(mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" \
        -N -e "SELECT COUNT(*) FROM schema_migrations" "$DB_NAME" 2>/dev/null)

    if [ "$RESULT" -gt 0 ]; then
        echo -e "${GREEN}✓ Migration table exists with $RESULT migrations${NC}"
    else
        echo -e "${YELLOW}⚠ No migrations recorded${NC}"
    fi

    # 检查新表
    if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" \
        -N -e "DESCRIBE scheduled_tasks" "$DB_NAME" &>/dev/null; then
        echo -e "${GREEN}✓ scheduled_tasks table exists${NC}"
    else
        echo -e "${RED}✗ scheduled_tasks table missing${NC}"
    fi

    if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" \
        -N -e "DESCRIBE task_logs" "$DB_NAME" &>/dev/null; then
        echo -e "${GREEN}✓ task_logs table exists${NC}"
    else
        echo -e "${RED}✗ task_logs table missing${NC}"
    fi

    # 检查 push_code 字段
    if mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" \
        -N -e "SHOW COLUMNS FROM sites LIKE 'push_code'" "$DB_NAME" 2>/dev/null | grep -q push_code; then
        echo -e "${GREEN}✓ push_code column exists in sites table${NC}"
    else
        echo -e "${RED}✗ push_code column missing${NC}"
    fi
}

# 主逻辑
case "$1" in
    up)
        run_migrate "up" "$2"
        ;;
    down)
        run_migrate "down" "$2"
        ;;
    status)
        run_migrate "status"
        ;;
    backup)
        backup_database
        ;;
    verify)
        verify_migration
        ;;
    help|--help|-h)
        show_help
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        echo ""
        show_help
        exit 1
        ;;
esac
```

**Step 2: 添加执行权限并提交**

```bash
chmod +x go-page-server/scripts/migrate.sh
git add go-page-server/scripts/migrate.sh
git commit -m "feat: add migration shell script"
```

---

## Task 6: 创建完整迁移指南

**Files:**
- Create: `go-page-server/docs/migration-guide.md`

**Step 1: 创建迁移指南**

```markdown
# 数据库迁移指南

## 前提条件

1. Go 1.21+ 已安装
2. MySQL 客户端已安装（用于备份）
3. 数据库访问权限

## 迁移步骤

### 1. 备份数据库

**强烈建议在迁移前备份数据库！**

```bash
# 使用脚本备份
./scripts/migrate.sh backup

# 或手动备份
mysqldump -u root -p seo_generator > backup.sql
```

### 2. 检查当前状态

```bash
./scripts/migrate.sh status
```

### 3. 执行迁移

```bash
# 执行所有待处理的迁移
./scripts/migrate.sh up

# 或指定目标版本
./scripts/migrate.sh up 002
```

### 4. 验证迁移

```bash
./scripts/migrate.sh verify
```

### 5. 回滚（如需要）

```bash
# 回滚所有迁移
./scripts/migrate.sh down

# 或回滚到指定版本
./scripts/migrate.sh down 001
```

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| DB_HOST | localhost | 数据库主机 |
| DB_PORT | 3306 | 数据库端口 |
| DB_USER | root | 数据库用户 |
| DB_PASS | - | 数据库密码 |
| DB_NAME | seo_generator | 数据库名 |

## 迁移文件说明

| 文件 | 说明 |
|------|------|
| 001_rename_baidu_token.sql | baidu_token → push_code |
| 002_scheduled_tasks.sql | 添加定时任务表 |

## 常见问题

### Q: 迁移失败怎么办？

1. 检查错误信息
2. 修复问题
3. 如需要，从备份恢复
4. 重新执行迁移

### Q: 如何添加新迁移？

1. 在 `migrations/` 目录创建新文件，格式：`NNN_description.sql`
2. 包含 `-- UP Migration` 和 `-- DOWN Migration` 注释
3. DOWN 部分用 `-- ` 前缀注释

### Q: 生产环境注意事项

1. 在低峰期执行迁移
2. 确保备份完成
3. 准备回滚计划
4. 监控服务状态
```

**Step 2: Commit**

```bash
git add go-page-server/docs/migration-guide.md
git commit -m "docs: add database migration guide"
```

---

## Task 7: 添加测试

**Files:**
- Create: `go-page-server/cmd/migrate/migrate_test.go`

**Step 1: 创建测试文件**

```go
package main

import (
	"testing"
)

func TestExtractUpSQL(t *testing.T) {
	content := `
-- migrations/test.sql

-- UP Migration
CREATE TABLE test (id INT);
INSERT INTO test VALUES (1);

-- DOWN Migration
-- DROP TABLE test;
`

	sql := extractUpSQL(content)

	if sql == "" {
		t.Error("Expected non-empty UP SQL")
	}

	if !strings.Contains(sql, "CREATE TABLE") {
		t.Error("Expected CREATE TABLE in UP SQL")
	}

	if strings.Contains(sql, "DROP TABLE") {
		t.Error("DOWN SQL should not be included")
	}
}

func TestExtractDownSQL(t *testing.T) {
	content := `
-- migrations/test.sql

-- UP Migration
CREATE TABLE test (id INT);

-- DOWN Migration
-- DROP TABLE test;
`

	sql := extractDownSQL(content)

	if sql == "" {
		t.Error("Expected non-empty DOWN SQL")
	}

	if !strings.Contains(sql, "DROP TABLE") {
		t.Error("Expected DROP TABLE in DOWN SQL")
	}
}

func TestExtractDownSQL_NoDown(t *testing.T) {
	content := `
-- migrations/test.sql

-- UP Migration
CREATE TABLE test (id INT);
`

	sql := extractDownSQL(content)

	if sql != "" {
		t.Error("Expected empty DOWN SQL when not present")
	}
}
```

需要添加导入：`"strings"`

**Step 2: 运行测试**

```bash
cd go-page-server && go test -v ./cmd/migrate/...
```

Expected: PASS

**Step 3: Commit**

```bash
git add go-page-server/cmd/migrate/migrate_test.go
git commit -m "test: add migration tool tests"
```

---

## 完整迁移检查清单

### 迁移前

- [ ] 备份生产数据库
- [ ] 确认没有正在进行的写操作
- [ ] 通知相关人员
- [ ] 准备回滚计划

### 迁移中

- [ ] 执行 `./scripts/migrate.sh status` 检查状态
- [ ] 执行 `./scripts/migrate.sh up` 运行迁移
- [ ] 查看迁移输出，确认无错误

### 迁移后

- [ ] 执行 `./scripts/migrate.sh verify` 验证
- [ ] 测试应用功能
- [ ] 检查日志无异常
- [ ] 确认服务正常运行

---

## 完成检查清单

- [ ] Task 1: 迁移文件目录
- [ ] Task 2: baidu_token 迁移
- [ ] Task 3: 定时任务表迁移
- [ ] Task 4: 迁移工具
- [ ] Task 5: 迁移脚本
- [ ] Task 6: 迁移指南
- [ ] Task 7: 测试覆盖
