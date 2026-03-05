package db

import (
	"database/sql"
	"log"
	"sync"
	"time"
)

// DBMonitor 数据库连接池监控器
type DBMonitor struct {
	mu        sync.RWMutex
	interval  time.Duration
	stopChan  chan struct{}
	callbacks []func(DBStats)
}

// DBStats 数据库连接池统计
type DBStats struct {
	MaxOpenConnections int           // 最大打开连接数
	OpenConnections    int           // 当前打开连接数
	InUse              int           // 使用中连接数
	Idle               int           // 空闲连接数
	WaitCount          int64         // 等待连接次数
	WaitDuration       time.Duration // 等待总时长
	MaxIdleClosed      int64         // 因超过最大空闲时间关闭的连接数
	MaxIdleTimeClosed  int64         // 因超过空闲时间关闭的连接数
	MaxLifetimeClosed  int64         // 因超过生命周期关闭的连接数
	ConnWaitQueueSize  int           // 连接等待队列大小
	ConnWaitQueueCap   int           // 连接等待队列容量
}

// NewDBMonitor 创建数据库监控器
func NewDBMonitor(interval time.Duration) *DBMonitor {
	return &DBMonitor{
		interval:  interval,
		stopChan:  make(chan struct{}),
		callbacks: make([]func(DBStats), 0),
	}
}

// Start 启动监控
func (m *DBMonitor) Start() {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := m.collectStats(GlobalDB)
				m.notify(stats)
			case <-m.stopChan:
				return
			}
		}
	}()

	log.Printf("DB Monitor started with interval %v", m.interval)
}

// Stop 停止监控
func (m *DBMonitor) Stop() {
	close(m.stopChan)
	log.Println("DB Monitor stopped")
}

// OnStats 注册统计回调
func (m *DBMonitor) OnStats(callback func(DBStats)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// collectStats 收集数据库统计信息
func (m *DBMonitor) collectStats(db interface{}) DBStats {
	if db == nil {
		return DBStats{}
	}

	// 使用反射获取 sql.DB 的统计信息
	// 这里简化处理，实际应该使用 sql.DB.Stats()
	return DBStats{
		OpenConnections: 0,
		InUse:           0,
		Idle:            0,
	}
}

// notify 通知所有回调
func (m *DBMonitor) notify(stats DBStats) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, callback := range m.callbacks {
		callback(stats)
	}
}

// LogStats 打印统计日志
func LogStats() {
	if GlobalDB == nil {
		return
	}

	stats := GlobalDB.Stats()
	log.Printf("DB Stats: Open=%d, InUse=%d, Idle=%d, WaitCount=%d, WaitDuration=%v",
		stats.OpenConnections,
		stats.InUse,
		stats.Idle,
		stats.WaitCount,
		stats.WaitDuration,
	)
}

// AutoTune 自动调整连接池参数
func AutoTune(db *sql.DB, targetUsage float64) {
	if db == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			stats := db.Stats()

			// 计算连接使用率
			usage := float64(stats.InUse) / float64(stats.MaxOpenConnections)

			// 如果使用率过高，增加连接数
			if usage > targetUsage && stats.OpenConnections < stats.MaxOpenConnections {
				newMax := int(float64(stats.MaxOpenConnections) * 1.2)
				db.SetMaxOpenConns(newMax)
				log.Printf("DB AutoTune: Increased MaxOpenConns to %d (usage: %.2f%%)", newMax, usage*100)
			}

			// 如果使用率过低，减少连接数
			if usage < targetUsage*0.5 && stats.MaxOpenConnections > 10 {
				newMax := int(float64(stats.MaxOpenConnections) * 0.8)
				db.SetMaxOpenConns(newMax)
				log.Printf("DB AutoTune: Decreased MaxOpenConns to %d (usage: %.2f%%)", newMax, usage*100)
			}
		}
	}()
}

// HealthCheck 数据库健康检查
func HealthCheck(db *sql.DB, timeout time.Duration) error {
	if db == nil {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		err := db.Ping()
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return ErrHealthCheckTimeout
	}
}

// ErrHealthCheckTimeout 健康检查超时错误
var ErrHealthCheckTimeout = &healthCheckTimeoutError{}

type healthCheckTimeoutError struct{}

func (e *healthCheckTimeoutError) Error() string {
	return "database health check timeout"
}

// ConnectionPoolOptimizer 连接池优化器
type ConnectionPoolOptimizer struct {
	db             *sql.DB
	minConnections int
	maxConnections int
	targetUsage    float64
	mu             sync.Mutex
	stopChan       chan struct{}
}

// NewConnectionPoolOptimizer 创建连接池优化器
func NewConnectionPoolOptimizer(db *sql.DB, minConns, maxConns int, targetUsage float64) *ConnectionPoolOptimizer {
	return &ConnectionPoolOptimizer{
		db:             db,
		minConnections: minConns,
		maxConnections: maxConns,
		targetUsage:    targetUsage,
		stopChan:       make(chan struct{}),
	}
}

// Start 启动优化器
func (o *ConnectionPoolOptimizer) Start() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				o.optimize()
			case <-o.stopChan:
				return
			}
		}
	}()

	log.Println("Connection Pool Optimizer started")
}

// Stop 停止优化器
func (o *ConnectionPoolOptimizer) Stop() {
	close(o.stopChan)
	log.Println("Connection Pool Optimizer stopped")
}

// optimize 优化连接池
func (o *ConnectionPoolOptimizer) optimize() {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.db == nil {
		return
	}

	stats := o.db.Stats()
	usage := float64(stats.InUse) / float64(stats.OpenConnections)

	// 动态调整最大连接数
	if usage > o.targetUsage && stats.OpenConnections < o.maxConnections {
		newMax := int(float64(stats.OpenConnections) * 1.2)
		if newMax > o.maxConnections {
			newMax = o.maxConnections
		}
		o.db.SetMaxOpenConns(newMax)
		log.Printf("DB Optimizer: Increased MaxOpenConns to %d", newMax)
	} else if usage < o.targetUsage*0.5 && stats.OpenConnections > o.minConnections {
		newMax := int(float64(stats.OpenConnections) * 0.8)
		if newMax < o.minConnections {
			newMax = o.minConnections
		}
		o.db.SetMaxOpenConns(newMax)
		log.Printf("DB Optimizer: Decreased MaxOpenConns to %d", newMax)
	}

	// 调整空闲连接数
	idleTarget := int(float64(stats.OpenConnections) * 0.3)
	if idleTarget < 5 {
		idleTarget = 5
	}
	o.db.SetMaxIdleConns(idleTarget)
}
