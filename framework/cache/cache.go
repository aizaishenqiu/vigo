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

// ==================== 内存缓存（LRU 淘汰策略） ====================

// MemoryCache 带自动过期清理和 LRU 淘汰策略的内存缓存
type MemoryCache struct {
	items   map[string]*cacheItem
	mu      sync.RWMutex
	maxSize int // 最大缓存条目数（0 = 不限制）
	// LRU 链表：最近使用的移到链表尾部
	head *cacheItem // 最久未使用
	tail *cacheItem // 最近使用
	// 统计信息
	hits   int64
	misses int64
}

// cacheItem 缓存项（包含 LRU 链表指针）
type cacheItem struct {
	Key        string
	Value      interface{}
	Expiry     int64
	Prev       *cacheItem // 前一个节点
	Next       *cacheItem // 后一个节点
	AccessTime int64      // 最后访问时间戳
}

// NewMemoryCache 创建内存缓存（启动后台清理协程）
// 可选参数：maxSize - 最大缓存条目数
func NewMemoryCache(opts ...int) *MemoryCache {
	maxSize := 0
	if len(opts) > 0 {
		maxSize = opts[0]
	}
	c := &MemoryCache{
		items:   make(map[string]*cacheItem),
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
	for k, item := range c.items {
		if now > item.Expiry {
			c.removeItem(k)
		}
	}
}

// removeItem 移除缓存项（同时从 LRU 链表中移除）
func (c *MemoryCache) removeItem(key string) {
	if item, ok := c.items[key]; ok {
		// 从 LRU 链表中移除
		if item.Prev != nil {
			item.Prev.Next = item.Next
		} else {
			c.head = item.Next
		}
		if item.Next != nil {
			item.Next.Prev = item.Prev
		} else {
			c.tail = item.Prev
		}
		delete(c.items, key)
	}
}

// moveToTail 将节点移到链表尾部（表示最近使用）
func (c *MemoryCache) moveToTail(item *cacheItem) {
	if item == c.tail {
		return // 已经在尾部
	}

	// 从原位置移除
	if item.Prev != nil {
		item.Prev.Next = item.Next
	} else {
		c.head = item.Next
	}
	if item.Next != nil {
		item.Next.Prev = item.Prev
	}

	// 添加到尾部
	item.Prev = c.tail
	item.Next = nil
	if c.tail != nil {
		c.tail.Next = item
	}
	c.tail = item
	if c.head == nil {
		c.head = item
	}
}

// evictLRU 淘汰最久未使用的条目（LRU 策略）
func (c *MemoryCache) evictLRU() {
	if c.head != nil {
		key := c.head.Key
		c.removeItem(key)
	}
}

// Set 设置缓存（LRU 淘汰策略）
func (c *MemoryCache) Set(key string, val interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// 如果 key 已存在，更新值并移到尾部
	if item, ok := c.items[key]; ok {
		item.Value = val
		item.Expiry = now + ttl.Nanoseconds()
		item.AccessTime = now
		c.moveToTail(item)
		return nil
	}

	// 缓存大小限制检查
	if c.maxSize > 0 && len(c.items) >= c.maxSize {
		// LRU 淘汰：删除最久未使用的条目
		c.evictLRU()
	}

	// 创建新项并添加到尾部
	item := &cacheItem{
		Key:        key,
		Value:      val,
		Expiry:     now + ttl.Nanoseconds(),
		AccessTime: now,
	}
	c.items[key] = item
	c.moveToTail(item)
	return nil
}

func (c *MemoryCache) Get(key string) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		c.misses++
		return nil
	}

	// 检查是否过期
	if time.Now().UnixNano() > item.Expiry {
		c.removeItem(key)
		c.misses++
		return nil
	}

	// 更新访问时间并移到尾部（LRU 策略）
	item.AccessTime = time.Now().UnixNano()
	c.moveToTail(item)
	c.hits++
	return item.Value
}

// Stats 获取缓存统计信息
func (c *MemoryCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"items":    len(c.items),
		"max_size": c.maxSize,
		"hits":     c.hits,
		"misses":   c.misses,
		"hit_rate": hitRate,
	}
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
	c.items = make(map[string]*cacheItem)
	c.head = nil
	c.tail = nil
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
