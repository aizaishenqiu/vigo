package db

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"
)

// PoolMonitor 数据库连接池监控器
// 实时监控连接池状态，提供告警和统计功能
type PoolMonitor struct {
	db        *sql.DB
	interval  time.Duration
	statsChan chan *PoolStats
	stopChan  chan struct{}
	mu        sync.RWMutex
	running   bool
	alerts    []AlertFunc
}

// PoolStats 连接池统计信息
type PoolStats struct {
	MaxOpen           int           // 最大打开连接数
	Open              int           // 当前打开连接数
	InUse             int           // 正在使用连接数
	Idle              int           // 空闲连接数
	WaitCount         int64         // 等待连接数
	WaitDuration      time.Duration // 等待总耗时
	MaxIdleClosed     int64         // 因超过空闲时间关闭的连接数
	MaxLifetimeClosed int64         // 因超过生命周期关闭的连接数
	Timestamp         time.Time     // 统计时间戳
}

// AlertFunc 告警函数类型
type AlertFunc func(*PoolStats)

// NewPoolMonitor 创建连接池监控器
// 参数:
//   - db: 数据库连接对象
//   - interval: 监控间隔
func NewPoolMonitor(db *sql.DB, interval time.Duration) *PoolMonitor {
	if interval <= 0 {
		interval = 1 * time.Second
	}

	return &PoolMonitor{
		db:        db,
		interval:  interval,
		statsChan: make(chan *PoolStats, 100),
		stopChan:  make(chan struct{}),
		alerts:    make([]AlertFunc, 0),
	}
}

// Start 启动监控
func (pm *PoolMonitor) Start() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return
	}

	pm.running = true

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[PoolMonitor] Panic recovered: %v", r)
			}
		}()

		ticker := time.NewTicker(pm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := pm.collectStats()
				// 非阻塞发送，避免通道满时阻塞
				select {
				case pm.statsChan <- stats:
					// 发送成功
				default:
					// 通道已满，跳过
					log.Printf("[PoolMonitor] Stats channel full, skipping")
				}
				pm.checkAlerts(stats)
			case <-pm.stopChan:
				return
			}
		}
	}()

	log.Printf("[PoolMonitor] 监控已启动（间隔：%v）", pm.interval)
}

// Stop 停止监控
func (pm *PoolMonitor) Stop() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return
	}

	pm.running = false
	close(pm.stopChan)

	log.Printf("[PoolMonitor] 监控已停止")
}

// IsRunning 检查是否正在运行
func (pm *PoolMonitor) IsRunning() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.running
}

// collectStats 收集统计信息
func (pm *PoolMonitor) collectStats() *PoolStats {
	stats := pm.db.Stats()

	return &PoolStats{
		MaxOpen:           stats.MaxOpenConnections,
		Open:              stats.OpenConnections,
		InUse:             stats.InUse,
		Idle:              stats.Idle,
		WaitCount:         stats.WaitCount,
		WaitDuration:      stats.WaitDuration,
		MaxIdleClosed:     stats.MaxIdleClosed,
		MaxLifetimeClosed: stats.MaxLifetimeClosed,
		Timestamp:         time.Now(),
	}
}

// checkAlerts 检查告警
func (pm *PoolMonitor) checkAlerts(stats *PoolStats) {
	// 执行所有告警函数
	for _, alertFn := range pm.alerts {
		alertFn(stats)
	}

	// 内置告警规则

	// 1. 连接使用率过高（> 90%）
	if stats.MaxOpen > 0 {
		usage := float64(stats.InUse) / float64(stats.MaxOpen) * 100
		if usage > 90 {
			log.Printf("[PoolMonitor][警告] 连接池使用率过高：%.2f%% (使用中:%d/最大:%d)",
				usage, stats.InUse, stats.MaxOpen)
		}
	}

	// 2. 等待连接过多
	if stats.WaitCount > 100 {
		log.Printf("[PoolMonitor][警告] 等待连接数过多：%d", stats.WaitCount)
	}

	// 3. 空闲连接过少
	if stats.Idle < 5 && stats.Open > 10 {
		log.Printf("[PoolMonitor][警告] 空闲连接过少：%d", stats.Idle)
	}

	// 4. 连接泄漏检测
	if stats.Idle == 0 && stats.Open > int(float64(stats.MaxOpen)*0.8) {
		log.Printf("[PoolMonitor][严重] 可能存在连接泄漏：打开连接数 %d，接近最大值 %d",
			stats.Open, stats.MaxOpen)
	}

	// 5. 等待时间过长
	if stats.WaitDuration > 10*time.Second {
		log.Printf("[PoolMonitor][警告] 等待时间过长：%v", stats.WaitDuration)
	}
}

// AddAlert 添加告警函数
// 参数:
//   - alertFn: 告警函数，接收统计信息
func (pm *PoolMonitor) AddAlert(alertFn AlertFunc) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.alerts = append(pm.alerts, alertFn)
}

// GetStatsChan 获取统计通道
func (pm *PoolMonitor) GetStatsChan() <-chan *PoolStats {
	return pm.statsChan
}

// GetStats 获取当前统计
func (pm *PoolMonitor) GetStats() *PoolStats {
	return pm.collectStats()
}

// PrintStats 打印统计信息
func (pm *PoolMonitor) PrintStats() {
	stats := pm.GetStats()

	fmt.Println("=== 数据库连接池统计 ===")
	fmt.Printf("最大连接数：%d\n", stats.MaxOpen)
	fmt.Printf("打开连接数：%d\n", stats.Open)
	fmt.Printf("使用中连接：%d\n", stats.InUse)
	fmt.Printf("空闲连接：%d\n", stats.Idle)
	fmt.Printf("等待连接数：%d\n", stats.WaitCount)
	fmt.Printf("等待耗时：%v\n", stats.WaitDuration)
	fmt.Printf("空闲超时关闭：%d\n", stats.MaxIdleClosed)
	fmt.Printf("生命周期关闭：%d\n", stats.MaxLifetimeClosed)
	fmt.Printf("统计时间：%s\n", stats.Timestamp.Format("2006-01-02 15:04:05"))
}

// GetUsage 获取连接使用率
func (pm *PoolMonitor) GetUsage() float64 {
	stats := pm.GetStats()
	if stats.MaxOpen == 0 {
		return 0
	}
	return float64(stats.InUse) / float64(stats.MaxOpen) * 100
}

// GetWaitRate 获取等待率
func (pm *PoolMonitor) GetWaitRate() float64 {
	stats := pm.GetStats()
	if stats.Open == 0 {
		return 0
	}
	return float64(stats.WaitCount) / float64(stats.Open) * 100
}

// ==================== 全局连接池监控 ====================

// GlobalPoolMonitor 全局连接池监控器
var GlobalPoolMonitor *PoolMonitor

// InitGlobalPoolMonitor 初始化全局连接池监控器
// 参数:
//   - db: 数据库连接对象
//   - interval: 监控间隔
func InitGlobalPoolMonitor(db *sql.DB, interval time.Duration) {
	GlobalPoolMonitor = NewPoolMonitor(db, interval)
}

// StartGlobalPoolMonitor 启动全局连接池监控
func StartGlobalPoolMonitor() {
	if GlobalPoolMonitor != nil {
		GlobalPoolMonitor.Start()
	}
}

// StopGlobalPoolMonitor 停止全局连接池监控
func StopGlobalPoolMonitor() {
	if GlobalPoolMonitor != nil {
		GlobalPoolMonitor.Stop()
	}
}

// GetGlobalPoolStats 获取全局连接池统计
func GetGlobalPoolStats() *PoolStats {
	if GlobalPoolMonitor == nil {
		return nil
	}
	return GlobalPoolMonitor.GetStats()
}

// PrintGlobalPoolStats 打印全局连接池统计
func PrintGlobalPoolStats() {
	if GlobalPoolMonitor == nil {
		log.Println("[PoolMonitor] 全局监控未初始化")
		return
	}
	GlobalPoolMonitor.PrintStats()
}

// AddGlobalPoolAlert 添加全局连接池告警
func AddGlobalPoolAlert(alertFn AlertFunc) {
	if GlobalPoolMonitor != nil {
		GlobalPoolMonitor.AddAlert(alertFn)
	}
}
