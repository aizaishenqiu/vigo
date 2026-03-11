package controller

import (
	"net/http"
	"vigo/framework/mvc"
)

type AdminController struct {
	mvc.Controller
}

// Index 管理后台首页
func (c *AdminController) Index(ctx *mvc.Context) {
	ctx.HTML(http.StatusOK, "admin/dashboard.html", map[string]interface{}{
		"title": "控制台",
	})
}
