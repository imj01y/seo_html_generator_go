# Vue Frontend Builder Dockerfile
# 构建前端静态文件并复制到 volume

FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files first for better caching
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy source code
COPY . .

# Build
RUN npm run build

# ========================================
# Runtime: 复制构建产物到 volume
# ========================================
FROM alpine:3.19

# 构建产物先放到 /build
COPY --from=builder /app/dist /build

# 启动时复制到 /dist (volume 挂载点)
CMD cp -r /build/* /dist/ && echo "Frontend built successfully" && tail -f /dev/null
