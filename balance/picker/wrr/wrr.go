package wrr

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"ngrpc/errors"

	"github.com/kanengo/goutil/pkg/metric"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/grpclog"
)

const Name = "wrr"

var logger = grpclog.Component("wrr")

// newBuilder creates a new wrr balancer builder.
func newBuilder() balancer.Builder {
	return base.NewBalancerBuilder(Name, &wrrPickerBuilder{}, base.Config{HealthCheck: true})
}

func RegisterBalancer() {
	balancer.Register(newBuilder())
}

func init() {
	balancer.Register(newBuilder())
}

type wrrPickerBuilder struct {
}

func (*wrrPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	logger.Infof("wrrPicker: Build called with info: %v", info)
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	p := &wrrPicker{
		subConns: nil,
		updateAt: 0,
		mu:       sync.Mutex{},
	}
	uid := int64(0)
	for sc := range info.ReadySCs {
		uid += 1
		_sc := &subConn{
			conn: sc,
			err: metric.NewRollingCounter(metric.RollingCounterOpts{
				Size:           10,
				BucketDuration: 100 * time.Millisecond,
			}),
			latency: metric.NewRollingGauge(metric.RollingGaugeOpts{
				Size:           10,
				BucketDuration: 100 * time.Millisecond,
			}),
			wt:    10,
			ewt:   10,
			cwt:   0,
			score: 01,
			id:    uid,
			si:    serverInfo{},
		}
		p.subConns = append(p.subConns, _sc)
	}

	return p
}

type serverInfo struct {
	cpu     int64
	mem     int64
	success uint64
}

type subConn struct {
	conn    balancer.SubConn
	err     metric.RollingCounter
	latency metric.RollingGauge
	wt      int64
	ewt     int64
	cwt     int64
	score   float64
	id      int64
	si      serverInfo
}

func (sc *subConn) errSummary() (errC int64, reqC int64) {
	sc.err.Reduce(func(iterator metric.BucketIterator) float64 {
		for iterator.Next() {
			bucket := iterator.Bucket()
			reqC += bucket.Count
			for _, point := range bucket.Points {
				errC += int64(point)
			}
		}
		return 0
	})
	return
}

func (sc *subConn) latencySummary() (latency float64, count int64) {
	sc.latency.Reduce(func(iterator metric.BucketIterator) float64 {
		for iterator.Next() {
			bucket := iterator.Bucket()
			count += bucket.Count
			for _, p := range bucket.Points {
				latency += p
			}
		}
		return 0
	})

	return latency / float64(count), count
}

type wrrPicker struct {
	// subConns is the snapshot of the roundrobin balancer when this picker was
	// created. The slice is immutable. Each Get() will do a round robin
	// selection from it and return the selected SubConn.
	subConns []*subConn
	updateAt int64

	mu sync.Mutex
}

func (p *wrrPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	if len(p.subConns) == 0 {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	var (
		conn        *subConn
		totalWeight int64
	)

	p.mu.Lock()
	for _, sc := range p.subConns {
		totalWeight += sc.ewt
		sc.cwt += sc.ewt
		if conn == nil || sc.cwt > conn.cwt {
			conn = sc
		}
	}

	if conn == nil {
		p.mu.Unlock()
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	conn.cwt -= totalWeight
	p.mu.Unlock()

	start := time.Now()
	done := func(doneInfo balancer.DoneInfo) {
		ev := int64(0)
		if doneInfo.Err != nil {
			if e, ok := errors.FromError(doneInfo.Err); ok && e.Internal() {
				ev = 1
			}
		}
		conn.err.Add(ev)

		now := time.Now()
		latency := now.Sub(start).Nanoseconds() / 1e5
		conn.latency.Add(latency)

		u := atomic.LoadInt64(&p.updateAt)
		if now.UnixNano()-u < int64(time.Second) {
			return
		}

		if !atomic.CompareAndSwapInt64(&p.updateAt, u, now.UnixNano()) {
			return
		}

		var (
			count int
			total float64
		)

		for _, conn := range p.subConns {
			errC, reqC := conn.errSummary()
			lagV, lagC := conn.latencySummary()

			if reqC > 0 && lagC > 0 && lagV > 0 {
				cs := 1 - (float64(errC) / float64(reqC))
				if cs <= 0 {
					cs = 0.1
				} else if cs <= 0.2 && reqC <= 5 {
					cs = 0.2
				}
				conn.score = math.Sqrt((cs * 1e9) / lagV)
			}

			if conn.score > 0 {
				total += conn.score
				count++
			}
		}
		if count < 2 {
			return
		}

		avgScore := total / float64(count)
		p.mu.Lock()
		for _, conn := range p.subConns {
			if conn.score <= 0 {
				conn.score = avgScore
			}
			conn.ewt = int64(conn.score * float64(conn.wt))
		}
		p.mu.Unlock()
	}

	return balancer.PickResult{SubConn: conn.conn, Done: done}, nil
}
