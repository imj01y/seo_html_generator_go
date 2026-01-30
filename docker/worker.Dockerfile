# Python Worker Dockerfile
# 用于运行爬虫和内容生成任务

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

# Create necessary directories
RUN mkdir -p /app/logs

# Set environment variables
ENV PYTHONUNBUFFERED=1 \
    ENV_FOR_DYNACONF=production \
    TZ=Asia/Shanghai

# Start the worker
CMD ["python", "main.py"]
