package controller

import (
	"net/http"
	"vigo/config"
	"vigo/framework/mvc"
	"runtime"
)

// IndexController 首页控制器
type IndexController struct {
	BaseController
}

// Index 框架首页
func (c *IndexController) Index(ctx *mvc.Context) {
	ctx.HTML(http.StatusOK, "index/index.html", map[string]interface{}{
		"title":      config.App.App.Name + " - 控制面板",
		"appName":    config.App.App.Name,
		"version":    config.App.App.Version,
		"mode":       config.App.App.Mode,
		"goVersion":  runtime.Version(),
		"port":       config.App.App.Port,
		"dbDriver":   config.App.Database.Driver,
		"dbHost":     config.App.Database.Host,
		"redisHost":  config.App.Redis.Host,
		"mqEnabled":  config.App.RabbitMQ.Enabled,
		"grpcEnabled": config.App.GRPC.Enabled,
	})
}

// Hello Hello方法
// @Summary 打招呼接口
// @Tags 基础
// @Param name query string false "用户名"
// @Success 200 {object} map[string]string
// @Router /hello [get]
func (c *IndexController) Hello(ctx *mvc.Context) {
	c.Init(ctx)
	name := c.Input("name")
	if name == "" {
		name = "Guest"
	}
	c.Success(map[string]string{
		"message": "Hello " + name,
	})
}
