package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestStructuredReplay(t *testing.T) {
	args := ` {"code":"", "extra":true} `
	msgs := []Msg{{Role: RoleAssistant, Text: "checking", ToolCalls: []ToolCall{{ID: "a", Name: ToolName, Arguments: args}, {ID: "b", Name: ToolName, Arguments: `{"code":"2"}`}}}, {Role: RoleTool, CallID: "a", Text: "ok"}, {Role: RoleTool, CallID: "b", Text: "2"}}
	chat := chatMessages("s", msgs)
	calls := chat[1]["tool_calls"].([]any)
	if len(calls) != 2 || chat[1]["content"] != "checking" || calls[0].(map[string]any)["function"].(map[string]any)["arguments"] != args {
		t.Fatalf("chat replay: %#v", chat)
	}
	input := responsesInput(msgs)
	if len(input) != 5 || input[0]["content"] != "checking" || input[1]["arguments"] != args {
		t.Fatalf("responses replay: %#v", input)
	}
	for _, m := range []Msg{{Role: RoleAssistant, CallID: "empty"}, {Role: RoleAssistant, Code: "1"}} {
		if len(responsesInput([]Msg{m})) != 1 || responsesInput([]Msg{m})[0]["type"] != "function_call" {
			t.Fatal("legacy identity lost")
		}
	}
}

func TestResponsesNativeReplay(t *testing.T) {
	items := []json.RawMessage{
		json.RawMessage(`{"type":"reasoning","id":"r","summary":[],"encrypted_content":"opaque"}`),
		json.RawMessage(`{"type":"message","id":"m","role":"assistant","status":"completed","content":[{"type":"output_text","text":"checking","annotations":[]}]}`),
		json.RawMessage(`{"type":"function_call","id":"f","call_id":"a","name":"js","arguments":" {\"code\":\"\"} "}`),
	}
	var events []Event
	for _, item := range items {
		if err := handleResponsesEvent(SSEEvent{Data: `{"type":"response.output_item.done","item":` + string(item) + `}`}, func(e Event) { events = append(events, e) }); err != nil {
			t.Fatal(err)
		}
	}
	var replay Msg
	replay.Role = RoleAssistant
	for _, e := range events {
		replay.Responses = append(replay.Responses, e.Item)
	}
	got := responsesInput([]Msg{replay})
	for i, item := range items {
		var want map[string]any
		_ = json.Unmarshal(item, &want)
		if !reflect.DeepEqual(got[i], want) {
			t.Fatalf("item %d changed: %#v", i, got[i])
		}
	}
	if events[2].Code != "" || events[2].Arguments != " {\"code\":\"\"} " {
		t.Fatalf("call: %+v", events[2])
	}
}

func TestResponsesRequestsEncryptedReasoning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(body["include"], []any{"reasoning.encrypted_content"}) {
			t.Errorf("include: %#v", body["include"])
		}
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
	}))
	defer srv.Close()
	collect(t, srv, ProtoResponses)
}
