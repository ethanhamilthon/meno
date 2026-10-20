package app

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func screenText(scr tcell.SimulationScreen) string {
	w, h := scr.Size()
	var b strings.Builder
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c, _, _, _ := scr.GetContent(x, y)
			b.WriteRune(c)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func TestMainScreenRenders(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.draw()
	out := screenText(scr)
	if !strings.Contains(out, "INPUT") {
		t.Fatalf("status badge missing:\n%s", out)
	}
	if !strings.Contains(out, "gpt-5.5") {
		t.Fatalf("model in the status bar missing:\n%s", out)
	}
}

func TestTypeAndSendPlaceholder(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	for _, r := range "héllo" {
		a.onKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	if string(a.st.Input) != "héllo" {
		t.Fatalf("input %q", string(a.st.Input))
	}
	a.onKey(tcell.NewEventKey(tcell.KeyBackspace2, 0, tcell.ModNone))
	if string(a.st.Input) != "héll" {
		t.Fatalf("backspace %q", string(a.st.Input))
	}
}

func TestCommandModeEnterAndEscape(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.onKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)) // commands start from normal mode
	if a.st.Mode.Label() != "COMMAND" {
		t.Fatalf("mode %s", a.st.Mode.Label())
	}
	a.onKey(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone))
	if a.st.Mode.Label() != "INPUT" {
		t.Fatalf("esc should go to INPUT, got %s", a.st.Mode.Label())
	}
}

func TestQuitCommand(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.runCommand("q")
	if !a.quit {
		t.Fatal(":q did not quit")
	}
}

func TestSettingsPersistThroughStore(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Settings.Model = "o4-mini"
	a.saveSession()
	if a.st.Settings.Model != "o4-mini" {
		t.Fatal("model not kept")
	}
}
