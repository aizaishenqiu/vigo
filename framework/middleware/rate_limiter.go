package middleware

import (
	"net/http"
	"sync"
	"time"

	"vigo/framework/mvc"
)

// RateLimiter 限流器
type RateLimiter struct {
	tokens   chan struct{}
	refill   *time.Ticker
	mu       sync.Mutex
	capacity int
}

// NewRateLimiter 创建新的限流器
func NewRateLimiter(rate int) *RateLimiter {
	limiter := &RateLimiter{
		tokens:   make(chan struct{}, rate),
		capacity: rate,
		refill:   time.NewTicker(time.Second),
	}

	// 预填充令牌桶
	for i := 0; i < rate; i++ {
		limiter.tokens <- struct{}{}
	}

	// 启动令牌桶填充
	go func() {
		for range limiter.refill.C {
			limiter.mu.Lock()
			// 填充令牌直到达到容量
		fillLoop:
			for len(limiter.tokens) < limiter.capacity {
				select {
				case limiter.tokens <- struct{}{}:
				default:
					// 通道已满，跳出循环
					break fillLoop
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

// Allow 检查是否允许请求
func (rl *RateLimiter) Allow() bool {
	select {
	case <-rl.tokens:
		return true
	default:
		return false
	}
}

// RateLimitMiddleware 限流中间件
func RateLimitMiddleware(rate int) mvc.HandlerFunc {
	limiter := NewRateLimiter(rate)

	return func(c *mvc.Context) {
		if !limiter.Allow() {
			c.Error(http.StatusTooManyRequests, "Rate limit exceeded")
			c.Abort()
			return
		}

		c.Next()
	}
}

// IPBasedRateLimiter 基于IP的限流器
type IPBasedRateLimiter struct {
	limits map[string]*RateLimiter
	mu     sync.RWMutex
	rate   int
}

// NewIPBasedRateLimiter 创建基于IP的限流器
func NewIPBasedRateLimiter(rate int) *IPBasedRateLimiter {
	limiter := &IPBasedRateLimiter{
		limits: make(map[string]*RateLimiter),
		rate:   rate,
	}

	// 定期清理不再活跃的IP限流器
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			limiter.mu.Lock()
			// 清理长时间未使用的IP限流器
			for ip, rl := range limiter.limits {
				// 如果令牌桶已满，说明该IP近期没有请求
				if len(rl.tokens) == cap(rl.tokens) {
					delete(limiter.limits, ip)
				}
			}
			limiter.mu.Unlock()
		}
	}()

	return limiter
}

// Allow 检查指定IP是否允许请求
func (ipl *IPBasedRateLimiter) Allow(ip string) bool {
	ipl.mu.RLock()
	limiter, exists := ipl.limits[ip]
	ipl.mu.RUnlock()

	if !exists {
		ipl.mu.Lock()
		// 双重检查
		if limiter, exists = ipl.limits[ip]; !exists {
			limiter = NewRateLimiter(ipl.rate)
			ipl.limits[ip] = limiter
		}
		ipl.mu.Unlock()
	}

	return limiter.Allow()
}

// IPBasedRateLimitMiddleware 基于IP的限流中间件
func IPBasedRateLimitMiddleware(rate int) mvc.HandlerFunc {
	limiter := NewIPBasedRateLimiter(rate)

	return func(c *mvc.Context) {
		clientIP := c.GetClientIP()
		if !limiter.Allow(clientIP) {
			c.Error(http.StatusTooManyRequests, "Rate limit exceeded for IP: "+clientIP)
			c.Abort()
			return
		}

		c.Next()
	}
}