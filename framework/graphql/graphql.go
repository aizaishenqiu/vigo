package graphql

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// GraphQL GraphQL 处理器
type GraphQL struct {
	schema    *Schema
	resolvers map[string]ResolverFunc
}

// Schema GraphQL Schema
type Schema struct {
	Query        *ObjectType
	Mutation     *ObjectType
	Subscription *ObjectType
	Types        map[string]*ObjectType
	Directives   []*Directive
}

// ObjectType 对象类型
type ObjectType struct {
	Name   string
	Fields []*Field
}

// Field 字段定义
type Field struct {
	Name    string
	Type    Type
	Args    map[string]*Argument
	Resolve ResolverFunc
}

// Argument 参数定义
type Argument struct {
	Name         string
	Type         Type
	DefaultValue interface{}
}

// Type 类型接口
type Type interface {
	Name() string
	Kind() string
}

// ScalarType 标量类型
type ScalarType struct {
	typeName string
}

func (s *ScalarType) Name() string { return s.typeName }
func (s *ScalarType) Kind() string { return "SCALAR" }

// ObjectType 对象类型实现
type ObjectTypeType struct {
	typeName string
}

func (o *ObjectTypeType) Name() string { return o.typeName }
func (o *ObjectTypeType) Kind() string { return "OBJECT" }

// InputObjectType 输入对象类型
type InputObjectType struct {
	typeName string
	Fields   map[string]Type
}

func (i *InputObjectType) Name() string { return i.typeName }
func (i *InputObjectType) Kind() string { return "INPUT_OBJECT" }

// ListType 列表类型
type ListType struct {
	Elem Type
}

func (l *ListType) Name() string { return "[" + l.Elem.Name() + "]" }
func (l *ListType) Kind() string { return "LIST" }

// NonNullType 非空类型
type NonNullType struct {
	Elem Type
}

func (n *NonNullType) Name() string { return n.Elem.Name() + "!" }
func (n *NonNullType) Kind() string { return "NON_NULL" }

// Directive 指令
type Directive struct {
	Name      string
	Locations []string
	Args      map[string]*Argument
}

// ResolverFunc 解析函数
type ResolverFunc func(args map[string]interface{}) (interface{}, error)

// Request GraphQL 请求
type Request struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// Response GraphQL 响应
type Response struct {
	Data   interface{} `json:"data,omitempty"`
	Errors []Error     `json:"errors,omitempty"`
}

// Error GraphQL 错误
type Error struct {
	Message    string                 `json:"message"`
	Locations  []Location             `json:"locations,omitempty"`
	Path       []interface{}          `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Location 错误位置
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// NewGraphQL 创建 GraphQL 实例
func NewGraphQL() *GraphQL {
	return &GraphQL{
		schema: &Schema{
			Types: make(map[string]*ObjectType),
		},
		resolvers: make(map[string]ResolverFunc),
	}
}

// Handler HTTP 处理器
func (g *GraphQL) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request Request

		// 解析请求体
		if r.Method == http.MethodPost {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				g.sendError(w, "读取请求体失败")
				return
			}
			defer r.Body.Close()

			if err := json.Unmarshal(body, &request); err != nil {
				g.sendError(w, "解析 JSON 失败")
				return
			}
		} else if r.Method == http.MethodGet {
			request.Query = r.URL.Query().Get("query")
			request.OperationName = r.URL.Query().Get("operationName")
		} else {
			g.sendError(w, "不支持的请求方法")
			return
		}

		// 执行查询
		result, err := g.Execute(request)
		if err != nil {
			g.sendError(w, err.Error())
			return
		}

		// 返回响应
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

// Execute 执行 GraphQL 查询
func (g *GraphQL) Execute(request Request) (*Response, error) {
	// 解析查询
	ast, err := g.parseQuery(request.Query)
	if err != nil {
		return &Response{
			Errors: []Error{{Message: err.Error()}},
		}, nil
	}

	// 执行查询
	data := make(map[string]interface{})

	for _, op := range ast.Operations {
		var obj *ObjectType
		switch op.Type {
		case "query":
			obj = g.schema.Query
		case "mutation":
			obj = g.schema.Mutation
		default:
			return &Response{
				Errors: []Error{{Message: "不支持的操作类型"}},
			}, nil
		}

		result, err := g.executeSelection(op.Selections, obj, request.Variables)
		if err != nil {
			return &Response{
				Errors: []Error{{Message: err.Error()}},
			}, nil
		}

		// 合并结果
		for k, v := range result.(map[string]interface{}) {
			data[k] = v
		}
	}

	return &Response{
		Data: data,
	}, nil
}

// QueryAST 抽象语法树
type QueryAST struct {
	Operations []*Operation
}

// Operation 操作
type Operation struct {
	Type       string
	Name       string
	Variables  map[string]string
	Selections []Selection
}

// Selection 选择集
type Selection struct {
	Name       string
	Alias      string
	Arguments  map[string]interface{}
	Selections []Selection
}

// parseQuery 解析 GraphQL 查询（简化实现）
func (g *GraphQL) parseQuery(query string) (*QueryAST, error) {
	// 简化实现，实际需要完整的 GraphQL 解析器
	ast := &QueryAST{
		Operations: make([]*Operation, 0),
	}

	// 简单的查询解析
	query = strings.TrimSpace(query)

	if strings.HasPrefix(query, "query") {
		op := &Operation{Type: "query"}

		// 解析字段
		fields := g.parseFields(query)
		op.Selections = fields

		ast.Operations = append(ast.Operations, op)
	} else if strings.HasPrefix(query, "mutation") {
		op := &Operation{Type: "mutation"}

		fields := g.parseFields(query)
		op.Selections = fields

		ast.Operations = append(ast.Operations, op)
	}

	return ast, nil
}

// parseFields 解析字段（简化实现）
func (g *GraphQL) parseFields(query string) []Selection {
	selections := make([]Selection, 0)

	// 移除 query/mutation 关键字
	query = strings.TrimPrefix(query, "query")
	query = strings.TrimPrefix(query, "mutation")

	// 查找花括号
	start := strings.Index(query, "{")
	end := strings.LastIndex(query, "}")

	if start == -1 || end == -1 {
		return selections
	}

	content := query[start+1 : end]
	content = strings.TrimSpace(content)

	// 简单分割字段
	fields := strings.Split(content, "\n")
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || strings.HasPrefix(field, "#") {
			continue
		}

		selection := Selection{
			Name: field,
		}

		// 处理带参数的字段
		if idx := strings.Index(field, "("); idx != -1 {
			name := field[:idx]
			selection.Name = strings.TrimSpace(name)

			// 解析参数（简化）
			selection.Arguments = make(map[string]interface{})
		}

		selections = append(selections, selection)
	}

	return selections
}

// executeSelection 执行选择集
func (g *GraphQL) executeSelection(selections []Selection, obj *ObjectType, variables map[string]interface{}) (interface{}, error) {
	result := make(map[string]interface{})

	for _, sel := range selections {
		// 查找字段
		field := g.findField(obj, sel.Name)
		if field == nil {
			continue
		}

		// 执行解析器
		if field.Resolve != nil {
			value, err := field.Resolve(sel.Arguments)
			if err != nil {
				return nil, err
			}
			result[sel.Name] = value
		}
	}

	return result, nil
}

// findField 查找字段
func (g *GraphQL) findField(obj *ObjectType, name string) *Field {
	if obj == nil {
		return nil
	}

	for _, field := range obj.Fields {
		if field.Name == name {
			return field
		}
	}

	return nil
}

// sendError 发送错误响应
func (g *GraphQL) sendError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(Response{
		Errors: []Error{{Message: message}},
	})
}

// DefineType 定义类型
func (g *GraphQL) DefineType(name string, fields map[string]Type) *ObjectType {
	obj := &ObjectType{
		Name:   name,
		Fields: make([]*Field, 0),
	}

	for fieldName, fieldType := range fields {
		obj.Fields = append(obj.Fields, &Field{
			Name: fieldName,
			Type: fieldType,
		})
	}

	g.schema.Types[name] = obj
	return obj
}

// DefineQuery 定义查询
func (g *GraphQL) DefineQuery(name string, returnType Type, resolver ResolverFunc) {
	if g.schema.Query == nil {
		g.schema.Query = &ObjectType{
			Name:   "Query",
			Fields: make([]*Field, 0),
		}
	}

	g.schema.Query.Fields = append(g.schema.Query.Fields, &Field{
		Name:    name,
		Type:    returnType,
		Resolve: resolver,
	})
}

// DefineMutation 定义变更
func (g *GraphQL) DefineMutation(name string, returnType Type, resolver ResolverFunc) {
	if g.schema.Mutation == nil {
		g.schema.Mutation = &ObjectType{
			Name:   "Mutation",
			Fields: make([]*Field, 0),
		}
	}

	g.schema.Mutation.Fields = append(g.schema.Mutation.Fields, &Field{
		Name:    name,
		Type:    returnType,
		Resolve: resolver,
	})
}

// String 字符串类型
var String = &ScalarType{typeName: "String"}

// Int 整数类型
var Int = &ScalarType{typeName: "Int"}

// Float 浮点数类型
var Float = &ScalarType{typeName: "Float"}

// Boolean 布尔类型
var Boolean = &ScalarType{typeName: "Boolean"}

// ID ID 类型
var ID = &ScalarType{typeName: "ID"}

// List 列表类型
func List(elem Type) Type {
	return &ListType{Elem: elem}
}

// NonNull 非空类型
func NonNull(elem Type) Type {
	return &NonNullType{Elem: elem}
}

// NewObject 创建对象类型
func NewObject(name string) *ObjectTypeType {
	return &ObjectTypeType{typeName: name}
}

// InputObject 输入对象类型
type InputObjectBuilder struct {
	name   string
	fields map[string]Type
}

func NewInputObject(name string) *InputObjectBuilder {
	return &InputObjectBuilder{
		name:   name,
		fields: make(map[string]Type),
	}
}

func (b *InputObjectBuilder) Field(name string, t Type) *InputObjectBuilder {
	b.fields[name] = t
	return b
}

func (b *InputObjectBuilder) Build() *InputObjectType {
	return &InputObjectType{
		typeName: b.name,
		Fields:   b.fields,
	}
}

// Middleware GraphQL 中间件
type Middleware func(next ResolverFunc) ResolverFunc

// Use 使用中间件
func (g *GraphQL) Use(middleware Middleware) {
	// 实现中间件逻辑
}

// Playground GraphQL Playground
func (g *GraphQL) Playground() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`
<!DOCTYPE html>
<html>
<head>
  <meta charset=utf-8/>
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>GraphQL Playground</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/css/index.css" />
  <link rel="shortcut icon" href="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/favicon.png" />
  <script src="https://cdn.jsdelivr.net/npm/graphql-playground-react/build/static/js/middleware.js"></script>
</head>
<body>
  <div id="root">
    <style>
      body { background-color: rgb(23, 42, 58); font-family: Open Sans, sans-serif; height: 90vh; }
      #root { height: 100%; width: 100%; display: flex; align-items: center; justify-content: center; }
      .loading { font-size: 32px; font-weight: 200; color: rgba(255, 255, 255, .6); margin-left: 28px; }
      img { width: 78px; height: 78px; }
      .title { font-weight: 400; }
    </style>
    <img src='https://cdn.jsdelivr.net/npm/graphql-playground-react/build/logo.png' alt='GraphQL Playground Logo'>
    <div class="loading">
      Loading <span class="title">GraphQL Playground</span>
    </div>
  </div>
  <script>window.addEventListener('load', function (event) {
      GraphQLPlayground.init(document.getElementById('root'), {
        endpoint: '/graphql',
      })
    })</script>
</body>
</html>
`))
	}
}

// SchemaIntrospection Schema 内省
func (g *GraphQL) SchemaIntrospection() map[string]interface{} {
	schemaInfo := map[string]interface{}{
		"__schema": map[string]interface{}{
			"queryType": map[string]interface{}{
				"name": "Query",
			},
			"mutationType": map[string]interface{}{
				"name": "Mutation",
			},
			"types": g.getTypesInfo(),
		},
	}

	return schemaInfo
}

// getTypesInfo 获取类型信息
func (g *GraphQL) getTypesInfo() []map[string]interface{} {
	types := make([]map[string]interface{}, 0)

	// 内置标量类型
	builtinTypes := []string{"String", "Int", "Float", "Boolean", "ID"}
	for _, typeName := range builtinTypes {
		types = append(types, map[string]interface{}{
			"name": typeName,
			"kind": "SCALAR",
		})
	}

	// 自定义类型
	for _, obj := range g.schema.Types {
		fields := make([]map[string]interface{}, 0)
		for _, field := range obj.Fields {
			fields = append(fields, map[string]interface{}{
				"name": field.Name,
				"type": map[string]interface{}{
					"name": field.Type.Name(),
					"kind": field.Type.Kind(),
				},
			})
		}

		types = append(types, map[string]interface{}{
			"name":   obj.Name,
			"kind":   "OBJECT",
			"fields": fields,
		})
	}

	return types
}
