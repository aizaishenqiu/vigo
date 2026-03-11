package circuit

import (
	"errors"
	"sync"
	"time"
)

// AdaptiveCircuitBreaker 自适应熔断器（类似 go-zero 的自适应熔断）
type AdaptiveCircuitBreaker struct {
	mu sync.RWMutex

	// 状态控制
	state      State
	lastChange time.Time

	// 自适应参数
	maxRT        time.Duration // 最大响应时间（动态调整）
	maxSuccesses uint64        // 最大成功数
	maxFailures  uint64        // 最大失败数
	maxRequests  uint64        // 最大请求数

	// 统计信息
	totalRequests       uint64
	totalSuccesses      uint64
	totalFailures       uint64
	consecutiveFailures uint64
	lastFailureTime     time.Time

	// 回调
	onStateChange []func(State, State)

	// 配置
	windowSize     time.Duration // 统计窗口大小
	sleepWindow    time.Duration // 熔断后重试等待时间
	halfOpenMaxReq uint64        // 半开状态最大请求数
}

// NewAdaptiveCircuitBreaker 创建自适应熔断器
func NewAdaptiveCircuitBreaker(opts ...CircuitBreakerOption) *AdaptiveCircuitBreaker {
	cb := &AdaptiveCircuitBreaker{
		state:          Closed,
		lastChange:     time.Now(),
		maxRT:          100 * time.Millisecond,
		maxSuccesses:   100,
		maxFailures:    10,
		maxRequests:    1000,
		windowSize:     10 * time.Second,
		sleepWindow:    5 * time.Second,
		halfOpenMaxReq: 10,
		onStateChange:  make([]func(State, State), 0),
	}

	for _, opt := range opts {
		opt(cb)
	}

	// 启动自适应调整协程
	go cb.autoAdjust()

	return cb
}

// CircuitBreakerOption 熔断器配置选项
type CircuitBreakerOption func(*AdaptiveCircuitBreaker)

// WithMaxRT 设置最大响应时间
func WithMaxRT(rt time.Duration) CircuitBreakerOption {
	return func(cb *AdaptiveCircuitBreaker) {
		cb.maxRT = rt
	}
}

// WithMaxFailures 设置最大失败数
func WithMaxFailures(count uint64) CircuitBreakerOption {
	return func(cb *AdaptiveCircuitBreaker) {
		cb.maxFailures = count
	}
}

// WithSleepWindow 设置熔断后等待时间
func WithSleepWindow(d time.Duration) CircuitBreakerOption {
	return func(cb *AdaptiveCircuitBreaker) {
		cb.sleepWindow = d
	}
}

// WithWindow 设置统计窗口大小
func WithWindow(d time.Duration) CircuitBreakerOption {
	return func(cb *AdaptiveCircuitBreaker) {
		cb.windowSize = d
	}
}

// OnStateChange 注册状态变化回调
func (cb *AdaptiveCircuitBreaker) OnStateChange(fn func(oldState, newState State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = append(cb.onStateChange, fn)
}

// Allow 检查是否允许请求
func (cb *AdaptiveCircuitBreaker) Allow() error {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case Open:
		// 熔断状态，检查是否过了等待时间
		if time.Since(cb.lastChange) > cb.sleepWindow {
			return nil // 允许一个请求试探
		}
		return ErrOpen

	case HalfOpen:
		// 半开状态，限制请求数
		if cb.consecutiveFailures < cb.halfOpenMaxReq {
			return nil
		}
		return ErrOpen

	case Closed:
		// 关闭状态，检查失败率
		if cb.totalRequests > 0 {
			failureRate := float64(cb.totalFailures) / float64(cb.totalRequests)
			if failureRate > 0.5 { // 失败率超过 50%
				return ErrOpen
			}
		}
		return nil
	}

	return nil
}

// MarkSuccess 标记成功
func (cb *AdaptiveCircuitBreaker) MarkSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++
	cb.totalSuccesses++
	cb.consecutiveFailures = 0

	// 半开状态下，成功一次就关闭熔断器
	if cb.state == HalfOpen {
		cb.changeState(Closed)
	}
}

// MarkFailure 标记失败
func (cb *AdaptiveCircuitBreaker) MarkFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++
	cb.totalFailures++
	cb.consecutiveFailures++
	cb.lastFailureTime = time.Now()

	// 检查是否需要打开熔断器
	if cb.shouldTrip() {
		cb.changeState(Open)
	}
}

// shouldTrip 判断是否应该触发熔断
func (cb *AdaptiveCircuitBreaker) shouldTrip() bool {
	// 连续失败次数超过阈值
	if cb.consecutiveFailures >= cb.maxFailures {
		return true
	}

	// 失败率超过 50%，且请求数达到最小阈值
	if cb.totalRequests >= 10 {
		failureRate := float64(cb.totalFailures) / float64(cb.totalRequests)
		if failureRate > 0.5 {
			return true
		}
	}

	return false
}

// changeState 改变状态
func (cb *AdaptiveCircuitBreaker) changeState(newState State) {
	oldState := cb.state
	if oldState == newState {
		return
	}

	cb.state = newState
	cb.lastChange = time.Now()

	// 触发回调
	for _, fn := range cb.onStateChange {
		fn(oldState, newState)
	}

	// 如果切换到 Open 状态，设置定时器自动切换到 HalfOpen
	if newState == Open {
		go func() {
			time.Sleep(cb.sleepWindow)
			cb.mu.Lock()
			if cb.state == Open {
				cb.state = HalfOpen
				cb.lastChange = time.Now()
				for _, fn := range cb.onStateChange {
					fn(Open, HalfOpen)
				}
			}
			cb.mu.Unlock()
		}()
	}
}

// autoAdjust 自适应调整参数
func (cb *AdaptiveCircuitBreaker) autoAdjust() {
	ticker := time.NewTicker(cb.windowSize)
	defer ticker.Stop()

	for range ticker.C {
		cb.mu.Lock()

		// 根据最近的成功率动态调整 maxRT
		if cb.totalSuccesses > 0 {
			successRate := float64(cb.totalSuccesses) / float64(cb.totalRequests)
			if successRate > 0.95 {
				// 成功率高，降低要求
				cb.maxRT = cb.maxRT * 110 / 100
			} else if successRate < 0.8 {
				// 成功率低，提高要求
				cb.maxRT = cb.maxRT * 90 / 100
			}
		}

		// 重置统计
		cb.totalRequests = 0
		cb.totalSuccesses = 0
		cb.totalFailures = 0

		cb.mu.Unlock()
	}
}

// State 获取当前状态
func (cb *AdaptiveCircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats 获取统计信息
func (cb *AdaptiveCircuitBreaker) Stats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":                cb.state,
		"total_requests":       cb.totalRequests,
		"total_successes":      cb.totalSuccesses,
		"total_failures":       cb.totalFailures,
		"consecutive_failures": cb.consecutiveFailures,
		"failure_rate":         float64(cb.totalFailures) / float64(cb.totalRequests),
		"max_rt":               cb.maxRT,
		"last_failure_time":    cb.lastFailureTime,
	}
}

// ErrOpen 熔断器打开错误
var ErrOpen = errors.New("circuit breaker is open")

// Do 执行函数（带熔断保护）
func (cb *AdaptiveCircuitBreaker) Do(fn func() error) error {
	if err := cb.Allow(); err != nil {
		return err
	}

	start := time.Now()
	err := fn()
	elapsed := time.Since(start)

	// 检查响应时间
	if elapsed > cb.maxRT {
		cb.MarkFailure()
		return ErrTimeout
	}

	if err != nil {
		cb.MarkFailure()
	} else {
		cb.MarkSuccess()
	}

	return err
}

// ErrTimeout 超时错误
var ErrTimeout = errors.New("request timeout")
