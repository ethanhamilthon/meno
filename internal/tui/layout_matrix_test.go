package tui

import (
	"strings"
	"testing"
)

// The terminal sizes the interface has to survive.
var layoutSizes = [][2]int{{120, 40}, {96, 30}, {80, 24}, {60, 20}, {48, 16}}

// Every size draws the main screen and each dialog without a panic, and the
// footer still tells the model and the context where there is room for it.
func TestLayoutMatrixMainScreenAndDialogs(t *testing.T) {
	for _, sz := range layoutSizes {
		a, s := visualFixture(t, sz[0], sz[1])
		for _, name := range []string{"", "model", "resume", "settings", "help", "context", "tasks", "skills", "mcp"} {
			if name != "" {
				a.runCommand(name)
			}
			a.draw()
			if name == "" {
				if !strings.Contains(visualRow(s, sz[1]-1), "gpt-5.5") {
					t.Errorf("%dx%d: no model in the footer: %q", sz[0], sz[1], visualRow(s, sz[1]-1))
				}
				if !strings.Contains(visualRow(s, sz[1]-1), "/") {
					t.Errorf("%dx%d: no context in the footer", sz[0], sz[1])
				}
			}
			a.closeOverlay()
		}
	}
}

// Wide characters and long paths do not break the bottom of the screen.
func TestLayoutMatrixCJKEmojiAndLongPaths(t *testing.T) {
	for _, sz := range layoutSizes {
		a, s := visualFixture(t, sz[0], sz[1])
		a.paths.Cwd = "~/проекты/" + strings.Repeat("очень-длинный-путь/", 6)
		a.setInput([]rune("日本語のテキスト 🙂 with emoji and a long line " + strings.Repeat("字", 60)))
		a.draw()
		for y := 0; y < sz[1]; y++ {
			if w := len([]rune(visualRow(s, y))); w != sz[0] {
				t.Fatalf("%dx%d: row %d has %d cells", sz[0], sz[1], y, w)
			}
		}
	}
}
