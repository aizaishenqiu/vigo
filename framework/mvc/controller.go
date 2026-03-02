package mvc

import "net/http"

// Controller 基础控制器结构体
type Controller struct {
	Ctx *Context
}

// Init 初始化控制器
func (c *Controller) Init(ctx *Context) {
	c.Ctx = ctx
}

// Request 获取请求对象
func (c *Controller) Request() *Context {
	return c.Ctx
}

// Input 获取请求参数
func (c *Controller) Input(key string) string {
	return c.Ctx.Input(key)
}

// Success 成功响应
func (c *Controller) Success(data interface{}) {
	c.Ctx.Success(data)
}

// Error 失败响应
func (c *Controller) Error(code int, msg string) {
	c.Ctx.Error(code, msg)
}

// Json JSON响应
func (c *Controller) Json(code int, data interface{}) {
	c.Ctx.Json(code, data)
}

// View 视图响应
func (c *Controller) View(name string, data interface{}) {
	c.Ctx.HTML(http.StatusOK, name, data)
}

// Redirect 重定向
func (c *Controller) Redirect(url string) {
	http.Redirect(c.Ctx.Writer, c.Ctx.Request, url, http.StatusFound)
}

// Assign 模板变量赋值 (保留方法，实际直接传给View)
func (c *Controller) Assign(key string, value interface{}) {
	// 暂未实现全局模板变量池，建议直接在 View 方法传递 data
}
