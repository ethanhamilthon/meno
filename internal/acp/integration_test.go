package acp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"meno/internal/api"
	"meno/internal/llm"
)

// gated is a provider whose requests can wait for the test.
type gated struct {
	mu      sync.Mutex
	steps   [][]llm.Event
	n       int
	hold    chan struct{} // when set, every request waits for it or for cancel
	started chan struct{}
}

func (g *gated) Stream(ctx context.Context, _ llm.Request, emit func(llm.Event)) error {
	if g.started != nil {
		select {
		case g.started <- struct{}{}:
		default:
		}
	}
	if g.hold != nil {
		select {
		case <-g.hold:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	g.mu.Lock()
	i := g.n
	g.n++
	g.mu.Unlock()
	if i >= len(g.steps) {
		emit(llm.Event{Kind: llm.EvText, Text: "ok"})
		emit(llm.Event{Kind: llm.EvDone})
		return nil
	}
	for _, e := range g.steps[i] {
		emit(e)
	}
	return nil
}

func bashCall(id, cmd string) llm.Event {
	b, _ := json.Marshal(map[string]string{"command": cmd, "title": "run"})
	return llm.Event{Kind: llm.EvTool, CallID: id, Name: "bash", Code: cmd, Arguments: string(b)}
}

func newSessionOn(t *testing.T, p *peer) string {
	t.Helper()
	p.call("initialize", map[string]any{"protocolVersion": 1})
	r := result(t, p.call("session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": []any{}}))
	id, _ := r["sessionId"].(string)
	if id == "" {
		t.Fatalf("no session id: %v", r)
	}
	return id
}

func promptText(t *testing.T, p *peer, id, text string) map[string]any {
	t.Helper()
	return result(t, p.call("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": text}}}))
}

func TestChatToolCardsAndLoad(t *testing.T) {
	g := &gated{steps: [][]llm.Event{
		{bashCall("c1", "echo hi-from-tool"), {Kind: llm.EvDone}},
		{{Kind: llm.EvText, Text: "all "}, {Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}},
	}}
	c, _ := daemonRig(t, g)
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	r := promptText(t, p, id, "hello there")
	if r["stopReason"] != "end_turn" {
		t.Fatalf("stop = %v", r)
	}
	if got := p.text("agent_message_chunk"); !strings.Contains(got, "all done") {
		t.Fatalf("message = %q", got)
	}
	starts, ends := p.updates("tool_call"), p.updates("tool_call_update")
	if len(starts) != 1 || len(ends) != 1 || starts[0]["toolCallId"] != "c1" || ends[0]["status"] != "completed" {
		t.Fatalf("tool cards: %v %v", starts, ends)
	}
	if !strings.Contains(string(mustJSON(ends[0])), "hi-from-tool") {
		t.Fatalf("tool output missing: %v", ends[0])
	}
	// A second adapter loads the history without duplicates.
	q := startPeer(t, c)
	q.call("initialize", map[string]any{"protocolVersion": 1})
	res := q.call("session/load", map[string]any{"sessionId": id, "cwd": "", "mcpServers": []any{}})
	if res.Error != nil {
		t.Fatalf("load: %+v", res.Error)
	}
	if q.text("user_message_chunk") != "hello there" {
		t.Fatalf("replayed user = %q", q.text("user_message_chunk"))
	}
	if got := q.text("agent_message_chunk"); strings.Count(got, "all done") != 1 {
		t.Fatalf("replayed agent = %q", got)
	}
}

func TestCancelStopsOnlyTheOwnedRun(t *testing.T) {
	g := &gated{hold: make(chan struct{}), started: make(chan struct{}, 1)}
	c, _ := daemonRig(t, g)
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	ch := p.start("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "go"}}})
	<-g.started
	p.notify("session/cancel", map[string]any{"sessionId": id})
	m := <-ch
	if r := result(t, m); r["stopReason"] != "cancelled" {
		t.Fatalf("stop = %v", r)
	}
	// The session is usable again.
	close(g.hold)
	if r := promptText(t, p, id, "again"); r["stopReason"] != "end_turn" {
		t.Fatalf("second prompt: %v", r)
	}
}

func TestQuestionWithoutNativeUIUsesChatAndResumes(t *testing.T) {
	g := &gated{hold: make(chan struct{}), started: make(chan struct{}, 1), steps: [][]llm.Event{{{Kind: llm.EvText, Text: "thanks for choosing"}, {Kind: llm.EvDone}}}}
	c, eng := daemonRig(t, g)
	p := startPeer(t, c)
	id := newSessionOn(t, p)
	snap, _ := c.Snapshot(context.Background(), id)
	ch := p.start("session/prompt", map[string]any{"sessionId": id, "prompt": []any{map[string]any{"type": "text", "text": "ask me"}}})
	<-g.started
	asked := make(chan error, 1)
	go func() {
		_, err := eng.Resource(context.Background(), "POST", "/v1/sessions/"+id+"/ask", "", api.ResourceRequest{Token: snap.AgentToken,
			Data: json.RawMessage(`{"form":{"questions":[{"id":"c","text":"Pick","type":"single","options":[{"id":"x","label":"X"},{"id":"y","label":"Y"}]}]}}`)})
		asked <- err
	}()
	r := result(t, <-ch)
	if r["stopReason"] != "end_turn" {
		t.Fatalf("question boundary: %v", r)
	}
	msg := p.text("agent_message_chunk")
	if !strings.Contains(msg, "/answer") || !strings.Contains(msg, "`x`") {
		t.Fatalf("question text: %q", msg)
	}
	var askID string
	for _, f := range strings.Fields(msg) {
		if strings.HasPrefix(f, "`") && strings.Contains(msg, "question id: "+f) {
			askID = strings.Trim(f, "`)")
		}
	}
	if askID == "" {
		t.Fatalf("no ask id in %q", msg)
	}
	// Other prompts do not steer while the question is open.
	if r := promptText(t, p, id, "unrelated"); r["stopReason"] != "end_turn" || !strings.Contains(p.text("agent_message_chunk"), "A question is open") {
		t.Fatalf("unrelated prompt: %v", r)
	}
	// A malformed answer keeps the question open.
	promptText(t, p, id, "/answer "+askID+" not-json")
	if !strings.Contains(p.text("agent_message_chunk"), "The question stays open") {
		t.Fatal("malformed answer was not reported")
	}
	// A valid answer resumes the original run.
	close(g.hold)
	r = promptText(t, p, id, `/answer `+askID+` {"c":"x"}`)
	if r["stopReason"] != "end_turn" {
		t.Fatalf("answer: %v", r)
	}
	if err := <-asked; err != nil {
		t.Fatalf("the agent's ask failed: %v", err)
	}
	if got := p.text("agent_message_chunk"); !strings.Contains(got, "thanks for choosing") {
		t.Fatalf("run did not resume: %q", got)
	}
}

func TestModelConfigurationAndCommands(t *testing.T) {
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	p.call("initialize", map[string]any{"protocolVersion": 1})
	r := result(t, p.call("session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": []any{}}))
	id := r["sessionId"].(string)
	opts, _ := r["configOptions"].([]any)
	if len(opts) == 0 || opts[0].(map[string]any)["id"] != "model" {
		t.Fatalf("config options: %v", r)
	}
	if cur := opts[0].(map[string]any)["currentValue"]; cur != modelValue("test", "fake") {
		t.Fatalf("current model = %v", cur)
	}
	p.waitFor("commands", func() bool { return len(p.updates("available_commands_update")) > 0 })
	if !strings.Contains(string(mustJSON(p.updates("available_commands_update")[0])), `"answer"`) {
		t.Fatal("answer command missing")
	}
	bad := p.call("session/set_config_option", map[string]any{"sessionId": id, "configId": "model", "value": "nope/none"})
	if bad.Error == nil {
		t.Fatal("unknown model accepted")
	}
	ok := p.call("session/set_config_option", map[string]any{"sessionId": id, "configId": "model", "value": modelValue("test", "fake")})
	if ok.Error != nil {
		t.Fatalf("set model: %+v", ok.Error)
	}
	// A control command never reaches the agent.
	promptText(t, p, id, "/rename Release notes")
	p.waitFor("title", func() bool { return len(p.updates("session_info_update")) > 0 })
	if p.updates("session_info_update")[0]["title"] != "Release notes" {
		t.Fatal("title update missing")
	}
	snap, _ := c.Snapshot(context.Background(), id)
	if snap.TurnCount != 0 {
		t.Fatalf("a command created %d turns", snap.TurnCount)
	}
}

func TestMCPServersAreRejectedNotDropped(t *testing.T) {
	c, _ := daemonRig(t, &gated{})
	p := startPeer(t, c)
	p.call("initialize", map[string]any{"protocolVersion": 1})
	m := p.call("session/new", map[string]any{"cwd": t.TempDir(), "mcpServers": []any{map[string]any{"name": "x", "command": "y"}}})
	if m.Error == nil || m.Error.Code != codeInvalidParams {
		t.Fatalf("got %+v", m.Error)
	}
}
