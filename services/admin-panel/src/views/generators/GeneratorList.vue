<template>
  <div class="page-container">
    <!-- 页面头部 -->
    <div class="page-header">
      <h2>生成器管理</h2>
      <el-button type="primary" @click="handleCreate">
        <el-icon><Plus /></el-icon>
        新增生成器
      </el-button>
    </div>

    <!-- 数据表格 -->
    <el-table :data="generators" v-loading="loading" stripe>
      <el-table-column prop="name" label="标识" width="120">
        <template #default="{ row }">
          <code class="code-name">{{ row.name }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="display_name" label="显示名称" width="150" />
      <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.description || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="version" label="版本" width="80" align="center">
        <template #default="{ row }">
          <el-tag size="small" type="info">v{{ row.version }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="is_default" label="默认" width="80" align="center">
        <template #default="{ row }">
          <el-icon v-if="row.is_default === 1" class="default-icon"><Star /></el-icon>
          <span v-else class="text-muted">-</span>
        </template>
      </el-table-column>
      <el-table-column prop="enabled" label="启用" width="80" align="center">
        <template #default="{ row }">
          <el-switch
            :model-value="row.enabled === 1"
            :loading="row.toggling"
            @change="handleToggle(row)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="updated_at" label="更新时间" width="170">
        <template #default="{ row }">
          {{ formatDate(row.updated_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
          <el-dropdown trigger="click" style="margin-left: 8px">
            <el-button size="small">
              更多 <el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item
                  :disabled="row.is_default === 1"
                  @click="handleSetDefault(row)"
                >
                  设为默认
                </el-dropdown-item>
                <el-dropdown-item @click="handleReload(row)">
                  <span v-if="row.reloading">重载中...</span>
                  <span v-else>热重载</span>
                </el-dropdown-item>
                <el-dropdown-item
                  divided
                  :disabled="row.is_default === 1"
                  @click="handleDelete(row)"
                  style="color: #f56c6c"
                >
                  删除
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @size-change="fetchGenerators"
        @current-change="fetchGenerators"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import {
  getGenerators,
  toggleGenerator,
  setDefaultGenerator,
  reloadGenerator,
  deleteGenerator
} from '@/api/generators'
import type { ContentGenerator } from '@/types'

const router = useRouter()
const route = useRoute()

// 状态
const loading = ref(false)
const generators = ref<(ContentGenerator & { toggling?: boolean; reloading?: boolean })[]>([])
const total = ref(0)
const currentPage = ref(Number(route.query.page) || 1)
const pageSize = ref(Number(route.query.pageSize) || 20)

// 同步分页状态到 URL
const updateUrlQuery = () => {
  router.replace({
    query: {
      ...route.query,
      page: currentPage.value.toString(),
      pageSize: pageSize.value.toString()
    }
  })
}

// 监听分页变化，同步到 URL
watch([currentPage, pageSize], () => {
  updateUrlQuery()
})

// 获取生成器列表
const fetchGenerators = async () => {
  loading.value = true
  try {
    const res = await getGenerators({
      page: currentPage.value,
      page_size: pageSize.value
    })
    generators.value = res.items
    total.value = res.total
  } finally {
    loading.value = false
  }
}

// 格式化日期
const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm')
}

// 新增
const handleCreate = () => {
  router.push('/generators/edit')
}

// 编辑
const handleEdit = (row: ContentGenerator) => {
  router.push(`/generators/edit/${row.id}`)
}

// 切换启用状态
const handleToggle = async (row: ContentGenerator & { toggling?: boolean }) => {
  row.toggling = true
  try {
    const res = await toggleGenerator(row.id)
    row.enabled = res.enabled
    ElMessage.success(res.message || '操作成功')
  } finally {
    row.toggling = false
  }
}

// 设为默认
const handleSetDefault = async (row: ContentGenerator) => {
  try {
    await ElMessageBox.confirm(
      `确定要将 "${row.display_name}" 设为默认生成器吗？`,
      '提示',
      { type: 'warning' }
    )
    await setDefaultGenerator(row.id)
    ElMessage.success('设置成功')
    fetchGenerators()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '设置失败')
    }
  }
}

// 热重载
const handleReload = async (row: ContentGenerator & { reloading?: boolean }) => {
  row.reloading = true
  try {
    const res = await reloadGenerator(row.id)
    ElMessage.success(res.message || '重载成功')
  } catch (e: any) {
    ElMessage.error(e.message || '重载失败')
  } finally {
    row.reloading = false
  }
}

// 删除
const handleDelete = async (row: ContentGenerator) => {
  await ElMessageBox.confirm(`确定要删除生成器 "${row.display_name}" 吗？`, '提示', {
    type: 'warning'
  })
  await deleteGenerator(row.id)
  ElMessage.success('删除成功')
  fetchGenerators()
}

onMounted(() => {
  fetchGenerators()
})
</script>

<style lang="scss" scoped>
.page-container {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
}

.default-icon {
  color: #e6a23c;
  font-size: 18px;
}
</style>
