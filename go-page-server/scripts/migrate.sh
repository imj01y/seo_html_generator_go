#!/bin/bash
# 数据库迁移脚本
# 使用方法: ./scripts/migrate.sh [command]
# 支持的命令: up, down, status, backup, verify, help

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
: "${DB_HOST:=localhost}"
: "${DB_PORT:=3306}"
: "${DB_USER:=root}"
: "${DB_PASSWORD:=}"
: "${DB_NAME:=seo_generator}"
: "${MIGRATIONS_PATH:=./migrations}"
: "${BACKUP_DIR:=./backups}"

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# 构建 DSN
build_dsn() {
    echo "${DB_USER}:${DB_PASSWORD}@tcp(${DB_HOST}:${DB_PORT})/${DB_NAME}?parseTime=true&charset=utf8mb4"
}

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查依赖
check_dependencies() {
    # 检查 migrate 工具是否存在
    MIGRATE_BIN="${PROJECT_DIR}/bin/migrate"
    if [ ! -f "$MIGRATE_BIN" ]; then
        # 尝试在 cmd/migrate 中构建
        if [ -f "${PROJECT_DIR}/cmd/migrate/main.go" ]; then
            print_info "构建迁移工具..."
            cd "$PROJECT_DIR"
            go build -o bin/migrate ./cmd/migrate
            print_success "迁移工具构建完成"
        else
            print_error "迁移工具不存在，请先构建: go build -o bin/migrate ./cmd/migrate"
            exit 1
        fi
    fi

    # 检查 mysql 客户端（用于备份）
    if ! command -v mysql &> /dev/null; then
        print_warning "mysql 客户端未安装，备份功能将不可用"
    fi

    if ! command -v mysqldump &> /dev/null; then
        print_warning "mysqldump 未安装，备份功能将不可用"
    fi
}

# 显示帮助信息
show_help() {
    echo "数据库迁移脚本"
    echo ""
    echo "使用方法: $0 [command] [options]"
    echo ""
    echo "命令:"
    echo "  up        执行所有待处理的迁移"
    echo "  down      回滚最近一次迁移"
    echo "  status    显示迁移状态"
    echo "  backup    备份数据库"
    echo "  verify    验证数据库结构"
    echo "  help      显示此帮助信息"
    echo ""
    echo "选项:"
    echo "  -t, --target VERSION  指定目标版本"
    echo ""
    echo "环境变量:"
    echo "  DB_HOST       数据库主机 (默认: localhost)"
    echo "  DB_PORT       数据库端口 (默认: 3306)"
    echo "  DB_USER       数据库用户 (默认: root)"
    echo "  DB_PASSWORD   数据库密码 (默认: 空)"
    echo "  DB_NAME       数据库名称 (默认: seo_generator)"
    echo "  MIGRATIONS_PATH  迁移文件路径 (默认: ./migrations)"
    echo "  BACKUP_DIR    备份文件目录 (默认: ./backups)"
    echo ""
    echo "示例:"
    echo "  DB_PASSWORD=secret $0 status"
    echo "  DB_PASSWORD=secret $0 up"
    echo "  DB_PASSWORD=secret $0 down -t 001"
    echo "  DB_PASSWORD=secret $0 backup"
}

# 执行迁移 (up)
do_migrate_up() {
    local target="$1"

    print_info "开始执行迁移..."

    DSN=$(build_dsn)
    MIGRATE_ARGS="-dsn \"$DSN\" -dir up -path \"$MIGRATIONS_PATH\""

    if [ -n "$target" ]; then
        MIGRATE_ARGS="$MIGRATE_ARGS -target $target"
        print_info "目标版本: $target"
    fi

    cd "$PROJECT_DIR"
    eval "$MIGRATE_BIN $MIGRATE_ARGS"

    print_success "迁移执行完成"
}

# 回滚迁移 (down)
do_migrate_down() {
    local target="$1"

    print_warning "开始回滚迁移..."

    DSN=$(build_dsn)
    MIGRATE_ARGS="-dsn \"$DSN\" -dir down -path \"$MIGRATIONS_PATH\""

    if [ -n "$target" ]; then
        MIGRATE_ARGS="$MIGRATE_ARGS -target $target"
        print_info "目标版本: $target"
    fi

    cd "$PROJECT_DIR"
    eval "$MIGRATE_BIN $MIGRATE_ARGS"

    print_success "回滚完成"
}

# 显示状态
do_status() {
    print_info "查询迁移状态..."

    DSN=$(build_dsn)

    cd "$PROJECT_DIR"
    "$MIGRATE_BIN" -dsn "$DSN" -dir status -path "$MIGRATIONS_PATH"
}

# 备份数据库
do_backup() {
    if ! command -v mysqldump &> /dev/null; then
        print_error "mysqldump 未安装，无法执行备份"
        exit 1
    fi

    # 创建备份目录
    mkdir -p "$BACKUP_DIR"

    # 生成备份文件名
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    BACKUP_FILE="${BACKUP_DIR}/${DB_NAME}_${TIMESTAMP}.sql"

    print_info "开始备份数据库..."
    print_info "备份文件: $BACKUP_FILE"

    # 执行备份
    MYSQL_PWD="$DB_PASSWORD" mysqldump \
        -h "$DB_HOST" \
        -P "$DB_PORT" \
        -u "$DB_USER" \
        --single-transaction \
        --routines \
        --triggers \
        "$DB_NAME" > "$BACKUP_FILE"

    # 压缩备份文件
    if command -v gzip &> /dev/null; then
        gzip "$BACKUP_FILE"
        BACKUP_FILE="${BACKUP_FILE}.gz"
    fi

    print_success "备份完成: $BACKUP_FILE"

    # 显示备份文件大小
    if [ -f "$BACKUP_FILE" ]; then
        SIZE=$(ls -lh "$BACKUP_FILE" | awk '{print $5}')
        print_info "备份文件大小: $SIZE"
    fi
}

# 验证数据库结构
do_verify() {
    if ! command -v mysql &> /dev/null; then
        print_error "mysql 客户端未安装，无法执行验证"
        exit 1
    fi

    print_info "验证数据库结构..."

    # 检查必要的表是否存在
    REQUIRED_TABLES=("sites" "schema_migrations" "scheduled_tasks" "task_logs")

    MISSING_TABLES=()

    for table in "${REQUIRED_TABLES[@]}"; do
        RESULT=$(MYSQL_PWD="$DB_PASSWORD" mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -N -e "
            SELECT COUNT(*) FROM information_schema.tables
            WHERE table_schema='$DB_NAME' AND table_name='$table'
        " 2>/dev/null || echo "0")

        if [ "$RESULT" -eq 0 ]; then
            MISSING_TABLES+=("$table")
            print_warning "表不存在: $table"
        else
            print_success "表存在: $table"
        fi
    done

    # 检查 sites 表的 push_code 字段
    if MYSQL_PWD="$DB_PASSWORD" mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -N -e "
        SELECT COUNT(*) FROM information_schema.columns
        WHERE table_schema='$DB_NAME' AND table_name='sites' AND column_name='push_code'
    " 2>/dev/null | grep -q "1"; then
        print_success "sites.push_code 字段存在"
    else
        print_warning "sites.push_code 字段不存在 (可能需要执行迁移)"
    fi

    # 检查迁移记录
    print_info ""
    print_info "迁移记录:"
    MYSQL_PWD="$DB_PASSWORD" mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -e "
        SELECT version, executed_at FROM $DB_NAME.schema_migrations ORDER BY version
    " 2>/dev/null || print_warning "无法读取迁移记录"

    echo ""
    if [ ${#MISSING_TABLES[@]} -eq 0 ]; then
        print_success "数据库结构验证通过"
    else
        print_error "缺少 ${#MISSING_TABLES[@]} 个表"
        exit 1
    fi
}

# 主函数
main() {
    # 检查依赖
    check_dependencies

    # 解析命令
    COMMAND="${1:-help}"
    shift || true

    # 解析选项
    TARGET=""
    while [[ $# -gt 0 ]]; do
        case $1 in
            -t|--target)
                TARGET="$2"
                shift 2
                ;;
            *)
                print_error "未知选项: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 执行命令
    case $COMMAND in
        up)
            do_migrate_up "$TARGET"
            ;;
        down)
            do_migrate_down "$TARGET"
            ;;
        status)
            do_status
            ;;
        backup)
            do_backup
            ;;
        verify)
            do_verify
            ;;
        help|--help|-h)
            show_help
            ;;
        *)
            print_error "未知命令: $COMMAND"
            show_help
            exit 1
            ;;
    esac
}

# 执行主函数
main "$@"
