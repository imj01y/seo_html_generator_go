// 认证相关
export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
}

export interface UserInfo {
  id: number
  username: string
}

// 分页
export interface PaginationParams {
  page?: number
  page_size?: number
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
}

// 站群（顶层管理单元）
export interface SiteGroup {
  id: number
  name: string
  description: string | null
  status: number  // 1=启用, 0=禁用
  created_at: string
  updated_at: string
}

export interface SiteGroupCreate {
  name: string
  description?: string
}

export interface SiteGroupUpdate {
  name?: string
  description?: string
  status?: number
  is_default?: number
}

export interface SiteGroupStats {
  sites_count: number
  keyword_groups_count: number
  image_groups_count: number
  article_groups_count: number
  templates_count: number
}

export interface SiteGroupWithStats extends SiteGroup {
  stats: SiteGroupStats
}

// 站点
export interface Site {
  id: number
  site_group_id: number          // 所属站群ID
  domain: string
  name: string
  template: string
  keyword_group_id: number | null  // 绑定的关键词分组ID
  image_group_id: number | null    // 绑定的图片分组ID
  article_group_id: number | null  // 绑定的文章分组ID
  icp_number: string | null
  baidu_token: string | null
  analytics: string | null
  status: number  // 1=启用, 0=禁用
  created_at: string
  updated_at: string
}

export interface SiteCreate {
  site_group_id: number          // 所属站群ID（必填）
  domain: string
  name: string
  template?: string
  keyword_group_id?: number  // 绑定的关键词分组ID
  image_group_id?: number    // 绑定的图片分组ID
  article_group_id?: number  // 绑定的文章分组ID
  icp_number?: string
  baidu_token?: string
  analytics?: string
}

export interface SiteUpdate {
  site_group_id?: number  // 所属站点分组ID
  name?: string
  template?: string
  status?: number  // 1=启用, 0=禁用
  keyword_group_id?: number  // 绑定的关键词分组ID
  image_group_id?: number    // 绑定的图片分组ID
  article_group_id?: number  // 绑定的文章分组ID
  icp_number?: string
  baidu_token?: string
  analytics?: string
}

// 关键词分组
export interface KeywordGroup {
  id: number
  site_group_id: number  // 所属站群ID
  name: string
  description: string | null
  is_default: number  // 1=是, 0=否
  status?: number  // 1=启用, 0=禁用
  created_at: string
}

export interface KeywordGroupCreate {
  site_group_id?: number  // 所属站群ID，默认1
  name: string
  description?: string
  is_default?: boolean
}

export interface KeywordGroupUpdate {
  name?: string
  description?: string
  is_default?: number
}

// 关键词
export interface Keyword {
  id: number
  group_id: number
  keyword: string
  status: number
  created_at: string
}

export interface KeywordBatchAdd {
  group_id: number
  keywords: string[]
}

// 图片分组
export interface ImageGroup {
  id: number
  site_group_id: number  // 所属站群ID
  name: string
  description: string | null
  is_default: number  // 1=是, 0=否
  status?: number  // 1=启用, 0=禁用
  created_at: string
}

export interface ImageGroupCreate {
  site_group_id?: number  // 所属站群ID，默认1
  name: string
  description?: string
  is_default?: boolean
}

export interface ImageGroupUpdate {
  name?: string
  description?: string
  is_default?: number
}

// 图片URL
export interface ImageUrl {
  id: number
  group_id: number
  url: string
  status: number
  created_at: string
}

export interface ImageUrlBatchAdd {
  group_id: number
  urls: string[]
}

// 文章分组
export interface ArticleGroup {
  id: number
  site_group_id: number  // 所属站群ID
  name: string
  description: string | null
  is_default?: number  // 1=是, 0=否
  status?: number  // 1=启用, 0=禁用
  created_at: string
}

export interface ArticleGroupCreate {
  site_group_id?: number  // 所属站群ID，默认1
  name: string
  description?: string
  is_default?: boolean
}

export interface ArticleGroupUpdate {
  name?: string
  description?: string
  is_default?: number
}

// 文章
export interface Article {
  id: number
  group_id: number
  title: string
  content: string
  status: number
  source_url?: string  // 原始URL（可选，手动添加的文章可能没有）
  created_at: string
  updated_at: string
}

export interface ArticleCreate {
  group_id: number
  title: string
  content: string
}

export interface ArticleUpdate {
  group_id?: number
  title?: string
  content?: string
  status?: number
}

// 模板
export interface Template {
  id: number
  site_group_id: number  // 所属站群ID
  name: string
  display_name: string
  description: string | null
  content: string
  status: number  // 1=启用, 0=禁用
  version: number
  created_at: string
  updated_at: string
}

export interface TemplateListItem {
  id: number
  site_group_id: number  // 所属站群ID
  name: string
  display_name: string
  description: string | null
  status: number
  version: number
  sites_count: number
  created_at: string
  updated_at: string
}

export interface TemplateCreate {
  site_group_id?: number  // 所属站群ID，默认1
  name: string
  display_name: string
  description?: string
  content: string
}

export interface TemplateUpdate {
  site_group_id?: number  // 所属站群ID
  display_name?: string
  description?: string
  content?: string
  status?: number
}

export interface TemplateOption {
  id: number
  name: string
  display_name: string
}

// 蜘蛛日志
export interface SpiderLog {
  id: number
  spider_type: string
  ip: string
  ua: string
  domain: string
  path: string
  dns_ok: number  // 1=验证通过, 0=未验证
  resp_time: number  // 响应时间(ms)
  cache_hit: number  // 1=命中, 0=未命中
  status: number  // HTTP状态码
  created_at: string
}

export interface SpiderLogQuery extends PaginationParams {
  spider_type?: string
  domain?: string
  start_date?: string
  end_date?: string
}

export interface SpiderStats {
  total_visits: number
  by_spider: Record<string, number>
  by_site: Record<string, number>
  by_status: Record<string, number>
}

export interface DailyStats {
  date: string
  total: number
  by_spider: Record<string, number>
}

// 仪表盘
export interface GroupStats {
  total: number
  cursor: number
  remaining: number
  loaded: boolean
  memory_mb: number
  // 滑动窗口图片管理器新增字段
  pool_start?: number
  pool_end?: number
  pool_size?: number
  pool_ahead?: number
  refilling?: boolean
  mysql_total?: number           // MySQL 总数
  redis_pool_size?: number       // Redis 缓存池计划大小（图片）
  redis_pool_count?: number      // Redis 缓存池实际数量（图片）
  redis_url_hash_count?: number  // Redis URL去重集合数量（图片）
  redis_cache_count?: number     // Redis 缓存数量（关键词/文章）
}

export interface DashboardStats {
  sites_count: number
  keywords_total: number
  images_total: number
  articles_total: number
  keyword_group_stats: GroupStats
  image_group_stats: GroupStats
}

// 分组选项（用于站点绑定）
export interface GroupOption {
  id: number
  name: string
  is_default: number
}

// 设置
export interface Setting {
  key: string
  value: string
  description: string | null
  updated_at: string
}

// 缓存统计
export interface CacheStats {
  keyword_cache_size: number
  image_cache_size: number
  keyword_group_stats: GroupStats
  image_group_stats: GroupStats
  html_cache_entries?: number
  html_cache_memory_mb?: number
}

// 批量操作结果
export interface BatchResult {
  added: number
  skipped: number
}

// 爬虫统计概览
export interface SpiderStatsOverview {
  total: number
  completed: number
  failed: number
  retried: number
  success_rate: number
  avg_speed: number
}

// 图表数据点
export interface SpiderChartDataPoint {
  time: string
  total: number
  completed: number
  failed: number
  retried: number
  success_rate: number
  avg_speed?: number
}

// 按项目统计
export interface SpiderStatsByProject {
  project_id: number
  project_name: string
  total: number
  completed: number
  failed: number
  retried: number
  success_rate: number
  avg_speed?: number
}

// 数据源相关类型
export * from './crawl'

// 生成器相关类型
export * from './generator'
