package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"meno/internal/llm"
)

// prefixClient records requests; a summary request is the one whose last
// message asks for a summary.
type prefixClient struct {
	mu       sync.Mutex
	reqs     []llm.Request
	failures int // summary requests that fail before one succeeds
	tool     bool
}

func (c *prefixClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	if strings.Contains(req.Msgs[len(req.Msgs)-1].Text, "Summarize this conversation") {
		if c.failures > 0 {
			c.failures--
			return errors.New("down")
		}
		if c.tool && req.Tools {
			emit(llm.Event{Kind: llm.EvTool, Code: "1"})
			emit(llm.Event{Kind: llm.EvDone})
			return nil
		}
		emit(llm.Event{Kind: llm.EvText, Text: "## Goal\nsummary"})
	} else {
		emit(llm.Event{Kind: llm.EvText, Text: "ok"})
	}
	emit(llm.Event{Kind: llm.EvDone})
	return nil
}

func compactFixture() (*Session, []llm.Msg) {
	s := bigSession(10)
	return s, s.ToMsgs()
}

// The summary request carries the session's system prompt and tools, and the
// messages it covers are a byte-identical prefix of the session's own request.
func TestCompactionSharesThePrefixOfTheSession(t *testing.T) {
	cl := &prefixClient{}
	s, full := compactFixture()
	_, _, err := CompactWith(context.Background(), cl, llm.Provider{}, "m", s, CompactOptions{Keep: 4, System: "SYS", Effort: "high"})
	if err != nil || s.Summary != "## Goal\nsummary" {
		t.Fatalf("%v %q", err, s.Summary)
	}
	if len(cl.reqs) != 1 {
		t.Fatalf("%d requests", len(cl.reqs))
	}
	r := cl.reqs[0]
	if r.System != "SYS" || !r.Tools || r.Effort != "high" {
		t.Fatalf("system/tools/effort differ from the session's: %+v", r)
	}
	head := len(r.Msgs) - 1
	if head != len(full)-4 {
		t.Fatalf("%d head messages, want %d", head, len(full)-4)
	}
	for i := 0; i < head; i++ {
		if r.Msgs[i].Text != full[i].Text || r.Msgs[i].Role != full[i].Role {
			t.Fatalf("message %d is not the session's", i)
		}
	}
	if !strings.Contains(r.Msgs[head].Text, "last 4 messages stay") {
		t.Fatalf("instruction: %q", r.Msgs[head].Text)
	}
}

func TestCompactionFallsBackToThePlainRequest(t *testing.T) {
	for name, cl := range map[string]*prefixClient{"error": {failures: 1}, "tool call": {tool: true}} {
		s, _ := compactFixture()
		_, _, err := CompactWith(context.Background(), cl, llm.Provider{}, "m", s, CompactOptions{Keep: 4, System: "SYS"})
		if err != nil || s.Summary == "" {
			t.Fatalf("%s: %v %q", name, err, s.Summary)
		}
		if len(cl.reqs) != 2 || cl.reqs[1].System != "" || len(cl.reqs[1].Msgs) != 1 {
			t.Fatalf("%s: fallback request: %+v", name, cl.reqs)
		}
	}
}

func TestCompactionWithoutSystemUsesThePlainRequest(t *testing.T) {
	cl := &prefixClient{}
	s, _ := compactFixture()
	if _, _, err := Compact(context.Background(), cl, llm.Provider{}, "m", s); err != nil {
		t.Fatal(err)
	}
	if len(cl.reqs) != 1 || cl.reqs[0].Tools || len(cl.reqs[0].Msgs) != 1 {
		t.Fatalf("%+v", cl.reqs)
	}
}

func TestCompactionCancelDoesNotRetry(t *testing.T) {
	cl := &prefixClient{failures: 5}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s, _ := compactFixture()
	if _, _, err := CompactWith(ctx, cl, llm.Provider{}, "m", s, CompactOptions{Keep: 4, System: "SYS"}); err == nil {
		t.Fatal("expected an error")
	}
	if len(cl.reqs) != 1 {
		t.Fatalf("a cancelled compaction asked again: %d requests", len(cl.reqs))
	}
}
