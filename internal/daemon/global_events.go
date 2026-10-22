package daemon

import (
	"encoding/json"
	"sync"

	"meno/internal/api"
)

// GlobalHub publishes status changes that span all sessions.
type GlobalHub struct {
	mu            sync.Mutex
	bootID        string
	seq           uint64
	ring          []api.Event
	cap           int
	subs          map[uint64]chan api.Event
	nextSub       uint64
	subscriberCap int
	closed        bool
}

// NewGlobalHub creates a global stream hub with a bounded replay ring.
func NewGlobalHub(bootID string, capacity int) *GlobalHub {
	if capacity <= 0 {
		capacity = 2000
	}
	return &GlobalHub{bootID: bootID, cap: capacity, subscriberCap: 4096, subs: map[uint64]chan api.Event{}}
}

// BootID returns the global stream epoch.
func (g *GlobalHub) BootID() string { g.mu.Lock(); defer g.mu.Unlock(); return g.bootID }

// SetBootID sets the global event epoch before clients attach.
func (g *GlobalHub) SetBootID(id string) { g.mu.Lock(); g.bootID = id; g.mu.Unlock() }

// Publish adds one per-boot global event.
func (g *GlobalHub) Publish(sessionID, typ string, data any) api.Event {
	b, _ := json.Marshal(data)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return api.Event{}
	}
	g.seq++
	e := api.Event{Seq: g.seq, Type: typ, BootID: g.bootID, SessionID: sessionID, Data: b}
	g.ring = append(g.ring, e)
	if len(g.ring) > g.cap {
		g.ring = append([]api.Event(nil), g.ring[len(g.ring)-g.cap:]...)
	}
	for id, ch := range g.subs {
		select {
		case ch <- e:
		default:
			for len(ch) > 0 {
				<-ch
			}
			ch <- api.Event{Seq: g.seq, Type: api.EventResync, BootID: g.bootID, Data: json.RawMessage([]byte("{}"))}
			close(ch)
			delete(g.subs, id)
		}
	}
	return e
}

// Subscribe replays global events after a cursor or asks the client to resync.
func (g *GlobalHub) Subscribe(after uint64, boot string) (<-chan api.Event, func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch := make(chan api.Event, g.subscriberCap)
	if g.closed {
		bye := api.Event{Seq: g.seq, Type: api.EventBye, BootID: g.bootID}
		bye.Data, _ = json.Marshal(api.ByeData{Reason: "daemon stopped"})
		ch <- bye
		close(ch)
		return ch, func() {}, false
	}
	if boot != "" && boot != g.bootID || len(g.ring) > 0 && after+1 < g.ring[0].Seq || len(g.ring) == 0 && after < g.seq {
		ch <- api.Event{Seq: g.seq, Type: api.EventResync, BootID: g.bootID, Data: json.RawMessage([]byte("{}"))}
		close(ch)
		return ch, func() {}, true
	}
	for _, e := range g.ring {
		if e.Seq > after {
			ch <- e
		}
	}
	g.nextSub++
	id := g.nextSub
	g.subs[id] = ch
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			if c := g.subs[id]; c != nil {
				delete(g.subs, id)
				close(c)
			}
		})
	}, false
}

// Sequence returns the latest global sequence.
func (g *GlobalHub) Sequence() uint64 { g.mu.Lock(); defer g.mu.Unlock(); return g.seq }

// Subscribers returns the global client count.
func (g *GlobalHub) Subscribers() int { g.mu.Lock(); defer g.mu.Unlock(); return len(g.subs) }

// Close publishes bye and closes global subscribers.
func (g *GlobalHub) Close(reason string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	g.closed = true
	g.seq++
	e := api.Event{Seq: g.seq, Type: api.EventBye, BootID: g.bootID}
	e.Data, _ = json.Marshal(api.ByeData{Reason: reason})
	for id, ch := range g.subs {
		select {
		case ch <- e:
		default:
		}
		close(ch)
		delete(g.subs, id)
	}
}
