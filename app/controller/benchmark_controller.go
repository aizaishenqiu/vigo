package controller

import (
	"net/http"
	"vigo/app/service/benchmark"
	"vigo/framework/mvc"
	"vigo/framework/websocket"
	"strconv"
	"time"
)

type BenchmarkController struct {
	BaseController
	Hub *websocket.Hub // 由路由注入
}

// WebSocket WebSocket 升级端点
func (b *BenchmarkController) WebSocket(c *mvc.Context) {
	if b.Hub == nil {
		c.Error(http.StatusInternalServerError, "WebSocket Hub 未初始化")
		return
	}
	websocket.Handler(b.Hub, c.Writer, c.Request)
}

// Index 展示压测主页
func (b *BenchmarkController) Index(c *mvc.Context) {
	c.HTML(http.StatusOK, "benchmark/index.html", map[string]interface{}{
		"title": "Vigo 性能压测监控大屏",
	})
}

// Start 启动压测（中间件/数据库/缓存模式）
func (b *BenchmarkController) Start(c *mvc.Context) {
	concurrency, _ := strconv.Atoi(c.Input("concurrency"))
	duration, _ := strconv.Atoi(c.Input("duration"))
	dbCount, _ := strconv.ParseInt(c.Input("db_count"), 10, 64)
	mqCount, _ := strconv.ParseInt(c.Input("mq_count"), 10, 64)
	enableInsert, _ := strconv.ParseBool(c.Input("enable_insert"))
	memoryLoadMB, _ := strconv.Atoi(c.Input("memory_load_mb"))
	if memoryLoadMB == 0 {
		memoryLoadMB, _ = strconv.Atoi(c.Input("memory_load"))
	}

	enableRedis, _ := strconv.ParseBool(c.Input("enable_redis"))
	enableDB, _ := strconv.ParseBool(c.Input("enable_db"))
	enableMQ, _ := strconv.ParseBool(c.Input("enable_mq"))

	if concurrency <= 0 {
		concurrency = 10
	}

	svc := benchmark.GetService()
	// 自动停止旧测试（支持刷新页面后重新启动）
	if svc.IsRunning() {
		svc.StopTest()
		time.Sleep(200 * time.Millisecond)
	}
	if err := svc.StartTest(concurrency, duration, dbCount, mqCount, enableInsert, memoryLoadMB, enableRedis, enableDB, enableMQ); err != nil {
		c.Error(http.StatusBadRequest, err.Error())
		return
	}
	c.Success("压测已启动")
}

// StartHTTP 启动 HTTP 压测
func (b *BenchmarkController) StartHTTP(c *mvc.Context) {
	concurrency, _ := strconv.Atoi(c.Input("concurrency"))
	duration, _ := strconv.Atoi(c.Input("duration"))
	targetURL := c.Input("target_url")
	method := c.Input("method")

	if concurrency <= 0 {
		concurrency = 10
	}
	if targetURL == "" {
		c.Error(http.StatusBadRequest, "target_url 不能为空")
		return
	}
	if method == "" {
		method = "GET"
	}

	svc := benchmark.GetService()
	if svc.IsRunning() {
		svc.StopTest()
		time.Sleep(200 * time.Millisecond)
	}
	if err := svc.StartHTTPTest(concurrency, duration, targetURL, method); err != nil {
		c.Error(http.StatusBadRequest, err.Error())
		return
	}
	c.Success("HTTP 压测已启动")
}

// Stop 停止压测
func (b *BenchmarkController) Stop(c *mvc.Context) {
	svc := benchmark.GetService()
	svc.StopTest()
	c.Success("压测已停止")
}

// Reset 清除数据
func (b *BenchmarkController) Reset(c *mvc.Context) {
	svc := benchmark.GetService()
	svc.Reset()
	c.Success("数据已清除")
}

// Stats 获取实时监控数据
func (b *BenchmarkController) Stats(c *mvc.Context) {
	svc := benchmark.GetService()
	stats := svc.GetStats()
	c.Success(stats)
}

// QPS 兼容旧接口
func (b *BenchmarkController) QPS(c *mvc.Context) {
	b.Stats(c)
}
