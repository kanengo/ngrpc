package srebreaker

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kanengo/goutil/pkg/metric"
	"github.com/kanengo/ngrpc/errors"
	"github.com/kanengo/ngrpc/middleware/circuitbreaker"
)

type sreBreaker struct {
	stat metric.RollingCounter
	r    *rand.Rand

	mu sync.Mutex

	k       float64
	request int64

	state int32
}

type Config struct {
	K       float64
	Request int64

	Bucket int
	Window time.Duration
}

func (c *Config) fix() {
	if c.K == 0 {
		c.K = 1.5
	}

	if c.Request == 0 {
		c.Request = 100
	}

	if c.Bucket == 0 {
		c.Bucket = 10
	}

	if c.Window == 0 {
		c.Window = time.Second * 3
	}
}

func New(c *Config) circuitbreaker.Breaker {
	if c == nil {
		c = &Config{}
	}
	c.fix()
	counterOpts := metric.RollingCounterOpts{
		Size:           c.Bucket,
		BucketDuration: c.Window / time.Duration(c.Bucket),
	}
	stat := metric.NewRollingCounter(counterOpts)
	return &sreBreaker{
		stat:    stat,
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
		k:       c.K,
		request: c.Request,
		state:   circuitbreaker.StateClosed,
	}
}

func (b *sreBreaker) Allow() error {
	success, total := b.summary()
	k := b.k * float64(success)
	if total < b.request || float64(total) < k {
		if atomic.LoadInt32(&b.state) == circuitbreaker.StateOpened {
			atomic.CompareAndSwapInt32(&b.state, circuitbreaker.StateOpened, circuitbreaker.StateClosed)
		}
		return nil
	}

	if atomic.LoadInt32(&b.state) == circuitbreaker.StateClosed {
		atomic.CompareAndSwapInt32(&b.state, circuitbreaker.StateClosed, circuitbreaker.StateOpened)
	}

	dr := math.Max(0, (float64(total)-k)/float64(total+1))
	drop := b.trueOnProba(dr)
	if drop {
		return errors.ServiceUnavailable("service unavailable")
	}

	return nil
}

func (b *sreBreaker) MarkSuccess() {
	b.stat.Add(1)
}

func (b *sreBreaker) MarkFailed() {
	b.stat.Add(0)
}

func (b *sreBreaker) summary() (success int64, total int64) {
	b.stat.Reduce(func(iterator metric.BucketIterator) float64 {
		for iterator.Next() {
			bucket := iterator.Bucket()
			total += bucket.Count
			for _, p := range bucket.Points {
				success += int64(p)
			}
		}
		return 0
	})
	return
}

func (b *sreBreaker) trueOnProba(proba float64) (truth bool) {
	b.mu.Lock()
	truth = b.r.Float64() < proba
	b.mu.Unlock()
	return
}
