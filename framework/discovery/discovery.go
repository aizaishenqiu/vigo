package discovery

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

// Instance 服务实例
type Instance struct {
	ID          string            `json:"id"`
	IP          string            `json:"ip"`
	Port        int               `json:"port"`
	Weight      float64           `json:"weight"`
	Enabled     bool              `json:"enabled"`
	Healthy     bool              `json:"healthy"`
	ClusterName string            `json:"clusterName"`
	ServiceName string            `json:"serviceName"`
	Metadata    map[string]string `json:"metadata"`
}

// GetAddress 获取服务地址
func (i *Instance) GetAddress() string {
	return fmt.Sprintf("%s:%d", i.IP, i.Port)
}

// Service 服务信息
type Service struct {
	Name        string      `json:"name"`
	Instances   []*Instance `json:"hosts"`
	CacheMillis int64       `json:"cacheMillis"`
}

// Discovery 服务发现客户端
type Discovery struct {
	mu          sync.RWMutex
	serverAddr  string
	namespace   string
	group       string
	client      *http.Client
	cache       map[string]*Service
	lastUpdated map[string]time.Time
	listeners   map[string][]func([]*Instance)
}

// DiscoveryOption 配置选项
type DiscoveryOption func(*Discovery)

// WithServerAddr 设置 Nacos 服务器地址
func WithServerAddr(addr string) DiscoveryOption {
	return func(d *Discovery) {
		d.serverAddr = addr
	}
}

// WithNamespace 设置命名空间
func WithNamespace(ns string) DiscoveryOption {
	return func(d *Discovery) {
		d.namespace = ns
	}
}

// WithGroup 设置分组
func WithGroup(group string) DiscoveryOption {
	return func(d *Discovery) {
		d.group = group
	}
}

// NewDiscovery 创建服务发现客户端
func NewDiscovery(opts ...DiscoveryOption) (*Discovery, error) {
	d := &Discovery{
		cache:       make(map[string]*Service),
		lastUpdated: make(map[string]time.Time),
		listeners:   make(map[string][]func([]*Instance)),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.serverAddr == "" {
		d.serverAddr = "http://localhost:8848"
	}

	if d.group == "" {
		d.group = "DEFAULT_GROUP"
	}

	return d, nil
}

// Register 注册服务实例
func (d *Discovery) Register(ctx context.Context, serviceName string, instance *Instance) error {
	url := fmt.Sprintf("%s/nacos/v1/ns/instance", d.serverAddr)

	data := fmt.Sprintf("serviceName=%s&groupName=%s&ip=%s&port=%d&weight=%.2f&enabled=%t&healthy=%t",
		serviceName, d.group, instance.IP, instance.Port, instance.Weight,
		instance.Enabled, instance.Healthy)

	// 添加元数据
	if len(instance.Metadata) > 0 {
		metadata, _ := json.Marshal(instance.Metadata)
		data += fmt.Sprintf("&metadata=%s", string(metadata))
	}

	resp, err := d.client.Post(url+"?namespaceId="+d.namespace,
		"application/x-www-form-urlencoded",
		strings.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register instance: %s", resp.Status)
	}

	return nil
}

// Deregister 注销服务实例
func (d *Discovery) Deregister(ctx context.Context, serviceName string, instance *Instance) error {
	url := fmt.Sprintf("%s/nacos/v1/ns/instance", d.serverAddr)

	data := fmt.Sprintf("serviceName=%s&groupName=%s&ip=%s&port=%d",
		serviceName, d.group, instance.IP, instance.Port)

	req, _ := http.NewRequest("DELETE", url+"?namespaceId="+d.namespace,
		strings.NewReader(data))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to deregister instance: %s", resp.Status)
	}

	return nil
}

// GetInstances 获取服务实例列表
func (d *Discovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	// 检查缓存
	d.mu.RLock()
	if service, ok := d.cache[serviceName]; ok {
		// 检查缓存是否过期（5 秒）
		if time.Since(d.lastUpdated[serviceName]) < 5*time.Second {
			d.mu.RUnlock()
			return service.Instances, nil
		}
	}
	d.mu.RUnlock()

	// 从 Nacos 获取
	url := fmt.Sprintf("%s/nacos/v1/ns/instance/list", d.serverAddr)
	params := fmt.Sprintf("?serviceName=%s&groupName=%s&namespaceId=%s",
		serviceName, d.group, d.namespace)

	resp, err := d.client.Get(url + params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var service Service
	if err := json.Unmarshal(body, &service); err != nil {
		return nil, err
	}

	// 更新缓存
	d.mu.Lock()
	d.cache[serviceName] = &service
	d.lastUpdated[serviceName] = time.Now()
	d.mu.Unlock()

	return service.Instances, nil
}

// GetHealthyInstances 获取健康的服务实例
func (d *Discovery) GetHealthyInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	instances, err := d.GetInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	healthy := make([]*Instance, 0)
	for _, inst := range instances {
		if inst.Healthy && inst.Enabled {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

// SelectOneInstance 选择一个服务实例（用于负载均衡）
func (d *Discovery) SelectOneInstance(ctx context.Context, serviceName string) (*Instance, error) {
	instances, err := d.GetHealthyInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instance for service: %s", serviceName)
	}

	// 简单轮询选择
	return instances[0], nil
}

// Subscribe 订阅服务变化
func (d *Discovery) Subscribe(serviceName string, callback func([]*Instance)) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.listeners[serviceName] == nil {
		d.listeners[serviceName] = make([]func([]*Instance), 0)
	}
	d.listeners[serviceName] = append(d.listeners[serviceName], callback)

	return nil
}

// Unsubscribe 取消订阅
func (d *Discovery) Unsubscribe(serviceName string, callback func([]*Instance)) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	listeners := d.listeners[serviceName]
	newListeners := make([]func([]*Instance), 0)
	for _, cb := range listeners {
		// 通过函数指针比较（简化实现，实际应该使用回调 ID）
		if fmt.Sprintf("%p", cb) != fmt.Sprintf("%p", callback) {
			newListeners = append(newListeners, cb)
		}
	}
	d.listeners[serviceName] = newListeners

	return nil
}

// StartWatching 启动服务监听
func (d *Discovery) StartWatching(ctx context.Context, serviceName string) error {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		var lastInstances []*Instance

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				instances, err := d.GetHealthyInstances(ctx, serviceName)
				if err != nil {
					continue
				}

				// 检查是否变化
				if len(instances) != len(lastInstances) {
					d.notifyListeners(serviceName, instances)
					lastInstances = instances
				}
			}
		}
	}()

	return nil
}

func (d *Discovery) notifyListeners(serviceName string, instances []*Instance) {
	d.mu.RLock()
	callbacks := d.listeners[serviceName]
	d.mu.RUnlock()

	for _, cb := range callbacks {
		go cb(instances)
	}
}

// WatchEvent 服务发现事件
type WatchEvent struct {
	ServiceName string      `json:"serviceName"`
	EventType   string      `json:"eventType"` // add, update, remove
	Instance    *Instance   `json:"instance"`
	Instances   []*Instance `json:"instances"`
	Timestamp   time.Time   `json:"timestamp"`
}

// GetAllServices 获取所有服务
func (d *Discovery) GetAllServices(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/nacos/v1/ns/services", d.serverAddr)
	params := fmt.Sprintf("?groupName=%s&namespaceId=%s&pageNo=1&pageSize=100",
		d.group, d.namespace)

	resp, err := d.client.Get(url + params)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Doms []string `json:"doms"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result.Doms, nil
}

// Close 关闭服务发现客户端
func (d *Discovery) Close() error {
	return nil
}
