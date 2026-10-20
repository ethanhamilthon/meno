package app

import (
	"strings"
	"testing"

	"meno/internal/ui"
)

func TestEditModeNavigatesAndEditsWithoutSelecting(t *testing.T) {
	a := visualApp(t)
	a.st.Session.Replace(nil)
	a.setInput([]rune("alpha beta"))
	a.st.Cursor = 0
	a.runCommand("edit")
	if a.st.Mode != ui.ModeEdit {
		t.Fatalf("mode %v", a.st.Mode)
	}
	vimKeys(a, "wx")
	if string(a.st.Input) != "alpha eta" || a.st.Mode != ui.ModeEdit {
		t.Fatalf("%q %v", string(a.st.Input), a.st.Mode)
	}
	vimKeys(a, "vl")
	if a.st.Mode != ui.ModeVisual {
		t.Fatal("v must start selection")
	}
	vimKeys(a, "<esc>")
	if a.st.Mode != ui.ModeEdit {
		t.Fatal("visual Esc must return to EDIT")
	}
	vimKeys(a, "<esc>")
	if a.st.Mode != ui.ModeInput {
		t.Fatal("Esc must return to INPUT")
	}
}

func TestEditModeColonAndInsert(t *testing.T) {
	a := visualApp(t)
	a.setInput([]rune("ab"))
	a.runCommand("edit")
	vimKeys(a, ":")
	if a.st.Mode != ui.ModeEdit || len(a.st.Input) != 2 {
		t.Fatal("colon must do nothing in EDIT")
	}
	a.runCommand("edit")
	vimKeys(a, "iX")
	if a.st.Mode != ui.ModeInput || !strings.Contains(string(a.st.Input), "X") {
		t.Fatalf("%v %q", a.st.Mode, string(a.st.Input))
	}
}

func TestEditModeTimelineIsReadOnlyAndVIsGone(t *testing.T) {
	a := visualApp(t)
	a.setInput(nil)
	a.runCommand("edit")
	if !a.st.InLog || a.st.Mode != ui.ModeEdit {
		t.Fatal("empty draft must put the cursor in history")
	}
	vimKeys(a, "x")
	if !strings.Contains(a.lastNotice(), "read-only") {
		t.Fatal(a.lastNotice())
	}
	for _, c := range commandList {
		if c.name == ":v" || c.name == ":visual" {
			t.Fatal("old command listed")
		}
	}
}
