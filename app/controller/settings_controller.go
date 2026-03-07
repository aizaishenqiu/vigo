package controller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"vigo/framework/mvc"

	"gopkg.in/yaml.v3"
)

// SettingsController 系统设置控制器
type SettingsController struct {
	BaseController
}

// Index 设置页面
func (c *SettingsController) Index(ctx *mvc.Context) {
	ctx.HTML(200, "settings/index.html", map[string]interface{}{
		"title": "系统设置",
	})
}

// Get 获取所有配置
func (c *SettingsController) Get(ctx *mvc.Context) {
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析配置文件失败：%v", err))
		return
	}

	ctx.Success(map[string]interface{}{
		"config": cfg,
	})
}

// Save 保存完整配置
func (c *SettingsController) Save(ctx *mvc.Context) {
	body := ctx.Input("config")
	if body == "" {
		ctx.Error(400, "配置数据不能为空")
		return
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(body), &cfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	// 读取现有配置保留注释
	configData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	// 解析现有配置
	var existingCfg map[string]interface{}
	if err := yaml.Unmarshal(configData, &existingCfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析现有配置失败：%v", err))
		return
	}

	// 合并配置
	mergeConfig(existingCfg, cfg)

	// 转换为 YAML
	yamlData, err := yaml.Marshal(existingCfg)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("转换配置失败：%v", err))
		return
	}

	// 备份原配置文件
	ioutil.WriteFile("config.yaml.bak", configData, 0644)

	// 写入新配置
	if err := ioutil.WriteFile("config.yaml", yamlData, 0644); err != nil {
		ctx.Error(500, fmt.Sprintf("保存配置文件失败：%v", err))
		ioutil.WriteFile("config.yaml", configData, 0644)
		return
	}

	ctx.Success(map[string]interface{}{
		"message": "配置保存成功",
	})
}

// SaveApp 保存应用配置
func (c *SettingsController) SaveApp(ctx *mvc.Context) {
	type AppConfig struct {
		Name         string `json:"name"`
		Port         int    `json:"port"`
		Debug        bool   `json:"debug"`
		Mode         string `json:"mode"`
		Console      bool   `json:"console"`
		AutoKillPort bool   `json:"auto_kill_port"`
		DebugToolbar bool   `json:"debug_toolbar"`
	}

	// 读取请求体
	body, err := ioutil.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.Error(400, fmt.Sprintf("读取请求体失败：%v", err))
		return
	}
	defer ctx.Request.Body.Close()

	// 解析 JSON
	var reqData map[string]interface{}
	if err := json.Unmarshal(body, &reqData); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	// 获取 config 对象
	configData, ok := reqData["config"].(map[string]interface{})
	if !ok {
		ctx.Error(400, "配置数据格式错误")
		return
	}

	// 转换为 AppConfig 结构
	configBytes, _ := json.Marshal(configData)
	var appCfg AppConfig
	if err := json.Unmarshal(configBytes, &appCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	// 直接修改 config.yaml 文件中的对应行
	updateYamlValue("app.name", appCfg.Name)
	updateYamlValue("app.port", appCfg.Port)
	updateYamlValue("app.debug", appCfg.Debug)
	updateYamlValue("app.mode", appCfg.Mode)
	updateYamlValue("app.console", appCfg.Console)
	updateYamlValue("app.auto_kill_port", appCfg.AutoKillPort)
	updateYamlValue("app.debug_toolbar", appCfg.DebugToolbar)

	ctx.Success(map[string]interface{}{
		"message": "应用配置保存成功",
	})
}

// SaveDatabase 保存数据库配置
func (c *SettingsController) SaveDatabase(ctx *mvc.Context) {
	type DatabaseConfig struct {
		Driver   string `json:"driver"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
		Charset  string `json:"charset"`
	}

	var dbCfg DatabaseConfig
	if err := json.Unmarshal([]byte(ctx.Input("config")), &dbCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	updateConfigValue("database.driver", dbCfg.Driver)
	updateConfigValue("database.host", dbCfg.Host)
	updateConfigValue("database.port", dbCfg.Port)
	updateConfigValue("database.name", dbCfg.Name)
	updateConfigValue("database.user", dbCfg.User)
	updateConfigValue("database.pass", dbCfg.Password)
	updateConfigValue("database.charset", dbCfg.Charset)

	ctx.Success(map[string]interface{}{
		"message": "数据库配置保存成功",
	})
}

// AddDatabase 添加多数据库
func (c *SettingsController) AddDatabase(ctx *mvc.Context) {
	name := ctx.Input("name")
	configData := ctx.Input("config")

	if name == "" {
		ctx.Error(400, "数据库名称不能为空")
		return
	}

	if configData == "" {
		ctx.Error(400, "配置数据不能为空")
		return
	}

	// 读取现有配置
	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析配置文件失败：%v", err))
		return
	}

	// 解析新数据库配置
	var dbCfg map[string]interface{}
	if err := json.Unmarshal([]byte(configData), &dbCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析数据库配置失败：%v", err))
		return
	}

	// 初始化 databases 如果不存在
	if cfg["databases"] == nil {
		cfg["databases"] = make(map[string]interface{})
	}

	// 添加新数据库
	databases := cfg["databases"].(map[string]interface{})
	databases[name] = dbCfg

	// 保存
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("转换配置失败：%v", err))
		return
	}

	ioutil.WriteFile("config.yaml", yamlData, 0644)

	ctx.Success(map[string]interface{}{
		"message": "数据库添加成功",
	})
}

// RemoveDatabase 删除数据库
func (c *SettingsController) RemoveDatabase(ctx *mvc.Context) {
	name := ctx.Input("name")
	if name == "" {
		ctx.Error(400, "数据库名称不能为空")
		return
	}

	// 读取现有配置
	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析配置文件失败：%v", err))
		return
	}

	// 删除数据库
	if cfg["databases"] != nil {
		databases := cfg["databases"].(map[string]interface{})
		delete(databases, name)
	}

	// 保存
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("转换配置失败：%v", err))
		return
	}

	ioutil.WriteFile("config.yaml", yamlData, 0644)

	ctx.Success(map[string]interface{}{
		"message": "数据库删除成功",
	})
}

// SaveMultiDatabase 保存多数据库配置（保留注释的版本）
func (c *SettingsController) SaveMultiDatabase(ctx *mvc.Context) {
	// 这个函数暂时不执行任何操作，因为数据库列表已经在内存中
	// 实际的保存操作在添加/删除时已经完成
	// 这里只是返回成功响应
	ctx.Success(map[string]interface{}{
		"message": "多数据库配置已保存",
	})
}

// ListDatabases 获取所有数据库配置
func (c *SettingsController) ListDatabases(ctx *mvc.Context) {
	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析配置文件失败：%v", err))
		return
	}

	databases := make(map[string]interface{})

	// 主数据库
	if cfg["database"] != nil {
		db := cfg["database"].(map[string]interface{})
		databases["default"] = map[string]interface{}{
			"name":   "主数据库",
			"driver": getMapValue(db, "driver", "mysql"),
			"host":   getMapValue(db, "host", "127.0.0.1"),
			"port":   getMapValue(db, "port", 3306),
			"status": "active",
		}
	}

	// 多数据库配置
	if cfg["databases"] != nil {
		multiDBs := cfg["databases"].(map[string]interface{})
		for name, db := range multiDBs {
			dbMap := db.(map[string]interface{})
			databases[name] = map[string]interface{}{
				"name":   name,
				"driver": getMapValue(dbMap, "driver", "mysql"),
				"host":   getMapValue(dbMap, "host", ""),
				"port":   getMapValue(dbMap, "port", 3306),
				"status": "active",
			}
		}
	}

	ctx.Success(map[string]interface{}{
		"databases": databases,
		"count":     len(databases),
	})
}

// SaveRedis 保存 Redis 配置
func (c *SettingsController) SaveRedis(ctx *mvc.Context) {
	type RedisConfig struct {
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Password string `json:"password"`
		DB       int    `json:"db"`
		PoolSize int    `json:"pool_size"`
	}

	var redisCfg RedisConfig
	if err := json.Unmarshal([]byte(ctx.Input("config")), &redisCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	updateConfigValue("redis.host", redisCfg.Host)
	updateConfigValue("redis.port", redisCfg.Port)
	updateConfigValue("redis.password", redisCfg.Password)
	updateConfigValue("redis.db", redisCfg.DB)
	updateConfigValue("redis.pool_size", redisCfg.PoolSize)

	ctx.Success(map[string]interface{}{
		"message": "Redis 配置保存成功",
	})
}

// SaveRabbitMQ 保存 RabbitMQ 配置
func (c *SettingsController) SaveRabbitMQ(ctx *mvc.Context) {
	type MQConfig struct {
		Enabled bool   `json:"enabled"`
		Host    string `json:"host"`
		Port    int    `json:"port"`
		User    string `json:"user"`
		Pass    string `json:"pass"`
		VHost   string `json:"vhost"`
	}

	var mqCfg MQConfig
	if err := json.Unmarshal([]byte(ctx.Input("config")), &mqCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	updateConfigValue("rabbitmq.enabled", mqCfg.Enabled)
	updateConfigValue("rabbitmq.host", mqCfg.Host)
	updateConfigValue("rabbitmq.port", mqCfg.Port)
	updateConfigValue("rabbitmq.user", mqCfg.User)
	updateConfigValue("rabbitmq.pass", mqCfg.Pass)
	updateConfigValue("rabbitmq.vhost", mqCfg.VHost)

	ctx.Success(map[string]interface{}{
		"message": "RabbitMQ 配置保存成功",
	})
}

// SaveNacos 保存 Nacos 配置
func (c *SettingsController) SaveNacos(ctx *mvc.Context) {
	type NacosConfig struct {
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Namespace string `json:"namespace"`
		DataID    string `json:"data_id"`
		Group     string `json:"group"`
	}

	var nacosCfg NacosConfig
	if err := json.Unmarshal([]byte(ctx.Input("config")), &nacosCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	updateConfigValue("nacos.host", nacosCfg.Host)
	updateConfigValue("nacos.port", nacosCfg.Port)
	updateConfigValue("nacos.namespace", nacosCfg.Namespace)
	updateConfigValue("nacos.data_id", nacosCfg.DataID)
	updateConfigValue("nacos.group", nacosCfg.Group)

	ctx.Success(map[string]interface{}{
		"message": "Nacos 配置保存成功",
	})
}

// SaveGRPC 保存 gRPC 配置
func (c *SettingsController) SaveGRPC(ctx *mvc.Context) {
	type GRPCConfig struct {
		Enabled bool `json:"enabled"`
		Port    int  `json:"port"`
	}

	var grpcCfg GRPCConfig
	if err := json.Unmarshal([]byte(ctx.Input("config")), &grpcCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	updateConfigValue("grpc.enabled", grpcCfg.Enabled)
	updateConfigValue("grpc.port", grpcCfg.Port)

	ctx.Success(map[string]interface{}{
		"message": "gRPC 配置保存成功",
	})
}

// SaveSecurity 保存安全配置
func (c *SettingsController) SaveSecurity(ctx *mvc.Context) {
	type SecurityConfig struct {
		EnableSecurityMiddleware bool     `json:"enable_security_middleware"`
		EnableCSRF               bool     `json:"enable_csrf"`
		EnableRateLimit          bool     `json:"enable_rate_limit"`
		RateLimit                int      `json:"rate_limit"`
		EnableCORS               bool     `json:"enable_cors"`
		AllowedDomains           []string `json:"allowed_domains"`
		IPWhitelist              []string `json:"ip_whitelist"`
		IPBlacklist              []string `json:"ip_blacklist"`
	}

	var secCfg SecurityConfig
	if err := json.Unmarshal([]byte(ctx.Input("config")), &secCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	// 保存基础配置
	updateYamlValue("security.enable_security_middleware", secCfg.EnableSecurityMiddleware)
	updateYamlValue("security.enable_csrf_protection", secCfg.EnableCSRF)
	updateYamlValue("security.enable_rate_limit", secCfg.EnableRateLimit)
	updateYamlValue("security.rate_limit", secCfg.RateLimit)
	updateYamlValue("security.enable_cors_domain_check", secCfg.EnableCORS)

	// 保存域名列表和 IP 列表（使用 YAML 数组格式）
	updateYamlArray("security.allowed_domains", secCfg.AllowedDomains)
	updateYamlArray("security.ip_whitelist", secCfg.IPWhitelist)
	updateYamlArray("security.ip_blacklist", secCfg.IPBlacklist)

	ctx.Success(map[string]interface{}{
		"message": "安全配置保存成功",
	})
}

// SavePayment 保存支付配置
func (c *SettingsController) SavePayment(ctx *mvc.Context) {
	paymentType := ctx.Input("type") // alipay 或 wechat
	configData := ctx.Input("config")

	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析配置文件失败：%v", err))
		return
	}

	var payCfg map[string]interface{}
	if err := json.Unmarshal([]byte(configData), &payCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	if paymentType == "alipay" {
		if cfg["payment"] == nil {
			cfg["payment"] = make(map[string]interface{})
		}
		payment := cfg["payment"].(map[string]interface{})
		payment["alipay"] = payCfg
	} else if paymentType == "wechat" {
		if cfg["payment"] == nil {
			cfg["payment"] = make(map[string]interface{})
		}
		payment := cfg["payment"].(map[string]interface{})
		payment["wechat_pay"] = payCfg
	}

	// 使用 updateYamlValue 保存支付配置
	for key, value := range payCfg {
		paymentKey := fmt.Sprintf("payment.%s.%s", paymentType, key)
		updateYamlValue(paymentKey, value)
	}

	ctx.Success(map[string]interface{}{
		"message": "支付配置保存成功",
	})
}

// SaveOAuth 保存第三方登录配置
func (c *SettingsController) SaveOAuth(ctx *mvc.Context) {
	oauthType := ctx.Input("type") // qq, wechat, alipay
	configData := ctx.Input("config")

	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		ctx.Error(500, fmt.Sprintf("读取配置文件失败：%v", err))
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		ctx.Error(500, fmt.Sprintf("解析配置文件失败：%v", err))
		return
	}

	var oauthCfg map[string]interface{}
	if err := json.Unmarshal([]byte(configData), &oauthCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	if cfg["oauth"] == nil {
		cfg["oauth"] = make(map[string]interface{})
	}
	oauth := cfg["oauth"].(map[string]interface{})
	oauth[oauthType] = oauthCfg

	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		ctx.Error(500, fmt.Sprintf("转换配置失败：%v", err))
		return
	}

	ioutil.WriteFile("config.yaml", yamlData, 0644)

	ctx.Success(map[string]interface{}{
		"message": "OAuth 配置保存成功",
	})
}

// SaveBenchmark 保存压测配置
func (c *SettingsController) SaveBenchmark(ctx *mvc.Context) {
	type BenchmarkConfig struct {
		MemLimitPercent int `json:"mem_limit_percent"`
		CPULimitPercent int `json:"cpu_limit_percent"`
	}

	configStr := ctx.Input("config")
	if configStr == "" {
		ctx.Error(400, "配置数据不能为空")
		return
	}

	var benchCfg BenchmarkConfig
	if err := json.Unmarshal([]byte(configStr), &benchCfg); err != nil {
		ctx.Error(400, fmt.Sprintf("解析配置失败：%v", err))
		return
	}

	updateYamlValue("benchmark.mem_limit_percent", benchCfg.MemLimitPercent)
	updateYamlValue("benchmark.cpu_limit_percent", benchCfg.CPULimitPercent)

	ctx.Success(map[string]interface{}{
		"message": "压测配置保存成功",
	})
}

// 辅助函数：更新配置值（保留注释）
func updateYamlValue(key string, value interface{}) {
	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return
	}

	lines := strings.Split(string(fileData), "\n")
	keyParts := strings.Split(key, ".")

	for i, line := range lines {
		// 检查是否包含该 key
		if strings.Contains(line, keyParts[len(keyParts)-1]+":") {
			// 检查缩进层级是否匹配
			indent := len(line) - len(strings.TrimLeft(line, " "))
			expectedIndent := (len(keyParts) - 1) * 2

			if indent == expectedIndent || len(keyParts) == 1 {
				// 替换该行的值
				valueStr := formatYamlValue(value)
				lines[i] = strings.TrimSpace(strings.Split(line, ":")[0]) + ": " + valueStr
				break
			}
		}
	}

	// 写回文件
	ioutil.WriteFile("config.yaml", []byte(strings.Join(lines, "\n")), 0644)
}

// 辅助函数：格式化 YAML 值
func formatYamlValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// 辅助函数：更新 YAML 数组值
func updateYamlArray(key string, values []string) {
	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return
	}

	lines := strings.Split(string(fileData), "\n")
	keyParts := strings.Split(key, ".")
	lastKey := keyParts[len(keyParts)-1]

	// 找到数组开始位置
	startIndex := -1
	indent := len(keyParts) * 2

	for i, line := range lines {
		if strings.Contains(line, lastKey+":") {
			startIndex = i
			break
		}
	}

	if startIndex == -1 {
		return
	}

	// 删除旧的数组行
	endIndex := startIndex + 1
	for endIndex < len(lines) {
		currentIndent := len(lines[endIndex]) - len(strings.TrimLeft(lines[endIndex], " "))
		if currentIndent <= indent && strings.TrimSpace(lines[endIndex]) != "" {
			break
		}
		endIndex++
	}

	// 构建新的数组内容
	newLines := make([]string, 0, startIndex+1+len(values))
	newLines = append(newLines, lines[:startIndex]...)
	newLines = append(newLines, lines[startIndex : startIndex+1][0]) // 保留 key 行

	// 添加数组值
	for _, val := range values {
		if val != "" {
			newLines = append(newLines, fmt.Sprintf("%s- \"%s\"", strings.Repeat(" ", indent+2), val))
		}
	}

	// 添加剩余的行
	newLines = append(newLines, lines[endIndex:]...)

	// 写回文件
	ioutil.WriteFile("config.yaml", []byte(strings.Join(newLines, "\n")), 0644)
}

// 辅助函数：更新配置值
func updateConfigValue(key string, value interface{}) {
	fileData, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(fileData, &cfg); err != nil {
		return
	}

	keys := splitKey(key)
	setNestedValue(cfg, keys, value)

	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return
	}

	os.WriteFile("config.yaml", yamlData, 0644)
}

// 辅助函数：分割键名
func splitKey(key string) []string {
	return strings.Split(key, ".")
}

// 辅助函数：设置嵌套值
func setNestedValue(cfg map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		cfg[keys[0]] = value
		return
	}

	if cfg[keys[0]] == nil {
		cfg[keys[0]] = make(map[string]interface{})
	}

	nextMap, ok := cfg[keys[0]].(map[string]interface{})
	if !ok {
		return
	}

	setNestedValue(nextMap, keys[1:], value)
}

// 辅助函数：合并配置
func mergeConfig(existing, new map[string]interface{}) {
	for k, v := range new {
		if existingMap, ok := existing[k].(map[string]interface{}); ok {
			if newMap, ok := v.(map[string]interface{}); ok {
				mergeConfig(existingMap, newMap)
			} else {
				existing[k] = v
			}
		} else {
			existing[k] = v
		}
	}
}

// 辅助函数：获取 map 值
func getMapValue(m map[string]interface{}, key string, defaultVal interface{}) interface{} {
	if v, ok := m[key]; ok {
		return v
	}
	return defaultVal
}
