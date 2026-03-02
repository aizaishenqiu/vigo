// Package config 提供数据库配置管理
package config

import "fmt"

// DatabaseConfig 默认数据库配置（主库）
// 支持读写分离和多从库负载均衡
type DatabaseConfig struct {
	Driver          string   `yaml:"driver"`            // 数据库驱动类型：mysql | postgres | sqlite | mssql
	Host            string   `yaml:"host"`              // 主库主机地址（写库）
	Port            int      `yaml:"port"`              // 主库端口
	Name            string   `yaml:"name"`              // 数据库名称
	User            string   `yaml:"user"`              // 数据库用户名
	Pass            string   `yaml:"pass"`              // 数据库密码
	Charset         string   `yaml:"charset"`           // 数据库字符集
	MaxOpenConns    int      `yaml:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int      `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime int      `yaml:"conn_max_lifetime"` // 连接最大生命周期（秒）
	ConnMaxIdleTime int      `yaml:"conn_max_idletime"` // 空闲连接回收时间（秒）
	RWSplit         bool     `yaml:"rw_split"`          // 是否开启读写分离
	Writes          []DBNode `yaml:"writes"`            // 写库节点列表（多主库）
	Reads           []DBNode `yaml:"reads"`             // 只读数据库节点列表（从库）
}

// DBConfig 多数据库配置（用于不同业务库）
// 每个业务库都可以独立配置读写分离
type DBConfig struct {
	Driver          string   `yaml:"driver"`            // 数据库驱动类型：mysql | postgres | sqlite | mssql
	Host            string   `yaml:"host"`              // 主库主机地址
	Port            int      `yaml:"port"`              // 主库端口
	Name            string   `yaml:"name"`              // 数据库名称
	User            string   `yaml:"user"`              // 数据库用户名
	Pass            string   `yaml:"pass"`              // 数据库密码
	Charset         string   `yaml:"charset"`           // 数据库字符集
	MaxOpenConns    int      `yaml:"max_open_conns"`    // 最大打开连接数
	MaxIdleConns    int      `yaml:"max_idle_conns"`    // 最大空闲连接数
	ConnMaxLifetime int      `yaml:"conn_max_lifetime"` // 连接最大生命周期（秒）
	ConnMaxIdleTime int      `yaml:"conn_max_idletime"` // 空闲连接回收时间（秒）
	RWSplit         bool     `yaml:"rw_split"`          // 是否开启读写分离
	Writes          []DBNode `yaml:"writes"`            // 写库节点列表
	Reads           []DBNode `yaml:"reads"`             // 只读数据库节点列表
}

// DBNode 数据库节点配置（用于读写分离）
type DBNode struct {
	Host    string `yaml:"host"`    // 节点主机地址
	Port    int    `yaml:"port"`    // 节点端口
	User    string `yaml:"user"`    // 节点用户名
	Pass    string `yaml:"pass"`    // 节点密码
	Charset string `yaml:"charset"` // 节点字符集
}

// GetDSN 获取数据库连接字符串
func (d *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local",
		d.User, d.Pass, d.Host, d.Port, d.Name, d.Charset)
}

// IsReadWriteSplit 判断是否启用读写分离
func (d *DatabaseConfig) IsReadWriteSplit() bool {
	return d.RWSplit && len(d.Reads) > 0
}
