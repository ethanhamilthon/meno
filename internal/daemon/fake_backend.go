package daemon

import (
	"context"
	"strings"
	"sync"

	"meno/internal/proto"
)

// FakeBackend is an in-memory backend for transport and mirror tests.
type FakeBackend struct {
	mu          sync.Mutex
	sessions    map[string]*fakeSession
	next        int
	Hub         *Hub
	idempotency *Idempotency
}
type fakeSession struct {
	item     proto.SessionItem
	snapshot proto.Snapshot
	turns    []proto.Turn
	running  bool
}

// NewFakeBackend returns an empty fake backend that emits events through hub.
func NewFakeBackend(hub *Hub) *FakeBackend {
	return &FakeBackend{sessions: map[string]*fakeSession{}, Hub: hub, idempotency: NewIdempotency()}
}
func (f *FakeBackend) ListSessions(_ context.Context, r proto.SessionsListRequest) (proto.SessionsListResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := proto.SessionsListResponse{Sessions: []proto.SessionItem{}}
	for _, s := range f.sessions {
		if r.Cwd != "" && s.item.Cwd != r.Cwd {
			continue
		}
		if r.Query != "" && !strings.Contains(strings.ToLower(s.item.Name+" "+s.item.Cwd), strings.ToLower(r.Query)) {
			continue
		}
		out.Sessions = append(out.Sessions, s.item)
	}
	if r.Limit > 0 && len(out.Sessions) > r.Limit {
		out.Sessions = out.Sessions[:r.Limit]
	}
	return out, nil
}
func (f *FakeBackend) CreateSession(_ context.Context, r proto.SessionCreateRequest) (proto.SessionCreateResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := fmtID(f.next)
	item := proto.SessionItem{ID: id, Cwd: r.Cwd, Model: r.Model}
	s := &fakeSession{item: item, snapshot: proto.Snapshot{Session: item, Cwd: r.Cwd, Model: r.Model, Live: proto.LiveState{Calls: []proto.CallState{}}}}
	f.sessions[id] = s
	return proto.SessionCreateResponse{Session: item}, nil
}
func (f *FakeBackend) find(id string) (*fakeSession, error) {
	s := f.sessions[id]
	if s == nil {
		return nil, &APIError{Code: proto.CodeNotFound, Message: "session not found"}
	}
	return s, nil
}
func (f *FakeBackend) Snapshot(_ context.Context, id string) (proto.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, e := f.find(id)
	if e != nil {
		return proto.Snapshot{}, e
	}
	v := s.snapshot
	if f.Hub != nil {
		v.Seq = f.Hub.Sequence(id)
	}
	v.Running = s.running
	v.TurnCount = len(s.turns)
	v.Turns = append([]proto.Turn(nil), s.turns...)
	return v, nil
}
func (f *FakeBackend) PatchSession(_ context.Context, id string, r proto.PatchSessionRequest) (proto.SessionItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, e := f.find(id)
	if e != nil {
		return proto.SessionItem{}, e
	}
	if r.Name != nil {
		s.item.Name = *r.Name
	}
	if r.Cwd != nil {
		s.item.Cwd = *r.Cwd
	}
	if r.Model != nil {
		s.item.Model = *r.Model
	}
	s.snapshot.Session = s.item
	return s.item, nil
}
func (f *FakeBackend) DeleteSession(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, e := f.find(id); e != nil {
		return e
	}
	delete(f.sessions, id)
	return nil
}
func (f *FakeBackend) Fork(_ context.Context, id string, r proto.ForkRequest) (proto.ForkResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, e := f.find(id)
	if e != nil {
		return proto.ForkResponse{}, e
	}
	f.next++
	item := s.item
	item.ID = fmtID(f.next)
	copyS := *s
	copyS.item = item
	copyS.snapshot.Session = item
	count := len(s.turns)
	if r.Turn > 0 && r.Turn < count {
		count = r.Turn
	}
	copyS.turns = append([]proto.Turn(nil), s.turns[:count]...)
	f.sessions[item.ID] = &copyS
	return proto.ForkResponse{Session: item}, nil
}
func (f *FakeBackend) Turns(_ context.Context, id string, r proto.TurnsPageRequest) (proto.TurnsPage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, e := f.find(id)
	if e != nil {
		return proto.TurnsPage{}, e
	}
	from := r.From
	if from < 0 {
		from = 0
	}
	if from > len(s.turns) {
		from = len(s.turns)
	}
	end := len(s.turns)
	if r.Limit > 0 && end > from+r.Limit {
		end = from + r.Limit
	}
	return proto.TurnsPage{Turns: append([]proto.Turn(nil), s.turns[from:end]...), From: from, Next: end, Total: len(s.turns)}, nil
}
