<template>
  <el-drawer
    v-model="visible"
    title="模板标签指南"
    direction="rtl"
    size="50%"
  >
    <div class="guide-content">
      <!-- 概述 -->
      <section class="guide-section">
        <h3>概述</h3>
        <p>模板使用 Jinja2 语法，支持变量输出、函数调用、循环和条件判断。所有标签使用 <code v-pre>{{ }}</code> 包裹，控制语句使用 <code v-pre>{% %}</code> 包裹。</p>
      </section>

      <!-- 核心标签 -->
      <section class="guide-section">
        <h3>核心标签</h3>
        <el-table :data="coreTags" border size="small">
          <el-table-column prop="tag" label="标签" width="220">
            <template #default="{ row }">
              <code>{{ row.tag }}</code>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="作用" min-width="150" />
          <el-table-column prop="example" label="输出示例" min-width="150" />
        </el-table>
      </section>

      <!-- 内容标签 -->
      <section class="guide-section">
        <h3>内容标签</h3>
        <el-table :data="contentTags" border size="small">
          <el-table-column prop="tag" label="标签" width="220">
            <template #default="{ row }">
              <code>{{ row.tag }}</code>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="作用" min-width="150" />
          <el-table-column prop="example" label="说明" min-width="150" />
        </el-table>
      </section>

      <!-- 系统标签 -->
      <section class="guide-section">
        <h3>系统标签</h3>
        <el-table :data="systemTags" border size="small">
          <el-table-column prop="tag" label="标签" width="220">
            <template #default="{ row }">
              <code>{{ row.tag }}</code>
            </template>
          </el-table-column>
          <el-table-column prop="description" label="作用" />
        </el-table>
      </section>

      <!-- 循环语法 -->
      <section class="guide-section">
        <h3>循环语法</h3>
        <p>使用 <code v-pre>{% for %}</code> 进行循环迭代：</p>
        <pre class="code-block" v-pre>{% for i in range(5) %}
  &lt;div class="{{ cls('item') }}"&gt;
    &lt;a href="{{ random_url() }}"&gt;{{ random_keyword() }}&lt;/a&gt;
  &lt;/div&gt;
{% endfor %}</pre>
        <p class="tip">range(n) 生成 0 到 n-1 的序列，常用于生成固定数量的元素</p>
      </section>

      <!-- 条件语法 -->
      <section class="guide-section">
        <h3>条件语法</h3>
        <p>使用 <code v-pre>{% if %}</code> 进行条件判断：</p>
        <pre class="code-block" v-pre>{% if site_id == 1 %}
  &lt;p&gt;这是站点1的内容&lt;/p&gt;
{% elif site_id == 2 %}
  &lt;p&gt;这是站点2的内容&lt;/p&gt;
{% else %}
  &lt;p&gt;其他站点内容&lt;/p&gt;
{% endif %}</pre>
      </section>

      <!-- CSS类名混淆 -->
      <section class="guide-section">
        <h3>CSS 类名混淆</h3>
        <p><code>cls("name")</code> 函数用于生成随机的 CSS 类名，同时保留原始名称作为标识：</p>
        <pre class="code-block" v-pre>&lt;!-- HTML 中使用 --&gt;
&lt;div class="{{ cls('header') }}"&gt;头部&lt;/div&gt;
&lt;div class="{{ cls('content') }}"&gt;内容&lt;/div&gt;

&lt;!-- 输出示例 --&gt;
&lt;div class="a7b3x9k2 header"&gt;头部&lt;/div&gt;
&lt;div class="m2n5p8q1 content"&gt;内容&lt;/div&gt;</pre>
        <p class="tip">每次页面加载时会生成不同的随机类名前缀，有助于防止样式被识别和抓取</p>
      </section>

      <!-- 文本编码 -->
      <section class="guide-section">
        <h3>文本编码</h3>
        <p><code>encode("text")</code> 函数将文本转换为 HTML 实体编码：</p>
        <pre class="code-block" v-pre>&lt;!-- 使用方式 --&gt;
&lt;span&gt;{{ encode("中文文本") }}&lt;/span&gt;

&lt;!-- 输出 --&gt;
&lt;span&gt;&amp;#x4E2D;&amp;#x6587;&amp;#x6587;&amp;#x672C;&lt;/span&gt;</pre>
        <p class="tip">编码后的文本在浏览器中显示正常，但在源代码中是实体格式，可防止简单的文本抓取</p>
      </section>

      <!-- 完整示例 -->
      <section class="guide-section">
        <h3>完整示例</h3>
        <pre class="code-block" v-pre>&lt;!DOCTYPE html&gt;
&lt;html lang="zh-CN"&gt;
&lt;head&gt;
    &lt;meta charset="UTF-8"&gt;
    &lt;title&gt;{{ title }}&lt;/title&gt;
    &lt;style&gt;
        .{{ cls('container') }} { max-width: 1200px; margin: 0 auto; }
        .{{ cls('item') }} { padding: 10px; border-bottom: 1px solid #eee; }
    &lt;/style&gt;
&lt;/head&gt;
&lt;body&gt;
    &lt;div class="{{ cls('container') }}"&gt;
        &lt;h1&gt;{{ keyword_with_emoji() }}&lt;/h1&gt;
        &lt;p&gt;发布时间：{{ now() }}&lt;/p&gt;

        {% for i in range(10) %}
        &lt;div class="{{ cls('item') }}"&gt;
            &lt;a href="{{ random_url() }}"&gt;{{ random_keyword() }}&lt;/a&gt;
            &lt;img src="{{ random_image() }}" alt=""&gt;
            &lt;span&gt;阅读量：{{ random_number(100, 9999) }}&lt;/span&gt;
        &lt;/div&gt;
        {% endfor %}

        &lt;div class="{{ cls('content') }}"&gt;
            {{ content() }}
        &lt;/div&gt;
    &lt;/div&gt;
    {{ analytics_code }}
    {{ baidu_push_js }}
&lt;/body&gt;
&lt;/html&gt;</pre>
      </section>

      <!-- 注意事项 -->
      <section class="guide-section">
        <h3>注意事项</h3>
        <ul class="feature-list">
          <li><code>random_keyword()</code> 和 <code>random_hotspot()</code> 是同一个函数的不同名称</li>
          <li><code>keyword_with_emoji()</code> 和 <code>random_keyword_emoji()</code> 是同一个函数的不同名称</li>
          <li><code>content()</code> 和 <code>content_with_pinyin()</code> 是同一个函数的不同名称</li>
          <li>所有随机函数每次调用都会返回不同的值</li>
          <li>修改模板后需要刷新缓存才能生效（或等待自动刷新）</li>
        </ul>
      </section>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const visible = ref(false)

const coreTags = [
  { tag: '{{ title }}', description: '页面标题', example: '最新科技资讯' },
  { tag: '{{ random_keyword() }}', description: '随机关键词(HTML编码)', example: '中文关键词' },
  { tag: '{{ random_hotspot() }}', description: 'random_keyword 的别名', example: '同上' },
  { tag: '{{ keyword_with_emoji() }}', description: '带 Emoji 的关键词', example: '科技新闻' },
  { tag: '{{ random_keyword_emoji() }}', description: 'keyword_with_emoji 的别名', example: '同上' },
  { tag: '{{ random_url() }}', description: '随机内链 URL', example: '/?123456789.html' },
  { tag: '{{ random_image() }}', description: '随机图片 URL', example: 'https://img.xxx.com/1.jpg' },
  { tag: '{{ random_number(min, max) }}', description: '范围内随机整数', example: '42' },
  { tag: '{{ now() }}', description: '当前时间', example: '2024-01-27 15:30:45' },
  { tag: '{{ cls("name") }}', description: '随机 CSS 类名', example: 'a7b3x9k2 name' },
  { tag: '{{ encode("text") }}', description: 'HTML 实体编码', example: '&#x6587;&#x672C;' },
]

const contentTags = [
  { tag: '{{ content() }}', description: '正文内容', example: '返回文章内容区块' },
  { tag: '{{ content_with_pinyin() }}', description: 'content 的别名', example: '同上' },
  { tag: '{{ article_content }}', description: '完整文章内容变量', example: '格式化后的文章' },
]

const systemTags = [
  { tag: '{{ site_id }}', description: '当前站点 ID' },
  { tag: '{{ analytics_code }}', description: '统计代码（如百度统计）' },
  { tag: '{{ baidu_push_js }}', description: '百度推送 JS 代码' },
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

.feature-list {
  margin: 12px 0;
  padding-left: 20px;
}

.feature-list li {
  font-size: 14px;
  color: #606266;
  line-height: 1.8;
}
</style>
