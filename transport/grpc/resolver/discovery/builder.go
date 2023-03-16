package discovery

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/kanengo/goutil/pkg/log"
	"go.uber.org/zap"

	"github.com/kanengo/ngrpc/registry"
	"google.golang.org/grpc/resolver"
)

var _ resolver.Builder = (*builder)(nil)

const (
	name = "discovery"
)

type Option func(o *builder)

func WithTimeout(timeout time.Duration) Option {
	return func(o *builder) {
		o.timeout = timeout
	}
}

func WithInsecure(insecure bool) Option {
	return func(o *builder) {
		o.insecure = insecure
	}
}

type builder struct {
	timeout   time.Duration
	discovery registry.Discovery
	insecure  bool
}

func NewBuilder(d registry.Discovery, opts ...Option) resolver.Builder {
	b := &builder{
		timeout:   time.Second * 10,
		discovery: d,
		insecure:  false,
	}

	for _, o := range opts {
		o(b)
	}

	return b
}

func (b *builder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	watchRes := &struct {
		w   registry.Watcher
		err error
	}{}
	done := make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		serviceName := strings.TrimPrefix(target.URL.Path, "/")
		w, err := b.discovery.Watch(ctx, serviceName)
		watchRes.w = w
		watchRes.err = err
		close(done)
	}()

	var err error
	select {
	case <-done:
		err = watchRes.err
	case <-time.After(b.timeout):
		err = errors.New("discovery create watcher timeout")
	}
	if err != nil {
		cancel()
		log.Error("resolver discovery watch failed", zap.Error(err))
		return nil, err
	}

	r := &discoveryResolver{
		w:        watchRes.w,
		cc:       cc,
		ctx:      ctx,
		cancel:   cancel,
		insecure: b.insecure,
	}

	go r.watch()

	return r, nil
}

func (b *builder) Scheme() string {
	return name
}
