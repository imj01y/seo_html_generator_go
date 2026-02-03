/**
 * 系统资源监控类型定义
 */

export interface CPUStats {
  usage_percent: number
  cores: number            // 逻辑处理器数（线程数）
  physical_cores: number   // 物理核心数
  base_mhz: number         // 基础主频 (MHz)
  current_mhz: number      // 当前频率 (MHz)
}

export interface MemoryStats {
  total_bytes: number
  used_bytes: number
  usage_percent: number
}

export interface LoadStats {
  load1: number
  load5: number
  load15: number
}

export interface NetworkStats {
  bytes_sent_per_sec: number
  bytes_recv_per_sec: number
}

export interface DiskStats {
  path: string
  total_bytes: number
  used_bytes: number
  usage_percent: number
}

export interface SystemStats {
  type: string
  timestamp: string
  cpu: CPUStats
  memory: MemoryStats
  load: LoadStats
  network: NetworkStats
  disks: DiskStats[]
}
