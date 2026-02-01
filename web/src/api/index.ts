// 共享类型和工具
export { assertSuccess, buildWsUrl, closeWebSocket } from './shared'
export type { SuccessResponse, CreateResponse, CountResponse } from './shared'

// 认证
export * from './auth'

// Dashboard（排除与其他模块冲突的导出）
export { getDashboardStats, getDailySpiderStats, getHourlySpiderStats, getKeywordGroupStats, getImageGroupStats } from './dashboard'
export { getCacheStats as getDashboardCacheStats } from './dashboard'
export { getSpiderStats as getDashboardSpiderStats } from './dashboard'

// 站点和站群
export * from './sites'
export * from './site-groups'

// 数据管理
export * from './keywords'
export * from './images'
export * from './articles'
export * from './templates'

// 蜘蛛（排除与 logs 冲突的 clearOldLogs）
export { getSpiderLogs, getSpiderStats, getDailyStats, getHourlyStats, testSpiderDetection, getSpiderConfig } from './spiders'
export { clearOldLogs as clearOldSpiderLogs } from './spiders'

// 日志
export * from './logs'

// 设置（排除冲突的 getCacheStats）
export {
  getSettings, getSetting, updateSettings,
  clearCache, getCacheStats, clearDomainCache,
  checkDatabase,
  getApiTokenSettings, updateApiTokenSettings, generateApiToken
} from './settings'
export type { ApiTokenResponse } from './settings'

// 数据加工
export * from './processor'

// 池配置
export * from './pool-config'

// 爬虫项目
export * from './spiderProjects'

// 内容工作区
export * from './contentWorker'
