package transport

import (
	"context"
)

type Kind string

func (k Kind) String() string { return string(k) }

// Defines a set of transport kind
const (
	KindGRPC Kind = "grpc"
	KindHTTP Kind = "http"
)

type (
	serverTransportKey struct{}
	clientTransportKey struct{}
)

type Header interface {
	Get(key string) string
	Set(Key, value string)
	Keys() []string
}

// Transporter transport context value interface
type Transporter interface {
	Kind() Kind
	//Endpoint server或者client的endpoint
	Endpoint() string

	FullMethod() string

	RequestHeader() Header

	ReplyHeader() Header
}

func NewServerContext(ctx context.Context, tr Transporter) context.Context {
	return context.WithValue(ctx, serverTransportKey{}, tr)
}

func FromServerContext(ctx context.Context) (tr Transporter, ok bool) {
	tr, ok = ctx.Value(serverTransportKey{}).(Transporter)
	return
}

func NewClientContext(ctx context.Context, tr Transporter) context.Context {
	return context.WithValue(ctx, clientTransportKey{}, tr)
}

func FromClientContext(ctx context.Context) (tr Transporter, ok bool) {
	tr, ok = ctx.Value(clientTransportKey{}).(Transporter)
	return
}
