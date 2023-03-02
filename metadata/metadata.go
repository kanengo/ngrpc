package metadata

import (
	"context"
)

type Metadata map[string]string

func New(mds ...map[string]string) Metadata {
	md := Metadata{}
	for _, m := range mds {
		for k, v := range m {
			md.Set(k, v)
		}
	}

	return md
}

func (m Metadata) Get(key string) string {
	return m[key]
}

func (m Metadata) Set(key string, value string) {
	if key == "" || value == "" {
		return
	}
	m[key] = value
}

func (m Metadata) Clone() Metadata {
	md := make(Metadata, len(m))
	for k, v := range m {
		md[k] = v
	}

	return md
}

type serverMetadataKey struct {
}

func NewServerContext(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, serverMetadataKey{}, md)
}

func FromServerContext(ctx context.Context) (Metadata, bool) {
	md, ok := ctx.Value(serverMetadataKey{}).(Metadata)
	return md, ok
}

type clientMetadataKey struct {
}

func NewClientContext(ctx context.Context, md Metadata) context.Context {
	return context.WithValue(ctx, clientMetadataKey{}, md)
}

func FromClientContext(ctx context.Context) (Metadata, bool) {
	md, ok := ctx.Value(clientMetadataKey{}).(Metadata)
	return md, ok
}
