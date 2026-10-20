package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
	"meno/internal/ui"
)

// gutterDecor are glyphs that may sit in the gutter: rules, markers, icons.
const gutterDecor = "─│┌┐└┘├┤┬┴┼━┃▸▾●○◆◇◫▎▏█▀▄░▒▓⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏ "

// gutterLeaks lists cells in the left or right gutter that hold text.
func gutterLeaks(s tcell.SimulationScreen, skipRows map[int]bool) []string {
	return gutterLeaksWith(s, skipRows, gutterDecor)
}

// welcomeDecor adds the glyphs of the welcome background wave (a texture).
const welcomeDecor = gutterDecor + ".:-=+*#"

func gutterLeaksWith(s tcell.SimulationScreen, skipRows map[int]bool, decor string) []string {
	w, h := s.Size()
	var out []string
	for y := 0; y < h; y++ {
		if skipRows[y] {
			continue
		}
		for x := 0; x < w; x++ {
			if x >= ui.Gutter && x < w-ui.Gutter {
				continue
			}
			r, _, _, _ := s.GetContent(x, y)
			if !strings.ContainsRune(decor, r) {
				out = append(out, string(r)+"@"+itoa(x)+","+itoa(y))
			}
		}
	}
	return out
}

func gutterApp(t *testing.T, w int) (*App, tcell.SimulationScreen) {
	a, s := visualFixture(t, w, 30)
	long := strings.Repeat("словo word 世界 ", 12)
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: long})
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: long + "\n\n- item " + long + "\n\n```go\nfmt.Println(1)\n```"})
	a.st.Session.Title = "a long session title that is cut"
	return a, s
}

func TestTextNeverEntersTheGutter(t *testing.T) {
	for _, w := range []int{20, 40, 80} {
		a, s := gutterApp(t, w)
		a.setInput([]rune(strings.Repeat("input text ", 6)))
		a.draw()
		if leaks := gutterLeaks(s, nil); len(leaks) > 0 {
			t.Errorf("width %d idle: %v\n%s", w, leaks, screenText(s))
		}
		a.st.Running = true
		a.draw()
		if leaks := gutterLeaks(s, nil); len(leaks) > 0 {
			t.Errorf("width %d running: %v", w, leaks)
		}
		a.setInput([]rune("x :mo"))
		a.draw()
		if leaks := gutterLeaks(s, nil); len(leaks) > 0 {
			t.Errorf("width %d popup: %v\n%s", w, leaks, screenText(s))
		}
	}
}

func TestStatusBarTextStartsAfterTheGutter(t *testing.T) {
	a, s := gutterApp(t, 80)
	a.draw()
	row := visualRow(s, 29)
	if !strings.Contains(row, a.footerModel()) {
		t.Fatalf("model label missing: %q", row)
	}
	if !strings.HasSuffix(strings.TrimRight(row, " "), ")") && !strings.Contains(row, "400k") {
		t.Fatalf("right side missing: %q", row)
	}
	if strings.TrimRight(row, " ") != strings.TrimRight(row[:len(row)-ui.Gutter], " ") && strings.HasSuffix(row, "k") {
		t.Fatalf("right text reaches the edge: %q", row)
	}
}

func TestOverlaysAndWelcomeKeepTheGutter(t *testing.T) {
	for _, w := range []int{40, 80} {
		a, s := visualFixture(t, w, 30)
		a.draw()
		if l := gutterLeaksWith(s, nil, welcomeDecor); len(l) > 0 {
			t.Errorf("welcome %d: %v\n%s", w, l, screenText(s))
		}
		a, s = gutterApp(t, w)
		for _, name := range []string{"context", "help", "model", "settings", "skills", "mcp", "sessions", "tasks", "fork", "login"} {
			a.runCommand(name)
			a.draw()
			// Dialog surfaces are measured against their own edge (see the plan);
			// the dimmed backdrop and screen edges must still be clear of text.
			if l := gutterLeaks(s, nil); len(l) > 0 {
				t.Errorf("%s %d: %v", name, w, l)
			}
			a.closeOverlay()
		}
	}
}

func TestQueuePanelKeepsTheGutter(t *testing.T) {
	a, s := gutterApp(t, 60)
	a.st.Running = true
	a.st.Queued = []string{strings.Repeat("queued message ", 8)}
	a.draw()
	if l := gutterLeaks(s, nil); len(l) > 0 {
		t.Fatal(l)
	}
}
