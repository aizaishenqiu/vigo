package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Cache 缓存接口（与 contract.Cache 对齐）
type Cache interface {
	Get(key string) interface{}
	Set(key string, val interface{}, ttl time.Duration) error
	Delete(key string) error
	Has(key string) bool
}

// ==================== 内存缓存 ====================

// MemoryCache 带自动过期清理的内存缓存
type MemoryCache struct {
	items   map[string]item
	mu      sync.RWMutex
	maxSize int // 最大缓存条目数（0 = 不限制）
}

type item struct {
	Value  interface{}
	Expiry int64
}

// NewMemoryCache 创建内存缓存（启动后台清理协程）
func NewMemoryCache(opts ...int) *MemoryCache {
	maxSize := 0
	if len(opts) > 0 {
		maxSize = opts[0]
	}
	c := &MemoryCache{
		items:   make(map[string]item),
		maxSize: maxSize,
	}
	// 后台清理：每 60 秒清理过期条目，防止内存泄漏
	go c.cleanupLoop(60 * time.Second)
	return c
}

// cleanupLoop 定期清理过期条目
func (c *MemoryCache) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		c.cleanup()
	}
}

// cleanup 清理所有过期条目
func (c *MemoryCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UnixNano()
	for k, v := range c.items {
		if now > v.Expiry {
			delete(c.items, k)
		}
	}
}

// Set 设置缓存（返回 error 以对齐 contract.Cache 接口）
func (c *MemoryCache) Set(key string, val interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 缓存大小限制检查
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		// 简单淘汰策略：删除最先过期的条目
		c.evictOne()
	}

	c.items[key] = item{
		Value:  val,
		Expiry: time.Now().Add(ttl).UnixNano(),
	}
	return nil
}

// evictOne 淘汰一个最先过期的条目（调用时必须持有写锁）
func (c *MemoryCache) evictOne() {
	var oldestKey string
	var oldestExpiry int64 = 1<<63 - 1
	for k, v := range c.items {
		if v.Expiry < oldestExpiry {
			oldestExpiry = v.Expiry
			oldestKey = k
		}
	}
	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

func (c *MemoryCache) Get(key string) interface{} {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok || time.Now().UnixNano() > it.Expiry {
		if ok {
			// 惰性删除过期条目
			c.mu.Lock()
			delete(c.items, key)
			c.mu.Unlock()
		}
		return nil
	}
	return it.Value
}

func (c *MemoryCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
	return nil
}

func (c *MemoryCache) Has(key string) bool {
	return c.Get(key) != nil
}

// Count 返回当前缓存条目数
func (c *MemoryCache) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Flush 清空所有缓存
func (c *MemoryCache) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]item)
}

// ==================== 文件缓存 ====================

// FileCache 文件缓存（修复错误处理）
type FileCache struct {
	dir string
	mu  sync.RWMutex // 添加并发安全保护
}

type fileItem struct {
	Value  interface{} `json:"value"`
	Expiry int64       `json:"expiry"`
}

func NewFileCache(dir string) *FileCache {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}
	return &FileCache{dir: dir}
}

func (c *FileCache) getPath(key string) string {
	return filepath.Join(c.dir, key+".cache")
}

func (c *FileCache) Set(key string, val interface{}, ttl time.Duration) error {
	it := fileItem{
		Value:  val,
		Expiry: time.Now().Add(ttl).UnixNano(),
	}
	data, err := json.Marshal(it)
	if err != nil {
		return fmt.Errorf("缓存序列化失败: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.WriteFile(c.getPath(key), data, 0644); err != nil {
		return fmt.Errorf("缓存写入失败: %w", err)
	}
	return nil
}

func (c *FileCache) Get(key string) interface{} {
	c.mu.RLock()
	path := c.getPath(key)
	data, err := os.ReadFile(path)
	c.mu.RUnlock()

	if err != nil {
		return nil
	}
	var it fileItem
	if err := json.Unmarshal(data, &it); err != nil {
		return nil
	}
	if time.Now().UnixNano() > it.Expiry {
		c.mu.Lock()
		os.Remove(path)
		c.mu.Unlock()
		return nil
	}
	return it.Value
}

func (c *FileCache) Delete(key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return os.Remove(c.getPath(key))
}

func (c *FileCache) Has(key string) bool {
	return c.Get(key) != nil
}
