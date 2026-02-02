/**
 * 格式化工具函数
 */

/**
 * 格式化内存大小（MB 为单位）
 * @param mb 内存大小（MB）
 * @returns 格式化后的字符串
 */
export function formatMemoryMB(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`
  if (mb >= 1) return `${mb.toFixed(2)} MB`
  return `${mb.toFixed(3)} MB`
}

/**
 * 格式化内存大小（字节为单位）
 * @param bytes 内存大小（字节）
 * @returns 格式化后的字符串
 */
export function formatMemoryBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(2)} KB`
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

/**
 * 格式化数字（万为单位）
 * @param num 数字
 * @returns 格式化后的字符串
 */
export function formatNumber(num: number): string {
  if (num >= 10000) {
    return (num / 10000).toFixed(1) + '万'
  }
  return num.toLocaleString()
}
