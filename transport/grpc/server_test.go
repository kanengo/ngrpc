package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kanengo/ngrpc/errors"
	pb "github.com/kanengo/ngrpc/internal/testdata/helloworld"
	"github.com/kanengo/ngrpc/middleware"
	"github.com/kanengo/ngrpc/transport"
	"google.golang.org/grpc"
)

type server struct {
	pb.UnimplementedGreeterServer
}

func (s *server) SayHelloStream(streamServer pb.Greeter_SayHelloStreamServer) error {
	var cnt uint
	for {
		in, err := streamServer.Recv()
		if err != nil {
			return err
		}
		if in.Name == "error" {
			return errors.BadRequest(fmt.Sprintf("invalid argument %s", in.Name))
		}
		if in.Name == "panic" {
			panic("server panic")
		}
		err = streamServer.Send(&pb.HelloReply{
			Message: fmt.Sprintf("hello %s", in.Name),
		})
		if err != nil {
			return err
		}
		cnt++
		if cnt > 1 {
			return nil
		}
	}
}

// SayHello implements pb.GreeterServer
func (s *server) SayHello(ctx context.Context, in *pb.HelloRequest) (*pb.HelloReply, error) {
	if in.Name == "error" {
		return nil, errors.BadRequest(fmt.Sprintf("invalid argument %s", in.Name))
	}
	if in.Name == "panic" {
		panic("server panic")
	}
	return &pb.HelloReply{Message: fmt.Sprintf("Hello %+v", in.Name)}, nil
}

type testKey struct{}

func TestServer(t *testing.T) {
	ctx := context.Background()
	ctx = context.WithValue(ctx, testKey{}, "test")
	srv := NewServer(
		Middleware(
			func(handler middleware.Handler) middleware.Handler {
				return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
					if tr, ok := transport.FromServerContext(ctx); ok {
						if tr.ReplyHeader() != nil {
							tr.ReplyHeader().Set("req_id", "3344")
						}
					}
					return handler(ctx, req)
				}
			}),
		UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			return handler(ctx, req)
		}),
		Options(grpc.InitialConnWindowSize(0)),
	)
	pb.RegisterGreeterServer(srv, &server{})

	if e, err := srv.Endpoint(); err != nil || e == nil || strings.HasSuffix(e.Host, ":0") {
		t.Fatal(e, err)
	}

	go func() {
		// start server
		if err := srv.Start(ctx); err != nil {
			panic(err)
		}
	}()
	time.Sleep(time.Second)
	//testClient(t, srv)
	_ = srv.Stop(ctx)
}
