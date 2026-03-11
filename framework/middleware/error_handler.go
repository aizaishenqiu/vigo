package middleware

import (
	"fmt"
	"net/http"
	"runtime"
	"vigo/framework/errors"
	"vigo/framework/mvc"
)

// RecoveryMiddleware 错误恢复中间件
func RecoveryMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		defer func() {
			if err := recover(); err != nil {
				// 记录堆栈跟踪
				stack := make([]byte, 4096)
				n := runtime.Stack(stack, false)
				stackStr := string(stack[:n])

				appErr := errors.InternalError("Internal Server Error")
				appErr.Stack = stackStr

				// 记录错误
				fmt.Printf("[PANIC] Panic recovered: %v\nStack trace:\n%s\nPath: %s Method: %s\n",
					err, stackStr, c.Request.URL.Path, c.Request.Method)

				c.Error(http.StatusInternalServerError, appErr.Message)
			}
		}()

		c.Next()
	}
}

// ErrorHandlerMiddleware 统一错误处理中间件
func ErrorHandlerMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		c.Next()

		// 检查是否有错误
		if statusCode := c.Status(); statusCode >= 400 {
			// 如果是API路径，返回JSON格式错误
			if isAPIPath(c.Request.URL.Path) {
				errorResponse := map[string]interface{}{
					"code":    statusCode,
					"message": http.StatusText(statusCode),
					"success": false,
				}

				c.Json(statusCode, errorResponse)
			}
		}
	}
}

// isAPIPath 检查是否为API路径
func isAPIPath(path string) bool {
	return len(path) > 4 && path[:4] == "/api"
}

// AppErrorMiddleware 应用错误处理中间件
func AppErrorMiddleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		c.Next()

		// 检查是否有错误对象存储在上下文中
		errObj, exists := c.Get("error")
		if exists {
			if appErr, ok := errObj.(*errors.AppError); ok {
				statusCode := appErr.Code
				if statusCode < 400 || statusCode > 599 {
					statusCode = 500
				}

				response := map[string]interface{}{
					"code":    appErr.Code,
					"message": appErr.Message,
					"success": false,
				}

				if appErr.Details != "" {
					response["details"] = appErr.Details
				}

				c.Json(statusCode, response)
			}
		}
	}
}
