package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"reflect"
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
	testClient(t, srv)
	_ = srv.Stop(ctx)
}

func testClient(t *testing.T, srv *Server) {
	u, err := srv.Endpoint()
	if err != nil {
		t.Fatal(err)
	}
	// new a gRPC client
	conn, err := DialInsecure(context.Background(),
		WithEndpoint(u.Host),
		WithOptions(grpc.WithBlock()),
		WithUnaryInterceptor(
			func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
				return invoker(ctx, method, req, reply, cc, opts...)
			}),
		WithMiddleware(func(handler middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
				if tr, ok := transport.FromClientContext(ctx); ok {
					header := tr.RequestHeader()
					header.Set("x-md-trace", "2233")
				}
				return handler(ctx, req)
			}
		}),
	)
	defer func() {
		_ = conn.Close()
	}()
	if err != nil {
		t.Fatal(err)
	}
	client := pb.NewGreeterClient(conn)
	reply, err := client.SayHello(context.Background(), &pb.HelloRequest{Name: "kratos"})
	t.Log(err)
	if err != nil {
		t.Errorf("failed to call: %v", err)
	}
	if !reflect.DeepEqual(reply.Message, "Hello kratos") {
		t.Errorf("expect %s, got %s", "Hello kratos", reply.Message)
	}

	streamCli, err := client.SayHelloStream(context.Background())
	if err != nil {
		t.Error(err)
		return
	}
	defer func() {
		_ = streamCli.CloseSend()
	}()
	err = streamCli.Send(&pb.HelloRequest{Name: "cc"})
	if err != nil {
		t.Error(err)
		return
	}
	reply, err = streamCli.Recv()
	if err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(reply.Message, "hello cc") {
		t.Errorf("expect %s, got %s", "hello cc", reply.Message)
	}
}

func TestTimeout(t *testing.T) {
	o := &Server{}
	v := time.Duration(123)
	Timeout(v)(o)
	if !reflect.DeepEqual(v, o.timeout) {
		t.Errorf("expect %s, got %s", v, o.timeout)
	}
}

func TestTLSConfig(t *testing.T) {
	o := &Server{}
	v := &tls.Config{}
	TLSConfig(v)(o)
	if !reflect.DeepEqual(v, o.tlsConf) {
		t.Errorf("expect %v, got %v", v, o.tlsConf)
	}
}

func TestUnaryInterceptor(t *testing.T) {
	o := &Server{}
	v := []grpc.UnaryServerInterceptor{
		func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			return nil, nil
		},
		func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			return nil, nil
		},
	}
	UnaryInterceptor(v...)(o)
	if !reflect.DeepEqual(v, o.unaryInts) {
		t.Errorf("expect %v, got %v", v, o.unaryInts)
	}
}

//func TestStreamInterceptor(t *testing.T) {
//	o := &Server{}
//	v := []grpc.StreamServerInterceptor{
//		func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
//			return nil
//		},
//		func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
//			return nil
//		},
//	}
//	StreamInterceptor(v...)(o)
//	if !reflect.DeepEqual(v, o.streamInts) {
//		t.Errorf("expect %v, got %v", v, o.streamInts)
//	}
//}

func TestOptions(t *testing.T) {
	o := &Server{}
	v := []grpc.ServerOption{
		grpc.EmptyServerOption{},
	}
	Options(v...)(o)
	if !reflect.DeepEqual(v, o.grpcOpts) {
		t.Errorf("expect %v, got %v", v, o.grpcOpts)
	}
}
