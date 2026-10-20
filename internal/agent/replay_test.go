package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"meno/internal/llm"
)

type replayClient struct {
	stream func(context.Context, llm.Request, func(llm.Event)) error
}

func (c replayClient) Stream(ctx context.Context, r llm.Request, emit func(llm.Event)) error {
	return c.stream(ctx, r, emit)
}

func TestReplayGroupedCallsAndPersistence(t *testing.T) {
	args := ` {"code":"", "extra":true} `
	reasoning := json.RawMessage(`{"type":"reasoning","id":"r","summary":[],"encrypted_content":"secret"}`)
	cl := &scripted{steps: [][]llm.Event{
		{{Kind: llm.EvThinking, Text: "private"}, {Kind: llm.EvHistory, Item: reasoning}, {Kind: llm.EvText, Text: "checking"}, {Kind: llm.EvTool, CallID: "a", Name: "js", Arguments: args}, {Kind: llm.EvTool, CallID: "b", Code: "echo 2", Arguments: `{"code":"console.log(2)"}`}},
		{{Kind: llm.EvText, Text: "done"}},
	}}
	var evs []Ev
	r := newRunner(cl, &evs)
	s := NewSession(".", "m")
	if err := r.Turn(context.Background(), s, "q"); err != nil {
		t.Fatal(err)
	}
	msgs := s.ToMsgs()
	if len(msgs) != 5 || len(msgs[1].ToolCalls) != 2 || msgs[1].ToolCalls[0].Arguments != args || msgs[1].Text != "checking" || msgs[1].Thought != "private" || string(msgs[1].Responses[0]) != string(reasoning) || msgs[2].CallID != "a" || msgs[3].CallID != "b" {
		t.Fatalf("replay: %+v", msgs)
	}
	dir := t.TempDir()
	if err := SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSession(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Stored JSON may reindent opaque items, but their values must be unchanged.
	before, _ := json.Marshal(msgs)
	after, _ := json.Marshal(loaded.ToMsgs())
	var a, b any
	_ = json.Unmarshal(before, &a)
	_ = json.Unmarshal(after, &b)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("replay changed after save/load")
	}
}

func TestLegacyEmptyCodeReplay(t *testing.T) {
	var s Session
	if err := json.Unmarshal([]byte(`{"turns":[{"role":"assistant","call_id":"a","text":"checking","output":"ok","err":"bad"}]}`), &s); err != nil {
		t.Fatal(err)
	}
	msgs := s.ToMsgs()
	// A call of the old js tool is sent as text: the model has no js tool now.
	if len(msgs) != 2 || !strings.Contains(msgs[0].Text, "ran js") || !strings.Contains(msgs[1].Text, "ERROR: bad") {
		t.Fatalf("legacy: %+v", msgs)
	}
}

func TestCompactIncludesErrorsAndPreservesGroups(t *testing.T) {
	s := NewSession(".", "m")
	s.Append(Turn{Role: llm.RoleAssistant, Err: "important failure"})
	replay := llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "a"}, {ID: "b"}}}
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "a", Replay: &replay, ReplayGroup: "g"})
	s.Append(Turn{Role: llm.RoleAssistant, CallID: "b", ReplayGroup: "g"})
	for i := 0; i < 3; i++ {
		s.Append(Turn{Role: llm.RoleUser, Text: "tail"})
	}
	cl := replayClient{stream: func(_ context.Context, r llm.Request, emit func(llm.Event)) error {
		if !strings.Contains(r.Msgs[0].Text, "ERROR: important failure") {
			t.Errorf("error omitted: %s", r.Msgs[0].Text)
		}
		emit(llm.Event{Kind: llm.EvText, Text: "summary"})
		return nil
	}}
	if _, _, err := Compact(context.Background(), cl, llm.Provider{}, "m", s); err != nil {
		t.Fatal(err)
	}
	if len(s.Snapshot()) != 5 || s.Snapshot()[0].Replay == nil {
		t.Fatal("tool group split")
	}
}

func TestFailedStreamCallsHaveOutputs(t *testing.T) {
	cl := replayClient{stream: func(_ context.Context, _ llm.Request, emit func(llm.Event)) error {
		emit(llm.Event{Kind: llm.EvTool, CallID: "a"})
		emit(llm.Event{Kind: llm.EvTool, CallID: "b"})
		return errors.New("disconnect")
	}}
	var evs []Ev
	s := NewSession(".", "m")
	if err := newRunner(cl, &evs).Turn(context.Background(), s, "q"); err == nil {
		t.Fatal("expected error")
	}
	msgs := s.ToMsgs()
	if len(msgs) != 4 || len(msgs[1].ToolCalls) != 2 || !strings.Contains(msgs[2].Text, "disconnect") || msgs[3].CallID != "b" {
		t.Fatalf("incomplete history: %+v", msgs)
	}
}

func TestNativeStreamToolsExecuteOnlyOnceAfterSuccess(t *testing.T) {
	item := `data: {"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"bash","arguments":"{\"command\":\"echo 1\"}"}}` + "\n\n"
	chat := `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"bash","arguments":"{\"command\":\"echo 1\"}"}}]}}]}` + "\n\n"
	for _, tt := range []struct {
		name, proto, stream string
		success             bool
	}{
		{"deduplicated", llm.ProtoResponses, item + item + "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n", true},
		{"failed", llm.ProtoResponses, item + "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"failed\"}}}\n\n", false},
		{"incomplete", llm.ProtoResponses, item + "data: {\"type\":\"response.incomplete\",\"response\":{\"incomplete_details\":{\"reason\":\"max_output_tokens\"}}}\n\n", false},
		{"truncated", llm.ProtoResponses, item, false},
		{"conflicting", llm.ProtoResponses, item + strings.Replace(item, "echo 1", "echo 99", 1), false},
		{"chat truncated", llm.ProtoChat, chat, false},
		{"chat failed", llm.ProtoChat, chat + "data: {\"error\":{\"message\":\"failed\"}}\n\ndata: [DONE]\n\n", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if requests.Add(1) == 1 {
					_, _ = w.Write([]byte(tt.stream))
				} else {
					_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"))
				}
			}))
			defer srv.Close()
			p := llm.Provider{Name: "t", BaseURL: srv.URL, Protocol: tt.proto}
			var events []Ev
			r := newRunner(llm.NewClient(p), &events)
			r.D.Provider = p
			s := NewSession(".", "m")
			err := r.Turn(context.Background(), s, "q")
			if (err == nil) != tt.success {
				t.Fatalf("err=%v", err)
			}
			starts := 0
			for _, e := range events {
				if e.Kind == EvJSStart {
					starts++
				}
			}
			wantStarts, wantRequests := 0, int32(1)
			if tt.success {
				wantStarts, wantRequests = 1, 2
			}
			if starts != wantStarts || requests.Load() != wantRequests {
				t.Fatalf("executions=%d requests=%d", starts, requests.Load())
			}
			if tt.success && strings.TrimSpace(s.Snapshot()[1].Output) != "1" {
				t.Fatalf("unexpected side effects: %+v", s.Snapshot())
			}
		})
	}
}
