package loadbalance

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// ServiceInstance 服务实例
type ServiceInstance struct {
	ID     string
	Host   string
	Port   uint
	Weight int
	Alive  bool
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	Select(instances []ServiceInstance) *ServiceInstance
}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	index int64
}

// Select 选择服务实例
func (rr *RoundRobinBalancer) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 过滤掉不健康的实例
	aliveInstances := make([]ServiceInstance, 0)
	for _, instance := range instances {
		if instance.Alive {
			aliveInstances = append(aliveInstances, instance)
		}
	}

	if len(aliveInstances) == 0 {
		return nil
	}

	// 使用原子操作递增索引，避免并发问题
	idx := atomic.AddInt64(&rr.index, 1) % int64(len(aliveInstances))
	return &aliveInstances[idx]
}

// RandomBalancer 随机负载均衡器
type RandomBalancer struct {
	rand *rand.Rand
	mu   sync.Mutex
}

// NewRandomBalancer 创建随机负载均衡器
func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select 选择服务实例
func (rb *RandomBalancer) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 过滤掉不健康的实例
	aliveInstances := make([]ServiceInstance, 0)
	for _, instance := range instances {
		if instance.Alive {
			aliveInstances = append(aliveInstances, instance)
		}
	}

	if len(aliveInstances) == 0 {
		return nil
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	idx := rb.rand.Intn(len(aliveInstances))
	return &aliveInstances[idx]
}

// WeightedRoundRobinBalancer 加权轮询负载均衡器
type WeightedRoundRobinBalancer struct {
	currentWeights []int
	index          int
	mu             sync.Mutex
}

// Select 选择服务实例
func (wrr *WeightedRoundRobinBalancer) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 过滤掉不健康的实例
	aliveInstances := make([]ServiceInstance, 0)
	for _, instance := range instances {
		if instance.Alive {
			aliveInstances = append(aliveInstances, instance)
		}
	}

	if len(aliveInstances) == 0 {
		return nil
	}

	wrr.mu.Lock()
	defer wrr.mu.Unlock()

	// 如果实例数量发生变化，重新初始化权重数组
	if len(wrr.currentWeights) != len(aliveInstances) {
		wrr.currentWeights = make([]int, len(aliveInstances))
		wrr.index = 0
	}

	// 计算总权重
	totalWeight := 0
	for _, instance := range aliveInstances {
		totalWeight += instance.Weight
	}

	if totalWeight <= 0 {
		// 如果总权重为0或负数，使用普通轮询
		idx := wrr.index % len(aliveInstances)
		wrr.index = (wrr.index + 1) % len(aliveInstances)
		return &aliveInstances[idx]
	}

	// 更新当前权重
	for i := range wrr.currentWeights {
		wrr.currentWeights[i] += aliveInstances[i].Weight
	}

	// 选择权重最高的实例
	selectedIdx := 0
	for i, weight := range wrr.currentWeights {
		if weight > wrr.currentWeights[selectedIdx] {
			selectedIdx = i
		}
	}

	// 减去总权重
	wrr.currentWeights[selectedIdx] -= totalWeight

	return &aliveInstances[selectedIdx]
}

// ConsistentHashBalancer 一致性哈希负载均衡器
type ConsistentHashBalancer struct {
	nodes    []string
	hashRing map[string]string // hash -> node
	mu       sync.RWMutex
}

// NewConsistentHashBalancer 创建一致性哈希负载均衡器
func NewConsistentHashBalancer(nodes []string) *ConsistentHashBalancer {
	chb := &ConsistentHashBalancer{
		nodes:    nodes,
		hashRing: make(map[string]string),
	}

	chb.buildHashRing()

	return chb
}

// buildHashRing 构建哈希环
func (chb *ConsistentHashBalancer) buildHashRing() {
	for _, node := range chb.nodes {
		for i := 0; i < 100; i++ { // 每个节点创建100个虚拟节点
			virtualNode := node + "#" + fmt.Sprintf("%d", i)
			hash := chb.hash(virtualNode)
			chb.hashRing[hash] = node
		}
	}
}

// hash 简单的哈希函数
func (chb *ConsistentHashBalancer) hash(key string) string {
	// 这里使用一个简单的哈希算法，实际应用中应该使用更复杂的哈希算法
	var hash uint32 = 5381
	for _, r := range key {
		hash = ((hash << 5) + hash) + uint32(r)
	}
	return fmt.Sprintf("%d", hash)
}

// Select 选择服务实例
func (chb *ConsistentHashBalancer) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 过滤掉不健康的实例
	aliveInstances := make([]ServiceInstance, 0)
	for _, instance := range instances {
		if instance.Alive {
			aliveInstances = append(aliveInstances, instance)
		}
	}

	if len(aliveInstances) == 0 {
		return nil
	}

	// 构建当前存活节点列表
	nodes := make([]string, len(aliveInstances))
	for i, instance := range aliveInstances {
		nodes[i] = instance.Host + ":" + fmt.Sprintf("%d", instance.Port)
	}

	// 重建哈希环
	chb.mu.Lock()
	chb.nodes = nodes
	chb.hashRing = make(map[string]string)
	for _, node := range chb.nodes {
		for i := 0; i < 100; i++ {
			virtualNode := node + "#" + fmt.Sprintf("%d", i)
			hash := chb.hash(virtualNode)
			chb.hashRing[hash] = node
		}
	}
	chb.mu.Unlock()

	// 使用当前时间作为key进行哈希计算
	key := time.Now().String()
	currentHash := chb.hash(key)

	// 在哈希环上查找节点
	chb.mu.RLock()
	defer chb.mu.RUnlock()

	// 在哈希环上查找最接近的节点
	var closestNode string
	for hashKey := range chb.hashRing {
		if hashKey >= currentHash && (closestNode == "" || hashKey < closestNode) {
			closestNode = hashKey
		}
	}

	// 如果没有找到更大hash值的节点，则使用最小hash值的节点（环形结构）
	if closestNode == "" {
		for hashKey := range chb.hashRing {
			if closestNode == "" || hashKey < closestNode {
				closestNode = hashKey
			}
		}
	}

	// 查找对应的实际节点
	if node, exists := chb.hashRing[closestNode]; exists {
		for i, instance := range aliveInstances {
			hostPort := instance.Host + ":" + fmt.Sprintf("%d", instance.Port)
			if hostPort == node {
				return &aliveInstances[i]
			}
		}
	}

	// 如果没找到，返回第一个实例
	return &aliveInstances[0]
}

// LeastConnectionBalancer 最少连接负载均衡器
type LeastConnectionBalancer struct {
	connectionCounts map[string]int64
	mu               sync.RWMutex
}

// NewLeastConnectionBalancer 创建最少连接负载均衡器
func NewLeastConnectionBalancer() *LeastConnectionBalancer {
	return &LeastConnectionBalancer{
		connectionCounts: make(map[string]int64),
	}
}

// Select 选择服务实例
func (lc *LeastConnectionBalancer) Select(instances []ServiceInstance) *ServiceInstance {
	if len(instances) == 0 {
		return nil
	}

	// 过滤掉不健康的实例
	aliveInstances := make([]ServiceInstance, 0)
	for _, instance := range instances {
		if instance.Alive {
			aliveInstances = append(aliveInstances, instance)
		}
	}

	if len(aliveInstances) == 0 {
		return nil
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	// 查找连接数最少的实例
	minConnections := int64(1<<63 - 1) // 最大值
	var selectedInstance *ServiceInstance

	for i, instance := range aliveInstances {
		key := instance.Host + ":" + fmt.Sprintf("%d", instance.Port)
		connections := lc.connectionCounts[key]

		if connections < minConnections {
			minConnections = connections
			selectedInstance = &aliveInstances[i]
		}
	}

	// 增加选中实例的连接数
	if selectedInstance != nil {
		key := selectedInstance.Host + ":" + fmt.Sprintf("%d", selectedInstance.Port)
		lc.connectionCounts[key]++
	}

	return selectedInstance
}

// ReleaseConnection 释放连接
func (lc *LeastConnectionBalancer) ReleaseConnection(instance *ServiceInstance) {
	if instance == nil {
		return
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	key := instance.Host + ":" + fmt.Sprintf("%d", instance.Port)
	count := lc.connectionCounts[key]
	if count > 0 {
		lc.connectionCounts[key]--
	}
}

// GetLoadBalancer 获取指定类型的负载均衡器
func GetLoadBalancer(strategy string) LoadBalancer {
	switch strategy {
	case "round_robin":
		return &RoundRobinBalancer{}
	case "random":
		return NewRandomBalancer()
	case "weighted_round_robin":
		return &WeightedRoundRobinBalancer{}
	case "least_connection":
		return NewLeastConnectionBalancer()
	case "consistent_hash":
		return &ConsistentHashBalancer{}
	default:
		return &RoundRobinBalancer{} // 默认使用轮询
	}
}
