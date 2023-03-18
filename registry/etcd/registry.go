package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	timeout   time.Duration
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

func Timeout(timeout time.Duration) Option {
	return func(o *options) { o.timeout = timeout }
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
		timeout:   time.Second * 3,
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
	key := fmt.Sprintf("%s/%s", r.opts.namespace, serviceName)
	resp, err := r.client.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	ins := make([]*registry.ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		in, err := unmarshal(kv.Value)
		if err != nil {
			return nil, err
		}
		if in.Name != serviceName {
			continue
		}
		ins = append(ins, in)
	}

	return ins, nil
}

func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	key := fmt.Sprintf("%s/%s", r.opts.namespace, serviceName)
	return newWatcher(ctx, key, serviceName, r.client, r)
}

func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	key := fmt.Sprintf("%s/%s/%s", r.opts.namespace, ins.Name, ins.ID)
	value, err := marshal(ins)
	if err != nil {
		return err
	}
	if r.lease != nil {
		_ = r.lease.Close()
	}

	r.lease = clientv3.NewLease(r.client)
	leaseID, err := r.registerWithKV(r.opts.ctx, key, value)
	if err != nil {
		return err
	}

	go r.heartbeat(r.opts.ctx, leaseID, key, value)

	return nil
}

func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
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
	timeoutCtx, cancel := context.WithTimeout(ctx, r.opts.timeout)
	grant, err := r.lease.Grant(timeoutCtx, int64(r.opts.ttl.Seconds()))
	cancel()
	if err != nil {
		return 0, err
	}
	timeoutCtx, cancel = context.WithTimeout(ctx, r.opts.timeout)
	_, err = r.client.Put(timeoutCtx, key, val, clientv3.WithLease(grant.ID))
	cancel()
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

				id, err := r.registerWithKV(ctx, key, val)
				if err != nil {
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

func marshal(data any) (string, error) {
	b := &strings.Builder{}
	enc := json.NewEncoder(b)
	err := enc.Encode(data)
	if err != nil {
		return "", err
	}

	return b.String(), nil
}

func unmarshal(data []byte) (in *registry.ServiceInstance, err error) {
	err = json.Unmarshal(data, &in)
	return
}
