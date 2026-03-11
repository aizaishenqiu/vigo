package benchmark

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"vigo/config"
	"vigo/framework/db"
	"vigo/framework/facade"
	"vigo/framework/websocket"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type Stats struct {
	QPS           int64   `json:"qps"`
	AvgLatency    float64 `json:"avg_latency_ms"`
	P95Latency    float64 `json:"p95_latency_ms"`
	P99Latency    float64 `json:"p99_latency_ms"`
	MinLatency    float64 `json:"min_latency_ms"`
	MaxLatency    float64 `json:"max_latency_ms"`
	TotalReq      int64   `json:"total_requests"`
	Success       int64   `json:"success_count"`
	Failed        int64   `json:"failed_count"`
	SuccessQPS    int64   `json:"success_qps"`
	FailedQPS     int64   `json:"failed_qps"`
	Concurrency   int     `json:"concurrency"`
	MQThroughput  int64   `json:"mq_throughput"`
	RedisOps      int64   `json:"redis_ops"`
	MySQLOps      int64   `json:"mysql_ops"`
	HTTPOps       int64   `json:"http_ops"`
	ActiveWorkers int32   `json:"active_workers"`
	RemainingTime int     `json:"remaining_time"`
	LastError     string  `json:"last_error"`
	StopReason    string  `json:"stop_reason"` // 停止原因

	// 服务状态
	RedisStatus   bool `json:"redis_status"`
	MQStatus      bool `json:"mq_status"`
	DBStatus      bool `json:"db_status"`
	WriteDBStatus bool `json:"write_db_status"`
	ReadDBStatus  bool `json:"read_db_status"`

	// 硬件信息
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryTotal uint64  `json:"memory_total"`
	NetSent     uint64  `json:"net_sent"`
	NetRecv     uint64  `json:"net_recv"`

	// HTTP 压测统计
	HTTPStatus2xx int64 `json:"http_status_2xx"`
	HTTPStatus4xx int64 `json:"http_status_4xx"`
	HTTPStatus5xx int64 `json:"http_status_5xx"`

	// 测试模式
	TestMode string `json:"test_mode"`
}

type systemStats struct {
	cpuUsage    float64
	memUsed     uint64
	memTotal    uint64
	netSentRate uint64
	netRecvRate uint64
}

type Service struct {
	isRunning int32
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Atomic counters for current second
	reqCount     int64
	latencySum   int64 // microseconds
	successCount int64
	failCount    int64
	mqCount      int64
	redisCount   int64
	mysqlCount   int64
	httpCount    int64

	// HTTP 状态码统计
	http2xx int64
	http4xx int64
	http5xx int64

	// 延迟采样（用于计算 P95/P99）
	latencySamples  []int64 // microseconds (当前秒)
	latencySampleMu sync.Mutex

	// 累积延迟采样（整个测试期间，最多保留10000个样本）
	allLatencySamples  []int64
	allLatencySampleMu sync.Mutex

	// Last second stats (snapshot)
	lastStats Stats
	mu        sync.RWMutex

	// 系统状态
	sysStats systemStats
	sysMu    sync.RWMutex

	// 测试限制
	targetDBCount int64
	targetMQCount int64
	totalDBOps    int64
	totalMQOps    int64
	testStartTime time.Time
	testDuration  int
	lastError     string

	// 测试配置
	concurrency  int
	enableInsert bool
	memoryLoadMB int
	memoryBlock  []byte

	// HTTP 压测配置
	httpTarget string
	httpMethod string
	enableHTTP bool
	httpClient *http.Client

	// 服务可用性
	canConnectRedis bool
	canConnectMQ    bool
	canConnectDB    bool
	canConnectRead  bool

	// 用户启用的模块
	enableRedis bool
	enableDB    bool
	enableMQ    bool

	// 预处理语句
	readStmt   *sql.Stmt
	insertStmt *sql.Stmt

	// 监控 goroutine 控制
	monitorCancel context.CancelFunc

	// WebSocket 广播
	wsHub *websocket.Hub

	// 测试完成后的最终统计数据（用于保留 P95/P99 等延迟指标）
	finalStats    Stats
	hasFinalStats bool

	// 资源限制配置
	memLimitPercent int    // 内存限制百分比
	cpuLimitPercent int    // CPU限制百分比
	serverMemTotal  uint64 // 服务器总内存
	stopReason      string // 停止原因
}

var instance *Service
var once sync.Once

func GetService() *Service {
	once.Do(func() {
		instance = &Service{
			memLimitPercent: config.App.Benchmark.MemLimitPercent,
			cpuLimitPercent: config.App.Benchmark.CPULimitPercent,
		}
		// 获取服务器总内存
		if vm, err := mem.VirtualMemory(); err == nil {
			instance.serverMemTotal = vm.Total
		}
		// 启动系统监控（独立于测试的生命周期）
		go instance.systemMonitorLoop()
		// 启动连接状态监控
		go instance.connectionMonitorLoop()
		// 启动 WebSocket 广播循环
		go instance.wsBroadcastLoop()
	})
	return instance
}

// SetHub 绑定 WebSocket Hub（由 app 初始化时调用）
func (s *Service) SetHub(hub *websocket.Hub) {
	s.wsHub = hub

	// 注册 benchmark 频道的指令处理器
	hub.OnCommand("benchmark", func(data json.RawMessage, reply func(interface{})) {
		var cmd struct {
			Action       string `json:"action"`
			Concurrency  int    `json:"concurrency"`
			Duration     int    `json:"duration"`
			TargetURL    string `json:"target_url"`
			Method       string `json:"method"`
			DBCount      int64  `json:"db_count"`
			MQCount      int64  `json:"mq_count"`
			EnableInsert bool   `json:"enable_insert"`
			MemoryLoadMB int    `json:"memory_load_mb"`
			EnableRedis  bool   `json:"enable_redis"`
			EnableDB     bool   `json:"enable_db"`
			EnableMQ     bool   `json:"enable_mq"`
		}

		// data 可能是 JSON 字符串（双重编码）或直接 JSON 对象
		raw := data
		var str string
		if json.Unmarshal(data, &str) == nil {
			raw = json.RawMessage(str)
		}

		if err := json.Unmarshal(raw, &cmd); err != nil {
			reply(map[string]interface{}{"code": 1, "msg": err.Error()})
			return
		}

		switch cmd.Action {
		case "start":
			if cmd.Concurrency <= 0 {
				cmd.Concurrency = 10
			}
			// 如果有旧测试在运行（如用户刷新页面后重新启动），自动停止旧测试
			if atomic.LoadInt32(&s.isRunning) == 1 {
				log.Printf("[Benchmark] 自动停止旧测试以启动新测试")
				s.StopTest()
				time.Sleep(200 * time.Millisecond) // 给 worker 退出的时间
			}
			if err := s.StartTest(cmd.Concurrency, cmd.Duration, cmd.DBCount, cmd.MQCount,
				cmd.EnableInsert, cmd.MemoryLoadMB, cmd.EnableRedis, cmd.EnableDB, cmd.EnableMQ); err != nil {
				reply(map[string]interface{}{"code": 1, "msg": err.Error()})
			} else {
				reply(map[string]interface{}{"code": 0, "msg": "压测已启动"})
			}
		case "start-http":
			if cmd.Concurrency <= 0 {
				cmd.Concurrency = 10
			}
			if cmd.TargetURL == "" {
				reply(map[string]interface{}{"code": 1, "msg": "target_url 不能为空"})
				return
			}
			if cmd.Method == "" {
				cmd.Method = "GET"
			}
			// 如果有旧测试在运行，自动停止
			if atomic.LoadInt32(&s.isRunning) == 1 {
				log.Printf("[Benchmark] 自动停止旧测试以启动 HTTP 压测")
				s.StopTest()
				time.Sleep(200 * time.Millisecond)
			}
			if err := s.StartHTTPTest(cmd.Concurrency, cmd.Duration, cmd.TargetURL, cmd.Method); err != nil {
				reply(map[string]interface{}{"code": 1, "msg": err.Error()})
			} else {
				reply(map[string]interface{}{"code": 0, "msg": "HTTP 压测已启动"})
			}
		case "stop":
			s.StopTest()
			reply(map[string]interface{}{"code": 0, "msg": "压测已停止"})
		case "reset":
			s.Reset()
			reply(map[string]interface{}{"code": 0, "msg": "数据已清除"})
		default:
			reply(map[string]interface{}{"code": 1, "msg": "未知指令: " + cmd.Action})
		}
	})

	log.Printf("[Benchmark] WebSocket 指令处理器已注册")
}

// wsBroadcastLoop 持续通过 WebSocket 推送 stats 数据
func (s *Service) wsBroadcastLoop() {
	// 自适应推送频率：空闲 3 秒，压测中 1 秒（和 snapshot 同频避免重复数据点）
	idleInterval := 3 * time.Second
	activeInterval := 1 * time.Second

	ticker := time.NewTicker(idleInterval)
	defer ticker.Stop()

	lastRunning := false

	for range ticker.C {
		if s.wsHub == nil || !s.wsHub.HasSubscribers("benchmark") {
			if lastRunning {
				ticker.Reset(idleInterval)
				lastRunning = false
			}
			continue
		}

		running := atomic.LoadInt32(&s.isRunning) == 1

		// 立即检测运行状态变化，切换到快速推送
		if running && !lastRunning {
			ticker.Reset(activeInterval)
			lastRunning = true
		} else if !running && lastRunning {
			// 测试刚停止，多推一次 idle 数据再切慢速
			lastRunning = false
			ticker.Reset(idleInterval)
		}

		stats := s.GetStats()
		s.wsHub.BroadcastToChannel("benchmark", "stats", stats)
	}
}

// systemMonitorLoop 持续更新硬件状态（独立运行，不受测试影响）
func (s *Service) systemMonitorLoop() {
	lastNetStat, _ := net.IOCounters(false)
	lastTime := time.Now()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cpuUsage := 0.0
		var memUsed, memTotal, netSent, netRecv uint64

		percent, err := cpu.Percent(0, false)
		if err == nil && len(percent) > 0 {
			cpuUsage = percent[0]
		}

		memStat, err := mem.VirtualMemory()
		if err == nil {
			memUsed = memStat.Used / 1024 / 1024
			memTotal = memStat.Total / 1024 / 1024
		}

		currNetStat, err := net.IOCounters(false)
		if err == nil && len(currNetStat) > 0 && len(lastNetStat) > 0 {
			now := time.Now()
			duration := now.Sub(lastTime).Seconds()
			if duration > 0 {
				// 防止 uint64 溢出（计数器回绕或异常）
				if currNetStat[0].BytesSent >= lastNetStat[0].BytesSent {
					netSent = uint64(float64(currNetStat[0].BytesSent-lastNetStat[0].BytesSent) / duration)
				}
				if currNetStat[0].BytesRecv >= lastNetStat[0].BytesRecv {
					netRecv = uint64(float64(currNetStat[0].BytesRecv-lastNetStat[0].BytesRecv) / duration)
				}
				// 上限保护：超过 10GB/s 视为异常数据
				if netSent > 10*1024*1024*1024 {
					netSent = 0
				}
				if netRecv > 10*1024*1024*1024 {
					netRecv = 0
				}
			}
			lastNetStat = currNetStat
			lastTime = now
		}

		s.sysMu.Lock()
		s.sysStats = systemStats{
			cpuUsage:    cpuUsage,
			memUsed:     memUsed,
			memTotal:    memTotal,
			netSentRate: netSent,
			netRecvRate: netRecv,
		}
		s.sysMu.Unlock()
	}
}

// connectionMonitorLoop 定期检查服务连接状态
func (s *Service) connectionMonitorLoop() {
	// 初始检查
	s.checkConnections()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		s.checkConnections()
	}
}

// testMonitorLoop 测试期间的监控（每秒更新统计 + 安全检查）
func (s *Service) testMonitorLoop(ctx context.Context) {
	// 立即执行第一次 snapshot，不等 1 秒，确保 WS 广播能马上拿到正确数据
	s.snapshot()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.snapshot()
			return
		case <-ticker.C:
			s.snapshot()

			// 安全检查：使用配置的阈值保护系统
			cpuPercent, _ := cpu.Percent(0, false)
			memInfo, _ := mem.VirtualMemory()

			// CPU 检查（使用配置阈值）
			cpuLimit := float64(s.cpuLimitPercent)
			if len(cpuPercent) > 0 && cpuPercent[0] > cpuLimit {
				s.lastError = fmt.Sprintf("系统保护：CPU > %.0f%%，已自动停止", cpuLimit)
				s.StopTest()
				return
			}

			// 内存检查（使用配置阈值）
			memLimit := float64(s.memLimitPercent)
			if memInfo != nil && memInfo.UsedPercent > memLimit {
				s.lastError = fmt.Sprintf("系统保护：内存 > %.0f%%，已自动停止", memLimit)
				s.StopTest()
				return
			}

			// Goroutine 数量检查
			numGoroutines := runtime.NumGoroutine()
			if numGoroutines > 10000 {
				s.lastError = fmt.Sprintf("系统保护：Goroutine 数量 %d 过多，已自动停止", numGoroutines)
				log.Printf("[Benchmark] Goroutine 保护触发: %d", numGoroutines)
				s.StopTest()
				return
			}

			// Go 堆内存检查
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			allocMB := memStats.Alloc / 1024 / 1024
			if allocMB > 800 {
				s.lastError = fmt.Sprintf("系统保护：Go 堆内存 %dMB 过高，已自动停止", allocMB)
				log.Printf("[Benchmark] 内存保护触发: %dMB", allocMB)
				s.StopTest()
				return
			}
		}
	}
}

// checkConnections 验证服务连接性
func (s *Service) checkConnections() {
	// Redis
	if rdb := facade.Redis(); rdb != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := rdb.Ping(ctx).Err(); err == nil {
			s.canConnectRedis = true
		} else {
			s.canConnectRedis = false
		}
		cancel()
	} else {
		s.canConnectRedis = false
	}

	// MQ
	if mq := facade.RabbitMQ(); mq != nil {
		s.canConnectMQ = mq.IsConnected()
	} else {
		s.canConnectMQ = false
	}

	// 主数据库
	if db.GlobalDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := db.GlobalDB.PingContext(ctx); err == nil {
			s.canConnectDB = true
		} else {
			s.canConnectDB = false
		}
		cancel()
	} else {
		s.canConnectDB = false
	}

	// 读数据库
	s.canConnectRead = false
	if len(db.ReadDBs) > 0 {
		for _, rdb := range db.ReadDBs {
			if err := rdb.Ping(); err == nil {
				s.canConnectRead = true
				break
			}
		}
	} else if s.canConnectDB {
		s.canConnectRead = true
	}
}

func (s *Service) snapshot() {
	req := atomic.SwapInt64(&s.reqCount, 0)
	lat := atomic.SwapInt64(&s.latencySum, 0)
	succ := atomic.SwapInt64(&s.successCount, 0)
	fail := atomic.SwapInt64(&s.failCount, 0)
	mq := atomic.SwapInt64(&s.mqCount, 0)
	redis := atomic.SwapInt64(&s.redisCount, 0)
	mysql := atomic.SwapInt64(&s.mysqlCount, 0)
	httpOps := atomic.SwapInt64(&s.httpCount, 0)
	h2xx := atomic.SwapInt64(&s.http2xx, 0)
	h4xx := atomic.SwapInt64(&s.http4xx, 0)
	h5xx := atomic.SwapInt64(&s.http5xx, 0)

	var avgLat, p95Lat, p99Lat, minLat, maxLat float64
	if req > 0 {
		avgLat = float64(lat) / float64(req) / 1000.0
	}

	// 获取当前秒的采样数据，并累积到全局采样列表
	s.latencySampleMu.Lock()
	currentSamples := make([]int64, len(s.latencySamples))
	copy(currentSamples, s.latencySamples)
	s.latencySamples = s.latencySamples[:0]
	s.latencySampleMu.Unlock()

	// 累积到全局采样列表，最多保留10000个样本
	if len(currentSamples) > 0 {
		s.allLatencySampleMu.Lock()
		s.allLatencySamples = append(s.allLatencySamples, currentSamples...)
		// 如果超过10000个样本，保留最新的10000个
		if len(s.allLatencySamples) > 10000 {
			trimStart := len(s.allLatencySamples) - 10000
			s.allLatencySamples = s.allLatencySamples[trimStart:]
		}
		s.allLatencySampleMu.Unlock()
	}

	// 使用累积的采样数据计算 P95/P99
	s.allLatencySampleMu.Lock()
	// 复制一份数据进行排序，避免修改原切片
	allSamples := make([]int64, len(s.allLatencySamples))
	copy(allSamples, s.allLatencySamples)
	s.allLatencySampleMu.Unlock()

	if len(allSamples) > 0 {
		sort.Slice(allSamples, func(i, j int) bool { return allSamples[i] < allSamples[j] })
		p95Idx := int(float64(len(allSamples)) * 0.95)
		p99Idx := int(float64(len(allSamples)) * 0.99)
		if p95Idx >= len(allSamples) {
			p95Idx = len(allSamples) - 1
		}
		if p99Idx >= len(allSamples) {
			p99Idx = len(allSamples) - 1
		}
		p95Lat = float64(allSamples[p95Idx]) / 1000.0
		p99Lat = float64(allSamples[p99Idx]) / 1000.0
		minLat = float64(allSamples[0]) / 1000.0
		maxLat = float64(allSamples[len(allSamples)-1]) / 1000.0
	}

	// 获取系统状态
	s.sysMu.RLock()
	sys := s.sysStats
	s.sysMu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	remaining := 0
	if atomic.LoadInt32(&s.isRunning) == 1 && s.testDuration > 0 {
		elapsed := time.Since(s.testStartTime).Seconds()
		if elapsed < float64(s.testDuration) {
			remaining = s.testDuration - int(elapsed)
		}
	}

	// 确定测试模式
	testMode := "idle"
	if atomic.LoadInt32(&s.isRunning) == 1 {
		if s.enableHTTP {
			testMode = "http"
		} else if s.enableDB && s.enableInsert {
			testMode = "db_write"
		} else if s.enableDB {
			testMode = "db_read"
		} else if s.enableRedis {
			testMode = "redis"
		} else if s.enableMQ {
			testMode = "mq"
		} else {
			testMode = "mixed"
		}
	}

	s.lastStats = Stats{
		QPS:           req,
		AvgLatency:    avgLat,
		P95Latency:    p95Lat,
		P99Latency:    p99Lat,
		MinLatency:    minLat,
		MaxLatency:    maxLat,
		TotalReq:      s.lastStats.TotalReq + req,
		Success:       s.lastStats.Success + succ,
		Failed:        s.lastStats.Failed + fail,
		SuccessQPS:    succ,
		FailedQPS:     fail,
		Concurrency:   s.concurrency,
		MQThroughput:  mq,
		RedisOps:      redis,
		MySQLOps:      mysql,
		HTTPOps:       httpOps,
		ActiveWorkers: atomic.LoadInt32(&s.isRunning),
		RemainingTime: remaining,
		LastError:     s.lastError,
		StopReason:    s.stopReason,
		RedisStatus:   s.canConnectRedis,
		MQStatus:      s.canConnectMQ,
		DBStatus:      s.canConnectDB,
		WriteDBStatus: s.canConnectDB,
		ReadDBStatus:  s.canConnectRead,
		CPUUsage:      sys.cpuUsage,
		MemoryUsed:    sys.memUsed,
		MemoryTotal:   sys.memTotal,
		NetSent:       sys.netSentRate,
		NetRecv:       sys.netRecvRate,
		HTTPStatus2xx: h2xx,
		HTTPStatus4xx: h4xx,
		HTTPStatus5xx: h5xx,
		TestMode:      testMode,
	}

	if atomic.LoadInt32(&s.isRunning) == 1 {
		s.lastStats.ActiveWorkers = int32(s.concurrency)

		// 检查资源限制
		stopTest := false
		var reason string

		// 检查内存使用
		if s.serverMemTotal > 0 {
			memUsedPercent := float64(sys.memUsed) / float64(s.serverMemTotal) * 100
			if memUsedPercent >= float64(s.memLimitPercent) {
				stopTest = true
				reason = fmt.Sprintf("内存使用率 %.1f%% 超过限制 %d%%", memUsedPercent, s.memLimitPercent)
			}
		}

		// 检查CPU使用
		if !stopTest && sys.cpuUsage >= float64(s.cpuLimitPercent) {
			stopTest = true
			reason = fmt.Sprintf("CPU使用率 %.1f%% 超过限制 %d%%", sys.cpuUsage, s.cpuLimitPercent)
		}

		if stopTest {
			s.stopReason = reason
			log.Printf("[Benchmark] %s，自动停止测试", reason)
			go s.StopTest()
		}
	} else if s.lastStats.QPS > 0 {
		s.lastStats.ActiveWorkers = int32(s.concurrency)
	}
}

func (s *Service) GetStats() Stats {
	s.mu.RLock()
	lastStats := s.lastStats
	hasFinal := s.hasFinalStats
	finalStats := s.finalStats
	s.mu.RUnlock()

	var stats Stats

	// 根据状态选择数据源
	if atomic.LoadInt32(&s.isRunning) == 1 {
		// 测试运行中：使用实时统计
		stats = lastStats
	} else if hasFinal {
		// 测试完成 + 有最终数据：使用最终数据
		stats = finalStats
	} else {
		// 空闲且无最终数据：返回干净状态
		stats = Stats{}
	}

	// 始终注入最新的连接状态（无论测试是否在运行）
	stats.RedisStatus = s.canConnectRedis
	stats.MQStatus = s.canConnectMQ
	stats.DBStatus = s.canConnectDB
	stats.WriteDBStatus = s.canConnectDB
	stats.ReadDBStatus = s.canConnectRead

	// 始终注入最新的系统指标
	s.sysMu.RLock()
	stats.CPUUsage = s.sysStats.cpuUsage
	stats.MemoryUsed = s.sysStats.memUsed
	stats.MemoryTotal = s.sysStats.memTotal
	stats.NetSent = s.sysStats.netSentRate
	stats.NetRecv = s.sysStats.netRecvRate
	s.sysMu.RUnlock()

	// 根据运行状态调整返回数据
	if atomic.LoadInt32(&s.isRunning) == 1 {
		// 测试运行中：确保 ActiveWorkers 正确（snapshot 还没跑时 lastStats 可能是 0）
		if stats.ActiveWorkers <= 0 && s.concurrency > 0 {
			stats.ActiveWorkers = int32(s.concurrency)
		}
		// 实时计算 RemainingTime（不依赖 snapshot）
		if s.testDuration > 0 {
			elapsed := time.Since(s.testStartTime).Seconds()
			if elapsed < float64(s.testDuration) {
				stats.RemainingTime = s.testDuration - int(elapsed)
			} else {
				stats.RemainingTime = 0
			}
		}
	} else {
		// 测试完成后，保留最终统计数据
		if hasFinal {
			stats.AvgLatency = finalStats.AvgLatency
			stats.P95Latency = finalStats.P95Latency
			stats.P99Latency = finalStats.P99Latency
			stats.MinLatency = finalStats.MinLatency
			stats.MaxLatency = finalStats.MaxLatency
			stats.TotalReq = finalStats.TotalReq
			stats.Success = finalStats.Success
			stats.Failed = finalStats.Failed
			stats.Concurrency = finalStats.Concurrency
			stats.ActiveWorkers = finalStats.ActiveWorkers
			stats.StopReason = finalStats.StopReason
			stats.QPS = finalStats.QPS
			stats.SuccessQPS = finalStats.SuccessQPS
			stats.FailedQPS = finalStats.FailedQPS
			stats.MQThroughput = finalStats.MQThroughput
			stats.RedisOps = finalStats.RedisOps
			stats.MySQLOps = finalStats.MySQLOps
			stats.HTTPOps = finalStats.HTTPOps
			stats.HTTPStatus2xx = finalStats.HTTPStatus2xx
			stats.HTTPStatus4xx = finalStats.HTTPStatus4xx
			stats.HTTPStatus5xx = finalStats.HTTPStatus5xx
		} else {
			// 没有最终统计数据时，确保关键指标为空
			stats = Stats{
				RedisStatus:   s.canConnectRedis,
				MQStatus:      s.canConnectMQ,
				DBStatus:      s.canConnectDB,
				WriteDBStatus: s.canConnectDB,
				ReadDBStatus:  s.canConnectRead,
				CPUUsage:      s.sysStats.cpuUsage,
				MemoryUsed:    s.sysStats.memUsed,
				MemoryTotal:   s.sysStats.memTotal,
				NetSent:       s.sysStats.netSentRate,
				NetRecv:       s.sysStats.netRecvRate,
				TestMode:      "idle",
			}
		}
		// 空闲时清理部分动态数据
		stats.TestMode = "idle"
		stats.RemainingTime = 0
		stats.ActiveWorkers = 0
	}

	return stats
}

// StartTest 启动压测
func (s *Service) StartTest(concurrency int, durationSeconds int, dbCount int64, mqCount int64, enableInsert bool, memoryLoadMB int, enableRedis, enableDB, enableMQ bool) error {
	if !atomic.CompareAndSwapInt32(&s.isRunning, 0, 1) {
		return fmt.Errorf("测试已在运行中")
	}

	// 强制设定持续时间（防止无限运行导致刷新后无法重新测试）
	if durationSeconds <= 0 {
		durationSeconds = 30
	}
	if durationSeconds > 300 {
		durationSeconds = 300
	}

	// 安全限制：根据是否有网络操作调整最大并发
	maxConcurrency := 10000 // 网络操作时最高并发
	if !enableRedis && !enableDB && !enableMQ {
		maxConcurrency = 50000 // 纯 QPS 无网络可更高
	}
	if concurrency > maxConcurrency {
		concurrency = maxConcurrency
		log.Printf("[Benchmark] 并发数已限制为 %d (安全上限)", maxConcurrency)
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if memoryLoadMB > 512 {
		memoryLoadMB = 512
		log.Printf("[Benchmark] 内存负载已限制为 512MB (安全上限)")
	}

	started := false
	defer func() {
		if !started {
			atomic.StoreInt32(&s.isRunning, 0)
			if s.cancel != nil {
				s.cancel()
			}
		}
	}()

	// 清理旧语句
	if s.insertStmt != nil {
		s.insertStmt.Close()
		s.insertStmt = nil
	}
	if s.readStmt != nil {
		s.readStmt.Close()
		s.readStmt = nil
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())
	if durationSeconds > 0 {
		s.ctx, s.cancel = context.WithTimeout(context.Background(), time.Duration(durationSeconds)*time.Second)
	}

	// 配置
	s.concurrency = concurrency
	s.targetDBCount = dbCount
	s.targetMQCount = mqCount
	s.testStartTime = time.Now()
	s.testDuration = durationSeconds
	s.lastError = ""
	s.stopReason = "" // 重置停止原因
	atomic.StoreInt64(&s.totalDBOps, 0)
	atomic.StoreInt64(&s.totalMQOps, 0)
	s.enableInsert = enableInsert
	s.memoryLoadMB = memoryLoadMB
	s.enableRedis = enableRedis
	s.enableDB = enableDB
	s.enableMQ = enableMQ
	s.enableHTTP = false

	// 清空累积延迟采样（每次新测试重新开始）
	s.allLatencySampleMu.Lock()
	s.allLatencySamples = s.allLatencySamples[:0]
	s.allLatencySampleMu.Unlock()

	// 内存分配
	if memoryLoadMB > 0 {
		size := memoryLoadMB * 1024 * 1024
		s.memoryBlock = make([]byte, size)
		for i := 0; i < size; i += 4096 {
			s.memoryBlock[i] = 1
		}
	} else {
		s.memoryBlock = nil
	}

	// 连接检查
	if s.enableRedis {
		if rdb := facade.Redis(); rdb != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := rdb.Ping(ctx).Err(); err != nil {
				cancel()
				return fmt.Errorf("Redis连接失败: %v", err)
			}
			cancel()
			s.canConnectRedis = true
		} else {
			return fmt.Errorf("Redis服务未初始化")
		}
	}

	if s.enableMQ {
		if mq := facade.RabbitMQ(); mq != nil && mq.IsConnected() {
			s.canConnectMQ = true
		} else {
			return fmt.Errorf("RabbitMQ服务未初始化或连接失败")
		}
	}

	if s.enableDB {
		if db.GlobalDB != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := db.GlobalDB.PingContext(ctx); err != nil {
				cancel()
				return fmt.Errorf("主数据库(Write)连接失败: %v", err)
			}
			cancel()
			s.canConnectDB = true
		} else {
			return fmt.Errorf("主数据库服务未初始化")
		}

		// 检查从库
		if len(db.ReadDBs) > 0 {
			for i, rdb := range db.ReadDBs {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := rdb.PingContext(ctx); err != nil {
					cancel()
					return fmt.Errorf("从数据库(Read-%d)连接失败: %v", i, err)
				}
				cancel()
			}
			s.canConnectRead = true
		} else {
			s.canConnectRead = s.canConnectDB
		}

		// 确保测试数据
		if !s.enableInsert && db.CurrentDriver == "mysql" && s.canConnectDB {
			var count int
			err := db.GlobalDB.QueryRow("SELECT COUNT(*) FROM platform_admin").Scan(&count)
			if err == nil && count < 1000 {
				log.Println("[Benchmark] 自动填充测试数据到 platform_admin...")
				stmt, err := db.GlobalDB.Prepare("INSERT INTO platform_admin (info_name, account_phone, account_email) VALUES (?, ?, ?)")
				if err == nil {
					for i := 0; i < 1000; i++ {
						stmt.Exec(fmt.Sprintf("user_%d", i), fmt.Sprintf("1380000%04d", i), fmt.Sprintf("test%d@test.com", i))
					}
					stmt.Close()
				}
			}
		}

		// 预编译语句
		var err error
		if s.enableInsert {
			var query string
			switch db.CurrentDriver {
			case "postgres":
				query = "INSERT INTO platform_admin (info_name, account_phone, account_email) VALUES ($1, $2, $3)"
			case "sqlserver":
				query = "INSERT INTO platform_admin (info_name, account_phone, account_email) VALUES (@p1, @p2, @p3)"
			default:
				query = "INSERT INTO platform_admin (info_name, account_phone, account_email) VALUES (?, ?, ?)"
			}
			s.insertStmt, err = db.GlobalDB.Prepare(query)
			if err != nil {
				log.Printf("[Benchmark] 预编译INSERT失败: %v", err)
			}
		} else {
			query := "SELECT info_name FROM platform_admin WHERE id = ?"
			if db.CurrentDriver == "postgres" {
				query = "SELECT info_name FROM platform_admin WHERE id = $1"
			} else if db.CurrentDriver == "sqlserver" {
				query = "SELECT info_name FROM platform_admin WHERE id = @p1"
			}
			s.readStmt, err = db.GlobalDB.Prepare(query)
			if err != nil {
				log.Printf("[Benchmark] 预编译SELECT失败: %v", err)
			}
		}
	}

	// 重置计数器
	s.mu.Lock()
	s.lastStats = Stats{}
	s.finalStats = Stats{}
	s.hasFinalStats = false
	s.mu.Unlock()
	s.resetCounters()

	// 定时停止
	if durationSeconds > 0 {
		time.AfterFunc(time.Duration(durationSeconds)*time.Second, func() {
			s.StopTest()
		})
	}

	// 启动 Worker 池
	for i := 0; i < concurrency; i++ {
		s.wg.Add(1)
		go s.worker(i)
	}

	// 等待完成
	go func() {
		s.wg.Wait()
		atomic.StoreInt32(&s.isRunning, 0)
		if s.cancel != nil {
			s.cancel()
		}
	}()

	started = true

	// 启动测试监控（仅在测试期间运行）
	monitorCtx, monitorCancel := context.WithCancel(s.ctx)
	s.monitorCancel = monitorCancel
	go s.testMonitorLoop(monitorCtx)

	return nil
}

// StartHTTPTest 启动 HTTP 压测
func (s *Service) StartHTTPTest(concurrency int, durationSeconds int, targetURL string, method string) error {
	if !atomic.CompareAndSwapInt32(&s.isRunning, 0, 1) {
		return fmt.Errorf("测试已在运行中")
	}

	if durationSeconds <= 0 {
		durationSeconds = 30
	}
	if durationSeconds > 300 {
		durationSeconds = 300
	}

	if concurrency > 1000 {
		concurrency = 1000
		log.Printf("[Benchmark] HTTP 并发数已限制为 1000 (安全上限)")
	}
	if concurrency < 1 {
		concurrency = 1
	}

	started := false
	defer func() {
		if !started {
			atomic.StoreInt32(&s.isRunning, 0)
			if s.cancel != nil {
				s.cancel()
			}
		}
	}()

	s.ctx, s.cancel = context.WithCancel(context.Background())
	if durationSeconds > 0 {
		s.ctx, s.cancel = context.WithTimeout(context.Background(), time.Duration(durationSeconds)*time.Second)
	}

	s.concurrency = concurrency
	s.testStartTime = time.Now()
	s.testDuration = durationSeconds
	s.lastError = ""
	s.httpTarget = targetURL
	s.httpMethod = method
	s.enableHTTP = true
	s.enableDB = false
	s.enableRedis = false
	s.enableMQ = false

	// 清空累积延迟采样（每次新测试重新开始）
	s.allLatencySampleMu.Lock()
	s.allLatencySamples = s.allLatencySamples[:0]
	s.allLatencySampleMu.Unlock()

	// 创建优化的 HTTP Client（高性能配置）
	s.httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:          concurrency * 2,
			MaxIdleConnsPerHost:   concurrency * 2,
			MaxConnsPerHost:       concurrency * 2,
			IdleConnTimeout:       90 * time.Second,
			DisableKeepAlives:     false,
			DisableCompression:    true,
			ResponseHeaderTimeout: 5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// 跳过 TLS 验证（压测场景）
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// 重置计数器
	s.mu.Lock()
	s.lastStats = Stats{}
	s.finalStats = Stats{}
	s.hasFinalStats = false
	s.mu.Unlock()
	s.resetCounters()

	// 定时停止
	if durationSeconds > 0 {
		time.AfterFunc(time.Duration(durationSeconds)*time.Second, func() {
			s.StopTest()
		})
	}

	// 启动 Worker
	for i := 0; i < concurrency; i++ {
		s.wg.Add(1)
		go s.httpWorker(i)
	}

	go func() {
		s.wg.Wait()
		atomic.StoreInt32(&s.isRunning, 0)
		if s.cancel != nil {
			s.cancel()
		}
	}()

	started = true

	monitorCtx, monitorCancel := context.WithCancel(s.ctx)
	s.monitorCancel = monitorCancel
	go s.testMonitorLoop(monitorCtx)

	return nil
}

func (s *Service) resetCounters() {
	atomic.StoreInt64(&s.reqCount, 0)
	atomic.StoreInt64(&s.latencySum, 0)
	atomic.StoreInt64(&s.successCount, 0)
	atomic.StoreInt64(&s.failCount, 0)
	atomic.StoreInt64(&s.mqCount, 0)
	atomic.StoreInt64(&s.redisCount, 0)
	atomic.StoreInt64(&s.mysqlCount, 0)
	atomic.StoreInt64(&s.httpCount, 0)
	atomic.StoreInt64(&s.http2xx, 0)
	atomic.StoreInt64(&s.http4xx, 0)
	atomic.StoreInt64(&s.http5xx, 0)
	atomic.StoreInt64(&s.totalDBOps, 0)
	atomic.StoreInt64(&s.totalMQOps, 0)

	s.latencySampleMu.Lock()
	s.latencySamples = s.latencySamples[:0]
	s.latencySampleMu.Unlock()

	s.allLatencySampleMu.Lock()
	s.allLatencySamples = s.allLatencySamples[:0]
	s.allLatencySampleMu.Unlock()
}

// IsRunning 返回当前是否有测试在运行
func (s *Service) IsRunning() bool {
	return atomic.LoadInt32(&s.isRunning) == 1
}

func (s *Service) Reset() {
	// 先停止任何正在运行的测试
	if atomic.LoadInt32(&s.isRunning) == 1 {
		s.StopTest()
	}
	// 强制确保 isRunning 归零（防止 StopTest 超时后仍卡住）
	atomic.StoreInt32(&s.isRunning, 0)

	s.mu.Lock()
	s.lastStats = Stats{}
	s.finalStats = Stats{}
	s.hasFinalStats = false
	s.mu.Unlock()

	// 清空累积延迟采样
	s.allLatencySampleMu.Lock()
	s.allLatencySamples = s.allLatencySamples[:0]
	s.allLatencySampleMu.Unlock()

	s.resetCounters()
	s.lastError = ""
}

func (s *Service) StopTest() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.monitorCancel != nil {
		s.monitorCancel()
	}

	// 关闭语句
	if s.insertStmt != nil {
		s.insertStmt.Close()
		s.insertStmt = nil
	}
	if s.readStmt != nil {
		s.readStmt.Close()
		s.readStmt = nil
	}

	// 清除内存
	s.mu.Lock()
	s.memoryBlock = nil
	s.mu.Unlock()

	// 等待 Worker 完成（带超时）
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		log.Printf("[Benchmark] Worker 等待超时，强制停止")
	}

	// 执行最后一次 snapshot 确保所有计数都被统计
	s.snapshot()

	// 保存最终统计数据（用于测试完成后显示 P95/P99 等延迟指标）
	s.mu.Lock()
	// 保存之前清除 lastError，避免旧错误显示
	s.lastStats.LastError = ""
	s.finalStats = s.lastStats
	s.hasFinalStats = true
	s.mu.Unlock()

	// 清空 lastError
	s.lastError = ""

	atomic.StoreInt32(&s.isRunning, 0)
}

// httpWorker HTTP 压测 worker（优化版：减少内存分配和 CPU 开销）
func (s *Service) httpWorker(id int) {
	defer s.wg.Done()

	var lReq, lLat, lSucc, lFail, lHTTP int64
	var loopCount int64

	flushCounters := func() {
		if lReq > 0 {
			atomic.AddInt64(&s.reqCount, lReq)
			atomic.AddInt64(&s.latencySum, lLat)
			atomic.AddInt64(&s.successCount, lSucc)
			atomic.AddInt64(&s.failCount, lFail)
			atomic.AddInt64(&s.httpCount, lHTTP)
			lReq, lLat, lSucc, lFail, lHTTP = 0, 0, 0, 0, 0
		}
	}
	defer flushCounters()

	// 预创建请求模板（GET 请求可复用）
	var reqTemplate *http.Request
	var err error
	if s.httpMethod == "GET" {
		reqTemplate, err = http.NewRequest(http.MethodGet, s.httpTarget, nil)
		if err != nil {
			log.Printf("[Benchmark] Worker %d: 创建请求失败: %v", id, err)
			return
		}
	}

	// 本地状态码计数器（减少 atomic 操作）
	var local2xx, local4xx, local5xx int64

	for {
		select {
		case <-s.ctx.Done():
			// 退出前刷新本地状态码计数
			if local2xx > 0 {
				atomic.AddInt64(&s.http2xx, local2xx)
			}
			if local4xx > 0 {
				atomic.AddInt64(&s.http4xx, local4xx)
			}
			if local5xx > 0 {
				atomic.AddInt64(&s.http5xx, local5xx)
			}
			return
		default:
			loopCount++
			start := time.Now()

			var resp *http.Response
			var req *http.Request

			// 复用 GET 请求模板，POST 需要每次创建
			if s.httpMethod == "GET" && reqTemplate != nil {
				req = reqTemplate.Clone(s.ctx)
			} else {
				req, err = http.NewRequestWithContext(s.ctx, s.httpMethod, s.httpTarget, nil)
				if err != nil {
					lReq++
					lFail++
					continue
				}
			}

			resp, err = s.httpClient.Do(req)
			elapsed := time.Since(start).Microseconds()

			lReq++
			lLat += elapsed
			lHTTP++

			// 采样延迟（每 50 次采样 1 次，减少锁竞争）
			if loopCount%50 == 0 {
				s.latencySampleMu.Lock()
				if len(s.latencySamples) < 10000 {
					s.latencySamples = append(s.latencySamples, elapsed)
				}
				s.latencySampleMu.Unlock()
			}

			if err != nil {
				lFail++
			} else {
				// 限制读取响应体大小（最多 1KB），避免大响应体消耗 CPU
				io.CopyN(io.Discard, resp.Body, 1024)
				resp.Body.Close()

				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					lSucc++
					local2xx++
				} else if resp.StatusCode >= 400 && resp.StatusCode < 500 {
					lFail++
					local4xx++
				} else if resp.StatusCode >= 500 {
					lFail++
					local5xx++
				} else {
					lSucc++
				}
			}

			// 每 100 次请求刷新一次计数器并让出 CPU
			if loopCount%100 == 0 {
				runtime.Gosched()
				flushCounters()
				// 批量刷新状态码计数
				if local2xx > 0 {
					atomic.AddInt64(&s.http2xx, local2xx)
					local2xx = 0
				}
				if local4xx > 0 {
					atomic.AddInt64(&s.http4xx, local4xx)
					local4xx = 0
				}
				if local5xx > 0 {
					atomic.AddInt64(&s.http5xx, local5xx)
					local5xx = 0
				}
			}
		}
	}
}

func (s *Service) worker(id int) {
	defer s.wg.Done()

	r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
	var loopCount int64

	var rdb = facade.Redis()

	var lReq, lLat, lSucc, lFail, lMQ, lRedis, lMysql int64

	flushCounters := func() {
		if lReq > 0 {
			atomic.AddInt64(&s.reqCount, lReq)
			atomic.AddInt64(&s.latencySum, lLat)
			atomic.AddInt64(&s.successCount, lSucc)
			atomic.AddInt64(&s.failCount, lFail)
			atomic.AddInt64(&s.mqCount, lMQ)
			atomic.AddInt64(&s.redisCount, lRedis)
			atomic.AddInt64(&s.mysqlCount, lMysql)
			lReq, lLat, lSucc, lFail, lMQ, lRedis, lMysql = 0, 0, 0, 0, 0, 0, 0
		}
	}

	defer flushCounters()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			loopCount++
			start := time.Now()

			canDoDB := true
			canDoMQ := true

			if s.targetDBCount > 0 {
				if atomic.LoadInt64(&s.totalDBOps) >= s.targetDBCount {
					canDoDB = false
				}
			}

			if s.targetMQCount > 0 {
				if atomic.LoadInt64(&s.totalMQOps) >= s.targetMQCount {
					canDoMQ = false
				}
			}

			if s.targetDBCount > 0 && s.targetMQCount > 0 && !canDoDB && !canDoMQ {
				return
			}

			needNetwork := (s.enableRedis && s.canConnectRedis && rdb != nil) ||
				(s.enableDB && canDoDB && s.canConnectDB) ||
				(s.enableMQ && canDoMQ && s.canConnectMQ)

			var redisErr, dbErr error
			didRedis, didDB := false, false

			if needNetwork {
				opCtx, cancel := context.WithTimeout(s.ctx, 3*time.Second)

				if s.enableRedis && s.canConnectRedis && rdb != nil {
					didRedis = true
					key := "bench:" + strconv.Itoa(id) + ":" + strconv.Itoa(int(loopCount%1000))
					val := strconv.FormatInt(time.Now().UnixNano(), 10)
					if loopCount%2 == 0 {
						if err := rdb.Set(opCtx, key, val, 60*time.Second).Err(); err != nil {
							redisErr = err
						} else {
							lRedis++
						}
					} else {
						if _, err := rdb.Get(opCtx, key).Result(); err != nil && err.Error() != "redis: nil" {
							redisErr = err
						} else {
							lRedis++
						}
					}
				}

				if s.enableDB && canDoDB && s.canConnectDB {
					if s.targetDBCount <= 0 || atomic.AddInt64(&s.totalDBOps, 1) <= s.targetDBCount {
						didDB = true
						if s.enableInsert {
							if s.insertStmt != nil {
								ts := time.Now().UnixNano()
								rnd := r.Int()
								val := "bench_" + strconv.FormatInt(ts, 10) + "_" + strconv.Itoa(rnd)
								_, err := s.insertStmt.ExecContext(opCtx, val, val, val)
								if err != nil {
									dbErr = err
								}
							}
						} else {
							randomID := r.Intn(1000) + 1
							var val string
							readDB := db.GetReadDB()
							if readDB != nil && readDB != db.GlobalDB {
								query := "SELECT info_name FROM platform_admin WHERE id = ?"
								if db.CurrentDriver == "postgres" {
									query = "SELECT info_name FROM platform_admin WHERE id = $1"
								}
								err := readDB.QueryRowContext(opCtx, query, randomID).Scan(&val)
								if err != nil && err != sql.ErrNoRows {
									dbErr = err
								}
							} else if s.readStmt != nil {
								err := s.readStmt.QueryRowContext(opCtx, randomID).Scan(&val)
								if err != nil && err != sql.ErrNoRows {
									dbErr = err
								}
							}
						}
						lMysql++
					}
				}

				cancel()

				if s.enableMQ && canDoMQ && s.canConnectMQ {
					if s.targetMQCount <= 0 || atomic.AddInt64(&s.totalMQOps, 1) <= s.targetMQCount {
						lMQ++
					}
				}
			}

			if s.memoryBlock != nil && len(s.memoryBlock) > 0 {
				_ = s.memoryBlock[time.Now().UnixNano()%int64(len(s.memoryBlock))]
			}

			elapsed := time.Since(start).Microseconds()
			lReq++
			lLat += elapsed

			if loopCount%10 == 0 {
				s.latencySampleMu.Lock()
				if len(s.latencySamples) < 10000 {
					s.latencySamples = append(s.latencySamples, elapsed)
				}
				s.latencySampleMu.Unlock()
			}

			hasFailure := false
			if didRedis && redisErr != nil {
				hasFailure = true
			}
			if didDB && dbErr != nil {
				hasFailure = true
			}

			if !hasFailure {
				lSucc++
			} else {
				lFail++
				if lFail == 1 || lFail%1000 == 0 {
					if redisErr != nil {
						s.lastError = redisErr.Error()
					}
					if dbErr != nil {
						s.lastError = dbErr.Error()
					}
				}
			}

			if loopCount%50 == 0 || elapsed > 100000 {
				runtime.Gosched()
				flushCounters()
			}
		}
	}
}
