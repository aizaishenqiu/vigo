package devtool

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// DevTool 开发者工具包
type DevTool struct {
	appName string
	rootDir string
}

// NewDevTool 创建开发者工具
func NewDevTool(appName string) *DevTool {
	rootDir, _ := os.Getwd()
	return &DevTool{
		appName: appName,
		rootDir: rootDir,
	}
}

// CreateController 创建控制器
func (dt *DevTool) CreateController(name string) error {
	// 创建目录
	controllerDir := filepath.Join(dt.rootDir, "app", "controllers")
	if err := os.MkdirAll(controllerDir, 0755); err != nil {
		return err
	}

	// 生成控制器代码
	tmpl := `package controllers

import (
	"vigo/framework/mvc"
)

// {{.Name}}Controller {{.Name}} 控制器
type {{.Name}}Controller struct {
	mvc.Controller
}

// New{{.Name}}Controller 创建控制器实例
func New{{.Name}}Controller() *{{.Name}}Controller {
	return &{{.Name}}Controller{}
}

// Index 首页
func (c *{{.Name}}Controller) Index(ctx *mvc.Context) {
	ctx.Json(200, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    "Welcome to {{.Name}}",
	})
}

// Show 详情
func (c *{{.Name}}Controller) Show(ctx *mvc.Context) {
	id := ctx.Param("id")
	ctx.Json(200, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    map[string]interface{}{"id": id},
	})
}

// Store 创建
func (c *{{.Name}}Controller) Store(ctx *mvc.Context) {
	// TODO: 实现创建逻辑
	ctx.Json(200, map[string]interface{}{
		"code":    0,
		"message": "创建成功",
	})
}

// Update 更新
func (c *{{.Name}}Controller) Update(ctx *mvc.Context) {
	id := ctx.Param("id")
	// TODO: 实现更新逻辑
	ctx.Json(200, map[string]interface{}{
		"code":    0,
		"message": "更新成功",
		"data":    map[string]interface{}{"id": id},
	})
}

// Delete 删除
func (c *{{.Name}}Controller) Delete(ctx *mvc.Context) {
	id := ctx.Param("id")
	// TODO: 实现删除逻辑
	ctx.Json(200, map[string]interface{}{
		"code":    0,
		"message": "删除成功",
		"data":    map[string]interface{}{"id": id},
	})
}
`

	data := map[string]string{
		"Name": strings.Title(name),
	}

	t := template.Must(template.New("controller").Parse(tmpl))
	filePath := filepath.Join(controllerDir, fmt.Sprintf("%s_controller.go", strings.ToLower(name)))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, data)
}

// CreateModel 创建模型
func (dt *DevTool) CreateModel(name string) error {
	modelDir := filepath.Join(dt.rootDir, "app", "models")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return err
	}

	tmpl := `package models

import (
	"vigo/framework/model"
)

// {{.Name}} {{.Name}} 模型
type {{.Name}} struct {
	*model.Model
	ID       int64  ` + "`json:\"id\"`" + `
	// TODO: 添加字段
	CreatedAt int64 ` + "`json:\"created_at\"`" + `
	UpdatedAt int64 ` + "`json:\"updated_at\"`" + `
}

// New{{.Name}} 创建模型实例
func New{{.Name}}() *{{.Name}} {
	m := &{{.Name}}{
		Model: model.New("{{.Table}}"),
	}
	return m
}

// Find 查找单个记录
func (m *{{.Name}}) Find(id int64) *{{.Name}} {
	m.Model.Find(id)
	return m
}

// Create 创建记录
func (m *{{.Name}}) Create(data map[string]interface{}) error {
	_, err := m.Model.Insert(data)
	return err
}

// Lists 查询列表
func (m *{{.Name}}) Lists(page, pageSize int) ([]map[string]interface{}, int64, error) {
	query := m.Model.NewQuery()
	
	total, err := query.Count()
	if err != nil {
		return nil, 0, err
	}
	
	offset := (page - 1) * pageSize
	list, err := query.
		Limit(pageSize).
		Offset(offset).
		OrderBy("id", "DESC").
		Select()
	
	return list, total, err
}
`

	data := map[string]string{
		"Name":  strings.Title(name),
		"Table": strings.ToLower(name) + "s",
	}

	t := template.Must(template.New("model").Parse(tmpl))
	filePath := filepath.Join(modelDir, fmt.Sprintf("%s.go", strings.ToLower(name)))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, data)
}

// CreateService 创建服务层
func (dt *DevTool) CreateService(name string) error {
	serviceDir := filepath.Join(dt.rootDir, "app", "services")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return err
	}

	tmpl := `package services

import (
	"vigo/app/models"
)

// {{.Name}}Service {{.Name}} 服务
type {{.Name}}Service struct {
	model *models.{{.Name}}
}

// New{{.Name}}Service 创建服务实例
func New{{.Name}}Service() *{{.Name}}Service {
	return &{{.Name}}Service{
		model: models.New{{.Name}}(),
	}
}

// GetByID 根据 ID 获取
func (s *{{.Name}}Service) GetByID(id int64) (*models.{{.Name}}, error) {
	return s.model.Find(id), nil
}

// Create 创建
func (s *{{.Name}}Service) Create(data map[string]interface{}) error {
	return s.model.Create(data)
}

// Lists 查询列表
func (s *{{.Name}}Service) Lists(page, pageSize int) ([]map[string]interface{}, int64, error) {
	return s.model.Lists(page, pageSize)
}

// Update 更新
func (s *{{.Name}}Service) Update(id int64, data map[string]interface{}) error {
	// TODO: 实现更新逻辑
	return nil
}

// Delete 删除
func (s *{{.Name}}Service) Delete(id int64) error {
	// TODO: 实现删除逻辑
	return nil
}
`

	data := map[string]string{
		"Name": strings.Title(name),
	}

	t := template.Must(template.New("service").Parse(tmpl))
	filePath := filepath.Join(serviceDir, fmt.Sprintf("%s_service.go", strings.ToLower(name)))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, data)
}

// CreateMigration 创建迁移文件
func (dt *DevTool) CreateMigration(name string) error {
	migrationDir := filepath.Join(dt.rootDir, "database", "migrations")
	if err := os.MkdirAll(migrationDir, 0755); err != nil {
		return err
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	tmpl := `package migrations

import (
	"vigo/framework/db"
)

// {{.Name}} 迁移
type {{.Name}} struct{}

// Up 执行迁移
func (m *{{.Name}}) Up() error {
	sql := ` + "`" + `
	CREATE TABLE IF NOT EXISTS {{.Table}} (
		id BIGINT PRIMARY KEY AUTO_INCREMENT,
		created_at BIGINT,
		updated_at BIGINT
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	` + "`" + `
	
	_, err := db.GlobalDB.Exec(sql)
	return err
}

// Down 回滚迁移
func (m *{{.Name}}) Down() error {
	sql := ` + "`DROP TABLE IF EXISTS {{.Table}}`" + `
	_, err := db.GlobalDB.Exec(sql)
	return err
}
`

	data := map[string]string{
		"Name":  strings.Title(name) + "Migration",
		"Table": strings.ToLower(name) + "s",
	}

	t := template.Must(template.New("migration").Parse(tmpl))
	filePath := filepath.Join(migrationDir, fmt.Sprintf("%s_%s.go", timestamp, strings.ToLower(name)))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, data)
}

// CreateMiddleware 创建中间件
func (dt *DevTool) CreateMiddleware(name string) error {
	middlewareDir := filepath.Join(dt.rootDir, "app", "middleware")
	if err := os.MkdirAll(middlewareDir, 0755); err != nil {
		return err
	}

	tmpl := `package middleware

import (
	"vigo/framework/mvc"
)

// {{.Name}}Middleware {{.Name}} 中间件
func {{.Name}}Middleware() mvc.HandlerFunc {
	return func(c *mvc.Context) {
		// TODO: 实现中间件逻辑
		
		// 继续处理请求
		c.Next()
		
		// TODO: 响应后处理
	}
}
`

	data := map[string]string{
		"Name": strings.Title(name),
	}

	t := template.Must(template.New("middleware").Parse(tmpl))
	filePath := filepath.Join(middlewareDir, fmt.Sprintf("%s_middleware.go", strings.ToLower(name)))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, data)
}

// CreateValidator 创建验证器
func (dt *DevTool) CreateValidator(name string) error {
	validatorDir := filepath.Join(dt.rootDir, "app", "validators")
	if err := os.MkdirAll(validatorDir, 0755); err != nil {
		return err
	}

	tmpl := `package validators

import (
	"vigo/framework/validate"
)

// {{.Name}}Validator {{.Name}} 验证器
type {{.Name}}Validator struct {
	validate.ValidateRuleSet
}

// New{{.Name}}Validator 创建验证器实例
func New{{.Name}}Validator() *{{.Name}}Validator {
	v := &{{.Name}}Validator{}
	v.initRules()
	return v
}

// initRules 初始化验证规则
func (v *{{.Name}}Validator) initRules() {
	// TODO: 添加验证规则
	// v.Rule("field", "required|min:3|max:100")
}

// Validate 验证数据
func (v *{{.Name}}Validator) Validate(data map[string]interface{}) error {
	return v.ValidateRuleSet.Validate(data)
}
`

	data := map[string]string{
		"Name": strings.Title(name),
	}

	t := template.Must(template.New("validator").Parse(tmpl))
	filePath := filepath.Join(validatorDir, fmt.Sprintf("%s_validator.go", strings.ToLower(name)))

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return t.Execute(file, data)
}

// Scaffold 一键生成 CRUD
func (dt *DevTool) Scaffold(name string) error {
	fmt.Printf("正在生成 %s 的 CRUD 代码...\n", name)

	// 生成模型
	fmt.Println("  创建 Model...")
	if err := dt.CreateModel(name); err != nil {
		return err
	}

	// 生成控制器
	fmt.Println("  创建 Controller...")
	if err := dt.CreateController(name); err != nil {
		return err
	}

	// 生成服务层
	fmt.Println("  创建 Service...")
	if err := dt.CreateService(name); err != nil {
		return err
	}

	// 生成验证器
	fmt.Println("  创建 Validator...")
	if err := dt.CreateValidator(name); err != nil {
		return err
	}

	// 生成迁移文件
	fmt.Println("  创建 Migration...")
	if err := dt.CreateMigration(name); err != nil {
		return err
	}

	fmt.Printf("✅ %s 的 CRUD 代码生成完成！\n", name)
	fmt.Println("")
	fmt.Println("下一步:")
	fmt.Println("  1. 完善 Model 字段定义")
	fmt.Println("  2. 完善 Validator 验证规则")
	fmt.Println("  3. 完善 Service 业务逻辑")
	fmt.Println("  4. 添加路由配置")
	fmt.Println("  5. 运行数据库迁移")

	return nil
}
