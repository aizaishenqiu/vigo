package event

import (
	"log"
	"sync"
)

// Listener 事件监听器（带错误处理）
type Listener func(data interface{}) error

// Dispatcher 事件分发器
type Dispatcher struct {
	listeners map[string][]listenerEntry
	mu        sync.RWMutex
}

type listenerEntry struct {
	id       int
	listener Listener
	priority int // 数字越大优先级越高
}

var (
	DefaultDispatcher = &Dispatcher{
		listeners: make(map[string][]listenerEntry),
	}
	nextID int
	idMu   sync.Mutex
)

func getNextID() int {
	idMu.Lock()
	defer idMu.Unlock()
	nextID++
	return nextID
}

// Listen 注册监听（返回 ID 用于取消订阅）
func Listen(event string, listener Listener, priority ...int) int {
	p := 0
	if len(priority) > 0 {
		p = priority[0]
	}
	id := getNextID()

	DefaultDispatcher.mu.Lock()
	defer DefaultDispatcher.mu.Unlock()

	entry := listenerEntry{id: id, listener: listener, priority: p}
	entries := append(DefaultDispatcher.listeners[event], entry)

	// 按优先级降序排列
	for i := len(entries) - 1; i > 0; i-- {
		if entries[i].priority > entries[i-1].priority {
			entries[i], entries[i-1] = entries[i-1], entries[i]
		}
	}
	DefaultDispatcher.listeners[event] = entries
	return id
}

// Remove 取消监听
func Remove(event string, id int) {
	DefaultDispatcher.mu.Lock()
	defer DefaultDispatcher.mu.Unlock()

	entries := DefaultDispatcher.listeners[event]
	for i, e := range entries {
		if e.id == id {
			DefaultDispatcher.listeners[event] = append(entries[:i], entries[i+1:]...)
			return
		}
	}
}

// Trigger 触发事件（异步执行监听器，带错误捕获）
func Trigger(event string, data interface{}) {
	DefaultDispatcher.mu.RLock()
	entries := make([]listenerEntry, len(DefaultDispatcher.listeners[event]))
	copy(entries, DefaultDispatcher.listeners[event])
	DefaultDispatcher.mu.RUnlock()

	for _, e := range entries {
		go func(l Listener) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Event] 监听器 panic: event=%s err=%v", event, r)
				}
			}()
			if err := l(data); err != nil {
				log.Printf("[Event] 监听器错误: event=%s err=%v", event, err)
			}
		}(e.listener)
	}
}

// TriggerSync 同步触发事件（按优先级顺序执行，遇错停止）
func TriggerSync(event string, data interface{}) error {
	DefaultDispatcher.mu.RLock()
	entries := make([]listenerEntry, len(DefaultDispatcher.listeners[event]))
	copy(entries, DefaultDispatcher.listeners[event])
	DefaultDispatcher.mu.RUnlock()

	for _, e := range entries {
		if err := e.listener(data); err != nil {
			return err
		}
	}
	return nil
}
