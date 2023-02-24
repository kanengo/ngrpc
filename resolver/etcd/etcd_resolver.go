package etcd

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"sync"
	"time"

	resolver2 "github.com/kanengo/ngrpc/resolver"

	"github.com/kanengo/goutil/pkg/utils"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/resolver"
)

var logger = grpclog.Component("etcd")

type etcdBuilder struct {
	cli *clientv3.Client
}

func NewBuilder(cli *clientv3.Client) resolver.Builder {
	return &etcdBuilder{
		cli: cli,
	}
}

func (b *etcdBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	svrPath := path.Join(target.URL.Host, target.URL.Path)
	ctx, cancel := context.WithCancel(context.Background())
	r := &etcdResolver{
		cli:     b.cli,
		cc:      cc,
		svrPath: svrPath,
		rn:      make(chan struct{}, 1),
		ctx:     ctx,
		cancel:  cancel,
	}

	r.wg.Add(2)
	go r.watcher()
	go r.loopFetch()

	return r, nil
}

func (b *etcdBuilder) Scheme() string {
	return "etcd"
}

type etcdResolver struct {
	cli     *clientv3.Client
	svrPath string
	cc      resolver.ClientConn
	rn      chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func (e *etcdResolver) ResolveNow(options resolver.ResolveNowOptions) {
	select {
	case e.rn <- struct{}{}:
	default:

	}
}

func (e *etcdResolver) Close() {
	e.cancel()
	e.wg.Wait()
}

func (e *etcdResolver) fetch() (*resolver.State, error) {
	ctx, cancel := context.WithTimeout(e.ctx, time.Second*3)
	getResponse, err := e.cli.Get(ctx, e.svrPath, clientv3.WithPrefix())
	cancel()
	if err != nil {
		logger.Error("fetchAll failed:", err)
		return nil, err
	}

	resolverAddress := make([]resolver.Address, 0, len(getResponse.Kvs))
	for _, kv := range getResponse.Kvs {
		address := utils.SliceByteToStringUnsafe(kv.Key)
		var endpoint resolver2.Endpoint
		err := json.Unmarshal(kv.Value, &endpoint)
		if err != nil {
			logger.Error("Unmarshal value failed:", err)
			continue
		}
		if address != endpoint.Address {
			logger.Warningf("kv address not match, key:[%s] value:[%s]", address, endpoint.Address)
			continue
		}
		ra := resolver.Address{
			Addr:               endpoint.Address,
			BalancerAttributes: nil,
			Attributes:         nil,
		}
		resolverAddress = append(resolverAddress, ra)
	}

	state := &resolver.State{
		Addresses:     resolverAddress,
		ServiceConfig: nil,
		Attributes:    nil,
	}
	return state, nil
}

func (e *etcdResolver) loopFetch() {
	defer e.wg.Done()
	for {
		select {
		case <-e.rn:
			logger.Info("fetch remote")
			backoffIndex := 0
			var err error
			var timer *time.Timer
			var state *resolver.State
			for {
				state, err = e.fetch()
				if err != nil {
					switch err {
					case context.Canceled:
						if timer != nil {
							timer.Stop()
						}
						return
					case context.DeadlineExceeded:
						logger.Warning("etcd fetch deadline exceeded")
					case rpctypes.ErrEmptyKey:
						logger.Warning("etcd fetch empty key")
					case rpctypes.ErrKeyNotFound:
						logger.Warning("etcd fetch key not found")
					default:
						logger.Error("etch fetch failed:", err)
					}
					e.cc.ReportError(err)
				} else {
					err = e.cc.UpdateState(*state)
				}
				if err == nil {
					backoffIndex = 0
					if timer != nil {
						timer.Stop()
					}
					break
				} else {
					if backoffIndex >= 64 {
						logger.Warning("fetch failed retry too much times")
						e.cc.ReportError(errors.New("etcd: failed to fetch,retry too many times"))
						if timer != nil {
							timer.Stop()
						}
						break
					}
					backOff := backoffIndex * 2
					if backOff == 0 {
						backOff = 1
					}
					backoffIndex = backOff
					timer = time.NewTimer(time.Duration(backOff) * time.Second)
					select {
					case <-timer.C:
						timer.Stop()
					case <-e.ctx.Done():
						timer.Stop()
						return
					}
				}
			}
		case <-e.ctx.Done():
			return
		}
	}
}

func (e *etcdResolver) watcher() {
	defer e.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	watcher := e.cli.Watch(e.ctx, e.svrPath, clientv3.WithPrefix())

	for {
		select {
		case <-ticker.C:
			//定时刷新一下
			e.ResolveNow(resolver.ResolveNowOptions{})
		case watcherResponse := <-watcher:
			err := watcherResponse.Err()
			if err != nil {
				logger.Error("watcher failed:", err)
				e.cc.ReportError(err)
				return
			}

			if len(watcherResponse.Events) > 0 {
				e.ResolveNow(resolver.ResolveNowOptions{})
			}
		case <-e.ctx.Done():
			return
		}
	}
}
