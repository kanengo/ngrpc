package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/kanengo/ngrpc/middleware"
	"github.com/kanengo/ngrpc/registry"
	"github.com/kanengo/ngrpc/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	grpcmd "google.golang.org/grpc/metadata"
)

type ClientOption func(options *clientOptions)

func WithEndpoint(endpoint string) ClientOption {
	return func(options *clientOptions) {
		options.endpoint = endpoint
	}
}

func WithTlsConfig(tlsConfig *tls.Config) ClientOption {
	return func(options *clientOptions) {
		options.tlsConf = tlsConfig
	}
}

func WithTimeout(timeout time.Duration) ClientOption {
	return func(options *clientOptions) {
		options.timeout = timeout
	}
}

func WithMiddleware(ms ...middleware.Middleware) ClientOption {
	return func(options *clientOptions) {
		options.middleware = ms
	}
}

func WithDiscovery(discovery registry.Discovery) ClientOption {
	return func(options *clientOptions) {
		options.discovery = discovery
	}
}

func WithUnaryInterceptor(in ...grpc.UnaryClientInterceptor) ClientOption {
	return func(options *clientOptions) {
		options.ints = in
	}
}

func WithOptions(opts ...grpc.DialOption) ClientOption {
	return func(options *clientOptions) {
		options.grpcOpts = opts
	}
}

type clientOptions struct {
	endpoint     string
	tlsConf      *tls.Config
	timeout      time.Duration
	middleware   []middleware.Middleware
	ints         []grpc.UnaryClientInterceptor
	grpcOpts     []grpc.DialOption
	discovery    registry.Discovery
	balancerName string
}

func Dial(ctx context.Context, opts ...ClientOption) (*grpc.ClientConn, error) {
	return dial(ctx, false, opts...)
}

func DialInsecure(ctx context.Context, opts ...ClientOption) (*grpc.ClientConn, error) {
	return dial(ctx, true, opts...)
}

func dial(ctx context.Context, insecure bool, opts ...ClientOption) (*grpc.ClientConn, error) {
	options := clientOptions{
		timeout:      2 * time.Second,
		balancerName: balancerName,
	}
	for _, o := range opts {
		o(&options)
	}

	ints := []grpc.UnaryClientInterceptor{
		unaryClientInterceptor(options.middleware, options.timeout),
	}

	if len(options.ints) > 0 {
		ints = append(ints, options.ints...)
	}

	grpcOpts := []grpc.DialOption{
		grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"loadBalancingConfig": [{"%s":{}}]}`, options.balancerName)),
		grpc.WithChainUnaryInterceptor(ints...),
	}

	if options.discovery != nil {
		grpcOpts = append(grpcOpts, grpc.WithResolvers())
	}

	if insecure {
		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	}

	if options.tlsConf != nil {
		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(credentials.NewTLS(options.tlsConf)))
	}

	if len(options.grpcOpts) > 0 {
		grpcOpts = append(grpcOpts, options.grpcOpts...)
	}

	return grpc.DialContext(ctx, options.endpoint, grpcOpts...)
}

func unaryClientInterceptor(ms []middleware.Middleware, timeout time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = transport.NewClientContext(ctx, &Transport{
			endpoint:    cc.Target(),
			fullMethod:  method,
			reqHeader:   headerCarrier{},
			replyHeader: nil,
		})
		if timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		h := func(ctx context.Context, req any) (any, error) {
			if tr, ok := transport.FromClientContext(ctx); ok {
				header := tr.RequestHeader()
				keys := header.Keys()
				keyValues := make([]string, 0, len(keys)*2)
				for _, k := range keys {
					keyValues = append(keyValues, k, header.Get(k))
				}
				ctx = grpcmd.AppendToOutgoingContext(ctx, keyValues...)
			}
			return reply, invoker(ctx, method, req, reply, cc, opts...)
		}

		if len(ms) > 0 {
			h = middleware.Chain(ms...)(h)
		}

		_, err := h(ctx, req)

		return err
	}
}
