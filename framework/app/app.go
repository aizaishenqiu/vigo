// Package app 提供应用核心框架和生命周期管理
// 包含服务初始化、启动、停止和优雅关闭等功能
package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"vigo/bootstrap"
	"vigo/config"
	"vigo/framework/admin"
	"vigo/framework/container"
	"vigo/framework/db"
	"vigo/framework/debug"
	frameworkGrpc "vigo/framework/grpc"
	"vigo/framework/log"
	"vigo/framework/logger"
	"vigo/framework/middleware"
	"vigo/framework/mvc"
	"vigo/framework/nacos"
	"vigo/framework/port"
	"vigo/framework/rabbitmq"
	"vigo/framework/redis"
	"vigo/framework/websocket"
	"vigo/route"
)

// ==================== 生命周期钩子类型 ====================

// HookFunc 生命周期钩子函数类型
type HookFunc func(*App) error

// ==================== App 核心 ====================

// App 应用核心结构体
// 管理应用的生命周期、服务容器和所有核心组件
type App struct {
	*container.Container                       // 服务容器
	BaseDir              string                // 应用根目录
	Version              string                // 应用版本号
	Debug                bool                  // 是否开启调试模式
	Mode                 string                // 运行模式: dev | test | prod
	providers            []ServiceProvider     // 服务提供者列表
	grpcServer           *frameworkGrpc.Server // gRPC 服务器
	httpServer           *http.Server          // HTTP 服务器
	wsHub                *websocket.Hub        // WebSocket Hub

	// 生命周期钩子
	onStarting []HookFunc // 启动前钩子
	onStarted  []HookFunc // 启动后钩子
	onStopping []HookFunc // 停止前钩子
	onStopped  []HookFunc // 停止后钩子
}

// ServiceProvider 服务提供者接口
// 用于扩展和注册应用服务
type ServiceProvider interface {
	Name() string  // 提供者名称
	Register(*App) // 注册阶段（绑定服务到容器）
	Boot(*App)     // 启动阶段（初始化服务逻辑）
}

// New 创建应用实例
// 注意：此方法不读取配置，配置仅在 Initialize() 中加载后生效
func New(baseDir string) *App {
	resolvedDir := resolveBaseDir(baseDir)

	app := &App{
		Container: container.App(),
		BaseDir:   resolvedDir,
		Version:   "",
		Debug:     false,
		Mode:      "",
		providers: make([]ServiceProvider, 0),
	}
	return app
}

// ==================== 生命周期钩子注册 ====================

// OnStarting 注册服务启动前钩子
// 在应用初始化完成后、HTTP 服务启动前执行
func (app *App) OnStarting(fn HookFunc) {
	app.onStarting = append(app.onStarting, fn)
}

// OnStarted 注册服务启动后钩子
// 在 HTTP 服务成功启动后执行
func (app *App) OnStarted(fn HookFunc) {
	app.onStarted = append(app.onStarted, fn)
}

// OnStopping 注册服务停止前钩子
// 在接收到停止信号后、开始关闭服务前执行
func (app *App) OnStopping(fn HookFunc) {
	app.onStopping = append(app.onStopping, fn)
}

// OnStopped 注册服务停止后钩子
// 在所有服务关闭完成后执行
func (app *App) OnStopped(fn HookFunc) {
	app.onStopped = append(app.onStopped, fn)
}

// runHooks 执行一组钩子函数
func (app *App) runHooks(hooks []HookFunc) {
	for _, fn := range hooks {
		if err := fn(app); err != nil {
			fmt.Printf("[Vigo] 钩子执行错误: %v\n", err)
		}
	}
}

// ==================== 目录解析 ====================

// resolveBaseDir 解析应用根目录
// 尝试从当前目录、可执行文件目录或父目录查找配置文件
func resolveBaseDir(baseDir string) string {
	if baseDir != "." {
		abs, err := filepath.Abs(baseDir)
		if err == nil {
			return abs
		}
		return baseDir
	}

	// 优先尝试当前工作目录
	cwd, _ := os.Getwd()
	if fileExists(filepath.Join(cwd, "config.yaml")) {
		return cwd
	}

	// 尝试可执行文件所在目录
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		if fileExists(filepath.Join(exeDir, "config.yaml")) {
			os.Chdir(exeDir)
			return exeDir
		}
		// 尝试父目录
		parentDir := filepath.Dir(exeDir)
		if fileExists(filepath.Join(parentDir, "config.yaml")) {
			os.Chdir(parentDir)
			return parentDir
		}
	}
	return cwd
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ==================== 服务提供者 ====================

// RegisterProvider 注册服务提供者
// 会立即调用提供者的 Register 方法
func (app *App) RegisterProvider(provider ServiceProvider) {
	provider.Register(app)
	app.providers = append(app.providers, provider)
}

// BootProviders 启动所有服务提供者
// 按注册顺序调用提供者的 Boot 方法
func (app *App) BootProviders() {
	for _, provider := range app.providers {
		provider.Boot(app)
	}
}

// ==================== 管理面板 ====================

// initAdminPanel 初始化管理面板（类似 Swagger 的自动注册）
func (app *App) initAdminPanel(r *mvc.Router) {
	// 从配置文件读取管理面板配置
	adminCfg := admin.Config{
		Enabled:      config.App.Admin.Enabled,
		BasePath:     config.App.Admin.BasePath,
		Username:     config.App.Admin.Username,
		Password:     config.App.Admin.Password,
		AllowIPs:     config.App.Admin.AllowIPs,
		AutoRegister: config.App.Admin.AutoRegister,
	}

	// 如果配置文件中未指定，使用默认配置
	if !adminCfg.Enabled && adminCfg.BasePath == "" {
		adminCfg.Enabled = true
		adminCfg.BasePath = "/admin"
		adminCfg.AutoRegister = true
	}

	// 先初始化 WebSocket 管理器（在 Init 之前）
	if adminCfg.Enabled {
		admin.InitWSManager()
		log.Log.Info("[Middleware] 管理面板 WebSocket 已启用")
	}

	// 初始化管理面板
	if adminCfg.Enabled {
		admin.Init(adminCfg, r)
		fmt.Printf("[Vigo] 管理面板已启用：%s\n", adminCfg.BasePath)
	}
}

// ==================== 初始化 ====================

// Initialize 初始化核心服务
// 包括：配置、日志、数据库、Redis、RabbitMQ、Nacos、gRPC、WebSocket 等
func (app *App) Initialize() {
	start := time.Now()

	// 确保工作目录正确
	if app.BaseDir != "" && app.BaseDir != "." {
		if err := os.Chdir(app.BaseDir); err != nil {
			fmt.Printf("[Vigo] 警告: 无法切换到项目目录 %s: %v\n", app.BaseDir, err)
		} else {
			fmt.Printf("[Vigo] 工作目录: %s\n", app.BaseDir)
		}
	}

	// 初始化配置
	config.Init()
	app.Version = config.App.App.Version
	app.Debug = config.App.App.Debug
	app.Mode = config.App.App.Mode

	// 初始化日志
	log.Init("runtime/log")

	// 初始化结构化日志和指标收集器
	logger.InitGlobalLogger(logger.INFO)
	logger.InitGlobalMetricsCollector()

	// 初始化 JWT
	middleware.InitJWT()

	// 初始化会话管理器
	middleware.InitSessionManager()

	// 注册全局中间件（根据配置文件）
	app.initMiddlewares()

	// 绑定核心服务到容器
	app.Singleton("app", app)
	app.Singleton("config", &config.App)

	// 端口检测
	app.checkPorts()

	// 初始化数据库（主库）
	dsn := buildDSN(config.App.Database.Driver, config.App.Database)
	if err := db.Init(config.App.Database.Driver, dsn,
		config.App.Database.MaxOpenConns,
		config.App.Database.MaxIdleConns,
		config.App.Database.ConnMaxLifetime,
		config.App.Database.ConnMaxIdleTime); err != nil {
		log.Log.Error(fmt.Sprintf("数据库连接失败: %v", err))
	} else {
		log.Log.Info(fmt.Sprintf("数据库连接成功 (%s)", config.App.Database.Driver))
	}

	// 初始化多写库（多主库写入负载均衡）
	if len(config.App.Database.Writes) > 0 {
		var writeDSNs []string
		for _, node := range config.App.Database.Writes {
			writeCfg := config.App.Database
			writeCfg.Host = node.Host
			writeCfg.Port = node.Port
			writeCfg.User = node.User
			writeCfg.Pass = node.Pass
			if node.Charset != "" {
				writeCfg.Charset = node.Charset
			}
			writeDSNs = append(writeDSNs, buildDSN(config.App.Database.Driver, writeCfg))
		}
		if err := db.InitWriteDBs(config.App.Database.Driver, writeDSNs,
			config.App.Database.MaxOpenConns,
			config.App.Database.MaxIdleConns,
			config.App.Database.ConnMaxLifetime,
			config.App.Database.ConnMaxIdleTime); err != nil {
			log.Log.Error(fmt.Sprintf("写数据库初始化失败: %v", err))
		}
	}

	// 初始化读写分离（从库）
	if config.App.Database.RWSplit && len(config.App.Database.Reads) > 0 {
		var readDSNs []string
		for _, node := range config.App.Database.Reads {
			readCfg := config.App.Database
			readCfg.Host = node.Host
			readCfg.Port = node.Port
			readCfg.User = node.User
			readCfg.Pass = node.Pass
			if node.Charset != "" {
				readCfg.Charset = node.Charset
			}
			readDSNs = append(readDSNs, buildDSN(config.App.Database.Driver, readCfg))
		}
		if err := db.InitReadDBs(config.App.Database.Driver, readDSNs,
			config.App.Database.MaxOpenConns,
			config.App.Database.MaxIdleConns,
			config.App.Database.ConnMaxLifetime,
			config.App.Database.ConnMaxIdleTime); err != nil {
			log.Log.Error(fmt.Sprintf("读数据库初始化失败: %v", err))
		}
	}

	// 初始化多数据库连接（不同业务库）
	for name, dbCfg := range config.App.Databases {
		driver := dbCfg.Driver
		if driver == "" {
			driver = "mysql"
		}
		charset := dbCfg.Charset
		if charset == "" {
			charset = "utf8mb4"
		}
		writeDSN := db.BuildDSN(driver, dbCfg.User, dbCfg.Pass, dbCfg.Host, dbCfg.Port, dbCfg.Name, charset)

		var readDSNs []string
		if dbCfg.RWSplit && len(dbCfg.Reads) > 0 {
			for _, node := range dbCfg.Reads {
				nodeCharset := node.Charset
				if nodeCharset == "" {
					nodeCharset = charset
				}
				readDSNs = append(readDSNs, db.BuildDSN(driver, node.User, node.Pass, node.Host, node.Port, dbCfg.Name, nodeCharset))
			}
		}

		maxOpen := dbCfg.MaxOpenConns
		if maxOpen <= 0 {
			maxOpen = 100
		}
		maxIdle := dbCfg.MaxIdleConns
		if maxIdle <= 0 {
			maxIdle = 10
		}
		maxLifeTime := dbCfg.ConnMaxLifetime
		if maxLifeTime <= 0 {
			maxLifeTime = 3600
		}
		maxIdleTime := dbCfg.ConnMaxIdleTime
		if maxIdleTime <= 0 {
			maxIdleTime = 300
		}

		if err := db.RegisterConnection(name, driver, writeDSN, readDSNs, maxOpen, maxIdle, maxLifeTime, maxIdleTime); err != nil {
			log.Log.Error(fmt.Sprintf("多数据库 '%s' 初始化失败: %v", name, err))
		}
	}

	// 初始化 Nacos 配置中心
	nacosClient := nacos.NewClient(nacos.Config{
		IpAddr:      config.App.Nacos.IpAddr,
		Port:        config.App.Nacos.Port,
		NamespaceId: config.App.Nacos.NamespaceId,
		DataId:      config.App.Nacos.DataId,
		Group:       config.App.Nacos.Group,
	})

	// 设置到 admin 包
	admin.SetNacosClient(nacosClient)

	// Nacos 服务发现和注册
	if config.App.Nacos.Discovery.Enabled {
		if nacosClient.CheckHealth() {
			log.Log.Info("Nacos 服务连接成功")
			if config.App.Nacos.Discovery.AutoRegister {
				serviceName := config.App.Nacos.Discovery.ServiceName
				if serviceName == "" {
					serviceName = config.App.App.Name
				}
				if err := nacosClient.AutoRegister(serviceName, uint64(config.App.App.Port)); err != nil {
					log.Log.Warn(fmt.Sprintf("Nacos 服务注册失败：%v", err))
				}
			}
		} else {
			log.Log.Warn("Nacos 服务不可达，跳过服务注册")
		}
	} else {
		// 即使未启用服务发现，也要检查 Nacos 连接（用于管理界面）
		if nacosClient.CheckHealth() {
			log.Log.Info("Nacos 配置中心连接成功")
		} else {
			log.Log.Warn("Nacos 配置中心不可达")
		}
	}
	app.Singleton("nacos", nacosClient)

	// 初始化 RabbitMQ 消息队列
	mqClient := rabbitmq.New(rabbitmq.Config{
		Host:           config.App.RabbitMQ.Host,
		Port:           config.App.RabbitMQ.Port,
		User:           config.App.RabbitMQ.User,
		Password:       config.App.RabbitMQ.Password,
		Vhost:          config.App.RabbitMQ.Vhost,
		ConnTimeout:    config.App.RabbitMQ.ConnTimeout,
		Heartbeat:      config.App.RabbitMQ.Heartbeat,
		ReconnectDelay: config.App.RabbitMQ.ReconnectDelay,
		MaxRetries:     config.App.RabbitMQ.MaxRetries,
	})

	// 设置到 admin 包
	admin.SetRabbitMQClient(mqClient)

	if config.App.RabbitMQ.Enabled {
		if err := mqClient.Connect(); err != nil {
			log.Log.Warn(fmt.Sprintf("RabbitMQ 连接失败：%v", err))
		} else {
			log.Log.Info("RabbitMQ 连接成功")
			// 获取 Channel 并设置到 admin 包
			channel, err := mqClient.GetChannel()
			if err != nil {
				log.Log.Warn(fmt.Sprintf("RabbitMQ Channel 获取失败：%v", err))
			} else {
				admin.SetRabbitMQChannel(channel)
				log.Log.Info("RabbitMQ Channel 已设置到管理面板")
			}
		}
	} else {
		log.Log.Info("RabbitMQ 未启用")
	}
	app.Singleton("rabbitmq", mqClient)

	// 初始化 Redis 缓存（支持单实例和集群）
	clusterAddrs := make([]string, len(config.App.Redis.Cluster.Addrs))
	copy(clusterAddrs, config.App.Redis.Cluster.Addrs)

	redisClient := redis.New(redis.Config{
		Host:         config.App.Redis.Host,
		Port:         config.App.Redis.Port,
		Password:     config.App.Redis.Password,
		DB:           config.App.Redis.DB,
		PoolSize:     config.App.Redis.PoolSize,
		MinIdleConns: config.App.Redis.MinIdleConns,
		MaxIdleConns: config.App.Redis.MaxIdleConns,
		Cluster: redis.ClusterConfig{
			Enabled: config.App.Redis.Cluster.Enabled,
			Addrs:   clusterAddrs,
		},
	})
	if err := redisClient.Connect(); err != nil {
		log.Log.Warn(fmt.Sprintf("Redis 连接失败: %v", err))
	} else {
		log.Log.Info("Redis 连接成功")
	}
	app.Singleton("redis", redisClient)

	// 初始化 gRPC 服务（可选）
	if config.App.GRPC.Enabled {
		grpcSrv := frameworkGrpc.NewServer(frameworkGrpc.ServerConfig{
			Port:           config.App.GRPC.Port,
			ServiceName:    config.App.GRPC.ServiceName,
			EnableRecovery: config.App.GRPC.EnableRecovery,
			EnableLogger:   config.App.GRPC.EnableLogger,
			MaxRecvMsgSize: config.App.GRPC.MaxRecvMsgSize,
			MaxSendMsgSize: config.App.GRPC.MaxSendMsgSize,
		})
		app.grpcServer = grpcSrv
		app.Singleton("grpc", grpcSrv)
		log.Log.Info(fmt.Sprintf("gRPC 服务已创建 (端口: %d)", config.App.GRPC.Port))
	}

	// 初始化 WebSocket Hub
	app.wsHub = websocket.NewHub()
	go app.wsHub.Run()
	app.Singleton("websocket", app.wsHub)

	// 调试模式输出
	if app.Debug {
		fmt.Println("[Vigo] 框架初始化完成，耗时:", time.Since(start))
		fmt.Println("[Vigo] 调试模式：开启")
		modules := "config, log, container, mvc, db, cache, event, nacos"
		if config.App.RabbitMQ.Enabled {
			modules += ", rabbitmq"
		}
		modules += ", redis, websocket"
		if config.App.GRPC.Enabled {
			modules += ", grpc"
		}
		fmt.Printf("[Vigo] 已加载模块：%s\n", modules)
	}
}

// initMiddlewares 初始化并注册全局中间件
// 所有中间件统一在此注册，避免在 route.go 中重复注册
func (app *App) initMiddlewares() {
	middlewareCount := 0

	// 1. 调试工具栏中间件（必须第一个注册，用于捕获所有请求）
	isDev := config.App.App.Mode == "dev" || config.App.App.Debug
	if isDev && config.App.App.DebugToolbar {
		debugToolbar := debug.NewDebugToolbar()
		middleware.Use(debugToolbar.Middleware())
		log.Log.Info("[Middleware] 调试工具栏已启用")
		middlewareCount++
	}

	// 2. 环境识别中间件（开发/生产环境自动适配）
	middleware.Use(middleware.EnvironmentMiddleware())
	log.Log.Info("[Middleware] 环境识别中间件已启用")
	middlewareCount++

	// 3. 安全中间件（根据配置自动启用）
	if config.App.Security.EnableSecurityMiddleware {
		middleware.Use(middleware.SecurityMiddleware())
		log.Log.Info("[Middleware] 安全中间件已启用")
		middlewareCount++
	}

	// 4. 错误恢复中间件（始终启用）
	middleware.Use(middleware.RecoveryMiddleware())
	log.Log.Info("[Middleware] 错误恢复中间件已启用")
	middlewareCount++

	// 5. 速率限制中间件（根据配置启用）
	if config.App.Security.EnableRateLimit && config.App.Security.RateLimit > 0 {
		middleware.Use(middleware.RateLimitMiddleware(config.App.Security.RateLimit))
		log.Log.Info(fmt.Sprintf("[Middleware] 速率限制中间件已启用 (%d req/s)", config.App.Security.RateLimit))
		middlewareCount++
	}

	// 6. CSRF 保护中间件（根据配置启用）
	if config.App.Security.EnableCSRFProtection {
		middleware.Use(middleware.CSRF())
		log.Log.Info("[Middleware] CSRF 保护中间件已启用")
		middlewareCount++
	}

	log.Log.Info(fmt.Sprintf("[Middleware] 共注册 %d 个全局中间件", middlewareCount))
}

// checkPorts 检测并释放配置中的端口
// 如果 auto_kill_port 为 true，会自动杀掉占用端口的进程
func (app *App) checkPorts() {
	ports := config.GetAllPorts()
	if len(ports) == 0 {
		return
	}
	autoKill := config.App.App.AutoKillPort
	if err := port.CheckAll(ports, autoKill); err != nil {
		log.Log.Error(fmt.Sprintf("端口检测失败: %v", err))
		fmt.Printf("[Vigo] 端口检测失败: %v\n", err)
		fmt.Println("请检查端口占用情况，或设置 auto_kill_port: true 自动释放端口")
	}
}

// GetGRPCServer 获取 gRPC 服务器实例
func (app *App) GetGRPCServer() *frameworkGrpc.Server {
	return app.grpcServer
}

// ==================== 启动服务 ====================

// Run 启动 HTTP 服务
// 参数 addr: 监听地址，如 ":8080"
func (app *App) Run(addr string) error {
	// 执行启动前钩子
	app.runHooks(app.onStarting)

	// 创建路由
	r := mvc.NewRouter()

	// 注册全局中间件（按顺序执行）
	// 1. 调试工具栏（仅开发环境）
	isDev := config.App.App.Mode == "dev" || config.App.App.Debug
	if isDev && config.App.App.DebugToolbar {
		debugToolbar := debug.NewDebugToolbar()
		r.Use(debugToolbar.Middleware())
	}

	// 2. 错误恢复中间件（必须靠前，捕获所有 panic）
	r.Use(middleware.RecoveryMiddleware())

	// 3. 日志中间件（记录所有请求）
	r.Use(middleware.Logger())

	// 4. 安全响应头
	r.Use(middleware.SecurityHeaders())

	// 5. XSS 防护
	r.Use(middleware.XSS())

	// 6. SQL 注入防护（已废弃，使用 SecurityMiddleware 代替）
	// r.Use(middleware.SQLInjection())

	// 7. CSRF 防护
	r.Use(middleware.CSRF())

	// 8. 请求大小限制（10MB）
	r.Use(middleware.RequestSizeLimit(10 << 20))

	// 9. 应用错误处理中间件
	r.Use(middleware.AppErrorMiddleware())

	// 10. CORS 跨域中间件
	r.Use(middleware.CORS())

	// 11. DoS 防护 / 限流（根据配置启用）
	if config.App.Security.DoS.Enable {
		if len(config.App.Security.DoS.BlackIP) > 0 {
			middleware.SetBlackIPs(config.App.Security.DoS.BlackIP)
		}
		limit := config.App.Security.DoS.Limit
		if limit <= 0 {
			limit = 100
		}
		window := time.Duration(config.App.Security.DoS.Window) * time.Second
		if window <= 0 {
			window = time.Minute
		}
		r.Use(middleware.RateLimit(limit, window))
	}

	// 12. 注册业务路由
	route.Init(r)

	// 13. 自动注册管理面板路由（类似 Swagger）
	app.initAdminPanel(r)

	// 启动 gRPC 服务（异步）
	if app.grpcServer != nil {
		go func() {
			if err := app.grpcServer.Start(); err != nil {
				log.Log.Error(fmt.Sprintf("gRPC 服务启动失败: %v", err))
			}
		}()
	}

	// 调试模式输出
	if app.Debug {
		fmt.Printf("[Vigo] HTTP 服务监听：%s\n", addr)
		if app.grpcServer != nil {
			fmt.Printf("[Vigo] gRPC 服务监听::%d\n", config.App.GRPC.Port)
		}
	}

	// 创建 HTTP 服务器
	app.httpServer = &http.Server{
		Addr:           addr,
		Handler:        r,
		ReadTimeout:    30 * time.Second,  // 读取超时
		WriteTimeout:   60 * time.Second,  // 写入超时
		IdleTimeout:    120 * time.Second, // 空闲超时
		MaxHeaderBytes: 1 << 20,           // 最大请求头大小（1MB）
	}

	// 启动优雅关闭监听
	go app.gracefulShutdown()

	// 执行启动后钩子
	app.runHooks(app.onStarted)

	// 启动 HTTP 服务
	return app.httpServer.ListenAndServe()
}

// gracefulShutdown 优雅关闭所有服务
// 监听 SIGINT 和 SIGTERM 信号，按顺序关闭所有服务
func (app *App) gracefulShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n[Vigo] 正在优雅关闭服务...")

	// 执行停止前钩子
	app.runHooks(app.onStopping)

	// 关闭 gRPC 服务
	if app.grpcServer != nil {
		app.grpcServer.GracefulStop()
	}

	// 关闭 HTTP 服务
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if app.httpServer != nil {
		if err := app.httpServer.Shutdown(ctx); err != nil {
			fmt.Printf("[Vigo] HTTP 服务关闭出错: %v\n", err)
		}
	}

	// 关闭数据库连接池（避免资源泄漏）
	db.CloseAll()

	// 关闭 RabbitMQ
	if mqClient, ok := app.Make("rabbitmq").(*rabbitmq.Client); ok && mqClient != nil {
		mqClient.Close()
	}

	// 关闭 Redis
	if redisClient, ok := app.Make("redis").(*redis.Client); ok && redisClient != nil {
		redisClient.Close()
	}

	// 关闭 Nacos
	if nacosClient, ok := app.Make("nacos").(*nacos.Client); ok && nacosClient != nil {
		nacosClient.Close()
	}

	// 关闭日志缓冲
	if fl, ok := log.Log.(*log.FileLogger); ok {
		fl.Close()
	}

	// 执行停止后钩子
	app.runHooks(app.onStopped)

	fmt.Println("[Vigo] 所有服务已关闭")
}

// buildDSN 根据数据库驱动构建数据源连接字符串
// 支持 MySQL、PostgreSQL、SQLite、SQL Server
func buildDSN(driver string, cfg config.DatabaseConfig) string {
	switch driver {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
			cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.Name, cfg.Charset)
	case "postgres":
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
			cfg.Host, cfg.Port, cfg.User, cfg.Pass, cfg.Name)
	case "sqlite3":
		return cfg.Name
	case "sqlserver":
		return fmt.Sprintf("sqlserver://%s:%s@%s:%d?database=%s",
			cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.Name)
	default:
		return ""
	}
}

// importAndRegisterMigrations 注册所有数据库迁移
// 此函数导入 migrations 包并注册所有迁移函数
func importAndRegisterMigrations(migrator *db.Migrator) {
	log.Log.Info("[Migration] 开始注册数据库迁移...")

	// 调用 bootstrap 包中的注册函数
	bootstrap.RegisterMigrations(migrator)

	log.Log.Info("[Migration] 数据库迁移注册完成")
}
