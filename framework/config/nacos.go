package config

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"gopkg.in/yaml.v3"
)

// NacosConfigOptions Nacos 配置选项
type NacosConfigOptions struct {
	// Nacos 服务器地址
	ServerAddr string `yaml:"server_addr"`
	// 用户名
	Username string `yaml:"username"`
	// 密码
	Password string `yaml:"password"`
	// 命名空间 ID
	NamespaceId string `yaml:"namespace_id"`
	// 配置分组
	Group string `yaml:"group"`
	// 配置 DataID
	DataId string `yaml:"data_id"`
	// 配置格式（json, yaml, properties）
	ConfigFormat string `yaml:"config_format"`
	// 超时时间（毫秒）
	TimeoutMs uint64 `yaml:"timeout_ms"`
	// 日志级别
	LogLevel string `yaml:"log_level"`
	// 日志路径
	LogDir string `yaml:"log_dir"`
	// 缓存目录
	CacheDir string `yaml:"cache_dir"`
}

// NacosConfig Nacos 配置客户端
type NacosConfig struct {
	client     config_client.IConfigClient
	opts       *NacosConfigOptions
	cache      map[string]interface{}
	listeners  map[string][]func(string, interface{})
	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	ctx        context.Context
}

// NewNacosConfig 创建 Nacos 配置客户端
func NewNacosConfig(opts *NacosConfigOptions) (*NacosConfig, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if opts.ServerAddr == "" {
		opts.ServerAddr = "127.0.0.1:8848"
	}

	if opts.Group == "" {
		opts.Group = "DEFAULT_GROUP"
	}

	if opts.DataId == "" {
		opts.DataId = "config.yaml"
	}

	if opts.ConfigFormat == "" {
		opts.ConfigFormat = "yaml"
	}

	if opts.TimeoutMs == 0 {
		opts.TimeoutMs = 10000
	}

	if opts.LogLevel == "" {
		opts.LogLevel = "info"
	}

	// 创建服务器配置
	serverConfigs := []constant.ServerConfig{
		*constant.NewServerConfig(opts.ServerAddr, 0),
	}

	// 创建客户端配置
	clientConfig := constant.NewClientConfig(
		constant.WithTimeoutMs(opts.TimeoutMs),
		constant.WithNamespaceId(opts.NamespaceId),
		constant.WithLogDir(opts.LogDir),
		constant.WithCacheDir(opts.CacheDir),
		constant.WithLogLevel(opts.LogLevel),
		constant.WithUsername(opts.Username),
		constant.WithPassword(opts.Password),
	)

	// 创建配置客户端
	client, err := clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig:  clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos config client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &NacosConfig{
		client:     client,
		opts:       opts,
		cache:      make(map[string]interface{}),
		listeners:  make(map[string][]func(string, interface{})),
		cancelFunc: cancel,
		ctx:        ctx,
	}, nil
}

// Load 加载配置
func (c *NacosConfig) Load(ctx context.Context) error {
	content, err := c.client.GetConfig(vo.ConfigParam{
		DataId: c.opts.DataId,
		Group:  c.opts.Group,
	})
	if err != nil {
		return fmt.Errorf("failed to get config from nacos: %v", err)
	}

	// 解析配置
	data, err := parseConfigContent(content, c.opts.ConfigFormat)
	if err != nil {
		return fmt.Errorf("failed to parse config: %v", err)
	}

	c.mu.Lock()
	c.cache = data
	c.mu.Unlock()

	return nil
}

// Watch 监听配置变更
func (c *NacosConfig) Watch(ctx context.Context) error {
	err := c.client.ListenConfig(vo.ConfigParam{
		DataId: c.opts.DataId,
		Group:  c.opts.Group,
		OnChange: func(namespace, group, dataId, data string) {
			// 解析新配置
			newData, err := parseConfigContent(data, c.opts.ConfigFormat)
			if err != nil {
				return
			}

			// 更新缓存
			c.mu.Lock()
			oldCache := c.cache
			c.cache = newData
			c.mu.Unlock()

			// 通知监听器
			c.notifyListeners(oldCache, newData)
		},
	})

	if err != nil {
		return fmt.Errorf("failed to listen config: %v", err)
	}

	return nil
}

// Get 获取配置值
func (c *NacosConfig) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		return val
	}
	return nil
}

// GetString 获取字符串配置值
func (c *NacosConfig) GetString(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// GetInt 获取整数配置值
func (c *NacosConfig) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case int32:
			return int(v)
		case int64:
			return int(v)
		case float32:
			return int(v)
		case float64:
			return int(v)
		}
	}
	return 0
}

// GetBool 获取布尔配置值
func (c *NacosConfig) GetBool(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// GetFloat 获取浮点数配置值
func (c *NacosConfig) GetFloat(key string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		switch v := val.(type) {
		case float32:
			return float64(v)
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0
}

// GetSection 获取整个分区的配置
func (c *NacosConfig) GetSection(section string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	for key, val := range c.cache {
		if len(section) > 0 && len(key) > len(section) && key[:len(section)] == section {
			result[key] = val
		}
	}
	return result
}

// Listen 监听配置变更
func (c *NacosConfig) Listen(key string, listener func(string, interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.listeners[key] = append(c.listeners[key], listener)
}

// Unmarshal 将配置解析到结构体
func (c *NacosConfig) Unmarshal(key string, obj interface{}) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if val, ok := c.cache[key]; ok {
		return convertToStruct(val, obj)
	}
	return fmt.Errorf("key not found: %s", key)
}

// GetAllKeys 获取所有配置键
func (c *NacosConfig) GetAllKeys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, len(c.cache))
	for key := range c.cache {
		keys = append(keys, key)
	}
	return keys
}

// Close 关闭连接
func (c *NacosConfig) Close() error {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	return nil
}

// notifyListeners 通知监听器
func (c *NacosConfig) notifyListeners(oldCache, newCache map[string]interface{}) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 找出变化的键
	changedKeys := make(map[string]interface{})
	for key, newVal := range newCache {
		oldVal, exists := oldCache[key]
		if !exists || oldVal != newVal {
			changedKeys[key] = newVal
		}
	}

	// 通知监听器
	for key, val := range changedKeys {
		if listeners, ok := c.listeners[key]; ok {
			for _, listener := range listeners {
				go listener(key, val)
			}
		}
	}
}

// parseConfigContent 解析配置内容
func parseConfigContent(content, format string) (map[string]interface{}, error) {
	var data map[string]interface{}
	var err error

	switch format {
	case "json":
		err = json.Unmarshal([]byte(content), &data)
	case "yaml", "yml":
		err = yaml.Unmarshal([]byte(content), &data)
	case "properties":
		data, err = parseProperties(content)
	default:
		err = yaml.Unmarshal([]byte(content), &data)
	}

	if err != nil {
		return nil, err
	}
	return data, nil
}

// parseProperties 解析 properties 格式配置
func parseProperties(content string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return result, nil
}

// convertToStruct 转换为结构体
func convertToStruct(data interface{}, obj interface{}) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}

	// 尝试使用 mapstructure 或类似库进行转换
	// 这里使用 json  marshal/unmarshal 作为简单实现
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %v", err)
	}

	err = json.Unmarshal(jsonData, obj)
	if err != nil {
		return fmt.Errorf("failed to unmarshal to struct: %v", err)
	}

	return nil
}
