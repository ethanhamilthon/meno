package render

import (
	"strings"
	"testing"
)

func TestSentMessageShowsObjectLabelsAndKeepsOldTextAlone(t *testing.T) {
	turns := []Turn{
		{Role: RoleUser, Text: "look at <file-link>/proj/main.go</file-link> and /not/a/tag.go then <skill>review</skill>"},
		{Role: RoleUser, Text: "legacy path /proj/old.go stays plain, <file-link>relative.go</file-link> too"},
	}
	lines := BuildLines(turns, 100, JSOpts{})
	var all []string
	var blue []string
	for _, l := range lines {
		var b strings.Builder
		for _, s := range l.Segs {
			b.WriteString(s.Text)
			if s.FG == Blue {
				blue = append(blue, s.Text)
			}
		}
		all = append(all, b.String())
	}
	text := strings.Join(all, "\n")
	if !strings.Contains(text, "look at @main.go and /not/a/tag.go then /skill:review") {
		t.Fatalf("labels:\n%s", text)
	}
	if strings.Join(blue, "|") != "@main.go|/skill:review" {
		t.Fatalf("only objects are highlighted: %v", blue)
	}
	if !strings.Contains(text, "legacy path /proj/old.go stays plain, <file-link>relative.go</file-link> too") {
		t.Fatalf("plain paths and malformed tags are not rewritten:\n%s", text)
	}
	if strings.Contains(text, objMark) {
		t.Fatal("the internal marker must never reach the screen text")
	}
}

func TestLabelStaysOneHighlightedUnitWhenItWraps(t *testing.T) {
	turns := []Turn{{Role: RoleUser, Text: "x <file-link>/proj/a-very-long-file-name-that-wraps.go</file-link> y"}}
	lines := BuildLines(turns, 24, JSOpts{})
	var blue []string
	for _, l := range lines {
		for _, s := range l.Segs {
			if s.FG == Blue {
				blue = append(blue, s.Text)
			}
		}
	}
	if len(blue) < 2 || strings.Join(blue, "") != "@a-very-long-file-name-that-wraps.go" {
		t.Fatalf("a wrapped label stays highlighted across rows: %q", blue)
	}
}
