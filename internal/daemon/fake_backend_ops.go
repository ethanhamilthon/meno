package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"meno/internal/api"
)

func (f *FakeBackend) Message(_ context.Context, id string, r api.MessageRequest) (api.MessageResponse, error) {
	f.mu.Lock()
	s, e := f.find(id)
	if e != nil {
		f.mu.Unlock()
		return api.MessageResponse{}, e
	}
	s.running = true
	s.snapshot.Running = true
	s.snapshot.Phase = "running"
	rid := r.ClientID
	if rid == "" {
		rid = "fake-request"
	}
	s.snapshot.RequestID = rid
	f.mu.Unlock()
	if f.Hub != nil {
		f.Hub.Publish(id, api.EventStatus, api.StatusData{Running: true, Phase: "running"})
		f.Hub.Publish(id, api.EventTextDelta, api.TextDeltaData{Text: r.Text})
		f.mu.Lock()
		s.running = false
		s.snapshot.Running = false
		s.snapshot.Phase = ""
		s.turns = append(s.turns, api.Turn{Role: "user", Content: r.Text, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)})
		s.snapshot.TurnCount = len(s.turns)
		t := s.turns[len(s.turns)-1]
		f.mu.Unlock()
		f.Hub.Publish(id, api.EventTurnAdded, api.TurnAddedData{Turn: t})
		f.Hub.Publish(id, api.EventStatus, api.StatusData{Running: false})
		f.Hub.Publish(id, api.EventDone, map[string]any{})
	}
	return api.MessageResponse{RequestID: rid}, nil
}
func (f *FakeBackend) Stop(_ context.Context, id string, _ api.StopRequest) error {
	f.mu.Lock()
	s, e := f.find(id)
	if e == nil {
		s.running = false
		s.snapshot.Running = false
	}
	f.mu.Unlock()
	if e == nil && f.Hub != nil {
		f.Hub.Publish(id, api.EventAborted, map[string]any{})
	}
	return e
}
func (f *FakeBackend) Compact(_ context.Context, id string, _ api.CompactRequest) (api.CompactResponse, error) {
	if _, e := f.Snapshot(context.Background(), id); e != nil {
		return api.CompactResponse{}, e
	}
	return api.CompactResponse{}, nil
}
func (f *FakeBackend) AnswerAsk(context.Context, string, string, api.AskAnswerRequest) error {
	return errors.New("no ask is open")
}
func (f *FakeBackend) Resource(context.Context, string, string, string, api.ResourceRequest) (api.ResourceResponse, error) {
	return api.ResourceResponse{Data: json.RawMessage(`{}`)}, nil
}
func (f *FakeBackend) HasWork(context.Context) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.sessions {
		if s.running || s.snapshot.Ask != nil {
			return true
		}
	}
	return false
}
func fmtID(n int) string { return fmt.Sprintf("fake-%06d", n) }
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
