// Package config 提供 Nacos 配置管理
package config

import "fmt"

// NacosConfig Nacos 配置中心和服务发现配置
type NacosConfig struct {
	IpAddr      string               `yaml:"host"`         // Nacos 服务地址
	Port        uint64               `yaml:"port"`         // Nacos 服务端口
	NamespaceId string               `yaml:"namespace"`    // Nacos 命名空间 ID
	DataId      string               `yaml:"data_id"`      // 配置数据 ID
	Group       string               `yaml:"group"`        // 配置分组
	InstallPath string               `yaml:"install_path"` // Nacos 安装路径（用于自动启动）
	Admin       AdminConfig          `yaml:"admin"`        // 管理界面配置
	Discovery   NacosDiscoveryConfig `yaml:"discovery"`    // 服务发现配置
}

// NacosDiscoveryConfig Nacos 服务发现配置
type NacosDiscoveryConfig struct {
	Enabled      bool   `yaml:"enabled"`       // 是否启用服务发现
	ServiceName  string `yaml:"service_name"`  // 注册的服务名称
	AutoRegister bool   `yaml:"auto_register"` // 是否自动注册服务
	Heartbeat    int    `yaml:"heartbeat"`     // 心跳间隔（秒）
}

// GetServerAddr 获取 Nacos 服务器地址
func (n *NacosConfig) GetServerAddr() string {
	return fmt.Sprintf("%s:%d", n.IpAddr, n.Port)
}

// IsDiscoveryEnabled 判断是否启用服务发现
func (n *NacosConfig) IsDiscoveryEnabled() bool {
	return n.Discovery.Enabled
}

// IsAdminEnabled 判断是否启用管理界面
func (n *NacosConfig) IsAdminEnabled() bool {
	return n.Admin.Enabled
}
