package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/model"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// NacosDiscoveryOptions Nacos 服务发现选项
type NacosDiscoveryOptions struct {
	// Nacos 服务器地址
	ServerAddr string `yaml:"server_addr"`
	// 用户名
	Username string `yaml:"username"`
	// 密码
	Password string `yaml:"password"`
	// 命名空间 ID
	NamespaceId string `yaml:"namespace_id"`
	// 集群名称
	ClusterName string `yaml:"cluster_name"`
	// 分组名称
	GroupName string `yaml:"group_name"`
	// 超时时间（毫秒）
	TimeoutMs uint64 `yaml:"timeout_ms"`
	// 日志级别
	LogLevel string `yaml:"log_level"`
	// 日志路径
	LogDir string `yaml:"log_dir"`
	// 缓存目录
	CacheDir string `yaml:"cache_dir"`
}

// NacosDiscovery Nacos 服务发现实现
type NacosDiscovery struct {
	namingClient naming_client.INamingClient
	opts         *NacosDiscoveryOptions
	instances    map[string][]*Instance
	watchers     map[string][]chan *WatchEvent
	mu           sync.RWMutex
	cancelFunc   context.CancelFunc
	ctx          context.Context
}

// NewNacosDiscovery 创建 Nacos 服务发现客户端
func NewNacosDiscovery(opts *NacosDiscoveryOptions) (*NacosDiscovery, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if opts.ServerAddr == "" {
		opts.ServerAddr = "127.0.0.1:8848"
	}

	if opts.GroupName == "" {
		opts.GroupName = "DEFAULT_GROUP"
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

	// 创建命名客户端
	namingClient, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  clientConfig,
			ServerConfigs: serverConfigs,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create nacos naming client: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &NacosDiscovery{
		namingClient: namingClient,
		opts:         opts,
		instances:    make(map[string][]*Instance),
		watchers:     make(map[string][]chan *WatchEvent),
		cancelFunc:   cancel,
		ctx:          ctx,
	}, nil
}

// Register 注册服务实例
func (d *NacosDiscovery) Register(ctx context.Context, serviceName string, instance *Instance) error {
	param := vo.RegisterInstanceParam{
		Ip:          instance.IP,
		Port:        uint64(instance.Port),
		ServiceName: serviceName,
		Weight:      instance.Weight,
		Enable:      instance.Enabled,
		Healthy:     instance.Healthy,
		ClusterName: d.opts.ClusterName,
		GroupName:   d.opts.GroupName,
		Metadata:    instance.Metadata,
	}

	success, err := d.namingClient.RegisterInstance(param)
	if err != nil {
		return fmt.Errorf("failed to register instance: %v", err)
	}

	if !success {
		return fmt.Errorf("register instance failed")
	}

	return nil
}

// Deregister 注销服务实例
func (d *NacosDiscovery) Deregister(ctx context.Context, serviceName string, instance *Instance) error {
	param := vo.DeregisterInstanceParam{
		Ip:          instance.IP,
		Port:        uint64(instance.Port),
		ServiceName: serviceName,
		Cluster:     d.opts.ClusterName,
		GroupName:   d.opts.GroupName,
	}

	success, err := d.namingClient.DeregisterInstance(param)
	if err != nil {
		return fmt.Errorf("failed to deregister instance: %v", err)
	}

	if !success {
		return fmt.Errorf("deregister instance failed")
	}

	return nil
}

// GetInstances 获取服务实例列表
func (d *NacosDiscovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	param := vo.SelectInstancesParam{
		ServiceName: serviceName,
		GroupName:   d.opts.GroupName,
		HealthyOnly: true,
	}

	instances, err := d.namingClient.SelectInstances(param)
	if err != nil {
		return nil, fmt.Errorf("failed to select instances: %v", err)
	}

	result := make([]*Instance, 0, len(instances))
	for _, inst := range instances {
		result = append(result, &Instance{
			ID:          fmt.Sprintf("%s:%d", inst.Ip, inst.Port),
			IP:          inst.Ip,
			Port:        int(inst.Port),
			Weight:      inst.Weight,
			Enabled:     inst.Enable,
			Healthy:     inst.Healthy,
			ClusterName: inst.ClusterName,
			ServiceName: serviceName,
			Metadata:    inst.Metadata,
		})
	}

	d.mu.Lock()
	d.instances[serviceName] = result
	d.mu.Unlock()

	return result, nil
}

// GetHealthyInstances 获取健康服务实例列表
func (d *NacosDiscovery) GetHealthyInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	instances, err := d.GetInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 过滤健康实例
	healthy := make([]*Instance, 0)
	for _, inst := range instances {
		if inst.Healthy {
			healthy = append(healthy, inst)
		}
	}

	return healthy, nil
}

// GetInstancesByMetadata 根据元数据获取服务实例
func (d *NacosDiscovery) GetInstancesByMetadata(ctx context.Context, serviceName string, metadata map[string]string) ([]*Instance, error) {
	instances, err := d.GetInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	// 过滤匹配的实例
	result := make([]*Instance, 0)
	for _, inst := range instances {
		if matchMetadata(inst.Metadata, metadata) {
			result = append(result, inst)
		}
	}

	return result, nil
}

// matchMetadata 检查元数据是否匹配
func matchMetadata(instanceMeta, filterMeta map[string]string) bool {
	for k, v := range filterMeta {
		if instanceMeta[k] != v {
			return false
		}
	}
	return true
}

// SelectOneHealthyInstance 选择一个健康实例
func (d *NacosDiscovery) SelectOneHealthyInstance(ctx context.Context, serviceName string) (*Instance, error) {
	instances, err := d.GetHealthyInstances(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instance found")
	}

	// 返回第一个健康实例
	return instances[0], nil
}

// Watch 监听服务变化
func (d *NacosDiscovery) Watch(ctx context.Context, serviceName string) (<-chan *WatchEvent, error) {
	eventChan := make(chan *WatchEvent, 10)

	param := vo.SubscribeParam{
		ServiceName:       serviceName,
		GroupName:         d.opts.GroupName,
		SubscribeCallback: d.createSubscribeCallback(serviceName, eventChan),
	}

	err := d.namingClient.Subscribe(&param)
	if err != nil {
		close(eventChan)
		return nil, fmt.Errorf("failed to subscribe service: %v", err)
	}

	d.mu.Lock()
	d.watchers[serviceName] = append(d.watchers[serviceName], eventChan)
	d.mu.Unlock()

	return eventChan, nil
}

// Unwatch 取消监听
func (d *NacosDiscovery) Unwatch(ctx context.Context, serviceName string, eventChan <-chan *WatchEvent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if watchers, ok := d.watchers[serviceName]; ok {
		for i, ch := range watchers {
			if ch == eventChan {
				close(ch)
				d.watchers[serviceName] = append(watchers[:i], watchers[i+1:]...)
				break
			}
		}
	}

	return nil
}

// Close 关闭连接
func (d *NacosDiscovery) Close() error {
	if d.cancelFunc != nil {
		d.cancelFunc()
	}

	// 关闭所有 watcher
	d.mu.Lock()
	defer d.mu.Unlock()

	for serviceName, watchers := range d.watchers {
		for _, ch := range watchers {
			close(ch)
		}
		delete(d.watchers, serviceName)
	}

	return nil
}

// createSubscribeCallback 创建订阅回调
func (d *NacosDiscovery) createSubscribeCallback(serviceName string, eventChan chan<- *WatchEvent) func(services []model.Instance, err error) {
	return func(services []model.Instance, err error) {
		if err != nil {
			return
		}

		instances := make([]*Instance, 0, len(services))
		for _, inst := range services {
			instances = append(instances, &Instance{
				ID:          fmt.Sprintf("%s:%d", inst.Ip, inst.Port),
				IP:          inst.Ip,
				Port:        int(inst.Port),
				Weight:      inst.Weight,
				Enabled:     inst.Enable,
				Healthy:     inst.Healthy,
				ClusterName: inst.ClusterName,
				ServiceName: serviceName,
				Metadata:    inst.Metadata,
			})
		}

		select {
		case eventChan <- &WatchEvent{
			ServiceName: serviceName,
			EventType:   "update",
			Instances:   instances,
			Timestamp:   time.Now(),
		}:
		default:
			// Channel full, skip
		}
	}
}
