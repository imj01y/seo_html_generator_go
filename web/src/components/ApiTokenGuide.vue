<template>
  <el-drawer
    v-model="visible"
    title="API Token 使用指南"
    direction="rtl"
    size="50%"
  >
    <div class="guide-content">
      <!-- 认证方式 -->
      <section class="guide-section">
        <h3>认证方式</h3>
        <p>外部系统调用 API 时，需要在请求头中携带 Token 进行认证：</p>

        <h4>方式一：X-API-Token（推荐）</h4>
        <pre class="code-block">X-API-Token: {{ displayToken }}</pre>

        <h4>方式二：Authorization Bearer</h4>
        <pre class="code-block">Authorization: Bearer {{ displayToken }}</pre>

        <p class="tip">两种方式等效，任选其一即可</p>
      </section>

      <!-- 支持的接口 -->
      <section class="guide-section">
        <h3>支持的接口</h3>
        <el-table :data="apiDocs" border size="small">
          <el-table-column prop="endpoint" label="接口" width="200" />
          <el-table-column prop="method" label="方法" width="80" align="center" />
          <el-table-column prop="description" label="说明" />
          <el-table-column prop="limit" label="限制" width="120" />
        </el-table>
      </section>

      <!-- 关键词接口 -->
      <section class="guide-section">
        <h3>关键词接口</h3>

        <h4>添加单个关键词</h4>
        <pre class="code-block">curl -X POST "http://localhost:8009/api/keywords/add" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: {{ displayToken }}" \
  -d '{"group_id": 1, "keyword": "SEO优化"}'</pre>

        <h4>批量添加关键词</h4>
        <pre class="code-block">curl -X POST "http://localhost:8009/api/keywords/batch" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: {{ displayToken }}" \
  -d '{"group_id": 1, "keywords": ["关键词1", "关键词2", "关键词3"]}'</pre>

        <p class="tip">批量接口单次最多支持 10 万条关键词</p>
      </section>

      <!-- 文章接口 -->
      <section class="guide-section">
        <h3>文章接口</h3>

        <h4>添加单篇文章</h4>
        <pre class="code-block">curl -X POST "http://localhost:8009/api/articles/add" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: {{ displayToken }}" \
  -d '{
    "group_id": 1,
    "title": "文章标题",
    "content": "文章内容...",
    "author": "作者名",
    "source_url": "https://example.com/source"
  }'</pre>

        <h4>批量添加文章</h4>
        <pre class="code-block">curl -X POST "http://localhost:8009/api/articles/batch" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: {{ displayToken }}" \
  -d '{
    "articles": [
      {"group_id": 1, "title": "标题1", "content": "内容1"},
      {"group_id": 1, "title": "标题2", "content": "内容2"}
    ]
  }'</pre>

        <p class="tip">批量接口单次最多支持 1000 篇文章</p>
      </section>

      <!-- 图片接口 -->
      <section class="guide-section">
        <h3>图片接口</h3>

        <h4>添加单个图片 URL</h4>
        <pre class="code-block">curl -X POST "http://localhost:8009/api/images/urls/add" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: {{ displayToken }}" \
  -d '{"group_id": 1, "url": "https://example.com/image.jpg"}'</pre>

        <h4>批量添加图片 URL</h4>
        <pre class="code-block">curl -X POST "http://localhost:8009/api/images/urls/batch" \
  -H "Content-Type: application/json" \
  -H "X-API-Token: {{ displayToken }}" \
  -d '{
    "group_id": 1,
    "urls": [
      "https://example.com/1.jpg",
      "https://example.com/2.jpg"
    ]
  }'</pre>

        <p class="tip">批量接口单次最多支持 10 万条图片 URL</p>
      </section>

      <!-- 响应格式 -->
      <section class="guide-section">
        <h3>响应格式</h3>

        <h4>成功响应</h4>
        <pre class="code-block">// 单条添加
{"success": true, "id": 123}

// 批量添加
{"success": true, "added": 10, "failed": 0}</pre>

        <h4>错误响应</h4>
        <pre class="code-block">// 401 未认证
{"detail": "Missing API token"}

// 401 Token 无效
{"detail": "Invalid API token"}

// 403 API 已禁用
{"detail": "API token authentication is disabled"}

// 400 参数错误
{"detail": "Missing required field: keyword"}</pre>
      </section>

      <!-- 批量限制 -->
      <section class="guide-section">
        <h3>批量限制说明</h3>
        <el-table :data="limitDocs" border size="small">
          <el-table-column prop="type" label="数据类型" width="120" />
          <el-table-column prop="single" label="单条接口" />
          <el-table-column prop="batch" label="批量接口" />
        </el-table>
        <p class="tip">如需导入大量数据，建议分批调用批量接口</p>
      </section>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = defineProps<{
  token: string
}>()

const visible = ref(false)

const displayToken = computed(() => props.token || 'your_token')

const apiDocs = [
  { endpoint: '/api/keywords/add', method: 'POST', description: '添加单个关键词', limit: '-' },
  { endpoint: '/api/keywords/batch', method: 'POST', description: '批量添加关键词', limit: '10万条/次' },
  { endpoint: '/api/articles/add', method: 'POST', description: '添加单篇文章', limit: '-' },
  { endpoint: '/api/articles/batch', method: 'POST', description: '批量添加文章', limit: '1000条/次' },
  { endpoint: '/api/images/urls/add', method: 'POST', description: '添加单个图片URL', limit: '-' },
  { endpoint: '/api/images/urls/batch', method: 'POST', description: '批量添加图片URL', limit: '10万条/次' },
]

const limitDocs = [
  { type: '关键词', single: '无限制', batch: '单次最多 100,000 条' },
  { type: '文章', single: '无限制', batch: '单次最多 1,000 篇' },
  { type: '图片URL', single: '无限制', batch: '单次最多 100,000 条' },
]

function show() {
  visible.value = true
}

defineExpose({
  show
})
</script>

<style scoped>
.guide-content {
  padding: 0 20px;
}

.guide-section {
  margin-bottom: 24px;
}

.guide-section h3 {
  font-size: 16px;
  font-weight: 600;
  margin-bottom: 12px;
  color: #303133;
  border-bottom: 1px solid #e4e7ed;
  padding-bottom: 8px;
}

.guide-section h4 {
  font-size: 14px;
  font-weight: 500;
  margin: 16px 0 8px;
  color: #606266;
}

.guide-section p {
  font-size: 14px;
  color: #606266;
  line-height: 1.6;
  margin-bottom: 12px;
}

.guide-section code {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  color: #409EFF;
}

.code-block {
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 12px 16px;
  border-radius: 4px;
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 13px;
  line-height: 1.5;
  overflow-x: auto;
  white-space: pre;
  margin: 8px 0;
}

.tip {
  background: #f0f9eb;
  border-left: 4px solid #67c23a;
  padding: 8px 12px;
  margin: 12px 0;
  border-radius: 0 4px 4px 0;
}
</style>
