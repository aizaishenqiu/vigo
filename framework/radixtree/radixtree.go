package radixtree

import (
	"strings"
	"sync"
)

// RadixTree 高性能 Radix Tree 路由器
type RadixTree struct {
	root       *Node
	cache      map[string]map[string]interface{} // 静态路由缓存：path -> method -> handlers
	cacheMutex sync.RWMutex
}

// Result 路由匹配结果
type Result struct {
	Handlers interface{}
	Params   Params
}

// Params 路由参数
type Params map[string]string

// New 创建 Radix Tree 路由器
func New() *RadixTree {
	return &RadixTree{
		root:  NewNode("/", root),
		cache: make(map[string]map[string]interface{}),
	}
}

// Add 添加路由
func (t *RadixTree) Add(method, path string, handlers interface{}) {
	// 静态路由使用缓存加速
	if !strings.Contains(path, ":") && !strings.Contains(path, "*") {
		t.cacheMutex.Lock()
		if _, ok := t.cache[path]; !ok {
			t.cache[path] = make(map[string]interface{})
		}
		t.cache[path][method] = handlers
		t.cacheMutex.Unlock()
		return
	}

	// 动态路由使用 Radix Tree
	t.insert(method, path, handlers)
}

// insert 插入动态路由到 Radix Tree
func (t *RadixTree) insert(method, path string, handlers interface{}) {
	current := t.root
	segments := strings.Split(strings.Trim(path, "/"), "/")

	for i, seg := range segments {
		var child *Node

		if strings.HasPrefix(seg, ":") {
			// 参数节点
			child = current.getParamChild()
			if child == nil {
				child = NewNode(seg[1:], param)
				current.children = append(current.children, child)
			}
		} else if strings.HasPrefix(seg, "*") {
			// 通配符节点
			child = current.getCatchAllChild()
			if child == nil {
				child = NewNode(seg[1:], catchAll)
				current.children = append(current.children, child)
			}
			// 通配符是叶子节点
			child.handlers = handlers
			return
		} else {
			// 静态节点
			child = current.getChild(seg)
			if child == nil {
				child = NewNode(seg, static)
				current.children = append(current.children, child)
			}
		}

		current = child

		// 到达路径末尾，设置处理器
		if i == len(segments)-1 {
			// 使用方法作为 key 存储多个处理器
			if existing, ok := current.handlers.(map[string]interface{}); ok {
				existing[method] = handlers
			} else {
				current.handlers = map[string]interface{}{method: handlers}
			}
		}
	}
}

// Get 获取路由匹配结果
func (t *RadixTree) Get(method, path string) *Result {
	// 1. 先查缓存（静态路由）
	t.cacheMutex.RLock()
	if methods, ok := t.cache[path]; ok {
		if handlers, ok := methods[method]; ok {
			t.cacheMutex.RUnlock()
			return &Result{
				Handlers: handlers,
				Params:   nil,
			}
		}
	}
	t.cacheMutex.RUnlock()

	// 2. Radix Tree 查找（动态路由）
	handlers, params := t.search(t.root, method, strings.Split(strings.Trim(path, "/"), "/"), 0, make(Params))
	if handlers != nil {
		return &Result{
			Handlers: handlers,
			Params:   params,
		}
	}

	return nil
}

// search 递归搜索 Radix Tree
func (t *RadixTree) search(node *Node, method string, segments []string, index int, params Params) (interface{}, Params) {
	if node == nil || index > len(segments) {
		return nil, nil
	}

	// 到达路径末尾
	if index == len(segments) {
		if handlersMap, ok := node.handlers.(map[string]interface{}); ok {
			if handlers, ok := handlersMap[method]; ok {
				return handlers, params
			}
		}
		return nil, nil
	}

	seg := segments[index]

	// 1. 尝试匹配静态子节点
	for _, child := range node.children {
		if child.nType == static && child.path == seg {
			return t.search(child, method, segments, index+1, params)
		}
	}

	// 2. 尝试匹配参数子节点
	for _, child := range node.children {
		if child.nType == param {
			newParams := make(Params)
			for k, v := range params {
				newParams[k] = v
			}
			newParams[child.path] = seg
			if handlers, resultParams := t.search(child, method, segments, index+1, newParams); handlers != nil {
				return handlers, resultParams
			}
		}
	}

	// 3. 尝试匹配通配符子节点
	for _, child := range node.children {
		if child.nType == catchAll {
			newParams := make(Params)
			for k, v := range params {
				newParams[k] = v
			}
			newParams[child.path] = strings.Join(segments[index:], "/")
			if handlersMap, ok := child.handlers.(map[string]interface{}); ok {
				if handlers, ok := handlersMap[method]; ok {
					return handlers, newParams
				}
			}
			return nil, nil
		}
	}

	return nil, nil
}

// Size 返回路由数量（用于监控）
func (t *RadixTree) Size() int {
	count := len(t.cache)
	count += t.countNodes(t.root)
	return count
}

// countNodes 递归计算节点数
func (t *RadixTree) countNodes(node *Node) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range node.children {
		count += t.countNodes(child)
	}
	return count
}

// ClearCache 清空缓存（用于热重载）
func (t *RadixTree) ClearCache() {
	t.cacheMutex.Lock()
	defer t.cacheMutex.Unlock()
	t.cache = make(map[string]map[string]interface{})
}
