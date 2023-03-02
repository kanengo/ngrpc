package grpc

import (
	"github.com/kanengo/ngrpc/transport"
	"google.golang.org/grpc/metadata"
)

type Transport struct {
	endpoint    string
	fullMethod  string
	reqHeader   headerCarrier
	replyHeader headerCarrier
}

func (t *Transport) Kind() transport.Kind {
	return transport.KindGRPC
}

func (t *Transport) Endpoint() string {
	return t.endpoint
}

func (t *Transport) FullMethod() string {
	return t.fullMethod
}

func (t *Transport) RequestHeader() transport.Header {
	return t.reqHeader
}

func (t *Transport) ReplyHeader() transport.Header {
	return t.replyHeader
}

type headerCarrier metadata.MD

func (h headerCarrier) Get(key string) string {
	val := metadata.MD(h).Get(key)
	if len(val) > 0 {
		return val[0]
	}
	return ""
}

func (h headerCarrier) Set(Key, value string) {
	metadata.MD(h).Set(Key, value)
}

func (h headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}
