package admin

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
	"vigo/config"
	"vigo/framework/mvc"
)

// Config 管理后台配置
type Config struct {
	Enabled      bool
	Port         int
	BasePath     string
	Username     string
	Password     string
	AllowIPs     []string
	AutoRegister bool
}

// Manager 管理后台管理器
type Manager struct {
	config *Config
}

// GlobalManager 全局管理器实例
var GlobalManager *Manager

//go:embed views/*
var adminViews embed.FS

//go:embed static/*
var adminStatic embed.FS

// Init 初始化管理后台
func Init(conf Config, r *mvc.Router) error {
	GlobalManager = &Manager{
		config: &conf,
	}

	// 初始化各模块管理器
	if err := InitConfigManager(); err != nil {
		fmt.Printf("ConfigManager init failed: %v\n", err)
	}

	// 自动注册路由
	if conf.AutoRegister {
		RegisterRoutes(r)
	}

	return nil
}

// RegisterRoutes 注册管理后台路由
func RegisterRoutes(r *mvc.Router) {
	if GlobalManager == nil || !GlobalManager.config.Enabled {
		return
	}

	basePath := "/admin"
	if GlobalManager.config.BasePath != "" {
		basePath = GlobalManager.config.BasePath
	}

	if GlobalManager.config.Port != 0 && GlobalManager.config.Port != config.App.App.Port {
		// 如果端口不同，可能需要启动独立服务，这里暂不处理，假设复用主服务路由
	}

	// 静态资源
	r.GET(basePath+"/static/*filepath", func(c *mvc.Context) {
		filepath := c.Param("filepath")

		// 移除开头的斜杠，因为 embed.FS 中的路径不带斜杠
		filepath = strings.TrimPrefix(filepath, "/")
		fmt.Printf("[DEBUG] Reading file: %s\n", filepath)

		embedPath := "static/" + filepath

		// 从 embed.FS 读取文件内容
		content, err := adminStatic.ReadFile(embedPath)
		if err != nil {
			fmt.Printf("[DEBUG] File not found: %s, error: %v\n", embedPath, err)
			c.String(404, "File not found: %s", filepath)
			return
		}
		fmt.Printf("[DEBUG] File found: %s, size: %d bytes\n", embedPath, len(content))

		// 根据文件扩展名设置 Content-Type
		contentType := "application/octet-stream"
		if strings.HasSuffix(filepath, ".css") {
			contentType = "text/css; charset=utf-8"
		} else if strings.HasSuffix(filepath, ".js") {
			contentType = "application/javascript; charset=utf-8"
		} else if strings.HasSuffix(filepath, ".html") {
			contentType = "text/html; charset=utf-8"
		} else if strings.HasSuffix(filepath, ".png") {
			contentType = "image/png"
		} else if strings.HasSuffix(filepath, ".jpg") || strings.HasSuffix(filepath, ".jpeg") {
			contentType = "image/jpeg"
		} else if strings.HasSuffix(filepath, ".gif") {
			contentType = "image/gif"
		} else if strings.HasSuffix(filepath, ".svg") {
			contentType = "image/svg+xml"
		} else if strings.HasSuffix(filepath, ".ico") {
			contentType = "image/x-icon"
		} else if strings.HasSuffix(filepath, ".woff") || strings.HasSuffix(filepath, ".woff2") {
			contentType = "font/woff2"
		} else if strings.HasSuffix(filepath, ".ttf") {
			contentType = "font/ttf"
		} else if strings.HasSuffix(filepath, ".eot") {
			contentType = "application/vnd.ms-fontobject"
		}

		c.Writer.Header().Set("Content-Type", contentType)
		c.Writer.WriteHeader(200)
		c.Writer.Write(content)
	})

	// 应用中间件
	r.Use(AuthMiddleware())

	// 页面路由（同时注册带斜杠和不带斜杠的版本）
	r.GET(basePath+"/login", loginHandler)
	r.POST(basePath+"/api/login", loginAPI)
	r.POST(basePath+"/api/logout", logoutAPI)
	r.POST(basePath+"/api/password", changePasswordAPI)

	r.GET(basePath, indexHandler)
	r.GET(basePath+"/", indexHandler)
	r.GET(basePath+"/welcome", welcomeHandler)
	r.GET(basePath+"/monitor", systemMonitor)
	r.GET(basePath+"/settings", settingsHandler)
	r.GET(basePath+"/nacos", nacosIndex)
	r.GET(basePath+"/rabbitmq", rabbitmqIndex)
	r.GET(basePath+"/stress", stressIndex)
	r.GET(basePath+"/migration", migrationIndex) // Placeholder

	// API 路由 - 配置管理
	r.GET(basePath+"/api/settings/get", getSettings)
	r.POST(basePath+"/api/settings/app", saveAppSettings)
	r.POST(basePath+"/api/settings/databases/list", listDatabases)
	r.POST(basePath+"/api/settings/database/update", updateDatabase)
	r.POST(basePath+"/api/settings/database/add", addDatabase)
	r.POST(basePath+"/api/settings/database/remove", removeDatabase)
	r.POST(basePath+"/api/settings/redis", saveRedisSettings)
	r.POST(basePath+"/api/settings/rabbitmq", saveMQSettings)
	r.POST(basePath+"/api/settings/nacos", saveNacosSettings)
	r.POST(basePath+"/api/settings/grpc", saveGRPCSettings)
	r.POST(basePath+"/api/settings/security", saveSecuritySettings)
	r.POST(basePath+"/api/settings/payment", savePaymentSettings)
	r.POST(basePath+"/api/settings/oauth", saveOAuthSettings)
	r.POST(basePath+"/api/settings/benchmark", saveBenchmarkSettings)
	r.GET(basePath+"/api/settings/benchmark", saveBenchmarkSettings)

	// API 路由 - 系统监控
	r.GET(basePath+"/api/monitor/stats", monitorStats)

	// API 路由 - Nacos
	r.GET(basePath+"/api/nacos/status", nacosStatus)
	r.GET(basePath+"/api/nacos/config", nacosConfig)
	r.POST(basePath+"/api/nacos/config/publish", nacosConfigPublish)
	r.POST(basePath+"/api/nacos/config/delete", nacosConfigDeleteV2)
	r.GET(basePath+"/api/nacos/services", nacosServices)
	r.GET(basePath+"/api/nacos/instances", nacosInstances)
	r.POST(basePath+"/api/nacos/service/register", nacosRegisterService)

	// API 路由 - RabbitMQ
	r.GET(basePath+"/api/rabbitmq/queues", rabbitmqQueueList)
	r.POST(basePath+"/api/rabbitmq/queue", rabbitmqQueueCreate)
	r.DELETE(basePath+"/api/rabbitmq/queue", rabbitmqQueueDelete)
	r.GET(basePath+"/api/rabbitmq/exchanges", rabbitmqExchangeList)
	r.POST(basePath+"/api/rabbitmq/exchange", rabbitmqExchangeCreate)
	r.DELETE(basePath+"/api/rabbitmq/exchange", rabbitmqExchangeDelete)
	r.GET(basePath+"/api/rabbitmq/status", rabbitmqStatusStub)

	// API 路由 - 压测
	r.POST(basePath+"/api/stress/start", stressTestStartStub)
	r.POST(basePath+"/api/stress/start-http", stressTestStartHttpStub)
	r.POST(basePath+"/api/stress/stop", stressTestStopStub)
	r.POST(basePath+"/api/stress/reset", stressTestResetStub)
	r.GET(basePath+"/api/stress/stats", stressTestStatsStub)
	r.GET(basePath+"/api/stress/services", stressTestServicesStub)

	fmt.Printf("Admin panel registered at %s\n", basePath)
}

// 页面 Handlers

func loginHandler(c *mvc.Context) {
	renderView(c, "views/login.html", map[string]interface{}{"title": "登录"})
}

func indexHandler(c *mvc.Context) {
	renderView(c, "views/index.html", map[string]interface{}{"title": "Vigo 管理后台"})
}

func welcomeHandler(c *mvc.Context) {
	renderView(c, "views/welcome.html", map[string]interface{}{"title": "工作台"})
}

func systemMonitor(c *mvc.Context) {
	renderView(c, "views/monitor.html", map[string]interface{}{"title": "系统监控"})
}

func settingsHandler(c *mvc.Context) {
	renderView(c, "views/settings.html", map[string]interface{}{"title": "配置管理"})
}

func nacosIndex(c *mvc.Context) {
	renderView(c, "views/nacos.html", map[string]interface{}{"title": "Nacos 管理"})
}

func rabbitmqIndex(c *mvc.Context) {
	renderView(c, "views/rabbitmq.html", map[string]interface{}{"title": "RabbitMQ 管理"})
}

func stressIndex(c *mvc.Context) {
	renderView(c, "views/stress.html", map[string]interface{}{"title": "压测中心"})
}

func migrationIndex(c *mvc.Context) {
	c.String(200, "Migration tool coming soon")
}

// 辅助函数
func renderView(c *mvc.Context, viewPath string, data map[string]interface{}) {
	tmpl, err := template.ParseFS(adminViews, viewPath)
	if err != nil {
		c.String(500, "Template error: %v", err)
		return
	}
	if err := tmpl.Execute(c.Writer, data); err != nil {
		c.String(500, "Render error: %v", err)
	}
}

// Stubs for missing handlers
func rabbitmqStatusStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{
		"code": 0, "msg": "success",
		"data": map[string]interface{}{"connected": true, "version": "3.12.0", "erlang_version": "26.0"},
	})
}

func stressTestStartStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{"code": 0, "msg": "started (stub)"})
}
func stressTestStartHttpStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{"code": 0, "msg": "started (stub)"})
}
func stressTestStopStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{"code": 0, "msg": "stopped (stub)"})
}
func stressTestResetStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{"code": 0, "msg": "reset (stub)"})
}
func stressTestStatsStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{
		"code": 0, "msg": "success",
		"data": map[string]interface{}{"qps": 0, "latency": 0, "cpu": 10, "memUsed": 1024 * 1024 * 100, "memTotal": 1024 * 1024 * 1024 * 8},
	})
}
func stressTestServicesStub(c *mvc.Context) {
	c.Json(200, map[string]interface{}{
		"code": 0, "msg": "success",
		"data": map[string]interface{}{"mysql": true, "redis": true, "mq": true},
	})
}
