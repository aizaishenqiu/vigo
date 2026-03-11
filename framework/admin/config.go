package admin

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"vigo/config"
	"vigo/framework/mvc"

	"gopkg.in/yaml.v3"
)

// ConfigController 配置管理控制器
type ConfigController struct{}

// ConfigSection 配置章节
type ConfigSection struct {
	Name    string        `json:"name"`
	Title   string        `json:"title"`
	Icon    string        `json:"icon"`
	Fields  []ConfigField `json:"fields"`
	Enabled bool          `json:"enabled"`
}

// ConfigField 配置字段
type ConfigField struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type"` // string, int, bool, array, object
	Value       interface{} `json:"value"`
	Default     interface{} `json:"default"`
	Description string      `json:"description"`
	Required    bool        `json:"required"`
	Options     []string    `json:"options,omitempty"` // 可选值（用于 select）
}

// ConfigManager 配置管理器
type ConfigManager struct {
	config     map[string]interface{}
	configPath string
	mu         sync.RWMutex
}

var globalConfigManager *ConfigManager

// InitConfigManager 初始化配置管理器
func InitConfigManager() error {
	configPath := "config.yaml"

	// 读取配置文件
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，创建默认配置
			globalConfigManager = &ConfigManager{
				config:     make(map[string]interface{}),
				configPath: configPath,
			}
			return nil
		}
		return err
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	globalConfigManager = &ConfigManager{
		config:     cfg,
		configPath: configPath,
	}

	return nil
}

// GetConfigData 获取配置
func GetConfigData() map[string]interface{} {
	if globalConfigManager == nil {
		return make(map[string]interface{})
	}

	globalConfigManager.mu.RLock()
	defer globalConfigManager.mu.RUnlock()

	return globalConfigManager.config
}

// GetConfigValue 获取配置值（支持点分隔路径，如 app.name）
func GetConfigValue(path string) interface{} {
	if globalConfigManager == nil {
		return nil
	}

	globalConfigManager.mu.RLock()
	defer globalConfigManager.mu.RUnlock()

	return getConfigValueByPath(globalConfigManager.config, path)
}

// SetConfigValue 设置配置值
func SetConfigValue(path string, value interface{}) error {
	if globalConfigManager == nil {
		return fmt.Errorf("配置管理器未初始化")
	}

	globalConfigManager.mu.Lock()
	defer globalConfigManager.mu.Unlock()

	setConfigValueByPath(globalConfigManager.config, path, value)

	// 保存到文件
	return globalConfigManager.save()
}

// SaveConfig 保存配置到文件
func SaveConfig() error {
	if globalConfigManager == nil {
		return fmt.Errorf("配置管理器未初始化")
	}

	globalConfigManager.mu.Lock()
	defer globalConfigManager.mu.Unlock()

	return globalConfigManager.save()
}

func (m *ConfigManager) save() error {
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return err
	}

	// 备份原文件
	if _, err := os.Stat(m.configPath); err == nil {
		os.Rename(m.configPath, m.configPath+".bak")
	}

	return ioutil.WriteFile(m.configPath, data, 0644)
}

// getConfigValueByPath 根据路径获取配置值
func getConfigValueByPath(cfg map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	current := cfg

	for i, part := range parts {
		if i == len(parts)-1 {
			return current[part]
		}

		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}

	return current
}

// setConfigValueByPath 根据路径设置配置值
func setConfigValueByPath(cfg map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := cfg

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}

		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}

		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			// 类型不匹配，创建新的 map
			newMap := make(map[string]interface{})
			current[part] = newMap
			current = newMap
		}
	}
}

// GetConfigSections 获取配置章节（用于 tabs）
func GetConfigSections() []ConfigSection {
	cfg := GetConfigData()

	sections := []ConfigSection{
		{
			Name:    "app",
			Title:   "应用配置",
			Icon:    "layui-icon-app",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "app.name",
					Label:       "应用名称",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "app.name"),
					Description: "应用服务名称",
				},
				{
					Key:         "app.port",
					Label:       "端口",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "app.port"),
					Description: "HTTP 服务监听端口",
				},
				{
					Key:         "app.debug",
					Label:       "调试模式",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "app.debug"),
					Description: "是否开启调试模式",
				},
				{
					Key:         "app.mode",
					Label:       "运行模式",
					Type:        "select",
					Value:       getConfigValueByPath(cfg, "app.mode"),
					Options:     []string{"dev", "test", "prod"},
					Description: "运行环境模式",
				},
				{
					Key:         "app.console",
					Label:       "控制台输出",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "app.console"),
					Description: "是否显示控制台输出",
				},
				{
					Key:         "app.auto_kill_port",
					Label:       "自动释放端口",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "app.auto_kill_port"),
					Description: "启动时自动释放占用端口",
				},
				{
					Key:         "app.debug_toolbar",
					Label:       "调试工具栏",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "app.debug_toolbar"),
					Description: "是否显示调试工具栏",
				},
			},
		},
		{
			Name:    "security",
			Title:   "安全防护",
			Icon:    "layui-icon-security",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "security.enable_security_middleware",
					Label:       "安全中间件",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "security.enable_security_middleware"),
					Description: "启用 SQL 注入、XSS 等防护",
				},
				{
					Key:         "security.enable_csrf_protection",
					Label:       "CSRF 保护",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "security.enable_csrf_protection"),
					Description: "启用 CSRF 令牌保护",
				},
				{
					Key:         "security.enable_rate_limit",
					Label:       "速率限制",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "security.enable_rate_limit"),
					Description: "启用请求频率限制",
				},
				{
					Key:         "security.rate_limit",
					Label:       "限制频率",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "security.rate_limit"),
					Description: "每秒请求数限制",
				},
				{
					Key:         "security.enable_cors_domain_check",
					Label:       "CORS 域名验证",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "security.enable_cors_domain_check"),
					Description: "启用 CORS 域名白名单验证",
				},
			},
		},
		{
			Name:    "database",
			Title:   "数据库配置",
			Icon:    "layui-icon-db",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "database.driver",
					Label:       "数据库驱动",
					Type:        "select",
					Value:       getConfigValueByPath(cfg, "database.driver"),
					Options:     []string{"mysql", "postgres", "sqlite", "sqlserver"},
					Description: "数据库类型",
				},
				{
					Key:         "database.host",
					Label:       "主机地址",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "database.host"),
					Description: "数据库服务器地址",
				},
				{
					Key:         "database.port",
					Label:       "端口",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "database.port"),
					Description: "数据库端口",
				},
				{
					Key:         "database.name",
					Label:       "数据库名",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "database.name"),
					Description: "数据库名称",
				},
				{
					Key:         "database.user",
					Label:       "用户名",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "database.user"),
					Description: "数据库用户名",
				},
				{
					Key:         "database.pass",
					Label:       "密码",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "database.pass"),
					Description: "数据库密码",
				},
				{
					Key:         "database.charset",
					Label:       "字符集",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "database.charset"),
					Description: "数据库字符集",
				},
				{
					Key:         "database.max_open_conns",
					Label:       "最大连接数",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "database.max_open_conns"),
					Description: "最大打开连接数",
				},
				{
					Key:         "database.max_idle_conns",
					Label:       "最大空闲连接",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "database.max_idle_conns"),
					Description: "最大空闲连接数",
				},
				{
					Key:         "database.conn_max_lifetime",
					Label:       "连接生命周期",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "database.conn_max_lifetime"),
					Description: "连接最大生命周期（秒）",
				},
				{
					Key:         "database.rw_split",
					Label:       "读写分离",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "database.rw_split"),
					Description: "是否开启读写分离",
				},
			},
		},
		{
			Name:    "redis",
			Title:   "Redis 配置",
			Icon:    "layui-icon-redis",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "redis.host",
					Label:       "主机地址",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "redis.host"),
					Description: "Redis 服务器地址",
				},
				{
					Key:         "redis.port",
					Label:       "端口",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "redis.port"),
					Description: "Redis 端口",
				},
				{
					Key:         "redis.password",
					Label:       "密码",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "redis.password"),
					Description: "Redis 密码",
				},
				{
					Key:         "redis.db",
					Label:       "数据库",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "redis.db"),
					Description: "Redis 数据库编号",
				},
				{
					Key:         "redis.pool_size",
					Label:       "连接池大小",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "redis.pool_size"),
					Description: "最大连接池大小",
				},
				{
					Key:         "redis.min_idle_conns",
					Label:       "最小空闲连接",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "redis.min_idle_conns"),
					Description: "最小空闲连接数",
				},
				{
					Key:         "redis.max_idle_conns",
					Label:       "最大空闲连接",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "redis.max_idle_conns"),
					Description: "最大空闲连接数",
				},
			},
		},
		{
			Name:    "rabbitmq",
			Title:   "RabbitMQ 配置",
			Icon:    "layui-icon-mq",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "rabbitmq.enabled",
					Label:       "启用 RabbitMQ",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "rabbitmq.enabled"),
					Description: "是否启用 RabbitMQ",
				},
				{
					Key:         "rabbitmq.host",
					Label:       "主机地址",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "rabbitmq.host"),
					Description: "RabbitMQ 服务器地址",
				},
				{
					Key:         "rabbitmq.port",
					Label:       "端口",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "rabbitmq.port"),
					Description: "RabbitMQ 端口",
				},
				{
					Key:         "rabbitmq.user",
					Label:       "用户名",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "rabbitmq.user"),
					Description: "RabbitMQ 用户名",
				},
				{
					Key:         "rabbitmq.pass",
					Label:       "密码",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "rabbitmq.pass"),
					Description: "RabbitMQ 密码",
				},
				{
					Key:         "rabbitmq.vhost",
					Label:       "虚拟主机",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "rabbitmq.vhost"),
					Description: "RabbitMQ 虚拟主机",
				},
				{
					Key:         "rabbitmq.conn_timeout",
					Label:       "连接超时",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "rabbitmq.conn_timeout"),
					Description: "连接超时时间（秒）",
				},
				{
					Key:         "rabbitmq.heartbeat",
					Label:       "心跳间隔",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "rabbitmq.heartbeat"),
					Description: "心跳间隔（秒）",
				},
			},
		},
		{
			Name:    "nacos",
			Title:   "Nacos 配置",
			Icon:    "layui-icon-nacos",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "nacos.enabled",
					Label:       "启用 Nacos",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "nacos.enabled"),
					Description: "是否启用 Nacos",
				},
				{
					Key:         "nacos.host",
					Label:       "主机地址",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "nacos.host"),
					Description: "Nacos 服务器地址",
				},
				{
					Key:         "nacos.port",
					Label:       "端口",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "nacos.port"),
					Description: "Nacos 端口",
				},
				{
					Key:         "nacos.namespace",
					Label:       "命名空间",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "nacos.namespace"),
					Description: "Nacos 命名空间 ID",
				},
				{
					Key:         "nacos.data_id",
					Label:       "数据 ID",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "nacos.data_id"),
					Description: "配置数据 ID",
				},
				{
					Key:         "nacos.group",
					Label:       "分组",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "nacos.group"),
					Description: "配置分组",
				},
			},
		},
		{
			Name:    "payment",
			Title:   "支付配置",
			Icon:    "layui-icon-payment",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "payment.alipay.enabled",
					Label:       "支付宝启用",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "payment.alipay.enabled"),
					Description: "是否启用支付宝支付",
				},
				{
					Key:         "payment.alipay.app_id",
					Label:       "支付宝 AppID",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "payment.alipay.app_id"),
					Description: "支付宝应用 ID",
				},
				{
					Key:         "payment.alipay.app_private_key",
					Label:       "支付宝私钥",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "payment.alipay.app_private_key"),
					Description: "支付宝应用私钥",
				},
				{
					Key:         "payment.alipay.alipay_public_key",
					Label:       "支付宝公钥",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "payment.alipay.alipay_public_key"),
					Description: "支付宝公钥",
				},
				{
					Key:         "payment.wechat_pay.enabled",
					Label:       "微信支付启用",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "payment.wechat_pay.enabled"),
					Description: "是否启用微信支付",
				},
				{
					Key:         "payment.wechat_pay.mch_id",
					Label:       "微信商户号",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "payment.wechat_pay.mch_id"),
					Description: "微信支付商户号",
				},
				{
					Key:         "payment.wechat_pay.api_key",
					Label:       "微信 API 密钥",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "payment.wechat_pay.api_key"),
					Description: "微信支付 API 密钥",
				},
			},
		},
		{
			Name:    "oauth",
			Title:   "第三方登录",
			Icon:    "layui-icon-oauth",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "oauth.qq.enabled",
					Label:       "QQ 登录启用",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "oauth.qq.enabled"),
					Description: "是否启用 QQ 登录",
				},
				{
					Key:         "oauth.qq.app_id",
					Label:       "QQ AppID",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "oauth.qq.app_id"),
					Description: "QQ 应用 ID",
				},
				{
					Key:         "oauth.qq.app_key",
					Label:       "QQ AppKey",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "oauth.qq.app_key"),
					Description: "QQ 应用密钥",
				},
				{
					Key:         "oauth.wechat.enabled",
					Label:       "微信登录启用",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "oauth.wechat.enabled"),
					Description: "是否启用微信登录",
				},
				{
					Key:         "oauth.wechat.app_id",
					Label:       "微信 AppID",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "oauth.wechat.app_id"),
					Description: "微信应用 ID",
				},
				{
					Key:         "oauth.wechat.app_secret",
					Label:       "微信 AppSecret",
					Type:        "password",
					Value:       getConfigValueByPath(cfg, "oauth.wechat.app_secret"),
					Description: "微信应用密钥",
				},
			},
		},
		{
			Name:    "grpc",
			Title:   "gRPC 配置",
			Icon:    "layui-icon-grpc",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "grpc.enabled",
					Label:       "启用 gRPC",
					Type:        "bool",
					Value:       getConfigValueByPath(cfg, "grpc.enabled"),
					Description: "是否启用 gRPC 服务",
				},
				{
					Key:         "grpc.port",
					Label:       "gRPC 端口",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "grpc.port"),
					Description: "gRPC 服务端口",
				},
				{
					Key:         "grpc.service_name",
					Label:       "服务名称",
					Type:        "string",
					Value:       getConfigValueByPath(cfg, "grpc.service_name"),
					Description: "gRPC 服务名称",
				},
			},
		},
		{
			Name:    "benchmark",
			Title:   "压力测试",
			Icon:    "layui-icon-benchmark",
			Enabled: true,
			Fields: []ConfigField{
				{
					Key:         "benchmark.mem_limit_percent",
					Label:       "内存限制",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "benchmark.mem_limit_percent"),
					Description: "内存限制百分比",
				},
				{
					Key:         "benchmark.cpu_limit_percent",
					Label:       "CPU 限制",
					Type:        "int",
					Value:       getConfigValueByPath(cfg, "benchmark.cpu_limit_percent"),
					Description: "CPU 限制百分比",
				},
			},
		},
	}

	return sections
}

// configIndex 配置管理首页
func configIndex(c *mvc.Context) {
	cfg := GetConfigData()

	data := map[string]interface{}{
		"title":     "系统配置",
		"base_path": GlobalManager.config.BasePath,
		"config":    cfg,
	}

	tmpl, err := template.ParseFS(adminViews, "views/config/index.html")
	if err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "模板加载失败：" + err.Error(),
		})
		return
	}

	tmpl.Execute(c.Writer, data)
}

// getConfig 获取所有配置
func getConfig(c *mvc.Context) {
	cfg := GetConfigData()

	c.Json(200, map[string]interface{}{
		"code":   0,
		"msg":    "success",
		"data":   cfg,
		"config": cfg, // 兼容旧接口
	})
}

// saveConfig 保存配置（按字段更新）
func saveConfig(c *mvc.Context) {
	var req struct {
		Section string                 `json:"section"`
		Key     string                 `json:"key"`
		Value   interface{}            `json:"value"`
		Data    map[string]interface{} `json:"data"`
	}

	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "JSON 解析失败：" + err.Error(),
		})
		return
	}

	// 支持两种模式：
	// 1. 单个字段更新：section + key + value
	// 2. 批量更新：data 包含多个键值对

	if req.Data != nil {
		// 批量更新模式
		for key, value := range req.Data {
			if err := SetConfigValue(key, value); err != nil {
				c.Json(500, map[string]interface{}{
					"code": 500,
					"msg":  fmt.Sprintf("保存配置 %s 失败：%v", key, err),
				})
				return
			}
		}

		c.Json(200, map[string]interface{}{
			"code": 0,
			"msg":  "配置保存成功",
		})
		return
	}

	// 单字段更新模式
	if req.Key == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "缺少 key 参数",
		})
		return
	}

	// 如果提供了 section，自动拼接为 section.key
	fullKey := req.Key
	if req.Section != "" && !strings.HasPrefix(req.Key, req.Section+".") {
		fullKey = req.Section + "." + req.Key
	}

	if err := SetConfigValue(fullKey, req.Value); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  "保存配置失败：" + err.Error(),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置保存成功",
	})
}

// saveSection 保存整个章节的配置
func saveSection(c *mvc.Context) {
	var req struct {
		Section string                 `json:"section"`
		Fields  map[string]interface{} `json:"fields"`
	}

	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "JSON 解析失败：" + err.Error(),
		})
		return
	}

	if req.Section == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "缺少 section 参数",
		})
		return
	}

	// 批量保存该章节的所有字段
	for key, value := range req.Fields {
		fullKey := req.Section + "." + key
		if err := SetConfigValue(fullKey, value); err != nil {
			c.Json(500, map[string]interface{}{
				"code": 500,
				"msg":  fmt.Sprintf("保存配置 %s 失败：%v", fullKey, err),
			})
			return
		}
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "配置保存成功",
	})
}

// getSection 获取指定章节的配置
func getSection(c *mvc.Context) {
	section := c.Request.URL.Query().Get("section")
	if section == "" {
		c.Json(400, map[string]interface{}{
			"code": 400,
			"msg":  "缺少 section 参数",
		})
		return
	}

	cfg := GetConfigData()
	sectionData := getConfigValueByPath(cfg, section)

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": sectionData,
	})
}

// 工具函数：更新 YAML 文件中的值（保留注释）
func updateYamlValue(key string, value interface{}) error {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	parts := strings.Split(key, ".")
	targetKey := parts[len(parts)-1]

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, targetKey+":") {
			// 保留缩进
			indent := len(line) - len(strings.TrimLeft(line, " "))
			newValue := formatYamlValue(value)
			lines[i] = strings.Repeat(" ", indent) + targetKey + ": " + newValue
			break
		}
	}

	return ioutil.WriteFile("config.yaml", []byte(strings.Join(lines, "\n")), 0644)
}

func updateYamlValueByPath(path string, value interface{}) error {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	parts := strings.Split(path, ".")
	start := 0
	parentIndent := -1

	for level, part := range parts {
		found := -1
		for i := start; i < len(lines); i++ {
			trimmed := strings.TrimSpace(lines[i])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			indent := len(lines[i]) - len(strings.TrimLeft(lines[i], " "))
			if parentIndent >= 0 && indent <= parentIndent {
				break
			}

			if level == 0 && indent != 0 {
				continue
			}

			if strings.HasPrefix(trimmed, part+":") {
				found = i
				break
			}
		}

		if found == -1 {
			return fmt.Errorf("path %s not found", path)
		}

		if level == len(parts)-1 {
			indent := len(lines[found]) - len(strings.TrimLeft(lines[found], " "))
			lines[found] = strings.Repeat(" ", indent) + part + ": " + formatYamlValue(value)
			return ioutil.WriteFile("config.yaml", []byte(strings.Join(lines, "\n")), 0644)
		}

		parentIndent = len(lines[found]) - len(strings.TrimLeft(lines[found], " "))
		start = found + 1
	}

	return fmt.Errorf("path %s not found", path)
}

// 更新 YAML 数组值
func updateYamlArray(key string, values []string) error {
	if len(values) == 0 {
		return updateYamlValue(key, []string{})
	}

	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	parts := strings.Split(key, ".")
	targetKey := parts[len(parts)-1]

	// 查找目标键的位置
	targetIndex := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, targetKey+":") {
			targetIndex = i
			break
		}
	}

	if targetIndex == -1 {
		return fmt.Errorf("key %s not found", key)
	}

	// 计算缩进
	indent := len(lines[targetIndex]) - len(strings.TrimLeft(lines[targetIndex], " "))

	// 构建新的数组内容
	newLines := make([]string, 0)
	newLines = append(newLines, lines[:targetIndex]...)
	newLines = append(newLines, strings.Repeat(" ", indent)+targetKey+":")

	for _, val := range values {
		newLines = append(newLines, strings.Repeat(" ", indent+2)+"- "+val)
	}

	newLines = append(newLines, lines[targetIndex+1:]...)

	return ioutil.WriteFile("config.yaml", []byte(strings.Join(newLines, "\n")), 0644)
}

// 格式化 YAML 值
func formatYamlValue(value interface{}) string {
	if value == nil {
		return ""
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int, reflect.Int64, reflect.Int32:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Float64, reflect.Float32:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.String:
		s := v.String()
		if strings.Contains(s, "\n") || strings.Contains(s, "#") {
			// 多行字符串或包含注释，使用引号
			return "\"" + strings.ReplaceAll(s, "\"", "\\\"") + "\""
		}
		return s
	default:
		// 复杂类型转换为 YAML
		data, _ := yaml.Marshal(value)
		return strings.TrimSpace(string(data))
	}
}

// 辅助函数：从 map 中获取值，如果不存在则返回默认值
func getMapValue(m map[string]interface{}, key string, defaultVal interface{}) interface{} {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultVal
}

// ==================== 扩展配置 API ====================

// listDatabases 获取所有数据库配置
func listDatabases(c *mvc.Context) {
	cfg := GetConfigData()
	var list []map[string]interface{}

	// 1. 主库
	if db, ok := cfg["database"].(map[string]interface{}); ok {
		list = append(list, map[string]interface{}{
			"id":                "main",
			"type":              "主数据库 (Main)",
			"driver":            getMapValue(db, "driver", "mysql"),
			"host":              getMapValue(db, "host", "127.0.0.1"),
			"port":              getMapValue(db, "port", 3306),
			"name":              getMapValue(db, "name", ""),
			"user":              getMapValue(db, "user", ""),
			"password":          getMapValue(db, "pass", ""), // Note: YAML key is 'pass'
			"charset":           getMapValue(db, "charset", "utf8mb4"),
			"max_open_conns":    getMapValue(db, "max_open_conns", 100),
			"max_idle_conns":    getMapValue(db, "max_idle_conns", 10),
			"conn_max_lifetime": getMapValue(db, "conn_max_lifetime", 3600),
			"conn_max_idletime": getMapValue(db, "conn_max_idletime", 600),
			"rw_split":          getMapValue(db, "rw_split", false),
		})

		// 2. 写库 (Writes)
		if writes, ok := db["writes"].([]interface{}); ok {
			for i, w := range writes {
				if wMap, ok := w.(map[string]interface{}); ok {
					list = append(list, map[string]interface{}{
						"id":       fmt.Sprintf("write_%d", i),
						"type":     "写数据库 (Write)",
						"driver":   getMapValue(db, "driver", "mysql"),
						"host":     getMapValue(wMap, "host", ""),
						"port":     getMapValue(wMap, "port", 3306),
						"name":     getMapValue(db, "name", ""),
						"user":     getMapValue(wMap, "user", ""),
						"password": getMapValue(wMap, "pass", ""),
						"charset":  getMapValue(wMap, "charset", "utf8mb4"),
					})
				}
			}
		}

		// 3. 读库 (Reads)
		if reads, ok := db["reads"].([]interface{}); ok {
			for i, r := range reads {
				if rMap, ok := r.(map[string]interface{}); ok {
					list = append(list, map[string]interface{}{
						"id":       fmt.Sprintf("read_%d", i),
						"type":     "读数据库 (Read)",
						"driver":   getMapValue(db, "driver", "mysql"),
						"host":     getMapValue(rMap, "host", ""),
						"port":     getMapValue(rMap, "port", 3306),
						"name":     getMapValue(db, "name", ""),
						"user":     getMapValue(rMap, "user", ""),
						"password": getMapValue(rMap, "pass", ""),
						"charset":  getMapValue(rMap, "charset", "utf8mb4"),
					})
				}
			}
		}
	}

	// 4. 多数据库 (Multi)
	if multiDBs, ok := cfg["databases"].(map[string]interface{}); ok {
		for key, db := range multiDBs {
			if dbMap, ok := db.(map[string]interface{}); ok {
				list = append(list, map[string]interface{}{
					"id":                "multi_" + key,
					"type":              "多数据库 (Multi)",
					"key":               key,
					"driver":            getMapValue(dbMap, "driver", "mysql"),
					"host":              getMapValue(dbMap, "host", ""),
					"port":              getMapValue(dbMap, "port", 3306),
					"name":              getMapValue(dbMap, "name", ""),
					"user":              getMapValue(dbMap, "user", ""),
					"password":          getMapValue(dbMap, "pass", ""),
					"charset":           getMapValue(dbMap, "charset", "utf8mb4"),
					"max_open_conns":    getMapValue(dbMap, "max_open_conns", 100),
					"max_idle_conns":    getMapValue(dbMap, "max_idle_conns", 10),
					"conn_max_lifetime": getMapValue(dbMap, "conn_max_lifetime", 3600),
					"conn_max_idletime": getMapValue(dbMap, "conn_max_idletime", 600),
					"rw_split":          getMapValue(dbMap, "rw_split", false),
				})
			}
		}
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": list,
	})
}

// addDatabase 添加数据库配置
func addDatabase(c *mvc.Context) {
	var req struct {
		Type   string `json:"type"` // main, read, write, multi
		Key    string `json:"key"`  // only for multi
		Config struct {
			Driver          string `json:"driver"`
			Host            string `json:"host"`
			Port            int    `json:"port"`
			Name            string `json:"name"`
			User            string `json:"user"`
			Password        string `json:"password"`
			Charset         string `json:"charset"`
			MaxOpenConns    int    `json:"max_open_conns"`
			MaxIdleConns    int    `json:"max_idle_conns"`
			ConnMaxLifetime int    `json:"conn_max_lifetime"`
			ConnMaxIdleTime int    `json:"conn_max_idletime"`
			RWSplit         bool   `json:"rw_split"`
		} `json:"config"`
	}

	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "无效的请求"})
		return
	}

	cfg := GetConfigData()
	dbCfg, ok := cfg["database"].(map[string]interface{})
	if !ok {
		dbCfg = make(map[string]interface{})
		cfg["database"] = dbCfg
	}

	switch req.Type {
	case "main":
		c.Json(400, map[string]interface{}{"code": 400, "msg": "主库已存在，请使用编辑功能"})
		return
	case "read":
		reads, ok := dbCfg["reads"].([]interface{})
		if !ok {
			reads = []interface{}{}
		}
		newRead := map[string]interface{}{
			"host":    req.Config.Host,
			"port":    req.Config.Port,
			"user":    req.Config.User,
			"pass":    req.Config.Password,
			"charset": req.Config.Charset,
		}
		reads = append(reads, newRead)
		dbCfg["reads"] = reads
		SetConfigValue("database.reads", reads)
	case "write":
		writes, ok := dbCfg["writes"].([]interface{})
		if !ok {
			writes = []interface{}{}
		}
		newWrite := map[string]interface{}{
			"host":    req.Config.Host,
			"port":    req.Config.Port,
			"user":    req.Config.User,
			"pass":    req.Config.Password,
			"charset": req.Config.Charset,
		}
		writes = append(writes, newWrite)
		dbCfg["writes"] = writes
		SetConfigValue("database.writes", writes)
	case "multi":
		if req.Key == "" {
			c.Json(400, map[string]interface{}{"code": 400, "msg": "多数据库必须指定 Key"})
			return
		}
		dbs, ok := cfg["databases"].(map[string]interface{})
		if !ok {
			dbs = make(map[string]interface{})
			cfg["databases"] = dbs
		}
		if _, exists := dbs[req.Key]; exists {
			c.Json(400, map[string]interface{}{"code": 400, "msg": "该数据库 Key 已存在"})
			return
		}
		dbs[req.Key] = map[string]interface{}{
			"driver":            req.Config.Driver,
			"host":              req.Config.Host,
			"port":              req.Config.Port,
			"name":              req.Config.Name,
			"user":              req.Config.User,
			"pass":              req.Config.Password,
			"charset":           req.Config.Charset,
			"max_open_conns":    req.Config.MaxOpenConns,
			"max_idle_conns":    req.Config.MaxIdleConns,
			"conn_max_lifetime": req.Config.ConnMaxLifetime,
			"conn_max_idletime": req.Config.ConnMaxIdleTime,
			"rw_split":          req.Config.RWSplit,
			"reads":             []interface{}{},
			"writes":            []interface{}{},
		}
		SetConfigValue("databases", dbs)
	default:
		c.Json(400, map[string]interface{}{"code": 400, "msg": "未知的数据库类型"})
		return
	}

	// 重新加载配置 (简化处理，实际应该通知应用重新加载)
	config.Init()

	c.Json(200, map[string]interface{}{"code": 0, "msg": "添加成功"})
}

// updateDatabase 更新数据库配置
func updateDatabase(c *mvc.Context) {
	var req struct {
		ID     string `json:"id"`
		Type   string `json:"type"`
		Key    string `json:"key"`
		Config struct {
			Driver          string `json:"driver"`
			Host            string `json:"host"`
			Port            int    `json:"port"`
			Name            string `json:"name"`
			User            string `json:"user"`
			Password        string `json:"password"`
			Charset         string `json:"charset"`
			MaxOpenConns    int    `json:"max_open_conns"`
			MaxIdleConns    int    `json:"max_idle_conns"`
			ConnMaxLifetime int    `json:"conn_max_lifetime"`
			ConnMaxIdleTime int    `json:"conn_max_idletime"`
			RWSplit         bool   `json:"rw_split"`
		} `json:"config"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "无效的请求"})
		return
	}

	cfg := GetConfigData()
	dbCfg, ok := cfg["database"].(map[string]interface{})
	if !ok {
		c.Json(500, map[string]interface{}{"code": 500, "msg": "配置错误"})
		return
	}

	if req.ID == "main" {
		main := map[string]interface{}{
			"driver":            req.Config.Driver,
			"host":              req.Config.Host,
			"port":              req.Config.Port,
			"name":              req.Config.Name,
			"user":              req.Config.User,
			"pass":              req.Config.Password,
			"charset":           req.Config.Charset,
			"max_open_conns":    req.Config.MaxOpenConns,
			"max_idle_conns":    req.Config.MaxIdleConns,
			"conn_max_lifetime": req.Config.ConnMaxLifetime,
			"conn_max_idletime": req.Config.ConnMaxIdleTime,
			"rw_split":          req.Config.RWSplit,
			"reads":             getMapValue(dbCfg, "reads", []interface{}{}),
			"writes":            getMapValue(dbCfg, "writes", []interface{}{}),
		}
		if err := SetConfigValue("database", main); err != nil {
			c.Json(500, map[string]interface{}{"code": 500, "msg": "保存失败：" + err.Error()})
			return
		}
		config.Init()
		c.Json(200, map[string]interface{}{"code": 0, "msg": "主数据库更新成功"})
		return
	}

	if strings.HasPrefix(req.ID, "read_") {
		idx := 0
		fmt.Sscanf(req.ID, "read_%d", &idx)
		reads, ok := dbCfg["reads"].([]interface{})
		if !ok || idx < 0 || idx >= len(reads) {
			c.Json(400, map[string]interface{}{"code": 400, "msg": "读库索引无效"})
			return
		}
		reads[idx] = map[string]interface{}{
			"host":    req.Config.Host,
			"port":    req.Config.Port,
			"user":    req.Config.User,
			"pass":    req.Config.Password,
			"charset": req.Config.Charset,
		}
		if err := SetConfigValue("database.reads", reads); err != nil {
			c.Json(500, map[string]interface{}{"code": 500, "msg": "保存失败：" + err.Error()})
			return
		}
		config.Init()
		c.Json(200, map[string]interface{}{"code": 0, "msg": "读数据库更新成功"})
		return
	}

	if strings.HasPrefix(req.ID, "write_") {
		idx := 0
		fmt.Sscanf(req.ID, "write_%d", &idx)
		writes, ok := dbCfg["writes"].([]interface{})
		if !ok || idx < 0 || idx >= len(writes) {
			c.Json(400, map[string]interface{}{"code": 400, "msg": "写库索引无效"})
			return
		}
		writes[idx] = map[string]interface{}{
			"host":    req.Config.Host,
			"port":    req.Config.Port,
			"user":    req.Config.User,
			"pass":    req.Config.Password,
			"charset": req.Config.Charset,
		}
		if err := SetConfigValue("database.writes", writes); err != nil {
			c.Json(500, map[string]interface{}{"code": 500, "msg": "保存失败：" + err.Error()})
			return
		}
		config.Init()
		c.Json(200, map[string]interface{}{"code": 0, "msg": "写数据库更新成功"})
		return
	}

	if strings.HasPrefix(req.ID, "multi_") || req.Type == "multi" {
		if req.Key == "" {
			req.Key = strings.TrimPrefix(req.ID, "multi_")
		}
		dbs, ok := cfg["databases"].(map[string]interface{})
		if !ok {
			c.Json(400, map[string]interface{}{"code": 400, "msg": "多数据库不存在"})
			return
		}
		if _, exists := dbs[req.Key]; !exists {
			c.Json(400, map[string]interface{}{"code": 400, "msg": "目标多数据库不存在"})
			return
		}
		dbs[req.Key] = map[string]interface{}{
			"driver":            req.Config.Driver,
			"host":              req.Config.Host,
			"port":              req.Config.Port,
			"name":              req.Config.Name,
			"user":              req.Config.User,
			"pass":              req.Config.Password,
			"charset":           req.Config.Charset,
			"max_open_conns":    req.Config.MaxOpenConns,
			"max_idle_conns":    req.Config.MaxIdleConns,
			"conn_max_lifetime": req.Config.ConnMaxLifetime,
			"conn_max_idletime": req.Config.ConnMaxIdleTime,
			"rw_split":          req.Config.RWSplit,
			"reads":             getMapValue(dbs[req.Key].(map[string]interface{}), "reads", []interface{}{}),
			"writes":            getMapValue(dbs[req.Key].(map[string]interface{}), "writes", []interface{}{}),
		}
		if err := SetConfigValue("databases", dbs); err != nil {
			c.Json(500, map[string]interface{}{"code": 500, "msg": "保存失败：" + err.Error()})
			return
		}
		config.Init()
		c.Json(200, map[string]interface{}{"code": 0, "msg": "多数据库更新成功"})
		return
	}

	c.Json(400, map[string]interface{}{"code": 400, "msg": "不支持的数据库类型"})
}

// removeDatabase 删除数据库配置
func removeDatabase(c *mvc.Context) {
	var req struct {
		ID   string `json:"id"`
		Type string `json:"type"`
		Key  string `json:"key"`
	}
	json.NewDecoder(c.Request.Body).Decode(&req)

	cfg := GetConfigData()
	dbCfg, ok := cfg["database"].(map[string]interface{})
	if !ok {
		c.Json(500, map[string]interface{}{"code": 500, "msg": "配置错误"})
		return
	}

	if req.Type == "multi" || (req.Key != "" && strings.HasPrefix(req.ID, "multi_")) {
		if req.Key == "" {
			req.Key = strings.TrimPrefix(req.ID, "multi_")
		}
		dbs, ok := cfg["databases"].(map[string]interface{})
		if ok {
			delete(dbs, req.Key)
			SetConfigValue("databases", dbs)
		}
	} else if strings.HasPrefix(req.ID, "read_") {
		idx := 0
		fmt.Sscanf(req.ID, "read_%d", &idx)
		reads, ok := dbCfg["reads"].([]interface{})
		if ok && idx >= 0 && idx < len(reads) {
			// Remove element at index
			reads = append(reads[:idx], reads[idx+1:]...)
			SetConfigValue("database.reads", reads)
		}
	} else if strings.HasPrefix(req.ID, "write_") {
		idx := 0
		fmt.Sscanf(req.ID, "write_%d", &idx)
		writes, ok := dbCfg["writes"].([]interface{})
		if ok && idx >= 0 && idx < len(writes) {
			writes = append(writes[:idx], writes[idx+1:]...)
			SetConfigValue("database.writes", writes)
		}
	} else if req.ID == "main" {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "主库不能删除"})
		return
	}

	c.Json(200, map[string]interface{}{"code": 0, "msg": "删除成功"})
}

// ========== 配置页面处理器（用于 iframe 嵌入） ==========

// configPageApp 应用配置页面
func configPageApp(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "应用配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/app.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageSecurity 安全配置页面
func configPageSecurity(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "安全防护",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/security.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageDatabase 数据库配置页面
func configPageDatabase(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "数据库配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/database.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageRedis Redis 配置页面
func configPageRedis(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "Redis 配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/redis.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageRabbitmq RabbitMQ 配置页面
func configPageRabbitmq(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "RabbitMQ 配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/rabbitmq.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageNacos Nacos 配置页面
func configPageNacos(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "Nacos 配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/nacos.html"))
	tmpl.Execute(c.Writer, data)
}

// configPagePayment 支付配置页面
func configPagePayment(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "支付配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/payment.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageOauth OAuth 配置页面
func configPageOauth(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "第三方登录",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/oauth.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageGrpc gRPC 配置页面
func configPageGrpc(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "gRPC 配置",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/grpc.html"))
	tmpl.Execute(c.Writer, data)
}

// configPageBenchmark 压力测试配置页面
func configPageBenchmark(c *mvc.Context) {
	data := map[string]interface{}{
		"title":  "压力测试",
		"config": GetConfigData(),
	}
	tmpl := template.Must(template.ParseFS(adminViews, "views/config/pages/benchmark.html"))
	tmpl.Execute(c.Writer, data)
}
