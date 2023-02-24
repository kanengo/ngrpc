package selector

import (
	"context"
)

type Node interface {
	Scheme() string

	ServiceName() string

	Address() string

	Version() string

	Metadata() map[string]string
}

type Selector interface {
	ReBalancer

	Select(ctx context.Context, opts ...SelectOption) (selected Node, done DoneFunc, err error)
}

type ReBalancer interface {
	Apply(nodes []Node)
}

type Builder interface {
	Build() Selector
}

type DoneInfo struct {
	Err error

	BytesSent bool

	BytesReceived bool
}

type DoneFunc func(ctx context.Context, di DoneInfo)
