<template>
  <div class="group-list-page site-list">
    <div class="page-header">
      <h2 class="title">站点管理</h2>
      <el-button type="primary" @click="handleAdd">
        <el-icon><Plus /></el-icon>
        新增站点
      </el-button>
    </div>

    <div class="page-container">
      <!-- 左侧边栏：分组管理 -->
      <aside class="group-sidebar">
        <div class="sidebar-header">
          <el-button type="primary" size="small" @click="groupDialogVisible = true" style="width: 100%">
            <el-icon><Plus /></el-icon> 新建分组
          </el-button>
        </div>
        <div class="sidebar-search">
          <el-input v-model="groupSearch" placeholder="搜索分组" size="small" clearable>
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
        </div>
        <div class="sidebar-list">
          <div
            v-for="group in filteredGroups"
            :key="group.id"
            class="group-item"
            :class="{ active: activeGroupId === group.id }"
            @click="selectGroup(group.id)"
            @contextmenu.prevent="showContextMenu($event, group)"
          >
            <el-icon><Folder /></el-icon>
            <span class="group-name">{{ group.name }}</span>
            <el-tag v-if="group.is_default" size="small" type="info">默认</el-tag>
          </div>
          <div v-if="filteredGroups.length === 0" class="empty-tip">
            暂无分组
          </div>
        </div>
      </aside>

      <!-- 右侧内容区 -->
      <main class="content-area">
        <!-- 批量操作栏 -->
        <transition name="fade">
          <div v-if="selectedItems.length > 0" class="batch-actions">
            <span class="selected-count">已选中 {{ selectedItems.length }} 项</span>
            <el-button size="small" type="danger" @click="handleBatchDelete">
              <el-icon><Delete /></el-icon> 删除
            </el-button>
            <el-button size="small" type="success" @click="handleBatchEnable">
              <el-icon><Check /></el-icon> 启用
            </el-button>
            <el-button size="small" type="warning" @click="handleBatchDisable">
              <el-icon><Close /></el-icon> 禁用
            </el-button>
            <el-button size="small" link @click="clearSelection">取消选择</el-button>
          </div>
        </transition>

        <!-- 搜索栏 -->
        <el-form :inline="true" class="search-form">
          <el-form-item>
            <el-input
              v-model="searchDomain"
              placeholder="搜索域名"
              clearable
              @clear="loadSites"
              @keyup.enter="loadSites"
            />
          </el-form-item>
          <el-form-item>
            <el-select v-model="searchStatus" placeholder="状态" clearable @change="loadSites" style="width: 100px">
              <el-option label="启用" :value="1" />
              <el-option label="禁用" :value="0" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" @click="loadSites">搜索</el-button>
          </el-form-item>
        </el-form>

        <!-- 表格 -->
        <el-table
          ref="tableRef"
          :data="filteredSites"
          v-loading="loading"
          stripe
          @selection-change="handleSelectionChange"
          class="data-table"
        >
          <el-table-column type="selection" width="55" />
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="domain" label="域名" min-width="150">
            <template #default="{ row }">
              <el-link :href="`http://${row.domain}`" target="_blank" type="primary">
                {{ row.domain }}
              </el-link>
            </template>
          </el-table-column>
          <el-table-column prop="name" label="站点名称" min-width="100" />
          <el-table-column prop="template" label="模板" width="140" />
          <el-table-column prop="keyword_group_id" label="词库" width="105">
            <template #default="{ row }">
              <el-tag v-if="row.keyword_group_id" size="small">
                {{ getGroupName(keywordGroups, row.keyword_group_id) }}
              </el-tag>
              <span v-else class="text-muted">默认</span>
            </template>
          </el-table-column>
          <el-table-column prop="image_group_id" label="图库" width="95">
            <template #default="{ row }">
              <el-tag v-if="row.image_group_id" size="small" type="success">
                {{ getGroupName(imageGroups, row.image_group_id) }}
              </el-tag>
              <span v-else class="text-muted">默认</span>
            </template>
          </el-table-column>
          <el-table-column prop="article_group_id" label="文章库" width="95">
            <template #default="{ row }">
              <el-tag v-if="row.article_group_id" size="small" type="info">
                {{ getArticleGroupName(row.article_group_id) }}
              </el-tag>
              <span v-else class="text-muted">默认</span>
            </template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="75">
            <template #default="{ row }">
              <el-switch
                v-model="row.status"
                :active-value="1"
                :inactive-value="0"
                @change="handleStatusChange(row)"
              />
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="创建时间" width="160">
            <template #default="{ row }">
              {{ formatDate(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="160">
            <template #default="{ row }">
              <el-button type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
              <el-button type="danger" size="small" @click="handleDelete(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>

        <!-- 分页 -->
        <div class="pagination-wrapper" v-if="total > 0">
          <el-pagination
            v-model:current-page="currentPage"
            v-model:page-size="pageSize"
            :page-sizes="[20, 50, 100]"
            :total="total"
            layout="total, sizes, prev, pager, next"
            @size-change="loadSites"
            @current-change="loadSites"
          />
        </div>
      </main>
    </div>

    <!-- 新增/编辑站点弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑站点' : '新增站点'"
      width="600px"
      @close="resetForm"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="100px"
      >
        <el-form-item label="所属分组" prop="site_group_id">
          <el-select v-model="form.site_group_id" style="width: 100%">
            <el-option
              v-for="group in siteGroups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="域名" prop="domain">
          <el-input v-model="form.domain" placeholder="example.com" :disabled="isEdit" />
        </el-form-item>
        <el-form-item label="站点名称" prop="name">
          <el-input v-model="form.name" placeholder="站点名称" />
        </el-form-item>
        <el-form-item label="模板" prop="template">
          <el-select v-model="form.template" placeholder="选择模板">
            <el-option
              v-for="tpl in templateOptions"
              :key="tpl.name"
              :label="tpl.display_name"
              :value="tpl.name"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="词库">
          <el-select v-model="form.keyword_group_id" placeholder="使用默认词库" clearable>
            <el-option
              v-for="group in keywordGroups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="图库">
          <el-select v-model="form.image_group_id" placeholder="使用默认图库" clearable>
            <el-option
              v-for="group in imageGroups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="文章库">
          <el-select v-model="form.article_group_id" placeholder="使用默认文章库" clearable>
            <el-option
              v-for="group in articleGroups"
              :key="group.id"
              :label="group.name + (group.is_default ? ' (默认)' : '')"
              :value="group.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="ICP备案号">
          <el-input v-model="form.icp_number" placeholder="京ICP备xxxxxxxx号" />
        </el-form-item>
        <el-form-item label="百度推送Token">
          <el-input v-model="form.baidu_token" placeholder="百度站长平台推送Token" />
        </el-form-item>
        <el-form-item label="统计代码">
          <el-input
            v-model="form.analytics"
            type="textarea"
            :rows="4"
            placeholder="Google Analytics / 百度统计代码"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="handleSubmit">
          确定
        </el-button>
      </template>
    </el-dialog>

    <!-- 新增/编辑分组弹窗 -->
    <el-dialog
      v-model="groupDialogVisible"
      :title="editingGroup ? '编辑分组' : '新建分组'"
      width="400px"
      @close="resetGroupForm"
    >
      <el-form :model="groupForm" label-width="80px">
        <el-form-item label="分组名称" required>
          <el-input v-model="groupForm.name" placeholder="输入分组名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="groupForm.description" type="textarea" :rows="3" placeholder="分组描述（选填）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="groupSubmitLoading" @click="handleGroupSubmit">确定</el-button>
      </template>
    </el-dialog>

    <!-- 右键菜单 -->
    <div
      v-if="contextMenuVisible"
      class="context-menu"
      :style="{ left: contextMenuX + 'px', top: contextMenuY + 'px' }"
      @click.stop
    >
      <div class="menu-item" @click="handleEditGroup">
        <el-icon><Edit /></el-icon>
        编辑分组
      </div>
      <div class="menu-item" @click="handleSetDefault" v-if="!contextMenuGroup?.is_default">
        <el-icon><Star /></el-icon>
        设为默认
      </div>
      <div class="menu-item danger" @click="handleDeleteGroup" v-if="!contextMenuGroup?.is_default">
        <el-icon><Delete /></el-icon>
        删除分组
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox, FormInstance, FormRules } from 'element-plus'
import { Plus, Search, Folder, Delete, Check, Close, Edit, Star } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import { getSites, createSite, updateSite, deleteSite, getGroupOptions, batchDeleteSites, batchUpdateSiteStatus } from '@/api/sites'
import { getTemplateOptions } from '@/api/templates'
import { getSiteGroups, createSiteGroup, updateSiteGroup, deleteSiteGroup } from '@/api/site-groups'
import { getArticleGroups } from '@/api/articles'
import type { Site, TemplateOption, GroupOption, SiteGroup, ArticleGroup, SiteGroupCreate, SiteGroupUpdate } from '@/types'

// 状态
const loading = ref(false)
const submitLoading = ref(false)
const groupSubmitLoading = ref(false)
const dialogVisible = ref(false)
const groupDialogVisible = ref(false)
const isEdit = ref(false)
const formRef = ref<FormInstance>()
const tableRef = ref()

// 数据
const sites = ref<Site[]>([])
const siteGroups = ref<SiteGroup[]>([])
const templateOptions = ref<TemplateOption[]>([])
const keywordGroups = ref<GroupOption[]>([])
const imageGroups = ref<GroupOption[]>([])
const articleGroups = ref<ArticleGroup[]>([])
const selectedItems = ref<Site[]>([])

// 分组相关
const activeGroupId = ref<number>(1)
const groupSearch = ref('')
const editingGroup = ref<SiteGroup | null>(null)

// 搜索和分页
const searchDomain = ref('')
const searchStatus = ref<number | ''>('')
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)

// 右键菜单
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuGroup = ref<SiteGroup | null>(null)

// 表单数据
const form = reactive({
  id: 0,
  site_group_id: 1 as number,
  domain: '',
  name: '',
  template: 'download_site',
  keyword_group_id: null as number | null,
  image_group_id: null as number | null,
  article_group_id: null as number | null,
  icp_number: '',
  baidu_token: '',
  analytics: ''
})

const groupForm = reactive({
  name: '',
  description: ''
})

// 表单验证规则
const rules: FormRules = {
  domain: [
    { required: true, message: '请输入域名', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9][-a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-a-zA-Z0-9]{0,62})+$/, message: '请输入有效的域名', trigger: 'blur' }
  ],
  name: [{ required: true, message: '请输入站点名称', trigger: 'blur' }],
  template: [{ required: true, message: '请选择模板', trigger: 'change' }]
}

// 计算属性
const filteredGroups = computed(() => {
  if (!groupSearch.value) return siteGroups.value
  return siteGroups.value.filter(g => g.name.toLowerCase().includes(groupSearch.value.toLowerCase()))
})

const filteredSites = computed(() => {
  let result = sites.value
  if (searchDomain.value) {
    result = result.filter(s => s.domain.includes(searchDomain.value))
  }
  if (searchStatus.value !== '') {
    result = result.filter(s => s.status === searchStatus.value)
  }
  return result
})

// 工具函数
const formatDate = (dateStr: string) => {
  return dayjs(dateStr).format('YYYY-MM-DD HH:mm')
}

const getGroupName = (groups: GroupOption[], groupId: number | null): string => {
  if (!groupId) return '默认'
  const group = groups.find(g => g.id === groupId)
  return group ? group.name : '未知'
}

const getArticleGroupName = (groupId: number | null): string => {
  if (!groupId) return '默认'
  const group = articleGroups.value.find(g => g.id === groupId)
  return group ? group.name : '未知'
}

// 数据加载
const loadSites = async () => {
  loading.value = true
  try {
    const allSites = await getSites()
    // 按当前选中的分组过滤
    sites.value = allSites.filter(s => s.site_group_id === activeGroupId.value)
    total.value = sites.value.length
  } finally {
    loading.value = false
  }
}

const loadTemplates = async () => {
  try {
    templateOptions.value = await getTemplateOptions()
  } catch {
    templateOptions.value = [{ id: 0, name: 'download_site', display_name: '下载站模板' }]
  }
}

const loadGroupOptions = async () => {
  try {
    const options = await getGroupOptions()
    keywordGroups.value = options.keyword_groups
    imageGroups.value = options.image_groups
  } catch {
    keywordGroups.value = []
    imageGroups.value = []
  }
}

const loadSiteGroups = async () => {
  try {
    siteGroups.value = await getSiteGroups()
    // 如果有分组，默认选中第一个
    if (siteGroups.value.length > 0 && !siteGroups.value.find(g => g.id === activeGroupId.value)) {
      activeGroupId.value = siteGroups.value[0].id
    }
  } catch {
    siteGroups.value = [{ id: 1, name: '默认分组', description: null, status: 1, is_default: 1, created_at: '', updated_at: '' }]
  }
}

const loadArticleGroups = async () => {
  try {
    articleGroups.value = await getArticleGroups()
  } catch {
    articleGroups.value = []
  }
}

// 分组操作
const selectGroup = (groupId: number) => {
  activeGroupId.value = groupId
  clearSelection()
  loadSites()
}

const showContextMenu = (event: MouseEvent, group: SiteGroup) => {
  contextMenuX.value = event.clientX
  contextMenuY.value = event.clientY
  contextMenuGroup.value = group
  contextMenuVisible.value = true
}

const hideContextMenu = () => {
  contextMenuVisible.value = false
  contextMenuGroup.value = null
}

const handleEditGroup = () => {
  if (!contextMenuGroup.value) return
  editingGroup.value = contextMenuGroup.value
  groupForm.name = contextMenuGroup.value.name
  groupForm.description = contextMenuGroup.value.description || ''
  groupDialogVisible.value = true
  hideContextMenu()
}

const handleSetDefault = async () => {
  if (!contextMenuGroup.value) return
  try {
    await updateSiteGroup(contextMenuGroup.value.id, { is_default: 1 })
    ElMessage.success('已设为默认分组')
    await loadSiteGroups()
  } catch (e: any) {
    ElMessage.error(e.message || '设置失败')
  }
  hideContextMenu()
}

const handleDeleteGroup = async () => {
  if (!contextMenuGroup.value) return
  try {
    await ElMessageBox.confirm(
      `确定要删除分组"${contextMenuGroup.value.name}"吗？该分组下的所有站点将被移动到默认分组。`,
      '提示',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    await deleteSiteGroup(contextMenuGroup.value.id)
    ElMessage.success('删除成功')
    await loadSiteGroups()
    if (activeGroupId.value === contextMenuGroup.value.id) {
      activeGroupId.value = siteGroups.value[0]?.id || 1
      loadSites()
    }
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '删除失败')
    }
  }
  hideContextMenu()
}

const handleGroupSubmit = async () => {
  if (!groupForm.name.trim()) {
    ElMessage.warning('请输入分组名称')
    return
  }
  groupSubmitLoading.value = true
  try {
    if (editingGroup.value) {
      await updateSiteGroup(editingGroup.value.id, {
        name: groupForm.name,
        description: groupForm.description || undefined
      } as SiteGroupUpdate)
      ElMessage.success('更新成功')
    } else {
      await createSiteGroup({
        name: groupForm.name,
        description: groupForm.description || undefined
      } as SiteGroupCreate)
      ElMessage.success('创建成功')
    }
    groupDialogVisible.value = false
    await loadSiteGroups()
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    groupSubmitLoading.value = false
  }
}

const resetGroupForm = () => {
  editingGroup.value = null
  groupForm.name = ''
  groupForm.description = ''
}

// 表格选择
const handleSelectionChange = (selection: Site[]) => {
  selectedItems.value = selection
}

const clearSelection = () => {
  selectedItems.value = []
  tableRef.value?.clearSelection()
}

// 批量操作
const handleBatchDelete = async () => {
  if (selectedItems.value.length === 0) return
  try {
    await ElMessageBox.confirm(
      `确定要删除选中的 ${selectedItems.value.length} 个站点吗？`,
      '批量删除',
      { confirmButtonText: '确定', cancelButtonText: '取消', type: 'warning' }
    )
    const ids = selectedItems.value.map(item => item.id)
    const result = await batchDeleteSites(ids)
    ElMessage.success(`成功删除 ${result.deleted} 个站点`)
    clearSelection()
    loadSites()
  } catch (e: any) {
    if (e !== 'cancel') {
      ElMessage.error(e.message || '批量删除失败')
    }
  }
}

const handleBatchEnable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const result = await batchUpdateSiteStatus(ids, 1)
    ElMessage.success(`成功启用 ${result.updated} 个站点`)
    clearSelection()
    loadSites()
  } catch (e: any) {
    ElMessage.error(e.message || '批量启用失败')
  }
}

const handleBatchDisable = async () => {
  if (selectedItems.value.length === 0) return
  try {
    const ids = selectedItems.value.map(item => item.id)
    const result = await batchUpdateSiteStatus(ids, 0)
    ElMessage.success(`成功禁用 ${result.updated} 个站点`)
    clearSelection()
    loadSites()
  } catch (e: any) {
    ElMessage.error(e.message || '批量禁用失败')
  }
}

// 站点操作
const handleAdd = () => {
  isEdit.value = false
  form.site_group_id = activeGroupId.value
  dialogVisible.value = true
}

const handleEdit = (row: Site) => {
  isEdit.value = true
  form.id = row.id
  form.site_group_id = row.site_group_id || 1
  form.domain = row.domain
  form.name = row.name || ''
  form.template = row.template
  form.keyword_group_id = row.keyword_group_id
  form.image_group_id = row.image_group_id
  form.article_group_id = row.article_group_id
  form.icp_number = row.icp_number || ''
  form.baidu_token = row.baidu_token || ''
  form.analytics = row.analytics || ''
  dialogVisible.value = true
}

const handleDelete = (row: Site) => {
  ElMessageBox.confirm(`确定要删除站点 "${row.domain}" 吗？`, '提示', {
    confirmButtonText: '确定',
    cancelButtonText: '取消',
    type: 'warning'
  }).then(async () => {
    await deleteSite(row.id)
    ElMessage.success('删除成功')
    loadSites()
  })
}

const handleStatusChange = async (row: Site) => {
  try {
    await updateSite(row.id, { status: row.status })
    ElMessage.success(row.status === 1 ? '已启用' : '已禁用')
  } catch {
    row.status = row.status === 1 ? 0 : 1
  }
}

const handleSubmit = async () => {
  await formRef.value?.validate()
  submitLoading.value = true
  try {
    if (isEdit.value) {
      await updateSite(form.id, {
        site_group_id: form.site_group_id,
        name: form.name,
        template: form.template,
        keyword_group_id: form.keyword_group_id,
        image_group_id: form.image_group_id,
        article_group_id: form.article_group_id,
        icp_number: form.icp_number,
        baidu_token: form.baidu_token,
        analytics: form.analytics
      })
      ElMessage.success('更新成功')
    } else {
      await createSite({
        site_group_id: form.site_group_id,
        domain: form.domain,
        name: form.name,
        template: form.template,
        keyword_group_id: form.keyword_group_id,
        image_group_id: form.image_group_id,
        article_group_id: form.article_group_id,
        icp_number: form.icp_number,
        baidu_token: form.baidu_token,
        analytics: form.analytics
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadSites()
  } finally {
    submitLoading.value = false
  }
}

const resetForm = () => {
  form.id = 0
  form.site_group_id = activeGroupId.value
  form.domain = ''
  form.name = ''
  form.template = 'download_site'
  form.keyword_group_id = null
  form.image_group_id = null
  form.article_group_id = null
  form.icp_number = ''
  form.baidu_token = ''
  form.analytics = ''
  formRef.value?.clearValidate()
}

// 生命周期
onMounted(() => {
  loadSiteGroups().then(() => {
    loadSites()
  })
  loadTemplates()
  loadGroupOptions()
  loadArticleGroups()
  document.addEventListener('click', hideContextMenu)
})

onUnmounted(() => {
  document.removeEventListener('click', hideContextMenu)
})
</script>

<style lang="scss" scoped>
.site-list {
  .page-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;

    .title {
      font-size: 20px;
      font-weight: 600;
      color: #303133;
    }
  }

  .page-container {
    display: flex;
    gap: 20px;
    flex: 1;
    min-height: 0;
  }

  .group-sidebar {
    width: 220px;
    flex-shrink: 0;
    background: #fff;
    border-radius: 8px;
    padding: 16px;
    display: flex;
    flex-direction: column;

    .sidebar-header {
      margin-bottom: 12px;
    }

    .sidebar-search {
      margin-bottom: 12px;
    }

    .sidebar-list {
      flex: 1;
      overflow-y: auto;
    }

    .group-item {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 10px 12px;
      border-radius: 6px;
      cursor: pointer;
      transition: all 0.2s;
      margin-bottom: 4px;

      &:hover {
        background: #f5f7fa;
      }

      &.active {
        background: #ecf5ff;
        color: #409eff;
      }

      .group-name {
        flex: 1;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    }

    .empty-tip {
      text-align: center;
      color: #909399;
      padding: 20px 0;
      font-size: 14px;
    }
  }

  .content-area {
    flex: 1;
    background: #fff;
    border-radius: 8px;
    padding: 20px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }

  .batch-actions {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    background: #f0f9eb;
    border-radius: 6px;
    margin-bottom: 16px;

    .selected-count {
      color: #67c23a;
      font-weight: 500;
    }
  }

  .search-form {
    margin-bottom: 16px;
  }

  .data-table {
    flex: 1;
    overflow: auto;
  }

  .pagination-wrapper {
    padding-top: 16px;
    display: flex;
    justify-content: flex-end;
  }

  .text-muted {
    color: #909399;
    font-size: 13px;
  }

  .context-menu {
    position: fixed;
    background: #fff;
    border-radius: 4px;
    box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
    padding: 4px 0;
    min-width: 120px;
    z-index: 3000;

    .menu-item {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 16px;
      cursor: pointer;
      font-size: 14px;
      color: #606266;

      &:hover {
        background: #f5f7fa;
      }

      &.danger {
        color: #f56c6c;

        &:hover {
          background: #fef0f0;
        }
      }
    }
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
