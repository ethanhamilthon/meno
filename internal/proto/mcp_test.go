package proto

import (
	"encoding/json"
	"testing"
)

func TestMCPResultWire(t *testing.T) {
	value := MCPResult{Text: "echo", StructuredContent: json.RawMessage(`{"n":1}`), ResultID: "r1", IsError: true, Artifacts: []MCPArtifact{{Index: 0, Kind: "image", Mime: "image/png", Size: 3, Path: "/image.png"}}, Size: 42, Notes: []string{"note"}}
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"text":"echo","structuredContent":{"n":1},"resultId":"r1","isError":true,"artifacts":[{"index":0,"kind":"image","mime":"image/png","size":3,"path":"/image.png"}],"size":42,"notes":["note"]}`
	if string(b) != want {
		t.Fatalf("wire %s", b)
	}
	var got MCPResult
	if err = json.Unmarshal(b, &got); err != nil || got.ResultID != "r1" || string(got.StructuredContent) != `{"n":1}` {
		t.Fatalf("decode %+v %v", got, err)
	}
}
