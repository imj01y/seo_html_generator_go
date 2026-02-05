package core

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

func init() {
	// 设置 gopsutil 读取宿主机的 /proc 和 /sys 目录
	// 这样在 Docker 容器中也能获取宿主机的真实系统信息
	if _, err := os.Stat("/host/proc"); err == nil {
		os.Setenv("HOST_PROC", "/host/proc")
	}
	if _, err := os.Stat("/host/sys"); err == nil {
		os.Setenv("HOST_SYS", "/host/sys")
	}
}

// CPUStats CPU 统计
type CPUStats struct {
	UsagePercent  float64 `json:"usage_percent"`
	Cores         int     `json:"cores"`          // 逻辑处理器数（线程数）
	PhysicalCores int     `json:"physical_cores"` // 物理核心数
	BaseMhz       float64 `json:"base_mhz"`       // 基础主频 (MHz)
	CurrentMhz    float64 `json:"current_mhz"`    // 当前频率 (MHz)
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
	CPU     CPUStats     `json:"cpu"`
	Memory  MemoryStats  `json:"memory"`
	Load    LoadStats    `json:"load"`
	Network NetworkStats `json:"network"`
	Disks   []DiskStats  `json:"disks"`
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

	// 获取逻辑处理器数（线程数）- 使用 gopsutil 以读取宿主机信息
	logicalCores, err := cpu.Counts(true)
	if err == nil {
		stats.CPU.Cores = logicalCores
	} else {
		stats.CPU.Cores = runtime.NumCPU() // 降级为容器内的值
	}

	// 获取物理核心数
	physicalCores, err := cpu.Counts(false)
	if err == nil {
		stats.CPU.PhysicalCores = physicalCores
	} else {
		stats.CPU.PhysicalCores = stats.CPU.Cores // 降级为逻辑核心数
	}

	// 获取 CPU 频率信息
	// cpu.Info() 在 Linux 上从 /proc/cpuinfo 读取当前频率
	// 在 Windows 上从 WMI 获取标称频率
	cpuInfos, err := cpu.Info()
	if err == nil && len(cpuInfos) > 0 {
		stats.CPU.BaseMhz = cpuInfos[0].Mhz // 第一个核心的频率作为基准

		// 计算所有核心的平均当前频率
		var totalMhz float64
		for _, info := range cpuInfos {
			totalMhz += info.Mhz
		}
		stats.CPU.CurrentMhz = totalMhz / float64(len(cpuInfos))
	}

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

	// 采集磁盘 - 优先获取宿主机磁盘信息
	stats.Disks = c.collectDiskStats()

	return stats, nil
}

// collectDiskStats 采集磁盘统计，优先获取宿主机信息
func (c *SystemStatsCollector) collectDiskStats() []DiskStats {
	var disks []DiskStats

	// 检查是否挂载了宿主机文件系统
	hostfsPath := "/hostfs"
	hostProcMounts := "/host/proc/mounts"

	if _, err := os.Stat(hostfsPath); err == nil {
		// 容器环境：读取宿主机分区信息
		disks = c.collectHostDiskStats(hostfsPath, hostProcMounts)
	}

	// 如果没有获取到宿主机磁盘，降级为容器内磁盘
	if len(disks) == 0 {
		partitions, err := disk.Partitions(false)
		if err == nil {
			for _, p := range partitions {
				usage, err := disk.Usage(p.Mountpoint)
				if err == nil {
					disks = append(disks, DiskStats{
						Path:         p.Mountpoint,
						TotalBytes:   usage.Total,
						UsedBytes:    usage.Used,
						UsagePercent: usage.UsedPercent,
					})
				}
			}
		}
	}

	return disks
}

// collectHostDiskStats 从宿主机挂载点采集磁盘统计
func (c *SystemStatsCollector) collectHostDiskStats(hostfsPath, mountsFile string) []DiskStats {
	var disks []DiskStats

	// 读取宿主机的 /proc/mounts
	file, err := os.Open(mountsFile)
	if err != nil {
		return disks
	}
	defer file.Close()

	// 虚拟文件系统类型（需要排除）
	virtualFS := map[string]bool{
		"proc": true, "sysfs": true, "devpts": true, "tmpfs": true,
		"devtmpfs": true, "cgroup": true, "cgroup2": true, "overlay": true,
		"squashfs": true, "autofs": true, "securityfs": true, "pstore": true,
		"debugfs": true, "tracefs": true, "fusectl": true, "configfs": true,
		"mqueue": true, "hugetlbfs": true, "binfmt_misc": true, "rpc_pipefs": true,
		"nfsd": true, "fuse.lxcfs": true, "nsfs": true,
	}

	// 已处理的设备（避免重复，如 bind mount）
	seenDevices := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := fields[1]
		fsType := fields[2]

		// 跳过虚拟文件系统
		if virtualFS[fsType] {
			continue
		}

		// 只处理以 /dev/ 开头的真实设备
		if !strings.HasPrefix(device, "/dev/") {
			continue
		}

		// 跳过已处理的设备
		if seenDevices[device] {
			continue
		}
		seenDevices[device] = true

		// 构建宿主机路径
		hostMountPath := filepath.Join(hostfsPath, mountPoint)

		// 获取磁盘使用情况
		usage, err := disk.Usage(hostMountPath)
		if err != nil {
			continue
		}

		disks = append(disks, DiskStats{
			Path:         mountPoint, // 显示宿主机的挂载点路径
			TotalBytes:   usage.Total,
			UsedBytes:    usage.Used,
			UsagePercent: usage.UsedPercent,
		})
	}

	return disks
}
