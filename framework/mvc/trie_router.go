package mvc

import (
	"strings"
	"sync"
)

// TrieNode Trie 树节点
type TrieNode struct {
	children   map[string]*TrieNode  // 子节点
	handlers   map[string][]HandlerFunc // 方法 -> 处理器链
	paramName  string                // 参数名（:id）
	isWildcard bool                  // 是否是通配符（*）
	isParam    bool                  // 是否是参数节点
	mu         sync.RWMutex          // 读写锁
}

// newTrieNode 创建新的 Trie 节点
func newTrieNode() *TrieNode {
	return &TrieNode{
		children: make(map[string]*TrieNode),
		handlers: make(map[string][]HandlerFunc),
	}
}

// TrieRouter Trie 树路由
type TrieRouter struct {
	root       *TrieNode
	middlewares []HandlerFunc
}

// NewTrieRouter 创建 Trie 树路由
func NewTrieRouter() *TrieRouter {
	return &TrieRouter{
		root:       newTrieNode(),
		middlewares: make([]HandlerFunc, 0),
	}
}

// AddRoute 添加路由到 Trie 树
func (t *TrieRouter) AddRoute(method, pattern string, handler HandlerFunc) {
	t.root.mu.Lock()
	defer t.root.mu.Unlock()

	// 构建中间件链
	chain := make([]HandlerFunc, 0, len(t.middlewares)+1)
	chain = append(chain, t.middlewares...)
	chain = append(chain, handler)

	// 解析路径段
	segments := strings.Split(strings.Trim(pattern, "/"), "/")
	current := t.root

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		// 处理参数 :id
		if strings.HasPrefix(segment, ":") {
			paramName := segment[1:]
			if current.children[":"] == nil {
				current.children[":"] = newTrieNode()
				current.children[":"].isParam = true
				current.children[":"].paramName = paramName
			}
			current = current.children[":"]
			continue
		}

		// 处理通配符 *
		if strings.HasPrefix(segment, "*") {
			wildcardName := segment[1:]
			if current.children["*"] == nil {
				current.children["*"] = newTrieNode()
				current.children["*"].isWildcard = true
				current.children["*"].paramName = wildcardName
			}
			current = current.children["*"]
			continue
		}

		// 普通路径段
		if current.children[segment] == nil {
			current.children[segment] = newTrieNode()
		}
		current = current.children[segment]
	}

	// 在叶子节点存储处理器
	current.handlers[method] = chain
}

// Use 注册全局中间件
func (t *TrieRouter) Use(middlewares ...HandlerFunc) {
	t.middlewares = append(t.middlewares, middlewares...)
}

// Match 匹配路由
// 返回：handlers 和参数 map
func (t *TrieRouter) Match(method, path string) ([]HandlerFunc, map[string]string) {
	t.root.mu.RLock()
	defer t.root.mu.RUnlock()

	segments := strings.Split(strings.Trim(path, "/"), "/")
	params := make(map[string]string)
	
	return t.matchRecursive(segments, 0, method, params, t.root)
}

// matchRecursive 递归匹配 Trie 树
func (t *TrieRouter) matchRecursive(segments []string, index int, method string, params map[string]string, node *TrieNode) ([]HandlerFunc, map[string]string) {
	// 到达叶子节点
	if index == len(segments) {
		if handlers, ok := node.handlers[method]; ok {
			return handlers, params
		}
		return nil, nil
	}

	segment := segments[index]

	// 1. 优先匹配普通路径段
	if child, ok := node.children[segment]; ok {
		if handlers, resultParams := t.matchRecursive(segments, index+1, method, params, child); handlers != nil {
			return handlers, resultParams
		}
	}

	// 2. 匹配参数节点 :id
	if child, ok := node.children[":"]; ok {
		newParams := make(map[string]string)
		for k, v := range params {
			newParams[k] = v
		}
		newParams[child.paramName] = segment
		
		if handlers, resultParams := t.matchRecursive(segments, index+1, method, newParams, child); handlers != nil {
			return handlers, resultParams
		}
	}

	// 3. 匹配通配符 *（匹配剩余所有段）
	if child, ok := node.children["*"]; ok {
		newParams := make(map[string]string)
		for k, v := range params {
			newParams[k] = v
		}
		// 通配符匹配剩余所有段
		remaining := strings.Join(segments[index:], "/")
		newParams[child.paramName] = remaining
		
		if handlers, ok := child.handlers[method]; ok {
			return handlers, newParams
		}
	}

	return nil, nil
}

// ========== 兼容性封装 ==========

// TrieRouterWrapper 兼容原有 Router 接口的包装器
type TrieRouterWrapper struct {
	trieRouter *TrieRouter
	staticRoutes map[string]map[string][]HandlerFunc // 静态路由（精确匹配）
}

// NewTrieRouterWrapper 创建 Trie 路由包装器
func NewTrieRouterWrapper() *TrieRouterWrapper {
	return &TrieRouterWrapper{
		trieRouter:   NewTrieRouter(),
		staticRoutes: make(map[string]map[string][]HandlerFunc),
	}
}

// AddRoute 添加路由（自动区分静态/动态）
func (w *TrieRouterWrapper) AddRoute(method, pattern string, handler HandlerFunc) {
	// 判断是否是静态路由
	if !strings.Contains(pattern, ":") && !strings.Contains(pattern, "*") {
		// 静态路由
		if _, ok := w.staticRoutes[pattern]; !ok {
			w.staticRoutes[pattern] = make(map[string][]HandlerFunc)
		}
		
		chain := make([]HandlerFunc, 0, len(w.trieRouter.middlewares)+1)
		chain = append(chain, w.trieRouter.middlewares...)
		chain = append(chain, handler)
		
		w.staticRoutes[pattern][method] = chain
	} else {
		// 动态路由使用 Trie 树
		w.trieRouter.AddRoute(method, pattern, handler)
	}
}

// Use 注册中间件
func (w *TrieRouterWrapper) Use(middlewares ...HandlerFunc) {
	w.trieRouter.Use(middlewares...)
}

// Match 匹配路由（优先静态，后 Trie）
func (w *TrieRouterWrapper) Match(method, path string) ([]HandlerFunc, map[string]string) {
	// 1. 优先匹配静态路由（O(1)）
	if methods, ok := w.staticRoutes[path]; ok {
		if handlers, ok := methods[method]; ok {
			return handlers, make(map[string]string)
		}
	}

	// 2. Trie 树匹配（O(log n)）
	return w.trieRouter.Match(method, path)
}
