package ui

import (
	"encoding/json"
	"testing"
)

func TestToolTurnsHaveNoExtraGap(t *testing.T) {
	opts := JSOpts{}
	tool := Turn{Role: RoleAssistant, Code: "echo one", Output: "one"}
	message := Turn{Role: RoleAssistant, Text: "next step"}
	user := Turn{Role: RoleUser, Text: "do it"}
	count := func(t Turn) int { return len(BuildLines([]Turn{t}, 80, opts)) }
	for _, tc := range []struct {
		name  string
		turns []Turn
		gaps  int
	}{
		{"tools", []Turn{tool, tool, tool}, 0},
		{"user then tools", []Turn{user, tool, tool}, 1},
		{"assistant then tools", []Turn{message, tool, tool}, 1},
		{"tool then assistant", []Turn{tool, message}, 0},
		{"messages", []Turn{user, message, tool}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.gaps
			for _, turn := range tc.turns {
				want += count(turn)
			}
			if got := len(BuildLines(tc.turns, 80, opts)); got != want {
				t.Fatalf("%d rows, want %d", got, want)
			}
		})
	}
}

func TestPureBlackMatchesFilePalette(t *testing.T) {
	raw := []byte(`{"name":"sample","dark":true,"bg":"000000","fg":"EDEDED","dim":"888888","blue":"52A8FF","purple":"BF7AF0","green":"62C073","yellow":"FFCA16","red":"FF6166","orange":"F5A623"}`)
	var want Theme
	for _, th := range Themes() {
		if th.Name == "pure-black" {
			want = th
		}
	}
	got, warnings := ThemesFromJSON([]json.RawMessage{raw})
	if len(warnings) != 0 || len(got) != 1 || got[0].Palette != want.Palette {
		t.Fatalf("palette mismatch: %v", warnings)
	}
}
