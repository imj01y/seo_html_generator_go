import { createRouter, createWebHistory, RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { title: '登录', noAuth: true }
  },
  {
    path: '/',
    component: () => import('@/components/Layout/MainLayout.vue'),
    redirect: '/dashboard',
    children: [
      {
        path: 'dashboard',
        name: 'Dashboard',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: '仪表盘', icon: 'DataAnalysis' }
      },
      {
        path: 'site-groups',
        name: 'SiteGroups',
        component: () => import('@/views/site-groups/SiteGroupList.vue'),
        meta: { title: '站群管理', icon: 'Grid' }
      },
      {
        path: 'sites',
        name: 'Sites',
        component: () => import('@/views/sites/SiteList.vue'),
        meta: { title: '站点管理', icon: 'Monitor' }
      },
      {
        path: 'templates',
        name: 'Templates',
        component: () => import('@/views/templates/TemplateList.vue'),
        meta: { title: '模板管理', icon: 'Document' }
      },
      {
        path: 'templates/edit/:id?',
        name: 'TemplateEdit',
        component: () => import('@/views/templates/TemplateEdit.vue'),
        meta: { title: '编辑模板', icon: 'Document', hidden: true }
      },
      {
        path: 'keywords',
        name: 'Keywords',
        component: () => import('@/views/keywords/KeywordList.vue'),
        meta: { title: '关键词管理', icon: 'Key' }
      },
      {
        path: 'articles',
        name: 'Articles',
        component: () => import('@/views/articles/ArticleList.vue'),
        meta: { title: '文章管理', icon: 'Document' }
      },
      {
        path: 'articles/edit/:id?',
        name: 'ArticleEdit',
        component: () => import('@/views/articles/ArticleEdit.vue'),
        meta: { title: '编辑文章', icon: 'Document', hidden: true }
      },
      {
        path: 'images',
        name: 'Images',
        component: () => import('@/views/images/ImageList.vue'),
        meta: { title: '图片管理', icon: 'Picture' }
      },
      {
        path: 'processor',
        name: 'Processor',
        component: () => import('@/views/processor/ProcessorManage.vue'),
        meta: { title: '数据加工', icon: 'Operation' }
      },
      {
        path: 'worker',
        name: 'WorkerCode',
        component: () => import('@/views/worker/WorkerCodeManager.vue'),
        meta: { title: 'Worker代码', icon: 'EditPen' }
      },
      {
        path: 'spiders/projects',
        name: 'SpiderProjects',
        component: () => import('@/views/spiders/ProjectList.vue'),
        meta: { title: '爬虫项目', icon: 'Cpu' }
      },
      {
        path: 'spiders/projects/:id',
        name: 'SpiderProjectEdit',
        component: () => import('@/views/spiders/ProjectEdit.vue'),
        meta: { title: '编辑项目', icon: 'Cpu', hidden: true }
      },
      {
        path: 'spiders/stats',
        name: 'SpiderStats',
        component: () => import('@/views/spiders/SpiderStats.vue'),
        meta: { title: '爬虫统计', icon: 'TrendCharts' }
      },
      {
        path: 'generators',
        name: 'Generators',
        component: () => import('@/views/generators/GeneratorList.vue'),
        meta: { title: '生成器管理', icon: 'Cpu' }
      },
      {
        path: 'generators/edit/:id?',
        name: 'GeneratorEdit',
        component: () => import('@/views/generators/GeneratorEdit.vue'),
        meta: { title: '编辑生成器', icon: 'Cpu', hidden: true }
      },
      {
        path: 'spiders',
        name: 'Spiders',
        component: () => import('@/views/spiders/SpiderLogs.vue'),
        meta: { title: '蜘蛛日志', icon: 'Connection' }
      },
      {
        path: 'cache',
        name: 'CacheManage',
        component: () => import('@/views/cache/CacheManage.vue'),
        meta: { title: '缓存管理', icon: 'Coin' }
      },
      {
        path: 'settings',
        name: 'Settings',
        component: () => import('@/views/settings/Settings.vue'),
        meta: { title: '系统设置', icon: 'Setting' }
      }
    ]
  },
  {
    path: '/:pathMatch(.*)*',
    redirect: '/dashboard'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// 路由守卫
router.beforeEach((to, _from, next) => {
  // 设置页面标题
  document.title = `${to.meta.title || 'SEO管理后台'} - SEO HTML Generator`

  // 检查登录状态
  const token = localStorage.getItem('token')

  if (to.meta.noAuth) {
    // 不需要登录的页面
    if (token && to.path === '/login') {
      next('/dashboard')
    } else {
      next()
    }
  } else {
    // 需要登录的页面
    if (!token) {
      next('/login')
    } else {
      next()
    }
  }
})

export default router
