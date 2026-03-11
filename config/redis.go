// Package config 提供 Redis 配置管理
package config

import "fmt"

// RedisConfig Redis 缓存配置
type RedisConfig struct {
	Host         string             `yaml:"host"`           // Redis 主机地址（单实例）
	Port         int                `yaml:"port"`           // Redis 端口
	Password     string             `yaml:"password"`       // Redis 密码
	DB           int                `yaml:"db"`             // Redis 数据库编号
	PoolSize     int                `yaml:"pool_size"`      // 最大连接池大小
	MinIdleConns int                `yaml:"min_idle_conns"` // 最小空闲连接数
	MaxIdleConns int                `yaml:"max_idle_conns"` // 最大空闲连接数
	Cluster      RedisClusterConfig `yaml:"cluster"`        // Redis 集群配置
}

// RedisClusterConfig Redis 集群配置
type RedisClusterConfig struct {
	Enabled bool     `yaml:"enabled"` // 是否启用集群模式
	Addrs   []string `yaml:"addrs"`   // 集群节点地址列表
}

// GetRedisDSN 获取 Redis 连接字符串
func (r *RedisConfig) GetRedisDSN() string {
	if r.Host == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

// IsClusterEnabled 判断是否启用集群模式
func (r *RedisConfig) IsClusterEnabled() bool {
	return r.Cluster.Enabled && len(r.Cluster.Addrs) > 0
}
