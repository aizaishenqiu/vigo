package admin

import (
	"net/http"
	"strings"
	"vigo/framework/mvc"
)

// AuthMiddleware 鉴权中间件
func AuthMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		// 静态资源和登录页面放行
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/admin/static") || 
		   path == "/admin/login" || 
		   path == "/admin/api/login" {
			c.Next()
			return
		}

		// 检查登录状态 (简单实现：检查 cookie)
		cookie, err := c.Request.Cookie("admin_token")
		if err != nil || cookie.Value == "" {
			// 如果是 API 请求，返回 JSON
			if strings.HasPrefix(path, "/admin/api") {
				c.Json(401, map[string]interface{}{
					"code": 401,
					"msg":  "未登录或会话已过期",
				})
				c.Abort()
				return
			}
			
			// 否则重定向到登录页
			c.Redirect(http.StatusFound, "/admin/login")
			c.Abort()
			return
		}

		// 可以在这里验证 token 有效性，目前简单起见只检查存在性
		// 实际项目中应校验 session/jwt

		c.Next()
	}
}
