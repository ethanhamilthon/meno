package mcp

import (
	"encoding/json"
	"testing"
)

func TestSignature(t *testing.T) {
	for _, c := range []struct{ schema, want string }{
		{`{"type":"object","properties":{"query":{"type":"string"},"limit":{"type":"integer"}},"required":["query"]}`, "{query: string, limit?: number}"},
		{`{"properties":{"z":{"type":"boolean"},"a":{"type":"array","items":{"type":"string"}},"mode":{"enum":["fast","slow"]}},"required":["z"]}`, `{z: boolean, a?: string[], mode?: "fast" | "slow"}`},
		{`{"type":"object"}`, ""},
		{`{"properties":{"x":{"type":["string","null"]},"y":{}}}`, "{x?: string | null, y?: any}"},
		{`not json`, "{...}"},
		{``, "{...}"},
	} {
		if got := (Tool{InputSchema: json.RawMessage(c.schema)}).Signature(); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.schema, got, c.want)
		}
	}
}

func TestExtractTextFallsBackToStructuredContent(t *testing.T) {
	got, err := ExtractText(json.RawMessage(`{"content":[],"structuredContent":{"n":1}}`))
	if err != nil || got != `{"n":1}` {
		t.Fatalf("%q %v", got, err)
	}
}
