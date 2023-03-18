package discovery

import (
	"context"
	"errors"
	"time"

	"github.com/kanengo/goutil/pkg/log"
	"github.com/kanengo/ngrpc/internal/endpoint"
	"go.uber.org/zap"
	"google.golang.org/grpc/attributes"

	"github.com/kanengo/ngrpc/registry"
	"google.golang.org/grpc/resolver"
)

var _ resolver.Resolver = (*discoveryResolver)(nil)

type discoveryResolver struct {
	w  registry.Watcher
	cc resolver.ClientConn

	ctx    context.Context
	cancel context.CancelFunc

	insecure bool
}

func (r *discoveryResolver) ResolveNow(options resolver.ResolveNowOptions) {

}

func (r *discoveryResolver) Close() {
	r.cancel()
	err := r.w.Stop()
	if err != nil {
		log.Error("[resolver] Failed to stop watcher", zap.Error(err))
	}
}

func (r *discoveryResolver) watch() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
		}
		ins, err := r.w.Next()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			log.Error("[resolver] Failed to watch discovery", zap.Error(err))
			time.Sleep(time.Second)
			continue
		}
		r.update(ins)
	}
}

func (r *discoveryResolver) update(ins []*registry.ServiceInstance) {
	addrs := make([]resolver.Address, 0, len(ins))
	endpoints := make(map[string]struct{})

	for _, in := range ins {
		ept, err := endpoint.ParseEndpoint(in.Endpoints, "grpc")
		if err != nil {
			log.Error("[resolver] Failed to parse discovery endpoint", zap.Error(err))
			continue
		}
		if ept == "" {
			continue
		}
		if _, ok := endpoints[ept]; ok {
			continue
		}
		endpoints[ept] = struct{}{}
		addr := resolver.Address{
			Addr:       ept,
			ServerName: in.Name,
			Attributes: parseAttributes(in.Metadata),
		}
		addr.Attributes.WithValue("__ServiceInstance__", in)
	}

	if len(addrs) == 0 {
		log.Warn("[resolver] Not any endpoint found", zap.Any("ins", ins))
		return
	}

	err := r.cc.UpdateState(resolver.State{
		Addresses: addrs,
	})
	if err != nil {
		log.Error("[resolver] failed to update state", zap.Error(err))
	}
}

func parseAttributes(metadata map[string]string) *attributes.Attributes {
	var attr *attributes.Attributes
	for k, v := range metadata {
		attr.WithValue(k, v)
	}
	return attr
}
