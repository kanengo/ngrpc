package grpc

import (
	"context"

	"github.com/kanengo/ngrpc/middleware"
	"github.com/kanengo/ngrpc/transport"
	"google.golang.org/grpc"
	grpcmd "google.golang.org/grpc/metadata"
)

func (s *Server) unaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		var cancel context.CancelFunc
		md, _ := grpcmd.FromIncomingContext(ctx)
		replyHeader := grpcmd.MD{}
		tr := &Transport{
			endpoint:    "",
			fullMethod:  "",
			reqHeader:   headerCarrier(md),
			replyHeader: headerCarrier(replyHeader),
		}

		if s.endpoint != nil {
			tr.endpoint = s.endpoint.String()
		}
		ctx = transport.NewServerContext(ctx, tr)
		if s.timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, s.timeout)
			defer cancel()
		}
		h := func(ctx context.Context, req any) (any, error) {
			return handler(ctx, req)
		}
		if ms := s.middleware.Match(tr.FullMethod()); len(ms) > 0 {
			h = middleware.Chain(ms...)(h)
		}

		resp, err = h(ctx, req)

		if len(replyHeader) > 0 {
			_ = grpc.SetHeader(ctx, replyHeader)
		}

		return
	}
}
