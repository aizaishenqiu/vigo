package container

import (
	"reflect"
	"sync"
)

// Container 基础容器
type Container struct {
	bindings  map[string]interface{} // 绑定关系
	instances map[string]interface{} // 单例实例
	mu        sync.RWMutex
}

var globalContainer *Container
var once sync.Once

// App 获取全局容器单例
func App() *Container {
	once.Do(func() {
		if globalContainer == nil {
			globalContainer = New()
		}
	})
	return globalContainer
}

func New() *Container {
	return &Container{
		bindings:  make(map[string]interface{}),
		instances: make(map[string]interface{}),
	}
}

// Bind 绑定一个接口到具体实现（或构造函数）
func (c *Container) Bind(key string, resolver interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[key] = resolver
}

// Singleton 绑定单例
func (c *Container) Singleton(key string, instance interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.instances[key] = instance
}

// Make 解析实例
func (c *Container) Make(key string) interface{} {
	c.mu.RLock()
	// 1. 检查单例
	if instance, ok := c.instances[key]; ok {
		c.mu.RUnlock()
		return instance
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double check
	if instance, ok := c.instances[key]; ok {
		return instance
	}

	// 2. 检查绑定
	if resolver, ok := c.bindings[key]; ok {
		return c.resolve(resolver)
	}

	return nil
}

func (c *Container) resolve(resolver interface{}) interface{} {
	t := reflect.TypeOf(resolver)
	if t.Kind() == reflect.Func {
		// 如果是函数，调用它
		// 这里简单处理，假设构造函数没有参数或参数也可以从容器解析
		// 简化版依赖注入实现
		vals := reflect.ValueOf(resolver).Call(nil)
		if len(vals) > 0 {
			return vals[0].Interface()
		}
	}
	return resolver
}
