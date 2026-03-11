package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ApolloConfig Apollo 配置中心客户端
type ApolloConfig struct {
	configService string
	adminService  string
	appID         string
	cluster       string
	namespace     string
	token         string
	client        *http.Client
	cache         map[string]interface{}
	cacheMu       sync.RWMutex
	listeners     map[string][]func(string, interface{})
	listenersMu   sync.RWMutex
}

// ApolloConfigOptions Apollo 配置选项
type ApolloConfigOptions struct {
	ConfigService string        `yaml:"config_service"`
	AdminService  string        `yaml:"admin_service"`
	AppID         string        `yaml:"app_id"`
	Cluster       string        `yaml:"cluster"`
	Namespace     string        `yaml:"namespace"`
	Token         string        `yaml:"token"`
	Timeout       time.Duration `yaml:"timeout"`
}

// NewApolloConfig 创建 Apollo 配置中心客户端
func NewApolloConfig(opts *ApolloConfigOptions) (*ApolloConfig, error) {
	if opts == nil {
		opts = &ApolloConfigOptions{}
	}

	if opts.ConfigService == "" {
		opts.ConfigService = "http://localhost:8080"
	}

	if opts.AdminService == "" {
		opts.AdminService = "http://localhost:8080"
	}

	if opts.AppID == "" {
		return nil, fmt.Errorf("appID is required")
	}

	if opts.Cluster == "" {
		opts.Cluster = "default"
	}

	if opts.Namespace == "" {
		opts.Namespace = "application"
	}

	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	return &ApolloConfig{
		configService: opts.ConfigService,
		adminService:  opts.AdminService,
		appID:         opts.AppID,
		cluster:       opts.Cluster,
		namespace:     opts.Namespace,
		token:         opts.Token,
		client: &http.Client{
			Timeout: opts.Timeout,
		},
		cache:     make(map[string]interface{}),
		listeners: make(map[string][]func(string, interface{})),
	}, nil
}

// Load 加载配置
func (c *ApolloConfig) Load(ctx context.Context) error {
	// 从 Apollo 获取配置
	url := fmt.Sprintf("%s/configs/%s/%s/%s",
		c.configService,
		c.appID,
		c.cluster,
		c.namespace,
	)

	// 添加 releaseKey 参数（如果有）
	releaseKey := ""
	if releaseKey != "" {
		url += fmt.Sprintf("?releaseKey=%s", releaseKey)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to load config: %s, body: %s", resp.Status, string(body))
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 解析响应
	var result struct {
		ApplicationID  string            `json:"appId"`
		Cluster        string            `json:"cluster"`
		NamespaceName  string            `json:"namespaceName"`
		Configurations map[string]string `json:"configurations"`
		ReleaseKey     string            `json:"releaseKey"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return err
	}

	// 更新缓存
	c.cacheMu.Lock()
	for k, v := range result.Configurations {
		// 尝试解析为 JSON
		var jsonValue interface{}
		if err := json.Unmarshal([]byte(v), &jsonValue); err == nil {
			c.cache[k] = jsonValue
		} else {
			c.cache[k] = v
		}
	}
	c.cacheMu.Unlock()

	return nil
}

// Watch 监听配置变更
func (c *ApolloConfig) Watch(ctx context.Context) error {
	// 使用长轮询监听配置变更
	notificationURL := fmt.Sprintf("%s/notifications/v2", c.configService)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 构建通知请求
			notifications := []map[string]interface{}{
				{
					"NamespaceName":  c.namespace,
					"NotificationId": 0, // 初始为 0
				},
			}

			data, _ := json.Marshal(notifications)

			req, err := http.NewRequestWithContext(ctx, "POST", notificationURL, strings.NewReader(string(data)))
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			req.Header.Set("Content-Type", "application/json")
			if c.token != "" {
				req.Header.Set("Authorization", "Bearer "+c.token)
			}

			resp, err := c.client.Do(req)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			_, err = ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			if resp.StatusCode == http.StatusNotModified {
				// 没有变更，继续轮询
				continue
			}

			// 配置有变更，重新加载
			if err := c.Load(ctx); err != nil {
				continue
			}

			// 触发监听器
			c.listenersMu.RLock()
			for key, listeners := range c.listeners {
				value := c.Get(key)
				for _, listener := range listeners {
					listener(key, value)
				}
			}
			c.listenersMu.RUnlock()
		}
	}()

	return nil
}

// Get 获取配置值
func (c *ApolloConfig) Get(key string) interface{} {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	return c.cache[key]
}

// GetString 获取字符串配置值
func (c *ApolloConfig) GetString(key string) string {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	if v, ok := c.cache[key].(string); ok {
		return v
	}
	return ""
}

// GetInt 获取整数配置值
func (c *ApolloConfig) GetInt(key string) int {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	if v, ok := c.cache[key].(float64); ok {
		return int(v)
	}
	return 0
}

// GetBool 获取布尔配置值
func (c *ApolloConfig) GetBool(key string) bool {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	if v, ok := c.cache[key].(bool); ok {
		return v
	}
	if v, ok := c.cache[key].(string); ok {
		return v == "true" || v == "TRUE" || v == "1"
	}
	return false
}

// GetFloat 获取浮点数配置值
func (c *ApolloConfig) GetFloat(key string) float64 {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()
	if v, ok := c.cache[key].(float64); ok {
		return v
	}
	return 0.0
}

// GetSection 获取整个分区的配置
func (c *ApolloConfig) GetSection(section string) map[string]interface{} {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	result := make(map[string]interface{})
	prefix := section + "."
	for k, v := range c.cache {
		if strings.HasPrefix(k, prefix) {
			result[k[len(prefix):]] = v
		}
	}
	return result
}

// Listen 监听配置变更
func (c *ApolloConfig) Listen(key string, listener func(string, interface{})) {
	c.listenersMu.Lock()
	defer c.listenersMu.Unlock()
	c.listeners[key] = append(c.listeners[key], listener)
}

// Unmarshal 将配置解析到结构体
func (c *ApolloConfig) Unmarshal(key string, obj interface{}) error {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	if v, ok := c.cache[key]; ok {
		data, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, obj)
	}

	return fmt.Errorf("key not found: %s", key)
}

// GetAllKeys 获取所有配置键
func (c *ApolloConfig) GetAllKeys() []string {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	keys := make([]string, 0, len(c.cache))
	for k := range c.cache {
		keys = append(keys, k)
	}
	return keys
}

// Close 关闭连接
func (c *ApolloConfig) Close() error {
	c.cacheMu.Lock()
	c.cache = make(map[string]interface{})
	c.cacheMu.Unlock()

	c.listenersMu.Lock()
	c.listeners = make(map[string][]func(string, interface{}))
	c.listenersMu.Unlock()

	return nil
}

// ApolloConfigInterface 配置接口
type ApolloConfigInterface interface {
	// Load 加载配置
	Load(ctx context.Context) error
	// Watch 监听配置变更
	Watch(ctx context.Context) error
	// Get 获取配置值
	Get(key string) interface{}
	// GetString 获取字符串配置值
	GetString(key string) string
	// GetInt 获取整数配置值
	GetInt(key string) int
	// GetBool 获取布尔配置值
	GetBool(key string) bool
	// GetFloat 获取浮点数配置值
	GetFloat(key string) float64
	// GetSection 获取整个分区的配置
	GetSection(section string) map[string]interface{}
	// Listen 监听配置变更
	Listen(key string, listener func(string, interface{}))
	// Unmarshal 将配置解析到结构体
	Unmarshal(key string, obj interface{}) error
	// GetAllKeys 获取所有配置键
	GetAllKeys() []string
	// Close 关闭连接
	Close() error
}

// ConfigFactory 配置工厂函数
type ConfigFactory func(interface{}) (ApolloConfigInterface, error)

// 注册配置工厂
var configFactories = make(map[string]ConfigFactory)

// RegisterConfigFactory 注册配置工厂
func RegisterConfigFactory(name string, factory ConfigFactory) {
	configFactories[name] = factory
}

// CreateConfig 创建配置客户端
func CreateConfig(name string, config interface{}) (ApolloConfigInterface, error) {
	factory, ok := configFactories[name]
	if !ok {
		return nil, fmt.Errorf("unknown config type: %s", name)
	}
	return factory(config)
}

// 初始化时注册所有可用的配置中心
func init() {
	// 注册 Apollo
	RegisterConfigFactory("apollo", func(cfg interface{}) (ApolloConfigInterface, error) {
		apolloCfg, ok := cfg.(*ApolloConfigOptions)
		if !ok {
			return nil, fmt.Errorf("invalid config type")
		}
		return NewApolloConfig(apolloCfg)
	})

	// 注册 Nacos
	RegisterConfigFactory("nacos", func(cfg interface{}) (ApolloConfigInterface, error) {
		nacosCfg, ok := cfg.(*NacosConfigOptions)
		if !ok {
			return nil, fmt.Errorf("invalid config type")
		}
		return NewNacosConfig(nacosCfg)
	})
}
