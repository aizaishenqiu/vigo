package trace

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TraceMiddleware 链路追踪中间件
func TraceMiddleware(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.Tracer(serviceName)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 从请求中提取 trace context
			ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagationCarrier{r})

			// 创建 span
			spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			ctx, span := tracer.Start(ctx, spanName,
				trace.WithSpanKind(trace.SpanKindServer),
				trace.WithAttributes(
					attribute.String("http.method", r.Method),
					attribute.String("http.url", r.URL.String()),
					attribute.String("http.user_agent", r.UserAgent()),
				),
			)
			defer span.End()

			// 添加请求 ID
			requestID := span.SpanContext().TraceID().String()
			w.Header().Set("X-Request-ID", requestID)

			// 调用下一个处理器
			next.ServeHTTP(w, r.WithContext(ctx))

			// 记录响应状态
			if rw, ok := w.(*responseWriter); ok {
				span.SetAttributes(
					attribute.Int("http.status_code", rw.status),
				)
			}
		})
	}
}

// propagationCarrier 实现 OpenTelemetry 的 TextMapCarrier
type propagationCarrier struct {
	*http.Request
}

func (p propagationCarrier) Get(key string) string {
	return p.Header.Get(key)
}

func (p propagationCarrier) Set(key string, value string) {
	p.Header.Set(key, value)
}

func (p propagationCarrier) Keys() []string {
	keys := make([]string, 0)
	for k := range p.Header {
		keys = append(keys, k)
	}
	return keys
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// ClientTrace HTTP 客户端追踪
func ClientTrace(ctx context.Context, url string, client *http.Client) (*http.Response, error) {
	tracer := otel.Tracer("http-client")

	ctx, span := tracer.Start(ctx, "HTTP "+url,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.url", url),
		),
	)
	defer span.End()

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := client.Do(req)
	duration := time.Since(start)

	if err != nil {
		span.RecordError(err)
		span.SetAttributes(
			attribute.String("error", err.Error()),
		)
	} else {
		span.SetAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
			attribute.Int64("http.duration", duration.Milliseconds()),
		)
	}

	return resp, err
}

// DBTrace 数据库操作追踪
func DBTrace(ctx context.Context, operation string, query string, args ...interface{}) context.Context {
	tracer := otel.Tracer("database")

	_, span := tracer.Start(ctx, "DB "+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.operation", operation),
			attribute.String("db.statement", query),
		),
	)
	defer span.End()

	return ctx
}

// CacheTrace 缓存操作追踪
func CacheTrace(ctx context.Context, operation string, key string) context.Context {
	tracer := otel.Tracer("cache")

	_, span := tracer.Start(ctx, "Cache "+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.key", key),
		),
	)
	defer span.End()

	return ctx
}

// RPC Trace RPC 调用追踪
func RPCTrace(ctx context.Context, method string, service string) context.Context {
	tracer := otel.Tracer("rpc")

	_, span := tracer.Start(ctx, "RPC "+method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("rpc.method", method),
			attribute.String("rpc.service", service),
		),
	)
	defer span.End()

	return ctx
}
