package client

import (
	"io"
	"meno/internal/proto"
	"net/http"
	"strings"
	"testing"
)

func TestSSEParser(t *testing.T) {
	c := &Client{}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(": ping\n\nid: 4\nevent: text_delta\ndata: {\"text\":\"hi\"}\n\n"))}
	var got []proto.Event
	if e := c.readSSE(resp, func(e proto.Event) error { got = append(got, e); return nil }); e != nil {
		t.Fatal(e)
	}
	if len(got) != 1 || got[0].Seq != 4 || got[0].Type != proto.EventTextDelta {
		t.Fatalf("events %#v", got)
	}
}
