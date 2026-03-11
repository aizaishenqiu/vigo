package controller

import (
	"net/http"
	"runtime"
	"time"
	"vigo/config"
	"vigo/framework/mvc"
)

// IndexController 首页控制器
type IndexController struct {
	BaseController
}

// Index 框架首页（科技王国控制台）
func (c *IndexController) Index(ctx *mvc.Context) {
	ctx.HTML(http.StatusOK, "index/index.html", map[string]interface{}{
		"title":        config.App.App.Name + " - 科技王国",
		"appName":      config.App.App.Name,
		"version":      config.App.App.Version,
		"mode":         config.App.App.Mode,
		"goVersion":    runtime.Version(),
		"port":         config.App.App.Port,
		"serverTime":   time.Now().Format("2006-01-02 15:04:05"),
		"numCPU":       runtime.NumCPU(),
		"numGoroutine": runtime.NumGoroutine(),
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
	})
}

// Hello 打招呼接口
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
