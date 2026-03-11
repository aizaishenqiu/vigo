package cache

import (
	"sync"
	"time"
)

// MultiLevelCache 实现多级缓存
type MultiLevelCache struct {
	memory *MemoryCache
	redis  *RedisCache
	ttl    time.Duration
	mutex  sync.Mutex // 用于防止缓存击穿
}

// RedisCache 是一个接口，代表 Redis 缓存实现
type RedisCache struct {
	// 这里应该包含 Redis 客户端的实际实现
	// 由于我们不能直接引用 Redis 客户端，使用一个简单的模拟实现
	data map[string]interface{}
	ttls map[string]time.Time
	mu   sync.RWMutex
}

// NewRedisCache 创建新的 Redis 缓存实例
func NewRedisCache() *RedisCache {
	rc := &RedisCache{
		data: make(map[string]interface{}),
		ttls: make(map[string]time.Time),
	}

	// 启动 TTL 清理协程
	go rc.cleanupExpired()

	return rc
}

// cleanupExpired 清理过期的键
func (rc *RedisCache) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rc.mu.Lock()
		now := time.Now()
		for key, expiry := range rc.ttls {
			if now.After(expiry) {
				delete(rc.data, key)
				delete(rc.ttls, key)
			}
		}
		rc.mu.Unlock()
	}
}

// Get 从 Redis 缓存获取值
func (rc *RedisCache) Get(key string) interface{} {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if expiry, exists := rc.ttls[key]; exists {
		if time.Now().After(expiry) {
			// 键已过期
			return nil
		}
	}

	return rc.data[key]
}

// Set 在 Redis 缓存中设置值
func (rc *RedisCache) Set(key string, val interface{}, ttl time.Duration) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.data[key] = val
	rc.ttls[key] = time.Now().Add(ttl)

	return nil
}

// Delete 从 Redis 缓存删除值
func (rc *RedisCache) Delete(key string) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	delete(rc.data, key)
	delete(rc.ttls, key)

	return nil
}

// Has 检查 Redis 缓存中是否存在键
func (rc *RedisCache) Has(key string) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if expiry, exists := rc.ttls[key]; exists {
		if time.Now().After(expiry) {
			return false
		}
	}

	_, exists := rc.data[key]
	return exists
}

// SetNX 设置键值，仅当键不存在时
func (rc *RedisCache) SetNX(key string, val interface{}, ttl time.Duration) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if _, exists := rc.data[key]; exists {
		return false
	}

	rc.data[key] = val
	rc.ttls[key] = time.Now().Add(ttl)

	return true
}

// NewMultiLevelCache 创建多级缓存实例
func NewMultiLevelCache(memorySize int, ttl time.Duration) *MultiLevelCache {
	return &MultiLevelCache{
		memory: NewMemoryCache(memorySize),
		redis:  NewRedisCache(),
		ttl:    ttl,
	}
}

// GetWithProtection 实现缓存穿透防护
func (mc *MultiLevelCache) GetWithProtection(key string, fetchFunc func() (interface{}, error)) (interface{}, error) {
	// 1. 先查内存缓存
	if val := mc.memory.Get(key); val != nil {
		return val, nil
	}

	// 2. 再查 Redis 缓存
	if val := mc.redis.Get(key); val != nil {
		// 同步到内存缓存
		mc.memory.Set(key, val, mc.ttl)
		return val, nil
	}

	// 3. 使用互斥锁防止缓存击穿
	mc.mutex.Lock()
	defer mc.mutex.Unlock()

	// 双重检查，防止并发问题
	if val := mc.memory.Get(key); val != nil {
		return val, nil
	}

	if val := mc.redis.Get(key); val != nil {
		mc.memory.Set(key, val, mc.ttl)
		return val, nil
	}

	// 4. 调用 fetchFunc 获取数据
	val, err := fetchFunc()
	if err != nil {
		// 如果数据库也查不到，缓存空值防止缓存穿透
		// 但空值的 TTL 应该短一些
		mc.memory.Set(key, nil, 5*time.Minute)
		mc.redis.Set(key, nil, 5*time.Minute)
		return nil, err
	}

	// 5. 将数据写入各级缓存
	if val != nil {
		mc.memory.Set(key, val, mc.ttl)
		mc.redis.Set(key, val, mc.ttl)
	}

	return val, nil
}

// Get 从多级缓存获取值
func (mc *MultiLevelCache) Get(key string) interface{} {
	// 先查内存缓存
	if val := mc.memory.Get(key); val != nil {
		return val
	}

	// 再查 Redis 缓存
	if val := mc.redis.Get(key); val != nil {
		// 同步到内存缓存
		mc.memory.Set(key, val, mc.ttl)
		return val
	}

	return nil
}

// Set 在多级缓存中设置值
func (mc *MultiLevelCache) Set(key string, val interface{}, ttl time.Duration) error {
	// 同时设置到内存和 Redis
	mc.memory.Set(key, val, ttl)
	mc.redis.Set(key, val, ttl)
	return nil
}

// Delete 从多级缓存删除值
func (mc *MultiLevelCache) Delete(key string) error {
	// 同时从内存和 Redis 删除
	mc.memory.Delete(key)
	mc.redis.Delete(key)
	return nil
}

// Has 检查多级缓存中是否存在键
func (mc *MultiLevelCache) Has(key string) bool {
	return mc.memory.Has(key) || mc.redis.Has(key)
}

// SetNX 设置键值，仅当键不存在时（主要用于分布式锁）
func (mc *MultiLevelCache) SetNX(key string, val interface{}, ttl time.Duration) bool {
	// 先尝试在内存中设置
	memHas := mc.memory.Has(key)
	if !memHas {
		mc.memory.Set(key, val, ttl)
		mc.redis.Set(key, val, ttl)
		return true
	}

	// 如果内存中已有，则检查 Redis
	return mc.redis.SetNX(key, val, ttl)
}

// Clear 清空所有缓存
func (mc *MultiLevelCache) Clear() {
	mc.memory.Flush()
	// 重新创建 Redis 缓存实例来清空
	mc.redis = NewRedisCache()
}
