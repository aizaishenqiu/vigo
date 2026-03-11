package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// EtcdDiscovery Etcd 服务发现
type EtcdDiscovery struct {
	client    *clientv3.Client
	endpoints []string
	prefix    string
	cache     map[string][]*Instance
	cacheMu   sync.RWMutex
	ttl       time.Duration
}

// EtcdConfig Etcd 配置
type EtcdConfig struct {
	Endpoints []string      `yaml:"endpoints"`
	Prefix    string        `yaml:"prefix"`
	Username  string        `yaml:"username"`
	Password  string        `yaml:"password"`
	Timeout   time.Duration `yaml:"timeout"`
	CacheTTL  time.Duration `yaml:"cache_ttl"`
}

// NewEtcdDiscovery 创建 Etcd 服务发现客户端
func NewEtcdDiscovery(cfg *EtcdConfig) (*EtcdDiscovery, error) {
	if cfg == nil {
		cfg = &EtcdConfig{}
	}

	if len(cfg.Endpoints) == 0 {
		cfg.Endpoints = []string{"localhost:2379"}
	}

	if cfg.Prefix == "" {
		cfg.Prefix = "/services"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 30 * time.Second
	}

	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.Timeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, err
	}

	return &EtcdDiscovery{
		client:    client,
		endpoints: cfg.Endpoints,
		prefix:    cfg.Prefix,
		cache:     make(map[string][]*Instance),
		ttl:       cfg.CacheTTL,
	}, nil
}

// Register 注册服务实例
func (d *EtcdDiscovery) Register(ctx context.Context, serviceName string, instance *Instance) error {
	key := path.Join(d.prefix, serviceName, fmt.Sprintf("%s-%d", instance.IP, instance.Port))

	data := map[string]interface{}{
		"id":       instance.ID,
		"ip":       instance.IP,
		"port":     instance.Port,
		"weight":   instance.Weight,
		"enabled":  instance.Enabled,
		"healthy":  instance.Healthy,
		"metadata": instance.Metadata,
	}

	value, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 创建租约
	leaseResp, err := d.client.Grant(ctx, int64(d.ttl.Seconds()))
	if err != nil {
		return err
	}

	// 注册服务（带租约）
	_, err = d.client.Put(ctx, key, string(value), clientv3.WithLease(leaseResp.ID))
	if err != nil {
		return err
	}

	// 保持租约活跃
	go d.keepAlive(ctx, leaseResp.ID)

	return nil
}

// Deregister 注销服务实例
func (d *EtcdDiscovery) Deregister(ctx context.Context, serviceName string, instance *Instance) error {
	key := path.Join(d.prefix, serviceName, fmt.Sprintf("%s-%d", instance.IP, instance.Port))

	_, err := d.client.Delete(ctx, key)
	return err
}

// GetInstances 获取服务实例列表
func (d *EtcdDiscovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	// 检查缓存
	d.cacheMu.RLock()
	if cached, ok := d.cache[serviceName]; ok {
		d.cacheMu.RUnlock()
		return cached, nil
	}
	d.cacheMu.RUnlock()

	// 从 Etcd 获取
	prefix := path.Join(d.prefix, serviceName) + "/"
	resp, err := d.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	instances := make([]*Instance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var data map[string]interface{}
		if err := json.Unmarshal(kv.Value, &data); err != nil {
			continue
		}

		instance := &Instance{
			ID:      getString(data, "id"),
			IP:      getString(data, "ip"),
			Port:    int(getFloat(data, "port")),
			Weight:  getFloat(data, "weight"),
			Enabled: getBool(data, "enabled"),
			Healthy: getBool(data, "healthy"),
		}

		if metadata, ok := data["metadata"].(map[string]interface{}); ok {
			instance.Metadata = make(map[string]string)
			for k, v := range metadata {
				if s, ok := v.(string); ok {
					instance.Metadata[k] = s
				}
			}
		}

		instances = append(instances, instance)
	}

	// 更新缓存
	d.cacheMu.Lock()
	d.cache[serviceName] = instances
	d.cacheMu.Unlock()

	return instances, nil
}

// GetHealthyInstances 获取健康服务实例列表
func (d *EtcdDiscovery) GetHealthyInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
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

// GetInstancesByMetadata 根据元数据获取服务实例
func (d *EtcdDiscovery) GetInstancesByMetadata(ctx context.Context, serviceName string, metadata map[string]string) ([]*Instance, error) {
	instances, err := d.GetInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	result := make([]*Instance, 0)
	for _, inst := range instances {
		match := true
		for k, v := range metadata {
			if inst.Metadata[k] != v {
				match = false
				break
			}
		}
		if match {
			result = append(result, inst)
		}
	}

	return result, nil
}

// Watch 监听服务实例变更
func (d *EtcdDiscovery) Watch(ctx context.Context, serviceName string) (<-chan *WatchEvent, error) {
	eventChan := make(chan *WatchEvent, 100)

	prefix := path.Join(d.prefix, serviceName) + "/"
	rch := d.client.Watch(ctx, prefix, clientv3.WithPrefix())

	go func() {
		defer close(eventChan)

		for {
			select {
			case <-ctx.Done():
				return
			case wresp, ok := <-rch:
				if !ok {
					return
				}

				for _, ev := range wresp.Events {
					var instance *Instance
					if len(ev.Kv.Value) > 0 {
						var data map[string]interface{}
						if err := json.Unmarshal(ev.Kv.Value, &data); err == nil {
							instance = &Instance{
								ID:      getString(data, "id"),
								IP:      getString(data, "ip"),
								Port:    int(getFloat(data, "port")),
								Weight:  getFloat(data, "weight"),
								Enabled: getBool(data, "enabled"),
								Healthy: getBool(data, "healthy"),
							}
						}
					}

					eventType := "update"
					switch ev.Type {
					case clientv3.EventTypePut:
						eventType = "add"
					case clientv3.EventTypeDelete:
						eventType = "remove"
					}

					eventChan <- &WatchEvent{
						EventType: eventType,
						Instances: []*Instance{instance},
					}
				}
			}
		}
	}()

	return eventChan, nil
}

// keepAlive 保持租约活跃
func (d *EtcdDiscovery) keepAlive(ctx context.Context, leaseID clientv3.LeaseID) {
	ticker := time.NewTicker(d.ttl / 3)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := d.client.KeepAliveOnce(ctx, leaseID); err != nil {
				return
			}
		}
	}
}

// Close 关闭连接
func (d *EtcdDiscovery) Close() error {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.cache = make(map[string][]*Instance)
	return d.client.Close()
}

// 辅助函数
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getFloat(m map[string]interface{}, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	return 1.0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return true
}

// 服务注册发现接口
type ServiceRegistry interface {
	// Register 注册服务
	Register(ctx context.Context, serviceName string, instance *Instance) error
	// Deregister 注销服务
	Deregister(ctx context.Context, serviceName string, instance *Instance) error
	// Close 关闭连接
	Close() error
}

// ServiceDiscovery 服务发现接口
type ServiceDiscovery interface {
	// GetInstances 获取服务实例列表
	GetInstances(ctx context.Context, serviceName string) ([]*Instance, error)
	// GetHealthyInstances 获取健康服务实例列表
	GetHealthyInstances(ctx context.Context, serviceName string) ([]*Instance, error)
	// GetInstancesByMetadata 根据元数据获取服务实例
	GetInstancesByMetadata(ctx context.Context, serviceName string, metadata map[string]string) ([]*Instance, error)
	// Watch 监听服务实例变更
	Watch(ctx context.Context, serviceName string) (<-chan *WatchEvent, error)
	// Close 关闭连接
	Close() error
}

// 统一的服务注册发现实现
type ServiceRegistryDiscovery struct {
	registry  ServiceRegistry
	discovery ServiceDiscovery
}

// NewServiceRegistryDiscovery 创建统一的服务注册发现客户端
func NewServiceRegistryDiscovery(registry ServiceRegistry, discovery ServiceDiscovery) *ServiceRegistryDiscovery {
	return &ServiceRegistryDiscovery{
		registry:  registry,
		discovery: discovery,
	}
}

// Register 注册服务（委托给 registry）
func (d *ServiceRegistryDiscovery) Register(ctx context.Context, serviceName string, instance *Instance) error {
	return d.registry.Register(ctx, serviceName, instance)
}

// Deregister 注销服务（委托给 registry）
func (d *ServiceRegistryDiscovery) Deregister(ctx context.Context, serviceName string, instance *Instance) error {
	return d.registry.Deregister(ctx, serviceName, instance)
}

// GetInstances 获取服务实例（委托给 discovery）
func (d *ServiceRegistryDiscovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	return d.discovery.GetInstances(ctx, serviceName)
}

// GetHealthyInstances 获取健康服务实例（委托给 discovery）
func (d *ServiceRegistryDiscovery) GetHealthyInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	return d.discovery.GetHealthyInstances(ctx, serviceName)
}

// GetInstancesByMetadata 根据元数据获取服务实例（委托给 discovery）
func (d *ServiceRegistryDiscovery) GetInstancesByMetadata(ctx context.Context, serviceName string, metadata map[string]string) ([]*Instance, error) {
	return d.discovery.GetInstancesByMetadata(ctx, serviceName, metadata)
}

// Watch 监听服务实例变更（委托给 discovery）
func (d *ServiceRegistryDiscovery) Watch(ctx context.Context, serviceName string) (<-chan *WatchEvent, error) {
	return d.discovery.Watch(ctx, serviceName)
}

// Close 关闭连接
func (d *ServiceRegistryDiscovery) Close() error {
	var err1, err2 error
	if d.registry != nil {
		err1 = d.registry.Close()
	}
	if d.discovery != nil {
		err2 = d.discovery.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}

// Factory 创建服务发现客户端的工厂函数
type DiscoveryFactory func(interface{}) (*ServiceRegistryDiscovery, error)

// 注册的服务发现工厂
var discoveryFactories = make(map[string]DiscoveryFactory)

// RegisterDiscoveryFactory 注册服务发现工厂
func RegisterDiscoveryFactory(name string, factory DiscoveryFactory) {
	discoveryFactories[name] = factory
}

// CreateDiscovery 创建服务发现客户端
func CreateDiscovery(name string, config interface{}) (*ServiceRegistryDiscovery, error) {
	factory, ok := discoveryFactories[name]
	if !ok {
		return nil, fmt.Errorf("unknown discovery type: %s", name)
	}
	return factory(config)
}

// 初始化时注册所有可用的服务发现
func init() {
	// 注册 Nacos
	RegisterDiscoveryFactory("nacos", func(cfg interface{}) (*ServiceRegistryDiscovery, error) {
		nacosCfg, ok := cfg.(*NacosDiscoveryOptions)
		if !ok {
			return nil, fmt.Errorf("invalid config type")
		}
		disc, err := NewNacosDiscovery(nacosCfg)
		if err != nil {
			return nil, err
		}
		return &ServiceRegistryDiscovery{
			registry:  disc,
			discovery: disc,
		}, nil
	})

	// 注册 Consul
	RegisterDiscoveryFactory("consul", func(cfg interface{}) (*ServiceRegistryDiscovery, error) {
		consulCfg, ok := cfg.(*ConsulConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type")
		}
		disc, err := NewConsulDiscovery(consulCfg)
		if err != nil {
			return nil, err
		}
		return &ServiceRegistryDiscovery{
			registry:  disc,
			discovery: disc,
		}, nil
	})

	// 注册 Etcd
	RegisterDiscoveryFactory("etcd", func(cfg interface{}) (*ServiceRegistryDiscovery, error) {
		etcdCfg, ok := cfg.(*EtcdConfig)
		if !ok {
			return nil, fmt.Errorf("invalid config type")
		}
		disc, err := NewEtcdDiscovery(etcdCfg)
		if err != nil {
			return nil, err
		}
		return &ServiceRegistryDiscovery{
			registry:  disc,
			discovery: disc,
		}, nil
	})
}
