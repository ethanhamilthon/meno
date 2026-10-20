package proto

import (
	"encoding/json"
	"testing"
)

func TestRequestAndResponseJSONGoldens(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want string
	}{
		{"snapshot_response", Snapshot{Seq: 1, Live: LiveState{}}, `{"session":{"id":"","cwd":"","running":false,"needs_you":false},"cwd":"","running":false,"seq":1,"turn_count":0,"live":{},"metrics":{},"task_count":0}`},
		{"turn_page", TurnsPage{Turns: []Turn{{Role: "user", Content: "hi"}}, From: 0, Next: 1, Total: 1}, `{"turns":[{"role":"user","content":"hi"}],"from":0,"next":1,"total":1}`},
		{"stop", StopRequest{RequestID: "r"}, `{"request_id":"r"}`},
		{"compact", CompactRequest{ClientID: "c", Keep: 8}, `{"client_id":"c","keep":8}`},
		{"ask_form", AskForm{AskID: "a", Questions: []AskQuestion{}}, `{"ask_id":"a","questions":[]}`},
		{"bye", ByeData{Reason: "shutdown"}, `{"reason":"shutdown"}`},
		{"global_status", GlobalStatus{SessionID: "s", Running: true}, `{"session_id":"s","running":true,"needs_you":false,"done_unseen":false}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, e := json.Marshal(tc.v)
			if e != nil {
				t.Fatal(e)
			}
			if string(b) != tc.want {
				t.Fatalf("got %s want %s", b, tc.want)
			}
		})
	}
}
