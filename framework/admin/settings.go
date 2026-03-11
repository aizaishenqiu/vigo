package admin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"vigo/config"
	"vigo/framework/mvc"

	"gopkg.in/yaml.v3"
)

// getSettings 获取所有配置
func getSettings(c *mvc.Context) {
	// 尝试读取配置文件
	configData, err := os.ReadFile("config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			c.Json(200, map[string]interface{}{
				"code": 0,
				"msg":  "success",
				"data": map[string]interface{}{},
			})
			return
		}
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  fmt.Sprintf("读取配置文件失败：%v", err),
		})
		return
	}

	var cfg map[string]interface{}
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		c.Json(500, map[string]interface{}{
			"code": 500,
			"msg":  fmt.Sprintf("解析配置文件失败：%v", err),
		})
		return
	}

	c.Json(200, map[string]interface{}{
		"code": 0,
		"msg":  "success",
		"data": cfg,
	})
}

// saveAppSettings 保存应用配置
func saveAppSettings(c *mvc.Context) {
	type AppConfig struct {
		Name         string `json:"name"`
		Port         int    `json:"port"`
		Debug        bool   `json:"debug"`
		Mode         string `json:"mode"`
		Console      bool   `json:"console"`
		AutoKillPort bool   `json:"auto_kill_port"`
		DebugToolbar bool   `json:"debug_toolbar"`
	}

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "读取请求失败"})
		return
	}

	var req struct {
		Config AppConfig `json:"config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "解析配置失败"})
		return
	}

	updateYamlValue("app.name", req.Config.Name)
	updateYamlValue("app.port", req.Config.Port)
	updateYamlValue("app.debug", req.Config.Debug)
	updateYamlValue("app.mode", req.Config.Mode)
	updateYamlValue("app.console", req.Config.Console)
	updateYamlValue("app.auto_kill_port", req.Config.AutoKillPort)
	updateYamlValue("app.debug_toolbar", req.Config.DebugToolbar)

	c.Json(200, map[string]interface{}{"code": 0, "msg": "应用配置保存成功"})
}

// saveDatabaseSettings 保存数据库配置
func saveDatabaseSettings(c *mvc.Context) {
	type DatabaseConfig struct {
		Driver          string        `json:"driver"`
		Host            string        `json:"host"`
		Port            int           `json:"port"`
		Name            string        `json:"name"`
		User            string        `json:"user"`
		Password        string        `json:"password"`
		Charset         string        `json:"charset"`
		MaxOpenConns    int           `json:"max_open_conns"`
		MaxIdleConns    int           `json:"max_idle_conns"`
		ConnMaxLifetime int           `json:"conn_max_lifetime"`
		ConnMaxIdleTime int           `json:"conn_max_idletime"`
		RWSplit         bool          `json:"rw_split"`
		Writes          []interface{} `json:"writes"` // Simplified for now
		Reads           []interface{} `json:"reads"`
	}

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "读取请求失败"})
		return
	}

	var req struct {
		Config DatabaseConfig `json:"config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "解析配置失败"})
		return
	}

	updateYamlValue("database.driver", req.Config.Driver)
	updateYamlValue("database.host", req.Config.Host)
	updateYamlValue("database.port", req.Config.Port)
	updateYamlValue("database.name", req.Config.Name)
	updateYamlValue("database.user", req.Config.User)
	updateYamlValue("database.pass", req.Config.Password)
	updateYamlValue("database.charset", req.Config.Charset)
	updateYamlValue("database.max_open_conns", req.Config.MaxOpenConns)
	updateYamlValue("database.max_idle_conns", req.Config.MaxIdleConns)
	updateYamlValue("database.conn_max_lifetime", req.Config.ConnMaxLifetime)
	updateYamlValue("database.conn_max_idletime", req.Config.ConnMaxIdleTime)
	updateYamlValue("database.rw_split", req.Config.RWSplit)

	c.Json(200, map[string]interface{}{"code": 0, "msg": "数据库配置保存成功"})
}

// saveRedisSettings 保存 Redis 配置
func saveRedisSettings(c *mvc.Context) {
	// 简单实现，直接更新
	updateConfigSection(c, "redis")
}

// saveMQSettings 保存 RabbitMQ 配置
func saveMQSettings(c *mvc.Context) {
	updateConfigSection(c, "rabbitmq")
}

// saveNacosSettings 保存 Nacos 配置
func saveNacosSettings(c *mvc.Context) {
	updateConfigSection(c, "nacos")
}

// saveGRPCSettings 保存 gRPC 配置
func saveGRPCSettings(c *mvc.Context) {
	updateConfigSection(c, "grpc")
}

// saveSecuritySettings 保存安全配置
func saveSecuritySettings(c *mvc.Context) {
	updateConfigSection(c, "security")
}

// savePaymentSettings 保存支付配置
func savePaymentSettings(c *mvc.Context) {
	body, _ := ioutil.ReadAll(c.Request.Body)
	var req struct {
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}
	json.Unmarshal(body, &req)

	if req.Type == "" {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "缺少支付类型"})
		return
	}

	for k, v := range req.Config {
		if err := updateYamlValueByPath(fmt.Sprintf("payment.%s.%s", req.Type, k), v); err != nil {
			c.Json(500, map[string]interface{}{"code": 500, "msg": "支付配置保存失败：" + err.Error()})
			return
		}
	}
	config.Init()
	_ = InitConfigManager()

	c.Json(200, map[string]interface{}{"code": 0, "msg": "支付配置保存成功"})
}

// saveOAuthSettings 保存 OAuth 配置
func saveOAuthSettings(c *mvc.Context) {
	body, _ := ioutil.ReadAll(c.Request.Body)
	var req struct {
		Type   string                 `json:"type"`
		Config map[string]interface{} `json:"config"`
	}
	json.Unmarshal(body, &req)

	if req.Type == "" {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "缺少 OAuth 类型"})
		return
	}

	for k, v := range req.Config {
		if err := updateYamlValueByPath(fmt.Sprintf("oauth.%s.%s", req.Type, k), v); err != nil {
			c.Json(500, map[string]interface{}{"code": 500, "msg": "OAuth 配置保存失败：" + err.Error()})
			return
		}
	}
	config.Init()
	_ = InitConfigManager()

	c.Json(200, map[string]interface{}{"code": 0, "msg": "OAuth 配置保存成功"})
}

// saveBenchmarkSettings 保存压测配置
func saveBenchmarkSettings(c *mvc.Context) {
	updateConfigSection(c, "benchmark")
}

// updateConfigSection 通用配置更新
func updateConfigSection(c *mvc.Context, section string) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "读取请求失败"})
		return
	}

	var req struct {
		Config map[string]interface{} `json:"config"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		c.Json(400, map[string]interface{}{"code": 400, "msg": "解析请求数据失败"})
		return
	}

	// 读取现有配置
	configData, _ := ioutil.ReadFile("config.yaml")
	var existingCfg map[string]interface{}
	if len(configData) > 0 {
		yaml.Unmarshal(configData, &existingCfg)
	} else {
		existingCfg = make(map[string]interface{})
	}
	if existingCfg == nil {
		existingCfg = make(map[string]interface{})
	}

	currentSection, _ := existingCfg[section].(map[string]interface{})
	if currentSection == nil {
		currentSection = make(map[string]interface{})
	}
	existingCfg[section] = deepMergeMap(currentSection, req.Config)

	yamlData, err := yaml.Marshal(existingCfg)
	if err != nil {
		c.Json(500, map[string]interface{}{"code": 500, "msg": "转换配置失败"})
		return
	}

	ioutil.WriteFile("config.yaml", yamlData, 0644)

	c.Json(200, map[string]interface{}{"code": 0, "msg": "保存成功"})
}

func deepMergeMap(dst map[string]interface{}, src map[string]interface{}) map[string]interface{} {
	if dst == nil {
		dst = make(map[string]interface{})
	}
	for k, v := range src {
		if vMap, ok := v.(map[string]interface{}); ok {
			dMap, _ := dst[k].(map[string]interface{})
			dst[k] = deepMergeMap(dMap, vMap)
		} else {
			dst[k] = v
		}
	}
	return dst
}
