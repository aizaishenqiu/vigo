// Package config 提供应用配置管理功能
// 支持 YAML 配置文件、环境变量和多环境配置
// 配置模块文件说明：
//   - config.go: 主配置文件（加载逻辑和路径常量）
//   - database.go: 数据库配置
//   - redis.go: Redis 配置
//   - nacos.go: Nacos 配置
//   - rabbitmq.go: RabbitMQ 配置
//   - grpc.go: gRPC 配置
//   - security.go: 安全配置
//   - benchmark.go: 压力测试配置
//   - view.go: 视图配置
package config

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"gopkg.in/yaml.v3"
)

// PathConstants 路径常量（ThinkPHP 风格）
var (
	ROOT_PATH   string // 项目根目录
	APP_PATH    string // 应用目录
	VIEW_PATH   string // 视图目录
	CONFIG_PATH string // 配置目录
	LIB_PATH    string // 类库目录
	RUN_PATH    string // 缓存目录
)

// AppConfig 应用配置总结构体
// 包含应用基础配置、数据库、缓存、消息队列等所有组件的配置
type AppConfig struct {
	App       BaseConfig          `yaml:"app"`       // 应用基础配置
	Database  DatabaseConfig      `yaml:"database"`  // 默认数据库配置（主库）
	Databases map[string]DBConfig `yaml:"databases"` // 多数据库配置（支持多业务库）
	Nacos     NacosConfig         `yaml:"nacos"`     // Nacos 配置中心配置
	RabbitMQ  RabbitMQConfig      `yaml:"rabbitmq"`  // RabbitMQ 消息队列配置
	Redis     RedisConfig         `yaml:"redis"`     // Redis 缓存配置
	GRPC      GRPCConfig          `yaml:"grpc"`      // gRPC 微服务配置
	Security  SecurityConfig      `yaml:"security"`  // 安全配置
	Payment   PaymentConfig       `yaml:"payment"`   // 支付配置
	OAuth     OAuthConfig         `yaml:"oauth"`     // 第三方登录配置
	Benchmark BenchmarkConfig     `yaml:"benchmark"` // 压力测试配置
	View      ViewConfig          `yaml:"view"`      // 视图配置
}

// BaseConfig 应用基础配置
type BaseConfig struct {
	Name         string `yaml:"name"`           // 应用名称
	Version      string `yaml:"version"`        // 应用版本号
	Port         int    `yaml:"port"`           // HTTP 服务监听端口
	Debug        bool   `yaml:"debug"`          // 是否开启调试模式
	Mode         string `yaml:"mode"`           // 运行模式：dev | test | prod
	ShowConsole  bool   `yaml:"console"`        // 是否显示控制台输出
	AutoKillPort bool   `yaml:"auto_kill_port"` // 启动时是否自动杀掉占用端口的进程
}

// AdminConfig 管理界面通用配置
type AdminConfig struct {
	Enabled  bool   `yaml:"enabled"`  // 是否启用管理界面
	Username string `yaml:"username"` // 管理界面用户名（不设置则留空，需通过环境变量或手动配置）
	Password string `yaml:"password"` // 管理界面密码（不设置则留空，需通过环境变量或手动配置）
	Port     int    `yaml:"port"`     // 管理界面端口
}

// App 全局配置实例
var App AppConfig

// envVarRegex 环境变量占位符正则表达式
// 匹配格式：${ENV_VAR} 或 ${ENV_VAR:default_value}
var envVarRegex = regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

// expandEnvVars 替换配置内容中的环境变量占位符
// 支持两种格式:
//   - ${VAR_NAME} - 仅使用环境变量，未设置则保持原样
//   - ${VAR_NAME:default} - 使用环境变量，未设置则使用默认值
func expandEnvVars(content []byte) []byte {
	return envVarRegex.ReplaceAllFunc(content, func(match []byte) []byte {
		parts := envVarRegex.FindSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		envName := string(parts[1])
		envVal := os.Getenv(envName)
		if envVal != "" {
			return []byte(envVal)
		}
		// 如果提供了默认值，则使用默认值
		if len(parts) >= 3 && len(parts[2]) > 0 {
			return parts[2]
		}
		return match // 环境变量未设置且无默认值，保持原样
	})
}

// Init 初始化配置
// 加载顺序：config.yaml → config.{env}.yaml → config.local.yaml → 环境变量
// 初始化路径常量（ThinkPHP 风格）
func Init() {
	// 1. 初始化路径常量
	initPathConstants()

	// 2. 加载主配置文件
	loadConfig("config.yaml")

	// 2. 加载环境特定配置文件
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}

	envFile := "config." + env + ".yaml"
	if _, err := os.Stat(envFile); err == nil {
		log.Printf("加载环境配置：%s\n", envFile)
		loadConfig(envFile)
	}

	// 3. 加载本地覆盖配置（不提交到版本控制）
	if _, err := os.Stat("config.local.yaml"); err == nil {
		log.Println("加载本地覆盖配置：config.local.yaml")
		loadConfig("config.local.yaml")
	}

	// 4. 从环境变量覆盖敏感配置（优先级最高）
	overrideFromEnv()

	// 5. 应用默认值
	applyDefaults()

	// 6. 校验配置
	validateConfig()

	// 7. 使用配置验证器进行详细验证
	if err := ValidateConfig(&App); err != nil {
		log.Fatalf("[Config] 配置验证失败：%v", err)
	}
}

// initPathConstants 初始化路径常量（ThinkPHP 风格）
// 设置 ROOT_PATH、APP_PATH、VIEW_PATH 等全局路径常量
func initPathConstants() {
	// 获取可执行文件所在目录作为根目录
	execPath, err := os.Executable()
	if err != nil {
		// 如果获取失败，使用当前工作目录
		execPath = "."
	}

	// 获取项目根目录（可执行文件所在目录的上一级）
	ROOT_PATH = filepath.Dir(execPath)

	// 设置其他路径常量
	APP_PATH = filepath.Join(ROOT_PATH, "app")
	VIEW_PATH = filepath.Join(APP_PATH, "view")
	CONFIG_PATH = ROOT_PATH
	LIB_PATH = filepath.Join(ROOT_PATH, "framework")
	RUN_PATH = filepath.Join(ROOT_PATH, "runtime")

	// 确保关键目录存在
	os.MkdirAll(VIEW_PATH, 0755)
	os.MkdirAll(RUN_PATH, 0755)
}

// validateConfig 校验必填配置项
// 缺失时记录警告日志或终止程序
func validateConfig() {
	if App.App.Name == "" {
		log.Printf("[Config] 警告：app.name 未配置，将使用默认应用名")
		App.App.Name = "vigo"
	}
	if App.Database.Driver == "" {
		log.Fatalf("[Config] 错误：database.driver 未配置")
	}
	if App.Database.Host == "" {
		log.Fatalf("[Config] 错误：database.host 未配置，请设置环境变量 DB_HOST")
	}
	if App.Database.User == "" {
		log.Fatalf("[Config] 错误：database.user 未配置，请设置环境变量 DB_USER")
	}
	if App.Database.Pass == "" {
		log.Fatalf("[Config] 错误：database.pass 未配置，请设置环境变量 DB_PASSWORD")
	}
	if App.Database.Name == "" {
		log.Fatalf("[Config] 错误：database.name 未配置，请设置环境变量 DB_NAME")
	}
	if App.Redis.Host == "" {
		log.Fatalf("[Config] 错误：redis.host 未配置，请设置环境变量 REDIS_HOST")
	}
	if App.Security.JWT.Secret == "" {
		log.Fatalf("[Config] 错误：security.jwt.secret 未配置，请设置环境变量 JWT_SECRET")
	}
	if App.App.Port <= 0 {
		App.App.Port = 8080
	}
}

// loadConfig 从指定文件加载配置
// 先尝试从当前目录加载，失败则从父目录尝试
func loadConfig(filename string) {
	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		yamlFile, err = os.ReadFile("../" + filename)
		if err != nil {
			return
		}
	}

	// 替换配置文件中的环境变量占位符
	yamlFile = expandEnvVars(yamlFile)

	// 解析 YAML 到全局配置对象
	err = yaml.Unmarshal(yamlFile, &App)
	if err != nil {
		log.Fatalf("配置文件解析失败 %s: %v", filename, err)
	}
}

// overrideFromEnv 从环境变量覆盖敏感配置
// 优先级最高，用于覆盖配置文件中的敏感信息
func overrideFromEnv() {
	if v := os.Getenv("DB_HOST"); v != "" {
		App.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if port := Int(v); port > 0 {
			App.Database.Port = port
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		App.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		App.Database.Pass = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		App.Database.Name = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		App.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if port := Int(v); port > 0 {
			App.Redis.Port = port
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		App.Redis.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		App.Security.JWT.Secret = v
	}
	if v := os.Getenv("APP_PORT"); v != "" {
		if port := Int(v); port > 0 {
			App.App.Port = port
		}
	}
}

// applyDefaults 应用配置默认值
func applyDefaults() {
	// 应用基础配置默认值
	if App.App.Mode == "" {
		App.App.Mode = "dev"
	}

	// 数据库配置默认值
	if App.Database.Driver == "" {
		App.Database.Driver = "mysql"
	}
	if App.Database.Port == 0 {
		App.Database.Port = 3306
	}
	if App.Database.Charset == "" {
		App.Database.Charset = "utf8mb4"
	}
	if App.Database.MaxOpenConns <= 0 {
		App.Database.MaxOpenConns = 100
	}
	if App.Database.MaxIdleConns <= 0 {
		App.Database.MaxIdleConns = 10
	}
	if App.Database.ConnMaxLifetime <= 0 {
		App.Database.ConnMaxLifetime = 3600
	}
	if App.Database.ConnMaxIdleTime <= 0 {
		App.Database.ConnMaxIdleTime = 300
	}

	// Redis 配置默认值
	if App.Redis.Port == 0 {
		App.Redis.Port = 6379
	}
	if App.Redis.PoolSize <= 0 {
		App.Redis.PoolSize = 100
	}
	if App.Redis.MinIdleConns <= 0 {
		App.Redis.MinIdleConns = 10
	}

	// 安全配置默认值
	if App.Security.Session.Lifetime == 0 {
		App.Security.Session.Lifetime = 7200
	}
	if App.Security.Password.MinLength == 0 {
		App.Security.Password.MinLength = 6
	}

	// 压力测试配置默认值
	if App.Benchmark.MemLimitPercent == 0 {
		App.Benchmark.MemLimitPercent = 80
	}
	if App.Benchmark.CPULimitPercent == 0 {
		App.Benchmark.CPULimitPercent = 90
	}

	// 视图配置默认值
	if App.View.Path == "" {
		App.View.Path = "app/view"
	}
	if App.View.Suffix == "" {
		App.View.Suffix = ".html"
	}
	if App.View.Type == "" {
		App.View.Type = "template"
	}
	if App.View.CachePath == "" {
		App.View.CachePath = "runtime/view_cache"
	}
}

// Int 辅助函数：字符串转整数
func Int(v string) int {
	i, _ := strconv.Atoi(v)
	return i
}

// GetAllPorts 获取所有配置的端口
func GetAllPorts() map[string]int {
	ports := make(map[string]int)

	// 应用 HTTP 端口
	if App.App.Port > 0 {
		ports["http"] = App.App.Port
	}

	// gRPC 端口
	if App.GRPC.Enabled && App.GRPC.Port > 0 {
		ports["grpc"] = App.GRPC.Port
	}

	// Nacos 管理端口
	if App.Nacos.Admin.Enabled && App.Nacos.Admin.Port > 0 {
		ports["nacos"] = App.Nacos.Admin.Port
	}

	// RabbitMQ 管理端口
	if App.RabbitMQ.Admin.Enabled && App.RabbitMQ.Admin.Port > 0 {
		ports["rabbitmq"] = App.RabbitMQ.Admin.Port
	}

	return ports
}
