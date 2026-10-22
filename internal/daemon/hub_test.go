package daemon

import (
	"fmt"
	"meno/internal/api"
	"testing"
)

func TestHubCutResumeNoLossNoRepeat(t *testing.T) {
	h := NewHub("boot", 2000)
	const n = 1000
	for i := 0; i < 150; i++ {
		h.Publish("s", api.EventTextDelta, map[string]int{"i": i})
	}
	ch, cancel, _ := h.Subscribe("s", 150, "boot")
	defer cancel()
	seen := map[uint64]bool{}
	for start := 150; start < n; {
		end := minInt(start+37, n)
		for i := start; i < end; i++ {
			h.Publish("s", api.EventTextDelta, map[string]int{"i": i})
		}
		for len(ch) > 0 {
			e := <-ch
			if e.Type == api.EventResync {
				t.Fatal("unexpected resync")
			}
			if seen[e.Seq] {
				t.Fatalf("repeated sequence %d", e.Seq)
			}
			seen[e.Seq] = true
		}
		start = end
	}
	for seq := uint64(151); seq <= n; seq++ {
		if !seen[seq] {
			t.Errorf("missing sequence %d", seq)
		}
	}
}
func TestHubRingAndBootResync(t *testing.T) {
	h := NewHub("new", 3)
	for i := 0; i < 5; i++ {
		h.Publish("s", api.EventStatus, struct{}{})
	}
	for _, tc := range []struct {
		after uint64
		boot  string
	}{{1, "new"}, {5, "old"}} {
		_, cancel, resync := h.Subscribe("s", tc.after, tc.boot)
		cancel()
		if !resync {
			t.Fatalf("expected resync for %+v", tc)
		}
	}
}
func TestHubCutsSlowSubscriber(t *testing.T) {
	h := NewHub("b", 8)
	h.SetSubscriberBuffer(1)
	ch, cancel, _ := h.Subscribe("s", 0, "b")
	defer cancel()
	h.Publish("s", api.EventTextDelta, map[string]string{"text": "one"})
	h.Publish("s", api.EventTextDelta, map[string]string{"text": "two"})
	e, ok := <-ch
	if !ok || e.Type != api.EventResync {
		t.Fatalf("expected terminal resync, got %#v %v", e, ok)
	}
	if h.Subscribers() != 0 {
		t.Fatalf("slow subscriber remains")
	}
}
func TestHubLiveDeliveryNoLoss(t *testing.T) {
	h := NewHub("b", 2000)
	for i := 0; i < 20; i++ {
		h.Publish("s", api.EventTextDelta, map[string]int{"n": i})
	}
	ch, cancel, resync := h.Subscribe("s", 0, "b")
	defer cancel()
	if resync {
		t.Fatal("unexpected resync")
	}
	for i := 0; i < 20; i++ {
		if e := <-ch; e.Seq != uint64(i+1) {
			t.Fatalf("sequence %d", e.Seq)
		}
	}
}

func TestHubPublishPerformance(t *testing.T) {
	h := NewHub("b", 2000)
	dur := make([]int64, 5000)
	for i := range dur {
		start := nowNano()
		h.Publish("s", api.EventTextDelta, map[string]string{"text": fmt.Sprint(i)})
		dur[i] = nowNano() - start
	}
	sortInt64(dur)
	if dur[4750] > 5_000_000 {
		t.Fatalf("event p95 %d ns exceeds 5ms", dur[4750])
	}
}
