package grpc

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"time"

	"github.com/kanengo/goutil/pkg/host"
	"github.com/kanengo/goutil/pkg/log"
	"github.com/kanengo/goutil/pkg/matcher"
	"github.com/kanengo/ngrpc/middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type ServerOption func(s *Server)

func Address(addr string) ServerOption {
	return func(s *Server) {
		s.address = addr
	}
}

func Timeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

func Middleware(m ...middleware.Middleware) ServerOption {
	return func(s *Server) {
		s.middleware.Use(m...)
	}
}

func TLSConfig(c *tls.Config) ServerOption {
	return func(s *Server) {
		s.tlsConf = c
	}
}

func Listener(lis net.Listener) ServerOption {
	return func(s *Server) {
		s.lis = lis
	}
}

func UnaryInterceptor(in ...grpc.UnaryServerInterceptor) ServerOption {
	return func(s *Server) {
		s.unaryInts = in
	}
}

func Options(opts ...grpc.ServerOption) ServerOption {
	return func(s *Server) {
		s.grpcOpts = opts
	}
}

type Server struct {
	*grpc.Server
	baseCtx    context.Context
	tlsConf    *tls.Config
	address    string
	endpoint   *url.URL
	timeout    time.Duration
	middleware matcher.Matcher[middleware.Middleware]
	unaryInts  []grpc.UnaryServerInterceptor
	grpcOpts   []grpc.ServerOption
	health     *health.Server
	lis        net.Listener
}

func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		baseCtx:    context.Background(),
		timeout:    3 * time.Second,
		middleware: matcher.New[middleware.Middleware](),
		unaryInts:  nil,
		grpcOpts:   nil,
		health:     health.NewServer(),
	}

	for _, o := range opts {
		o(srv)
	}

	unaryInts := []grpc.UnaryServerInterceptor{
		srv.unaryServerInterceptor(),
	}

	if len(srv.unaryInts) > 0 {
		unaryInts = append(unaryInts, srv.unaryInts...)
	}

	grpcOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInts...),
	}

	if srv.tlsConf != nil {
		grpcOpts = append(grpcOpts, grpc.Creds(credentials.NewTLS(srv.tlsConf)))
	}

	if len(srv.grpcOpts) > 0 {
		grpcOpts = append(grpcOpts, srv.grpcOpts...)
	}

	srv.Server = grpc.NewServer(grpcOpts...)

	if srv.health != nil {
		grpc_health_v1.RegisterHealthServer(srv.Server, srv.health)
	}

	reflection.Register(srv.Server)

	return srv
}

func (s *Server) AddMiddleware(selector string, ms ...middleware.Middleware) {
	s.middleware.Add(selector, ms...)
}

func (s *Server) Endpoint() (*url.URL, error) {
	if err := s.listenAndCheckEndpoint(); err != nil {
		return nil, err
	}

	return s.endpoint, nil
}

func (s *Server) Start(ctx context.Context) error {
	if err := s.listenAndCheckEndpoint(); err != nil {
		return err
	}
	s.baseCtx = ctx
	log.Info("[grpc] server start", zap.String("listener", s.lis.Addr().String()), zap.Any("endpoint", s.endpoint))
	if s.health != nil {
		s.health.Resume()
	}
	return s.Serve(s.lis)
}

func (s *Server) Stop(ctx context.Context) error {
	if s.health != nil {
		s.health.Shutdown()
	}
	s.GracefulStop()
	log.Info("[grpc] server stopping")
	return nil
}

func (s *Server) listenAndCheckEndpoint() error {
	if s.lis == nil {
		lis, err := net.Listen("tcp", s.address)
		if err != nil {
			return err
		}
		s.lis = lis
	}
	if s.endpoint == nil {
		addr, err := host.Extract(s.address, s.lis)
		if err != nil {
			return err
		}
		s.endpoint = &url.URL{
			Scheme: "grpc",
			Host:   addr,
		}
	}

	return nil
}
