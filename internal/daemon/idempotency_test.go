package daemon

import (
	"sync"
	"testing"
	"time"
)

func TestClientIDRetrySafety(t *testing.T) {
	i := NewIdempotency()
	var wg sync.WaitGroup
	calls := 0
	var mu sync.Mutex
	for n := 0; n < 10; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := i.Do("same", func() (any, error) {
				mu.Lock()
				calls++
				mu.Unlock()
				time.Sleep(time.Millisecond)
				return "created", nil
			})
			if e != nil || v != "created" {
				t.Errorf("result %v %v", v, e)
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("operation ran %d times", calls)
	}
}
