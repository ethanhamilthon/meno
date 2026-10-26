package render

import (
	"meno/internal/config"
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

func TestDefaultMatchesFilePalette(t *testing.T) {
	colors := config.DefaultThemeColors()
	if PaletteFromColors(colors.Colors()) != DefaultPalette() {
		t.Fatal("default palette mismatch")
	}
}
