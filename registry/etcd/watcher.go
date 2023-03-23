package etcd

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/kanengo/goutil/pkg/log"

	"github.com/kanengo/ngrpc/registry"

	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	_ registry.Watcher = (*watcher)(nil)
)

type watcher struct {
	key         string
	ctx         context.Context
	cancel      context.CancelFunc
	client      *clientv3.Client
	watchChan   clientv3.WatchChan
	watcher     clientv3.Watcher
	serviceName string

	r      *Registry
	ticker *time.Ticker
}

func newWatcher(ctx context.Context, key string, serviceName string, client *clientv3.Client, r *Registry) (*watcher, error) {
	w := &watcher{
		key:         key,
		ctx:         nil,
		cancel:      nil,
		client:      client,
		watchChan:   nil,
		watcher:     clientv3.NewWatcher(client),
		serviceName: serviceName,
		r:           r,
		ticker:      time.NewTicker(time.Minute),
	}

	w.ctx, w.cancel = context.WithCancel(ctx)
	w.watchChan = w.watcher.Watch(w.ctx, key, clientv3.WithPrefix(), clientv3.WithRev(0), clientv3.WithKeysOnly())
	err := w.watcher.RequestProgress(w.ctx)
	if err != nil {
		return nil, err
	}

	return w, nil
}

func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case watchResp, ok := <-w.watchChan:
		if !ok || watchResp.Err() != nil {
			log.Error("[discovery] watch failed", zap.Bool("ok", ok), zap.Error(watchResp.Err()))
			time.Sleep(time.Second)
			err := w.reWatch()
			if err != nil {
				return nil, err
			}
		}
		//log.Info("Next", zap.Any("watchResp", watchResp))
		return w.getInstance()
	case <-w.ticker.C:
		return w.getInstance()
	}
}

func (w *watcher) getInstance() ([]*registry.ServiceInstance, error) {
	ctx, cancel := context.WithTimeout(w.ctx, w.r.opts.timeout)
	defer cancel()
	return w.r.ListService(ctx, w.serviceName)
}

func (w *watcher) reWatch() error {
	_ = w.watcher.Close()
	w.watcher = clientv3.NewWatcher(w.client)
	w.watchChan = w.watcher.Watch(w.ctx, w.key, clientv3.WithPrefix(), clientv3.WithRev(0), clientv3.WithKeysOnly())
	return w.watcher.RequestProgress(w.ctx)
}

func (w *watcher) Stop() error {
	w.ticker.Stop()
	w.cancel()
	return w.watcher.Close()
}
