package performance

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"vigo/framework/db"
	"vigo/framework/facade"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type Result struct {
	Name         string        `json:"name"`
	TotalOps     int64         `json:"total_ops"`
	Duration     time.Duration `json:"duration"`
	OpsPerSecond float64       `json:"ops_per_second"`
	AvgLatency   time.Duration `json:"avg_latency"`
	MinLatency   time.Duration `json:"min_latency"`
	MaxLatency   time.Duration `json:"max_latency"`
	ErrorCount   int64         `json:"error_count"`
	SuccessRate  float64       `json:"success_rate"`
	Goroutines   int           `json:"goroutines"`
	MemoryMB     float64       `json:"memory_mb"`
	CPUUsage     float64       `json:"cpu_usage"`
}

type Config struct {
	Name        string
	Concurrency int
	Iterations  int
	Duration    time.Duration
	Warmup      int
}

type Service struct {
	results map[string]*Result
	mu      sync.RWMutex
}

func NewService() *Service {
	return &Service{
		results: make(map[string]*Result),
	}
}

func (s *Service) RunDatabaseBenchmark(cfg Config) (*Result, error) {
	result := &Result{
		Name: cfg.Name,
	}

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Iterations <= 0 {
		cfg.Iterations = 1000
	}

	var wg sync.WaitGroup
	var totalOps int64
	var errorCount int64
	var totalLatency int64
	var minLatency int64 = int64(time.Hour)
	var maxLatency int64
	var m runtime.MemStats

	startTime := time.Now()
	opsPerWorker := cfg.Iterations / cfg.Concurrency

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				opStart := time.Now()

				_, err := db.Table("benchmark_test").Where("id = ?", 1).Find()

				opLatency := time.Since(opStart)
				latencyNs := opLatency.Nanoseconds()

				atomic.AddInt64(&totalOps, 1)
				atomic.AddInt64(&totalLatency, latencyNs)

				for {
					old := atomic.LoadInt64(&minLatency)
					if latencyNs >= old || atomic.CompareAndSwapInt64(&minLatency, old, latencyNs) {
						break
					}
				}

				for {
					old := atomic.LoadInt64(&maxLatency)
					if latencyNs <= old || atomic.CompareAndSwapInt64(&maxLatency, old, latencyNs) {
						break
					}
				}

				if err != nil && err != sql.ErrNoRows {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}()
	}

	wg.Wait()
	result.Duration = time.Since(startTime)

	runtime.ReadMemStats(&m)
	result.MemoryMB = float64(m.Alloc) / 1024 / 1024
	result.Goroutines = runtime.NumGoroutine()

	cpuPercent, _ := cpu.Percent(time.Second, false)
	if len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}

	result.TotalOps = totalOps
	result.ErrorCount = errorCount
	if result.Duration.Seconds() > 0 {
		result.OpsPerSecond = float64(totalOps) / result.Duration.Seconds()
	}
	if totalOps > 0 {
		result.AvgLatency = time.Duration(totalLatency / totalOps)
		result.MinLatency = time.Duration(minLatency)
		result.MaxLatency = time.Duration(maxLatency)
		result.SuccessRate = float64(totalOps-errorCount) / float64(totalOps) * 100
	} else {
		result.AvgLatency = 0
		result.MinLatency = 0
		result.MaxLatency = 0
		result.SuccessRate = 0
	}

	s.mu.Lock()
	s.results[cfg.Name] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) RunDatabaseWriteBenchmark(cfg Config) (*Result, error) {
	result := &Result{
		Name: cfg.Name,
	}

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Iterations <= 0 {
		cfg.Iterations = 1000
	}

	var wg sync.WaitGroup
	var totalOps int64
	var errorCount int64
	var totalLatency int64
	var minLatency int64 = int64(time.Hour)
	var maxLatency int64
	var m runtime.MemStats

	startTime := time.Now()
	opsPerWorker := cfg.Iterations / cfg.Concurrency

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				opStart := time.Now()

				data := map[string]interface{}{
					"name":       fmt.Sprintf("test_%d_%d", workerID, j),
					"value":      j,
					"created_at": time.Now(),
				}
				_, err := db.Table("benchmark_test").Insert(data)

				opLatency := time.Since(opStart)
				latencyNs := opLatency.Nanoseconds()

				atomic.AddInt64(&totalOps, 1)
				atomic.AddInt64(&totalLatency, latencyNs)

				for {
					old := atomic.LoadInt64(&minLatency)
					if latencyNs >= old || atomic.CompareAndSwapInt64(&minLatency, old, latencyNs) {
						break
					}
				}

				for {
					old := atomic.LoadInt64(&maxLatency)
					if latencyNs <= old || atomic.CompareAndSwapInt64(&maxLatency, old, latencyNs) {
						break
					}
				}

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(startTime)

	runtime.ReadMemStats(&m)
	result.MemoryMB = float64(m.Alloc) / 1024 / 1024
	result.Goroutines = runtime.NumGoroutine()

	cpuPercent, _ := cpu.Percent(time.Second, false)
	if len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}

	result.TotalOps = totalOps
	result.ErrorCount = errorCount
	if result.Duration.Seconds() > 0 {
		result.OpsPerSecond = float64(totalOps) / result.Duration.Seconds()
	}
	if totalOps > 0 {
		result.AvgLatency = time.Duration(totalLatency / totalOps)
		result.MinLatency = time.Duration(minLatency)
		result.MaxLatency = time.Duration(maxLatency)
		result.SuccessRate = float64(totalOps-errorCount) / float64(totalOps) * 100
	} else {
		result.AvgLatency = 0
		result.MinLatency = 0
		result.MaxLatency = 0
		result.SuccessRate = 0
	}

	s.mu.Lock()
	s.results[cfg.Name] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) RunRedisBenchmark(cfg Config) (*Result, error) {
	result := &Result{
		Name: cfg.Name,
	}

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Iterations <= 0 {
		cfg.Iterations = 1000
	}

	var wg sync.WaitGroup
	var totalOps int64
	var errorCount int64
	var totalLatency int64
	var minLatency int64 = int64(time.Hour)
	var maxLatency int64
	var m runtime.MemStats

	startTime := time.Now()
	opsPerWorker := cfg.Iterations / cfg.Concurrency

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				opStart := time.Now()

				key := fmt.Sprintf("benchmark:%d:%d", workerID, j)
				value := fmt.Sprintf("value_%d_%d", workerID, j)

				rdb := facade.Redis()
				var err error
				if rdb != nil {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					err = rdb.Set(ctx, key, value, time.Minute).Err()
					if err == nil {
						_, err = rdb.Get(ctx, key).Result()
					}
					rdb.Del(ctx, key)
					cancel()
				} else {
					err = fmt.Errorf("redis not available")
				}

				opLatency := time.Since(opStart)
				latencyNs := opLatency.Nanoseconds()

				atomic.AddInt64(&totalOps, 1)
				atomic.AddInt64(&totalLatency, latencyNs)

				for {
					old := atomic.LoadInt64(&minLatency)
					if latencyNs >= old || atomic.CompareAndSwapInt64(&minLatency, old, latencyNs) {
						break
					}
				}

				for {
					old := atomic.LoadInt64(&maxLatency)
					if latencyNs <= old || atomic.CompareAndSwapInt64(&maxLatency, old, latencyNs) {
						break
					}
				}

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(startTime)

	runtime.ReadMemStats(&m)
	result.MemoryMB = float64(m.Alloc) / 1024 / 1024
	result.Goroutines = runtime.NumGoroutine()

	cpuPercent, _ := cpu.Percent(time.Second, false)
	if len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}

	result.TotalOps = totalOps
	result.ErrorCount = errorCount
	if result.Duration.Seconds() > 0 {
		result.OpsPerSecond = float64(totalOps) / result.Duration.Seconds()
	}
	if totalOps > 0 {
		result.AvgLatency = time.Duration(totalLatency / totalOps)
		result.MinLatency = time.Duration(minLatency)
		result.MaxLatency = time.Duration(maxLatency)
		result.SuccessRate = float64(totalOps-errorCount) / float64(totalOps) * 100
	} else {
		result.AvgLatency = 0
		result.MinLatency = 0
		result.MaxLatency = 0
		result.SuccessRate = 0
	}

	s.mu.Lock()
	s.results[cfg.Name] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) RunQueueBenchmark(cfg Config) (*Result, error) {
	result := &Result{
		Name: cfg.Name,
	}

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}
	if cfg.Iterations <= 0 {
		cfg.Iterations = 1000
	}

	var wg sync.WaitGroup
	var totalOps int64
	var errorCount int64
	var totalLatency int64
	var minLatency int64 = int64(time.Hour)
	var maxLatency int64
	var m runtime.MemStats

	queue := make(chan interface{}, cfg.Iterations)

	startTime := time.Now()
	opsPerWorker := cfg.Iterations / cfg.Concurrency

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < opsPerWorker; j++ {
				opStart := time.Now()

				data := map[string]interface{}{
					"worker_id": workerID,
					"job_id":    j,
					"timestamp": time.Now().UnixNano(),
				}

				select {
				case queue <- data:
					atomic.AddInt64(&totalOps, 1)
				default:
					atomic.AddInt64(&errorCount, 1)
				}

				opLatency := time.Since(opStart)
				latencyNs := opLatency.Nanoseconds()

				atomic.AddInt64(&totalLatency, latencyNs)

				for {
					old := atomic.LoadInt64(&minLatency)
					if latencyNs >= old || atomic.CompareAndSwapInt64(&minLatency, old, latencyNs) {
						break
					}
				}

				for {
					old := atomic.LoadInt64(&maxLatency)
					if latencyNs <= old || atomic.CompareAndSwapInt64(&maxLatency, old, latencyNs) {
						break
					}
				}
			}
		}(i)
	}

	wg.Wait()
	result.Duration = time.Since(startTime)

	close(queue)
	for range queue {
	}

	runtime.ReadMemStats(&m)
	result.MemoryMB = float64(m.Alloc) / 1024 / 1024
	result.Goroutines = runtime.NumGoroutine()

	cpuPercent, _ := cpu.Percent(time.Second, false)
	if len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}

	result.TotalOps = totalOps
	result.ErrorCount = errorCount
	if result.Duration.Seconds() > 0 {
		result.OpsPerSecond = float64(totalOps) / result.Duration.Seconds()
	}
	if totalOps > 0 {
		result.AvgLatency = time.Duration(totalLatency / totalOps)
		result.MinLatency = time.Duration(minLatency)
		result.MaxLatency = time.Duration(maxLatency)
		result.SuccessRate = float64(totalOps-errorCount) / float64(totalOps) * 100
	} else {
		result.AvgLatency = 0
		result.MinLatency = 0
		result.MaxLatency = 0
		result.SuccessRate = 0
	}

	s.mu.Lock()
	s.results[cfg.Name] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) RunQPSBenchmark(cfg Config) (*Result, error) {
	result := &Result{
		Name: cfg.Name,
	}

	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 100
	}
	if cfg.Duration <= 0 {
		cfg.Duration = 10 * time.Second
	}

	var wg sync.WaitGroup
	var totalOps int64
	var errorCount int64
	var totalLatency int64
	var minLatency int64 = int64(time.Hour)
	var maxLatency int64
	var m runtime.MemStats
	var stopFlag int64

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	startTime := time.Now()

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadInt64(&stopFlag) == 0 {
				select {
				case <-ctx.Done():
					atomic.StoreInt64(&stopFlag, 1)
					return
				default:
					opStart := time.Now()

					atomic.AddInt64(&totalOps, 1)

					opLatency := time.Since(opStart)
					latencyNs := opLatency.Nanoseconds()

					atomic.AddInt64(&totalLatency, latencyNs)

					for {
						old := atomic.LoadInt64(&minLatency)
						if latencyNs >= old || atomic.CompareAndSwapInt64(&minLatency, old, latencyNs) {
							break
						}
					}

					for {
						old := atomic.LoadInt64(&maxLatency)
						if latencyNs <= old || atomic.CompareAndSwapInt64(&maxLatency, old, latencyNs) {
							break
						}
					}
				}
			}
		}()
	}

	wg.Wait()
	result.Duration = time.Since(startTime)

	runtime.ReadMemStats(&m)
	result.MemoryMB = float64(m.Alloc) / 1024 / 1024
	result.Goroutines = runtime.NumGoroutine()

	cpuPercent, _ := cpu.Percent(time.Second, false)
	if len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}

	result.TotalOps = totalOps
	result.ErrorCount = errorCount
	if result.Duration.Seconds() > 0 {
		result.OpsPerSecond = float64(totalOps) / result.Duration.Seconds()
	}
	if totalOps > 0 {
		result.AvgLatency = time.Duration(totalLatency / totalOps)
		result.MinLatency = time.Duration(minLatency)
		result.MaxLatency = time.Duration(maxLatency)
		result.SuccessRate = 100.0
	} else {
		result.AvgLatency = 0
		result.MinLatency = 0
		result.MaxLatency = 0
		result.SuccessRate = 0
	}

	s.mu.Lock()
	s.results[cfg.Name] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) RunMemoryBenchmark(cfg Config) (*Result, error) {
	result := &Result{
		Name: cfg.Name,
	}

	if cfg.Iterations <= 0 {
		cfg.Iterations = 10000
	}

	var m runtime.MemStats
	startTime := time.Now()

	data := make([][]byte, cfg.Iterations)
	for i := 0; i < cfg.Iterations; i++ {
		data[i] = make([]byte, 1024)
	}

	result.Duration = time.Since(startTime)

	runtime.ReadMemStats(&m)
	result.MemoryMB = float64(m.Alloc) / 1024 / 1024
	result.Goroutines = runtime.NumGoroutine()
	result.TotalOps = int64(cfg.Iterations)
	if result.Duration.Seconds() > 0 {
		result.OpsPerSecond = float64(cfg.Iterations) / result.Duration.Seconds()
	}
	result.SuccessRate = 100.0

	cpuPercent, _ := cpu.Percent(time.Second, false)
	if len(cpuPercent) > 0 {
		result.CPUUsage = cpuPercent[0]
	}

	s.mu.Lock()
	s.results[cfg.Name] = result
	s.mu.Unlock()

	return result, nil
}

func (s *Service) RunAllBenchmarks(cfg Config) (map[string]*Result, error) {
	results := make(map[string]*Result)

	if dbResult, err := s.RunDatabaseBenchmark(Config{
		Name:        "database_read",
		Concurrency: cfg.Concurrency,
		Iterations:  cfg.Iterations,
	}); err == nil {
		results["database_read"] = dbResult
	}

	if dbWriteResult, err := s.RunDatabaseWriteBenchmark(Config{
		Name:        "database_write",
		Concurrency: cfg.Concurrency,
		Iterations:  cfg.Iterations / 10,
	}); err == nil {
		results["database_write"] = dbWriteResult
	}

	if redisResult, err := s.RunRedisBenchmark(Config{
		Name:        "redis",
		Concurrency: cfg.Concurrency,
		Iterations:  cfg.Iterations,
	}); err == nil {
		results["redis"] = redisResult
	}

	if queueResult, err := s.RunQueueBenchmark(Config{
		Name:        "queue",
		Concurrency: cfg.Concurrency,
		Iterations:  cfg.Iterations,
	}); err == nil {
		results["queue"] = queueResult
	}

	if qpsResult, err := s.RunQPSBenchmark(Config{
		Name:        "qps",
		Concurrency: cfg.Concurrency,
		Duration:    cfg.Duration,
	}); err == nil {
		results["qps"] = qpsResult
	}

	if memResult, err := s.RunMemoryBenchmark(Config{
		Name:       "memory",
		Iterations: cfg.Iterations,
	}); err == nil {
		results["memory"] = memResult
	}

	return results, nil
}

func (s *Service) GetResult(name string) (*Result, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result, ok := s.results[name]
	return result, ok
}

func (s *Service) GetAllResults() map[string]*Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := make(map[string]*Result, len(s.results))
	for k, v := range s.results {
		results[k] = v
	}
	return results
}

func (s *Service) ClearResults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results = make(map[string]*Result)
}

func GetSystemInfo() map[string]interface{} {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	cpuPercent, _ := cpu.Percent(time.Second, false)
	cpuPercentAvg := 0.0
	if len(cpuPercent) > 0 {
		cpuPercentAvg = cpuPercent[0]
	}

	memInfo, _ := mem.VirtualMemory()
	hostInfo, _ := host.Info()

	currentProcess, _ := process.NewProcess(int32(os.Getpid()))

	systemInfo := map[string]interface{}{
		"goroutines":      runtime.NumGoroutine(),
		"cpu_cores":       runtime.NumCPU(),
		"cpu_percent":     cpuPercentAvg,
		"go_version":      runtime.Version(),
		"memory_alloc":    float64(m.Alloc) / 1024 / 1024,
		"memory_total":    float64(m.TotalAlloc) / 1024 / 1024,
		"memory_sys":      float64(m.Sys) / 1024 / 1024,
		"memory_heap":     float64(m.HeapAlloc) / 1024 / 1024,
		"memory_heap_sys": float64(m.HeapSys) / 1024 / 1024,
		"memory_lookups":  m.Lookups,
		"memory_mallocs":  m.Mallocs,
		"memory_frees":    m.Frees,
		"gc_cycles":       m.NumGC,
		"gc_pause_total":  m.PauseTotalNs,
		"gc_pause_avg":    0,
		"status":          "ok",
	}

	if hostInfo != nil {
		systemInfo["host"] = map[string]interface{}{
			"hostname":         hostInfo.Hostname,
			"uptime":           hostInfo.Uptime,
			"boot_time":        hostInfo.BootTime,
			"procs":            hostInfo.Procs,
			"os":               hostInfo.OS,
			"platform":         hostInfo.Platform,
			"platform_family":  hostInfo.PlatformFamily,
			"platform_version": hostInfo.PlatformVersion,
			"kernel_version":   hostInfo.KernelVersion,
			"kernel_arch":      hostInfo.KernelArch,
		}
	} else {
		systemInfo["host"] = map[string]interface{}{
			"error": "host info not available",
		}
	}

	if memInfo != nil {
		systemInfo["memory"] = map[string]interface{}{
			"total":        memInfo.Total,
			"available":    memInfo.Available,
			"used":         memInfo.Used,
			"used_percent": memInfo.UsedPercent,
			"free":         memInfo.Free,
			"cached":       memInfo.Cached,
			"buffers":      memInfo.Buffers,
		}
	} else {
		systemInfo["memory"] = map[string]interface{}{
			"error": "memory info not available",
		}
	}

	if currentProcess != nil {
		createTime, _ := currentProcess.CreateTime()
		systemInfo["process"] = map[string]interface{}{
			"pid":         currentProcess.Pid,
			"create_time": createTime,
		}
	} else {
		systemInfo["process"] = map[string]interface{}{
			"pid":         os.Getpid(),
			"create_time": 0,
			"error":       "process info not available",
		}
	}

	return systemInfo
}
