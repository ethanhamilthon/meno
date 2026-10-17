package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadSSE(t *testing.T) {
	in := "event: a\ndata: 1\n\n: comment\ndata: 2\ndata: 3\n\n"
	var got []SSEEvent
	if err := ReadSSE(strings.NewReader(in), func(e SSEEvent) error { got = append(got, e); return nil }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Event != "a" || got[0].Data != "1" || got[1].Data != "2\n3" {
		t.Fatalf("parsed: %+v", got)
	}
}

func TestValidateProvider(t *testing.T) {
	cases := map[string]Provider{
		"empty name":   {Name: "", BaseURL: "https://a", Protocol: ProtoChat},
		"slash name":   {Name: "a/b", BaseURL: "https://a", Protocol: ProtoChat},
		"bad url":      {Name: "a", BaseURL: "ftp://a", Protocol: ProtoChat},
		"bad protocol": {Name: "a", BaseURL: "https://a", Protocol: "x"},
	}
	for name, p := range cases {
		if Validate(p) == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if err := Validate(Provider{Name: "ok", BaseURL: "http://localhost:1", Protocol: ProtoResponses}); err != nil {
		t.Fatalf("valid rejected: %v", err)
	}
}

func TestMask(t *testing.T) {
	if Mask("") != "(not set)" {
		t.Fatal("empty")
	}
	if !strings.HasSuffix(Mask("sk-proj-abc9a"), "9a") {
		t.Fatal("suffix")
	}
}

func TestFilterModels(t *testing.T) {
	all := []Model{{ID: "gpt-5.5", Provider: "openai"}, {ID: "llama3.1", Provider: "local"}}
	if len(FilterModels(all, "")) != 2 {
		t.Fatal("empty filter must keep all")
	}
	for _, m := range FilterModels(all, "GPT") {
		if !strings.Contains(m.ID, "gpt") {
			t.Fatalf("leak %s", m.ID)
		}
	}
	if len(FilterModels(all, "zzz")) != 0 {
		t.Fatal("no match should be empty")
	}
}

func TestParseModelsSortsAndRejectsEmpty(t *testing.T) {
	ms, err := parseModels("p", []byte(`{"data":[{"id":"b"},{"id":"a"},{"id":""}]}`))
	if err != nil || len(ms) != 2 || ms[0].ID != "a" || ms[0].Provider != "p" {
		t.Fatalf("got %+v %v", ms, err)
	}
	if _, err := parseModels("p", []byte(`{"data":[]}`)); err == nil {
		t.Fatal("empty list must error")
	}
	if _, err := parseModels("p", []byte(`not json`)); err == nil {
		t.Fatal("bad json must error")
	}
}

func TestFetchModelsUsesBearerAndSkipsHostOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" || r.Header.Get("Authorization") != "Bearer k" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"data":[{"id":"m1"}]}`))
	}))
	defer srv.Close()
	ms, err := FetchModels(context.Background(), Provider{Name: "p", BaseURL: srv.URL + "/v1", APIKey: "k", Protocol: ProtoChat})
	if err != nil || len(ms) != 1 || ms[0].ID != "m1" {
		t.Fatalf("got %+v %v", ms, err)
	}
	_, err = FetchModels(context.Background(), Provider{Name: "p", BaseURL: srv.URL + "/v1", Protocol: ProtoChat})
	if err == nil {
		t.Fatal("401 must be an error")
	}
}

func collect(t *testing.T, srv *httptest.Server, proto string) []Event {
	t.Helper()
	p := Provider{Name: "t", BaseURL: srv.URL, Protocol: proto}
	var evs []Event
	err := NewClient(p).Stream(context.Background(), Request{Provider: p, Model: "m", Effort: "high", System: "s", Msgs: []Msg{{Role: RoleUser, Text: "hi"}}, Tools: true}, func(e Event) { evs = append(evs, e) })
	if err != nil {
		t.Fatal(err)
	}
	return evs
}

func TestResponsesStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Write([]byte("event: x\ndata: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"th\"}\n\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n" +
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"function_call\",\"call_id\":\"c1\",\"arguments\":\"{\\\"code\\\":\\\"1+1\\\"}\"}}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":5,\"output_tokens\":7}}}\n\n"))
	}))
	defer srv.Close()
	evs := collect(t, srv, ProtoResponses)
	if len(evs) != 4 || evs[0].Kind != EvThinking || evs[1].Text != "hel" || evs[2].Code != "1+1" || evs[2].CallID != "c1" || evs[3].Usage.Out != 7 {
		t.Fatalf("events: %+v", evs)
	}
}

func TestChatStreamToolCallAssembly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"A\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"t1\",\"function\":{\"arguments\":\"{\\\"code\\\":\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"2+2\\\"}\"}}]}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" +
			"data: {\"choices\":[],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":4}}\n\n" +
			"data: [DONE]\n\n"))
	}))
	defer srv.Close()
	evs := collect(t, srv, ProtoChat)
	var tool *Event
	for i := range evs {
		if evs[i].Kind == EvTool {
			tool = &evs[i]
		}
	}
	if tool == nil || tool.Code != "2+2" || tool.CallID != "t1" {
		t.Fatalf("tool assembly: %+v", evs)
	}
	if evs[len(evs)-1].Kind != EvDone || evs[len(evs)-1].Usage.In != 3 {
		t.Fatalf("usage: %+v", evs)
	}
}

func TestHTTPErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", 401)
	}))
	defer srv.Close()
	p := Provider{Name: "t", BaseURL: srv.URL, Protocol: ProtoChat}
	err := NewClient(p).Stream(context.Background(), Request{Provider: p, Msgs: []Msg{{Role: RoleUser, Text: "x"}}}, func(Event) {})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("want 401 error, got %v", err)
	}
}

func TestStreamTerminalValidation(t *testing.T) {
	tool := `{"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"js","arguments":"{\"code\":\"print(1)\"}"}}`
	chatTool := `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"js","arguments":"{\"code\":\"print(1)\"}"}}]}}]}`
	for _, tt := range []struct {
		name, proto, stream, want string
		tools                     int
	}{
		{"responses EOF", ProtoResponses, tool, "unexpected EOF", 1},
		{"responses DONE only", ProtoResponses, tool + "\n[DONE]", "unexpected EOF", 1},
		{"responses failed", ProtoResponses, tool + `
{"type":"response.failed","response":{"status":"failed","error":{"code":"server_error","message":"backend failed"}}}`, "backend failed", 1},
		{"responses incomplete", ProtoResponses, tool + `
{"type":"response.incomplete","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`, "max_output_tokens", 1},
		{"responses nested error", ProtoResponses, `{"type":"error","error":{"message":"nested failure"}}`, "nested failure", 0},
		{"responses false completion", ProtoResponses, tool + `
{"type":"response.completed","response":{"status":"incomplete","incomplete_details":{"reason":"max_output_tokens"}}}`, "max_output_tokens", 1},
		{"responses missing call ID", ProtoResponses, `{"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call","name":"js","arguments":"{}"}}
{"type":"response.completed"}`, "missing call_id", 0},
		{"chat EOF", ProtoChat, chatTool, "unexpected EOF", 0},
		{"chat failed", ProtoChat, chatTool + `
{"error":{"message":"backend failed"}}
[DONE]`, "backend failed", 0},
		{"chat length", ProtoChat, chatTool + `
{"choices":[{"delta":{},"finish_reason":"length"}]}
[DONE]`, "length", 0},
		{"chat filtered", ProtoChat, chatTool + `
{"choices":[{"delta":{},"finish_reason":"content_filter"}]}
[DONE]`, "content_filter", 0},
		{"chat malformed", ProtoChat, chatTool + "\n{", "chat stream", 0},
		{"chat finish only", ProtoChat, chatTool + `
{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`, "", 1},
		{"chat DONE only", ProtoChat, chatTool + "\n[DONE]", "", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for _, data := range strings.Split(tt.stream, "\n") {
					_, _ = w.Write([]byte("data: " + data + "\n\n"))
				}
			}))
			defer srv.Close()
			p := Provider{BaseURL: srv.URL, Protocol: tt.proto}
			var events []Event
			err := NewClient(p).Stream(context.Background(), Request{Provider: p, Tools: true}, func(e Event) { events = append(events, e) })
			if tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
			if tt.want == "" && err != nil {
				t.Fatal(err)
			}
			tools, done := 0, 0
			for _, e := range events {
				if e.Kind == EvTool {
					tools++
				}
				if e.Kind == EvDone {
					done++
				}
			}
			if tools != tt.tools || (tt.want != "" && done != 0) || (tt.want == "" && done != 1) {
				t.Fatalf("events: %+v", events)
			}
		})
	}
}

func TestResponsesOutputDeduplication(t *testing.T) {
	for _, tt := range []struct {
		name, first, second string
		conflict            bool
	}{
		{"item ID", `{"id":"r1","type":"reasoning","summary":[]}`, `{"summary":[],"type":"reasoning","id":"r1"}`, false},
		{"call ID", `{"type":"function_call","call_id":"c1","name":"js","arguments":"{}"}`, `{"type":"function_call","call_id":"c1","name":"js","arguments":"{}"}`, false},
		{"item conflict", `{"id":"r1","type":"reasoning","summary":[]}`, `{"id":"r1","type":"reasoning","summary":["changed"]}`, true},
		{"call conflict", `{"id":"f1","type":"function_call","call_id":"c1","arguments":"{}"}`, `{"id":"f2","type":"function_call","call_id":"c1","arguments":"{}"}`, true},
		{"changed call ID", `{"id":"f1","type":"function_call","call_id":"c1","arguments":"{}"}`, `{"id":"f1","type":"function_call","call_id":"c2","arguments":"{}"}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for _, item := range []string{tt.first, tt.second} {
					_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":` + item + "}\n\n"))
				}
				_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n"))
			}))
			defer srv.Close()
			p := Provider{BaseURL: srv.URL, Protocol: ProtoResponses}
			items, done := 0, 0
			err := NewClient(p).Stream(context.Background(), Request{Provider: p}, func(e Event) {
				if e.Kind == EvTool || e.Kind == EvHistory {
					items++
				}
				if e.Kind == EvDone {
					done++
				}
			})
			if (err != nil) != tt.conflict || items != 1 || (tt.conflict && done != 0) || (!tt.conflict && done != 1) {
				t.Fatalf("err=%v items=%d done=%d", err, items, done)
			}
		})
	}
}
