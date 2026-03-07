package main

import (
	"embed"
	"fmt"
	"vigo/config"
	_ "vigo/docs"
	"vigo/framework/app"
	"vigo/framework/view"
)

// 嵌入视图模板文件（打包进二进制，解决 Linux 部署找不到文件的问题）
// 使用通配符嵌入 view 目录下所有文件
//
//go:embed view/*
var viewFS embed.FS

// @title Vigo API
// @version 1.0
// @description 基于 Go 语言开发的极简、优雅的企业级 SaaS 框架 API 文档
// @termsOfService http://swagger.io/terms/

// @contact.name Vigo
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /
func main() {
	// 注入嵌入的视图文件系统到模板引擎
	view.SetEmbeddedViews(viewFS)

	// 1. 创建应用实例
	application := app.New(".")

	// 2. 初始化核心服务（包含：配置加载、端口检测、数据库、Redis、RabbitMQ、gRPC等）
	application.Initialize()

	// 3. 打印启动信息
	port := config.App.App.Port
	fmt.Printf("[Vigo] 启动 %s v%s (模式: %s)\n", config.App.App.Name, config.App.App.Version, application.Mode)

	// 4. 启动 HTTP 服务（同时启动 gRPC 如已启用，并支持优雅关闭）
	addr := fmt.Sprintf(":%d", port)
	if err := application.Run(addr); err != nil {
		// http.ErrServerClosed 是优雅关闭的正常结果
		if err.Error() != "http: Server closed" {
			fmt.Printf("[Vigo] 服务启动失败: %v\n", err)
			fmt.Println("按回车键退出...")
			fmt.Scanln()
		}
	}
}
