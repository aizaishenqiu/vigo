package trace

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

// Config 链路追踪配置
type Config struct {
	ServiceName    string  // 服务名称
	Endpoint       string  // Jaeger/Zipkin 端点
	SampleRate     float64 // 采样率 (0.0-1.0)
	MaxQueueSize   int     // 最大队列大小
	BatchTimeout   int     // 批量超时（毫秒）
	MaxExportBatch int     // 最大批量导出数
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "vigo-service",
		Endpoint:       "http://localhost:14268/api/traces",
		SampleRate:     1.0,
		MaxQueueSize:   51200,
		BatchTimeout:   5000,
		MaxExportBatch: 512,
	}
}

// Tracer 全局 Tracer
var Tracer *sdktrace.TracerProvider

// Init 初始化链路追踪
func Init(cfg *Config) (*sdktrace.TracerProvider, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 创建 Jaeger 导出器
	exporter, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(cfg.Endpoint),
		),
	)
	if err != nil {
		return nil, err
	}

	// 创建资源
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// 创建 Tracer Provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(
			exporter,
			sdktrace.WithMaxQueueSize(cfg.MaxQueueSize),
			sdktrace.WithBatchTimeout(time.Duration(cfg.BatchTimeout)*time.Millisecond),
			sdktrace.WithMaxExportBatchSize(cfg.MaxExportBatch),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(cfg.SampleRate),
		)),
	)

	// 设置全局 Tracer Provider
	otel.SetTracerProvider(tp)

	// 设置全局 Propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	Tracer = tp
	return tp, nil
}

// Shutdown 关闭链路追踪
func Shutdown() error {
	if Tracer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return Tracer.Shutdown(ctx)
	}
	return nil
}

// GetTracer 获取 Tracer
func GetTracer(name string) *CustomTracer {
	return &CustomTracer{
		name: name,
	}
}

// CustomTracer 自定义 Tracer
type CustomTracer struct {
	name string
}

// Start 开始追踪
func (t *CustomTracer) Start(ctx context.Context, spanName string, opts ...Option) (context.Context, *Span) {
	// 实现简化版本
	return ctx, &Span{
		name: spanName,
	}
}

// Span 自定义 Span
type Span struct {
	name string
}

// SetAttribute 设置属性
func (s *Span) SetAttribute(key string, value interface{}) {
	// 实现简化版本
}

// End 结束 Span
func (s *Span) End() {
	// 实现简化版本
}

// Option Span 选项
type Option func(*spanConfig)

type spanConfig struct {
	Attributes map[string]interface{}
}

// WithAttribute 设置属性
func WithAttribute(key string, value interface{}) Option {
	return func(cfg *spanConfig) {
		if cfg.Attributes == nil {
			cfg.Attributes = make(map[string]interface{})
		}
		cfg.Attributes[key] = value
	}
}
