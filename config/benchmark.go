// Package config 提供压力测试配置管理
package config

// BenchmarkConfig 压力测试配置
type BenchmarkConfig struct {
	MemLimitPercent int `yaml:"mem_limit_percent"` // 内存限制百分比
	CPULimitPercent int `yaml:"cpu_limit_percent"` // CPU 限制百分比
}

// IsMemoryLimitEnabled 判断是否启用内存限制
func (b *BenchmarkConfig) IsMemoryLimitEnabled() bool {
	return b.MemLimitPercent > 0
}

// IsCPULimitEnabled 判断是否启用 CPU 限制
func (b *BenchmarkConfig) IsCPULimitEnabled() bool {
	return b.CPULimitPercent > 0
}

// GetMemoryLimit 获取内存限制百分比
func (b *BenchmarkConfig) GetMemoryLimit() int {
	if b.MemLimitPercent <= 0 {
		return 80 // 默认 80%
	}
	return b.MemLimitPercent
}

// GetCPULimit 获取 CPU 限制百分比
func (b *BenchmarkConfig) GetCPULimit() int {
	if b.CPULimitPercent <= 0 {
		return 90 // 默认 90%
	}
	return b.CPULimitPercent
}
