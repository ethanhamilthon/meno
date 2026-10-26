package mcp

import (
	"strings"
	"testing"
)

// A stored result is read in pages: DefaultReadBytes when the caller gives no
// limit, never more than MaxReadBytes whatever the caller asks for.
func TestResultStoreReadPageSizes(t *testing.T) {
	s := ResultStore{Dir: t.TempDir()}
	big := strings.Repeat("x", 3*MaxReadBytes)
	st, err := s.Save("sess", "srv", "tool", Result{Text: big, Raw: resultJSON(t, map[string]any{"content": []any{map[string]any{"type": "text", "text": big}}})})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ limit, want int }{
		{0, DefaultReadBytes},
		{100, 100},
		{MaxReadBytes + 5000, MaxReadBytes},
		{10 * MaxReadBytes, MaxReadBytes},
	} {
		got, err := s.Read("sess", st.ID, 0, c.limit)
		if err != nil {
			t.Fatal(err)
		}
		if got.End-got.Offset != c.want {
			t.Errorf("limit %d: read %d bytes, want %d", c.limit, got.End-got.Offset, c.want)
		}
		if got.NextOffset == nil || *got.NextOffset != got.End {
			t.Errorf("limit %d: nextOffset %v, want %d", c.limit, got.NextOffset, got.End)
		}
	}
}

// internal/cli/help/mcp-results.md states these two numbers; change both together.
func TestResultStorePageSizesMatchTheHelp(t *testing.T) {
	if DefaultReadBytes != 10000 || MaxReadBytes != 65536 {
		t.Fatalf("page sizes are %d and %d; update help/mcp-results.md and its test", DefaultReadBytes, MaxReadBytes)
	}
}
