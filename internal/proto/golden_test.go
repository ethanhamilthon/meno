package proto

import (
	"encoding/json"
	"testing"
)

func TestWireJSONGoldens(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"hello", Hello{Protocol: 1, Version: "v", BootID: "b", PID: 7, StartedAt: "now"}, `{"protocol":1,"version":"v","boot_id":"b","pid":7,"started_at":"now"}`},
		{"error", ErrorEnvelope{Error: ErrorBody{Code: CodeNotFound, Message: "missing"}}, `{"error":{"code":"not_found","message":"missing"}}`},
		{"sessions", SessionsListResponse{Sessions: []SessionItem{{ID: "s", Cwd: "/w", Running: true}}}, `{"sessions":[{"id":"s","cwd":"/w","running":true,"needs_you":false}]}`},
		{"create", SessionCreateRequest{Cwd: "/w", ClientID: "uuid"}, `{"cwd":"/w","client_id":"uuid"}`},
		{"message", MessageRequest{Text: "hi", ClientID: "uuid"}, `{"text":"hi","client_id":"uuid"}`},
		{"snapshot", Snapshot{Session: SessionItem{ID: "s", Cwd: "/w"}, Cwd: "/w", Seq: 4}, `{"session":{"id":"s","cwd":"/w","running":false,"needs_you":false},"cwd":"/w","running":false,"seq":4,"turn_count":0,"live":{},"metrics":{},"task_count":0,"agent_count":0}`},
		{"event", Event{Seq: 2, Type: EventTextDelta, Data: json.RawMessage(`{"text":"x"}`)}, `{"seq":2,"type":"text_delta","data":{"text":"x"}}`},
		{"call", ToolCall{ID: "c", Name: "bash", Arguments: json.RawMessage(`{"command":"echo"}`)}, `{"id":"c","name":"bash","arguments":{"command":"echo"}}`},
		{"ask", AskAnswerRequest{Answers: map[string]json.RawMessage{"q": json.RawMessage(`"a"`)}}, `{"answers":{"q":"a"}}`},
		{"task", Task{ID: "t", Status: "running"}, `{"id":"t","status":"running"}`},
		{"agent", Agent{ID: "a", Status: "running"}, `{"id":"a","status":"running"}`},
		{"resource", ResourceRequest{ClientID: "x", Data: json.RawMessage(`{"v":1}`)}, `{"client_id":"x","data":{"v":1}}`},
		{"usage", UsageData{InputTokens: 3, OutputTokens: 5}, `{"input_tokens":3,"output_tokens":5}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, e := json.Marshal(tt.value)
			if e != nil {
				t.Fatal(e)
			}
			if string(b) != tt.want {
				t.Fatalf("got %s\nwant %s", b, tt.want)
			}
		})
	}
}
