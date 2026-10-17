package agent

import (
	"context"
	"strings"
	"testing"
	"time"

	"meno/internal/jsvm"
	"meno/internal/llm"
)

// scripted replays fixed event lists, one per request, and records the requests.
type timeoutClient struct {
	calls [][]llm.Event
	reqs  []llm.Request
}

func (c *timeoutClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.reqs = append(c.reqs, req)
	i := len(c.reqs) - 1
	if i >= len(c.calls) {
		emit(llm.Event{Kind: llm.EvText, Text: "done"})
		emit(llm.Event{Kind: llm.EvDone})
		return nil
	}
	for _, e := range c.calls[i] {
		emit(e)
	}
	return nil
}

func runWithTimeout(t *testing.T, model bool, args string) (string, *timeoutClient) {
	t.Helper()
	vm := jsvm.New(jsvm.Options{Root: t.TempDir(), Timeout: 30 * time.Second})
	cl := &timeoutClient{calls: [][]llm.Event{{
		{Kind: llm.EvTool, CallID: "c1", Code: "String(sh('sleep 5').timedOut)", Arguments: args},
		{Kind: llm.EvDone},
	}}}
	r := &Runner{D: Deps{Retry: RetryPolicy{Attempts: 1}, Client: cl, VM: vm, ModelTimeout: model, Emit: func(Ev) {}}}
	s := NewSession(t.TempDir(), "m")
	start := time.Now()
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("the call was not stopped by its timeout")
	}
	for _, turn := range s.Snapshot() {
		if turn.CallID == "c1" {
			return turn.Err + turn.Output, cl
		}
	}
	t.Fatal("no tool turn")
	return "", cl
}

func TestModelChoosesTimeout(t *testing.T) {
	out, cl := runWithTimeout(t, true, `{"code":"String(sh('sleep 5').timedOut)","timeout":0.3}`)
	if !cl.reqs[0].ModelTimeout {
		t.Fatal("request does not offer the timeout argument")
	}
	if !strings.Contains(out, "timeout after 300ms") && strings.TrimSpace(out) != "true" {
		t.Fatalf("expected the model's 0.3s timeout to stop the call, got %q", out)
	}
}

func TestTimeoutArgumentIgnoredUnlessModelDecides(t *testing.T) {
	vm := jsvm.New(jsvm.Options{Root: t.TempDir(), Timeout: 300 * time.Millisecond})
	cl := &timeoutClient{calls: [][]llm.Event{{
		{Kind: llm.EvTool, CallID: "c1", Code: "String(sh('sleep 5').timedOut)", Arguments: `{"code":"String(sh('sleep 5').timedOut)","timeout":100}`},
		{Kind: llm.EvDone},
	}}}
	r := &Runner{D: Deps{Retry: RetryPolicy{Attempts: 1}, Client: cl, VM: vm, Emit: func(Ev) {}}}
	s := NewSession(t.TempDir(), "m")
	start := time.Now()
	if err := r.Turn(context.Background(), s, "go"); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 4*time.Second || cl.reqs[0].ModelTimeout {
		t.Fatal("configured timeout must win when the model is not in charge")
	}
}

func TestToolSchemaTimeoutOnlyWhenModelDecides(t *testing.T) {
	props := func(m bool) map[string]any {
		return llm.ToolSchema(m)["parameters"].(map[string]any)["properties"].(map[string]any)
	}
	if _, ok := props(false)["timeout"]; ok {
		t.Fatal("timeout offered although the user sets it")
	}
	if _, ok := props(true)["timeout"]; !ok {
		t.Fatal("timeout missing")
	}
}
