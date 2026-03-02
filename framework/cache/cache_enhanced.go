package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// TaggedCache 带标签的缓存（类似 TP 8.1.4）
type TaggedCache struct {
	cache      *MemoryCache
	tags       map[string]map[string]bool // tag -> keys
	keyTags    map[string]map[string]bool // key -> tags
	mu         sync.RWMutex
	failDelete bool // 类似 TP 8.1.4 的 fail_delete 配置
}

// NewTaggedCache 创建带标签的缓存
func NewTaggedCache(cache *MemoryCache) *TaggedCache {
	tc := &TaggedCache{
		cache:      cache,
		tags:       make(map[string]map[string]bool),
		keyTags:    make(map[string]map[string]bool),
		failDelete: false,
	}

	// 启动后台清理
	go tc.cleanupLoop(60 * time.Second)

	return tc
}

// SetFailDelete 设置 fail_delete 配置（类似 TP 8.1.4）
func (tc *TaggedCache) SetFailDelete(enabled bool) {
	tc.failDelete = enabled
}

// Set 设置缓存（支持标签）
func (tc *TaggedCache) Set(key string, value interface{}, ttl time.Duration, tags ...string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 设置缓存
	if err := tc.cache.Set(key, value, ttl); err != nil {
		// fail_delete: 异常时强制删除缓存
		if tc.failDelete {
			tc.cache.Delete(key)
		}
		return err
	}

	// 记录标签关系
	if len(tags) > 0 {
		if tc.keyTags[key] == nil {
			tc.keyTags[key] = make(map[string]bool)
		}
		for _, tag := range tags {
			tc.keyTags[key][tag] = true

			if tc.tags[tag] == nil {
				tc.tags[tag] = make(map[string]bool)
			}
			tc.tags[tag][key] = true
		}
	}

	return nil
}

// Get 获取缓存（类似 TP 8.1.4 的 get 方法，default 参数支持闭包）
func (tc *TaggedCache) Get(key string, defaultValue ...interface{}) interface{} {
	// 尝试获取缓存
	value := tc.cache.Get(key)
	if value != nil {
		return value
	}

	// 缓存不存在，使用默认值
	if len(defaultValue) > 0 {
		// 支持闭包（类似 TP 8.1.4）
		if fn, ok := defaultValue[0].(func() interface{}); ok {
			return fn()
		}
		return defaultValue[0]
	}

	return nil
}

// Delete 删除缓存
func (tc *TaggedCache) Delete(key string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	// 删除标签关系
	if tags, ok := tc.keyTags[key]; ok {
		for tag := range tags {
			delete(tc.tags[tag], key)
		}
		delete(tc.keyTags, key)
	}

	return tc.cache.Delete(key)
}

// Tag 指定缓存标签（类似 TP 8.1.4）
func (tc *TaggedCache) Tag(tags ...string) *TaggedCacheWriter {
	return &TaggedCacheWriter{
		cache: tc,
		tags:  tags,
	}
}

// ClearTags 清空指定标签的所有缓存（类似 TP 8.1.4）
func (tc *TaggedCache) ClearTags(tags ...string) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	for _, tag := range tags {
		if keys, ok := tc.tags[tag]; ok {
			for key := range keys {
				tc.cache.Delete(key)
				delete(tc.keyTags, key)
			}
			delete(tc.tags, tag)
		}
	}

	return nil
}

// GetTags 获取缓存的标签
func (tc *TaggedCache) GetTags(key string) []string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tags, ok := tc.keyTags[key]; ok {
		result := make([]string, 0, len(tags))
		for tag := range tags {
			result = append(result, tag)
		}
		return result
	}

	return nil
}

// cleanupLoop 定期清理过期标签
func (tc *TaggedCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		tc.mu.Lock()
		for key := range tc.keyTags {
			if tc.cache.Get(key) == nil {
				// 缓存已过期，清理标签关系
				if tags, ok := tc.keyTags[key]; ok {
					for tag := range tags {
						delete(tc.tags[tag], key)
					}
					delete(tc.keyTags, key)
				}
			}
		}
		tc.mu.Unlock()
	}
}

// TaggedCacheWriter 带标签的缓存写入器
type TaggedCacheWriter struct {
	cache *TaggedCache
	tags  []string
}

// Set 设置带标签的缓存
func (w *TaggedCacheWriter) Set(key string, value interface{}, ttl time.Duration) error {
	return w.cache.Set(key, value, ttl, w.tags...)
}

// Remember 记住缓存（类似 TP 的 remember 方法）
func (w *TaggedCacheWriter) Remember(key string, ttl time.Duration, callback func() interface{}) (interface{}, error) {
	// 尝试获取缓存
	value := w.cache.Get(key)
	if value != nil {
		return value, nil
	}

	// 执行回调获取值
	value = callback()

	// 设置缓存
	if err := w.cache.Set(key, value, ttl, w.tags...); err != nil {
		return nil, err
	}

	return value, nil
}

// CacheDependency 缓存依赖（类似 TP 8.1.4）
type CacheDependency struct {
	keys       []string
	lastModify map[string]time.Time
}

// NewCacheDependency 创建缓存依赖
func NewCacheDependency(keys ...string) *CacheDependency {
	return &CacheDependency{
		keys:       keys,
		lastModify: make(map[string]time.Time),
	}
}

// Add 添加依赖键
func (cd *CacheDependency) Add(keys ...string) {
	cd.keys = append(cd.keys, keys...)
}

// Check 检查依赖是否有效
func (cd *CacheDependency) Check(cache *MemoryCache) bool {
	for _, key := range cd.keys {
		if cache.Get(key) == nil {
			return false
		}
	}
	return true
}

// CacheWithDependency 带依赖的缓存
type CacheWithDependency struct {
	cache      *MemoryCache
	dependency *CacheDependency
}

// NewCacheWithDependency 创建带依赖的缓存
func NewCacheWithDependency(cache *MemoryCache, dep *CacheDependency) *CacheWithDependency {
	return &CacheWithDependency{
		cache:      cache,
		dependency: dep,
	}
}

// Set 设置缓存（检查依赖）
func (c *CacheWithDependency) Set(key string, value interface{}, ttl time.Duration) error {
	// 检查依赖是否有效
	if !c.dependency.Check(c.cache) {
		// 依赖失效，删除缓存
		c.cache.Delete(key)
		return fmt.Errorf("cache dependency invalid")
	}

	return c.cache.Set(key, value, ttl)
}

// Get 获取缓存
func (c *CacheWithDependency) Get(key string, defaultValue ...interface{}) interface{} {
	// 检查依赖
	if !c.dependency.Check(c.cache) {
		c.cache.Delete(key)
		if len(defaultValue) > 0 {
			if fn, ok := defaultValue[0].(func() interface{}); ok {
				return fn()
			}
			return defaultValue[0]
		}
		return nil
	}

	value := c.cache.Get(key)
	if value == nil && len(defaultValue) > 0 {
		if fn, ok := defaultValue[0].(func() interface{}); ok {
			return fn()
		}
		return defaultValue[0]
	}
	return value
}

// CachePull 缓存 pull 方法（类似 TP 8.1.4，增加 default 参数）
func (c *MemoryCache) Pull(key string, defaultValue ...interface{}) interface{} {
	value := c.Get(key)
	if value == nil {
		if len(defaultValue) > 0 {
			if fn, ok := defaultValue[0].(func() interface{}); ok {
				return fn()
			}
			return defaultValue[0]
		}
		return nil
	}

	// 获取后删除
	c.Delete(key)
	return value
}

// CacheSerialize 缓存序列化改进（类似 TP 8.1.4）
func (c *MemoryCache) SetSerialize(key string, value interface{}, ttl time.Duration) error {
	// 自动序列化
	data, err := serialize(value)
	if err != nil {
		return err
	}
	return c.Set(key, data, ttl)
}

// GetSerialize 缓存反序列化（类似 TP 8.1.4 改进反序列化异常处理）
func (c *MemoryCache) GetSerialize(key string, defaultValue ...interface{}) interface{} {
	value := c.Get(key)
	if value == nil {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return nil
	}

	// 自动反序列化（改进异常处理，类似 TP 8.1.4）
	result, err := deserialize(value)
	if err != nil {
		// 反序列化失败，返回默认值
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
		return value
	}

	return result
}

// serialize 序列化
func serialize(value interface{}) ([]byte, error) {
	data, ok := value.([]byte)
	if ok {
		return data, nil
	}

	return json.Marshal(value)
}

// deserialize 反序列化
func deserialize(value interface{}) (interface{}, error) {
	data, ok := value.([]byte)
	if !ok {
		return value, nil
	}

	var result interface{}
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// CacheWarm 缓存预热（类似 TP 8.1.4）
type CacheWarm struct {
	cache *TaggedCache
}

// NewCacheWarm 创建缓存预热
func NewCacheWarm(cache *TaggedCache) *CacheWarm {
	return &CacheWarm{cache: cache}
}

// Add 添加预热数据
func (w *CacheWarm) Add(key string, value interface{}, ttl time.Duration, tags ...string) error {
	return w.cache.Set(key, value, ttl, tags...)
}

// Batch 批量预热
func (w *CacheWarm) Batch(items map[string]interface{}, ttl time.Duration, tags ...string) error {
	for key, value := range items {
		if err := w.cache.Set(key, value, ttl, tags...); err != nil {
			return err
		}
	}
	return nil
}

// CacheFailDelete 配置（类似 TP 8.1.4 的 fail_delete 配置参数）
type CacheFailDeleteConfig struct {
	Enabled bool `yaml:"enabled"` // 是否启用 fail_delete
}

// NewCacheWithFailDelete 创建带 fail_delete 的缓存
func NewCacheWithFailDelete(cache *MemoryCache, config CacheFailDeleteConfig) *TaggedCache {
	tc := NewTaggedCache(cache)
	tc.SetFailDelete(config.Enabled)
	return tc
}
