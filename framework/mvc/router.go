package mvc

import (
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"vigo/framework/radixtree"
)

// Router 路由结构体（支持动态参数和路由分组）
type Router struct {
	staticRoutes  map[string]map[string][]HandlerFunc // 精确匹配：path -> method -> handler chain
	dynamicRoutes []dynamicRoute                      // 动态参数路由（保留向后兼容）
	radixTree     *radixtree.RadixTree                // 高性能 Radix Tree 路由器
	fileHandlers  map[string]http.Handler             // 静态资源路由
	middlewares   []HandlerFunc                       // 全局中间件
	ReadTimeout   time.Duration                       // 读取超时 (Slow Loris 防护)
	pool          sync.Pool                           // Context 对象池
	viewEngine    Engine                              // 全局共享视图引擎
}

// Engine 视图引擎接口（与 view.Engine 签名一致，避免循环导入）
type Engine interface {
	Render(w io.Writer, name string, data interface{}) error
}

// dynamicRoute 动态路由项
type dynamicRoute struct {
	method   string
	pattern  string      // 原始模式：/user/:id
	parts    []routePart // 解析后的路由段
	handlers []HandlerFunc
}

type routePart struct {
	value   string
	isParam bool // :id
	isWild  bool // *filepath
}

// RouteGroup 路由分组
type RouteGroup struct {
	prefix             string
	router             *Router
	middlewares        []HandlerFunc
	excludeMiddlewares []string // 需要排除的中间件名称（类似 TP 8.1.0 的 withoutMiddleware）
}

// NewRouter 创建路由
func NewRouter() *Router {
	r := &Router{
		staticRoutes:  make(map[string]map[string][]HandlerFunc),
		dynamicRoutes: make([]dynamicRoute, 0),
		radixTree:     radixtree.New(), // 初始化 Radix Tree
		fileHandlers:  make(map[string]http.Handler),
		middlewares:   make([]HandlerFunc, 0),
		ReadTimeout:   10 * time.Second,
	}
	r.pool.New = func() interface{} {
		return &Context{
			Params: make(map[string]string),
		}
	}
	return r
}

// SetViewEngine 设置全局共享视图引擎（避免每个 Context 重复创建）
func (r *Router) SetViewEngine(engine Engine) {
	r.viewEngine = engine
}

// parseParts 解析路由模式为路由段
func parseParts(pattern string) []routePart {
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	parts := make([]routePart, 0, len(segments))
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, ":") {
			parts = append(parts, routePart{value: seg[1:], isParam: true})
		} else if strings.HasPrefix(seg, "*") {
			parts = append(parts, routePart{value: seg[1:], isWild: true})
		} else {
			parts = append(parts, routePart{value: seg})
		}
	}
	return parts
}

// isDynamic 判断路由模式是否包含动态参数
func isDynamic(pattern string) bool {
	return strings.Contains(pattern, ":") || strings.Contains(pattern, "*")
}

// AddRoute 添加路由（自动区分静态/动态路由）
func (r *Router) AddRoute(method string, pattern string, handler HandlerFunc) {
	chain := make([]HandlerFunc, 0, len(r.middlewares)+1)
	chain = append(chain, r.middlewares...)
	chain = append(chain, handler)

	if isDynamic(pattern) {
		// 动态路由同时添加到 Radix Tree 和旧数组（向后兼容）
		r.radixTree.Add(method, pattern, chain)
		r.dynamicRoutes = append(r.dynamicRoutes, dynamicRoute{
			method:   method,
			pattern:  pattern,
			parts:    parseParts(pattern),
			handlers: chain,
		})
	} else {
		// 静态路由添加到 Radix Tree
		r.radixTree.Add(method, pattern, chain)
		if _, ok := r.staticRoutes[pattern]; !ok {
			r.staticRoutes[pattern] = make(map[string][]HandlerFunc)
		}
		r.staticRoutes[pattern][method] = chain
	}
}

// addRouteWithMiddlewares 内部方法：添加路由并附加额外中间件
func (r *Router) addRouteWithMiddlewares(method string, pattern string, handler HandlerFunc, groupMiddlewares []HandlerFunc) {
	chain := make([]HandlerFunc, 0, len(r.middlewares)+len(groupMiddlewares)+1)
	chain = append(chain, r.middlewares...)
	chain = append(chain, groupMiddlewares...)
	chain = append(chain, handler)

	if isDynamic(pattern) {
		r.radixTree.Add(method, pattern, chain)
		r.dynamicRoutes = append(r.dynamicRoutes, dynamicRoute{
			method:   method,
			pattern:  pattern,
			parts:    parseParts(pattern),
			handlers: chain,
		})
	} else {
		r.radixTree.Add(method, pattern, chain)
		if _, ok := r.staticRoutes[pattern]; !ok {
			r.staticRoutes[pattern] = make(map[string][]HandlerFunc)
		}
		r.staticRoutes[pattern][method] = chain
	}
}

// Handle 注册静态资源处理
func (r *Router) Handle(pattern string, handler http.Handler) {
	r.fileHandlers[pattern] = handler
}

// ServeHTTP 实现 http.Handler 接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path

	// 1. 优先匹配静态文件
	for prefix, handler := range r.fileHandlers {
		if strings.HasPrefix(path, prefix) {
			handler.ServeHTTP(w, req)
			return
		}
	}

	// 2. 使用 Radix Tree 快速匹配（性能优化）
	result := r.radixTree.Get(req.Method, path)
	if result != nil {
		c := r.pool.Get().(*Context)
		c.Reset(w, req)
		c.handlers = result.Handlers.([]HandlerFunc)
		if r.viewEngine != nil {
			c.ViewEngine = r.viewEngine
		}
		for k, v := range result.Params {
			c.Params[k] = v
		}
		c.Next()
		r.pool.Put(c)
		return
	}

	http.NotFound(w, req)
}

// ==================== 路由注册快捷方法 ====================

// Use 注册全局中间件
func (r *Router) Use(middleware ...HandlerFunc) {
	r.middlewares = append(r.middlewares, middleware...)
}

func (r *Router) GET(pattern string, handler HandlerFunc)    { r.AddRoute("GET", pattern, handler) }
func (r *Router) POST(pattern string, handler HandlerFunc)   { r.AddRoute("POST", pattern, handler) }
func (r *Router) PUT(pattern string, handler HandlerFunc)    { r.AddRoute("PUT", pattern, handler) }
func (r *Router) DELETE(pattern string, handler HandlerFunc) { r.AddRoute("DELETE", pattern, handler) }
func (r *Router) PATCH(pattern string, handler HandlerFunc)  { r.AddRoute("PATCH", pattern, handler) }
func (r *Router) OPTIONS(pattern string, handler HandlerFunc) {
	r.AddRoute("OPTIONS", pattern, handler)
}
func (r *Router) HEAD(pattern string, handler HandlerFunc) { r.AddRoute("HEAD", pattern, handler) }

// Any 注册所有 HTTP 方法
func (r *Router) Any(pattern string, handler HandlerFunc) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"} {
		r.AddRoute(method, pattern, handler)
	}
}

// ==================== 路由分组 ====================

// Group 创建路由分组
func (r *Router) Group(prefix string, middlewares ...HandlerFunc) *RouteGroup {
	return &RouteGroup{
		prefix:             prefix,
		router:             r,
		middlewares:        middlewares,
		excludeMiddlewares: make([]string, 0),
	}
}

// WithoutMiddleware 排除某个路由的中间件（类似 TP 8.1.0 的 withoutMiddleware）
func (g *RouteGroup) WithoutMiddleware(names ...string) *RouteGroup {
	g.excludeMiddlewares = append(g.excludeMiddlewares, names...)
	return g
}

// Append 追加中间件（类似 TP 8.1.2 的 append 方法）
func (g *RouteGroup) Append(middlewares ...HandlerFunc) *RouteGroup {
	g.middlewares = append(g.middlewares, middlewares...)
	return g
}

// Rule 创建路由规则构建器（支持变量验证、枚举验证等）
func (g *RouteGroup) Rule(pattern string) *RouteRuleBuilder {
	builder := (&RouteRuleBuilder{
		rule: &RouteRule{
			pattern:     g.fullPath(pattern),
			validators:  make(map[string]RuleFunc),
			enumRules:   make(map[string][]string),
			typeRules:   make(map[string]string),
			middlewares: make([]HandlerFunc, 0),
		},
	}).Middleware(g.middlewares...)

	// 设置 router 引用以便注册路由
	builder.router = g.router
	builder.group = g

	return builder
}

func (g *RouteGroup) fullPath(path string) string {
	return g.prefix + path
}

// RouteGroup 路由注册方法
func (g *RouteGroup) GET(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("GET", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) POST(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("POST", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) PUT(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("PUT", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) DELETE(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("DELETE", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) PATCH(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("PATCH", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) OPTIONS(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("OPTIONS", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) HEAD(pattern string, handler HandlerFunc) {
	g.router.addRouteWithMiddlewares("HEAD", g.fullPath(pattern), handler, g.middlewares)
}

func (g *RouteGroup) Any(pattern string, handler HandlerFunc) {
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD"} {
		g.router.addRouteWithMiddlewares(method, g.fullPath(pattern), handler, g.middlewares)
	}
}
