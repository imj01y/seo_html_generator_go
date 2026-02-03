package core

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// CPUStats CPU 统计
type CPUStats struct {
	UsagePercent float64 `json:"usage_percent"`
	Cores        int     `json:"cores"`
}

// MemoryStats 内存统计
type MemoryStats struct {
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// LoadStats 系统负载统计
type LoadStats struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// NetworkStats 网络统计
type NetworkStats struct {
	BytesSentPerSec uint64 `json:"bytes_sent_per_sec"`
	BytesRecvPerSec uint64 `json:"bytes_recv_per_sec"`
}

// DiskStats 磁盘统计
type DiskStats struct {
	Path         string  `json:"path"`
	TotalBytes   uint64  `json:"total_bytes"`
	UsedBytes    uint64  `json:"used_bytes"`
	UsagePercent float64 `json:"usage_percent"`
}

// SystemStats 系统统计汇总
type SystemStats struct {
	CPU     CPUStats    `json:"cpu"`
	Memory  MemoryStats `json:"memory"`
	Load    LoadStats   `json:"load"`
	Network NetworkStats `json:"network"`
	Disks   []DiskStats `json:"disks"`
}

// SystemStatsCollector 系统统计采集器
type SystemStatsCollector struct {
	lastNetBytesSent uint64
	lastNetBytesRecv uint64
	lastCollectAt    time.Time
	mu               sync.Mutex
}

// NewSystemStatsCollector 创建系统统计采集器
func NewSystemStatsCollector() *SystemStatsCollector {
	return &SystemStatsCollector{}
}

// Collect 采集系统统计
func (c *SystemStatsCollector) Collect() (*SystemStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := &SystemStats{}

	// 采集 CPU
	cpuPercent, err := cpu.Percent(0, false)
	if err == nil && len(cpuPercent) > 0 {
		stats.CPU.UsagePercent = cpuPercent[0]
	}
	stats.CPU.Cores = runtime.NumCPU()

	// 采集内存
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		stats.Memory.TotalBytes = memInfo.Total
		stats.Memory.UsedBytes = memInfo.Used
		stats.Memory.UsagePercent = memInfo.UsedPercent
	}

	// 采集负载 (Windows 不支持，返回 0)
	loadInfo, err := load.Avg()
	if err == nil && loadInfo != nil {
		stats.Load.Load1 = loadInfo.Load1
		stats.Load.Load5 = loadInfo.Load5
		stats.Load.Load15 = loadInfo.Load15
	}

	// 采集网络
	netIO, err := net.IOCounters(false)
	if err == nil && len(netIO) > 0 {
		now := time.Now()
		if !c.lastCollectAt.IsZero() {
			elapsed := now.Sub(c.lastCollectAt).Seconds()
			if elapsed > 0 {
				stats.Network.BytesSentPerSec = uint64(float64(netIO[0].BytesSent-c.lastNetBytesSent) / elapsed)
				stats.Network.BytesRecvPerSec = uint64(float64(netIO[0].BytesRecv-c.lastNetBytesRecv) / elapsed)
			}
		}
		c.lastNetBytesSent = netIO[0].BytesSent
		c.lastNetBytesRecv = netIO[0].BytesRecv
		c.lastCollectAt = now
	}

	// 采集磁盘
	partitions, err := disk.Partitions(false)
	if err == nil {
		for _, p := range partitions {
			usage, err := disk.Usage(p.Mountpoint)
			if err == nil {
				stats.Disks = append(stats.Disks, DiskStats{
					Path:         p.Mountpoint,
					TotalBytes:   usage.Total,
					UsedBytes:    usage.Used,
					UsagePercent: usage.UsedPercent,
				})
			}
		}
	}

	return stats, nil
}
