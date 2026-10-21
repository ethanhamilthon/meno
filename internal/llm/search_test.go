package llm

import (
	"context"
	"errors"
	"testing"
)

func TestSearchUnsupportedProtocol(t *testing.T) {
	_, err := Search(context.Background(), Provider{Name: "p", Protocol: ProtoChat}, "m", "q")
	if !errors.Is(err, ErrSearchUnsupported) {
		t.Fatalf("got %v", err)
	}
}

func TestParseSearchCollectsCitations(t *testing.T) {
	raw := []byte(`{"output":[{"type":"web_search_call"},{"type":"message","content":[{"text":"Go is fast.","annotations":[{"type":"url_citation","url":"https://go.dev","title":"Go"},{"type":"url_citation","url":"https://go.dev","title":"dup"}]}]}]}`)
	a, err := parseSearch(raw)
	if err != nil || a.Text != "Go is fast." || len(a.Sources) != 1 || a.Sources[0].URL != "https://go.dev" {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err = parseSearch([]byte(`{"output":[]}`)); err == nil {
		t.Fatal("an empty answer must be an error, not a made-up one")
	}
}
