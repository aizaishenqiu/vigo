package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// DistributedLimiter 分布式限流器（基于 Redis）
type DistributedLimiter struct {
	client   *redis.Client
	key      string
	rate     int           // 每秒允许的请求数
	burst    int           // 突发容量
	interval time.Duration // 时间间隔
}

// DistributedLimiterOptions 分布式限流器选项
type DistributedLimiterOptions struct {
	// Redis 客户端
	Client *redis.Client `yaml:"client"`
	// 键前缀
	Prefix string `yaml:"prefix"`
	// 限流键名
	Key string `yaml:"key"`
	// 速率（每秒请求数）
	Rate int `yaml:"rate"`
	// 突发容量
	Burst int `yaml:"burst"`
	// 时间间隔
	Interval time.Duration `yaml:"interval"`
}

// NewDistributedLimiter 创建分布式限流器
func NewDistributedLimiter(opts *DistributedLimiterOptions) (*DistributedLimiter, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if opts.Client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	if opts.Key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	if opts.Rate <= 0 {
		opts.Rate = 100
	}

	if opts.Burst <= 0 {
		opts.Burst = opts.Rate * 2
	}

	if opts.Interval == 0 {
		opts.Interval = time.Second
	}

	prefix := opts.Prefix
	if prefix == "" {
		prefix = "limiter:"
	}

	return &DistributedLimiter{
		client:   opts.Client,
		key:      prefix + opts.Key,
		rate:     opts.Rate,
		burst:    opts.Burst,
		interval: opts.Interval,
	}, nil
}

// Allow 检查是否允许请求（令牌桶算法）
func (l *DistributedLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN 检查是否允许 N 个请求
func (l *DistributedLimiter) AllowN(n int) bool {
	ctx := context.Background()

	// Lua 脚本实现原子限流
	script := redis.NewScript(`
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local burst = tonumber(ARGV[2])
		local interval = tonumber(ARGV[3])
		local now = tonumber(ARGV[4])
		local n = tonumber(ARGV[5])

		-- 获取当前桶状态
		local bucket = redis.call('HMGET', key, 'tokens', 'last_time')
		local tokens = tonumber(bucket[1])
		local last_time = tonumber(bucket[2])

		-- 如果桶不存在，初始化
		if tokens == nil then
			tokens = burst
			last_time = now
		end

		-- 计算新增令牌数
		local elapsed = (now - last_time) / 1000
		local new_tokens = math.min(burst, tokens + elapsed * rate)

		-- 检查是否有足够的令牌
		local allowed = 0
		if new_tokens >= n then
			new_tokens = new_tokens - n
			allowed = 1
		end

		-- 更新桶状态
		redis.call('HMSET', key, 'tokens', new_tokens, 'last_time', now)
		redis.call('PEXPIRE', key, interval * 1000)

		return allowed
	`)

	now := time.Now().UnixMilli()
	result, err := script.Run(ctx, l.client, []string{l.key}, l.rate, l.burst, int(l.interval.Seconds()), now, n).Int()
	if err != nil {
		// Redis 错误时，默认允许（fail-open）
		return true
	}

	return result == 1
}

// Wait 等待直到获得令牌
func (l *DistributedLimiter) Wait(ctx context.Context) error {
	return l.WaitN(ctx, 1)
}

// WaitN 等待直到获得 N 个令牌
func (l *DistributedLimiter) WaitN(ctx context.Context, n int) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if l.AllowN(n) {
				return nil
			}
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// Reserve 预留令牌
func (l *DistributedLimiter) Reserve(n int) (time.Duration, error) {
	if l.AllowN(n) {
		return 0, nil
	}

	// 计算需要等待的时间
	ctx := context.Background()
	tokens, err := l.client.HGet(ctx, l.key, "tokens").Int()
	if err != nil {
		return 0, err
	}

	// 等待时间 = (需要的令牌数 - 当前令牌数) / 速率
	waitTime := time.Duration((n-tokens)*1000/l.rate) * time.Millisecond
	return waitTime, nil
}

// GetTokens 获取当前令牌数
func (l *DistributedLimiter) GetTokens() (int, error) {
	ctx := context.Background()
	tokens, err := l.client.HGet(ctx, l.key, "tokens").Int()
	if err != nil {
		if err == redis.Nil {
			return l.burst, nil
		}
		return 0, err
	}
	return tokens, nil
}

// Reset 重置限流器
func (l *DistributedLimiter) Reset() error {
	ctx := context.Background()
	return l.client.Del(ctx, l.key).Err()
}

// GetState 获取限流器状态
func (l *DistributedLimiter) GetState() (*LimiterState, error) {
	ctx := context.Background()

	bucket, err := l.client.HMGet(ctx, l.key, "tokens", "last_time").Result()
	if err != nil {
		return nil, err
	}

	tokens := l.burst
	if bucket[0] != nil {
		if t, ok := bucket[0].(string); ok {
			if parsed, err := time.ParseDuration(t + "s"); err == nil {
				tokens = int(parsed.Seconds())
			}
		}
	}

	return &LimiterState{
		Tokens:     tokens,
		Rate:       l.rate,
		Burst:      l.burst,
		LastUpdate: time.Now(),
	}, nil
}

// LimiterState 限流器状态
type LimiterState struct {
	Tokens     int
	Rate       int
	Burst      int
	LastUpdate time.Time
}

// DistributedRateLimiter 分布式速率限流器（滑动窗口算法）
type DistributedRateLimiter struct {
	client    *redis.Client
	key       string
	rate      int
	window    time.Duration
	precision time.Duration
}

// DistributedRateLimiterOptions 分布式速率限流器选项
type DistributedRateLimiterOptions struct {
	// Redis 客户端
	Client *redis.Client `yaml:"client"`
	// 键前缀
	Prefix string `yaml:"prefix"`
	// 限流键名
	Key string `yaml:"key"`
	// 速率（窗口内允许的请求数）
	Rate int `yaml:"rate"`
	// 窗口大小
	Window time.Duration `yaml:"window"`
	// 精度（窗口分片数）
	Precision time.Duration `yaml:"precision"`
}

// NewDistributedRateLimiter 创建分布式速率限流器
func NewDistributedRateLimiter(opts *DistributedRateLimiterOptions) (*DistributedRateLimiter, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	if opts.Client == nil {
		return nil, fmt.Errorf("redis client cannot be nil")
	}

	if opts.Key == "" {
		return nil, fmt.Errorf("key cannot be empty")
	}

	if opts.Rate <= 0 {
		opts.Rate = 100
	}

	if opts.Window == 0 {
		opts.Window = time.Second
	}

	if opts.Precision == 0 {
		opts.Precision = 100 * time.Millisecond
	}

	prefix := opts.Prefix
	if prefix == "" {
		prefix = "rate_limiter:"
	}

	return &DistributedRateLimiter{
		client:    opts.Client,
		key:       prefix + opts.Key,
		rate:      opts.Rate,
		window:    opts.Window,
		precision: opts.Precision,
	}, nil
}

// Allow 检查是否允许请求（滑动窗口算法）
func (l *DistributedRateLimiter) Allow() bool {
	ctx := context.Background()
	now := time.Now()

	// 计算当前窗口的键
	_ = fmt.Sprintf("%s:%d", l.key, now.UnixNano()/int64(l.precision))

	// Lua 脚本
	script := redis.NewScript(`
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local precision = tonumber(ARGV[4])

		-- 计算窗口索引
		local window_index = math.floor(now / precision)
		local current_key = key .. ':' .. window_index

		-- 获取所有相关窗口的计数
		local count = 0
		local windows = math.ceil(window / precision)

		for i = 0, windows - 1 do
			local k = key .. ':' .. (window_index - i)
			count = count + tonumber(redis.call('GET', k) or '0')
		end

		-- 检查是否超过限制
		if count >= rate then
			return 0
		end

		-- 增加计数
		redis.call('INCR', current_key)
		redis.call('PEXPIRE', current_key, window)

		return 1
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.rate, int(l.window.Milliseconds()), now.UnixNano()/int64(time.Millisecond), int(l.precision.Milliseconds())).Int()
	if err != nil {
		return true // fail-open
	}

	return result == 1
}

// AllowN 检查是否允许 N 个请求
func (l *DistributedRateLimiter) AllowN(n int) bool {
	ctx := context.Background()
	now := time.Now()
	_ = fmt.Sprintf("%s:%d", l.key, now.UnixNano()/int64(l.precision))

	script := redis.NewScript(`
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local window = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local precision = tonumber(ARGV[4])
		local n = tonumber(ARGV[5])

		local window_index = math.floor(now / precision)
		local current_key = key .. ':' .. window_index

		local count = 0
		local windows = math.ceil(window / precision)

		for i = 0, windows - 1 do
			local k = key .. ':' .. (window_index - i)
			count = count + tonumber(redis.call('GET', k) or '0')
		end

		if count + n > rate then
			return 0
		end

		redis.call('INCRBY', current_key, n)
		redis.call('PEXPIRE', current_key, window)

		return 1
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.rate, int(l.window.Milliseconds()), now.UnixNano()/int64(time.Millisecond), int(l.precision.Milliseconds()), n).Int()
	if err != nil {
		return true
	}

	return result == 1
}
