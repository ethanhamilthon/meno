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

func TestFilterCatalog(t *testing.T) {
	if len(Filter("")) != len(Catalog()) {
		t.Fatal("empty filter")
	}
	for _, m := range Filter("gpt") {
		if !strings.Contains(m.ID, "gpt") {
			t.Fatalf("leak %s", m.ID)
		}
	}
	if _, ok := Lookup("gpt-5.5"); !ok {
		t.Fatal("lookup")
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
