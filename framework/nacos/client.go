package nacos

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"vigo/framework/circuit"
	"vigo/framework/loadbalance"
)

type Config struct {
	IpAddr      string
	Port        uint64
	NamespaceId string
	DataId      string
	Group       string
	InstallPath string
}

type Client struct {
	config       Config
	mu           sync.RWMutex
	connected    bool
	baseURL      string
	client       *http.Client
	stopChan     chan struct{}
	wg           sync.WaitGroup
	registered   bool
	serviceName  string
	instanceIP   string
	instancePort uint64

	// 新增微服务功能
	circuitBreakers map[string]*circuit.CircuitBreaker
	loadBalancers   map[string]loadbalance.LoadBalancer
	serviceCache    map[string][]loadbalance.ServiceInstance
	cacheMutex      sync.RWMutex
}

func NewClient(cfg Config) *Client {
	client := &Client{
		config:          cfg,
		baseURL:         fmt.Sprintf("http://%s:%d", cfg.IpAddr, cfg.Port),
		client:          &http.Client{Timeout: 10 * time.Second}, // 增加超时时间
		stopChan:        make(chan struct{}),
		circuitBreakers: make(map[string]*circuit.CircuitBreaker),
		loadBalancers:   make(map[string]loadbalance.LoadBalancer),
		serviceCache:    make(map[string][]loadbalance.ServiceInstance),
	}

	// 检查并设置安装路径
	if cfg.InstallPath != "" {
		client.config.InstallPath = cfg.InstallPath
	}

	return client
}

func (c *Client) GetConfigInfo() map[string]interface{} {
	return map[string]interface{}{
		"host":      c.config.IpAddr,
		"port":      c.config.Port,
		"namespace": c.config.NamespaceId,
		"data_id":   c.config.DataId,
		"group":     c.config.Group,
	}
}

func (c *Client) CheckHealth() bool {
	urls := []string{
		c.baseURL + "/nacos/v1/console/health/liveness",
		c.baseURL + "/nacos/v2/console/health/liveness",
		c.baseURL + "/nacos/",
	}

	for _, u := range urls {
		resp, err := c.client.Get(u)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 || resp.StatusCode == 302 {
				c.mu.Lock()
				c.connected = true
				c.mu.Unlock()
				return true
			}
		}
	}

	// 尝试连接失败时，检查Nacos是否已安装但未启动
	if !c.isNacosRunning() {
		// Nacos可能未安装或未启动，尝试启动或提示用户
		if !c.isNacosInstalled() {
			fmt.Println("[Nacos] 未检测到Nacos，请按以下步骤安装：")
			fmt.Println("1. 下载地址: https://github.com/alibaba/nacos/releases")
			fmt.Println("2. 解压后进入 bin 目录")
			if runtime.GOOS == "windows" {
				fmt.Println("3. Windows系统执行: startup.cmd -m standalone")
			} else {
				fmt.Println("3. Linux/Mac系统执行: sh startup.sh -m standalone")
			}
			fmt.Println("4. 启动后等待2分钟再重试")
		} else {
			fmt.Println("[Nacos] 检测到Nacos已安装但未运行，尝试启动...")
			if err := c.startNacos(); err != nil {
				fmt.Printf("[Nacos] 启动失败: %v\n", err)
				fmt.Println("[Nacos] 请手动启动Nacos服务")
			} else {
				fmt.Println("[Nacos] Nacos启动命令已发出，请稍等2分钟后再试")
			}
		}
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return false
}

// isNacosInstalled 检查Nacos是否已安装
func (c *Client) isNacosInstalled() bool {
	if c.config.InstallPath == "" {
		return false
	}

	var startupScript string
	if runtime.GOOS == "windows" {
		startupScript = "bin/startup.cmd"
	} else {
		startupScript = "bin/startup.sh"
	}

	fullPath := fmt.Sprintf("%s/%s", c.config.InstallPath, startupScript)

	_, err := os.Stat(fullPath)
	return err == nil
}

// isNacosRunning 检查Nacos进程是否在运行
func (c *Client) isNacosRunning() bool {
	// 尝试通过端口检查Nacos是否在运行
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", c.config.IpAddr, c.config.Port), 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// startNacos 尝试启动Nacos
func (c *Client) startNacos() error {
	if c.config.InstallPath == "" {
		return fmt.Errorf("未配置Nacos安装路径，无法自动启动")
	}

	var startupScript string
	if runtime.GOOS == "windows" {
		startupScript = fmt.Sprintf("%s\\bin\\startup.cmd", c.config.InstallPath)
	} else {
		startupScript = fmt.Sprintf("%s/bin/startup.sh", c.config.InstallPath)
	}

	// 检查启动脚本是否存在
	if _, err := os.Stat(startupScript); err != nil {
		return fmt.Errorf("启动脚本不存在: %s", startupScript)
	}

	// 设置执行权限（Linux/Mac）
	if runtime.GOOS != "windows" {
		if err := os.Chmod(startupScript, 0755); err != nil {
			fmt.Printf("[Nacos] 设置启动脚本权限失败: %v\n", err)
		}
	}

	// 尝试启动Nacos
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", startupScript, "-m", "standalone")
	} else {
		cmd = exec.Command("sh", startupScript, "-m", "standalone")
	}

	cmd.Dir = fmt.Sprintf("%s/bin", c.config.InstallPath)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("启动Nacos失败: %v", err)
	}

	fmt.Printf("[Nacos] Nacos启动命令已发出，PID: %d\n", cmd.Process.Pid)
	fmt.Println("[Nacos] 请等待几分钟让Nacos完成启动...")

	return nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) GetConfig() (string, error) {
	return c.GetConfigByID(c.config.DataId, c.config.Group)
}

func (c *Client) GetConfigByID(dataId, group string) (string, error) {
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	params := url.Values{
		"dataId": {dataId},
		"group":  {group},
	}
	if c.config.NamespaceId != "" {
		params.Set("tenant", c.config.NamespaceId)
	}

	resp, err := c.client.Get(c.baseURL + "/nacos/v1/cs/configs?" + params.Encode())
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("获取配置失败 (%d): %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

func (c *Client) PublishConfig(dataId, group, content string) error {
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	data := url.Values{
		"dataId":  {dataId},
		"group":   {group},
		"content": {content},
	}
	if c.config.NamespaceId != "" {
		data.Set("tenant", c.config.NamespaceId)
	}

	resp, err := c.client.Post(
		c.baseURL+"/nacos/v1/cs/configs",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("发布配置失败 (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) DeleteConfig(dataId, group string) error {
	if group == "" {
		group = "DEFAULT_GROUP"
	}

	params := url.Values{
		"dataId": {dataId},
		"group":  {group},
	}
	if c.config.NamespaceId != "" {
		params.Set("tenant", c.config.NamespaceId)
	}

	req, err := http.NewRequest("DELETE", c.baseURL+"/nacos/v1/cs/configs?"+params.Encode(), nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("删除配置失败 (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) ListServices(page, pageSize int) (map[string]interface{}, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	params := url.Values{
		"pageNo":   {fmt.Sprintf("%d", page)},
		"pageSize": {fmt.Sprintf("%d", pageSize)},
	}
	if c.config.NamespaceId != "" {
		params.Set("namespaceId", c.config.NamespaceId)
	}

	resp, err := c.client.Get(c.baseURL + "/nacos/v1/ns/service/list?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取服务列表失败 (%d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) GetServiceInstances(serviceName string) (map[string]interface{}, error) {
	params := url.Values{
		"serviceName": {serviceName},
	}
	if c.config.NamespaceId != "" {
		params.Set("namespaceId", c.config.NamespaceId)
	}

	resp, err := c.client.Get(c.baseURL + "/nacos/v1/ns/instance/list?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取实例列表失败 (%d): %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Client) RegisterInstance(ip string, port uint64, serviceName string) error {
	data := url.Values{
		"ip":          {ip},
		"port":        {fmt.Sprintf("%d", port)},
		"serviceName": {serviceName},
		"healthy":     {"true"},
		"enabled":     {"true"},
		"weight":      {"1.0"},
		"ephemeral":   {"true"},
	}
	if c.config.NamespaceId != "" {
		data.Set("namespaceId", c.config.NamespaceId)
	}

	resp, err := c.client.Post(
		c.baseURL+"/nacos/v1/ns/instance",
		"application/x-www-form-urlencoded",
		strings.NewReader(data.Encode()),
	)
	if err != nil {
		return fmt.Errorf("注册实例失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("注册实例失败 (%d): %s", resp.StatusCode, string(body))
	}

	c.mu.Lock()
	c.registered = true
	c.serviceName = serviceName
	c.instanceIP = ip
	c.instancePort = port
	c.mu.Unlock()

	return nil
}

func (c *Client) DeregisterInstance(ip string, port uint64, serviceName string) error {
	params := url.Values{
		"ip":          {ip},
		"port":        {fmt.Sprintf("%d", port)},
		"serviceName": {serviceName},
		"ephemeral":   {"true"},
	}
	if c.config.NamespaceId != "" {
		params.Set("namespaceId", c.config.NamespaceId)
	}

	req, err := http.NewRequest("DELETE", c.baseURL+"/nacos/v1/ns/instance?"+params.Encode(), nil)
	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("注销实例失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("注销实例失败 (%d): %s", resp.StatusCode, string(body))
	}

	c.mu.Lock()
	c.registered = false
	c.mu.Unlock()

	return nil
}

func (c *Client) sendHeartbeat() error {
	c.mu.RLock()
	if !c.registered {
		c.mu.RUnlock()
		return nil
	}
	serviceName := c.serviceName
	ip := c.instanceIP
	port := c.instancePort
	c.mu.RUnlock()

	data := url.Values{
		"serviceName": {serviceName},
		"ip":          {ip},
		"port":        {fmt.Sprintf("%d", port)},
		"beat":        {fmt.Sprintf(`{"serviceName":"%s","ip":"%s","port":%d,"weight":1.0}`, serviceName, ip, port)},
	}
	if c.config.NamespaceId != "" {
		data.Set("namespaceId", c.config.NamespaceId)
	}

	req, err := http.NewRequest("PUT", c.baseURL+"/nacos/v1/ns/instance/beat", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("创建心跳请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送心跳失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("心跳失败 (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) StartHeartbeat(interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-c.stopChan:
				return
			case <-ticker.C:
				if err := c.sendHeartbeat(); err != nil {
					fmt.Printf("[Nacos] 心跳发送失败: %v\n", err)
				}
			}
		}
	}()
}

func (c *Client) AutoRegister(serviceName string, port uint64) error {
	// 首先检查Nacos连接状态
	if !c.CheckHealth() {
		return fmt.Errorf("Nacos服务不可用，请先启动Nacos服务")
	}

	ip, err := c.getLocalIP()
	if err != nil {
		ip = "127.0.0.1"
	}

	if serviceName == "" {
		hostname, _ := os.Hostname()
		serviceName = "vigo-" + hostname
	}

	if err := c.RegisterInstance(ip, port, serviceName); err != nil {
		return err
	}

	fmt.Printf("[Nacos] 服务注册成功: %s @ %s:%d\n", serviceName, ip, port)

	c.StartHeartbeat(5 * time.Second)

	return nil
}

func (c *Client) getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no valid IP found")
}

func (c *Client) DiscoverService(serviceName string) ([]Instance, error) {
	result, err := c.GetServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	var instances []Instance
	if hosts, ok := result["hosts"].([]interface{}); ok {
		for _, h := range hosts {
			if m, ok := h.(map[string]interface{}); ok {
				inst := Instance{}
				if ip, ok := m["ip"].(string); ok {
					inst.IP = ip
				}
				if port, ok := m["port"].(float64); ok {
					inst.Port = int(port)
				}
				if healthy, ok := m["healthy"].(bool); ok {
					inst.Healthy = healthy
				}
				if weight, ok := m["weight"].(float64); ok {
					inst.Weight = weight
				}
				instances = append(instances, inst)
			}
		}
	}
	return instances, nil
}

type Instance struct {
	IP      string
	Port    int
	Healthy bool
	Weight  float64
}

func (c *Client) ListNamespaces() ([]map[string]interface{}, error) {
	resp, err := c.client.Get(c.baseURL + "/nacos/v1/console/namespaces")
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("获取命名空间失败 (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		var arr []map[string]interface{}
		if err2 := json.Unmarshal(body, &arr); err2 != nil {
			return nil, err
		}
		return arr, nil
	}
	return result.Data, nil
}

func (c *Client) Close() error {
	close(c.stopChan)
	c.wg.Wait()

	c.mu.RLock()
	if c.registered {
		svcName := c.serviceName
		ip := c.instanceIP
		port := c.instancePort
		c.mu.RUnlock()
		c.DeregisterInstance(ip, port, svcName)
		fmt.Printf("[Nacos] 服务注销: %s\n", svcName)
	} else {
		c.mu.RUnlock()
	}

	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()
	return nil
}

// GetCircuitBreaker 获取指定服务的熔断器
func (c *Client) GetCircuitBreaker(serviceName string) *circuit.CircuitBreaker {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cb, exists := c.circuitBreakers[serviceName]; exists {
		return cb
	}

	// 创建新的熔断器，默认参数
	cb := circuit.NewCircuitBreaker(serviceName, 5, 60*time.Second)
	c.circuitBreakers[serviceName] = cb
	return cb
}

// GetLoadBalancer 获取指定服务的负载均衡器
func (c *Client) GetLoadBalancer(serviceName string, strategy string) loadbalance.LoadBalancer {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := serviceName + "_" + strategy
	if lb, exists := c.loadBalancers[key]; exists {
		return lb
	}

	// 创建新的负载均衡器
	lb := loadbalance.GetLoadBalancer(strategy)
	c.loadBalancers[key] = lb
	return lb
}

// GetServiceInstancesWithLoadBalance 获取服务实例并使用负载均衡
func (c *Client) GetServiceInstancesWithLoadBalance(serviceName string, strategy string) (*loadbalance.ServiceInstance, error) {
	// 先获取所有实例
	instances, err := c.GetServiceInstances(serviceName)
	if err != nil {
		return nil, err
	}

	// 将实例转换为内部格式
	serviceInstances := make([]loadbalance.ServiceInstance, 0)
	if hosts, ok := instances["hosts"].([]interface{}); ok {
		for _, h := range hosts {
			if m, ok := h.(map[string]interface{}); ok {
				inst := loadbalance.ServiceInstance{}
				if ip, ok := m["ip"].(string); ok {
					inst.Host = ip
				}
				if port, ok := m["port"].(float64); ok {
					inst.Port = uint(port)
				}
				if healthy, ok := m["healthy"].(bool); ok {
					inst.Alive = healthy
				}
				if weight, ok := m["weight"].(float64); ok {
					inst.Weight = int(weight)
				}
				inst.ID = fmt.Sprintf("%s:%d", inst.Host, inst.Port)
				serviceInstances = append(serviceInstances, inst)
			}
		}
	}

	// 使用负载均衡器选择实例
	lb := c.GetLoadBalancer(serviceName, strategy)
	selected := lb.Select(serviceInstances)
	if selected == nil {
		return nil, fmt.Errorf("no available service instance for %s", serviceName)
	}

	return selected, nil
}

// UpdateServiceCache 更新服务实例缓存
func (c *Client) UpdateServiceCache(serviceName string) error {
	instances, err := c.GetServiceInstances(serviceName)
	if err != nil {
		return err
	}

	serviceInstances := make([]loadbalance.ServiceInstance, 0)
	if hosts, ok := instances["hosts"].([]interface{}); ok {
		for _, h := range hosts {
			if m, ok := h.(map[string]interface{}); ok {
				inst := loadbalance.ServiceInstance{}
				if ip, ok := m["ip"].(string); ok {
					inst.Host = ip
				}
				if port, ok := m["port"].(float64); ok {
					inst.Port = uint(port)
				}
				if healthy, ok := m["healthy"].(bool); ok {
					inst.Alive = healthy
				}
				if weight, ok := m["weight"].(float64); ok {
					inst.Weight = int(weight)
				}
				inst.ID = fmt.Sprintf("%s:%d", inst.Host, inst.Port)
				serviceInstances = append(serviceInstances, inst)
			}
		}
	}

	c.cacheMutex.Lock()
	c.serviceCache[serviceName] = serviceInstances
	c.cacheMutex.Unlock()

	return nil
}

// GetCachedServiceInstances 获取缓存的服务实例
func (c *Client) GetCachedServiceInstances(serviceName string) []loadbalance.ServiceInstance {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	if instances, exists := c.serviceCache[serviceName]; exists {
		return instances
	}
	return []loadbalance.ServiceInstance{}
}
