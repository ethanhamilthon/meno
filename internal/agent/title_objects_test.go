package agent

import (
	"meno/internal/llm"
	"strings"
	"testing"
)

func TestAutomaticTitleObjectLabels(t *testing.T) {
	cases := map[string]string{
		"Inspect <file-link>/repo/main.go</file-link>":               "Inspect @main.go",
		"<folder-link>/repo/src</folder-link> <skill>review</skill>": "@src/ /review",
		"\n  hello\t世界\x00\u2063 ":                                   "hello 世界",
		"<literal>x</literal>":                                       "<literal>x</literal>",
		strings.Repeat("界", 61):                                      strings.Repeat("界", 60),
	}
	for text, want := range cases {
		s := &Session{Turns: []Turn{{Role: llm.RoleUser, Text: text}}}
		if got := deriveTitle(s); got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}
