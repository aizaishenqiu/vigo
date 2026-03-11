package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
)

// ServerConfig gRPC 服务端配置
type ServerConfig struct {
	Port           int
	ServiceName    string
	EnableRecovery bool
	EnableLogger   bool
	MaxRecvMsgSize int // MB
	MaxSendMsgSize int // MB
}

// Server gRPC 服务端封装
type Server struct {
	*grpc.Server
	config      ServerConfig
	healthSrv   *health.Server
	listener    net.Listener
	mu          sync.Mutex
	isRunning   bool
	services    map[string]ServiceRegistrar // 已注册的服务
	interceptors []grpc.UnaryServerInterceptor
}

// ServiceRegistrar 服务注册接口
type ServiceRegistrar interface {
	RegisterService(s *grpc.Server)
}

// NewServer 创建 gRPC 服务端
func NewServer(cfg ServerConfig) *Server {
	if cfg.MaxRecvMsgSize <= 0 {
		cfg.MaxRecvMsgSize = 4
	}
	if cfg.MaxSendMsgSize <= 0 {
		cfg.MaxSendMsgSize = 4
	}

	s := &Server{
		config:   cfg,
		services: make(map[string]ServiceRegistrar),
	}

	// 构建拦截器链
	var interceptors []grpc.UnaryServerInterceptor
	if cfg.EnableRecovery {
		interceptors = append(interceptors, recoveryInterceptor())
	}
	if cfg.EnableLogger {
		interceptors = append(interceptors, loggerInterceptor())
	}
	s.interceptors = interceptors

	// 构建 gRPC 选项
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize * 1024 * 1024),
		grpc.MaxSendMsgSize(cfg.MaxSendMsgSize * 1024 * 1024),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Minute,
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	}

	if len(interceptors) > 0 {
		opts = append(opts, grpc.ChainUnaryInterceptor(interceptors...))
	}

	s.Server = grpc.NewServer(opts...)

	// 注册健康检查服务
	s.healthSrv = health.NewServer()
	healthpb.RegisterHealthServer(s.Server, s.healthSrv)
	s.healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	return s
}

// RegisterService 注册 gRPC 服务（便于后续管理）
func (s *Server) RegisterService(name string, registrar ServiceRegistrar) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.services[name] = registrar
	registrar.RegisterService(s.Server)
	// 设置健康状态
	s.healthSrv.SetServingStatus(name, healthpb.HealthCheckResponse_SERVING)
	log.Printf("[gRPC] 注册服务: %s", name)
}

// SetServiceStatus 设置服务健康状态
func (s *Server) SetServiceStatus(name string, serving bool) {
	if serving {
		s.healthSrv.SetServingStatus(name, healthpb.HealthCheckResponse_SERVING)
	} else {
		s.healthSrv.SetServingStatus(name, healthpb.HealthCheckResponse_NOT_SERVING)
	}
}

// Start 启动 gRPC 服务
func (s *Server) Start() error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return fmt.Errorf("gRPC 服务已在运行中")
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.config.Port))
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("gRPC 监听端口 %d 失败: %v", s.config.Port, err)
	}

	s.listener = lis
	s.isRunning = true
	s.mu.Unlock()

	log.Printf("[gRPC] 服务启动，监听端口 :%d (服务名: %s)", s.config.Port, s.config.ServiceName)
	return s.Serve(lis)
}

// GracefulStop 优雅关闭 gRPC 服务
func (s *Server) GracefulStop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	log.Printf("[gRPC] 正在优雅关闭...")

	// 设置所有服务为不可用
	s.healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	for name := range s.services {
		s.healthSrv.SetServingStatus(name, healthpb.HealthCheckResponse_NOT_SERVING)
	}

	s.Server.GracefulStop()
	s.isRunning = false
	log.Printf("[gRPC] 服务已关闭")
}

// IsRunning 检查服务是否在运行
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// recoveryInterceptor 异常恢复拦截器
func recoveryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[gRPC Recovery] panic: %v (method: %s)", r, info.FullMethod)
				err = fmt.Errorf("内部错误: %v", r)
			}
		}()
		return handler(ctx, req)
	}
}

// loggerInterceptor 日志拦截器
func loggerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		start := time.Now()
		resp, err = handler(ctx, req)
		duration := time.Since(start)

		if err != nil {
			log.Printf("[gRPC] %s | %v | ERROR: %v", info.FullMethod, duration, err)
		} else {
			log.Printf("[gRPC] %s | %v | OK", info.FullMethod, duration)
		}
		return resp, err
	}
}

// NewClient 创建 gRPC 客户端连接
func NewClient(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	defaultOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:                10 * time.Second,
			Timeout:             20 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	defaultOpts = append(defaultOpts, opts...)
	return grpc.NewClient(target, defaultOpts...)
}
