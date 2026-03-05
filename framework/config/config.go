package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config 配置中心客户端
type Config struct {
	mu         sync.RWMutex
	serverAddr string
	namespace  string
	dataID     string
	group      string
	cache      map[string]interface{}
	listeners  map[string][]func(string)
	client     *http.Client
}

// ConfigOption 配置选项
type ConfigOption func(*Config)

// WithServerAddr 设置 Nacos 服务器地址
func WithServerAddr(addr string) ConfigOption {
	return func(c *Config) {
		c.serverAddr = addr
	}
}

// WithNamespace 设置命名空间
func WithNamespace(ns string) ConfigOption {
	return func(c *Config) {
		c.namespace = ns
	}
}

// WithDataID 设置数据 ID
func WithDataID(dataID string) ConfigOption {
	return func(c *Config) {
		c.dataID = dataID
	}
}

// WithGroup 设置分组
func WithGroup(group string) ConfigOption {
	return func(c *Config) {
		c.group = group
	}
}

// NewConfig 创建配置中心客户端
func NewConfig(opts ...ConfigOption) (*Config, error) {
	c := &Config{
		cache:     make(map[string]interface{}),
		listeners: make(map[string][]func(string)),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.serverAddr == "" {
		c.serverAddr = "http://localhost:8848"
	}

	if c.dataID == "" {
		c.dataID = "vigo.yaml"
	}

	if c.group == "" {
		c.group = "DEFAULT_GROUP"
	}

	return c, nil
}

// Get 获取配置值
func (c *Config) Get(key string) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[key]
}

// GetString 获取字符串配置
func (c *Config) GetString(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.cache[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt 获取整数配置
func (c *Config) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.cache[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			var i int
			fmt.Sscanf(val, "%d", &i)
			return i
		}
	}
	return 0
}

// GetBool 获取布尔配置
func (c *Config) GetBool(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.cache[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			return strings.ToLower(val) == "true"
		}
	}
	return false
}

// GetFloat 获取浮点数配置
func (c *Config) GetFloat(key string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if v, ok := c.cache[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		case string:
			var f float64
			fmt.Sscanf(val, "%f", &f)
			return f
		}
	}
	return 0
}

// GetAll 获取所有配置
func (c *Config) GetAll() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range c.cache {
		result[k] = v
	}
	return result
}

// Load 加载配置
func (c *Config) Load(ctx context.Context) error {
	// 从 Nacos 获取配置
	url := fmt.Sprintf("%s/nacos/v1/cs/configs", c.serverAddr)
	params := fmt.Sprintf("?dataId=%s&group=%s&tenant=%s",
		c.dataID, c.group, c.namespace)

	resp, err := c.client.Get(url + params)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 解析配置（简化版，假设是 JSON 格式）
	var config map[string]interface{}
	if err := json.Unmarshal(body, &config); err != nil {
		// 如果不是 JSON，尝试 YAML
		return c.parseYAML(body)
	}

	c.mu.Lock()
	for k, v := range config {
		c.cache[k] = v
	}
	c.mu.Unlock()

	return nil
}

// parseYAML 解析 YAML 配置（简化实现）
func (c *Config) parseYAML(data []byte) error {
	// 这里应该使用 yaml.v3 库解析
	// 简化实现：按行解析 key=value 格式
	lines := strings.Split(string(data), "\n")

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			c.cache[key] = value
		}
	}

	return nil
}

// Watch 监听配置变化
func (c *Config) Watch(key string, callback func(string)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.listeners[key] == nil {
		c.listeners[key] = make([]func(string), 0)
	}
	c.listeners[key] = append(c.listeners[key], callback)
}

// StartLongPolling 启动长轮询监听配置变化
func (c *Config) StartLongPolling(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 检查配置是否变化
				url := fmt.Sprintf("%s/nacos/v1/cs/configs", c.serverAddr)
				params := fmt.Sprintf("?dataId=%s&group=%s&tenant=%s&listen=true",
					c.dataID, c.group, c.namespace)

				resp, err := c.client.Post(url+params, "application/json", nil)
				if err != nil {
					continue
				}

				if resp.StatusCode == http.StatusOK {
					// 配置已变化，重新加载
					if err := c.Load(ctx); err != nil {
						continue
					}

					// 触发回调
					c.mu.RLock()
					for key, callbacks := range c.listeners {
						if v, ok := c.cache[key]; ok {
							for _, cb := range callbacks {
								go cb(fmt.Sprintf("%v", v))
							}
						}
					}
					c.mu.RUnlock()
				}

				resp.Body.Close()
			}
		}
	}()
}

// Publish 发布配置
func (c *Config) Publish(ctx context.Context, key string, value interface{}) error {
	url := fmt.Sprintf("%s/nacos/v1/cs/configs", c.serverAddr)

	data := fmt.Sprintf("dataId=%s&group=%s&tenant=%s&content=%v",
		c.dataID, c.group, c.namespace, value)

	resp, err := c.client.Post(url, "application/x-www-form-urlencoded",
		strings.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to publish config")
	}

	// 更新本地缓存
	c.mu.Lock()
	c.cache[key] = value
	c.mu.Unlock()

	return nil
}

// Delete 删除配置
func (c *Config) Delete(ctx context.Context, key string) error {
	url := fmt.Sprintf("%s/nacos/v1/cs/configs", c.serverAddr)
	params := fmt.Sprintf("?dataId=%s&group=%s&tenant=%s",
		c.dataID, c.group, c.namespace)

	req, _ := http.NewRequest("DELETE", url+params, nil)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to delete config")
	}

	// 删除本地缓存
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()

	return nil
}

// Refresh 刷新配置
func (c *Config) Refresh(ctx context.Context) error {
	return c.Load(ctx)
}

// Close 关闭配置中心客户端
func (c *Config) Close() error {
	return nil
}
