package radixtree

// Node Radix Tree 节点
type Node struct {
	path     string
	children []*Node
	handlers interface{} // 存储处理器
	priority uint32      // 优先级（用于优化）
	nType    nodeType
}

type nodeType uint8

const (
	static nodeType = iota
	root
	param
	catchAll
)

// NewNode 创建新节点
func NewNode(path string, nType nodeType) *Node {
	return &Node{
		path:     path,
		children: make([]*Node, 0, 2),
		nType:    nType,
	}
}

// insertChild 插入子节点
func (n *Node) insertChild(path string, handlers interface{}) {
	child := NewNode(path, param)
	child.handlers = handlers
	n.children = append(n.children, child)
	n.priority++
}

// insertStatic 插入静态节点
func (n *Node) insertStatic(path string, handlers interface{}) {
	child := NewNode(path, static)
	child.handlers = handlers
	n.children = append(n.children, child)
	n.priority++
}

// getChild 获取匹配的子节点
func (n *Node) getChild(path string) *Node {
	for _, child := range n.children {
		if child.nType == static && child.path == path {
			return child
		}
	}
	return nil
}

// getParamChild 获取参数子节点
func (n *Node) getParamChild() *Node {
	for _, child := range n.children {
		if child.nType == param {
			return child
		}
	}
	return nil
}

// getCatchAllChild 获取通配符子节点
func (n *Node) getCatchAllChild() *Node {
	for _, child := range n.children {
		if child.nType == catchAll {
			return child
		}
	}
	return nil
}
