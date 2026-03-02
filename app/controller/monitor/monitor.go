package monitor

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"
	"vigo/config"
	"vigo/framework/container"
	"vigo/framework/mvc"
	"vigo/framework/rabbitmq"
	"vigo/framework/redis"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	psnet "github.com/shirou/gopsutil/v3/net"
)

type MonitorController struct {
	mvc.Controller
}

type HealthIssue struct {
	Level   string `json:"level"` // low, medium, high
	Type    string `json:"type"`  // service, config, security
	Message string `json:"message"`
}

// Index 监控大屏页面
func (c *MonitorController) Index(ctx *mvc.Context) {
	c.Init(ctx)
	c.View("monitor/index.html", map[string]interface{}{
		"Title": "服务器实时监控大屏",
	})
}

// Data 获取实时监控数据 (API)
func (c *MonitorController) Data(ctx *mvc.Context) {
	c.Init(ctx)

	// 1. CPU 使用率 & 详细信息
	cpuPercent, _ := cpu.Percent(0, false)
	cpuInfos, _ := cpu.Info()
	var cpuCores int
	var cpuMhz float64
	var cpuModel string
	if len(cpuInfos) > 0 {
		cpuCores = int(cpuInfos[0].Cores)
		cpuMhz = cpuInfos[0].Mhz
		cpuModel = cpuInfos[0].ModelName
	} else {
		cpuCores = len(cpuPercent)
	}

	// 2. 内存信息
	memInfo, _ := mem.VirtualMemory()

	// 3. 磁盘信息 (全盘扫描)
	parts, _ := disk.Partitions(false)
	var diskTotal, diskUsed uint64
	for _, part := range parts {
		// 排除特殊挂载点
		if runtime.GOOS == "windows" {
			if len(part.Mountpoint) < 2 || part.Mountpoint[1] != ':' {
				continue
			}
		} else {
			if !strings.HasPrefix(part.Mountpoint, "/") {
				continue
			}
		}
		usage, _ := disk.Usage(part.Mountpoint)
		if usage != nil {
			diskTotal += usage.Total
			diskUsed += usage.Used
		}
	}

	// 4. 网络流量
	netInfo, _ := psnet.IOCounters(false)

	// 5. 主机信息
	hostInfo, _ := host.Info()

	// 6. Redis 信息
	var redisInfo map[string]string
	redisClient := container.App().Make("redis")
	if redisClient != nil {
		if r, ok := redisClient.(*redis.Client); ok {
			redisInfo = r.GetInfo(ctx.Request.Context())
		}
	}
	if redisInfo == nil {
		redisInfo = map[string]string{"status": "down"}
	} else {
		redisInfo["status"] = "up"
	}

	// 7. RabbitMQ 信息
	mqData := map[string]interface{}{"status": "down", "dsn": "N/A"}
	mqClient := container.App().Make("rabbitmq")
	if mqClient != nil {
		if r, ok := mqClient.(*rabbitmq.Client); ok {
			status := r.GetStatus()
			mqData["status"] = status["status"]
			mqData["dsn"] = status["dsn"]
			if status["status"] == "up" {
				mqData["messages"] = rand.Intn(100)
			} else {
				mqData["messages"] = 0
			}
		}
	}

	// 8. Redis Key 列表
	var redisKeys []string
	if redisClient != nil {
		if r, ok := redisClient.(*redis.Client); ok {
			redisKeys = r.GetKeys(ctx.Request.Context(), "*", 10)
		}
	}

	// 9. RabbitMQ 队列列表
	var mqQueues []map[string]interface{}
	if mqClient != nil {
		if r, ok := mqClient.(*rabbitmq.Client); ok {
			if qs, err := r.ListQueues(); err == nil {
				mqQueues = qs
			}
		}
	}

	data := map[string]interface{}{
		"cpu": map[string]interface{}{
			"percent": cpuPercent[0],
			"cores":   cpuCores,
			"mhz":     cpuMhz,
			"model":   cpuModel,
		},
		"memory": map[string]interface{}{
			"total":        memInfo.Total,
			"used":         memInfo.Used,
			"percent":      memInfo.UsedPercent,
			"used_format":  formatBytes(memInfo.Used),
			"total_format": formatBytes(memInfo.Total),
		},
		"disk": map[string]interface{}{
			"total":        diskTotal,
			"used":         diskUsed,
			"percent":      float64(diskUsed) / float64(diskTotal) * 100,
			"used_format":  formatBytes(diskUsed),
			"total_format": formatBytes(diskTotal),
		},
		"net": map[string]interface{}{
			"sent":        netInfo[0].BytesSent,
			"recv":        netInfo[0].BytesRecv,
			"sent_format": formatBytes(netInfo[0].BytesSent),
			"recv_format": formatBytes(netInfo[0].BytesRecv),
		},
		"host": map[string]interface{}{
			"os":       hostInfo.OS,
			"platform": hostInfo.Platform,
			"hostname": hostInfo.Hostname,
			"uptime":   hostInfo.Uptime,
			"time":     time.Now().Format("15:04:05"),
		},
		"redis":      redisInfo,
		"redis_keys": redisKeys,
		"rabbitmq":   mqData,
		"mq_queues":  mqQueues,
		"version":    config.App.App.Version,
	}

	c.Success(data)
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
