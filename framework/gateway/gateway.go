package gateway

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
	"vigo/framework/circuit"
	"vigo/framework/middleware"
)

// Gateway 网关结构体
type Gateway struct {
	targetURL      *url.URL
	circuitBreaker *circuit.CircuitBreaker
	rateLimiter    *middleware.RateLimiter
	fallback       http.HandlerFunc
	retryCount     int
	retryDelay     time.Duration
	timeout        time.Duration
	mu             sync.RWMutex
}

// GatewayConfig 网关配置
type GatewayConfig struct {
	TargetURL    string        // 目标服务地址
	MaxFailures  int64         // 最大失败次数（熔断）
	ResetTimeout time.Duration // 熔断恢复时间
	RateLimit    int           // 每秒请求数限制（限流）
	RetryCount   int           // 重试次数
	RetryDelay   time.Duration // 重试延迟
	Timeout      time.Duration // 请求超时
	FallbackURL  string        // 降级服务地址
}

// DefaultGatewayConfig 默认配置
func DefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		MaxFailures:  5,                      // 5 次失败后熔断
		ResetTimeout: 30 * time.Second,       // 30 秒后尝试恢复
		RateLimit:    1000,                   // 1000 req/s
		RetryCount:   3,                      // 重试 3 次
		RetryDelay:   100 * time.Millisecond, // 重试间隔 100ms
		Timeout:      30 * time.Second,       // 30 秒超时
		FallbackURL:  "",                     // 无降级
	}
}

// NewGateway 创建网关
func NewGateway(config *GatewayConfig) (*Gateway, error) {
	targetURL, err := url.Parse(config.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("解析目标 URL 失败：%v", err)
	}

	gw := &Gateway{
		targetURL:      targetURL,
		circuitBreaker: circuit.NewCircuitBreaker("gateway", config.MaxFailures, config.ResetTimeout),
		rateLimiter:    middleware.NewRateLimiter(config.RateLimit),
		retryCount:     config.RetryCount,
		retryDelay:     config.RetryDelay,
		timeout:        config.Timeout,
	}

	// 设置降级处理
	if config.FallbackURL != "" {
		fallbackURL, err := url.Parse(config.FallbackURL)
		if err != nil {
			return nil, fmt.Errorf("解析降级 URL 失败：%v", err)
		}
		gw.fallback = createProxyHandler(fallbackURL)
	}

	return gw, nil
}

// ServeHTTP 实现 http.Handler 接口
func (gw *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 限流检查
	if !gw.rateLimiter.Allow() {
		http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
		return
	}

	// 2. 熔断检查
	err := gw.circuitBreaker.Call(func() error {
		return gw.handleRequest(w, r)
	})

	if err != nil {
		// 3. 降级处理
		if gw.fallback != nil {
			gw.fallback(w, r)
			return
		}

		// 无降级，返回错误
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}
}

// handleRequest 处理请求（带重试）
func (gw *Gateway) handleRequest(w http.ResponseWriter, r *http.Request) error {
	var lastErr error

	for i := 0; i <= gw.retryCount; i++ {
		// 创建代理
		proxy := createProxyHandler(gw.targetURL)

		// 创建响应记录器
		recorder := httptest.NewRecorder()

		// 执行请求
		proxy.ServeHTTP(recorder, r)

		// 检查响应状态
		if recorder.Code < 500 {
			// 成功或客户端错误，复制响应
			copyResponse(w, recorder)
			return nil
		}

		lastErr = fmt.Errorf("服务器错误：%d", recorder.Code)

		// 重试延迟
		if i < gw.retryCount {
			time.Sleep(gw.retryDelay * time.Duration(i+1))
		}
	}

	return lastErr
}

// createProxyHandler 创建代理处理器
func createProxyHandler(targetURL *url.URL) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		proxy := httputil.NewSingleHostReverseProxy(targetURL)

		// 修改请求头
		r.URL.Host = targetURL.Host
		r.URL.Scheme = targetURL.Scheme
		r.Header.Set("X-Forwarded-Host", r.Header.Get("Host"))
		r.Host = targetURL.Host

		// 设置超时上下文
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		r = r.WithContext(ctx)

		proxy.ServeHTTP(w, r)
	}
}

// copyResponse 复制响应
func copyResponse(w http.ResponseWriter, recorder *httptest.ResponseRecorder) {
	// 复制头
	for key, values := range recorder.Header() {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// 复制状态码
	w.WriteHeader(recorder.Code)

	// 复制 body
	w.Write(recorder.Body.Bytes())
}

// GatewayMiddleware 网关中间件（用于路由）
func GatewayMiddleware(config *GatewayConfig) http.HandlerFunc {
	gw, err := NewGateway(config)
	if err != nil {
		panic(fmt.Sprintf("创建网关失败：%v", err))
	}

	return func(w http.ResponseWriter, r *http.Request) {
		gw.ServeHTTP(w, r)
	}
}

// AutoLoadGateway 自动加载网关配置（单体/微服务通用）
func AutoLoadGateway() {
	// 从配置文件加载网关配置
	// 如果配置了网关，自动启用
	// 注意：此函数需要配合配置文件使用
	// 目前网关配置需要通过代码手动创建
}
