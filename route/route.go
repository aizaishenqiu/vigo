package route

import (
	"context"
	"database/sql"
	"net/http"
	"runtime"
	"time"
	"vigo/app/controller"
	"vigo/framework/mvc"
	"vigo/framework/redis"

	httpSwagger "github.com/swaggo/http-swagger"
)

// startTime 服务启动时间（用于计算运行时长）
var startTime = time.Now()

// Init 注册路由
// 注意：所有中间件已在 framework/app/app.go 中统一注册，此处只负责业务路由
func Init(r *mvc.Router) {
	// 首页路由
	index := &controller.IndexController{}
	r.GET("/", index.Index)      // 官网首页
	r.GET("/hello", index.Hello) // 打招呼接口

	// 健康检查
	r.GET("/health", healthCheck)          // 轻量级健康检查
	r.GET("/health/full", healthCheckFull) // 完整健康检查
	r.GET("/health/page", healthPage)      // 健康检查可视化页面

	// Swagger 文档
	r.Handle("/docs/swagger/", httpSwagger.WrapHandler)
	r.GET("/swagger", func(c *mvc.Context) {
		http.Redirect(c.Writer, c.Request, "/docs/swagger/", http.StatusMovedPermanently)
	})

	// 静态资源
	r.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("public/static"))))
}

// healthCheck 轻量级健康检查
func healthCheck(c *mvc.Context) {
	c.Success(map[string]interface{}{
		"status":  "healthy",
		"service": "vigo",
	})
}

// healthCheckFull 完整健康检查（包含 DB/Redis 实际连通性测试）
func healthCheckFull(c *mvc.Context) {
	checks := map[string]interface{}{
		"status":  "healthy",
		"service": "vigo",
		"version": "2.0.12",
		"uptime":  time.Since(startTime).String(),
		"checks":  make(map[string]interface{}),
	}

	checkDetails := checks["checks"].(map[string]interface{})

	// 检查数据库连接（实际执行 ping 测试）
	dbStatus := map[string]interface{}{
		"status": "not_configured",
	}
	if db, ok := c.Get("database"); ok && db != nil {
		// 执行实际的数据库 ping 测试
		if err := db.(*sql.DB).Ping(); err != nil {
			dbStatus["status"] = "down"
			dbStatus["error"] = err.Error()
			checks["status"] = "unhealthy"
		} else {
			dbStatus["status"] = "up"
			dbStatus["message"] = "数据库连接正常"
			// 获取连接池统计
			stats := db.(*sql.DB).Stats()
			dbStatus["stats"] = map[string]interface{}{
				"max_open_connections": stats.MaxOpenConnections,
				"open_connections":     stats.OpenConnections,
				"in_use":               stats.InUse,
				"idle":                 stats.Idle,
				"wait_count":           stats.WaitCount,
				"wait_duration":        stats.WaitDuration.String(),
				"max_idle_closed":      stats.MaxIdleClosed,
				"max_lifetime_closed":  stats.MaxLifetimeClosed,
			}
		}
	}
	checkDetails["database"] = dbStatus

	// 检查 Redis 连接（实际执行 ping 测试）
	redisStatus := map[string]interface{}{
		"status": "not_configured",
	}
	if redisClient, ok := c.Get("redis"); ok && redisClient != nil {
		// 执行实际的 Redis ping 测试
		ctx := context.Background()
		if err := redisClient.(*redis.Client).Ping(ctx).Err(); err != nil {
			redisStatus["status"] = "down"
			redisStatus["error"] = err.Error()
			checks["status"] = "unhealthy"
		} else {
			redisStatus["status"] = "up"
			redisStatus["message"] = "Redis 连接正常"
		}
	}
	checkDetails["redis"] = redisStatus

	// 检查内存使用情况
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	checkDetails["memory"] = map[string]interface{}{
		"alloc_mb":       m.Alloc / 1024 / 1024,
		"total_alloc_mb": m.TotalAlloc / 1024 / 1024,
		"sys_mb":         m.Sys / 1024 / 1024,
		"num_gc":         m.NumGC,
		"goroutines":     runtime.NumGoroutine(),
	}

	c.Success(checks)
}

// healthPage 健康检查可视化页面
func healthPage(c *mvc.Context) {
	c.HTML(http.StatusOK, "health/index.html", map[string]interface{}{
		"title": "健康检查 - Vigo",
	})
}
