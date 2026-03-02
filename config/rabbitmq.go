// Package config 提供 RabbitMQ 配置管理
package config

import "fmt"

// RabbitMQConfig RabbitMQ 消息队列配置
type RabbitMQConfig struct {
	Enabled        bool        `yaml:"enabled"`         // 是否启用 RabbitMQ
	Host           string      `yaml:"host"`            // RabbitMQ 主机地址
	Port           int         `yaml:"port"`            // RabbitMQ 端口
	User           string      `yaml:"user"`            // RabbitMQ 用户名
	Password       string      `yaml:"pass"`            // RabbitMQ 密码
	Vhost          string      `yaml:"vhost"`           // 虚拟主机
	ConnTimeout    int         `yaml:"conn_timeout"`    // 连接超时时间（秒）
	Heartbeat      int         `yaml:"heartbeat"`       // 心跳间隔（秒）
	ReconnectDelay int         `yaml:"reconnect_delay"` // 重连延迟（秒）
	MaxRetries     int         `yaml:"max_retries"`     // 最大重连次数
	Admin          AdminConfig `yaml:"admin"`           // 管理界面配置
}

// GetDSN 获取 RabbitMQ 连接字符串
func (r *RabbitMQConfig) GetDSN() string {
	return fmt.Sprintf("amqp://%s:%s@%s:%d%s",
		r.User, r.Password, r.Host, r.Port, r.Vhost)
}

// GetManagementURL 获取管理界面 URL
func (r *RabbitMQConfig) GetManagementURL() string {
	port := r.Admin.Port
	if port == 0 {
		port = 15672
	}
	return fmt.Sprintf("http://%s:%d", r.Host, port)
}

// IsEnabled 判断是否启用 RabbitMQ
func (r *RabbitMQConfig) IsEnabled() bool {
	return r.Enabled
}

// IsAdminEnabled 判断是否启用管理界面
func (r *RabbitMQConfig) IsAdminEnabled() bool {
	return r.Admin.Enabled
}
