package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

// QueryCache 查询缓存管理器
// 提供数据库查询结果缓存功能，支持自动缓存失效和统计
type QueryCache struct {
	cache   Cache
	enabled bool
	prefix  string
	stats   *QueryCacheStats
}

// QueryCacheStats 查询缓存统计（并发安全）
type QueryCacheStats struct {
	Hits        int64 // 命中次数
	Misses      int64 // 未命中次数
	Sets        int64 // 写入次数
	Invalidates int64 // 失效次数
	Errors      int64 // 错误次数
}

// NewQueryCache 创建查询缓存管理器
// 参数:
//   - cache: 底层缓存实现（内存/Redis）
func NewQueryCache(cache Cache) *QueryCache {
	return &QueryCache{
		cache:   cache,
		enabled: true,
		prefix:  "query:",
		stats:   &QueryCacheStats{},
	}
}

// CacheQuery 缓存查询结果
// 参数:
//   - key: 缓存键
//   - queryFunc: 查询函数
//   - ttl: 缓存时间
//
// 返回:
//   - result: 查询结果
//   - err: 错误信息
func (qc *QueryCache) CacheQuery(key string, queryFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	if !qc.enabled {
		return queryFunc()
	}

	// 尝试从缓存获取
	cached := qc.cache.Get(key)
	if cached != nil {
		atomic.AddInt64(&qc.stats.Hits, 1)
		return cached, nil
	}

	atomic.AddInt64(&qc.stats.Misses, 1)

	// 执行查询
	result, err := queryFunc()
	if err != nil {
		return nil, err
	}

	// 写入缓存
	if err := qc.cache.Set(key, result, ttl); err != nil {
		atomic.AddInt64(&qc.stats.Errors, 1)
		log.Printf("[QueryCache] 写入缓存失败：%v", err)
	} else {
		atomic.AddInt64(&qc.stats.Sets, 1)
	}

	return result, nil
}

// CacheQueryWithKey 使用自定义键生成函数缓存查询
// 参数:
//   - keyFunc: 键生成函数
//   - queryFunc: 查询函数
//   - ttl: 缓存时间
func (qc *QueryCache) CacheQueryWithKey(keyFunc func() string, queryFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	key := qc.prefix + keyFunc()
	return qc.CacheQuery(key, queryFunc, ttl)
}

// InvalidateCache 使缓存失效
// 参数:
//   - key: 缓存键
func (qc *QueryCache) InvalidateCache(key string) error {
	atomic.AddInt64(&qc.stats.Invalidates, 1)
	return qc.cache.Delete(key)
}

// InvalidateCachePattern 使匹配模式的缓存失效
// 参数:
//   - pattern: 缓存键模式（支持 * 通配符）
func (qc *QueryCache) InvalidateCachePattern(pattern string) error {
	// 如果缓存支持模式删除
	if dc, ok := qc.cache.(interface{ DeletePattern(string) error }); ok {
		atomic.AddInt64(&qc.stats.Invalidates, 1)
		return dc.DeletePattern(qc.prefix + pattern)
	}

	// 否则记录日志
	log.Printf("[QueryCache] 缓存不支持模式删除：%s", pattern)
	return nil
}

// Enable 启用缓存
func (qc *QueryCache) Enable() {
	qc.enabled = true
}

// Disable 禁用缓存
func (qc *QueryCache) Disable() {
	qc.enabled = false
}

// IsEnabled 检查是否启用
func (qc *QueryCache) IsEnabled() bool {
	return qc.enabled
}

// SetPrefix 设置缓存键前缀
func (qc *QueryCache) SetPrefix(prefix string) {
	qc.prefix = prefix
}

// GetStats 获取缓存统计
func (qc *QueryCache) GetStats() *QueryCacheStats {
	return qc.stats
}

// GetHitRate 获取缓存命中率
func (qc *QueryCache) GetHitRate() float64 {
	total := qc.stats.Hits + qc.stats.Misses
	if total == 0 {
		return 0
	}
	return float64(qc.stats.Hits) / float64(total) * 100
}

// ResetStats 重置统计
func (qc *QueryCache) ResetStats() {
	qc.stats = &QueryCacheStats{}
}

// PrintStats 打印统计信息
func (qc *QueryCache) PrintStats() {
	log.Printf("=== 查询缓存统计 ===")
	log.Printf("命中次数：%d", qc.stats.Hits)
	log.Printf("未命中次数：%d", qc.stats.Misses)
	log.Printf("写入次数：%d", qc.stats.Sets)
	log.Printf("失效次数：%d", qc.stats.Invalidates)
	log.Printf("错误次数：%d", qc.stats.Errors)
	log.Printf("命中率：%.2f%%", qc.GetHitRate())
}

// ==================== 缓存键生成工具 ====================

// GenerateCacheKey 生成缓存键（MD5）
// 参数:
//   - parts: 键的组成部分
//
// 返回:
//   - 缓存键字符串
func GenerateCacheKey(parts ...string) string {
	key := strings.Join(parts, ":")
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// CacheKey 生成简单缓存键
func CacheKey(prefix string, id interface{}) string {
	return fmt.Sprintf("%s:%v", prefix, id)
}

// CacheKeyWithTable 生成表查询缓存键
func CacheKeyWithTable(table string, id interface{}) string {
	return fmt.Sprintf("query:%s:%v", table, id)
}

// CacheKeyWithQuery 生成复杂查询缓存键
func CacheKeyWithQuery(table string, where map[string]interface{}, order string, limit, offset int) string {
	var parts []string
	parts = append(parts, "query", table)

	// 添加 where 条件
	for k, v := range where {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}

	// 添加排序
	if order != "" {
		parts = append(parts, "order:"+order)
	}

	// 添加分页
	parts = append(parts, fmt.Sprintf("limit:%d", limit))
	parts = append(parts, fmt.Sprintf("offset:%d", offset))

	return GenerateCacheKey(parts...)
}

// CacheKeyWithUser 生成用户相关缓存键
func CacheKeyWithUser(userID interface{}, resource string) string {
	return fmt.Sprintf("query:user:%v:%s", userID, resource)
}

// CacheKeyWithTenant 生成租户相关缓存键
func CacheKeyWithTenant(tenantID interface{}, resource string) string {
	return fmt.Sprintf("query:tenant:%v:%s", tenantID, resource)
}

// CacheKeyWithTime 生成时间范围缓存键
func CacheKeyWithTime(prefix string, start, end time.Time) string {
	return fmt.Sprintf("%s:%s:%s", prefix,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"))
}

// CacheKeyWithPage 生成分页缓存键
func CacheKeyWithPage(prefix string, page, limit int) string {
	return fmt.Sprintf("%s:page:%d:limit:%d", prefix, page, limit)
}

// CacheKeyWithParams 生成带参数的缓存键
func CacheKeyWithParams(prefix string, params map[string]interface{}) string {
	parts := []string{prefix}
	for k, v := range params {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return GenerateCacheKey(parts...)
}

// GlobalQueryCache 全局查询缓存实例
var GlobalQueryCache *QueryCache

// InitGlobalQueryCache 初始化全局查询缓存
func InitGlobalQueryCache(cache Cache) {
	GlobalQueryCache = NewQueryCache(cache)
}

// GetGlobalQueryCache 获取全局查询缓存
func GetGlobalQueryCache() *QueryCache {
	return GlobalQueryCache
}

// CacheGlobalQuery 使用全局查询缓存
func CacheGlobalQuery(key string, queryFunc func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	if GlobalQueryCache == nil {
		return queryFunc()
	}
	return GlobalQueryCache.CacheQuery(key, queryFunc, ttl)
}
