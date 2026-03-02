package middleware

import (
	"sync"
	"vigo/framework/mvc"
)

// MiddlewareManager 中间件管理器
type MiddlewareManager struct {
	middlewares []mvc.HandlerFunc
	mu          sync.RWMutex
}

// GlobalMiddlewareManager 全局中间件管理器
var GlobalMiddlewareManager *MiddlewareManager

func init() {
	GlobalMiddlewareManager = NewMiddlewareManager()
}

// NewMiddlewareManager 创建中间件管理器
func NewMiddlewareManager() *MiddlewareManager {
	return &MiddlewareManager{
		middlewares: make([]mvc.HandlerFunc, 0),
	}
}

// Use 注册全局中间件
// 用法：middleware.Use(CORSMiddleware(), LogMiddleware())
func (m *MiddlewareManager) Use(middlewares ...mvc.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.middlewares = append(m.middlewares, middlewares...)
}

// GetMiddlewares 获取所有已注册的中间件
func (m *MiddlewareManager) GetMiddlewares() []mvc.HandlerFunc {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]mvc.HandlerFunc, len(m.middlewares))
	copy(result, m.middlewares)
	return result
}

// Clear 清空所有中间件
func (m *MiddlewareManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.middlewares = make([]mvc.HandlerFunc, 0)
}

// Use 注册全局中间件（便捷函数）
// 用法：middleware.Use(CORSMiddleware(), LogMiddleware())
func Use(middlewares ...mvc.HandlerFunc) {
	GlobalMiddlewareManager.Use(middlewares...)
}

// GetMiddlewares 获取所有已注册的中间件（便捷函数）
func GetMiddlewares() []mvc.HandlerFunc {
	return GlobalMiddlewareManager.GetMiddlewares()
}

// Clear 清空所有中间件（便捷函数）
func Clear() {
	GlobalMiddlewareManager.Clear()
}

// MiddlewareGroup 中间件分组（用于路由分组）
type MiddlewareGroup struct {
	middlewares []mvc.HandlerFunc
}

// NewMiddlewareGroup 创建中间件分组
func NewMiddlewareGroup(middlewares ...mvc.HandlerFunc) *MiddlewareGroup {
	return &MiddlewareGroup{
		middlewares: middlewares,
	}
}

// Use 添加中间件到分组
func (g *MiddlewareGroup) Use(middlewares ...mvc.HandlerFunc) {
	g.middlewares = append(g.middlewares, middlewares...)
}

// GetMiddlewares 获取分组中的所有中间件
func (g *MiddlewareGroup) GetMiddlewares() []mvc.HandlerFunc {
	return g.middlewares
}

// Merge 合并多个中间件分组
func Merge(groups ...*MiddlewareGroup) *MiddlewareGroup {
	all := make([]mvc.HandlerFunc, 0)
	for _, group := range groups {
		all = append(all, group.middlewares...)
	}
	return &MiddlewareGroup{
		middlewares: all,
	}
}

// ==================== 常用中间件组合 ====================

// SecurityGroup 安全防护中间件组合
func SecurityGroup() *MiddlewareGroup {
	return NewMiddlewareGroup(
		SecurityMiddleware(),
	)
}

// LogGroup 日志中间件组合
func LogGroup() *MiddlewareGroup {
	return NewMiddlewareGroup(
		RecoveryMiddleware(),
	)
}

// AuthGroup 认证中间件组合
func AuthGroup() *MiddlewareGroup {
	return NewMiddlewareGroup(
		JWTAuth(),
	)
}

// RateLimitGroup 限流中间件组合
func RateLimitGroup(requestsPerSecond int) *MiddlewareGroup {
	return NewMiddlewareGroup(
		RateLimitMiddleware(requestsPerSecond),
	)
}

// FullStack 完整中间件栈（安全 + 日志 + 认证）
func FullStack(authRequired bool, rateLimit int) *MiddlewareGroup {
	middlewares := []mvc.HandlerFunc{
		SecurityMiddleware(),
		RecoveryMiddleware(),
	}

	if authRequired {
		middlewares = append(middlewares, JWTAuth())
	}

	if rateLimit > 0 {
		middlewares = append(middlewares, RateLimitMiddleware(rateLimit))
	}

	return NewMiddlewareGroup(middlewares...)
}
