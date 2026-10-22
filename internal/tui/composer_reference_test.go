package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

func TestReferenceAutocompleteInMiddleOfText(t *testing.T) {
	a := refFixture(t)
	a.setInput([]rune("please /skill:api now"))
	a.st.Cursor = len("please /skill:api")
	if !a.acActive() {
		t.Fatal("popup should open for /skill:api mid-text")
	}
	press(a, tcell.KeyTab, 0)
	if got := labelText(a.st.Input); got != "please /skill:api-security now" || a.st.Cursor != len("please ")+2 {
		t.Fatalf("accepted: %q cursor %d", got, a.st.Cursor)
	}
	if a.acActive() {
		t.Fatal("popup should close after completion")
	}
	// Skills use '/', only enabled ones are offered, and a path is not a trigger.
	a.setInput([]rune("use /skill:a here"))
	a.st.Cursor = len("use /skill:a")
	if m := a.acMatches(); len(m) != 1 || m[0].label() != "/skill:api-security" {
		t.Fatalf("skill matches %+v", m)
	}
	a.setInput([]rune("use /skill:d"))
	if len(a.acMatches()) != 0 {
		t.Fatal("disabled skill offered")
	}
	a.setInput([]rune("see src/a"))
	if len(a.acMatches()) != 0 {
		t.Fatal("sigil inside a word must not trigger")
	}
}

func TestReferenceHighlight(t *testing.T) {
	a := refFixture(t)
	a.setInput([]rune("fix with /skill:api-security, ignore /skill:nope and /usr/bin"))
	toks := a.inputTokens()
	if len(toks) != 1 || toks[0].name != "api-security" {
		t.Fatalf("tokens %+v", toks)
	}
	a.st.Mode = render.ModeInput
	a.draw()
	s := a.scr.(tcell.SimulationScreen)
	find := func(text string) (int, int) {
		for y := 0; y < 30; y++ {
			if i := strings.Index(visualRow(s, y), text); i >= 0 {
				return i, y
			}
		}
		t.Fatalf("%q not on screen", text)
		return 0, 0
	}
	fg := func(text string) tcell.Color {
		x, y := find(text)
		_, _, st, _ := s.GetContent(x, y)
		f, _, _ := st.Decompose()
		return f
	}
	if fg("/skill:api-security") != render.Blue || fg("/skill:nope") != render.FgWhite || fg("/usr/bin") != render.FgWhite {
		t.Fatal("wrong highlight colors")
	}
}

func TestEnterOnCompleteReferenceSelectsSkill(t *testing.T) {
	a := refFixture(t)
	a.setInput([]rune("go /skill:api-security"))
	if !a.acActive() {
		t.Fatal("popup expected while the word is a valid prefix")
	}
	if !a.acKey(tcell.NewEventKey(tcell.KeyEnter, 0, 0)) || !strings.Contains(a.messageText(), "<skill>api-security</skill>") {
		t.Fatal("Enter must select a skill object without submitting")
	}
}
