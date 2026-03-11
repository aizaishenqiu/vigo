package idl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// APIDefinition API 定义
type APIDefinition struct {
	Name        string     `yaml:"name"`
	Version     string     `yaml:"version"`
	Description string     `yaml:"description"`
	BasePath    string     `yaml:"basePath"`
	Schemes     []string   `yaml:"schemes"`
	Consumes    []string   `yaml:"consumes"`
	Produces    []string   `yaml:"produces"`
	Paths       []PathItem `yaml:"paths"`
	Models      []ModelDef `yaml:"models"`
	Tags        []Tag      `yaml:"tags"`
}

// PathItem 路径项
type PathItem struct {
	Path        string      `yaml:"path"`
	Method      string      `yaml:"method"`
	Summary     string      `yaml:"summary"`
	Description string      `yaml:"description"`
	OperationID string      `yaml:"operationId"`
	Tags        []string    `yaml:"tags"`
	Parameters  []Parameter `yaml:"parameters"`
	Responses   []Response  `yaml:"responses"`
	Deprecated  bool        `yaml:"deprecated"`
}

// Parameter 参数定义
type Parameter struct {
	Name        string      `yaml:"name"`
	In          string      `yaml:"in"` // path, query, header, body
	Required    bool        `yaml:"required"`
	Type        string      `yaml:"type"`
	Format      string      `yaml:"format"`
	Default     interface{} `yaml:"default"`
	Description string      `yaml:"description"`
	Min         *int        `yaml:"min"`
	Max         *int        `yaml:"max"`
	Pattern     string      `yaml:"pattern"`
	Enum        []string    `yaml:"enum"`
}

// Response 响应定义
type Response struct {
	StatusCode  string                 `yaml:"statusCode"`
	Description string                 `yaml:"description"`
	Schema      *Schema                `yaml:"schema"`
	Headers     map[string]string      `yaml:"headers"`
	Examples    map[string]interface{} `yaml:"examples"`
}

// Schema 数据结构
type Schema struct {
	Type       string              `yaml:"type"`
	Format     string              `yaml:"format"`
	Ref        string              `yaml:"$ref"`
	Items      *Schema             `yaml:"items"`
	Properties map[string]Property `yaml:"properties"`
	Required   []string            `yaml:"required"`
}

// Property 属性定义
type Property struct {
	Type        string      `yaml:"type"`
	Format      string      `yaml:"format"`
	Ref         string      `yaml:"$ref"`
	Description string      `yaml:"description"`
	Default     interface{} `yaml:"default"`
	Required    bool        `yaml:"required"`
}

// ModelDef 模型定义
type ModelDef struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Properties  map[string]Property `yaml:"properties"`
	Required    []string            `yaml:"required"`
}

// Tag 标签定义
type Tag struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// InitProject 初始化 IDL 项目
func InitProject(dir string) error {
	// 创建目录结构
	dirs := []string{
		filepath.Join(dir, "apis"),
		filepath.Join(dir, "models"),
		filepath.Join(dir, "examples"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("创建目录失败：%w", err)
		}
	}

	// 创建示例 API 文件
	apiFile := filepath.Join(dir, "apis", "user.api")
	apiContent := `# 用户管理 API
name: user-api
version: v1
basePath: /api/v1
schemes:
  - http
  - https
consumes:
  - application/json
produces:
  - application/json

tags:
  - name: user
    description: 用户管理

paths:
  - path: /users
    method: GET
    summary: 获取用户列表
    operationId: listUsers
    tags:
      - user
    parameters:
      - name: page
        in: query
        type: integer
        default: 1
      - name: pageSize
        in: query
        type: integer
        default: 10
    responses:
      - statusCode: "200"
        description: 成功
        schema:
          type: array
          items:
            $ref: "#/models/User"

  - path: /users
    method: POST
    summary: 创建用户
    operationId: createUser
    tags:
      - user
    parameters:
      - name: body
        in: body
        required: true
        schema:
          $ref: "#/models/CreateUserRequest"
    responses:
      - statusCode: "201"
        description: 创建成功
        schema:
          $ref: "#/models/User"

models:
  - name: User
    description: 用户信息
    properties:
      id:
        type: string
        format: uuid
        description: 用户 ID
      username:
        type: string
        description: 用户名
        required: true
      email:
        type: string
        format: email
        description: 邮箱
        required: true
      createdAt:
        type: string
        format: date-time
        description: 创建时间

  - name: CreateUserRequest
    description: 创建用户请求
    properties:
      username:
        type: string
        required: true
        min: 3
        max: 32
      email:
        type: string
        required: true
      password:
        type: string
        required: true
        min: 6
`

	if err := os.WriteFile(apiFile, []byte(apiContent), 0644); err != nil {
		return fmt.Errorf("创建示例文件失败：%w", err)
	}

	// 创建 README
	readmeFile := filepath.Join(dir, "README.md")
	readmeContent := `# IDL 项目

## 目录结构

- apis/ - API 定义文件
- models/ - 共享模型定义
- examples/ - 示例配置

## 语法说明

### API 定义

` + "```" + `yaml
name: api-name          # API 名称
version: v1             # 版本号
basePath: /api/v1       # 基础路径
schemes:                # 支持的协议
  - http
  - https
paths:
  - path: /resource     # 路径
    method: GET         # HTTP 方法
    summary: 描述       # 简要描述
    operationId: getId  # 操作 ID（用于生成代码）
    parameters:         # 参数
      - name: id
        in: path
        type: string
        required: true
    responses:          # 响应
      - statusCode: "200"
        schema:
          $ref: "#/models/ModelName"

models:
  - name: ModelName
    properties:
      fieldName:
        type: string
        required: true
` + "```" + `

### 支持的类型

- string
- integer (int32, int64)
- number (float, double)
- boolean
- array
- object

### 参数位置

- path - 路径参数
- query - 查询参数
- header - 请求头
- body - 请求体
`

	if err := os.WriteFile(readmeFile, []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("创建 README 失败：%w", err)
	}

	fmt.Printf("✓ IDL 项目初始化成功：%s\n", dir)
	fmt.Printf("  - 创建示例 API: %s\n", apiFile)
	fmt.Printf("  - 创建 README: %s\n", readmeFile)

	return nil
}

// Validate 验证 IDL 文件
func Validate(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("读取文件失败：%w", err)
	}

	content := string(data)
	var errors []string

	// 基本验证
	if !strings.Contains(content, "name:") {
		errors = append(errors, "缺少 name 字段")
	}

	if !strings.Contains(content, "version:") {
		errors = append(errors, "缺少 version 字段")
	}

	if !strings.Contains(content, "paths:") {
		errors = append(errors, "缺少 paths 字段")
	}

	if len(errors) > 0 {
		fmt.Println("❌ 验证失败:")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("验证失败")
	}

	fmt.Println("✓ IDL 文件验证通过")
	return nil
}

// Parse 解析 IDL 文件
func Parse(file string) (*APIDefinition, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败：%w", err)
	}

	// 使用 YAML 解析器
	api := &APIDefinition{}

	// TODO: 使用 gopkg.in/yaml.v3 实现完整的 YAML 解析
	// 这里实现简化的解析逻辑

	// 按行解析
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析键值对
		if idx := strings.Index(line, ":"); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])

			switch key {
			case "name":
				api.Name = strings.Trim(value, "\"'")
			case "version":
				api.Version = strings.Trim(value, "\"'")
			case "description":
				api.Description = strings.Trim(value, "\"'")
			case "basePath":
				api.BasePath = strings.Trim(value, "\"'")
			case "schemes":
				api.Schemes = parseArray(value)
			case "consumes":
				api.Consumes = parseArray(value)
			case "produces":
				api.Produces = parseArray(value)
			}
		}
	}

	return api, nil
}

// parseArray 解析 YAML 数组
func parseArray(value string) []string {
	// 简化处理，支持 [a, b, c] 格式
	value = strings.Trim(value, "[]")
	if value == "" {
		return nil
	}

	items := strings.Split(value, ",")
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		item = strings.Trim(item, "\"'")
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
