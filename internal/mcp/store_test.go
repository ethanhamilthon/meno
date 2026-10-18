package mcp

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

func resultJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseResultKeepsTextAndStructuredApart(t *testing.T) {
	r, err := ParseResult(resultJSON(t, map[string]any{"isError": true, "content": []any{
		map[string]any{"type": "text", "text": `{"looks":"like json"}`},
		map[string]any{"type": "image", "data": "AAAA", "mimeType": "image/png"},
		map[string]any{"type": "text", "text": "two"},
	}}))
	if err != nil {
		t.Fatal(err)
	}
	if r.Text != `{"looks":"like json"}`+"\ntwo" || r.Structured != nil || !r.IsError || len(r.Content) != 3 {
		t.Fatalf("%+v", r)
	}
	r, _ = ParseResult(json.RawMessage(`{"content":[],"structuredContent":{"n":1}}`))
	if r.Text != "" || string(r.Structured) != `{"n":1}` {
		t.Fatalf("structured must not leak into text: %+v", r)
	}
}

func TestStoreMaterializesBinaryAndReadsBackBounded(t *testing.T) {
	s := ResultStore{Dir: t.TempDir()}
	png := []byte("\x89PNG....binary")
	enc := base64.StdEncoding.EncodeToString
	res, err := ParseResult(resultJSON(t, map[string]any{
		"content": []any{
			map[string]any{"type": "text", "text": "héllo wörld 日本語"},
			map[string]any{"type": "image", "data": enc(png), "mimeType": "image/png"},
			map[string]any{"type": "resource", "resource": map[string]any{"uri": "file:///x", "mimeType": "application/pdf", "blob": enc([]byte("%PDF"))}},
		},
		"structuredContent": map[string]any{"a": []int{1, 2}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Save("sess1", "srv", "tool", res)
	if err != nil || len(st.Artifacts) != 2 || len(st.Notes) != 0 {
		t.Fatalf("%+v %v", st, err)
	}
	b, err := os.ReadFile(st.Artifacts[0].Path)
	if err != nil || string(b) != string(png) || st.Artifacts[0].Mime != "image/png" || !strings.HasSuffix(st.Artifacts[0].Path, ".png") {
		t.Fatalf("artifact: %q %v %+v", b, err, st.Artifacts[0])
	}
	// The canonical text has references, not base64 payloads.
	full, _ := s.Read("sess1", st.ID, 0, 1<<20)
	if strings.Contains(full.Text, enc(png)) || !strings.Contains(full.Text, "artifact-1.png") || full.NextOffset != nil {
		t.Fatalf("canonical form: %s", full.Text)
	}
	// Reading in small steps never splits a rune and reassembles the whole.
	var got strings.Builder
	off := 0
	for i := 0; i < 1000; i++ {
		sl, err := s.Read("sess1", st.ID, off, 7)
		if err != nil || !utf8.ValidString(sl.Text) || sl.Offset < off {
			t.Fatalf("slice at %d: %+v %v", off, sl, err)
		}
		got.WriteString(sl.Text)
		if sl.NextOffset == nil {
			break
		}
		off = *sl.NextOffset
	}
	if got.String() != full.Text {
		t.Fatalf("reassembled text differs")
	}
	if _, err := s.Read("other-session", st.ID, 0, 10); err == nil {
		t.Fatal("a result must not be readable from another session")
	}
	if _, err := s.Read("sess1", "../x", 0, 10); err == nil {
		t.Fatal("path traversal in an id")
	}
	if info, err := s.Info("sess1", st.ID); err != nil || len(info.Artifacts) != 2 || info.Size != full.Size {
		t.Fatalf("%+v %v", info, err)
	}
	if err := s.DeleteSession("sess1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Info("sess1", st.ID); err == nil {
		t.Fatal("deleted with the session")
	}
}

func TestStoreReportsUndecodablePayload(t *testing.T) {
	s := ResultStore{Dir: t.TempDir()}
	res, _ := ParseResult(resultJSON(t, map[string]any{"content": []any{map[string]any{"type": "image", "data": "!!!not base64", "mimeType": "image/png"}}}))
	st, err := s.Save("s", "a", "b", res)
	if err != nil || len(st.Artifacts) != 0 || len(st.Notes) != 1 {
		t.Fatalf("%+v %v", st, err)
	}
}
