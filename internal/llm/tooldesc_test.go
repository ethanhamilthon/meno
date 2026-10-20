package llm

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func schemaDescription(desc string) string {
	return ToolSchema(desc, false)["description"].(string)
}

func TestToolSchemaCarriesTheDescription(t *testing.T) {
	if got := schemaDescription("rules and globals"); got != "rules and globals" {
		t.Fatalf("got %q", got)
	}
	if got := schemaDescription(""); got != DefaultToolDescription {
		t.Fatalf("empty must give the default, got %q", got)
	}
}

func TestToolDescriptionIsInBothProtocols(t *testing.T) {
	for _, proto := range []string{ProtoChat, ProtoResponses} {
		var body string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			if proto == ProtoChat {
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
				return
			}
			_, _ = w.Write([]byte("data: {\"type\":\"response.completed\"}\n\n"))
		}))
		p := Provider{Name: "t", BaseURL: srv.URL, Protocol: proto}
		req := Request{Provider: p, Model: "m", System: "s", Msgs: []Msg{{Role: RoleUser, Text: "hi"}}, Tools: true, ToolDescription: "DOC-MARKER"}
		_ = NewClient(p).Stream(context.Background(), req, func(Event) {})
		srv.Close()
		if !strings.Contains(body, "DOC-MARKER") {
			t.Errorf("%s: the request body lacks the tool description: %s", proto, body)
		}
	}
}
