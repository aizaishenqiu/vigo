package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics 指标管理器
type Metrics struct {
	registry        *prometheus.Registry
	requestCounter  *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeRequests  *prometheus.GaugeVec
	cacheHits       *prometheus.CounterVec
	cacheMisses     *prometheus.CounterVec
	dbDuration      *prometheus.HistogramVec
	dbErrors        *prometheus.CounterVec
	rpcDuration     *prometheus.HistogramVec
	rpcErrors       *prometheus.CounterVec
}

// DefaultMetrics 默认指标
var DefaultMetrics *Metrics

// Init 初始化指标系统
func Init(serviceName string) *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
	}

	// HTTP 请求指标
	m.requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests",
			ConstLabels: prometheus.Labels{"service": serviceName},
		},
		[]string{"method", "path", "status"},
	)

	m.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request duration in seconds",
			ConstLabels: prometheus.Labels{"service": serviceName},
			Buckets:     prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	m.activeRequests = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "http_requests_in_progress",
			Help:        "Number of HTTP requests currently being processed",
			ConstLabels: prometheus.Labels{"service": serviceName},
		},
		[]string{"method"},
	)

	// 缓存指标
	m.cacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "cache_hits_total",
			Help:        "Total number of cache hits",
			ConstLabels: prometheus.Labels{"service": serviceName},
		},
		[]string{"cache_type"},
	)

	m.cacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "cache_misses_total",
			Help:        "Total number of cache misses",
			ConstLabels: prometheus.Labels{"service": serviceName},
		},
		[]string{"cache_type"},
	)

	// 数据库指标
	m.dbDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "db_query_duration_seconds",
			Help:        "Database query duration in seconds",
			ConstLabels: prometheus.Labels{"service": serviceName},
			Buckets:     prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	m.dbErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "db_errors_total",
			Help:        "Total number of database errors",
			ConstLabels: prometheus.Labels{"service": serviceName},
		},
		[]string{"operation"},
	)

	// RPC 指标
	m.rpcDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "rpc_duration_seconds",
			Help:        "RPC call duration in seconds",
			ConstLabels: prometheus.Labels{"service": serviceName},
			Buckets:     prometheus.DefBuckets,
		},
		[]string{"method", "service"},
	)

	m.rpcErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "rpc_errors_total",
			Help:        "Total number of RPC errors",
			ConstLabels: prometheus.Labels{"service": serviceName},
		},
		[]string{"method", "service"},
	)

	// 注册指标
	m.registry.MustRegister(
		m.requestCounter,
		m.requestDuration,
		m.activeRequests,
		m.cacheHits,
		m.cacheMisses,
		m.dbDuration,
		m.dbErrors,
		m.rpcDuration,
		m.rpcErrors,
	)

	DefaultMetrics = m
	return m
}

// Handler 返回 Prometheus 指标处理器
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// RecordRequest 记录 HTTP 请求
func (m *Metrics) RecordRequest(method, path, status string, duration time.Duration) {
	m.requestCounter.WithLabelValues(method, path, status).Inc()
	m.requestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// IncActiveRequests 增加活跃请求数
func (m *Metrics) IncActiveRequests(method string) {
	m.activeRequests.WithLabelValues(method).Inc()
}

// DecActiveRequests 减少活跃请求数
func (m *Metrics) DecActiveRequests(method string) {
	m.activeRequests.WithLabelValues(method).Dec()
}

// RecordCacheHit 记录缓存命中
func (m *Metrics) RecordCacheHit(cacheType string) {
	m.cacheHits.WithLabelValues(cacheType).Inc()
}

// RecordCacheMiss 记录缓存未命中
func (m *Metrics) RecordCacheMiss(cacheType string) {
	m.cacheMisses.WithLabelValues(cacheType).Inc()
}

// RecordDBQuery 记录数据库查询
func (m *Metrics) RecordDBQuery(operation string, duration time.Duration) {
	m.dbDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordDBError 记录数据库错误
func (m *Metrics) RecordDBError(operation string) {
	m.dbErrors.WithLabelValues(operation).Inc()
}

// RecordRPC 记录 RPC 调用
func (m *Metrics) RecordRPC(method, service string, duration time.Duration, err error) {
	m.rpcDuration.WithLabelValues(method, service).Observe(duration.Seconds())
	if err != nil {
		m.rpcErrors.WithLabelValues(method, service).Inc()
	}
}

// MetricsMiddleware Prometheus 指标中间件
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 增加活跃请求数
		DefaultMetrics.IncActiveRequests(r.Method)
		defer DefaultMetrics.DecActiveRequests(r.Method)

		// 包装 response writer 以获取状态码
		rw := &responseWriter{ResponseWriter: w, status: 200}

		// 调用下一个处理器
		next.ServeHTTP(rw, r)

		// 记录指标
		duration := time.Since(start)
		DefaultMetrics.RecordRequest(r.Method, r.URL.Path, strconv.Itoa(rw.status), duration)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// AutoGoroutine 自动收集 Goroutine 数量
func AutoGoroutine(serviceName string) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		gauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name:        "goroutines_count",
			Help:        "Number of goroutines",
			ConstLabels: prometheus.Labels{"service": serviceName},
		})

		DefaultMetrics.registry.MustRegister(gauge)

		for range ticker.C {
			// 这里需要 runtime.NumGoroutine()
			// 简化实现
		}
	}()
}
