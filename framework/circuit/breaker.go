package circuit

import (
	"sync"
	"sync/atomic"
	"time"
)

// State 表示熔断器的状态
type State int

const (
	Closed State = iota // 关闭状态 - 正常调用
	Open               // 开启状态 - 拒绝调用
	HalfOpen           // 半开状态 - 尝试调用
)

// CircuitBreaker 服务熔断器
type CircuitBreaker struct {
	name          string
	maxFailures   int64
	resetTimeout  time.Duration
	requestCount  int64
	failureCount  int64
	lastFailure   time.Time
	state         State
	stateChangeAt time.Time
	mu            sync.RWMutex
}

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(name string, maxFailures int64, resetTimeout time.Duration) *CircuitBreaker {
	cb := &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        Closed,
	}

	// 定期清理请求计数，避免长时间运行导致的计数累积
	go cb.cleanupPeriodically()

	return cb
}

// Call 执行被保护的操作
func (cb *CircuitBreaker) Call(operation func() error) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 检查是否需要从开启状态切换到半开状态
	if cb.state == Open && time.Since(cb.stateChangeAt) >= cb.resetTimeout {
		cb.state = HalfOpen
		cb.stateChangeAt = time.Now()
	}

	// 根据当前状态决定是否执行操作
	switch cb.state {
	case Open:
		return &OpenCircuitError{cb.name}
	case HalfOpen:
		// 在半开状态下只允许一个请求通过
		if !cb.tryAcquireHalfOpenPermission() {
			return &OpenCircuitError{cb.name}
		}
	}

	// 更新请求计数
	atomic.AddInt64(&cb.requestCount, 1)

	// 执行操作
	err := operation()
	if err != nil {
		cb.onFailure()
		return err
	}

	cb.onSuccess()
	return nil
}

// onSuccess 成功时的处理
func (cb *CircuitBreaker) onSuccess() {
	atomic.StoreInt64(&cb.failureCount, 0)
	cb.state = Closed
	cb.stateChangeAt = time.Now()
}

// onFailure 失败时的处理
func (cb *CircuitBreaker) onFailure() {
	failureCount := atomic.AddInt64(&cb.failureCount, 1)
	cb.lastFailure = time.Now()

	// 如果失败次数超过阈值，切换到开启状态
	if failureCount >= cb.maxFailures && cb.state != Open {
		cb.state = Open
		cb.stateChangeAt = time.Now()
	}
}

// tryAcquireHalfOpenPermission 尝试获取半开状态下的执行权限
func (cb *CircuitBreaker) tryAcquireHalfOpenPermission() bool {
	// 简单的实现：每次半开状态只允许一个请求
	return cb.state == HalfOpen
}

// cleanupPeriodically 定期清理
func (cb *CircuitBreaker) cleanupPeriodically() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cb.mu.Lock()
		// 如果距离上次失败已经超过重置超时的两倍，重置计数器
		if time.Since(cb.lastFailure) > cb.resetTimeout*2 {
			atomic.StoreInt64(&cb.requestCount, 0)
			atomic.StoreInt64(&cb.failureCount, 0)
		}
		cb.mu.Unlock()
	}
}

// GetStats 获取熔断器统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"name":           cb.name,
		"state":          cb.state,
		"request_count":  atomic.LoadInt64(&cb.requestCount),
		"failure_count":  atomic.LoadInt64(&cb.failureCount),
		"last_failure":   cb.lastFailure,
		"state_change_at": cb.stateChangeAt,
	}
}

// OpenCircuitError 熔断器开启错误
type OpenCircuitError struct {
	ServiceName string
}

func (e *OpenCircuitError) Error() string {
	return "circuit breaker is open for service: " + e.ServiceName
}

// IsOpen 检查熔断器是否开启
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == Open
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	atomic.StoreInt64(&cb.requestCount, 0)
	atomic.StoreInt64(&cb.failureCount, 0)
	cb.state = Closed
	cb.stateChangeAt = time.Now()
	cb.lastFailure = time.Time{}
}

// StateName 获取状态名称
func (s State) String() string {
	switch s {
	case Closed:
		return "Closed"
	case Open:
		return "Open"
	case HalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}