package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisCacheAdapter Redis 缓存适配器
type RedisCacheAdapter struct {
	client  *redis.Client
	prefix  string
	options *RedisCacheOptions
}

// RedisCacheOptions Redis 缓存选项
type RedisCacheOptions struct {
	Prefix       string        `yaml:"prefix"`
	DefaultTTL   time.Duration `yaml:"default_ttl"`
	MaxIdleConns int           `yaml:"max_idle_conns"`
	PoolSize     int           `yaml:"pool_size"`
	DialTimeout  time.Duration `yaml:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
}

// NewRedisCacheAdapter 创建 Redis 缓存适配器
func NewRedisCacheAdapter(client *redis.Client, opts *RedisCacheOptions) *RedisCacheAdapter {
	if opts == nil {
		opts = &RedisCacheOptions{}
	}

	if opts.Prefix == "" {
		opts.Prefix = "cache:"
	}

	if opts.DefaultTTL == 0 {
		opts.DefaultTTL = 24 * time.Hour
	}

	return &RedisCacheAdapter{
		client:  client,
		prefix:  opts.Prefix,
		options: opts,
	}
}

// Get 获取缓存
func (c *RedisCacheAdapter) Get(key string) (interface{}, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	data, err := c.client.Get(ctx, fullKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}

	var value interface{}
	if err := json.Unmarshal(data, &value); err != nil {
		return string(data), nil
	}

	return value, nil
}

// GetString 获取字符串缓存
func (c *RedisCacheAdapter) GetString(key string) (string, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	val, err := c.client.Get(ctx, fullKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}

	return val, nil
}

// Set 设置缓存
func (c *RedisCacheAdapter) Set(key string, value interface{}, ttl time.Duration) error {
	ctx := context.Background()
	fullKey := c.prefix + key

	if ttl == 0 {
		ttl = c.options.DefaultTTL
	}

	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, fullKey, data, ttl).Err()
}

// SetNX 设置缓存（不存在时）
func (c *RedisCacheAdapter) SetNX(key string, value interface{}, ttl time.Duration) (bool, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	if ttl == 0 {
		ttl = c.options.DefaultTTL
	}

	data, err := json.Marshal(value)
	if err != nil {
		return false, err
	}

	return c.client.SetNX(ctx, fullKey, data, ttl).Result()
}

// Delete 删除缓存
func (c *RedisCacheAdapter) Delete(key string) error {
	ctx := context.Background()
	fullKey := c.prefix + key

	return c.client.Del(ctx, fullKey).Err()
}

// Exists 检查键是否存在
func (c *RedisCacheAdapter) Exists(key string) (bool, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	count, err := c.client.Exists(ctx, fullKey).Result()
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// Expire 设置过期时间
func (c *RedisCacheAdapter) Expire(key string, ttl time.Duration) error {
	ctx := context.Background()
	fullKey := c.prefix + key

	return c.client.Expire(ctx, fullKey, ttl).Err()
}

// TTL 获取剩余过期时间
func (c *RedisCacheAdapter) TTL(key string) (time.Duration, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	return c.client.TTL(ctx, fullKey).Result()
}

// Incr 自增
func (c *RedisCacheAdapter) Incr(key string) (int64, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	return c.client.Incr(ctx, fullKey).Result()
}

// Decr 自减
func (c *RedisCacheAdapter) Decr(key string) (int64, error) {
	ctx := context.Background()
	fullKey := c.prefix + key

	return c.client.Decr(ctx, fullKey).Result()
}

// SetWithTags 设置带标签的缓存
func (c *RedisCacheAdapter) SetWithTags(key string, value interface{}, ttl time.Duration, tags ...string) error {
	if err := c.Set(key, value, ttl); err != nil {
		return err
	}

	for _, tag := range tags {
		tagKey := c.prefix + "tag:" + tag
		ctx := context.Background()
		c.client.SAdd(ctx, tagKey, key)
		c.client.Expire(ctx, tagKey, ttl)
	}

	return nil
}

// DeleteByTag 根据标签删除缓存
func (c *RedisCacheAdapter) DeleteByTag(tag string) error {
	tagKey := c.prefix + "tag:" + tag
	ctx := context.Background()

	keys, err := c.client.SMembers(ctx, tagKey).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		fullKeys := make([]string, 0, len(keys))
		for _, key := range keys {
			fullKeys = append(fullKeys, c.prefix+key)
		}
		c.client.Del(ctx, fullKeys...)
	}

	c.client.Del(ctx, tagKey)
	return nil
}

// GetMulti 批量获取
func (c *RedisCacheAdapter) GetMulti(keys []string) (map[string]interface{}, error) {
	ctx := context.Background()
	result := make(map[string]interface{})

	fullKeys := make([]string, 0, len(keys))
	keyMap := make(map[string]string)

	for _, key := range keys {
		fullKey := c.prefix + key
		fullKeys = append(fullKeys, fullKey)
		keyMap[fullKey] = key
	}

	values, err := c.client.MGet(ctx, fullKeys...).Result()
	if err != nil {
		return nil, err
	}

	for i, val := range values {
		if val != nil {
			originalKey := keyMap[fullKeys[i]]
			result[originalKey] = val
		}
	}

	return result, nil
}

// SetMulti 批量设置
func (c *RedisCacheAdapter) SetMulti(items map[string]interface{}, ttl time.Duration) error {
	ctx := context.Background()

	if ttl == 0 {
		ttl = c.options.DefaultTTL
	}

	pairs := make([]interface{}, 0, len(items)*2)
	for key, value := range items {
		fullKey := c.prefix + key
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		pairs = append(pairs, fullKey, data)
	}

	return c.client.MSet(ctx, pairs...).Err()
}

// Clear 清空缓存
func (c *RedisCacheAdapter) Clear() error {
	ctx := context.Background()
	cursor := uint64(0)
	pattern := c.prefix + "*"

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			c.client.Del(ctx, keys...)
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// Stats 获取统计信息
func (c *RedisCacheAdapter) Stats() map[string]interface{} {
	ctx := context.Background()
	info, err := c.client.Info(ctx, "stats").Result()
	if err != nil {
		return map[string]interface{}{}
	}

	return map[string]interface{}{
		"info": info,
	}
}

// Close 关闭连接
func (c *RedisCacheAdapter) Close() error {
	return c.client.Close()
}

// Ping 检查连接
func (c *RedisCacheAdapter) Ping() error {
	ctx := context.Background()
	_, err := c.client.Ping(ctx).Result()
	return err
}

// CreateRedisClient 创建 Redis 客户端
func CreateRedisClient(addr, password string, db int, opts *RedisCacheOptions) (*redis.Client, error) {
	if opts == nil {
		opts = &RedisCacheOptions{}
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     opts.PoolSize,
		DialTimeout:  opts.DialTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %v", err)
	}

	return client, nil
}
