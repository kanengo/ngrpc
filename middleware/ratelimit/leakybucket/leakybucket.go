package leakybucket

import (
	"sync"
	"time"

	"github.com/kanengo/ngrpc/middleware/ratelimit"
)

var (
	_ ratelimit.Limiter = (*LeakyBucket)(nil)
)

type LeakyBucket struct {
	capacity        int64
	remainingTokens int64
	fillRate        time.Duration
	lastFilled      time.Time
	mu              sync.Mutex
}

func (lb *LeakyBucket) Allow() error {
	if !lb.TryAcquire(1) {
		return ratelimit.ErrTriggerLimit
	}
	return nil
}

func NewLeakyBucket(capacity int64, fillRate time.Duration) *LeakyBucket {
	return &LeakyBucket{
		capacity:        capacity,
		remainingTokens: capacity,
		fillRate:        fillRate,
		lastFilled:      time.Now(),
	}
}

func (lb *LeakyBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(lb.lastFilled)
	newTokens := int64(elapsed / lb.fillRate)
	if newTokens > 0 {
		lb.remainingTokens += newTokens
		if lb.remainingTokens > lb.capacity {
			lb.remainingTokens = lb.capacity
		}
		lb.lastFilled = now
	}
}

func (lb *LeakyBucket) TryAcquire(n int64) bool {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.refill()
	if lb.remainingTokens >= n {
		lb.remainingTokens -= n
		return true
	}
	return false
}

func (lb *LeakyBucket) GetWaitTime(n int64) time.Duration {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.refill()
	if lb.remainingTokens >= n {
		return 0
	}
	neededTokens := n - lb.remainingTokens
	return time.Duration(neededTokens) * lb.fillRate
}
