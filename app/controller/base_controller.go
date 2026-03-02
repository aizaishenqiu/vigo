package controller

import (
	"vigo/framework/mvc"
)

// BaseController 基础控制器
type BaseController struct {
	mvc.Controller
}

// Before 前置操作
func (c *BaseController) Before() {
	// 全局前置操作
}

// After 后置操作
func (c *BaseController) After() {
	// 全局后置操作
}
