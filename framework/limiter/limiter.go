package limiter

import (
	"sync"
	"time"
)

// Strategy 限流策略
type Strategy int

const (
	TokenBucket Strategy = iota // 令牌桶
	LeakyBucket               // 漏桶
	SlidingWindow             // 滑动窗口
	FixedWindow               // 固定窗口
)

// RateLimiter 限流器接口
type RateLimiter interface {
	Allow() bool
	AllowN(n int) bool
	Wait() bool
	WaitN(n int) bool
}

// TokenBucketLimiter 令牌桶限流器
type TokenBucketLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // 每秒补充的令牌数
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucketLimiter 创建令牌桶限流器
func NewTokenBucketLimiter(rate int, burst int) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: float64(rate),
		lastRefill: time.Now(),
	}
}

// Allow 检查是否允许请求
func (l *TokenBucketLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许 n 个请求
func (l *TokenBucketLimiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastRefill = now

	// 检查是否有足够令牌
	if l.tokens >= float64(n) {
		l.tokens -= float64(n)
		return true
	}

	return false
}

// Wait 等待直到有令牌
func (l *TokenBucketLimiter) Wait() bool {
	return l.WaitN(1)
}

// WaitN 等待直到有 n 个令牌
func (l *TokenBucketLimiter) WaitN(n int) bool {
	for i := 0; i < 100; i++ { // 最多重试 100 次
		if l.AllowN(n) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// LeakyBucketLimiter 漏桶限流器
type LeakyBucketLimiter struct {
	capacity   int
	water      int
	lastLeak   time.Time
	leakRate   int // 每秒漏出的水量
	mu         sync.Mutex
}

// NewLeakyBucketLimiter 创建漏桶限流器
func NewLeakyBucketLimiter(capacity int, rate int) *LeakyBucketLimiter {
	return &LeakyBucketLimiter{
		capacity: capacity,
		water:    0,
		lastLeak: time.Now(),
		leakRate: rate,
	}
}

// Allow 检查是否允许请求
func (l *LeakyBucketLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许 n 个请求
func (l *LeakyBucketLimiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 漏水
	now := time.Now()
	elapsed := now.Sub(l.lastLeak).Seconds()
	leaked := int(elapsed * float64(l.leakRate))
	if leaked > l.water {
		leaked = l.water
	}
	l.water -= leaked
	l.lastLeak = now

	// 检查是否有空间
	if l.water+n <= l.capacity {
		l.water += n
		return true
	}

	return false
}

// Wait 等待直到有空间
func (l *LeakyBucketLimiter) Wait() bool {
	return l.WaitN(1)
}

// WaitN 等待直到有 n 个空间
func (l *LeakyBucketLimiter) WaitN(n int) bool {
	for i := 0; i < 100; i++ {
		if l.AllowN(n) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// SlidingWindowLimiter 滑动窗口限流器
type SlidingWindowLimiter struct {
	windowSize  time.Duration
	maxRequests int
	requests    []time.Time
	mu          sync.Mutex
}

// NewSlidingWindowLimiter 创建滑动窗口限流器
func NewSlidingWindowLimiter(maxRequests int, windowSize time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		windowSize:  windowSize,
		maxRequests: maxRequests,
		requests:    make([]time.Time, 0),
	}
}

// Allow 检查是否允许请求
func (l *SlidingWindowLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许 n 个请求
func (l *SlidingWindowLimiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.windowSize)

	// 移除窗口外的请求
	validRequests := make([]time.Time, 0)
	for _, t := range l.requests {
		if t.After(cutoff) {
			validRequests = append(validRequests, t)
		}
	}
	l.requests = validRequests

	// 检查是否超过限制
	if len(l.requests)+n <= l.maxRequests {
		for i := 0; i < n; i++ {
			l.requests = append(l.requests, now)
		}
		return true
	}

	return false
}

// Wait 等待直到允许请求
func (l *SlidingWindowLimiter) Wait() bool {
	return l.WaitN(1)
}

// WaitN 等待直到允许 n 个请求
func (l *SlidingWindowLimiter) WaitN(n int) bool {
	for i := 0; i < 100; i++ {
		if l.AllowN(n) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// FixedWindowLimiter 固定窗口限流器
type FixedWindowLimiter struct {
	windowSize  time.Duration
	maxRequests int
	windowStart time.Time
	count       int
	mu          sync.Mutex
}

// NewFixedWindowLimiter 创建固定窗口限流器
func NewFixedWindowLimiter(maxRequests int, windowSize time.Duration) *FixedWindowLimiter {
	return &FixedWindowLimiter{
		windowSize:  windowSize,
		maxRequests: maxRequests,
		windowStart: time.Now(),
		count:       0,
	}
}

// Allow 检查是否允许请求
func (l *FixedWindowLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许 n 个请求
func (l *FixedWindowLimiter) AllowN(n int) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// 检查是否需要重置窗口
	if now.Sub(l.windowStart) > l.windowSize {
		l.windowStart = now
		l.count = 0
	}

	// 检查是否超过限制
	if l.count+n <= l.maxRequests {
		l.count += n
		return true
	}

	return false
}

// Wait 等待直到允许请求
func (l *FixedWindowLimiter) Wait() bool {
	return l.WaitN(1)
}

// WaitN 等待直到允许 n 个请求
func (l *FixedWindowLimiter) WaitN(n int) bool {
	for i := 0; i < 100; i++ {
		if l.AllowN(n) {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// CreateLimiter 根据策略创建限流器
func CreateLimiter(strategy Strategy, rate int, burst int, windowSize time.Duration) RateLimiter {
	switch strategy {
	case TokenBucket:
		return NewTokenBucketLimiter(rate, burst)
	case LeakyBucket:
		return NewLeakyBucketLimiter(burst, rate)
	case SlidingWindow:
		return NewSlidingWindowLimiter(rate, windowSize)
	case FixedWindow:
		return NewFixedWindowLimiter(rate, windowSize)
	default:
		return NewTokenBucketLimiter(rate, burst)
	}
}
