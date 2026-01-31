# Worker 在线代码编辑器设计

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在管理后台实现 Worker 代码在线编辑、运行测试、文件管理功能

**Architecture:** 前端使用 Element Plus + Monaco Editor，通过 Go API 操作宿主机文件系统（挂载卷），WebSocket 实时推送运行日志

**Tech Stack:** Vue 3 + Element Plus + Monaco Editor + Go Gin + WebSocket + Redis Pub/Sub + Docker

---

## 1. 功能清单

| 功能 | 说明 | 组件 |
|------|------|------|
| 文件浏览 | 列表展示目录内容 | el-table |
| 目录导航 | 面包屑路径导航 | el-breadcrumb |
| 文件编辑 | 编辑任意 .py 文件 | Monaco Editor |
| 新建 | 新建文件/目录 | el-dialog |
| 删除 | 删除文件/目录 | el-popconfirm |
| 重命名 | 重命名文件/目录 | el-dialog |
| 移动 | 移动到其他目录 | el-tree + el-dialog |
| 上传 | 上传文件到当前目录 | el-upload |
| 下载 | 下载文件 | a 标签 |
| 运行测试 | 执行当前文件，实时日志 | WebSocket |
| 重启 Worker | 优雅重启进程 | Redis Pub/Sub |
| 重新构建 | 重新构建 Docker 镜像 | docker-compose |

---

## 2. 界面设计

### 2.1 整体布局

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Worker 代码管理                        [重启 Worker] [重新构建]         │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌─ 工具栏 ──────────────────────────────────────────────────────────┐ │
│  │  📁 / worker / core                [上传] [新建文件] [新建目录]    │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  ┌─ 文件列表 ────────────────────────────────────────────────────────┐ │
│  │  名称                    大小      修改时间          操作          │ │
│  │  ───────────────────────────────────────────────────────────────  │ │
│  │  📁 processors           -         01-30 14:30       [更多 ▼]      │ │
│  │  📄 cleaner.py           2.3 KB    01-31 10:15       [更多 ▼]      │ │
│  │  📄 encoder.py           1.8 KB    01-28 09:00       [更多 ▼]      │ │
│  │  📄 title_generator.py   3.1 KB    01-29 16:45       [更多 ▼]      │ │
│  └───────────────────────────────────────────────────────────────────┘ │
│                                                                         │
│  拖拽文件到此处上传                                                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘

[更多 ▼] 下拉菜单：
  ├─ 编辑（仅文件）
  ├─ 重命名
  ├─ 移动
  ├─ 下载（仅文件）
  └─ 删除
```

### 2.2 编辑器页面

```
┌─────────────────────────────────────────────────────────────────────────┐
│  📄 core/cleaner.py                    [运行] [保存] [关闭]             │
├─────────────────────────────────────────────────────────────────────────┤
│  ┌─ Monaco Editor ───────────────────────────────────────────────────┐ │
│  │  1  import re                                                      │ │
│  │  2                                                                 │ │
│  │  3  def clean_text(text):                                          │ │
│  │  4      """清洗文本，去除HTML标签"""                                │ │
│  │  5      return re.sub(r'<[^>]+>', '', text)                        │ │
│  │  6                                                                 │ │
│  │  7  if __name__ == "__main__":                                     │ │
│  │  8      result = clean_text("<p>测试</p>")                         │ │
│  │  9      print(f"结果: {result}")                                   │ │
│  └───────────────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────────────────┤
│  运行日志                                                    [清空]     │
│  ─────────────────────────────────────────────────────────────────────  │
│  > python core/cleaner.py                                               │
│  > 结果: 测试                                                           │
│  > 进程退出，code=0，耗时 45ms                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### 2.3 移动文件弹窗

```
┌─────────────────────────────────────────────┐
│  移动到                              [×]    │
├─────────────────────────────────────────────┤
│  选择目标目录：                             │
│                                             │
│  📁 worker                                  │
│  ├─ 📁 core          ← 点击选中             │
│  ├─ 📁 crawler                              │
│  ├─ 📁 database                             │
│  └─ 📁 processors                           │
│                                             │
│  当前选择：/worker/core                     │
│                                             │
│            [取消]  [确定移动]               │
└─────────────────────────────────────────────┘
```

---

## 3. API 设计

### 3.1 文件操作 API

```
GET    /api/worker/files              - 获取目录树（用于移动弹窗）
GET    /api/worker/files/*path        - 读取文件内容 / 列出目录内容
PUT    /api/worker/files/*path        - 保存文件内容
POST   /api/worker/files/*path        - 新建文件/目录
DELETE /api/worker/files/*path        - 删除文件/目录
PATCH  /api/worker/files/*path        - 重命名/移动
```

### 3.2 上传下载 API

```
POST   /api/worker/upload/*path       - 上传文件到指定目录
GET    /api/worker/download/*path     - 下载文件
```

### 3.3 运行测试 API (WebSocket)

```
WS /api/worker/run

// 前端 → 后端
{ "action": "run", "file": "core/cleaner.py" }

// 后端 → 前端
{ "type": "stdout", "data": "输出内容" }
{ "type": "stderr", "data": "错误内容" }
{ "type": "done", "exit_code": 0, "duration_ms": 23 }
```

### 3.4 控制 API

```
POST /api/worker/restart    - 重启 Worker 进程
POST /api/worker/rebuild    - 重新构建镜像
```

---

## 4. API 响应格式

### 4.1 列出目录内容

```
GET /api/worker/files/core

Response:
{
  "path": "/core",
  "items": [
    { "name": "cleaner.py", "type": "file", "size": 2350, "mtime": "2026-01-31T10:15:00Z" },
    { "name": "encoder.py", "type": "file", "size": 1820, "mtime": "2026-01-28T09:00:00Z" },
    { "name": "processors", "type": "dir", "mtime": "2026-01-30T14:30:00Z" }
  ]
}
```

### 4.2 读取文件内容

```
GET /api/worker/files/core/cleaner.py

Response:
{
  "path": "/core/cleaner.py",
  "content": "import re\n\ndef clean_text(text):\n    ...",
  "size": 2350,
  "mtime": "2026-01-31T10:15:00Z"
}
```

### 4.3 获取目录树（用于移动弹窗）

```
GET /api/worker/files?tree=true

Response:
{
  "name": "worker",
  "path": "/",
  "type": "dir",
  "children": [
    {
      "name": "core",
      "path": "/core",
      "type": "dir",
      "children": [...]
    },
    ...
  ]
}
```

---

## 5. 前端组件设计

### 5.1 组件结构

```
src/views/worker/
├─ WorkerCodeManager.vue      # 主页面（路由入口）
├─ components/
│  ├─ FileToolbar.vue         # 工具栏（面包屑 + 操作按钮）
│  ├─ FileTable.vue           # 文件列表表格
│  ├─ FileEditor.vue          # 代码编辑器（Monaco + 日志）
│  ├─ MoveDialog.vue          # 移动文件弹窗
│  ├─ CreateDialog.vue        # 新建文件/目录弹窗
│  └─ RenameDialog.vue        # 重命名弹窗
└─ composables/
   ├─ useFileApi.ts           # 文件 API 封装
   └─ useWebSocket.ts         # WebSocket 封装
```

### 5.2 FileToolbar.vue

```vue
<template>
  <div class="file-toolbar">
    <!-- 面包屑导航 -->
    <el-breadcrumb separator="/">
      <el-breadcrumb-item
        v-for="(segment, index) in pathSegments"
        :key="index"
        @click="navigateTo(index)"
      >
        {{ segment || 'worker' }}
      </el-breadcrumb-item>
    </el-breadcrumb>

    <!-- 操作按钮 -->
    <div class="actions">
      <el-upload
        :action="`/api/worker/upload${currentPath}`"
        :show-file-list="false"
        :on-success="onUploadSuccess"
        multiple
      >
        <el-button :icon="Upload">上传</el-button>
      </el-upload>
      <el-button :icon="DocumentAdd" @click="showCreateFile">新建文件</el-button>
      <el-button :icon="FolderAdd" @click="showCreateDir">新建目录</el-button>
    </div>
  </div>
</template>
```

### 5.3 FileTable.vue

```vue
<template>
  <el-table
    :data="files"
    @row-dblclick="handleOpen"
    v-loading="loading"
  >
    <!-- 文件名 -->
    <el-table-column label="名称" min-width="200">
      <template #default="{ row }">
        <el-icon v-if="row.type === 'dir'"><Folder /></el-icon>
        <el-icon v-else><Document /></el-icon>
        <span class="file-name">{{ row.name }}</span>
      </template>
    </el-table-column>

    <!-- 大小 -->
    <el-table-column label="大小" width="100">
      <template #default="{ row }">
        {{ row.type === 'dir' ? '-' : formatSize(row.size) }}
      </template>
    </el-table-column>

    <!-- 修改时间 -->
    <el-table-column label="修改时间" width="160">
      <template #default="{ row }">
        {{ formatTime(row.mtime) }}
      </template>
    </el-table-column>

    <!-- 操作 -->
    <el-table-column label="操作" width="100" fixed="right">
      <template #default="{ row }">
        <el-dropdown @command="handleCommand($event, row)">
          <el-button text>
            更多 <el-icon><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item v-if="row.type === 'file'" command="edit">
                编辑
              </el-dropdown-item>
              <el-dropdown-item command="rename">重命名</el-dropdown-item>
              <el-dropdown-item command="move">移动</el-dropdown-item>
              <el-dropdown-item v-if="row.type === 'file'" command="download">
                下载
              </el-dropdown-item>
              <el-dropdown-item command="delete" divided>
                <span style="color: #f56c6c">删除</span>
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </template>
    </el-table-column>
  </el-table>

  <!-- 拖拽上传区域 -->
  <el-upload
    class="upload-dragger"
    drag
    :action="`/api/worker/upload${currentPath}`"
    :show-file-list="false"
    :on-success="onUploadSuccess"
    multiple
  >
    <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
    <div class="el-upload__text">拖拽文件到此处上传</div>
  </el-upload>
</template>
```

### 5.4 FileEditor.vue

```vue
<template>
  <div class="file-editor">
    <!-- 工具栏 -->
    <div class="editor-toolbar">
      <span class="file-path">📄 {{ filePath }}</span>
      <div class="actions">
        <el-button type="primary" :icon="VideoPlay" @click="runFile" :loading="running">
          运行
        </el-button>
        <el-button type="success" :icon="Check" @click="saveFile" :loading="saving">
          保存
        </el-button>
        <el-button @click="closeEditor">关闭</el-button>
      </div>
    </div>

    <!-- 编辑器 -->
    <div class="editor-container" ref="editorContainer"></div>

    <!-- 运行日志 -->
    <div class="log-panel">
      <div class="log-header">
        <span>运行日志</span>
        <el-button text @click="clearLog">清空</el-button>
      </div>
      <div class="log-content" ref="logContainer">
        <div
          v-for="(log, index) in logs"
          :key="index"
          :class="['log-line', log.type]"
        >
          {{ log.data }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import * as monaco from 'monaco-editor'
import { ref, onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  filePath: string
  content: string
}>()

const emit = defineEmits(['save', 'close'])

const editorContainer = ref<HTMLElement>()
let editor: monaco.editor.IStandaloneCodeEditor

const logs = ref<{ type: string; data: string }[]>([])
const running = ref(false)
const saving = ref(false)

onMounted(() => {
  editor = monaco.editor.create(editorContainer.value!, {
    value: props.content,
    language: 'python',
    theme: 'vs-dark',
    automaticLayout: true,
    minimap: { enabled: true },
    fontSize: 14,
    tabSize: 4,
  })
})

onUnmounted(() => {
  editor?.dispose()
})

// 运行文件
function runFile() {
  running.value = true
  logs.value = []
  logs.value.push({ type: 'info', data: `> python ${props.filePath}` })

  const ws = new WebSocket(`ws://${location.host}/api/worker/run`)

  ws.onopen = () => {
    ws.send(JSON.stringify({ action: 'run', file: props.filePath }))
  }

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data)
    if (msg.type === 'done') {
      logs.value.push({
        type: 'info',
        data: `> 进程退出，code=${msg.exit_code}，耗时 ${msg.duration_ms}ms`
      })
      running.value = false
      ws.close()
    } else {
      logs.value.push({ type: msg.type, data: msg.data })
    }
  }

  ws.onerror = () => {
    logs.value.push({ type: 'stderr', data: '连接错误' })
    running.value = false
  }
}

// 保存文件
async function saveFile() {
  saving.value = true
  const content = editor.getValue()
  emit('save', content)
  saving.value = false
}

function clearLog() {
  logs.value = []
}

function closeEditor() {
  emit('close')
}
</script>

<style scoped>
.editor-container {
  height: 60vh;
  border: 1px solid #dcdfe6;
}

.log-panel {
  margin-top: 10px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
}

.log-content {
  height: 150px;
  overflow-y: auto;
  padding: 10px;
  background: #1e1e1e;
  font-family: monospace;
  font-size: 13px;
}

.log-line {
  white-space: pre-wrap;
  line-height: 1.5;
}

.log-line.stdout { color: #d4d4d4; }
.log-line.stderr { color: #f48771; }
.log-line.info { color: #808080; }
</style>
```

### 5.5 MoveDialog.vue

```vue
<template>
  <el-dialog v-model="visible" title="移动到" width="400px">
    <p>选择目标目录：</p>
    <el-tree
      :data="dirTree"
      :props="{ label: 'name', children: 'children' }"
      node-key="path"
      highlight-current
      :expand-on-click-node="false"
      @node-click="selectDir"
      default-expand-all
    >
      <template #default="{ node, data }">
        <el-icon><Folder /></el-icon>
        <span>{{ data.name }}</span>
      </template>
    </el-tree>

    <p v-if="selectedPath" style="margin-top: 10px; color: #409eff;">
      当前选择：{{ selectedPath }}
    </p>

    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" @click="confirmMove" :disabled="!selectedPath">
        确定移动
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { getFileTree } from '@/api/worker'

const props = defineProps<{
  modelValue: boolean
  filePath: string
}>()

const emit = defineEmits(['update:modelValue', 'confirm'])

const visible = ref(props.modelValue)
const dirTree = ref([])
const selectedPath = ref('')

watch(() => props.modelValue, async (val) => {
  visible.value = val
  if (val) {
    const res = await getFileTree()
    dirTree.value = [res.data]
    selectedPath.value = ''
  }
})

watch(visible, (val) => {
  emit('update:modelValue', val)
})

function selectDir(data: any) {
  selectedPath.value = data.path
}

function confirmMove() {
  emit('confirm', selectedPath.value)
  visible.value = false
}
</script>
```

---

## 6. 后端实现

### 6.1 目录内容列表

```go
type FileInfo struct {
    Name  string    `json:"name"`
    Type  string    `json:"type"` // "file" or "dir"
    Size  int64     `json:"size,omitempty"`
    Mtime time.Time `json:"mtime"`
}

func (h *Handler) ListDir(c *gin.Context) {
    path := c.Param("path")
    fullPath := filepath.Join(h.workerDir, path)

    entries, err := os.ReadDir(fullPath)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    items := make([]FileInfo, 0, len(entries))
    for _, entry := range entries {
        info, _ := entry.Info()
        item := FileInfo{
            Name:  entry.Name(),
            Mtime: info.ModTime(),
        }
        if entry.IsDir() {
            item.Type = "dir"
        } else {
            item.Type = "file"
            item.Size = info.Size()
        }
        items = append(items, item)
    }

    c.JSON(200, gin.H{"path": path, "items": items})
}
```

### 6.2 文件上传

```go
func (h *Handler) UploadFile(c *gin.Context) {
    path := c.Param("path")
    fullPath := filepath.Join(h.workerDir, path)

    form, err := c.MultipartForm()
    if err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    files := form.File["files"]
    for _, file := range files {
        dst := filepath.Join(fullPath, file.Filename)
        if err := c.SaveUploadedFile(file, dst); err != nil {
            c.JSON(500, gin.H{"error": err.Error()})
            return
        }
    }

    c.JSON(200, gin.H{"message": fmt.Sprintf("上传 %d 个文件成功", len(files))})
}
```

### 6.3 文件下载

```go
func (h *Handler) DownloadFile(c *gin.Context) {
    path := c.Param("path")
    fullPath := filepath.Join(h.workerDir, path)

    c.FileAttachment(fullPath, filepath.Base(path))
}
```

### 6.4 移动/重命名

```go
func (h *Handler) MoveFile(c *gin.Context) {
    path := c.Param("path")
    var req struct {
        NewPath string `json:"new_path"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    oldPath := filepath.Join(h.workerDir, path)
    newPath := filepath.Join(h.workerDir, req.NewPath)

    if err := os.Rename(oldPath, newPath); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"message": "移动成功"})
}
```

### 6.5 运行测试 (WebSocket)

```go
func (h *Handler) RunWorkerFile(c *gin.Context) {
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    var req struct {
        Action string `json:"action"`
        File   string `json:"file"`
    }
    if err := conn.ReadJSON(&req); err != nil {
        return
    }

    // 执行 Python 文件
    cmd := exec.Command("python", filepath.Join(h.workerDir, req.File))
    cmd.Dir = h.workerDir

    stdout, _ := cmd.StdoutPipe()
    stderr, _ := cmd.StderrPipe()

    start := time.Now()
    if err := cmd.Start(); err != nil {
        conn.WriteJSON(map[string]interface{}{
            "type": "stderr",
            "data": err.Error(),
        })
        return
    }

    // 并发读取 stdout 和 stderr
    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        h.pipeToWS(stdout, conn, "stdout")
    }()

    go func() {
        defer wg.Done()
        h.pipeToWS(stderr, conn, "stderr")
    }()

    wg.Wait()
    cmd.Wait()

    conn.WriteJSON(map[string]interface{}{
        "type":        "done",
        "exit_code":   cmd.ProcessState.ExitCode(),
        "duration_ms": time.Since(start).Milliseconds(),
    })
}

func (h *Handler) pipeToWS(r io.Reader, conn *websocket.Conn, typ string) {
    scanner := bufio.NewScanner(r)
    for scanner.Scan() {
        conn.WriteJSON(map[string]string{
            "type": typ,
            "data": scanner.Text(),
        })
    }
}
```

### 6.6 重启 Worker

```go
func (h *Handler) RestartWorker(c *gin.Context) {
    err := h.redis.Publish(c, "worker:command", "restart").Err()
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, gin.H{"message": "重启指令已发送"})
}
```

### 6.7 重新构建

```go
func (h *Handler) RebuildWorker(c *gin.Context) {
    cmd := exec.Command("docker-compose",
        "-f", "/project/docker-compose.yml",
        "up", "-d", "--build", "worker")

    output, err := cmd.CombinedOutput()
    if err != nil {
        c.JSON(500, gin.H{
            "error":  err.Error(),
            "output": string(output),
        })
        return
    }
    c.JSON(200, gin.H{"message": "Worker 重新构建完成"})
}
```

---

## 7. Docker 配置调整

### 7.1 docker-compose.yml

```yaml
services:
  api:
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./worker:/project/worker                    # Worker 代码目录
      - ./docker-compose.yml:/project/docker-compose.yml:ro
      - /var/run/docker.sock:/var/run/docker.sock   # Docker socket
    # ...

  worker:
    build:
      context: ./worker
      dockerfile: ../docker/worker.Dockerfile
    volumes:
      - ./worker:/app                # 挂载代码目录
      - ./config.yaml:/app/config.yaml:ro
    # ...
```

### 7.2 worker.Dockerfile

```dockerfile
FROM python:3.11-slim

WORKDIR /app

# 安装依赖
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 代码通过挂载卷获取，不需要 COPY

CMD ["python", "main.py"]
```

---

## 8. Worker 命令监听

```python
# worker/core/command_listener.py

import sys
import logging
import asyncio
from redis.asyncio import Redis

logger = logging.getLogger(__name__)

class CommandListener:
    def __init__(self, redis: Redis):
        self.redis = redis
        self.should_exit = False

    async def listen(self):
        """监听 Redis 命令频道"""
        pubsub = self.redis.pubsub()
        await pubsub.subscribe("worker:command")

        logger.info("CommandListener started, waiting for commands...")

        async for message in pubsub.listen():
            if message["type"] == "message":
                command = message["data"]
                if isinstance(command, bytes):
                    command = command.decode()

                if command == "restart":
                    logger.info("收到重启指令，等待当前任务完成...")
                    self.should_exit = True

    def check_exit(self):
        """在每个任务完成后调用，检查是否需要退出"""
        if self.should_exit:
            logger.info("任务完成，退出进程...")
            sys.exit(0)
```

---

## 9. 安全考虑

- 本系统仅供内部运营人员使用
- 不对外暴露，通过内网访问
- 文件操作限制在 /worker 目录内（防止路径穿越）
- 操作日志记录到 worker_file_logs 表（可选）

---

## 10. 后续扩展（可选）

- 版本管理：保存历史版本，支持回滚
- 语法检查：保存前进行 Python 语法检查
- 自动补全：Monaco Editor 配置 Python 语言服务
- 多人协作：显示当前编辑者，防止冲突
- 终端模拟：在线执行任意命令
