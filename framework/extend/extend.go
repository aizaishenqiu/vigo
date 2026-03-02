// Package extend 提供框架扩展机制
// 支持用户自定义插件和门面（Facade）系统
// 参考 Laravel 的服务提供者（Service Provider）和门面模式
package extend

import (
	"fmt"
	"sync"
)

// IExtend 扩展接口
// 所有自定义扩展都必须实现此接口
type IExtend interface {
	// Name 返回扩展名称
	Name() string
	
	// Init 初始化扩展
	Init() error
	
	// Run 运行扩展
	Run() error
	
	// Stop 停止扩展
	Stop() error
}

// IFacade 门面接口
// 所有门面插件都必须实现此接口
type IFacade interface {
	// Name 返回门面名称
	Name() string
	
	// Boot 引导门面（在应用启动时调用）
	Boot() error
	
	// GetAccessor 返回访问器（用于全局访问）
	GetAccessor() interface{}
}

// ExtendManager 扩展管理器
type ExtendManager struct {
	mu       sync.RWMutex
	extends  map[string]IExtend   // 已注册的扩展
	facades  map[string]IFacade   // 已注册的门面
	booted   map[string]bool      // 已引导的门面
}

// 全局扩展管理器实例
var globalExtendManager = &ExtendManager{
	extends: make(map[string]IExtend),
	facades: make(map[string]IFacade),
	booted:  make(map[string]bool),
}

// Register 注册扩展
// 参数:
//   - extend: 扩展实例
// 返回: 错误信息
func Register(extend IExtend) error {
	globalExtendManager.mu.Lock()
	defer globalExtendManager.mu.Unlock()
	
	name := extend.Name()
	if _, ok := globalExtendManager.extends[name]; ok {
		return fmt.Errorf("扩展 '%s' 已存在", name)
	}
	
	globalExtendManager.extends[name] = extend
	return nil
}

// RegisterFacade 注册门面
// 参数:
//   - facade: 门面实例
// 返回: 错误信息
func RegisterFacade(facade IFacade) error {
	globalExtendManager.mu.Lock()
	defer globalExtendManager.mu.Unlock()
	
	name := facade.Name()
	if _, ok := globalExtendManager.facades[name]; ok {
		return fmt.Errorf("门面 '%s' 已存在", name)
	}
	
	globalExtendManager.facades[name] = facade
	return nil
}

// Get 获取已注册的扩展
// 参数:
//   - name: 扩展名称
// 返回: 扩展实例和错误信息
func Get(name string) (IExtend, error) {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	extend, ok := globalExtendManager.extends[name]
	if !ok {
		return nil, fmt.Errorf("扩展 '%s' 不存在", name)
	}
	
	return extend, nil
}

// GetFacade 获取已注册的门面
// 参数:
//   - name: 门面名称
// 返回: 门面实例和错误信息
func GetFacade(name string) (IFacade, error) {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	facade, ok := globalExtendManager.facades[name]
	if !ok {
		return nil, fmt.Errorf("门面 '%s' 不存在", name)
	}
	
	return facade, nil
}

// Boot 引导所有门面
// 在应用启动时调用，会按注册顺序引导所有门面
func Boot() error {
	globalExtendManager.mu.Lock()
	defer globalExtendManager.mu.Unlock()
	
	for name, facade := range globalExtendManager.facades {
		if globalExtendManager.booted[name] {
			continue
		}
		
		if err := facade.Boot(); err != nil {
			return fmt.Errorf("引导门面 '%s' 失败：%v", name, err)
		}
		
		globalExtendManager.booted[name] = true
	}
	
	return nil
}

// Init 初始化所有扩展
// 在应用启动时调用，会按注册顺序初始化所有扩展
func Init() error {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	for name, extend := range globalExtendManager.extends {
		if err := extend.Init(); err != nil {
			return fmt.Errorf("初始化扩展 '%s' 失败：%v", name, err)
		}
	}
	
	return nil
}

// Run 运行所有扩展
// 在应用启动后调用，会按注册顺序运行所有扩展
func Run() error {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	for name, extend := range globalExtendManager.extends {
		if err := extend.Run(); err != nil {
			return fmt.Errorf("运行扩展 '%s' 失败：%v", name, err)
		}
	}
	
	return nil
}

// Stop 停止所有扩展
// 在应用关闭时调用，会按注册顺序停止所有扩展
func Stop() error {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	for name, extend := range globalExtendManager.extends {
		if err := extend.Stop(); err != nil {
			return fmt.Errorf("停止扩展 '%s' 失败：%v", name, err)
		}
	}
	
	return nil
}

// List 列出所有已注册的扩展
func List() []string {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	names := make([]string, 0, len(globalExtendManager.extends))
	for name := range globalExtendManager.extends {
		names = append(names, name)
	}
	
	return names
}

// ListFacades 列出所有已注册的门面
func ListFacades() []string {
	globalExtendManager.mu.RLock()
	defer globalExtendManager.mu.RUnlock()
	
	names := make([]string, 0, len(globalExtendManager.facades))
	for name := range globalExtendManager.facades {
		names = append(names, name)
	}
	
	return names
}

// BaseExtend 基础扩展实现
// 可以嵌入此结构体，只需实现必要的方法
type BaseExtend struct {
	name string
}

// NewBaseExtend 创建基础扩展
func NewBaseExtend(name string) *BaseExtend {
	return &BaseExtend{name: name}
}

// Name 返回扩展名称
func (b *BaseExtend) Name() string {
	return b.name
}

// Init 初始化扩展（默认空实现）
func (b *BaseExtend) Init() error {
	return nil
}

// Run 运行扩展（默认空实现）
func (b *BaseExtend) Run() error {
	return nil
}

// Stop 停止扩展（默认空实现）
func (b *BaseExtend) Stop() error {
	return nil
}

// BaseFacade 基础门面实现
// 可以嵌入此结构体，只需实现必要的方法
type BaseFacade struct {
	name     string
	accessor interface{}
}

// NewBaseFacade 创建基础门面
func NewBaseFacade(name string, accessor interface{}) *BaseFacade {
	return &BaseFacade{
		name:     name,
		accessor: accessor,
	}
}

// Name 返回门面名称
func (b *BaseFacade) Name() string {
	return b.name
}

// Boot 引导门面（默认空实现）
func (b *BaseFacade) Boot() error {
	return nil
}

// GetAccessor 返回访问器
func (b *BaseFacade) GetAccessor() interface{} {
	return b.accessor
}
