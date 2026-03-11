package mvc

import (
	"fmt"
	"regexp"
	"strings"
)

// RuleFunc 验证规则函数
type RuleFunc func(value string) bool

// RouteRule 路由规则配置（类似 TP 8.1.0-8.1.4 的路由验证）
type RouteRule struct {
	pattern     string                 // 路由模式
	validators  map[string]RuleFunc    // 字段验证规则
	enumRules   map[string][]string    // 枚举验证规则
	typeRules   map[string]string      // 类型转换规则（integer, float）
	middlewares []HandlerFunc          // 路由中间件
	name        string                 // 路由名称
}

// RouteRuleBuilder 路由规则构建器
type RouteRuleBuilder struct {
	rule  *RouteRule
	router *Router
	group *RouteGroup
}

// Middleware 添加中间件
func (rb *RouteRuleBuilder) Middleware(handlers ...HandlerFunc) *RouteRuleBuilder {
	rb.rule.middlewares = append(rb.rule.middlewares, handlers...)
	return rb
}

// When 路由变量验证方法（类似 TP 8.1.0 的 when 方法）
func (rb *RouteRuleBuilder) When(field string, rule string) *RouteRuleBuilder {
	// 支持预定义规则
	predefinedRules := map[string]RuleFunc{
		"id": func(value string) bool {
			_, err := regexp.MatchString(`^\d+$`, value)
			return err == nil
		},
		"name": func(value string) bool {
			_, err := regexp.MatchString(`^[a-zA-Z][a-zA-Z0-9_]*$`, value)
			return err == nil
		},
		"email": func(value string) bool {
			_, err := regexp.MatchString(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, value)
			return err == nil
		},
		"mobile": func(value string) bool {
			_, err := regexp.MatchString(`^1[3-9]\d{9}$`, value)
			return err == nil
		},
	}

	if ruleFunc, ok := predefinedRules[rule]; ok {
		rb.rule.validators[field] = ruleFunc
		return rb
	}

	// 支持正则规则
	if strings.HasPrefix(rule, "regex:") {
		pattern := strings.TrimPrefix(rule, "regex:")
		regex, err := regexp.Compile(pattern)
		if err == nil {
			rb.rule.validators[field] = func(value string) bool {
				return regex.MatchString(value)
			}
		}
		return rb
	}

	// 支持枚举规则 enum:0,1,2
	if strings.HasPrefix(rule, "enum:") {
		values := strings.Split(strings.TrimPrefix(rule, "enum:"), ",")
		rb.rule.enumRules[field] = values
		return rb
	}

	return rb
}

// Type 设置类型转换
func (rb *RouteRuleBuilder) Type(field string, typeName string) *RouteRuleBuilder {
	rb.rule.typeRules[field] = typeName
	return rb
}

// Name 设置路由名称
func (rb *RouteRuleBuilder) Name(name string) *RouteRuleBuilder {
	rb.rule.name = name
	return rb
}

// Register 注册路由
func (rb *RouteRuleBuilder) Register(method string, handler HandlerFunc) {
	// 创建验证中间件
	validateMiddleware := func(c *Context) {
		// 1. 验证路由变量
		for field, validator := range rb.rule.validators {
			value := c.Param(field)
			if value == "" {
				value = c.Request.URL.Query().Get(field)
			}
			if value == "" {
				value = c.Request.FormValue(field)
			}

			if value != "" && !validator(value) {
				c.Json(400, map[string]interface{}{
					"code":    400,
					"message": fmt.Sprintf("参数验证失败：%s 不符合规则", field),
				})
				c.Abort()
				return
			}
		}

		// 2. 验证枚举值
		for field, enumValues := range rb.rule.enumRules {
			value := c.Param(field)
			if value == "" {
				value = c.Request.URL.Query().Get(field)
			}
			if value == "" {
				value = c.Request.FormValue(field)
			}

			if value != "" {
				valid := false
				for _, enumValue := range enumValues {
					if value == enumValue {
						valid = true
						break
					}
				}
				if !valid {
					c.Json(400, map[string]interface{}{
						"code":    400,
						"message": fmt.Sprintf("参数验证失败：%s 必须是以下值之一：%v", field, enumValues),
					})
					c.Abort()
					return
				}
			}
		}

		// 3. 类型转换
		for field, typeName := range rb.rule.typeRules {
			value := c.Param(field)
			if value == "" {
				value = c.Request.URL.Query().Get(field)
			}
			if value == "" {
				value = c.Request.FormValue(field)
			}

			if value != "" {
				switch typeName {
				case "integer", "int":
					// 可以在这里进行类型转换并存储到 Context 中
					_ = value // 暂时不处理，留给 handler 使用
				case "float":
					_ = value
				}
			}
		}

		c.Next()
	}

	// 合并中间件
	allMiddlewares := append([]HandlerFunc{validateMiddleware}, rb.rule.middlewares...)

	// 注册路由
	if rb.group != nil {
		rb.group.router.addRouteWithMiddlewares(method, rb.rule.pattern, handler, allMiddlewares)
	} else if rb.router != nil {
		rb.router.addRouteWithMiddlewares(method, rb.rule.pattern, handler, allMiddlewares)
	}
}

// GET 注册 GET 路由
func (rb *RouteRuleBuilder) GET(handler HandlerFunc) {
	rb.Register("GET", handler)
}

// POST 注册 POST 路由
func (rb *RouteRuleBuilder) POST(handler HandlerFunc) {
	rb.Register("POST", handler)
}

// PUT 注册 PUT 路由
func (rb *RouteRuleBuilder) PUT(handler HandlerFunc) {
	rb.Register("PUT", handler)
}

// DELETE 注册 DELETE 路由
func (rb *RouteRuleBuilder) DELETE(handler HandlerFunc) {
	rb.Register("DELETE", handler)
}

// ANY 注册所有方法的路由
func (rb *RouteRuleBuilder) ANY(handler HandlerFunc) {
	rb.Register("GET", handler)
	rb.Register("POST", handler)
	rb.Register("PUT", handler)
	rb.Register("DELETE", handler)
	rb.Register("PATCH", handler)
	rb.Register("HEAD", handler)
	rb.Register("OPTIONS", handler)
}
