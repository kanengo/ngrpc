package leakybucket

import (
	"fmt"
	"testing"
	"time"
)

func TestLeakyBucket(t *testing.T) {
	bucket := NewLeakyBucket(10, time.Millisecond*100)

	for i := 0; i < 20; i++ {
		ok := bucket.TryAcquire(1)
		//waitTime := bucket.GetWaitTime(1)
		fmt.Printf("Request %d: Acquired: %v\n", i+1, ok)

		//if !ok {
		//	time.Sleep(waitTime)
		//}
	}
	time.Sleep(500 * time.Millisecond)

	for i := 0; i < 20; i++ {
		ok := bucket.TryAcquire(1)
		//waitTime := bucket.GetWaitTime(1)
		fmt.Printf("Request-2 %d: Acquired: %v\n", i+1, ok)

		//if !ok {
		//	time.Sleep(waitTime)
		//}
	}
}
