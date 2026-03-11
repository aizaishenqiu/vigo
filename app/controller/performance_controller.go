package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"vigo/app/service/performance"
	"vigo/framework/mvc"
)

type PerformanceController struct {
	BaseController
	service *performance.Service
}

func NewPerformanceController() *PerformanceController {
	return &PerformanceController{
		service: performance.NewService(),
	}
}

func (c *PerformanceController) Index(ctx *mvc.Context) {
	ctx.HTML(http.StatusOK, "performance/index.html", map[string]interface{}{
		"title": "Vigo 系统性能测试中心",
	})
}

func (c *PerformanceController) Run(ctx *mvc.Context) {
	testType := ctx.Input("type")
	if testType == "" {
		testType = "all"
	}
	concurrency, _ := strconv.Atoi(ctx.Input("concurrency"))
	if concurrency <= 0 {
		concurrency = 10
	}
	iterations, _ := strconv.Atoi(ctx.Input("iterations"))
	if iterations <= 0 {
		iterations = 1000
	}
	durationStr := ctx.Input("duration")
	if durationStr == "" {
		durationStr = "10s"
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		duration = 10 * time.Second
	}

	cfg := performance.Config{
		Concurrency: concurrency,
		Iterations:  iterations,
		Duration:    duration,
	}

	var results map[string]*performance.Result

	switch testType {
	case "database_read":
		cfg.Name = "database_read"
		result, err := c.service.RunDatabaseBenchmark(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		results = map[string]*performance.Result{"database_read": result}

	case "database_write":
		cfg.Name = "database_write"
		result, err := c.service.RunDatabaseWriteBenchmark(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		results = map[string]*performance.Result{"database_write": result}

	case "redis":
		cfg.Name = "redis"
		result, err := c.service.RunRedisBenchmark(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		results = map[string]*performance.Result{"redis": result}

	case "queue":
		cfg.Name = "queue"
		result, err := c.service.RunQueueBenchmark(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		results = map[string]*performance.Result{"queue": result}

	case "qps":
		cfg.Name = "qps"
		result, err := c.service.RunQPSBenchmark(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		results = map[string]*performance.Result{"qps": result}

	case "memory":
		cfg.Name = "memory"
		result, err := c.service.RunMemoryBenchmark(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
		results = map[string]*performance.Result{"memory": result}

	default:
		var err error
		results, err = c.service.RunAllBenchmarks(cfg)
		if err != nil {
			ctx.Error(http.StatusInternalServerError, err.Error())
			return
		}
	}

	formattedResults := make(map[string]map[string]interface{})
	for name, result := range results {
		formattedResults[name] = map[string]interface{}{
			"name":           result.Name,
			"total_ops":      result.TotalOps,
			"duration":       formatDuration(result.Duration),
			"ops_per_second": result.OpsPerSecond,
			"avg_latency":    formatDuration(result.AvgLatency),
			"min_latency":    formatDuration(result.MinLatency),
			"max_latency":    formatDuration(result.MaxLatency),
			"error_count":    result.ErrorCount,
			"success_rate":   result.SuccessRate,
			"goroutines":     result.Goroutines,
			"memory_mb":      result.MemoryMB,
			"cpu_usage":      result.CPUUsage,
		}
	}

	ctx.Success(map[string]interface{}{
		"results":    formattedResults,
		"systemInfo": performance.GetSystemInfo(),
	})
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.2f μs", float64(d.Nanoseconds())/1000)
	} else if d < time.Second {
		return fmt.Sprintf("%.2f ms", float64(d.Nanoseconds())/1000000)
	}
	return fmt.Sprintf("%.2f s", d.Seconds())
}

func (c *PerformanceController) GetResults(ctx *mvc.Context) {
	results := c.service.GetAllResults()
	ctx.Success(results)
}

func (c *PerformanceController) ClearResults(ctx *mvc.Context) {
	c.service.ClearResults()
	ctx.Success("results cleared")
}

func (c *PerformanceController) SystemInfo(ctx *mvc.Context) {
	info := performance.GetSystemInfo()
	ctx.Success(info)
}

func (c *PerformanceController) DatabaseTest(ctx *mvc.Context) {
	driver := ctx.Input("driver")
	if driver == "" {
		driver = "mysql"
	}

	var driverName string
	switch driver {
	case "mysql":
		driverName = "MySQL"
	case "postgres":
		driverName = "PostgreSQL"
	case "sqlite3":
		driverName = "SQLite"
	case "mssql":
		driverName = "SQL Server"
	default:
		driverName = driver
	}

	cfg := performance.Config{
		Name:        "database_" + driver,
		Concurrency: 10,
		Iterations:  100,
	}

	result, err := c.service.RunDatabaseBenchmark(cfg)
	if err != nil {
		ctx.Error(http.StatusInternalServerError, err.Error())
		return
	}

	formattedResult := map[string]interface{}{
		"name":           result.Name,
		"total_ops":      result.TotalOps,
		"duration":       formatDuration(result.Duration),
		"ops_per_second": result.OpsPerSecond,
		"avg_latency":    formatDuration(result.AvgLatency),
		"min_latency":    formatDuration(result.MinLatency),
		"max_latency":    formatDuration(result.MaxLatency),
		"error_count":    result.ErrorCount,
		"success_rate":   result.SuccessRate,
		"goroutines":     result.Goroutines,
		"memory_mb":      result.MemoryMB,
		"cpu_usage":      result.CPUUsage,
	}

	ctx.Success(map[string]interface{}{
		"driver": driverName,
		"result": formattedResult,
	})
}
