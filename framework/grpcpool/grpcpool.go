package grpcpool

import (
	"context"
	"errors"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientConn 客户端连接包装
type ClientConn struct {
	conn      *grpc.ClientConn
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
}

// Close 关闭连接
func (c *ClientConn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ConnPool gRPC 连接池
type ConnPool struct {
	mu           sync.Mutex
	target       string
	pool         []*ClientConn
	maxSize      int
	minSize      int
	idleTimeout  time.Duration
	maxLifetime  time.Duration
	closed       bool
	dialOptions  []grpc.DialOption
	createConnFn func() (*ClientConn, error)
}

// PoolOption 连接池选项
type PoolOption func(*ConnPool)

// WithMaxSize 设置最大连接数
func WithMaxSize(size int) PoolOption {
	return func(p *ConnPool) {
		p.maxSize = size
	}
}

// WithMinSize 设置最小连接数
func WithMinSize(size int) PoolOption {
	return func(p *ConnPool) {
		p.minSize = size
	}
}

// WithIdleTimeout 设置空闲超时
func WithIdleTimeout(timeout time.Duration) PoolOption {
	return func(p *ConnPool) {
		p.idleTimeout = timeout
	}
}

// WithMaxLifetime 设置最大生命周期
func WithMaxLifetime(lifetime time.Duration) PoolOption {
	return func(p *ConnPool) {
		p.maxLifetime = lifetime
	}
}

// WithDialOptions 设置 gRPC 拨号选项
func WithDialOptions(opts ...grpc.DialOption) PoolOption {
	return func(p *ConnPool) {
		p.dialOptions = append(p.dialOptions, opts...)
	}
}

// NewConnPool 创建 gRPC 连接池
func NewConnPool(target string, opts ...PoolOption) (*ConnPool, error) {
	p := &ConnPool{
		target:      target,
		pool:        make([]*ClientConn, 0),
		maxSize:     10,
		minSize:     2,
		idleTimeout: 5 * time.Minute,
		maxLifetime: 30 * time.Minute,
		dialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
	}

	for _, opt := range opts {
		opt(p)
	}

	// 预创建最小连接数
	for i := 0; i < p.minSize; i++ {
		conn, err := p.createConn()
		if err != nil {
			p.Close()
			return nil, err
		}
		p.pool = append(p.pool, conn)
	}

	// 启动连接池维护协程
	go p.maintenance()

	return p, nil
}

func (p *ConnPool) createConn() (*ClientConn, error) {
	if p.createConnFn != nil {
		return p.createConnFn()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, p.target, p.dialOptions...)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	return &ClientConn{
		conn:      conn,
		createdAt: now,
		lastUsed:  now,
		inUse:     false,
	}, nil
}

// Get 获取连接
func (p *ConnPool) Get(ctx context.Context) (*ClientConn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, errors.New("pool is closed")
	}

	// 查找可用连接
	for _, conn := range p.pool {
		if !conn.inUse {
			// 检查连接是否过期
			if p.isExpired(conn) {
				p.closeConn(conn)
				continue
			}

			conn.inUse = true
			conn.lastUsed = time.Now()
			return conn, nil
		}
	}

	// 如果没有可用连接，尝试创建新连接
	if len(p.pool) < p.maxSize {
		conn, err := p.createConn()
		if err != nil {
			return nil, err
		}
		conn.inUse = true
		p.pool = append(p.pool, conn)
		return conn, nil
	}

	// 等待可用连接
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, errors.New("timeout waiting for connection")
		case <-ticker.C:
			p.mu.Lock()
			for _, conn := range p.pool {
				if !conn.inUse && !p.isExpired(conn) {
					conn.inUse = true
					conn.lastUsed = time.Now()
					p.mu.Unlock()
					return conn, nil
				}
			}
			p.mu.Unlock()
		}
	}
}

// Put 归还连接
func (p *ConnPool) Put(conn *ClientConn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		p.closeConn(conn)
		return
	}

	conn.inUse = false
	conn.lastUsed = time.Now()
}

// Close 关闭连接池
func (p *ConnPool) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.closed = true

	for _, conn := range p.pool {
		p.closeConn(conn)
	}

	p.pool = nil
	return nil
}

func (p *ConnPool) closeConn(conn *ClientConn) {
	if conn != nil {
		conn.Close()
	}
}

func (p *ConnPool) isExpired(conn *ClientConn) bool {
	now := time.Now()

	// 检查生命周期
	if p.maxLifetime > 0 && now.Sub(conn.createdAt) > p.maxLifetime {
		return true
	}

	// 检查空闲超时
	if p.idleTimeout > 0 && now.Sub(conn.lastUsed) > p.idleTimeout {
		return true
	}

	return false
}

// maintenance 定期维护连接池
func (p *ConnPool) maintenance() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		if p.closed {
			p.mu.Unlock()
			break
		}

		// 清理过期连接
		newPool := make([]*ClientConn, 0)
		for _, conn := range p.pool {
			if conn.inUse || !p.isExpired(conn) {
				newPool = append(newPool, conn)
			} else {
				p.closeConn(conn)
			}
		}

		// 补充最小连接数
		for len(newPool) < p.minSize {
			conn, err := p.createConn()
			if err != nil {
				break
			}
			newPool = append(newPool, conn)
		}

		p.pool = newPool
		p.mu.Unlock()
	}
}

// Stats 连接池统计
type Stats struct {
	TotalConnections   int
	AvailableConnections int
	InUseConnections    int
}

// GetStats 获取连接池统计
func (p *ConnPool) GetStats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	inUse := 0
	for _, conn := range p.pool {
		if conn.inUse {
			inUse++
		}
	}

	return Stats{
		TotalConnections:     len(p.pool),
		AvailableConnections: len(p.pool) - inUse,
		InUseConnections:     inUse,
	}
}
