package ui

import (
	"strings"
	"testing"
)

func TestMarkdownStylesAndSingleMarker(t *testing.T) {
	lines := MarkdownLines("# Title\n\n**bold** and `code`\n\n- item one\n- item two\n\n```go\nfmt.Println(1)\n```", 15, "◆")
	markers := 0
	bold, code := false, false
	var text strings.Builder
	for _, l := range lines {
		for _, s := range l.Segs {
			text.WriteString(s.Text)
			markers += strings.Count(s.Text, "◆")
			bold = bold || s.Bold
			code = code || s.FG == Blue
		}
	}
	if markers != 1 || !bold || !code {
		t.Fatalf("markers=%d bold=%v code=%v", markers, bold, code)
	}
	if strings.Contains(text.String(), "**") || strings.Contains(text.String(), "```") || !strings.Contains(text.String(), "•") {
		t.Fatalf("unrendered markdown: %s", text.String())
	}
}
func TestMarkdownLongLineWidths(t *testing.T) {
	for _, width := range []int{1, 5, 20} {
		lines := MarkdownLines(strings.Repeat("**hello** ", 30), width, "◆")
		for _, l := range lines {
			if l.Width() > width+2 {
				t.Fatalf("width %d: %d", width, l.Width())
			}
		}
	}
}
func TestMarkdownIncompleteAndEmpty(t *testing.T) {
	for _, s := range []string{"", "**", "```js\nconst a=1", "[label](https://example.com)", "你好 👩🏽‍💻"} {
		_ = MarkdownLines(s, 0, "◆")
	}
}
