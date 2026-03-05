package middleware

import (
	"context"
	"sync"
	"time"

	"vigo/framework/mvc"
)

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	// 失败阈值（连续失败多少次后打开熔断器）
	FailureThreshold int `yaml:"failure_threshold"`
	// 成功阈值（半开状态下连续成功多少次后关闭熔断器）
	SuccessThreshold int `yaml:"success_threshold"`
	// 超时时间
	Timeout time.Duration `yaml:"timeout"`
	// 熔断器打开后的休眠时间
	SleepWindow time.Duration `yaml:"sleep_window"`
	// 是否启用
	Enabled bool `yaml:"enabled"`
	// 熔断器打开时的处理
	OnOpen func(ctx *mvc.Context) `yaml:"-"`
}

// CircuitBreaker 熔断器中间件
func CircuitBreaker(cfg *CircuitBreakerConfig) mvc.HandlerFunc {
	if cfg == nil {
		cfg = &CircuitBreakerConfig{}
	}

	if cfg.FailureThreshold == 0 {
		cfg.FailureThreshold = 5
	}

	if cfg.SuccessThreshold == 0 {
		cfg.SuccessThreshold = 3
	}

	if cfg.SleepWindow == 0 {
		cfg.SleepWindow = 30 * time.Second
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	if !cfg.Enabled {
		return func(ctx *mvc.Context) {
			ctx.Next()
		}
	}

	// 熔断器状态
	var (
		failures    int
		successes   int
		lastFailure time.Time
		state       = "closed" // closed, open, half-open
		stateMu     sync.RWMutex
	)

	return func(ctx *mvc.Context) {
		stateMu.RLock()
		currentState := state
		stateMu.RUnlock()

		// 检查熔断器状态
		if currentState == "open" {
			// 检查是否可以进入半开状态
			if time.Since(lastFailure) > cfg.SleepWindow {
				stateMu.Lock()
				state = "half-open"
				stateMu.Unlock()
			} else {
				// 熔断器打开，拒绝请求
				if cfg.OnOpen != nil {
					cfg.OnOpen(ctx)
					return
				}
				ctx.Json(503, map[string]string{
					"error": "Service Unavailable",
				})
				return
			}
		}

		// 执行请求
		start := time.Now()

		// 使用 context 控制超时
		ctxWithTimeout, cancel := context.WithTimeout(ctx.Request.Context(), cfg.Timeout)
		defer cancel()

		done := make(chan struct{}, 1)
		go func() {
			ctx.Next()
			done <- struct{}{}
		}()

		select {
		case <-done:
			// 请求完成
			_ = time.Since(start)

			stateMu.Lock()
			if ctx.Status() >= 500 {
				// 服务器错误，记录失败
				failures++
				successes = 0
				if failures >= cfg.FailureThreshold {
					state = "open"
					lastFailure = time.Now()
				}
			} else {
				// 成功
				if state == "half-open" {
					successes++
					if successes >= cfg.SuccessThreshold {
						// 关闭熔断器
						state = "closed"
						failures = 0
						successes = 0
					}
				} else {
					// 重置计数器
					failures = 0
					successes = 0
				}
			}
			stateMu.Unlock()

		case <-ctxWithTimeout.Done():
			// 超时
			stateMu.Lock()
			failures++
			if failures >= cfg.FailureThreshold {
				state = "open"
				lastFailure = time.Now()
			}
			stateMu.Unlock()

			ctx.Json(504, map[string]string{
				"error": "Gateway Timeout",
			})
		}
	}
}

// FallbackConfig 降级配置
type FallbackConfig struct {
	// 是否启用降级
	Enabled bool `yaml:"enabled"`
	// 降级处理函数
	Fallback func(ctx *mvc.Context) `yaml:"-"`
	// 降级条件（满足条件时触发降级）
	Condition func(ctx *mvc.Context) bool `yaml:"-"`
}

// Fallback 降级中间件
func Fallback(cfg *FallbackConfig) mvc.HandlerFunc {
	if cfg == nil {
		cfg = &FallbackConfig{}
	}

	if !cfg.Enabled || cfg.Fallback == nil {
		return func(ctx *mvc.Context) {
			ctx.Next()
		}
	}

	return func(ctx *mvc.Context) {
		ctx.Next()

		// 检查是否需要降级
		shouldFallback := false
		if cfg.Condition != nil {
			shouldFallback = cfg.Condition(ctx)
		} else if ctx.Status() >= 500 {
			shouldFallback = true
		}

		if shouldFallback {
			cfg.Fallback(ctx)
		}
	}
}

// RetryConfig 重试配置
type RetryConfig struct {
	// 最大重试次数
	MaxRetries int `yaml:"max_retries"`
	// 重试间隔
	Interval time.Duration `yaml:"interval"`
	// 重试条件
	Condition func(ctx *mvc.Context) bool `yaml:"-"`
	// 重试失败处理
	OnMaxRetriesExceeded func(ctx *mvc.Context) `yaml:"-"`
}

// Retry 重试中间件
func Retry(cfg *RetryConfig) mvc.HandlerFunc {
	if cfg == nil {
		cfg = &RetryConfig{}
	}

	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}

	if cfg.Interval == 0 {
		cfg.Interval = 100 * time.Millisecond
	}

	return func(ctx *mvc.Context) {
		for i := 0; i <= cfg.MaxRetries; i++ {
			// 执行请求
			ctx.Next()

			// 检查是否需要重试
			shouldRetry := false
			if cfg.Condition != nil {
				shouldRetry = cfg.Condition(ctx)
			} else if ctx.Status() >= 500 {
				shouldRetry = true
			}

			if !shouldRetry || i == cfg.MaxRetries {
				// 不需要重试或已达到最大重试次数
				if i == cfg.MaxRetries && shouldRetry {
					if cfg.OnMaxRetriesExceeded != nil {
						cfg.OnMaxRetriesExceeded(ctx)
					}
				}
				return
			}

			// 等待后重试
			time.Sleep(cfg.Interval)
		}
	}
}

// TimeoutConfig 超时配置
type TimeoutConfig struct {
	// 超时时间
	Timeout time.Duration `yaml:"timeout"`
	// 超时处理函数
	OnTimeout func(ctx *mvc.Context) `yaml:"-"`
}

// Timeout 超时中间件
func Timeout(cfg *TimeoutConfig) mvc.HandlerFunc {
	if cfg == nil {
		cfg = &TimeoutConfig{}
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return func(ctx *mvc.Context) {
		// 创建带超时的 context
		ctxWithTimeout, cancel := context.WithTimeout(ctx.Request.Context(), cfg.Timeout)
		defer cancel()

		// 使用 channel 接收结果
		done := make(chan struct{}, 1)
		go func() {
			ctx.Next()
			done <- struct{}{}
		}()

		select {
		case <-done:
			// 正常完成
			return
		case <-ctxWithTimeout.Done():
			// 超时
			if cfg.OnTimeout != nil {
				cfg.OnTimeout(ctx)
				return
			}
			ctx.Json(504, map[string]string{
				"error": "Gateway Timeout",
			})
		}
	}
}
