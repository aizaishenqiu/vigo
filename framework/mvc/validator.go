package mvc

import (
	"net/http"
	"regexp"
)

// Validator 验证器
type Validator struct {
	ctx *Context
}

// NewValidator 创建验证器
func NewValidator(c *Context) *Validator {
	return &Validator{ctx: c}
}

// Required 必填校验
func (v *Validator) Required(key string, msg string) string {
	val := v.ctx.Input(key)
	if val == "" {
		v.ctx.Fail(http.StatusBadRequest, 400, msg, nil)
		v.ctx.Abort()
	}
	return val
}

// Regex 正则校验
func (v *Validator) Regex(key string, pattern string, msg string) string {
	val := v.ctx.Input(key)
	matched, _ := regexp.MatchString(pattern, val)
	if !matched {
		v.ctx.Fail(http.StatusBadRequest, 400, msg, nil)
		v.ctx.Abort()
	}
	return val
}

// Email 邮箱校验
func (v *Validator) Email(key string, msg string) string {
	return v.Regex(key, `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`, msg)
}

// Mobile 手机号校验
func (v *Validator) Mobile(key string, msg string) string {
	return v.Regex(key, `^1[3-9]\d{9}$`, msg)
}
