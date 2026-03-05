package balancer

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy 负载均衡策略
type Strategy int

const (
	Random             Strategy = iota // 随机
	RoundRobin                         // 轮询
	LeastConn                          // 最少连接
	WeightedRoundRobin                 // 加权轮询
	WeightedRandom                     // 加权随机
)

// Instance 服务实例
type Instance struct {
	Address  string
	Weight   int32 // 权重（用于加权算法）
	Metadata map[string]string
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	AddInstance(instance *Instance)
	RemoveInstance(address string)
	GetInstance(ctx context.Context) (*Instance, error)
	MarkSuccess(address string)
	MarkFailure(address string)
}

// RandomBalancer 随机负载均衡器
type RandomBalancer struct {
	mu        sync.RWMutex
	instances []*Instance
}

func NewRandomBalancer() *RandomBalancer {
	return &RandomBalancer{
		instances: make([]*Instance, 0),
	}
}

func (b *RandomBalancer) AddInstance(instance *Instance) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.instances = append(b.instances, instance)
}

func (b *RandomBalancer) RemoveInstance(address string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, inst := range b.instances {
		if inst.Address == address {
			b.instances = append(b.instances[:i], b.instances[i+1:]...)
			break
		}
	}
}

func (b *RandomBalancer) GetInstance(ctx context.Context) (*Instance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.instances) == 0 {
		return nil, errors.New("no available instances")
	}

	return b.instances[rand.Intn(len(b.instances))], nil
}

func (b *RandomBalancer) MarkSuccess(address string) {}
func (b *RandomBalancer) MarkFailure(address string) {}

// RoundRobinBalancer 轮询负载均衡器
type RoundRobinBalancer struct {
	mu        sync.RWMutex
	instances []*Instance
	current   uint64
}

func NewRoundRobinBalancer() *RoundRobinBalancer {
	return &RoundRobinBalancer{}
}

func (b *RoundRobinBalancer) AddInstance(instance *Instance) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.instances = append(b.instances, instance)
}

func (b *RoundRobinBalancer) RemoveInstance(address string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, inst := range b.instances {
		if inst.Address == address {
			b.instances = append(b.instances[:i], b.instances[i+1:]...)
			break
		}
	}
}

func (b *RoundRobinBalancer) GetInstance(ctx context.Context) (*Instance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.instances) == 0 {
		return nil, errors.New("no available instances")
	}

	idx := atomic.AddUint64(&b.current, 1) % uint64(len(b.instances))
	return b.instances[idx], nil
}

func (b *RoundRobinBalancer) MarkSuccess(address string) {}
func (b *RoundRobinBalancer) MarkFailure(address string) {}

// LeastConnBalancer 最少连接负载均衡器
type LeastConnBalancer struct {
	mu        sync.RWMutex
	instances []*Instance
	conns     map[string]int64
}

func NewLeastConnBalancer() *LeastConnBalancer {
	return &LeastConnBalancer{
		conns: make(map[string]int64),
	}
}

func (b *LeastConnBalancer) AddInstance(instance *Instance) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.instances = append(b.instances, instance)
	b.conns[instance.Address] = 0
}

func (b *LeastConnBalancer) RemoveInstance(address string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, inst := range b.instances {
		if inst.Address == address {
			b.instances = append(b.instances[:i], b.instances[i+1:]...)
			delete(b.conns, address)
			break
		}
	}
}

func (b *LeastConnBalancer) GetInstance(ctx context.Context) (*Instance, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.instances) == 0 {
		return nil, errors.New("no available instances")
	}

	// 选择连接数最少的实例
	var minConn int64 = -1
	var selected *Instance

	for _, inst := range b.instances {
		conn := b.conns[inst.Address]
		if minConn == -1 || conn < minConn {
			minConn = conn
			selected = inst
		}
	}

	return selected, nil
}

func (b *LeastConnBalancer) MarkSuccess(address string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.conns[address]++
}

func (b *LeastConnBalancer) MarkFailure(address string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conns[address] > 0 {
		b.conns[address]--
	}
}

// CreateBalancer 创建负载均衡器
func CreateBalancer(strategy Strategy, instances []*Instance) LoadBalancer {
	var balancer LoadBalancer

	switch strategy {
	case Random:
		balancer = NewRandomBalancer()
	case RoundRobin:
		balancer = NewRoundRobinBalancer()
	case LeastConn:
		balancer = NewLeastConnBalancer()
	default:
		balancer = NewRoundRobinBalancer()
	}

	// 添加实例
	for _, inst := range instances {
		switch b := balancer.(type) {
		case *RandomBalancer:
			b.AddInstance(inst)
		case *RoundRobinBalancer:
			b.AddInstance(inst)
		case *LeastConnBalancer:
			b.AddInstance(inst)
		}
	}

	return balancer
}

// ReverseProxy 简单的反向代理
type ReverseProxy struct {
	balancer LoadBalancer
	client   *http.Client
}

func NewReverseProxy(balancer LoadBalancer) *ReverseProxy {
	return &ReverseProxy{
		balancer: balancer,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 获取实例
	instance, err := p.balancer.GetInstance(r.Context())
	if err != nil {
		http.Error(w, "no available instance", http.StatusServiceUnavailable)
		return
	}

	// 创建代理请求
	target := &url.URL{
		Scheme: "http",
		Host:   instance.Address,
	}

	proxyReq := &http.Request{
		Method:        r.Method,
		URL:           target,
		Header:        r.Header.Clone(),
		Body:          r.Body,
		ContentLength: r.ContentLength,
	}

	// 转发请求
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		p.balancer.MarkFailure(instance.Address)
		http.Error(w, "proxy error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	p.balancer.MarkSuccess(instance.Address)

	// 复制响应
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// 复制响应体
	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
}
