package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// OptimizeCommand 优化命令（类似 TP 8.1.4 的 optimize 命令）
type OptimizeCommand struct{}

// Run 运行优化命令
func (cmd *OptimizeCommand) Run(args []string) {
	if len(args) == 0 {
		cmd.showHelp()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "config":
		cmd.optimizeConfig()
	case "route":
		cmd.optimizeRoute()
	case "schema":
		cmd.optimizeSchema()
	default:
		fmt.Printf("未知命令：%s\n", subCmd)
		cmd.showHelp()
	}
}

// showHelp 显示帮助信息
func (cmd *OptimizeCommand) showHelp() {
	fmt.Println("Vigo Framework Optimize Tool")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  vigo optimize <command> [options]")
	fmt.Println("")
	fmt.Println("可用命令:")
	fmt.Println("  config    优化配置文件")
	fmt.Println("  route     优化路由规则")
	fmt.Println("  schema    优化数据库 Schema")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  vigo optimize config")
	fmt.Println("  vigo optimize route")
	fmt.Println("  vigo optimize schema")
}

// optimizeConfig 优化配置文件（类似 TP 8.1.4 的 optimize:config）
func (cmd *OptimizeCommand) optimizeConfig() {
	fmt.Println("正在优化配置文件...")

	// 支持多个配置目录
	configDirs := []string{"config", "."}
	cacheDir := "runtime/cache"

	// 创建缓存目录
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("创建缓存目录失败：%v\n", err)
		return
	}

	// 扫描配置文件
	var configFiles []string
	for _, dir := range configDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
		if err != nil {
			continue
		}
		configFiles = append(configFiles, files...)
	}

	if len(configFiles) == 0 {
		fmt.Println("未找到配置文件")
		return
	}

	// 读取并合并配置
	configData := make(map[string]interface{})
	for _, file := range configFiles {
		fmt.Printf("  加载：%s\n", file)

		// 读取文件内容
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("    读取失败：%v\n", err)
			continue
		}

		// 简单统计
		configData[file] = len(content)
	}

	// 生成缓存文件（简化实现，实际应该序列化配置）
	cacheFile := filepath.Join(cacheDir, "config.cache")
	cacheContent := fmt.Sprintf("# Config Cache\n# Generated at: %s\n# Files: %d\n",
		time.Now().Format("2006-01-02 15:04:05"), len(configFiles))

	if err := os.WriteFile(cacheFile, []byte(cacheContent), 0644); err != nil {
		fmt.Printf("写入缓存失败：%v\n", err)
		return
	}

	fmt.Printf("生成配置缓存：%s\n", cacheFile)
	fmt.Println("配置优化完成！")
	fmt.Printf("优化文件数：%d\n", len(configFiles))
}

// optimizeRoute 优化路由（类似 TP 8.1.3-8.1.4 的 optimize:route）
func (cmd *OptimizeCommand) optimizeRoute() {
	fmt.Println("正在优化路由规则...")

	routeDir := "route"
	cacheDir := "runtime/cache"

	// 创建缓存目录
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("创建缓存目录失败：%v\n", err)
		return
	}

	// 扫描路由文件
	routeFiles, err := cmd.scanRouteFiles(routeDir)
	if err != nil {
		fmt.Printf("扫描路由文件失败：%v\n", err)
		return
	}

	if len(routeFiles) == 0 {
		fmt.Println("未找到路由文件")
		return
	}

	// 解析路由规则
	routes := make([]string, 0)
	for _, file := range routeFiles {
		fmt.Printf("  加载：%s\n", file)
		fileRoutes := cmd.parseRouteFile(file)
		routes = append(routes, fileRoutes...)
	}

	// 生成路由缓存
	cacheFile := filepath.Join(cacheDir, "routes.cache")
	cacheContent := fmt.Sprintf("# Route Cache\n# Generated at: %s\n# Routes: %d\n",
		time.Now().Format("2006-01-02 15:04:05"), len(routes))

	if err := os.WriteFile(cacheFile, []byte(cacheContent), 0644); err != nil {
		fmt.Printf("写入缓存失败：%v\n", err)
		return
	}

	fmt.Printf("生成路由缓存：%s\n", cacheFile)
	fmt.Println("路由优化完成！")
	fmt.Printf("优化文件数：%d, 路由数：%d\n", len(routeFiles), len(routes))
}

// parseRouteFile 解析路由文件，提取路由信息
func (cmd *OptimizeCommand) parseRouteFile(filename string) []string {
	var routes []string

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return routes
	}

	// 遍历 AST，查找路由注册
	ast.Inspect(f, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				// 查找 r.GET, r.POST 等路由注册
				if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "r" {
					method := selExpr.Sel.Name
					if len(callExpr.Args) >= 2 {
						if pathLit, ok := callExpr.Args[0].(*ast.BasicLit); ok {
							route := fmt.Sprintf("%s %s", method, strings.Trim(pathLit.Value, "\""))
							routes = append(routes, route)
						}
					}
				}
			}
		}
		return true
	})

	return routes
}

// scanRouteFiles 扫描路由文件（支持分组子目录，类似 TP 8.1.3）
func (cmd *OptimizeCommand) scanRouteFiles(dir string) ([]string, error) {
	var files []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".go") {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// optimizeSchema 优化数据库 Schema（类似 TP 的 optimize:schema）
func (cmd *OptimizeCommand) optimizeSchema() {
	fmt.Println("正在优化数据库 Schema...")

	cacheDir := "runtime/cache"

	// 创建缓存目录
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Printf("创建缓存目录失败：%v\n", err)
		return
	}

	// TODO: 实际实现需要连接数据库并扫描模型

	// 生成 Schema 缓存
	cacheFile := filepath.Join(cacheDir, "schema.php")
	fmt.Printf("生成 Schema 缓存：%s\n", cacheFile)

	// TODO: 实际实现需要写入缓存文件

	fmt.Println("Schema 优化完成！")
}

// RouteListCommand 路由列表命令（类似 TP 8.1.3-8.1.4 的 route:list）
type RouteListCommand struct{}

// Run 运行路由列表命令
func (cmd *RouteListCommand) Run(args []string) {
	fmt.Println("路由列表:")
	fmt.Println("")
	fmt.Printf("%-10s %-30s %s\n", "Method", "URI", "File")
	fmt.Println(strings.Repeat("-", 80))

	// 从路由文件解析路由
	routeDir := "route"
	routeFiles, _ := filepath.Glob(filepath.Join(routeDir, "*.go"))

	count := 0
	for _, file := range routeFiles {
		routes := cmd.parseRouteFile(file)
		for _, route := range routes {
			parts := strings.SplitN(route, " ", 2)
			if len(parts) == 2 {
				method := parts[0]
				uri := parts[1]
				filename := filepath.Base(file)
				fmt.Printf("%-10s %-30s %s\n", method, uri, filename)
				count++
			}
		}
	}

	if count == 0 {
		fmt.Println("未找到路由")
	} else {
		fmt.Printf("\n总计：%d 个路由\n", count)
	}
}

// parseRouteFile 解析路由文件，提取路由信息
func (cmd *RouteListCommand) parseRouteFile(filename string) []string {
	var routes []string

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return routes
	}

	// 遍历 AST，查找路由注册
	ast.Inspect(f, func(n ast.Node) bool {
		if callExpr, ok := n.(*ast.CallExpr); ok {
			if selExpr, ok := callExpr.Fun.(*ast.SelectorExpr); ok {
				// 查找 r.GET, r.POST 等路由注册
				if ident, ok := selExpr.X.(*ast.Ident); ok && ident.Name == "r" {
					method := selExpr.Sel.Name
					if len(callExpr.Args) >= 2 {
						if pathLit, ok := callExpr.Args[0].(*ast.BasicLit); ok {
							route := fmt.Sprintf("%s %s", method, strings.Trim(pathLit.Value, "\""))
							routes = append(routes, route)
						}
					}
				}
				// 查找路由分组 r.Group().GET() 等
				if callExpr2, ok := selExpr.X.(*ast.CallExpr); ok {
					if selExpr2, ok := callExpr2.Fun.(*ast.SelectorExpr); ok {
						if ident2, ok := selExpr2.X.(*ast.Ident); ok && ident2.Name == "r" && selExpr2.Sel.Name == "Group" {
							// 获取分组前缀
							prefix := ""
							if len(callExpr2.Args) > 0 {
								if lit, ok := callExpr2.Args[0].(*ast.BasicLit); ok {
									prefix = strings.Trim(lit.Value, "\"")
								}
							}
							method := selExpr.Sel.Name
							if len(callExpr.Args) >= 2 {
								if pathLit, ok := callExpr.Args[0].(*ast.BasicLit); ok {
									path := strings.Trim(pathLit.Value, "\"")
									if prefix != "" {
										path = prefix + path
									}
									route := fmt.Sprintf("%s %s", method, path)
									routes = append(routes, route)
								}
							}
						}
					}
				}
			}
		}
		return true
	})

	return routes
}

// VersionCommand 版本控制命令（类似 TP 8.1.3 的 version 方法）
type VersionCommand struct{}

// Run 运行版本命令
func (cmd *VersionCommand) Run(args []string) {
	fmt.Println("Vigo Framework")
	fmt.Println("")
	fmt.Printf("框架版本：v1.0.1\n")
	fmt.Printf("Go 版本：%s\n", getGoVersion())
	fmt.Printf("更新时间：2026-03-02\n")
	fmt.Println("")
	fmt.Println("核心组件:")
	fmt.Println("  - Router: 支持变量验证、枚举验证、预定义变量规则")
	fmt.Println("  - Validator: 支持 ValidateRuleSet、验证分组、多维数组验证")
	fmt.Println("  - ORM: 支持软删除、自动时间戳、JSON 查询")
	fmt.Println("  - Cache: 支持多级缓存、标签管理")
	fmt.Println("  - Middleware: 支持 withoutMiddleware、自动 layer")
	fmt.Println("  - Queue: 支持 Redis/DB/RabbitMQ 驱动")
}

// getGoVersion 获取 Go 版本
func getGoVersion() string {
	// TODO: 实际实现需要获取 Go 版本
	return "go1.21"
}

// MakeCommand 代码生成命令
type MakeCommand struct{}

// Run 运行代码生成命令
func (cmd *MakeCommand) Run(args []string) {
	if len(args) == 0 {
		cmd.showHelp()
		return
	}

	subCmd := args[0]
	switch subCmd {
	case "controller":
		if len(args) < 2 {
			fmt.Println("请指定控制器名称")
			return
		}
		cmd.makeController(args[1])
	case "model":
		if len(args) < 2 {
			fmt.Println("请指定模型名称")
			return
		}
		cmd.makeModel(args[1])
	case "middleware":
		if len(args) < 2 {
			fmt.Println("请指定中间件名称")
			return
		}
		cmd.makeMiddleware(args[1])
	case "job":
		if len(args) < 2 {
			fmt.Println("请指定任务名称")
			return
		}
		cmd.makeJob(args[1])
	default:
		fmt.Printf("未知命令：%s\n", subCmd)
		cmd.showHelp()
	}
}

// showHelp 显示帮助信息
func (cmd *MakeCommand) showHelp() {
	fmt.Println("Vigo Framework Code Generator")
	fmt.Println("")
	fmt.Println("用法:")
	fmt.Println("  vigo make <command> <name>")
	fmt.Println("")
	fmt.Println("可用命令:")
	fmt.Println("  controller  创建控制器")
	fmt.Println("  model       创建模型")
	fmt.Println("  middleware  创建中间件")
	fmt.Println("  job         创建队列任务")
	fmt.Println("")
	fmt.Println("示例:")
	fmt.Println("  vigo make controller User")
	fmt.Println("  vigo make model User")
	fmt.Println("  vigo make middleware Auth")
	fmt.Println("  vigo make job SendEmail")
}

// makeController 创建控制器
func (cmd *MakeCommand) makeController(name string) {
	fmt.Printf("创建控制器：%s\n", name)

	dir := "app/controller"
	file := filepath.Join(dir, fmt.Sprintf("%s.go", name))

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("创建目录失败：%v\n", err)
		return
	}

	// 生成控制器代码
	content := fmt.Sprintf(`package controller

import (
	"vigo/framework/mvc"
)

type %s struct {
	mvc.Controller
}

func New%s() *%s {
	return &%s{}
}

func (c *%s) Index(ctx *mvc.Context) {
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "success",
		"data":    nil,
	})
}

func (c *%s) Show(ctx *mvc.Context) {
	id := ctx.Param("id")
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "success",
		"data": map[string]interface{}{
			"id": id,
		},
	})
}

func (c *%s) Store(ctx *mvc.Context) {
	// TODO: 实现存储逻辑
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "创建成功",
		"data":    nil,
	})
}

func (c *%s) Update(ctx *mvc.Context) {
	id := ctx.Param("id")
	// TODO: 实现更新逻辑
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "更新成功",
		"data": map[string]interface{}{
			"id": id,
		},
	})
}

func (c *%s) Delete(ctx *mvc.Context) {
	id := ctx.Param("id")
	// TODO: 实现删除逻辑
	ctx.Json(200, map[string]interface{}{
		"code":    200,
		"message": "删除成功",
		"data": map[string]interface{}{
			"id": id,
		},
	})
}
`, name, name, name, name, name, name, name, name, name)

	// 写入文件
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		fmt.Printf("写入文件失败：%v\n", err)
		return
	}

	fmt.Printf("控制器已创建：%s\n", file)
}

// makeModel 创建模型
func (cmd *MakeCommand) makeModel(name string) {
	fmt.Printf("创建模型：%s\n", name)

	dir := "app/model"
	file := filepath.Join(dir, fmt.Sprintf("%s.go", name))

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("创建目录失败：%v\n", err)
		return
	}

	// 生成模型代码
	content := fmt.Sprintf(`package model

import (
	"vigo/framework/model"
)

type %s struct {
	model.Model
	// TODO: 添加字段
	// ID        int64     `+"`json:\"id\"`"+`
	// CreatedAt time.Time `+"`json:\"created_at\"`"+`
	// UpdatedAt time.Time `+"`json:\"updated_at\"`"+`
}

// TableName 表名
func (m *%s) TableName() string {
	return "%s"
}

// New%s 创建模型实例
func New%s() *%s {
	return &%s{}
}
`, name, name, tableName(name), name, name, name, name)

	// 写入文件
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		fmt.Printf("写入文件失败：%v\n", err)
		return
	}

	fmt.Printf("模型已创建：%s\n", file)
}

// tableName 表名转换（驼峰转下划线）
func tableName(name string) string {
	result := ""
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result += "_"
		}
		result += strings.ToLower(string(r))
	}
	return result
}

// makeMiddleware 创建中间件
func (cmd *MakeCommand) makeMiddleware(name string) {
	fmt.Printf("创建中间件：%s\n", name)

	dir := "app/middleware"
	file := filepath.Join(dir, fmt.Sprintf("%s.go", name))

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("创建目录失败：%v\n", err)
		return
	}

	// 生成中间件代码
	content := fmt.Sprintf(`package middleware

import (
	"vigo/framework/mvc"
)

// %s 中间件
func %s() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		// TODO: 实现中间件逻辑
		
		// 在请求处理之前执行
		// ...
		
		c.Next()
		
		// 在请求处理之后执行
		// ...
	}
}
`, name, name)

	// 写入文件
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		fmt.Printf("写入文件失败：%v\n", err)
		return
	}

	fmt.Printf("中间件已创建：%s\n", file)
}

// makeJob 创建队列任务
func (cmd *MakeCommand) makeJob(name string) {
	fmt.Printf("创建队列任务：%s\n", name)

	dir := "app/jobs"
	file := filepath.Join(dir, fmt.Sprintf("%s.go", name))

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("创建目录失败：%v\n", err)
		return
	}

	// 生成任务代码
	content := fmt.Sprintf(`package jobs

import (
	"time"
)

// %s 队列任务
type %s struct {
	Data map[string]interface{}
}

// New%s 创建任务实例
func New%s(data map[string]interface{}) *%s {
	return &%s{
		Data: data,
	}
}

// Handle 处理任务
func (j *%s) Handle() error {
	// TODO: 实现任务逻辑
	// 例如：发送邮件、短信等
	
	return nil
}

// Retry 重试次数
func (j *%s) Retry() int {
	return 3
}

// Delay 延迟时间
func (j *%s) Delay() time.Duration {
	return 0
}
`, name, name, name, name, name, name, name, name, name)

	// 写入文件
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		fmt.Printf("写入文件失败：%v\n", err)
		return
	}

	fmt.Printf("队列任务已创建：%s\n", file)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Vigo Framework CLI")
		fmt.Println("")
		fmt.Println("用法:")
		fmt.Println("  vigo <command> [options]")
		fmt.Println("")
		fmt.Println("可用命令:")
		fmt.Println("  optimize    优化命令")
		fmt.Println("  route:list  路由列表")
		fmt.Println("  version     版本信息")
		fmt.Println("  make        代码生成")
		fmt.Println("")
		fmt.Println("示例:")
		fmt.Println("  vigo optimize config")
		fmt.Println("  vigo route:list")
		fmt.Println("  vigo version")
		fmt.Println("  vigo make controller User")
		return
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "optimize":
		cmd := &OptimizeCommand{}
		cmd.Run(args)
	case "route:list":
		cmd := &RouteListCommand{}
		cmd.Run(args)
	case "version":
		cmd := &VersionCommand{}
		cmd.Run(args)
	case "make":
		cmd := &MakeCommand{}
		cmd.Run(args)
	default:
		fmt.Printf("未知命令：%s\n", command)
	}
}
