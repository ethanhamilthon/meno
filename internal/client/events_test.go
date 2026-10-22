package client

import (
	"io"
	"meno/internal/api"
	"net/http"
	"strings"
	"testing"
)

func TestSSEParser(t *testing.T) {
	c := &Client{}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(": ping\n\nid: 4\nevent: text_delta\ndata: {\"text\":\"hi\"}\n\n"))}
	var got []api.Event
	if e := c.readSSE(resp, func(e api.Event) error { got = append(got, e); return nil }); e != nil {
		t.Fatal(e)
	}
	if len(got) != 1 || got[0].Seq != 4 || got[0].Type != api.EventTextDelta {
		t.Fatalf("events %#v", got)
	}
}
