package middleware

import (
	"net"
	"net/http"
	"strings"
	"vigo/config"
	"vigo/framework/mvc"
)

// EnvironmentMiddleware 环境识别中间件
// 开发环境：自动禁用 CORS 限制
// 生产环境：验证域名和 IP
func EnvironmentMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		// 判断是否为开发环境
		isDev := config.App.App.Mode == "dev" || config.App.App.Debug

		if isDev {
			// 开发环境：允许所有 CORS
			c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

			// 处理预检请求
			if c.Request.Method == "OPTIONS" {
				c.Abort()
				c.Json(http.StatusOK, map[string]interface{}{
					"code":    200,
					"message": "CORS 预检请求通过",
				})
				return
			}

			c.Next()
			return
		}

		// 生产环境：验证域名和 IP
		if config.App.Security.EnableCorsDomainCheck {
			// 验证域名（包括 Electron 桌面应用）
			if !validateOrigin(c) {
				c.Abort()
				c.Json(http.StatusForbidden, map[string]interface{}{
					"code":    403,
					"message": "禁止访问：域名未授权",
				})
				return
			}
		}

		// 验证 IP 白名单/黑名单
		if !validateIP(c) {
			c.Abort()
			c.Json(http.StatusForbidden, map[string]interface{}{
				"code":    403,
				"message": "禁止访问：IP 未授权",
			})
			return
		}

		c.Next()
	}
}

// validateOrigin 验证请求来源域名
func validateOrigin(c *mvc.Context) bool {
	origin := c.Request.Header.Get("Origin")

	// 如果没有 Origin 头，可能是：
	// 1. 同源请求
	// 2. 移动端 App
	// 3. Electron 桌面应用（file:// 协议）
	// 允许通过
	if origin == "" {
		return true
	}

	// Electron 桌面应用特殊处理
	// Electron 的 origin 可能是：
	// - file://
	// - app://
	// - http://localhost:xxxx (开发环境)
	if strings.HasPrefix(origin, "file://") ||
		strings.HasPrefix(origin, "app://") ||
		strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "https://localhost") {
		// Electron 桌面应用，允许访问
		c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		return true
	}

	// 检查是否在允许的域名列表中
	allowedDomains := config.App.Security.AllowedDomains
	if len(allowedDomains) == 0 {
		// 没有配置允许的域名，默认拒绝
		return false
	}

	for _, allowed := range allowedDomains {
		if origin == allowed {
			// 设置 CORS 头
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowed)
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			return true
		}
	}

	return false
}

// validateIP 验证 IP 白名单/黑名单
func validateIP(c *mvc.Context) bool {
	clientIP := GetClientIP(c.Request)

	// 检查黑名单
	blacklist := config.App.Security.IPBlacklist
	for _, ip := range blacklist {
		if matchIP(clientIP, ip) {
			return false
		}
	}

	// 检查白名单（如果配置了白名单）
	whitelist := config.App.Security.IPWhitelist
	if len(whitelist) > 0 {
		for _, ip := range whitelist {
			if matchIP(clientIP, ip) {
				return true
			}
		}
		// 不在白名单中，拒绝访问
		return false
	}

	// 没有配置白名单，允许访问
	return true
}

// GetClientIP 获取客户端真实 IP（导出函数）
func GetClientIP(r *http.Request) string {
	// 尝试 X-Forwarded-For
	ip := r.Header.Get("X-Forwarded-For")
	if ip != "" {
		// X-Forwarded-For 可能包含多个 IP，取第一个
		ips := strings.Split(ip, ",")
		if len(ips) > 0 {
			ip = strings.TrimSpace(ips[0])
		}
	}

	// 尝试 X-Real-IP
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}

	// 使用 RemoteAddr
	if ip == "" {
		ip = r.RemoteAddr
		// 去除端口
		if colonIndex := strings.LastIndex(ip, ":"); colonIndex != -1 {
			ip = ip[:colonIndex]
		}
	}

	return ip
}

// matchIP 匹配 IP（支持 CIDR 格式）
func matchIP(ip string, pattern string) bool {
	// 精确匹配
	if ip == pattern {
		return true
	}

	// CIDR 匹配
	if strings.Contains(pattern, "/") {
		_, cidr, err := net.ParseCIDR(pattern)
		if err != nil {
			return false
		}

		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return false
		}

		return cidr.Contains(parsedIP)
	}

	// 通配符匹配（如 192.168.1.*）
	if strings.Contains(pattern, "*") {
		patternParts := strings.Split(pattern, ".")
		ipParts := strings.Split(ip, ".")

		if len(patternParts) != 4 || len(ipParts) != 4 {
			return false
		}

		for i := 0; i < 4; i++ {
			if patternParts[i] == "*" {
				continue
			}
			if patternParts[i] != ipParts[i] {
				return false
			}
		}
		return true
	}

	return false
}

// CORS 预检请求处理中间件
func CORSPreflightMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Abort()
			c.Json(http.StatusOK, map[string]interface{}{
				"code":    200,
				"message": "CORS 预检请求通过",
			})
			return
		}
		c.Next()
	}
}
