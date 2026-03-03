package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisLock Redis 分布式锁
type RedisLock struct {
	client   *redis.Client
	key      string
	value    string
	ttl      time.Duration
	retry    int
	interval time.Duration
}

// LockConfig 锁配置
type LockConfig struct {
	TTL      time.Duration // 锁超时时间
	Retry    int           // 重试次数
	Interval time.Duration // 重试间隔
}

// DefaultConfig 默认配置
var DefaultConfig = LockConfig{
	TTL:      5 * time.Second,
	Retry:    3,
	Interval: 100 * time.Millisecond,
}

// NewRedisLock 创建分布式锁
// 参数:
//   - client: Redis 客户端
//   - key: 锁键（会自动添加 "lock:" 前缀）
//   - config: 锁配置（可选）
//
// 示例:
//
//	lock := lock.NewRedisLock(redisClient, "product:1001:stock")
//	if lock.Lock(ctx) {
//	    defer lock.Unlock(ctx)
//	    // 执行业务逻辑
//	}
func NewRedisLock(client *redis.Client, key string, config ...LockConfig) *RedisLock {
	cfg := DefaultConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return &RedisLock{
		client:   client,
		key:      "lock:" + key,
		value:    uuid.New().String(),
		ttl:      cfg.TTL,
		retry:    cfg.Retry,
		interval: cfg.Interval,
	}
}

// Lock 尝试获取锁
// 返回：是否成功获取锁
func (l *RedisLock) Lock(ctx context.Context) bool {
	for i := 0; i < l.retry; i++ {
		success, _ := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
		if success {
			return true
		}
		time.Sleep(l.interval)
	}
	return false
}

// LockWithContextTimeout 带超时的锁（自动重试直到超时）
// 参数:
//   - ctx: 上下文
//   - timeout: 获取锁的最大等待时间
//
// 返回：是否成功获取锁
func (l *RedisLock) LockWithContextTimeout(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return false
		}

		success, _ := l.client.SetNX(ctx, l.key, l.value, l.ttl).Result()
		if success {
			return true
		}
		time.Sleep(l.interval)
	}
	return false
}

// Unlock 释放锁
// 使用 Lua 脚本保证原子性，防止误删其他锁
func (l *RedisLock) Unlock(ctx context.Context) error {
	// Lua 脚本：检查锁是否属于当前客户端，是则删除
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
	`
	_, err := l.client.Eval(ctx, script, []string{l.key}, l.value).Result()
	return err
}

// Extend 延长锁的 TTL（续期）
// 只有当前持有锁的客户端才能续期
func (l *RedisLock) Extend(ctx context.Context, ttl time.Duration) error {
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("pexpire", KEYS[1], ARGV[2])
	else
		return 0
	end
	`
	_, err := l.client.Eval(ctx, script, []string{l.key}, l.value, int(ttl.Milliseconds())).Result()
	if err != nil {
		return err
	}
	return nil
}

// IsLocked 检查锁是否被占用
func (l *RedisLock) IsLocked(ctx context.Context) (bool, error) {
	exists, err := l.client.Exists(ctx, l.key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// GetOwner 获取锁的持有者（用于调试）
func (l *RedisLock) GetOwner(ctx context.Context) (string, error) {
	return l.client.Get(ctx, l.key).Result()
}

// WithLock 使用锁执行闭包（自动获取和释放）
// 参数:
//   - ctx: 上下文
//   - lock: 锁实例
//   - fn: 要执行的函数
//
// 返回：函数执行的错误
//
// 示例:
//
//	err := lock.WithLock(ctx, func() error {
//	    // 检查库存
//	    product := model.New().Table("products").Find(1001)
//	    if product.Stock < 1 {
//	        return fmt.Errorf("库存不足")
//	    }
//	    // 扣减库存
//	    return model.New().Table("products").
//	        Where("id", 1001).
//	        Decrement("stock", 1)
//	})
func WithLock(ctx context.Context, lock *RedisLock, fn func() error) error {
	if !lock.Lock(ctx) {
		return fmt.Errorf("failed to acquire lock")
	}
	defer lock.Unlock(ctx)
	return fn()
}

// ========== 看门狗（自动续期）==========

// Watchdog 看门狗配置
type Watchdog struct {
	lock     *RedisLock
	interval time.Duration
	stopChan chan struct{}
}

// NewWatchdog 创建看门狗（自动续期）
// 参数:
//   - lock: 锁实例
//   - interval: 续期间隔（建议为 TTL 的 1/3）
func NewWatchdog(lock *RedisLock, interval time.Duration) *Watchdog {
	return &Watchdog{
		lock:     lock,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start 启动看门狗
func (w *Watchdog) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// 自动续期
				if err := w.lock.Extend(ctx, w.lock.ttl); err != nil {
					return
				}
			case <-w.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Stop 停止看门狗
func (w *Watchdog) Stop() {
	close(w.stopChan)
}

// WithWatchdogLock 使用看门狗锁执行闭包（自动续期）
// 适合执行时间不确定的长任务
func WithWatchdogLock(ctx context.Context, lock *RedisLock, fn func() error) error {
	if !lock.Lock(ctx) {
		return fmt.Errorf("failed to acquire lock")
	}
	defer lock.Unlock(ctx)

	// 启动看门狗（每 2 秒续期一次）
	watchdog := NewWatchdog(lock, 2*time.Second)
	watchdog.Start(ctx)
	defer watchdog.Stop()

	return fn()
}

// ========== RedLock（多 Redis 实例）==========

// RedLock 多 Redis 实例分布式锁（Redlock 算法）
type RedLock struct {
	clients  []*redis.Client
	key      string
	value    string
	ttl      time.Duration
	retry    int
	interval time.Duration
}

// NewRedLock 创建 RedLock
// 参数:
//   - clients: 多个 Redis 客户端（建议 5 个独立实例）
//   - key: 锁键
//   - config: 锁配置
func NewRedLock(clients []*redis.Client, key string, config ...LockConfig) *RedLock {
	cfg := DefaultConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return &RedLock{
		clients:  clients,
		key:      "lock:" + key,
		value:    uuid.New().String(),
		ttl:      cfg.TTL,
		retry:    cfg.Retry,
		interval: cfg.Interval,
	}
}

// Lock 获取 RedLock（需要获得多数实例的锁）
func (l *RedLock) Lock(ctx context.Context) bool {
	for i := 0; i < l.retry; i++ {
		successCount := 0

		for _, client := range l.clients {
			success, _ := client.SetNX(ctx, l.key, l.value, l.ttl).Result()
			if success {
				successCount++
			}
		}

		// 需要获得多数实例的锁
		if successCount > len(l.clients)/2 {
			return true
		}

		// 释放所有已获得的锁
		l.Unlock(ctx)
		time.Sleep(l.interval)
	}
	return false
}

// Unlock 释放 RedLock（释放所有实例的锁）
func (l *RedLock) Unlock(ctx context.Context) error {
	script := `
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("del", KEYS[1])
	else
		return 0
	end
	`

	for _, client := range l.clients {
		client.Eval(ctx, script, []string{l.key}, l.value)
	}
	return nil
}
