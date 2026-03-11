package pool

import (
	"sync"
)

// ContextPool Context 对象池
type ContextPool struct {
	pool sync.Pool
}

// NewContextPool 创建 Context 对象池
func NewContextPool(newFunc func() interface{}) *ContextPool {
	return &ContextPool{
		pool: sync.Pool{
			New: newFunc,
		},
	}
}

// Get 从池中获取对象
func (p *ContextPool) Get() interface{} {
	return p.pool.Get()
}

// Put 将对象放回池中
func (p *ContextPool) Put(x interface{}) {
	p.pool.Put(x)
}

// BufferPool 字节缓冲区对象池
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool 创建 Buffer 对象池
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 0, 1024) // 初始容量 1KB
			},
		},
	}
}

// Get 获取缓冲区
func (p *BufferPool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put 归还缓冲区
func (p *BufferPool) Put(buf []byte) {
	// 重置但保留容量
	buf = buf[:0]
	p.pool.Put(buf)
}

// ByteSlicePool 可配置大小的字节切片池
type ByteSlicePool struct {
	pool sync.Pool
	size int
}

// NewByteSlicePool 创建可配置大小的字节切片池
func NewByteSlicePool(size int) *ByteSlicePool {
	return &ByteSlicePool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, size)
			},
		},
		size: size,
	}
}

// Get 获取字节切片
func (p *ByteSlicePool) Get() []byte {
	return p.pool.Get().([]byte)
}

// Put 归还字节切片
func (p *ByteSlicePool) Put(buf []byte) {
	if cap(buf) >= p.size {
		buf = buf[:p.size]
		p.pool.Put(buf)
	}
}

// ResponseWriterPool ResponseWriter 对象池
type ResponseWriterPool struct {
	pool sync.Pool
}

// NewResponseWriterPool 创建 ResponseWriter 对象池
func NewResponseWriterPool(newFunc func() interface{}) *ResponseWriterPool {
	return &ResponseWriterPool{
		pool: sync.Pool{
			New: newFunc,
		},
	}
}

// Get 获取 ResponseWriter
func (p *ResponseWriterPool) Get() interface{} {
	return p.pool.Get()
}

// Put 归还 ResponseWriter
func (p *ResponseWriterPool) Put(x interface{}) {
	// 重置状态
	if resetter, ok := x.(interface{ Reset() }); ok {
		resetter.Reset()
	}
	p.pool.Put(x)
}

// GenericPool 通用对象池
type GenericPool[T any] struct {
	pool sync.Pool
}

// NewGenericPool 创建通用对象池
func NewGenericPool[T any](newFunc func() T) *GenericPool[T] {
	return &GenericPool[T]{
		pool: sync.Pool{
			New: func() interface{} {
				return newFunc()
			},
		},
	}
}

// Get 获取对象
func (p *GenericPool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put 归还对象
func (p *GenericPool[T]) Put(x T) {
	p.pool.Put(x)
}

// Stats 对象池统计
type PoolStats struct {
	GetCount uint64
	PutCount uint64
}

// TrackedPool 带统计的对象池
type TrackedPool struct {
	pool     sync.Pool
	getCount uint64
	putCount uint64
	mu       sync.Mutex
}

// NewTrackedPool 创建带统计的对象池
func NewTrackedPool(newFunc func() interface{}) *TrackedPool {
	return &TrackedPool{
		pool: sync.Pool{
			New: newFunc,
		},
	}
}

// Get 获取对象并统计
func (p *TrackedPool) Get() interface{} {
	p.mu.Lock()
	p.getCount++
	p.mu.Unlock()
	return p.pool.Get()
}

// Put 归还对象并统计
func (p *TrackedPool) Put(x interface{}) {
	p.mu.Lock()
	p.putCount++
	p.mu.Unlock()
	p.pool.Put(x)
}

// Stats 获取统计信息
func (p *TrackedPool) Stats() PoolStats {
	p.mu.Lock()
	defer p.mu.Unlock()
	return PoolStats{
		GetCount: p.getCount,
		PutCount: p.putCount,
	}
}
