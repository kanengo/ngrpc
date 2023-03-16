package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kanengo/ngrpc/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	_ registry.Registrar = (*Registry)(nil)
	_ registry.Discovery = (*Registry)(nil)
)

type options struct {
	ctx       context.Context
	namespace string
	ttl       time.Duration
	maxRetry  int
}

type Option func(*options)

// Context with registry context.
func Context(ctx context.Context) Option {
	return func(o *options) { o.ctx = ctx }
}

// Namespace with registry namespace.
func Namespace(ns string) Option {
	return func(o *options) { o.namespace = ns }
}

// RegisterTTL with register ttl.
func RegisterTTL(ttl time.Duration) Option {
	return func(o *options) { o.ttl = ttl }
}

func MaxRetry(num int) Option {
	return func(o *options) { o.maxRetry = num }
}

type Registry struct {
	opts   *options
	client *clientv3.Client
	lease  clientv3.Lease
}

func New(client *clientv3.Client, opt ...Option) *Registry {
	opts := &options{
		ctx:       context.Background(),
		namespace: "",
		ttl:       time.Second * 15,
		maxRetry:  5,
	}

	for _, o := range opt {
		o(opts)
	}

	r := &Registry{
		opts:   opts,
		client: client,
	}

	return r
}

func (r *Registry) ListService(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	//TODO implement me
	panic("implement me")
}

func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	//TODO implement me
	panic("implement me")
}

func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, ins.Name, ins.ID)
	b, err := json.Marshal(ins)
	if err != nil {
		return err
	}
	if r.lease != nil {
		_ = r.lease.Close()
	}
	value := string(b)
	r.lease = clientv3.NewLease(r.client)
	leaseID, err := r.registerWithKV(r.opts.ctx, key, value)
	if err != nil {
		return err
	}

	go r.heartbeat(r.opts.ctx, leaseID, key, value)

	return nil
}

func (r *Registry) DeRegister(ctx context.Context, ins *registry.ServiceInstance) error {
	key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, ins.Name, ins.ID)
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer func() {
		cancel()
		if r.lease != nil {
			_ = r.lease.Close()
		}
	}()
	_, err := r.client.Delete(timeoutCtx, key)
	return err
}

func (r *Registry) registerWithKV(ctx context.Context, key, val string) (clientv3.LeaseID, error) {
	grant, err := r.lease.Grant(ctx, int64(r.opts.ttl.Seconds()))
	if err != nil {
		return 0, err
	}

	_, err = r.client.Put(ctx, key, val, clientv3.WithLease(grant.ID))
	if err != nil {
		return 0, err
	}

	return grant.ID, nil
}

func (r *Registry) heartbeat(ctx context.Context, leaseID clientv3.LeaseID, key, val string) {
	curLeaseID := leaseID
	kac, err := r.client.KeepAlive(ctx, leaseID)
	if err != nil {
		curLeaseID = 0
	}

	for {
		if curLeaseID == 0 {
			for retryCnt := 0; retryCnt < r.opts.maxRetry; retryCnt++ {
				if ctx.Err() != nil { //ctx done
					return
				}

				timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*3)
				id, err := r.registerWithKV(timeoutCtx, key, val)
				cancel()
				if err == nil {
					continue
				}
				curLeaseID = id
				kac, err = r.client.KeepAlive(ctx, curLeaseID)
				if err == nil {
					break
				}
				time.Sleep(time.Second * 3)
			}
		}
		select {
		case _, ok := <-kac:
			if !ok {
				if ctx.Err() != nil { //ctx has done
					return
				}
				curLeaseID = 0
				continue
			}
		case <-r.opts.ctx.Done():
			return
		}
	}
}
