package ewma

import (
	"container/list"
	"context"
	stderrors "errors"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kanengo/ngrpc/errors"
	"github.com/kanengo/ngrpc/selector"
)

const (
	// The mean lifetime of `cost`, it reaches its half-life after Tau*ln(2).
	tau = int64(time.Millisecond * 600)
	// if statistic not collected,we add a big lag penalty to endpoint
	penalty = uint64(time.Second * 10)
)

var (
	_ selector.WeightNode        = (*Node)(nil)
	_ selector.WeightNodeBuilder = (*Builder)(nil)
)

type Node struct {
	selector.Node
	inflight int64
	lag      int64
	success  uint64
	weight   int64

	predictTs int64
	predict   int64 //估算延迟

	inflights *list.List //当前进行中的请求

	lastPick   int64
	mu         sync.RWMutex
	errHandler func(err error) bool
}

func (n *Node) PickLastTime() int64 {
	return atomic.LoadInt64(&n.lastPick)
}

func (n *Node) health() uint64 {
	return atomic.LoadUint64(&n.success)
}

func (n *Node) load() (load uint64) {
	now := time.Now().UnixNano()
	avgLag := atomic.LoadInt64(&n.lag)
	lastPredictTs := atomic.LoadInt64(&n.predictTs)
	predictInterval := avgLag / 5
	if predictInterval < int64(time.Millisecond*5) {
		predictInterval = int64(time.Millisecond * 5)
	} else if predictInterval > int64(time.Millisecond*200) {
		predictInterval = int64(time.Millisecond * 200)
	}
	if now-lastPredictTs > predictInterval && atomic.CompareAndSwapInt64(&n.predictTs, lastPredictTs, now) {
		var (
			total   int64
			count   int
			predict int64
		)
		n.mu.RLock()
		first := n.inflights.Front()
		for first != nil {
			lag := now - first.Value.(int64)
			if lag > avgLag {
				count++
				total += lag
			}
			first = first.Next()
		}
		if count > (n.inflights.Len()/2 + 1) {
			predict = total / int64(count)
		}
		n.mu.RUnlock()
		atomic.StoreInt64(&n.predict, predict)

	}
	if avgLag == 0 {
		load = penalty * uint64(atomic.LoadInt64(&n.inflight))
		return
	}
	predict := atomic.LoadInt64(&n.predict)
	if predict > avgLag {
		avgLag = predict
	}
	load = uint64(avgLag) * uint64(atomic.LoadInt64(&n.inflight))
	return
}

func (n *Node) Weight() float64 {
	return float64(n.health()*uint64(time.Second)) / float64(n.load())
}

func (n *Node) Raw() selector.Node {
	return n.Node
}

func (n *Node) Pick() selector.DoneFunc {
	now := time.Now().UnixNano()
	atomic.AddInt64(&n.inflight, 1)
	atomic.StoreInt64(&n.lastPick, now)
	n.mu.Lock()
	e := n.inflights.PushBack(now)
	n.mu.Unlock()
	return func(ctx context.Context, di selector.DoneInfo) {
		n.mu.Lock()
		n.inflights.Remove(e)
		n.mu.Unlock()
		atomic.AddInt64(&n.inflight, -1)
		doneNow := time.Now().UnixNano()
		td := doneNow - now
		if td < 0 {
			td = 0
		}

		w := math.Exp(float64(-td) / float64(tau))
		lag := td
		if lag < 0 {
			lag = 0
		}
		oldLag := atomic.LoadInt64(&n.lag)
		if oldLag == 0 {
			w = 0.0
		}
		lag = int64(float64(oldLag)*w + float64(lag)*(1.0-w))
		atomic.StoreInt64(&n.lag, lag)

		success := uint64(1000)
		if di.Err != nil {
			err := errors.FromError(di.Err)
			if err.Internal() {
				success = 0
			} else {
				var netError net.Error
				if stderrors.Is(context.DeadlineExceeded, di.Err) || stderrors.Is(context.Canceled, di.Err) ||
					stderrors.As(di.Err, &netError) {
					success = 0
				}
			}
		}
		oldSuccess := atomic.LoadUint64(&n.success)
		success = uint64(float64(oldSuccess)*w + float64(success)*(1.0-w))
		atomic.StoreUint64(&n.success, success)
	}
}

type Builder struct {
	ErrHandler func(err error) bool
}

func (b *Builder) Build(node selector.Node) selector.WeightNode {
	n := Node{
		Node:       node,
		success:    1000,
		inflight:   1,
		inflights:  list.New(),
		errHandler: b.ErrHandler,
	}

	return &n
}

func NewBuilder() selector.WeightNodeBuilder {
	return &Builder{}
}
