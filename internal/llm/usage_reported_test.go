package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompletionDistinguishesAbsentAndZeroUsage(t *testing.T) {
	for _, protocol := range []string{ProtoChat, ProtoResponses} {
		for _, reported := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reported=%v", protocol, reported), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					usage := ""
					if reported {
						usage = `,"usage":{"prompt_tokens":0,"completion_tokens":0,"input_tokens":0,"output_tokens":0}`
					}
					if protocol == ProtoChat {
						fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]%s}\n\ndata: [DONE]\n\n", usage)
					} else {
						fmt.Fprintf(w, "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"%s}}\n\n", usage)
					}
				}))
				defer server.Close()
				req := Request{Provider: Provider{Name: "test", BaseURL: server.URL, Protocol: protocol}, Model: "m"}
				done := false
				err := NewClient(req.Provider).Stream(context.Background(), req, func(ev Event) {
					if ev.Kind == EvDone {
						done = true
						if ev.UsageReported != reported {
							t.Fatalf("usage presence=%v, want %v", ev.UsageReported, reported)
						}
					}
				})
				if err != nil || !done {
					t.Fatalf("completion: %v done=%v", err, done)
				}
			})
		}
	}
}
