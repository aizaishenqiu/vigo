package controller

import (
	"net/http"
	"runtime"
	"vigo/config"
	"vigo/framework/mvc"
)

// HomeController 首页控制器（测试用）
type HomeController struct {
	BaseController
}

// Index 首页视图（用于测试热重载）
func (c *HomeController) Index(ctx *mvc.Context) {
	ctx.HTML(http.StatusOK, "index/index.html", map[string]interface{}{
		"title":       config.App.App.Name + " - 热重载测试",
		"appName":     config.App.App.Name,
		"version":     config.App.App.Version,
		"mode":        config.App.App.Mode,
		"goVersion":   runtime.Version(),
		"port":        config.App.App.Port,
		"dbDriver":    config.App.Database.Driver,
		"dbHost":      config.App.Database.Host,
		"redisHost":   config.App.Redis.Host,
		"mqEnabled":   config.App.RabbitMQ.Enabled,
		"grpcEnabled": config.App.GRPC.Enabled,
		"isTest":      true,
	})
}
