package agent

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"meno/internal/llm"
)

func TestReplayJSTurnsRendersPlainText(t *testing.T) {
	turns := []Turn{
		{Role: llm.RoleUser, Text: "question"},
		{Role: llm.RoleAssistant, Text: "checking", CallID: "c1", Code: "print(1)", Output: "1"},
		{Role: llm.RoleAssistant, Text: "done"},
	}
	got := ReplayJSTurns(turns)
	want := []llm.Msg{
		{Role: llm.RoleUser, Text: "question"},
		{Role: llm.RoleAssistant, Text: "checking\nran js: print(1)"},
		{Role: llm.RoleUser, Text: "result: 1"},
		{Role: llm.RoleAssistant, Text: "done"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("messages: %#v", got)
	}
}

func TestReplayJSTurnsReadsChatAndResponsesCallShapes(t *testing.T) {
	chat := &llm.Msg{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "chat-1", Name: "js", Arguments: `{"code":"chat()"}`}}}
	responseItem, _ := json.Marshal(map[string]any{"type": "function_call", "call_id": "resp-1", "name": "js", "arguments": `{"code":"responses()"}`})
	responses := &llm.Msg{Role: llm.RoleAssistant, Responses: []json.RawMessage{responseItem}}
	for _, tc := range []struct {
		name     string
		replay   *llm.Msg
		id, code string
	}{
		{"chat", chat, "chat-1", "chat()"}, {"responses", responses, "resp-1", "responses()"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ReplayJSTurns([]Turn{{Role: llm.RoleAssistant, CallID: tc.id, Replay: tc.replay, Output: "ok"}})
			if len(got) != 2 || got[0].Role != llm.RoleAssistant || !strings.HasPrefix(got[0].Text, "ran js: ") || got[1].Role != llm.RoleUser || got[1].Text != "result: ok" {
				t.Fatalf("messages: %#v", got)
			}
		})
	}
}

func TestReplayJSTurnsGroupsMultipleCallsAsText(t *testing.T) {
	replay := &llm.Msg{Role: llm.RoleAssistant, Text: "one answer", ToolCalls: []llm.ToolCall{{ID: "a", Name: "js", Arguments: `{"code":"first()"}`}, {ID: "b", Name: "js", Arguments: `{"code":"second()"}`}}}
	turns := []Turn{{Role: llm.RoleAssistant, Text: "one answer", CallID: "a", Replay: replay, ReplayGroup: "group", Output: "one"}, {Role: llm.RoleAssistant, CallID: "b", ReplayGroup: "group", Code: "second()", Output: "two"}}
	got := ReplayJSTurns(turns)
	if len(got) != 4 || got[0].Text != "one answer\nran js: first()" || got[2].Text != "one answer\nran js: second()" {
		t.Fatalf("messages: %#v", got)
	}
}

func TestReplayJSTurnsKeepsErrorsAndEmptyLegacyCalls(t *testing.T) {
	got := ReplayJSTurns([]Turn{{Role: llm.RoleAssistant, CallID: "old", Replay: &llm.Msg{Role: llm.RoleAssistant, CallID: "old"}, Err: "failed"}})
	if len(got) != 2 || !strings.HasPrefix(got[0].Text, "ran js: ") || got[1].Text != "result: \nERROR: failed" {
		t.Fatalf("messages: %#v", got)
	}
}
