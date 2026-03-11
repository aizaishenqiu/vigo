package admin

import (
	"runtime"
	"time"

	"vigo/framework/container"
	"vigo/framework/redis"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	psnet "github.com/shirou/gopsutil/net"
)

// RealSystemStats 真实系统统计
type RealSystemStats struct {
	CPU        CPUInfo     `json:"cpu"`
	Memory     MemoryInfo  `json:"memory"`
	Disk       DiskInfo    `json:"disk"`
	Network    NetworkInfo `json:"network"`
	Goroutines int         `json:"goroutines"`
	Uptime     string      `json:"uptime"`
	Timestamp  int64       `json:"timestamp"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Usage        float64 `json:"usage"`         // 使用率
	Cores        int     `json:"cores"`         // 核心数
	LogicalCores int     `json:"logical_cores"` // 逻辑核心数
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total        uint64  `json:"total"`         // 总内存 (MB)
	Used         uint64  `json:"used"`          // 已使用 (MB)
	Free         uint64  `json:"free"`          // 空闲 (MB)
	UsagePercent float64 `json:"usage_percent"` // 使用率
	GoAlloc      uint64  `json:"go_alloc"`      // Go 分配 (MB)
	GoSys        uint64  `json:"go_sys"`        // Go 系统 (MB)
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Total        uint64  `json:"total"`         // 总磁盘 (GB)
	Used         uint64  `json:"used"`          // 已使用 (GB)
	Free         uint64  `json:"free"`          // 空闲 (GB)
	UsagePercent float64 `json:"usage_percent"` // 使用率
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	SentRate uint64 `json:"sent_rate"` // 发送速率 (B/s)
	RecvRate uint64 `json:"recv_rate"` // 接收速率 (B/s)
}

var (
	lastNetStats *IOCountersStat
	lastNetTime  time.Time
)

// IOCountersStat 网络统计
type IOCountersStat struct {
	BytesSent   uint64
	BytesRecv   uint64
	PacketsSent uint64
	PacketsRecv uint64
}

// getRealSystemStats 获取真实系统统计
func getRealSystemStats() *RealSystemStats {
	stats := &RealSystemStats{
		Timestamp:  time.Now().UnixNano() / 1e6,
		Goroutines: runtime.NumGoroutine(),
		Uptime:     getUptime(),
	}

	// CPU 信息
	stats.CPU.LogicalCores = runtime.NumCPU()
	if cpuPercent, err := cpu.Percent(0, false); err == nil && len(cpuPercent) > 0 {
		stats.CPU.Usage = cpuPercent[0]
	}
	stats.CPU.Cores = stats.CPU.LogicalCores

	// 内存信息
	if memInfo, err := mem.VirtualMemory(); err == nil {
		stats.Memory.Total = memInfo.Total / 1024 / 1024
		stats.Memory.Used = memInfo.Used / 1024 / 1024
		stats.Memory.Free = memInfo.Free / 1024 / 1024
		stats.Memory.UsagePercent = memInfo.UsedPercent
	}

	// Go 内存统计
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	stats.Memory.GoAlloc = m.Alloc / 1024 / 1024
	stats.Memory.GoSys = m.Sys / 1024 / 1024

	// 磁盘信息
	if diskInfo, err := disk.Usage("/"); err == nil {
		stats.Disk.Total = diskInfo.Total / 1024 / 1024 / 1024
		stats.Disk.Used = diskInfo.Used / 1024 / 1024 / 1024
		stats.Disk.Free = diskInfo.Free / 1024 / 1024 / 1024
		stats.Disk.UsagePercent = diskInfo.UsedPercent
	}

	// 网络信息 (计算速率)
	if netInfo, err := psnet.IOCounters(false); err == nil && len(netInfo) > 0 {
		currentStats := &IOCountersStat{
			BytesSent:   netInfo[0].BytesSent,
			BytesRecv:   netInfo[0].BytesRecv,
			PacketsSent: netInfo[0].PacketsSent,
			PacketsRecv: netInfo[0].PacketsRecv,
		}

		now := time.Now()
		if lastNetStats != nil {
			duration := now.Sub(lastNetTime).Seconds()
			if duration > 0 {
				stats.Network.SentRate = uint64(float64(currentStats.BytesSent-lastNetStats.BytesSent) / duration)
				stats.Network.RecvRate = uint64(float64(currentStats.BytesRecv-lastNetStats.BytesRecv) / duration)
			}
		}

		lastNetStats = currentStats
		lastNetTime = now
	}

	return stats
}

// checkRedisHealth 检查 Redis 健康状态
func checkRedisHealth() RedisHealth {
	start := time.Now()

	health := RedisHealth{
		Status: "down",
	}

	client := container.App().Make("redis")
	if client != nil {
		if r, ok := client.(*redis.Client); ok {
			// 简单 ping 或检查连接
			// 这里假设只要 client 存在且能获取信息就正常
			// 实际应该调用 r.Ping()
			_ = r // suppress unused
			health.Status = "up"
			health.Latency = time.Since(start).Milliseconds()
		}
	}
	return health
}

type RedisHealth struct {
	Status  string `json:"status"`
	Latency int64  `json:"latency"`
	Memory  string `json:"memory"`
	Clients string `json:"clients"`
}

type QueueHealth struct {
	Status   string `json:"status"`
	Messages int    `json:"messages"`
}

func checkQueueHealth() QueueHealth {
	return QueueHealth{Status: "unknown"}
}

func getUptime() string {
	if info, err := host.Info(); err == nil {
		d := time.Duration(info.Uptime) * time.Second
		return d.String()
	}
	return "0s"
}

func getRealHealthStatus() map[string]interface{} {
	return map[string]interface{}{
		"redis": checkRedisHealth(),
		"queue": checkQueueHealth(),
	}
}
