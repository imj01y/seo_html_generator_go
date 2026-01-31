# Go API Server Dockerfile
# 用于构建和运行 Go API 服务

# ========================================
# Stage 1: Build
# ========================================
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install git for go mod download
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/main.go

# ========================================
# Stage 2: Runtime
# ========================================
FROM alpine:3.19

WORKDIR /app

# Install ca-certificates for HTTPS and tzdata for timezone
RUN apk add --no-cache ca-certificates tzdata wget

# Copy binary from builder
COPY --from=builder /build/server .

# Copy templates
COPY --from=builder /build/templates ./templates

# Create directories
RUN mkdir -p /data/cache /app/logs /app/data

# Set environment variables
ENV TZ=Asia/Shanghai \
    GIN_MODE=release

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD wget -q --spider http://localhost:8080/health || exit 1

# Start server
CMD ["./server"]
