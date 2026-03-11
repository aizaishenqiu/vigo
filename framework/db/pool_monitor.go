package db

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
	"vigo/framework/mvc"

	"github.com/redis/go-redis/v9"
)

// PoolMonitor 连接池监控器
type PoolMonitor struct {
	dbs      []*sql.DB
	redis    []*redis.Client
	interval time.Duration
	alerts   []AlertFunc
	mu       sync.RWMutex
	stopChan chan struct{}
}

// AlertFunc 告警函数
type AlertFunc func(message string)

// PoolStats 连接池统计
type PoolStats struct {
	MaxOpenConnections int           // 最大连接数
	OpenConnections    int           // 当前打开的连接数
	InUse              int           // 正在使用的连接数
	Idle               int           // 空闲连接数
	WaitCount          int64         // 等待连接次数
	WaitDuration       time.Duration // 等待总时长
	MaxIdleClosed      int64         // 因超过空闲连接数而关闭的连接数
	MaxIdleTimeClosed  int64         // 因超过空闲时间而关闭的连接数
	MaxLifetimeClosed  int64         // 因超过生存时间而关闭的连接数
}

// NewPoolMonitor 创建连接池监控器
// 参数:
//   - interval: 监控间隔（默认 10 秒）
//   - alerts: 告警函数列表
//
// 示例:
//
//	monitor := db.NewPoolMonitor(10*time.Second, func(msg string) {
//	    log.Printf("ALERT: %s", msg)
//	})
//	monitor.AddDB(GlobalDB)
//	monitor.Start()
func NewPoolMonitor(interval time.Duration, alerts ...AlertFunc) *PoolMonitor {
	if interval <= 0 {
		interval = 10 * time.Second
	}

	return &PoolMonitor{
		dbs:      make([]*sql.DB, 0),
		redis:    make([]*redis.Client, 0),
		interval: interval,
		alerts:   alerts,
		stopChan: make(chan struct{}),
	}
}

// AddDB 添加数据库连接池到监控
func (pm *PoolMonitor) AddDB(dbs ...*sql.DB) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.dbs = append(pm.dbs, dbs...)
}

// AddRedis 添加 Redis 连接池到监控
func (pm *PoolMonitor) AddRedis(clients ...*redis.Client) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.redis = append(pm.redis, clients...)
}

// AddAlert 添加告警函数
func (pm *PoolMonitor) AddAlert(alert AlertFunc) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alerts = append(pm.alerts, alert)
}

// Start 启动监控
func (pm *PoolMonitor) Start() {
	go pm.monitorLoop()
}

// Stop 停止监控
func (pm *PoolMonitor) Stop() {
	close(pm.stopChan)
}

// monitorLoop 监控循环
func (pm *PoolMonitor) monitorLoop() {
	ticker := time.NewTicker(pm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pm.checkAll()
		case <-pm.stopChan:
			return
		}
	}
}

// checkAll 检查所有连接池
func (pm *PoolMonitor) checkAll() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 检查数据库连接池
	for i, db := range pm.dbs {
		stats := db.Stats()
		pm.checkDBStats(i, stats)
	}

	// 检查 Redis 连接池
	for i, client := range pm.redis {
		pm.checkRedisStats(i, client)
	}
}

// checkDBStats 检查数据库连接池统计
func (pm *PoolMonitor) checkDBStats(index int, stats sql.DBStats) {
	// 计算使用率
	usage := float64(stats.InUse) / float64(stats.MaxOpenConnections) * 100

	// 告警：连接池使用率超过 80%
	if usage > 80 {
		pm.alert(fmt.Sprintf("数据库连接池使用率过高：%.1f%% (当前：%d, 最大：%d)",
			usage, stats.InUse, stats.MaxOpenConnections))
	}

	// 告警：等待队列过长
	if stats.WaitCount > 100 {
		pm.alert(fmt.Sprintf("数据库连接池等待队列过长：%d 次", stats.WaitCount))
	}

	// 告警：等待时间过长
	if stats.WaitDuration > time.Second {
		pm.alert(fmt.Sprintf("数据库连接池等待时间过长：%v", stats.WaitDuration))
	}

	// 日志记录
	log.Printf("[DB Pool Monitor] 连接池#%d: 打开=%d, 使用中=%d, 空闲=%d, 等待=%d, 使用率=%.1f%%",
		index, stats.OpenConnections, stats.InUse, stats.Idle, stats.WaitCount, usage)
}

// checkRedisStats 检查 Redis 连接池统计
func (pm *PoolMonitor) checkRedisStats(index int, client *redis.Client) {
	// Redis 连接池统计
	poolStats := client.PoolStats()

	// 计算使用率（使用 TotalConns）
	totalConns := poolStats.IdleConns + poolStats.Hits
	if totalConns == 0 {
		return
	}
	usage := float64(poolStats.Hits) / float64(totalConns) * 100

	// 告警：连接池使用率超过 80%
	if usage > 80 {
		pm.alert(fmt.Sprintf("Redis 连接池使用率过高：%.1f%% (Hits=%d, Misses=%d)",
			usage, poolStats.Hits, poolStats.Misses))
	}

	// 日志记录
	log.Printf("[Redis Pool Monitor] 连接池#%d: 空闲=%d, 命中=%d, 未命中=%d, 陈旧=%d",
		index, poolStats.IdleConns, poolStats.Hits, poolStats.Misses, poolStats.StaleConns)
}

// alert 发送告警
func (pm *PoolMonitor) alert(message string) {
	log.Printf("[ALERT] %s", message)

	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, alert := range pm.alerts {
		alert(message)
	}
}

// GetStats 获取所有数据库连接池统计
func (pm *PoolMonitor) GetStats() []PoolStats {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := make([]PoolStats, 0, len(pm.dbs))
	for _, db := range pm.dbs {
		s := db.Stats()
		stats = append(stats, PoolStats{
			MaxOpenConnections: s.MaxOpenConnections,
			OpenConnections:    s.OpenConnections,
			InUse:              s.InUse,
			Idle:               s.Idle,
			WaitCount:          s.WaitCount,
			WaitDuration:       s.WaitDuration,
			MaxIdleClosed:      s.MaxIdleClosed,
			MaxIdleTimeClosed:  s.MaxIdleTimeClosed,
			MaxLifetimeClosed:  s.MaxLifetimeClosed,
		})
	}

	return stats
}

// PrintStats 打印连接池统计（用于调试）
func (pm *PoolMonitor) PrintStats() {
	stats := pm.GetStats()
	for i, s := range stats {
		fmt.Printf("连接池 #%d:\n", i)
		fmt.Printf("  最大连接数：%d\n", s.MaxOpenConnections)
		fmt.Printf("  当前打开：%d\n", s.OpenConnections)
		fmt.Printf("  使用中：%d\n", s.InUse)
		fmt.Printf("  空闲：%d\n", s.Idle)
		fmt.Printf("  等待次数：%d\n", s.WaitCount)
		fmt.Printf("  等待时长：%v\n", s.WaitDuration)
		fmt.Printf("  空闲关闭：%d\n", s.MaxIdleClosed)
		fmt.Printf("  超时关闭：%d\n", s.MaxIdleTimeClosed)
		fmt.Printf("  寿命关闭：%d\n", s.MaxLifetimeClosed)
		fmt.Println()
	}
}

// ========== 全局监控器 ==========

var globalPoolMonitor *PoolMonitor

// InitGlobalPoolMonitor 初始化全局连接池监控器
func InitGlobalPoolMonitor(interval time.Duration, alerts ...AlertFunc) {
	globalPoolMonitor = NewPoolMonitor(interval, alerts...)
	globalPoolMonitor.Start()
}

// GetGlobalPoolMonitor 获取全局连接池监控器
func GetGlobalPoolMonitor() *PoolMonitor {
	return globalPoolMonitor
}

// MonitorDBPool 监控数据库连接池（便捷函数）
func MonitorDBPool(db *sql.DB, name string, interval time.Duration) {
	monitor := NewPoolMonitor(interval, func(msg string) {
		log.Printf("[DB Alert] %s: %s", name, msg)
	})
	monitor.AddDB(db)
	monitor.Start()
}

// MonitorRedisPool 监控 Redis 连接池（便捷函数）
func MonitorRedisPool(client *redis.Client, name string, interval time.Duration) {
	monitor := NewPoolMonitor(interval, func(msg string) {
		log.Printf("[Redis Alert] %s: %s", name, msg)
	})
	monitor.AddRedis(client)
	monitor.Start()
}

// ========== HTTP 接口 ==========

// PoolStatsHandler 连接池状态 HTTP 处理器
func PoolStatsHandler() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if globalPoolMonitor == nil {
			c.Json(500, map[string]interface{}{
				"error": "pool monitor not initialized",
			})
			return
		}

		stats := globalPoolMonitor.GetStats()
		c.Json(200, map[string]interface{}{
			"stats": stats,
		})
	}
}
