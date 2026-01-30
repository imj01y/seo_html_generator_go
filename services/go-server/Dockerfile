# =============================================================================
# Go Page Server Dockerfile
# 支持容器内编译和热重载
# =============================================================================

# 使用包含完整工具链的镜像（支持容器内编译）
FROM golang:1.22-alpine AS builder

# 安装必要工具
RUN apk add --no-cache git gcc musl-dev

# 安装 quicktemplate 编译器
RUN go install github.com/valyala/quicktemplate/qtc@latest

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 初次编译 quicktemplate（如果模板存在）
RUN if [ -d "templates" ] && ls templates/*.qtpl 1> /dev/null 2>&1; then \
        qtc -dir=templates/; \
    fi

# 编译应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server .

# =============================================================================
# 运行时镜像（包含编译工具以支持动态编译）
# =============================================================================
FROM golang:1.22-alpine

# 安装必要工具
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev \
    ca-certificates \
    tzdata

# 安装 quicktemplate 编译器
RUN go install github.com/valyala/quicktemplate/qtc@latest

# 设置时区
ENV TZ=Asia/Shanghai

WORKDIR /app

# 复制编译后的应用
COPY --from=builder /app/server .

# 复制源码（用于动态编译）
COPY --from=builder /app/go.mod .
COPY --from=builder /app/go.sum .
COPY --from=builder /app/*.go ./
COPY --from=builder /app/core ./core/
COPY --from=builder /app/config ./config/
COPY --from=builder /app/database ./database/
COPY --from=builder /app/handlers ./handlers/
COPY --from=builder /app/models ./models/
COPY --from=builder /app/templates ./templates/

# 下载依赖（供动态编译使用）
RUN go mod download

# 创建数据目录
RUN mkdir -p /app/data /app/logs

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# 启动服务
CMD ["./server"]
