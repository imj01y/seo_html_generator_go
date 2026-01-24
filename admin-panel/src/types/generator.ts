// 正文生成器相关类型定义

// 正文生成器
export interface ContentGenerator {
  id: number
  name: string
  display_name: string
  description: string | null
  code: string
  enabled: number
  is_default: number
  version: number
  created_at: string
  updated_at: string
}

// 创建生成器请求
export interface GeneratorCreate {
  name: string
  display_name: string
  description?: string
  code: string
  enabled?: number
  is_default?: number
}

// 更新生成器请求
export interface GeneratorUpdate {
  display_name?: string
  description?: string
  code?: string
  enabled?: number
  is_default?: number
}

// 代码模板
export interface CodeTemplate {
  name: string
  display_name: string
  description: string
  code: string
}

// 测试代码请求
export interface TestCodeRequest {
  code: string
  paragraphs: string[]
  titles?: string[]
}

// 测试代码结果
export interface TestCodeResult {
  success: boolean
  content?: string
  message?: string
}

// 生成器列表查询参数
export interface GeneratorQuery {
  page?: number
  page_size?: number
  enabled?: number
}
