package http2

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Server HTTP/2 服务器配置
type Server struct {
	Addr           string
	Handler        http.Handler
	TLSConfig      *tls.Config
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	IdleTimeout    time.Duration
	MaxHeaderBytes int
}

// NewServer 创建 HTTP/2 服务器
func NewServer(addr string, handler http.Handler) *Server {
	return &Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    120 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1MB
	}
}

// ListenAndServe 启动 HTTP/2 服务器（h2c，非加密）
func (s *Server) ListenAndServe() error {
	h2s := &http2.Server{
		MaxConcurrentStreams: 1000,
		IdleTimeout:          s.IdleTimeout,
	}

	// 使用 h2c 支持非加密 HTTP/2
	h2cHandler := h2c.NewHandler(s.Handler, h2s)

	server := &http.Server{
		Addr:           s.Addr,
		Handler:        h2cHandler,
		ReadTimeout:    s.ReadTimeout,
		WriteTimeout:   s.WriteTimeout,
		IdleTimeout:    s.IdleTimeout,
		MaxHeaderBytes: s.MaxHeaderBytes,
	}

	return server.ListenAndServe()
}

// ListenAndServeTLS 启动 HTTP/2 服务器（TLS 加密）
func (s *Server) ListenAndServeTLS(certFile, keyFile string) error {
	if s.TLSConfig == nil {
		s.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		}
	}

	server := &http.Server{
		Addr:           s.Addr,
		Handler:        s.Handler,
		TLSConfig:      s.TLSConfig,
		ReadTimeout:    s.ReadTimeout,
		WriteTimeout:   s.WriteTimeout,
		IdleTimeout:    s.IdleTimeout,
		MaxHeaderBytes: s.MaxHeaderBytes,
	}

	// HTTP/2 需要 TLS
	return server.ListenAndServeTLS(certFile, keyFile)
}

// Client HTTP/2 客户端
type Client struct {
	client          *http.Client
	Transport       *http2.Transport
	MaxConnsPerHost int
	Timeout         time.Duration
}

// NewClient 创建 HTTP/2 客户端
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		MaxConnsPerHost: 100,
		Timeout:         30 * time.Second,
	}

	for _, opt := range opts {
		opt(c)
	}

	c.Transport = &http2.Transport{
		AllowHTTP: true, // 允许非加密 HTTP/2
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			// 使用 h2c 时不需要 TLS
			return net.Dial(network, addr)
		},
	}

	c.client = &http.Client{
		Transport: c.Transport,
		Timeout:   c.Timeout,
	}

	return c
}

// ClientOption 客户端选项
type ClientOption func(*Client)

// WithMaxConnsPerHost 设置每个主机的最大连接数
func WithMaxConnsPerHost(n int) ClientOption {
	return func(c *Client) {
		c.MaxConnsPerHost = n
	}
}

// WithTimeout 设置超时时间
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// Get HTTP GET 请求
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.client.Do(req)
}

// Post HTTP POST 请求
func (c *Client) Post(ctx context.Context, url string, contentType string, body interface{}) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return c.client.Do(req)
}

// Do 发送请求
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// Close 关闭客户端
func (c *Client) Close() error {
	c.client.CloseIdleConnections()
	return nil
}

// PushSupport 检查是否支持 HTTP/2 Push
func PushSupport(w http.ResponseWriter) bool {
	pusher, ok := w.(http.Pusher)
	return ok && pusher != nil
}

// Push 推送资源到客户端
func Push(w http.ResponseWriter, target string, opts *http.PushOptions) error {
	pusher, ok := w.(http.Pusher)
	if !ok {
		return fmt.Errorf("HTTP/2 Push not supported")
	}
	return pusher.Push(target, opts)
}

// Middleware HTTP/2 中间件（添加 HTTP/2 特定头部）
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否是 HTTP/2 请求
		if r.ProtoMajor == 2 {
			w.Header().Set("X-Protocol", "HTTP/2")
		} else {
			w.Header().Set("X-Protocol", "HTTP/1.1")
		}
		next.ServeHTTP(w, r)
	})
}

// ConfigureServer 配置 HTTP/2 服务器参数
func ConfigureServer(server *http.Server, maxConcurrentStreams uint32, idleTimeout time.Duration) {
	if server.TLSConfig == nil {
		server.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"h2", "http/1.1"},
		}
	}

	http2.ConfigureServer(server, &http2.Server{
		MaxConcurrentStreams: maxConcurrentStreams,
		IdleTimeout:          idleTimeout,
	})
}
