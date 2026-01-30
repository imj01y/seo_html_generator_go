<template>
  <el-drawer
    v-model="visible"
    title="爬虫编写指南"
    direction="rtl"
    size="50%"
  >
    <div class="guide-content">
      <!-- Feapder 风格（推荐） -->
      <section class="guide-section">
        <h3>Feapder 风格（推荐）</h3>
        <p>继承 <code>Spider</code> 类，实现 <code>start_requests</code> 和 <code>parse</code> 方法：</p>
        <pre class="code-block">from core.crawler import Spider, Request

class MySpider(Spider):
    name = "example"

    def start_requests(self):
        """生成初始请求"""
        for page in range(1, 10):
            yield Request(f"https://example.com/list?page={page}")

    def parse(self, request, response):
        """解析响应，返回数据或新请求"""
        for item in response.css('.article'):
            # 返回详情页请求
            url = item.css('a::attr(href)').get()
            yield Request(response.urljoin(url), callback=self.parse_detail)

    def parse_detail(self, request, response):
        """解析详情页"""
        yield {
            'title': response.css('h1::text').get(),
            'content': response.css('.content').get(),
            'source_url': response.url,
        }</pre>
        <p class="tip">优势：自动 URL 去重、失败重试、断点续抓、并发控制</p>
      </section>

      <!-- Request / Response -->
      <section class="guide-section">
        <h3>Request / Response</h3>

        <h4>Request 参数</h4>
        <pre class="code-block">yield Request(
    url='https://example.com/page',
    callback=self.parse_detail,  # 回调方法
    method='GET',                # HTTP 方法
    headers={'Token': 'xxx'},    # 自定义请求头
    meta={'page': 1},            # 透传数据到 Response
    priority=10,                 # 优先级（数值越大越优先）
    dont_filter=False,           # True 跳过 URL 去重
    timeout=30,                  # 超时时间（秒）
)</pre>

        <h4>Response 方法</h4>
        <pre class="code-block">def parse(self, request, response):
    # CSS 选择器
    title = response.css('h1::text').get()
    links = response.css('a::attr(href)').getall()

    # XPath
    content = response.xpath('//div[@class="content"]/text()').get()

    # JSON 解析
    data = response.json()

    # 获取透传数据
    page = response.meta.get('page', 1)

    # URL 合并
    full_url = response.urljoin('/next')

    # 跟随链接
    yield response.follow('/next', callback=self.parse)</pre>
      </section>

      <!-- Spider 高级功能 -->
      <section class="guide-section">
        <h3>Spider 高级功能</h3>
        <pre class="code-block">class MySpider(Spider):
    name = "advanced"

    # 自定义配置
    __custom_setting__ = {
        'CONCURRENT_REQUESTS': 3,
        'DOWNLOAD_DELAY': 1,
    }

    def download_midware(self, request):
        """下载中间件：修改请求或返回 None 跳过"""
        request.headers['Sign'] = calc_sign(request.url)
        return request

    def validate(self, request, response):
        """响应验证：返回 False 触发重试"""
        return response.status == 200

    def failed_request(self, request, response, e):
        """请求失败回调（超过重试次数后）"""
        logger.error(f"请求失败: {request.url}")</pre>
      </section>

      <!-- 数据格式 -->
      <section class="guide-section">
        <h3>数据格式</h3>
        <el-table :data="fieldDocs" border size="small">
          <el-table-column prop="field" label="字段" width="120" />
          <el-table-column prop="required" label="必填" width="60" align="center">
            <template #default="{ row }">
              <span :class="row.required ? 'text-success' : 'text-muted'">
                {{ row.required ? '✓' : '✗' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column prop="type" label="类型" width="80" />
          <el-table-column prop="description" label="说明" />
        </el-table>
      </section>

      <!-- 关键词爬虫 -->
      <section class="guide-section">
        <h3>关键词爬虫</h3>
        <p>抓取关键词并写入 <code>keywords</code> 表，使用 <code>type: "keywords"</code>：</p>
        <pre class="code-block">async def main_async():
    keywords = ["关键词1", "关键词2", "关键词3"]

    # yield 关键词数据，框架自动写入 keywords 表
    yield {
        "type": "keywords",
        "keywords": keywords,
        "group_id": 1  # 可选，默认使用项目配置
    }</pre>
        <p class="tip">支持批量写入，自动去重（数据库唯一索引）</p>
      </section>

      <!-- 图片爬虫 -->
      <section class="guide-section">
        <h3>图片爬虫</h3>
        <p>抓取图片URL并写入 <code>images</code> 表，使用 <code>type: "images"</code>：</p>
        <pre class="code-block">async def main_async():
    urls = ["https://example.com/img1.jpg", "https://example.com/img2.jpg"]

    # yield 图片数据，框架自动写入 images 表
    yield {
        "type": "images",
        "urls": urls,
        "group_id": 1  # 可选，默认使用项目配置
    }</pre>
        <p class="tip">支持批量写入，自动去重（数据库唯一索引）</p>
      </section>

      <!-- 数据类型汇总 -->
      <section class="guide-section">
        <h3>数据类型汇总</h3>
        <el-table :data="typeDocs" border size="small">
          <el-table-column prop="type" label="type 值" width="100" />
          <el-table-column prop="target" label="目标表" width="140" />
          <el-table-column prop="required" label="必填字段" />
        </el-table>
      </section>

      <!-- 日志输出 -->
      <section class="guide-section">
        <h3>日志输出</h3>
        <pre class="code-block">from loguru import logger

logger.info('普通信息')
logger.warning('警告信息')
logger.error('错误信息')</pre>
      </section>

      <!-- 多文件支持 -->
      <section class="guide-section">
        <h3>多文件支持</h3>
        <p>点击 [+ 新建文件] 创建额外文件，然后直接导入：</p>
        <pre class="code-block"># spider.py
from utils import clean_html
...

# utils.py
def clean_html(html):
    ...</pre>
      </section>

      <!-- 本地测试 -->
      <section class="guide-section">
        <h3>本地测试</h3>
        <pre class="code-block">if __name__ == '__main__':
    spider = MySpider()
    for req in spider.start_requests():
        print(req.url)</pre>
      </section>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const visible = ref(false)

const fieldDocs = [
  // 通用字段
  { field: 'type', required: false, type: 'str', description: '数据类型：article(默认)、keywords、images' },
  { field: 'group_id', required: false, type: 'int', description: '目标分组ID（默认使用项目配置）' },
  // 文章类型字段
  { field: 'title', required: true, type: 'str', description: '文章标题（type=article 时必填）' },
  { field: 'content', required: true, type: 'str', description: '文章内容（type=article 时必填）' },
  { field: 'source_url', required: false, type: 'str', description: '来源链接' },
  { field: 'author', required: false, type: 'str', description: '作者' },
  { field: 'publish_date', required: false, type: 'str', description: '发布日期（ISO 格式）' },
  { field: 'tags', required: false, type: 'list', description: '标签列表' },
  // 关键词类型字段
  { field: 'keywords', required: true, type: 'list', description: '关键词列表（type=keywords 时必填）' },
  // 图片类型字段
  { field: 'urls', required: true, type: 'list', description: '图片URL列表（type=images 时必填）' },
]

const typeDocs = [
  { type: 'article', target: 'original_articles', required: 'title, content' },
  { type: 'keywords', target: 'keywords', required: 'keywords (列表)' },
  { type: 'images', target: 'images', required: 'urls (列表)' },
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

.text-success {
  color: #67c23a;
  font-weight: bold;
}

.text-muted {
  color: #909399;
}

.doc-link {
  color: #409EFF;
  text-decoration: none;
  margin-left: 8px;
}

.doc-link:hover {
  text-decoration: underline;
}

.feature-list {
  margin: 12px 0;
  padding-left: 20px;
}

.tip {
  background: #f0f9eb;
  border-left: 4px solid #67c23a;
  padding: 8px 12px;
  margin: 12px 0;
  border-radius: 0 4px 4px 0;
}
</style>
