// Package config 提供 gRPC 配置管理
package config

import "fmt"

// GRPCConfig gRPC 微服务配置
type GRPCConfig struct {
	Enabled        bool   `yaml:"enabled"`           // 是否启用 gRPC 服务
	Port           int    `yaml:"port"`              // gRPC 服务端口
	ServiceName    string `yaml:"service_name"`      // 服务名称
	EnableRecovery bool   `yaml:"enable_recovery"`   // 是否启用 panic 恢复
	EnableLogger   bool   `yaml:"enable_logger"`     // 是否启用日志
	MaxRecvMsgSize int    `yaml:"max_recv_msg_size"` // 最大接收消息大小（MB）
	MaxSendMsgSize int    `yaml:"max_send_msg_size"` // 最大发送消息大小（MB）
}

// GetServerAddr 获取 gRPC 服务器地址
func (g *GRPCConfig) GetServerAddr() string {
	return fmt.Sprintf(":%d", g.Port)
}

// IsEnabled 判断是否启用 gRPC 服务
func (g *GRPCConfig) IsEnabled() bool {
	return g.Enabled
}

// IsRecoveryEnabled 判断是否启用 panic 恢复
func (g *GRPCConfig) IsRecoveryEnabled() bool {
	return g.EnableRecovery
}

// IsLoggerEnabled 判断是否启用日志
func (g *GRPCConfig) IsLoggerEnabled() bool {
	return g.EnableLogger
}

// GetMaxRecvMsgSize 获取最大接收消息大小（字节）
func (g *GRPCConfig) GetMaxRecvMsgSize() int {
	if g.MaxRecvMsgSize <= 0 {
		return 4 * 1024 * 1024 // 默认 4MB
	}
	return g.MaxRecvMsgSize * 1024 * 1024
}

// GetMaxSendMsgSize 获取最大发送消息大小（字节）
func (g *GRPCConfig) GetMaxSendMsgSize() int {
	if g.MaxSendMsgSize <= 0 {
		return 4 * 1024 * 1024 // 默认 4MB
	}
	return g.MaxSendMsgSize * 1024 * 1024
}
