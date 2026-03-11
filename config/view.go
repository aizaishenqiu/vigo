// Package config 提供视图配置管理
package config

// ViewConfig 视图配置（ThinkPHP 风格）
type ViewConfig struct {
	// 视图目录（支持绝对路径或相对路径）
	// 相对路径相对于项目根目录
	Path string `yaml:"path"` // 默认："app/view"

	// 视图文件后缀
	Suffix string `yaml:"suffix"` // 默认：".html"

	// 视图引擎类型
	// 支持：template（默认模板引擎）、blade（Laravel 风格）
	Type string `yaml:"type"` // 默认："template"

	// 是否启用视图缓存
	Cache bool `yaml:"cache"` // 默认：false

	// 缓存目录
	CachePath string `yaml:"cache_path"` // 默认："runtime/view_cache"
}

// GetViewPath 获取视图目录路径
func (v *ViewConfig) GetViewPath() string {
	if v.Path == "" {
		return "app/view"
	}
	return v.Path
}

// GetSuffix 获取视图文件后缀
func (v *ViewConfig) GetSuffix() string {
	if v.Suffix == "" {
		return ".html"
	}
	return v.Suffix
}

// GetType 获取视图引擎类型
func (v *ViewConfig) GetType() string {
	if v.Type == "" {
		return "template"
	}
	return v.Type
}

// IsCacheEnabled 判断是否启用视图缓存
func (v *ViewConfig) IsCacheEnabled() bool {
	return v.Cache
}

// GetCachePath 获取缓存目录
func (v *ViewConfig) GetCachePath() string {
	if v.CachePath == "" {
		return "runtime/view_cache"
	}
	return v.CachePath
}
