package acp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"meno/internal/api"
	"meno/internal/client"
)

// scriptBackend lets a test push projection updates into a followed session.
type scriptBackend struct {
	Backend
	mu   sync.Mutex
	recv func(client.ProjectionUpdate) error
	up   chan struct{}
}

func (b *scriptBackend) CreateSession(context.Context, api.SessionCreateRequest) (api.SessionCreateResponse, error) {
	return api.SessionCreateResponse{Session: api.SessionItem{ID: "s1"}}, nil
}
func (b *scriptBackend) Models(context.Context, api.ModelsRequest) (api.ModelsResponse, error) {
	return api.ModelsResponse{}, nil
}
func (b *scriptBackend) Skills(context.Context) (api.SkillsResponse, error) {
	return api.SkillsResponse{}, nil
}
func (b *scriptBackend) Follow(ctx context.Context, _ string, recv func(client.ProjectionUpdate) error) error {
	_ = recv(client.ProjectionUpdate{Reset: true, Changed: client.AllChanged})
	b.mu.Lock()
	b.recv = recv
	b.mu.Unlock()
	close(b.up)
	<-ctx.Done()
	return ctx.Err()
}
func (b *scriptBackend) push(u client.ProjectionUpdate) {
	b.mu.Lock()
	r := b.recv
	b.mu.Unlock()
	_ = r(u)
}

func liveUpdate(text string) client.ProjectionUpdate {
	return client.ProjectionUpdate{Changed: client.LiveChanged, Values: client.MirrorState{Live: api.LiveState{Text: text}}}
}

func stepTurn(id, text string) api.Turn {
	return api.Turn{ID: id, Role: "assistant", Parts: []api.TurnPart{{Type: "text", Text: text}}}
}

// The live text is cleared in the same update that commits the turn: the text
// must reach the client once.
func TestCommittedTextIsNotSentAgain(t *testing.T) {
	for _, tc := range []struct {
		name  string
		steps func(b *scriptBackend)
	}{
		{"clear and commit together", func(b *scriptBackend) {
			b.push(liveUpdate("Yes, that is it."))
			u := liveUpdate("")
			u.Changed |= client.TurnsChanged
			u.Values.Turns = []api.Turn{stepTurn("t1", "Yes, that is it.")}
			b.push(u)
		}},
		{"clear then commit", func(b *scriptBackend) {
			b.push(liveUpdate("Yes, that is it."))
			b.push(liveUpdate(""))
			b.push(client.ProjectionUpdate{Changed: client.TurnsChanged, Values: client.MirrorState{Turns: []api.Turn{stepTurn("t1", "Yes, that is it.")}}})
		}},
		{"commit then clear", func(b *scriptBackend) {
			b.push(liveUpdate("Yes, that is it."))
			b.push(client.ProjectionUpdate{Changed: client.TurnsChanged, Values: client.MirrorState{Turns: []api.Turn{stepTurn("t1", "Yes, that is it.")}}})
			b.push(liveUpdate(""))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := &scriptBackend{up: make(chan struct{})}
			p := startPeer(t, b)
			newSessionOn(t, p)
			<-b.up
			tc.steps(b)
			b.push(liveUpdate("Next."))
			p.waitFor("text", func() bool { return len(p.text("agent_message_chunk")) >= len("Yes, that is it.Next.") })
			if got := p.text("agent_message_chunk"); got != "Yes, that is it.Next." {
				t.Fatalf("text = %q", got)
			}
		})
	}
}

// A notification that belongs to a reply is written after it, never before.
func TestAfterReplyRunsAfterTheResponse(t *testing.T) {
	var out lockedBuf
	c := newConn(&out)
	methods := map[string]handler{"m": func(json.RawMessage) (any, error) {
		return afterReply{value: map[string]any{"ok": true}, then: func() { _ = c.Notify("n", nil) }}, nil
	}}
	_ = c.serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"m"}`+"\n"), methods, nil)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"result"`) || !strings.Contains(lines[1], `"method":"n"`) {
		t.Fatalf("wire = %q", lines)
	}
}
