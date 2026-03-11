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

// ConsulDiscovery Consul 服务发现
type ConsulDiscovery struct {
	serverAddr string
	datacenter string
	token      string
	client     *http.Client
	cache      map[string][]*Instance
	cacheMu    sync.RWMutex
	ttl        time.Duration
}

// ConsulConfig Consul 配置
type ConsulConfig struct {
	ServerAddr string        `yaml:"server_addr"`
	Datacenter string        `yaml:"datacenter"`
	Token      string        `yaml:"token"`
	Timeout    time.Duration `yaml:"timeout"`
	CacheTTL   time.Duration `yaml:"cache_ttl"`
}

// NewConsulDiscovery 创建 Consul 服务发现客户端
func NewConsulDiscovery(cfg *ConsulConfig) (*ConsulDiscovery, error) {
	if cfg == nil {
		cfg = &ConsulConfig{}
	}

	if cfg.ServerAddr == "" {
		cfg.ServerAddr = "http://localhost:8500"
	}

	if cfg.Datacenter == "" {
		cfg.Datacenter = "dc1"
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 30 * time.Second
	}

	return &ConsulDiscovery{
		serverAddr: cfg.ServerAddr,
		datacenter: cfg.Datacenter,
		token:      cfg.Token,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		cache: make(map[string][]*Instance),
		ttl:   cfg.CacheTTL,
	}, nil
}

// Register 注册服务实例
func (d *ConsulDiscovery) Register(ctx context.Context, serviceName string, instance *Instance) error {
	url := fmt.Sprintf("%s/v1/agent/service/register", d.serverAddr)

	// 构建注册请求
	payload := map[string]interface{}{
		"ID":      fmt.Sprintf("%s-%s-%d", serviceName, instance.IP, instance.Port),
		"Name":    serviceName,
		"Address": instance.IP,
		"Port":    instance.Port,
		"Tags":    d.buildTags(instance),
		"Meta":    instance.Metadata,
		"Check": map[string]interface{}{
			"HTTP":     fmt.Sprintf("http://%s:%d/health", instance.IP, instance.Port),
			"Interval": "10s",
			"Timeout":  "5s",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, strings.NewReader(string(data)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if d.token != "" {
		req.Header.Set("X-Consul-Token", d.token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("failed to register service: %s, body: %s", resp.Status, string(body))
	}

	return nil
}

// Deregister 注销服务实例
func (d *ConsulDiscovery) Deregister(ctx context.Context, serviceName string, instance *Instance) error {
	instanceID := fmt.Sprintf("%s-%s-%d", serviceName, instance.IP, instance.Port)
	url := fmt.Sprintf("%s/v1/agent/service/deregister/%s", d.serverAddr, instanceID)

	req, err := http.NewRequestWithContext(ctx, "PUT", url, nil)
	if err != nil {
		return err
	}

	if d.token != "" {
		req.Header.Set("X-Consul-Token", d.token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to deregister service: %s", resp.Status)
	}

	return nil
}

// GetInstances 获取服务实例列表
func (d *ConsulDiscovery) GetInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
	// 检查缓存
	d.cacheMu.RLock()
	if cached, ok := d.cache[serviceName]; ok {
		d.cacheMu.RUnlock()
		return cached, nil
	}
	d.cacheMu.RUnlock()

	// 从 Consul 获取
	url := fmt.Sprintf("%s/v1/health/service/%s?dc=%s", d.serverAddr, serviceName, d.datacenter)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if d.token != "" {
		req.Header.Set("X-Consul-Token", d.token)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get instances: %s", resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var entries []struct {
		Service struct {
			ID      string            `json:"ID"`
			Service string            `json:"Service"`
			Address string            `json:"Address"`
			Port    int               `json:"Port"`
			Tags    []string          `json:"Tags"`
			Meta    map[string]string `json:"Meta"`
			Healthy bool              `json:"Healthy"`
		} `json:"Service"`
		Checks []struct {
			Status string `json:"Status"`
		} `json:"Checks"`
	}

	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}

	instances := make([]*Instance, 0)
	for _, entry := range entries {
		// 检查健康状态
		healthy := true
		for _, check := range entry.Checks {
			if check.Status != "passing" {
				healthy = false
				break
			}
		}

		if !healthy {
			continue
		}

		instance := &Instance{
			ID:       entry.Service.ID,
			IP:       entry.Service.Address,
			Port:     entry.Service.Port,
			Weight:   1.0,
			Enabled:  true,
			Healthy:  healthy,
			Metadata: entry.Service.Meta,
		}

		// 解析标签获取权重等信息
		for _, tag := range entry.Service.Tags {
			if strings.HasPrefix(tag, "weight=") {
				fmt.Sscanf(tag, "weight=%f", &instance.Weight)
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
func (d *ConsulDiscovery) GetHealthyInstances(ctx context.Context, serviceName string) ([]*Instance, error) {
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
func (d *ConsulDiscovery) GetInstancesByMetadata(ctx context.Context, serviceName string, metadata map[string]string) ([]*Instance, error) {
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
func (d *ConsulDiscovery) Watch(ctx context.Context, serviceName string) (<-chan *WatchEvent, error) {
	eventChan := make(chan *WatchEvent, 100)

	go func() {
		defer close(eventChan)

		lastIndex := uint64(0)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			url := fmt.Sprintf("%s/v1/health/service/%s?dc=%s&wait=10s&index=%d",
				d.serverAddr, serviceName, d.datacenter, lastIndex)

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				continue
			}

			if d.token != "" {
				req.Header.Set("X-Consul-Token", d.token)
			}

			resp, err := d.client.Do(req)
			if err != nil {
				time.Sleep(time.Second)
				continue
			}

			// 获取新的 index
			lastIndexStr := resp.Header.Get("X-Consul-Index")
			var newIndex uint64
			fmt.Sscanf(lastIndexStr, "%d", &newIndex)

			if newIndex == lastIndex {
				resp.Body.Close()
				continue
			}

			lastIndex = newIndex

			_, err = ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			// 解析实例列表（与 GetInstances 相同）
			// ... 省略解析代码 ...

			eventChan <- &WatchEvent{
				EventType: "update",
				Instances: nil, // 解析后的实例列表
			}
		}
	}()

	return eventChan, nil
}

// buildTags 构建标签列表
func (d *ConsulDiscovery) buildTags(instance *Instance) []string {
	tags := make([]string, 0)

	if instance.Weight != 1.0 {
		tags = append(tags, fmt.Sprintf("weight=%.2f", instance.Weight))
	}

	for k, v := range instance.Metadata {
		tags = append(tags, fmt.Sprintf("%s=%s", k, v))
	}

	return tags
}

// Close 关闭连接
func (d *ConsulDiscovery) Close() error {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.cache = make(map[string][]*Instance)
	return nil
}
