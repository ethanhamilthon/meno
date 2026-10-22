package daemon

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"meno/internal/api"
)

// fanout publishes n events to the given number of clients. The publisher waits
// until every reading client has taken each event, as a healthy consumer keeps
// up; with more than two clients the last one never reads.
func fanout(t testing.TB, clients, n int) (delivered []int, cut bool) {
	h := NewHub("b", 4096)
	h.SetSubscriberBuffer(64)
	var wg sync.WaitGroup
	var taken atomic.Int64
	delivered = make([]int, clients)
	readers := clients
	if clients > 2 {
		readers--
	}
	for i := 0; i < clients; i++ {
		ch, stop, resync := h.Subscribe("s", 0, "b")
		if resync {
			t.Fatal("resync on a fresh stream")
		}
		defer stop()
		if i >= readers {
			continue // a stalled client: it never reads its channel
		}
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for e := range ch {
				if e.Type == api.EventResync {
					return
				}
				delivered[i]++
				taken.Add(1)
				if delivered[i] == n {
					return
				}
			}
		}()
	}
	for k := 0; k < n; k++ {
		h.Publish("s", api.EventTextDelta, map[string]int{"n": k})
		for taken.Load() < int64((k+1)*readers) {
			runtime.Gosched()
		}
	}
	wg.Wait()
	return delivered, h.Subscribers() < clients
}

// With 2, 3 and 5 clients every reader gets every event in order and a client
// that stops reading is cut off instead of holding the others back.
func TestFanoutToManyClientsAndOneStalled(t *testing.T) {
	const n = 500
	for _, clients := range []int{2, 3, 5} {
		got, cut := fanout(t, clients, n)
		readers := clients
		if clients > 2 {
			readers--
		}
		for i := 0; i < readers; i++ {
			if got[i] != n {
				t.Errorf("%d clients: reader %d got %d of %d events", clients, i, got[i], n)
			}
		}
		if clients > 2 && !cut {
			t.Errorf("%d clients: the stalled one was not cut", clients)
		}
	}
}

// The event numbers a reader sees are strictly increasing without gaps.
func TestFanoutKeepsOrderWithoutGaps(t *testing.T) {
	h := NewHub("b", 4096)
	chs := make([]<-chan api.Event, 5)
	for i := range chs {
		ch, stop, _ := h.Subscribe("s", 0, "b")
		defer stop()
		chs[i] = ch
	}
	for k := 0; k < 200; k++ {
		h.Publish("s", api.EventTextDelta, k)
	}
	for i, ch := range chs {
		for want := uint64(1); want <= 200; want++ {
			if e := <-ch; e.Seq != want {
				t.Fatalf("client %d: event %d, want %d", i, e.Seq, want)
			}
		}
	}
}

func BenchmarkHubFanout(b *testing.B) {
	for _, clients := range []int{1, 2, 3, 5} {
		b.Run(fmt.Sprintf("clients=%d", clients), func(b *testing.B) {
			h := NewHub("b", 4096)
			h.SetSubscriberBuffer(1 << 16)
			var wg sync.WaitGroup
			for i := 0; i < clients; i++ {
				ch, stop, _ := h.Subscribe("s", 0, "b")
				defer stop()
				wg.Add(1)
				go func() {
					defer wg.Done()
					for range ch {
					}
				}()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.Publish("s", api.EventTextDelta, i)
			}
			b.StopTimer()
		})
	}
}
