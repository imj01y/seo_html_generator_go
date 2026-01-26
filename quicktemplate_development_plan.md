# quicktemplate 性能优化开发方案

## 一、背景

### 当前性能状态
- 原始 Go html/template：~100ms
- 优化后 strings.Replacer：~15ms
- 目标 quicktemplate：~3-5ms

### 瓶颈分析
当前 strings.Replacer 方案的瓶颈在于每次请求都需要扫描 2.4MB 模板查找占位符，耗时约 10ms。

## 二、技术方案

### 2.1 quicktemplate 简介

quicktemplate 是高性能 Go 模板引擎，核心原理：
1. **编译时代码生成**：模板 → Go 源码 → 编译为二进制
2. **零反射**：所有操作都是直接函数调用
3. **直接 io.Writer**：顺序写入 buffer，无需扫描

### 2.2 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      前端/后台管理界面                        │
│                    （编辑 Jinja2 模板）                       │
└─────────────────────┬───────────────────────────────────────┘
                      │ 点击"编译"按钮
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                      编译 API 端点                           │
│                  POST /api/template/compile                  │
├─────────────────────────────────────────────────────────────┤
│  1. Jinja2 语法检测                                          │
│  2. 标签白名单检测                                           │
│  3. Jinja2 → quicktemplate 语法转换                         │
│  4. qtc 编译生成 .go 文件                                    │
│  5. go build 编译                                           │
│  6. 服务热重载                                               │
└─────────────────────┬───────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────┐
│                    编译后的 Go 服务                          │
│              （直接执行生成的渲染代码，~3-5ms）               │
└─────────────────────────────────────────────────────────────┘
```

### 2.3 Jinja2 → quicktemplate 语法转换规则

#### 完整标签清单

**一、变量输出类**

| Jinja2 语法 | quicktemplate 语法 | 说明 |
|-------------|-------------------|------|
| `{{ title }}` | `{%s p.Title %}` | 页面标题 |
| `{{ site_id }}` | `{%d p.SiteID %}` | 站点ID（整数） |
| `{{ analytics_code or '' }}` | `{%s= p.AnalyticsCode %}` | 统计代码（HTML原样输出） |
| `{{ baidu_push_js or '' }}` | `{%s= p.BaiduPushJS %}` | 百度推送JS（HTML原样输出） |
| `{{ article_content }}` | `{%s= p.ArticleContent %}` | 文章内容（HTML原样输出） |
| `{{ i }}` | `{%d i %}` | 循环变量 |

**二、无参数函数**

| Jinja2 语法 | quicktemplate 语法 | 说明 |
|-------------|-------------------|------|
| `{{ random_url() }}` | `{%s p.RandomURL() %}` | 随机URL |
| `{{ random_keyword() }}` | `{%s= p.RandomKeyword() %}` | 随机关键词（已编码） |
| `{{ random_hotspot() }}` | `{%s= p.RandomKeyword() %}` | 随机热点词（别名，映射到RandomKeyword） |
| `{{ keyword_with_emoji() }}` | `{%s= p.RandomKeyword() %}` | 带Emoji关键词（别名，映射到RandomKeyword） |
| `{{ random_image() }}` | `{%s p.RandomImage() %}` | 随机图片URL |
| `{{ now() }}` | `{%s p.Now() %}` | 当前时间 |
| `{{ content() }}` | `{%s p.Content %}` | 内容 |
| `{{ content_with_pinyin() }}` | `{%s p.Content %}` | 带拼音内容（别名，映射到Content） |

**三、带参数函数**

| Jinja2 语法 | quicktemplate 语法 | 说明 |
|-------------|-------------------|------|
| `{{ cls('header') }}` | `{%s p.Cls("header") %}` | 生成随机CSS类名+指定名称 |
| `{{ cls('') }}` | `{%s p.Cls("") %}` | 生成随机CSS类名（空参数） |
| `{{ random_number(1, 100) }}` | `{%d p.RandomNumber(1, 100) %}` | 指定范围随机整数 |
| `{{ encode('text') }}` | `{%s= p.Encode("text") %}` | HTML实体编码 |
| `{{ encode_text('text') }}` | `{%s= p.Encode("text") %}` | HTML实体编码（别名） |

**四、控制语句**

| Jinja2 语法 | quicktemplate 语法 | 说明 |
|-------------|-------------------|------|
| `{% for i in range(10) %}` | `{% for i := 0; i < 10; i++ %}` | 循环N次 |
| `{% endfor %}` | `{% endfor %}` | 结束循环 |
| `{% if condition %}` | `{% if condition %}` | 条件判断 |
| `{% elif condition %}` | `{% elseif condition %}` | 否则如果 |
| `{% else %}` | `{% else %}` | 否则 |
| `{% endif %}` | `{% endif %}` | 结束条件 |

**五、注释**

| Jinja2 语法 | quicktemplate 语法 | 说明 |
|-------------|-------------------|------|
| `{# comment #}` | `{%# comment %}` 或删除 | 注释（建议转换时直接删除） |

#### 输出类型说明

- `{%s %}` - 字符串输出，自动HTML转义
- `{%s= %}` - 字符串原样输出，不转义（用于已编码内容或HTML片段）
- `{%d %}` - 整数输出
- `{%f %}` - 浮点数输出（当前模板未使用）

#### 函数别名映射表

| 原始函数 | 别名函数 | 映射目标 |
|---------|---------|---------|
| `random_keyword()` | `random_hotspot()`, `keyword_with_emoji()` | `p.RandomKeyword()` |
| `content()` | `content_with_pinyin()` | `p.Content` |
| `encode()` | `encode_text()` | `p.Encode()` |

### 2.4 热重载方案

采用 **Graceful Restart（优雅重启）** 方案：

#### 工作原理

```
1. 编译完成后生成新的可执行文件
2. 向当前进程发送 SIGHUP 信号
3. 当前进程 fork 新进程，继承监听 socket
4. 新进程开始接受新连接
5. 旧进程停止接受新连接，处理完现有请求后退出
6. 全程服务不中断，用户无感知
```

#### 实现方式

使用 `endless` 或 `grace` 库：

```go
import "github.com/fvbock/endless"

func main() {
    router := setupRouter()

    // 使用 endless 代替标准 http.ListenAndServe
    endless.ListenAndServe(":8080", router)
}
```

#### 触发重启

```bash
# 编译完成后发送信号触发重启
kill -HUP $(pidof go-page-server)

# 或在代码中调用
syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
```

#### 编译 API 中的调用

```go
func compileHandler(c *gin.Context) {
    // 1. 语法检测
    // 2. 转换模板
    // 3. qtc 编译
    // 4. go build 生成新二进制

    // 5. 触发优雅重启
    go func() {
        time.Sleep(100 * time.Millisecond)  // 确保响应已发送
        syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
    }()

    c.JSON(200, gin.H{"success": true, "message": "编译成功，服务正在重载"})
}
```

#### 平台支持

| 平台 | 支持情况 | 说明 |
|------|---------|------|
| Linux | ✅ 完全支持 | 使用 SIGHUP 信号 |
| macOS | ✅ 完全支持 | 使用 SIGHUP 信号 |
| Windows | ⚠️ 需适配 | Windows 不支持信号，需使用命名管道或其他IPC方式 |

#### Windows 适配方案：Docker 部署

Windows 环境推荐使用 **Docker 部署**，容器内运行 Linux 环境，完全支持 Graceful Restart。

### 2.5 Docker 部署方案

#### 架构

```
Windows 宿主机
    └── Docker Desktop (WSL2)
            └── Linux 容器
                    ├── Go 服务（运行时）
                    ├── Go 工具链（编译用）
                    └── qtc 工具（模板编译）
```

#### Dockerfile

```dockerfile
# 使用包含完整工具链的镜像（支持容器内编译）
FROM golang:1.21-alpine

# 安装必要工具
RUN apk add --no-cache git

# 安装 quicktemplate 编译器
RUN go install github.com/valyala/quicktemplate/qtc@latest

WORKDIR /app

# 复制源码
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 初次编译
RUN qtc -dir=templates/ && go build -o server .

EXPOSE 8080

CMD ["./server"]
```

#### docker-compose.yml

```yaml
version: '3.8'
services:
  go-server:
    build: ./go-page-server
    ports:
      - "8080:8080"
    volumes:
      # 挂载模板目录，支持动态更新
      - ./templates:/app/templates
      # 挂载数据目录
      - ./data:/app/data
    environment:
      - GIN_MODE=release
    restart: unless-stopped
```

#### 容器内编译流程

```
POST /api/template/compile
         │
         ▼
┌─────────────────────────────────────┐
│         容器内执行                    │
├─────────────────────────────────────┤
│ 1. 保存 Jinja2 模板到文件             │
│ 2. 转换为 quicktemplate 语法         │
│ 3. 执行 qtc 编译生成 .go 文件         │
│ 4. 执行 go build 生成新二进制         │
│ 5. 发送 SIGHUP 信号触发 Graceful Restart │
└─────────────────────────────────────┘
         │
         ▼
    服务无缝重载
```

#### 编译 API 实现

```go
func compileHandler(c *gin.Context) {
    var req CompileRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    // 1. 语法检测
    if err := validateTemplate(req.Template); err != nil {
        c.JSON(400, gin.H{"error": err.Error(), "stage": "validation"})
        return
    }

    // 2. 转换语法
    qtpl := convertToQuickTemplate(req.Template)

    // 3. 保存 .qtpl 文件
    if err := os.WriteFile("templates/page.qtpl", []byte(qtpl), 0644); err != nil {
        c.JSON(500, gin.H{"error": err.Error(), "stage": "save"})
        return
    }

    // 4. 执行 qtc 编译
    cmd := exec.Command("qtc", "-dir=templates/")
    if output, err := cmd.CombinedOutput(); err != nil {
        c.JSON(500, gin.H{"error": string(output), "stage": "qtc"})
        return
    }

    // 5. 执行 go build
    cmd = exec.Command("go", "build", "-o", "server_new", ".")
    if output, err := cmd.CombinedOutput(); err != nil {
        c.JSON(500, gin.H{"error": string(output), "stage": "build"})
        return
    }

    // 6. 替换二进制文件
    os.Rename("server_new", "server")

    // 7. 触发 Graceful Restart
    c.JSON(200, gin.H{"success": true, "message": "编译成功，服务正在重载"})

    go func() {
        time.Sleep(100 * time.Millisecond)
        syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
    }()
}

### 2.6 一键部署方案（Vue + Python + Go）

#### 设计目标

1. **一键部署**：`docker-compose up -d` 启动整个系统
2. **无缝替换**：Go 接管 /page 接口，Python 专注管理后台
3. **统一入口**：Nginx 统一分发请求

#### 部署架构

```
                         ┌─────────────────────────────┐
                         │      Nginx 容器 (:80)       │
                         │     (新增，统一入口)         │
                         └──────────────┬──────────────┘
                                        │
          ┌─────────────────────────────┼─────────────────────────────┐
          │                             │                             │
          ▼                             ▼                             ▼
   /page/* 请求                  /api/* 请求                    静态资源
          │                             │                             │
          ▼                             ▼                             ▼
┌─────────────────┐          ┌─────────────────┐          ┌─────────────────┐
│   Go 容器       │          │  Python 容器    │          │   Vue 静态文件   │
│   (新增)        │          │   (现有)        │          │   (现有)         │
│                 │          │                 │          │                 │
│ - /page 渲染    │          │ - 管理后台 API  │          │ - 前端界面      │
│ - /api/go/*     │          │ - 数据 CRUD     │          │                 │
│   (编译API)     │          │ - 用户认证      │          │                 │
└────────┬────────┘          └────────┬────────┘          └─────────────────┘
         │                            │
         └────────────┬───────────────┘
                      ▼
              ┌─────────────┐
              │   MySQL     │
              │  (现有)      │
              └─────────────┘
```

#### 现有项目结构

```
seo_html_generator/
├── docker-compose.yml          # 现有配置（需要添加 go-server）
├── Dockerfile                  # Python 后端 Dockerfile
├── main.py                     # Python 后端入口
├── core/                       # Python 核心逻辑
├── api/                        # Python API 路由
├── admin-panel/                # Vue 前端
│   ├── src/
│   └── dist/                   # 构建产物
├── go-page-server/             # Go 服务（已存在）
│   ├── main.go
│   ├── core/
│   └── handlers/
├── docker/
│   └── nginx/
│       ├── nginx.conf
│       └── conf.d/
│           └── default.conf    # 需要修改，添加 Go 路由
├── database/
│   └── schema.sql
└── ...
```

#### 现有服务配置

| 服务 | 容器名 | 端口 | 说明 |
|------|--------|------|------|
| nginx | seo-generator-nginx | 8009, 8008 | 反向代理 |
| app | seo-generator-app | expose 8009 | Python 后端 |
| mysql | seo-generator-mysql | 3306 | 数据库 |
| redis | seo-generator-redis | 6379 | 缓存 |

#### 修改方案：在现有 docker-compose.yml 中添加 Go 服务

在 `services:` 下添加以下内容：

```yaml
  # ========================================
  # Go Page Server (quicktemplate)
  # ========================================
  go-server:
    build:
      context: ./go-page-server
      dockerfile: Dockerfile
    container_name: seo-generator-go
    restart: unless-stopped
    expose:
      - "8080"
    environment:
      # 使用与 Python 相同的数据库配置
      - DB_HOST=mysql
      - DB_PORT=3306
      - DB_USER=root
      - DB_PASSWORD=mysql_6yh7uJ
      - DB_NAME=seo_generator
      # Redis (可选)
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - REDIS_PASSWORD=redis_6yh7uJ
      # App
      - GIN_MODE=release
      - TZ=Asia/Shanghai
    volumes:
      - ./go-page-server/templates:/app/templates
      - ./go-page-server/data:/app/data
      - ./logs/go:/app/logs
    depends_on:
      mysql:
        condition: service_healthy
    networks:
      - seo-network
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 30s
```

并修改 nginx 服务的 depends_on：

```yaml
  nginx:
    # ... 其他配置不变 ...
    depends_on:
      app:
        condition: service_healthy
      go-server:                    # 新增
        condition: service_healthy  # 新增
```

#### 修改 Nginx 配置 (docker/nginx/nginx.conf)

在 `http {}` 块中添加 Go 上游：

```nginx
    # 上游服务定义
    upstream fastapi_backend {
        server app:8009;
        keepalive 32;
    }

    # 新增：Go 服务上游
    upstream go_backend {
        server go-server:8080;
        keepalive 32;
    }
```

#### 修改 Nginx 站点配置 (docker/nginx/conf.d/default.conf)

在 `location /api/` 之前添加 `/page` 路由：

```nginx
    # ==========================================
    # Go 服务路由 (quicktemplate 高性能渲染)
    # ==========================================

    # /page 路由到 Go 服务
    location /page {
        proxy_pass http://go_backend;
        proxy_http_version 1.1;

        proxy_connect_timeout 10s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Spider $is_spider;
        proxy_set_header Connection "";

        proxy_buffering on;
        proxy_buffer_size 8k;
        proxy_buffers 16 8k;

        add_header X-Served-By "go-server" always;
    }

    # Go 编译 API
    location /api/go/ {
        proxy_pass http://go_backend/api/;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Connection "";
    }
```

#### 新增 Nginx 配置文件 (nginx/nginx.conf)

```nginx
events {
    worker_connections 1024;
}

http {
    include       /etc/nginx/mime.types;
    default_type  application/octet-stream;

    sendfile        on;
    keepalive_timeout  65;
    gzip  on;
    gzip_types text/plain text/css application/json application/javascript text/xml;

    # 上游服务器（使用现有 docker-compose 中的服务名）
    upstream go_backend {
        server go-server:8080;      # 新增的 Go 服务
    }

    upstream python_backend {
        server python-server:5000;  # 现有的 Python 服务（根据实际名称调整）
    }

    server {
        listen 80;
        server_name _;

        # /page 路由到 Go 服务
        location /page {
            proxy_pass http://go_backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_read_timeout 30s;
        }

        # Go 服务的编译 API
        location /api/go/ {
            proxy_pass http://go_backend/api/;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }

        # Python 管理后台 API（现有接口）
        location /api/ {
            proxy_pass http://python_backend;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }

        # Vue 静态资源（路径根据现有结构调整）
        location / {
            root /var/www/html;
            index index.html;
            try_files $uri $uri/ /index.html;
        }
    }
}
```

#### 一键部署步骤

```bash
# 1. 构建 Vue 前端（如果有修改）
cd admin-panel && npm run build && cd ..

# 2. 一键启动所有服务（包括新增的 Go 服务）
docker-compose up -d --build

# 3. 查看服务状态
docker-compose ps

# 4. 查看日志
docker-compose logs -f go-server  # 查看 Go 服务日志

# 5. 验证服务
# Go 处理 /page
curl "http://localhost:8009/page?path=/test.html&domain=example.com"

# Python 处理 /api
curl "http://localhost:8009/api/health"

# Go 编译 API
curl -X POST "http://localhost:8009/api/go/template/compile" \
     -H "Content-Type: application/json" \
     -d '{"template_id": 1}'
```

#### 服务分工说明

| 路径 | 处理服务 | 说明 |
|------|---------|------|
| `/page` | **Go 服务** | 高性能页面渲染（quicktemplate） |
| `/api/go/*` | **Go 服务** | 模板编译 API |
| `/api/*` | Python 服务 | 管理后台 API（CRUD、认证等） |
| `/` | Nginx | HTML 缓存 → 回源 Python |
| `/static/*` | Nginx | 静态资源 |

#### Python /page 接口处理

当前 Python 的 `/page` 是通过 Nginx 的 `location /` 回源处理的。
添加 Go 服务后，需要在 Nginx 中添加 `/page` 路由优先到 Go，这样：

1. `/page` 请求 → Go 服务处理（高性能）
2. 其他路径（如 `/123.html`）→ 先查缓存，miss 则回源 Python（保持兼容）

**无需修改 Python 代码**，只需修改 Nginx 配置即可实现无缝切换。

### 2.7 前端模板编辑与编译功能

#### 完整交互流程

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Vue 前端 (admin-panel)                          │
├─────────────────────────────────────────────────────────────────────────┤
│  1. 用户编辑模板内容（Jinja2 语法）                                       │
│  2. 点击"保存"按钮 → 调用 Python API 保存到数据库                         │
│  3. 点击"编译"按钮 → 调用 Go 编译 API                                    │
│  4. 显示编译结果（成功/失败+错误信息）                                    │
│  5. 编译成功后，Go 服务自动热重载                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    ▼                               ▼
        ┌─────────────────────┐         ┌─────────────────────┐
        │    Python API       │         │      Go API         │
        │  POST /api/templates│         │ POST /api/go/compile│
        │  (保存到数据库)      │         │  (编译+热重载)       │
        └─────────────────────┘         └─────────────────────┘
```

#### Vue 前端实现

**1. 模板编辑页面组件 (TemplateEditor.vue)**

```vue
<template>
  <div class="template-editor">
    <!-- 模板选择 -->
    <div class="header">
      <el-select v-model="selectedTemplateId" @change="loadTemplate">
        <el-option
          v-for="t in templates"
          :key="t.id"
          :label="t.display_name"
          :value="t.id"
        />
      </el-select>
    </div>

    <!-- 代码编辑器 -->
    <div class="editor-container">
      <MonacoEditor
        v-model="templateContent"
        language="html"
        :options="editorOptions"
      />
    </div>

    <!-- 操作按钮 -->
    <div class="actions">
      <el-button type="primary" @click="saveTemplate" :loading="saving">
        保存
      </el-button>
      <el-button type="success" @click="compileTemplate" :loading="compiling">
        编译并发布
      </el-button>
      <el-button @click="previewTemplate">预览</el-button>
    </div>

    <!-- 编译结果 -->
    <div v-if="compileResult" class="compile-result" :class="compileResult.success ? 'success' : 'error'">
      <h4>{{ compileResult.success ? '编译成功' : '编译失败' }}</h4>
      <p v-if="compileResult.stage">阶段: {{ compileResult.stage }}</p>
      <pre v-if="compileResult.error">{{ compileResult.error }}</pre>
      <p v-if="compileResult.success">服务正在热重载，约 2 秒后生效...</p>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import * as api from '@/api/template'

const templates = ref([])
const selectedTemplateId = ref(null)
const templateContent = ref('')
const saving = ref(false)
const compiling = ref(false)
const compileResult = ref(null)

// 加载模板列表
onMounted(async () => {
  templates.value = await api.getTemplateList()
})

// 加载选中的模板
async function loadTemplate() {
  const template = await api.getTemplate(selectedTemplateId.value)
  templateContent.value = template.content
}

// 保存模板到数据库
async function saveTemplate() {
  saving.value = true
  try {
    await api.saveTemplate(selectedTemplateId.value, templateContent.value)
    ElMessage.success('保存成功')
  } catch (e) {
    ElMessage.error('保存失败: ' + e.message)
  } finally {
    saving.value = false
  }
}

// 编译模板
async function compileTemplate() {
  compiling.value = true
  compileResult.value = null

  try {
    // 先保存
    await api.saveTemplate(selectedTemplateId.value, templateContent.value)

    // 再编译
    const result = await api.compileTemplate(selectedTemplateId.value)
    compileResult.value = result

    if (result.success) {
      ElMessage.success('编译成功，服务正在热重载')
    } else {
      ElMessage.error('编译失败: ' + result.error)
    }
  } catch (e) {
    compileResult.value = { success: false, error: e.message }
    ElMessage.error('编译请求失败')
  } finally {
    compiling.value = false
  }
}

// 预览
function previewTemplate() {
  window.open(`/page?preview=1&template_id=${selectedTemplateId.value}`, '_blank')
}
</script>
```

**2. API 调用模块 (api/template.js)**

```javascript
const API_BASE = ''  // 通过 Nginx 代理

// 获取模板列表（Python API）
export async function getTemplateList() {
  const res = await fetch(`${API_BASE}/api/templates`)
  return res.json()
}

// 获取单个模板（Python API）
export async function getTemplate(id) {
  const res = await fetch(`${API_BASE}/api/templates/${id}`)
  return res.json()
}

// 保存模板（Python API）
export async function saveTemplate(id, content) {
  const res = await fetch(`${API_BASE}/api/templates/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content })
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

// 编译模板（Go API）
export async function compileTemplate(templateId) {
  const res = await fetch(`${API_BASE}/api/go/template/compile`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ template_id: templateId })
  })
  return res.json()
}
```

#### Go 编译 API 实现

**1. 编译 Handler (handlers/compile.go)**

```go
package handlers

import (
    "net/http"
    "os"
    "os/exec"
    "syscall"
    "time"

    "github.com/gin-gonic/gin"
    "go-page-server/core"
)

type CompileRequest struct {
    TemplateID int `json:"template_id" binding:"required"`
}

type CompileResponse struct {
    Success bool   `json:"success"`
    Message string `json:"message,omitempty"`
    Error   string `json:"error,omitempty"`
    Stage   string `json:"stage,omitempty"`
}

func (h *PageHandler) CompileTemplate(c *gin.Context) {
    var req CompileRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, CompileResponse{
            Success: false,
            Error:   "无效的请求参数",
            Stage:   "参数解析",
        })
        return
    }

    // 1. 从数据库获取模板
    template, err := h.dataManager.GetTemplateByID(req.TemplateID)
    if err != nil {
        c.JSON(http.StatusNotFound, CompileResponse{
            Success: false,
            Error:   "模板不存在",
            Stage:   "加载模板",
        })
        return
    }

    // 2. Jinja2 语法检测
    if err := core.ValidateJinja2Syntax(template.Content); err != nil {
        c.JSON(http.StatusBadRequest, CompileResponse{
            Success: false,
            Error:   err.Error(),
            Stage:   "Jinja2 语法检测",
        })
        return
    }

    // 3. 函数白名单检测
    if err := core.ValidateFunctions(template.Content); err != nil {
        c.JSON(http.StatusBadRequest, CompileResponse{
            Success: false,
            Error:   err.Error(),
            Stage:   "函数白名单检测",
        })
        return
    }

    // 4. 转换为 quicktemplate 语法
    qtplContent := core.ConvertToQuickTemplate(template.Content, template.Name)

    // 5. 保存 .qtpl 文件
    qtplPath := "templates/" + template.Name + ".qtpl"
    if err := os.WriteFile(qtplPath, []byte(qtplContent), 0644); err != nil {
        c.JSON(http.StatusInternalServerError, CompileResponse{
            Success: false,
            Error:   "保存模板文件失败: " + err.Error(),
            Stage:   "保存文件",
        })
        return
    }

    // 6. 执行 qtc 编译
    cmd := exec.Command("qtc", "-dir=templates/")
    output, err := cmd.CombinedOutput()
    if err != nil {
        c.JSON(http.StatusInternalServerError, CompileResponse{
            Success: false,
            Error:   string(output),
            Stage:   "quicktemplate 编译",
        })
        return
    }

    // 7. 执行 go build
    cmd = exec.Command("go", "build", "-o", "server_new", ".")
    output, err = cmd.CombinedOutput()
    if err != nil {
        c.JSON(http.StatusInternalServerError, CompileResponse{
            Success: false,
            Error:   string(output),
            Stage:   "Go 编译",
        })
        return
    }

    // 8. 替换二进制
    if err := os.Rename("server_new", "server"); err != nil {
        c.JSON(http.StatusInternalServerError, CompileResponse{
            Success: false,
            Error:   "替换二进制失败: " + err.Error(),
            Stage:   "替换文件",
        })
        return
    }

    // 9. 返回成功，然后触发热重载
    c.JSON(http.StatusOK, CompileResponse{
        Success: true,
        Message: "编译成功，服务正在热重载",
    })

    // 10. 异步触发 Graceful Restart
    go func() {
        time.Sleep(100 * time.Millisecond) // 确保响应已发送
        syscall.Kill(syscall.Getpid(), syscall.SIGHUP)
    }()
}
```

**2. 路由注册 (main.go 修改)**

```go
// 在路由设置中添加
api := r.Group("/api")
{
    api.POST("/template/compile", handler.CompileTemplate)
    api.GET("/health", handler.Health)
}
```

**3. Graceful Restart 支持 (main.go 修改)**

```go
package main

import (
    "github.com/fvbock/endless"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()

    // 设置路由...
    setupRoutes(r)

    // 使用 endless 支持 Graceful Restart
    // 收到 SIGHUP 信号时会优雅重启
    endless.ListenAndServe(":8080", r)
}
```

#### Go Dockerfile（支持容器内编译）

```dockerfile
# 使用包含完整工具链的镜像
FROM golang:1.21-alpine

# 安装必要工具
RUN apk add --no-cache git gcc musl-dev

# 安装 quicktemplate 编译器
RUN go install github.com/valyala/quicktemplate/qtc@latest

# 安装 endless（Graceful Restart）
RUN go install github.com/fvbock/endless@latest

WORKDIR /app

# 复制依赖文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 初次编译 quicktemplate
RUN if [ -d "templates" ] && [ "$(ls -A templates/*.qtpl 2>/dev/null)" ]; then \
        qtc -dir=templates/; \
    fi

# 编译应用
RUN go build -o server .

EXPOSE 8080

# 启动服务
CMD ["./server"]
```

#### 完整编译流程时序图

```
用户                Vue 前端              Python API           Go API              Go 服务
 │                    │                      │                   │                   │
 │  编辑模板内容       │                      │                   │                   │
 │───────────────────>│                      │                   │                   │
 │                    │                      │                   │                   │
 │  点击"保存"        │                      │                   │                   │
 │───────────────────>│                      │                   │                   │
 │                    │  PUT /api/templates  │                   │                   │
 │                    │─────────────────────>│                   │                   │
 │                    │       200 OK         │                   │                   │
 │                    │<─────────────────────│                   │                   │
 │      保存成功      │                      │                   │                   │
 │<───────────────────│                      │                   │                   │
 │                    │                      │                   │                   │
 │  点击"编译"        │                      │                   │                   │
 │───────────────────>│                      │                   │                   │
 │                    │         POST /api/go/template/compile    │                   │
 │                    │─────────────────────────────────────────>│                   │
 │                    │                      │                   │                   │
 │                    │                      │     1. 从数据库获取模板               │
 │                    │                      │                   │──────────────────>│
 │                    │                      │                   │<──────────────────│
 │                    │                      │     2. 语法检测   │                   │
 │                    │                      │     3. 转换语法   │                   │
 │                    │                      │     4. qtc 编译   │                   │
 │                    │                      │     5. go build   │                   │
 │                    │                      │     6. 返回结果   │                   │
 │                    │        { success: true, message: "..." } │                   │
 │                    │<─────────────────────────────────────────│                   │
 │     编译成功       │                      │     7. SIGHUP 热重载                  │
 │<───────────────────│                      │                   │─ ─ ─ ─ ─ ─ ─ ─ ─>│
 │                    │                      │                   │     新进程启动    │
 │                    │                      │                   │     旧进程退出    │
```

#### 错误处理与用户提示

**编译错误示例展示**：

```json
// 语法错误
{
  "success": false,
  "stage": "Jinja2 语法检测",
  "error": "第 15 行：变量标签 {{ 未正确闭合"
}

// 未定义函数
{
  "success": false,
  "stage": "函数白名单检测",
  "error": "第 23 行：未定义的函数 'random_color'，允许的函数: cls, random_url, random_keyword, random_image, random_number, encode, now, content"
}

// quicktemplate 编译错误
{
  "success": false,
  "stage": "quicktemplate 编译",
  "error": "templates/download_site.qtpl:45: unexpected token..."
}

// Go 编译错误
{
  "success": false,
  "stage": "Go 编译",
  "error": "undefined: RandomColor"
}
```

#### Vue 前端调用示例

```javascript
// api.js

// 调用 Go 服务的模板编译 API
export async function compileTemplate(templateId, templateContent) {
    const response = await fetch('/api/go/template/compile', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${getToken()}`  // 如需认证
        },
        body: JSON.stringify({
            id: templateId,
            template: templateContent
        })
    });
    return response.json();
}

// 调用 Python 服务的管理 API
export async function getTemplateList() {
    const response = await fetch('/api/templates', {
        headers: {
            'Authorization': `Bearer ${getToken()}`
        }
    });
    return response.json();
}

// 预览页面（调用 Go 渲染）
export function previewPage(path) {
    window.open(`/page?path=${path}&preview=1`, '_blank');
}
```

#### 目录结构

```
seo_html_generator/
├── docker-compose.yml
├── .env                      # 环境变量（DB_PASSWORD 等）
├── nginx/
│   └── nginx.conf
├── go-page-server/           # Go 服务
│   ├── Dockerfile
│   ├── main.go
│   ├── core/
│   └── templates/
├── python-backend/           # Python 服务
│   ├── Dockerfile
│   ├── app.py
│   └── ...
├── vue-frontend/             # Vue 源码
│   └── ...
├── vue-dist/                 # Vue 构建产物（由 CI 或手动构建）
│   ├── index.html
│   └── assets/
└── mysql/
    └── init.sql
```

#### 启动命令

```bash
# 构建并启动所有服务
docker-compose up -d --build

# 查看日志
docker-compose logs -f

# 只重启 Go 服务（模板编译后）
docker-compose restart go-server

# 进入 Go 容器调试
docker-compose exec go-server sh
```

## 三、模板语法检测

### 3.1 检测层次

```
第1层：Jinja2 语法检测
  ├── {{ }} 标签配对检查
  ├── {% %} 标签配对检查
  └── for/endfor、if/endif 配对检查

第2层：标签白名单检测
  └── 只允许预定义的函数调用

第3层：qtc 编译检测
  └── quicktemplate 语法错误

第4层：go build 检测
  └── 未定义函数、类型错误
```

### 3.2 允许的函数白名单

```go
var allowedFunctions = map[string]bool{
    // 核心函数
    "cls":            true,  // CSS 类名生成
    "random_url":     true,  // 随机 URL
    "random_keyword": true,  // 随机关键词
    "random_image":   true,  // 随机图片
    "random_number":  true,  // 随机数字
    "encode":         true,  // HTML 实体编码
    "content":        true,  // 内容函数
    "now":            true,  // 当前时间

    // 别名函数（映射到核心函数）
    "random_hotspot":      true,  // → random_keyword
    "keyword_with_emoji":  true,  // → random_keyword
    "content_with_pinyin": true,  // → content
    "encode_text":         true,  // → encode
}

// 允许的变量白名单
var allowedVariables = map[string]bool{
    "title":           true,  // 页面标题
    "site_id":         true,  // 站点ID
    "analytics_code":  true,  // 统计代码
    "baidu_push_js":   true,  // 百度推送JS
    "article_content": true,  // 文章内容
    "i":               true,  // 循环变量
}
```

### 3.3 错误响应格式

```json
{
    "success": false,
    "stage": "Jinja2 语法检测",
    "error": "第 15 行：变量标签 {{ }} 未正确闭合",
    "line": 15,
    "suggestion": "请检查第 15 行的 {{ 是否有对应的 }}"
}
```

## 四、性能对比

| 指标 | 当前方案 (Replacer) | quicktemplate | 提升 |
|------|-------------------|---------------|------|
| 渲染时间 | 15ms | 3-5ms | **3-5x** |
| 单线程 QPS | 66 | 200-333 | **3-5x** |
| 8核 QPS | 530 | 1600-2600 | **3-5x** |
| 内存分配/次 | 多次 | ~1次 | 显著减少 |

## 五、实现步骤

### 阶段一：PoC 验证（优先）

1. **安装 quicktemplate**
   ```bash
   go install github.com/valyala/quicktemplate/qtc@latest
   ```

2. **创建简化测试模板**
   - 手动编写一个简化版的 quicktemplate 模板
   - 包含核心功能：循环、cls、random_url 等

3. **编写 Benchmark 对比测试**
   - 对比 strings.Replacer vs quicktemplate
   - 验证性能提升是否达到预期

### 阶段二：语法转换器

1. **新增文件**：`core/jinja2_to_quicktemplate.go`
   - 实现 Jinja2 → quicktemplate 语法转换
   - 复用部分现有 template_converter.go 逻辑

2. **转换规则实现**
   - 变量输出转换
   - 循环语句转换
   - 条件语句转换
   - 函数调用转换

### 阶段三：语法检测

1. **新增文件**：`core/template_validator.go`
   - Jinja2 语法检测
   - 函数白名单检测
   - 错误信息格式化

### 阶段四：编译 API

1. **新增文件**：`handlers/compile.go`
   - POST /api/template/compile 端点
   - 调用语法检测
   - 调用转换器
   - 调用 qtc 和 go build
   - 触发热重载

### 阶段五：热重载机制

1. **修改文件**：`main.go`
   - 添加优雅退出信号处理
   - 支持外部触发重启

2. **服务管理配置**
   - Windows: NSSM 配置
   - Linux: systemd 配置

### 阶段六：集成测试

1. 完整流程测试
2. 错误处理测试
3. 性能回归测试

## 六、文件结构变化

```
go-page-server/
├── core/
│   ├── jinja2_to_quicktemplate.go  (新增) 语法转换器
│   ├── template_validator.go        (新增) 语法检测
│   ├── quicktemplate_renderer.go    (新增) QT 渲染器
│   └── ... 现有文件
├── handlers/
│   ├── compile.go                   (新增) 编译 API
│   └── ... 现有文件
├── templates/                        (新增) quicktemplate 模板目录
│   └── page.qtpl                     生成的 qtpl 文件
├── main.go                           (修改) 添加热重载支持
└── ...
```

## 七、回滚方案

如果 quicktemplate 方案出现问题，可以快速回滚：

1. 保留现有 strings.Replacer 方案代码
2. 通过配置开关切换渲染方式
3. 回滚只需修改配置，无需重新部署

```go
// config.yaml
renderer:
  type: "quicktemplate"  # 或 "replacer"
```

## 八、验证方法

```bash
# 1. 启动服务
./go-page-server

# 2. 发送测试请求
curl "http://localhost:8080/page?ua=Baiduspider&path=/test.html&domain=example.com"

# 3. 查看日志中的 render_time
# 目标：render_time < 10ms
```

## 九、风险与注意事项

1. **编译时间**：qtc + go build 可能需要几秒到几十秒
2. **热重载中断**：简单重启方案有 1-2 秒中断
3. **模板兼容性**：部分复杂 Jinja2 语法可能需要特殊处理
4. **调试难度**：编译后的代码比模板更难调试

## 十、后续优化方向

1. **实时预览**：编辑时实时预览渲染效果
2. **增量编译**：只重新编译变化的模板
3. **多模板支持**：支持多个模板的独立编译和加载
