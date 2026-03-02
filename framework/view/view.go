package view

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Engine 视图引擎接口
type Engine interface {
	Render(w io.Writer, name string, data interface{}) error
}

// 全局嵌入的视图文件系统（由 main 包注入）
var embeddedViews embed.FS

// SetEmbeddedViews 设置嵌入的视图文件系统（main 包调用）
func SetEmbeddedViews(fsys embed.FS) {
	embeddedViews = fsys
}

// TemplateEngine 标准模板引擎
type TemplateEngine struct {
	dir      string
	suffix   string
	funcMap  template.FuncMap
	cache    map[string]*template.Template
	useCache bool
	useEmbed bool
}

// NewTemplateEngine 创建模板引擎
func NewTemplateEngine(dir string) *TemplateEngine {
	return &TemplateEngine{
		dir:    dir,
		suffix: ".html",
		funcMap: template.FuncMap{
			"raw": func(s string) template.HTML {
				return template.HTML(s)
			},
			"upper": strings.ToUpper,
			"lower": strings.ToLower,
			"add": func(a, b int) int {
				return a + b
			},
			"sub": func(a, b int) int {
				return a - b
			},
			"mul": func(a, b int) int {
				return a * b
			},
			"div": func(a, b int) int {
				if b == 0 {
					return 0
				}
				return a / b
			},
			"default": func(val, defaultVal interface{}) interface{} {
				if val == nil || val == "" {
					return defaultVal
				}
				return val
			},
			"safe": func(s string) template.HTML {
				return template.HTML(s)
			},
		},
		cache:    make(map[string]*template.Template),
		useCache: false,
		useEmbed: true,
	}
}

// AddFunc 添加自定义函数
func (e *TemplateEngine) AddFunc(name string, fn interface{}) {
	e.funcMap[name] = fn
}

// Render 渲染模板
func (e *TemplateEngine) Render(w io.Writer, name string, data interface{}) error {
	path := filepath.Join(e.dir, name)
	if !strings.HasSuffix(path, e.suffix) {
		path += e.suffix
	}

	var tmpl *template.Template
	var err error

	// 优先使用嵌入的文件系统
	if e.useEmbed && embeddedViews != (embed.FS{}) {
		tmpl, err = e.renderFromEmbed(path)
		if err == nil {
			return tmpl.Execute(w, data)
		}
		// 嵌入文件系统失败，尝试从磁盘读取
	}

	// 从磁盘读取
	tmpl, err = e.renderFromDisk(path)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, data)
}

// renderFromEmbed 从嵌入的文件系统渲染
func (e *TemplateEngine) renderFromEmbed(path string) (*template.Template, error) {
	// 嵌入文件系统使用正斜杠
	embedPath := strings.ReplaceAll(path, "\\", "/")

	// 检查缓存
	if e.useCache {
		if t, ok := e.cache[embedPath]; ok {
			return t, nil
		}
	}

	// 读取嵌入的模板文件
	content, err := fs.ReadFile(embeddedViews, embedPath)
	if err != nil {
		return nil, fmt.Errorf("嵌入文件系统找不到: %s", embedPath)
	}

	// 解析模板
	tmpl, err := template.New(filepath.Base(path)).Funcs(e.funcMap).Parse(string(content))
	if err != nil {
		return nil, err
	}

	if e.useCache {
		e.cache[embedPath] = tmpl
	}

	return tmpl, nil
}

// renderFromDisk 从磁盘渲染
func (e *TemplateEngine) renderFromDisk(path string) (*template.Template, error) {
	// 如果是相对路径，尝试转为绝对路径
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err == nil {
			path = absPath
		}
	}

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		cwd, _ := os.Getwd()
		return nil, fmt.Errorf("模板文件不存在: %s (工作目录: %s, 视图目录: %s)", path, cwd, e.dir)
	}

	// 检查缓存
	if e.useCache {
		if t, ok := e.cache[path]; ok {
			return t, nil
		}
	}

	tmpl, err := template.New(filepath.Base(path)).Funcs(e.funcMap).ParseFiles(path)
	if err != nil {
		return nil, err
	}

	if e.useCache {
		e.cache[path] = tmpl
	}

	return tmpl, nil
}

// SetUseCache 设置是否使用缓存
func (e *TemplateEngine) SetUseCache(useCache bool) {
	e.useCache = useCache
}

// SetUseEmbed 设置是否使用嵌入文件系统
func (e *TemplateEngine) SetUseEmbed(useEmbed bool) {
	e.useEmbed = useEmbed
}
