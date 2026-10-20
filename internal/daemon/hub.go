package daemon

import (
	"encoding/json"
	"meno/internal/proto"
	"sync"
)

// Hub publishes ordered session events with bounded replay history.
type Hub struct {
	mu                 sync.Mutex
	bootID             string
	cap, subscriberCap int
	closed             bool
	streams            map[string]*eventStream
}
type eventStream struct {
	next    uint64
	ring    []proto.Event
	subs    map[uint64]chan proto.Event
	nextSub uint64
}

// NewHub creates an event hub. A non-positive ring size uses 2000 events.
func NewHub(bootID string, ringSize int) *Hub {
	if ringSize <= 0 {
		ringSize = 2000
	}
	return &Hub{bootID: bootID, cap: ringSize, subscriberCap: 4096, streams: map[string]*eventStream{}}
}

// SetSubscriberBuffer sets each subscriber's bounded event queue size.
func (h *Hub) SetSubscriberBuffer(n int) {
	if n < 1 {
		n = 1
	}
	h.mu.Lock()
	h.subscriberCap = n
	h.mu.Unlock()
}
func (h *Hub) stream(key string) *eventStream {
	s := h.streams[key]
	if s == nil {
		s = &eventStream{subs: map[uint64]chan proto.Event{}}
		h.streams[key] = s
	}
	return s
}

// BootID returns this daemon process's event epoch.
func (h *Hub) BootID() string { h.mu.Lock(); defer h.mu.Unlock(); return h.bootID }

// SetBootID sets the event epoch before the hub serves subscribers.
func (h *Hub) SetBootID(id string) { h.mu.Lock(); h.bootID = id; h.mu.Unlock() }

// Publish assigns the next sequence number and notifies session subscribers.
func (h *Hub) Publish(sessionID, typ string, data any) proto.Event {
	b, _ := json.Marshal(data)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return proto.Event{}
	}
	s := h.stream(sessionID)
	s.next++
	e := proto.Event{Seq: s.next, Type: typ, BootID: h.bootID, SessionID: sessionID, Data: b}
	s.ring = append(s.ring, e)
	if len(s.ring) > h.cap {
		s.ring = append([]proto.Event(nil), s.ring[len(s.ring)-h.cap:]...)
	}
	for id, ch := range s.subs {
		select {
		case ch <- e:
		default:
			for len(ch) > 0 {
				<-ch
			}
			r := proto.Event{Seq: s.next, Type: proto.EventResync, BootID: h.bootID, SessionID: sessionID, Data: json.RawMessage(`{}`)}
			ch <- r
			close(ch)
			delete(s.subs, id)
		}
	}
	return e
}

// Sequence returns the latest session sequence.
func (h *Hub) Sequence(sessionID string) uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stream(sessionID).next
}

// Subscribe replays events after the cursor or reports resync when history is unsafe.
func (h *Hub) Subscribe(sessionID string, after uint64, boot string) (<-chan proto.Event, func(), bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	s := h.stream(sessionID)
	ch := make(chan proto.Event, h.subscriberCap)
	if h.closed {
		e := proto.Event{Seq: s.next, Type: proto.EventBye, BootID: h.bootID, SessionID: sessionID}
		e.Data, _ = json.Marshal(proto.ByeData{Reason: "daemon stopped"})
		ch <- e
		close(ch)
		return ch, func() {}, false
	}
	if boot != "" && boot != h.bootID || len(s.ring) > 0 && after+1 < s.ring[0].Seq || len(s.ring) == 0 && after < s.next {
		ch <- proto.Event{Seq: s.next, Type: proto.EventResync, BootID: h.bootID, SessionID: sessionID, Data: json.RawMessage(`{}`)}
		close(ch)
		return ch, func() {}, true
	}
	for _, e := range s.ring {
		if e.Seq > after {
			ch <- e
		}
	}
	s.nextSub++
	id := s.nextSub
	s.subs[id] = ch
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if c := s.subs[id]; c != nil {
				delete(s.subs, id)
				close(c)
			}
		})
	}, false
}

// Subscribers returns the count of attached session streams.
func (h *Hub) Subscribers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, s := range h.streams {
		n += len(s.subs)
	}
	return n
}

// Close publishes bye and closes session subscribers.
func (h *Hub) Close(reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for key, s := range h.streams {
		s.next++
		e := proto.Event{Seq: s.next, Type: proto.EventBye, BootID: h.bootID, SessionID: key}
		e.Data, _ = json.Marshal(proto.ByeData{Reason: reason})
		for id, ch := range s.subs {
			select {
			case ch <- e:
			default:
			}
			close(ch)
			delete(s.subs, id)
		}
	}
}
