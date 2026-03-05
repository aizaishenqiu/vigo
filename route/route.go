package route

import (
	"net/http"
	"sync"
	"time"
	"vigo/app/controller"
	"vigo/app/controller/monitor"
	"vigo/config"
	"vigo/framework/container"
	"vigo/framework/db"
	"vigo/framework/middleware"
	"vigo/framework/mvc"
	"vigo/framework/redis"
	"vigo/framework/websocket"

	httpSwagger "github.com/swaggo/http-swagger"
)

// wsHub 全局 WebSocket Hub（由 InitWithHub 注入）
var wsHub *websocket.Hub

// 健康检查缓存（减少高并发下的 DB/Redis Ping 开销）
var (
	healthCache      map[string]interface{}
	healthCacheTime  time.Time
	healthCacheMutex sync.RWMutex
	healthCacheTTL   = 3 * time.Second
)

// InitWithHub 带 WebSocket Hub 的路由初始化
func InitWithHub(r *mvc.Router, hub *websocket.Hub) {
	wsHub = hub
	Init(r)
}

// Init 注册路由
func Init(r *mvc.Router) {
	// 全局安全中间件（兜底保护）
	// 根据配置文件自动启用，拦截 SQL 注入、XSS、命令注入等攻击
	if config.App.Security.EnableSecurityMiddleware {
		r.Use(middleware.SecurityMiddleware())
	}

	// 首页路由
	index := &controller.IndexController{}
	r.GET("/", index.Index)
	r.GET("/hello", index.Hello)

	// 视图测试页面
	home := &controller.HomeController{}
	r.GET("/home", home.Index)

	// 登录接口 (添加限流保护防暴力破解)
	auth := &controller.AuthController{}
	loginGroup := r.Group("/api", middleware.IPBasedRateLimitMiddleware(10)) // 限制每个 IP 每秒最多 10 次请求
	loginGroup.POST("/login", auth.Login)
	loginGroup.GET("/login", auth.Login)

	// 监控大屏
	mon := &monitor.MonitorController{}
	r.GET("/monitor", mon.Index)
	r.GET("/monitor/data", mon.Data)

	// Swagger 文档
	r.Handle("/docs/", httpSwagger.WrapHandler)
	r.GET("/docs", func(c *mvc.Context) {
		http.Redirect(c.Writer, c.Request, "/docs/index.html", http.StatusMovedPermanently)
	})

	// 静态资源
	r.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public/static"))))

	// 压力测试面板
	benchmarkCtrl := &controller.BenchmarkController{Hub: wsHub}
	r.GET("/benchmark", benchmarkCtrl.Index)
	r.GET("/benchmark/ws", benchmarkCtrl.WebSocket) // WebSocket 端点
	r.GET("/benchmark/stats", benchmarkCtrl.Stats)  // 保留 HTTP 降级接口
	r.POST("/benchmark/start", benchmarkCtrl.Start)
	r.POST("/benchmark/start-http", benchmarkCtrl.StartHTTP)
	r.POST("/benchmark/stop", benchmarkCtrl.Stop)
	r.POST("/benchmark/reset", benchmarkCtrl.Reset)
	r.GET("/benchmark/qps", benchmarkCtrl.QPS)

	// 性能测试中心
	performance := controller.NewPerformanceController()
	r.GET("/performance", performance.Index)
	r.GET("/performance/run", performance.Run)
	r.GET("/performance/results", performance.GetResults)
	r.POST("/performance/clear", performance.ClearResults)
	r.GET("/performance/system", performance.SystemInfo)
	r.GET("/performance/database", performance.DatabaseTest)

	// RabbitMQ 管理中心
	mqCtrl := &controller.RabbitMQController{}
	r.GET("/rabbitmq", mqCtrl.Index)
	r.GET("/rabbitmq/status", mqCtrl.Status)
	r.GET("/rabbitmq/queues", mqCtrl.Queues)
	r.POST("/rabbitmq/queue/create", mqCtrl.CreateQueue)
	r.POST("/rabbitmq/queue/delete", mqCtrl.DeleteQueue)
	r.POST("/rabbitmq/queue/purge", mqCtrl.PurgeQueue)
	r.GET("/rabbitmq/exchanges", mqCtrl.Exchanges)
	r.POST("/rabbitmq/exchange/create", mqCtrl.CreateExchange)
	r.POST("/rabbitmq/exchange/delete", mqCtrl.DeleteExchange)
	r.POST("/rabbitmq/publish", mqCtrl.Publish)

	// Nacos 服务管理中心
	nacosCtrl := &controller.NacosController{}
	r.GET("/nacos", nacosCtrl.Index)
	r.GET("/nacos/status", nacosCtrl.Status)
	r.GET("/nacos/config", nacosCtrl.GetConfig)
	r.POST("/nacos/config/publish", nacosCtrl.PublishConfig)
	r.POST("/nacos/config/delete", nacosCtrl.DeleteConfig)
	r.GET("/nacos/services", nacosCtrl.Services)
	r.GET("/nacos/instances", nacosCtrl.Instances)
	r.POST("/nacos/service/register", nacosCtrl.RegisterService)

	// 健康检查接口（供微服务探活）
	// /health - 轻量级健康检查，无 IO 操作，适合高并发压测
	// /health/full - 完整健康检查，包含数据库与 Redis 连接状态，带 3 秒缓存
	r.GET("/health", healthCheck)
	r.GET("/health/full", healthCheckFull)
}

// healthCheck 轻量级健康检查（无 IO 操作，高性能）
func healthCheck(c *mvc.Context) {
	c.Success(map[string]interface{}{
		"status":  "healthy",
		"service": "vigo",
	})
}

// healthCheckFull 完整健康检查（包含 DB/Redis 状态，带缓存）
func healthCheckFull(c *mvc.Context) {
	// 检查缓存是否有效
	healthCacheMutex.RLock()
	if healthCache != nil && time.Since(healthCacheTime) < healthCacheTTL {
		result := healthCache
		healthCacheMutex.RUnlock()
		c.Success(result)
		return
	}
	healthCacheMutex.RUnlock()

	// 执行实际检查
	checks := map[string]interface{}{
		"status":  "healthy",
		"service": "vigo",
	}

	// 检查数据库连接
	if db.GlobalDB != nil {
		if err := db.GlobalDB.Ping(); err != nil {
			checks["database"] = "down"
		} else {
			checks["database"] = "up"
		}
	} else {
		checks["database"] = "not_configured"
	}

	// 检查 Redis 连接
	if rdb := container.App().Make("redis"); rdb != nil {
		if cli, ok := rdb.(*redis.Client); ok {
			if err := cli.Ping(c.Request.Context()).Err(); err != nil {
				checks["redis"] = "down"
			} else {
				checks["redis"] = "up"
			}
		}
	} else {
		checks["redis"] = "not_configured"
	}

	// 更新缓存
	healthCacheMutex.Lock()
	healthCache = checks
	healthCacheTime = time.Now()
	healthCacheMutex.Unlock()

	c.Success(checks)
}
