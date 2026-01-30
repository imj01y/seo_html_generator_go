# ========================================
# Stage 1: Build frontend
# ========================================
FROM node:20-alpine AS frontend-builder

WORKDIR /app/admin-panel

# Copy package files first for better caching
COPY admin-panel/package*.json ./

# Install dependencies
RUN npm install

# Copy frontend source
COPY admin-panel/ ./

# Build frontend
RUN npm run build

# ========================================
# Stage 2: Python backend + frontend static files
# ========================================
FROM python:3.11.9-slim

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    default-libmysqlclient-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Copy requirements first for better caching
COPY requirements.txt ./

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy application code
COPY . .

# Copy built frontend from stage 1 (to dist-build, will be copied to shared volume at runtime)
COPY --from=frontend-builder /app/admin-panel/dist ./admin-panel/dist-build

# Create necessary directories
RUN mkdir -p /app/logs /app/cache /app/data

# Set environment variables
ENV PYTHONUNBUFFERED=1 \
    ENV_FOR_DYNACONF=production \
    TZ=Asia/Shanghai

# Expose port
EXPOSE 8009

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD python -c "import urllib.request; urllib.request.urlopen('http://localhost:8009/api/health')" || exit 1

# Start application (copy frontend to shared volume first)
CMD cp -r /app/admin-panel/dist-build/* /app/admin-panel/dist/ 2>/dev/null || true && \
    uvicorn main:app --host 0.0.0.0 --port 8009
