package controller

import (
	"vigo/framework/mvc"
	"time"
)

// HomeController 首页控制器
type HomeController struct {
	BaseController
}

// Index 首页视图
func (c *HomeController) Index(ctx *mvc.Context) {
	c.Init(ctx)

	data := map[string]interface{}{
		"Title":   "首页测试页面",
		"Message": "热更新测试：如果您看到这段话，说明 Air 热重载已生效23！",
		"Time":    time.Now().Format("2006-01-02 15:04:05"),
	}

	c.View("home/index.html", data)
}
