// Package redis 提供 Redis 缓存客户端封装
// 同时支持单实例和集群模式，可通过配置一键切换
package redis

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config Redis 客户端配置
type Config struct {
	Host         string        // Redis 主机地址（单实例模式）
	Port         int           // Redis 端口
	Password     string        // Redis 密码
	DB           int           // Redis 数据库编号（0-15）
	PoolSize     int           // 最大连接池大小
	MinIdleConns int           // 最小空闲连接数
	MaxIdleConns int           // 最大空闲连接数
	Cluster      ClusterConfig // Redis 集群配置
}

// ClusterConfig Redis 集群配置
type ClusterConfig struct {
	Enabled bool     // 是否启用集群模式
	Addrs   []string // 集群节点地址列表
}

// Client Redis 客户端封装
// 可以是单实例客户端，也可以是集群客户端
type Client struct {
	rdb        *redis.Client        // 单实例客户端
	clusterRdb *redis.ClusterClient // 集群客户端
	isCluster  bool                 // 是否为集群模式
}

// silentLogger 静默日志记录器
// 用于抑制 go-redis 内部连接池日志，避免高并发时控制台刷屏
type silentLogger struct{}

func (s silentLogger) Printf(_ context.Context, _ string, _ ...interface{}) {}

// New 创建 Redis 客户端
// 根据配置自动选择单实例或集群模式
func New(cfg Config) *Client {
	redis.SetLogger(&silentLogger{})
	_ = io.Discard

	// 如果启用了集群模式且配置了集群节点，则使用集群客户端
	if cfg.Cluster.Enabled && len(cfg.Cluster.Addrs) > 0 {
		rdb := redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:           cfg.Cluster.Addrs,
			Password:        cfg.Password,
			PoolSize:        cfg.PoolSize,
			MinIdleConns:    cfg.MinIdleConns,
			MaxIdleConns:    cfg.MaxIdleConns,
			ConnMaxIdleTime: 5 * time.Minute,  // 空闲连接最大存活时间
			ConnMaxLifetime: 30 * time.Minute, // 连接最大生命周期
			DialTimeout:     3 * time.Second,  // 连接超时
			ReadTimeout:     3 * time.Second,  // 读取超时
			WriteTimeout:    3 * time.Second,  // 写入超时
			PoolTimeout:     5 * time.Second,  // 获取连接超时
		})
		return &Client{clusterRdb: rdb, isCluster: true}
	}

	// 否则使用单实例客户端
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	poolSize := cfg.PoolSize
	if poolSize <= 0 {
		poolSize = 50
	}

	minIdle := cfg.MinIdleConns
	if minIdle <= 0 {
		minIdle = 10
	}

	maxIdle := cfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = poolSize / 2
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:            addr,
		Password:        cfg.Password,
		DB:              cfg.DB,
		PoolSize:        poolSize,
		MinIdleConns:    minIdle,
		MaxIdleConns:    maxIdle,
		ConnMaxIdleTime: 5 * time.Minute,  // 空闲连接最大存活时间
		ConnMaxLifetime: 30 * time.Minute, // 连接最大生命周期
		DialTimeout:     3 * time.Second,  // 连接超时
		ReadTimeout:     3 * time.Second,  // 读取超时
		WriteTimeout:    3 * time.Second,  // 写入超时
		PoolTimeout:     5 * time.Second,  // 获取连接超时
	})

	return &Client{rdb: rdb, isCluster: false}
}

// Connect 测试 Redis 连接
// 使用带超时的 context，不依赖全局 Context
func (c *Client) Connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if c.isCluster {
		return c.clusterRdb.Ping(ctx).Err()
	}
	return c.rdb.Ping(ctx).Err()
}

// GetInfo 获取 Redis 统计信息
// 返回 Redis INFO 命令的结果（解析为 map）
func (c *Client) GetInfo(ctx context.Context) map[string]string {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var infoStr string
	var err error
	if c.isCluster {
		infoStr, err = c.clusterRdb.Info(ctx).Result()
	} else {
		infoStr, err = c.rdb.Info(ctx).Result()
	}

	if err != nil {
		log.Printf("Failed to get Redis info: %v", err)
		return nil
	}

	info := make(map[string]string)
	lines := splitLines(infoStr)
	for _, line := range lines {
		if len(line) > 0 && line[0] != '#' && contains(line, ':') {
			parts := splitKeyVal(line)
			if len(parts) == 2 {
				info[parts[0]] = parts[1]
			}
		}
	}
	return info
}

// GetKeys 使用 SCAN 迭代获取 Key 列表
// 替代阻塞型 KEYS 命令，适合生产环境
// 参数:
//   - pattern: 匹配模式，如 "user:*"
//   - limit: 返回最大数量，0 表示不限制
func (c *Client) GetKeys(ctx context.Context, pattern string, limit int) []string {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var keys []string
	if c.isCluster {
		iter := c.clusterRdb.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
			if limit > 0 && len(keys) >= limit {
				break
			}
		}
		if err := iter.Err(); err != nil {
			log.Printf("Redis SCAN error: %v", err)
		}
	} else {
		iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			keys = append(keys, iter.Val())
			if limit > 0 && len(keys) >= limit {
				break
			}
		}
		if err := iter.Err(); err != nil {
			log.Printf("Redis SCAN error: %v", err)
		}
	}
	return keys
}

// Set 设置键值对
// 参数:
//   - key: 键名
//   - value: 值
//   - expiration: 过期时间，0 表示永不过期
func (c *Client) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	if c.isCluster {
		return c.clusterRdb.Set(ctx, key, value, expiration)
	}
	return c.rdb.Set(ctx, key, value, expiration)
}

// Get 获取键的值
func (c *Client) Get(ctx context.Context, key string) *redis.StringCmd {
	if c.isCluster {
		return c.clusterRdb.Get(ctx, key)
	}
	return c.rdb.Get(ctx, key)
}

// Ping 发送 Ping 命令测试连接
func (c *Client) Ping(ctx context.Context) *redis.StatusCmd {
	if c.isCluster {
		return c.clusterRdb.Ping(ctx)
	}
	return c.rdb.Ping(ctx)
}

// Del 删除一个或多个键
func (c *Client) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	if c.isCluster {
		return c.clusterRdb.Del(ctx, keys...)
	}
	return c.rdb.Del(ctx, keys...)
}

// Close 关闭 Redis 连接
func (c *Client) Close() error {
	if c.isCluster {
		return c.clusterRdb.Close()
	}
	return c.rdb.Close()
}

// splitLines 将字符串按换行符分割
func splitLines(s string) []string {
	var lines []string
	var current []rune
	for _, r := range s {
		if r == '\r' {
			continue
		}
		if r == '\n' {
			lines = append(lines, string(current))
			current = []rune{}
		} else {
			current = append(current, r)
		}
	}
	if len(current) > 0 {
		lines = append(lines, string(current))
	}
	return lines
}

// contains 判断字符串是否包含指定字符
func contains(s string, char rune) bool {
	for _, c := range s {
		if c == char {
			return true
		}
	}
	return false
}

// splitKeyVal 按冒号分割键值对
func splitKeyVal(s string) []string {
	for i, c := range s {
		if c == ':' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
