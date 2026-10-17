package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/ui"
)

func key(r rune) *tcell.EventKey { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

func keyK(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }

func typeRunes(a *App, s string) {
	for _, r := range s {
		a.onKey(key(r))
	}
}

func TestWelcomeShownWhenEmpty(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.draw()
	out := screenText(scr)
	for _, want := range []string{":model", "/skill", "███"} {
		if !strings.Contains(out, want) {
			t.Fatalf("welcome missing %q:\n%s", want, out)
		}
	}
}

func TestSkillsToggleAndSearch(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	dir := a.paths.GlobalSkillsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"alpha.md": "---\ndescription: first skill\n---\n", "beta.md": "second skill\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a.runCommand("skills")
	if len(a.st.Skills) != 2 || !a.st.Skills[0].Enabled {
		t.Fatalf("skills not discovered: %+v", a.st.Skills)
	}
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "alpha") || !strings.Contains(text, "first skill") || !strings.Contains(text, "global") {
		t.Fatalf("skills not shown:\n%s", text)
	}
	a.onKey(key(' '))
	if a.st.Skills[0].Enabled {
		t.Fatal("space did not toggle skill")
	}
	saved, _ := a.store.LoadSkills()
	if saved[0].Enabled || !saved[1].Enabled {
		t.Fatalf("toggle not persisted: %+v", saved)
	}
	typeRunes(a, "zzz-none")
	if n := len(a.skillIdx()); n != 0 {
		t.Fatalf("filter matched %d skills", n)
	}
	a.draw()
	if !strings.Contains(screenText(scr), "nothing here") {
		t.Fatal("empty state missing")
	}
}

func TestSkillsDialogExplainsWhereToPutSkills(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.runCommand("skills")
	a.draw()
	if text := screenText(scr); !strings.Contains(text, ".meno/skills") {
		t.Fatalf("no hint for an empty skills list:\n%s", text)
	}
}

func TestSessionsOverlayListsTheCurrentSession(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openSessions()
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "new session") || !strings.Contains(text, "current") {
		t.Fatalf("sessions overlay not rendered:\n%s", text)
	}
	a.st.Query = []rune("nothing like this")
	a.draw()
	if !strings.Contains(screenText(scr), "no matching sessions") {
		t.Fatal("filter")
	}
}

func TestParseEnv(t *testing.T) {
	m, err := parseEnv("A=1, B=two=2 ,")
	if err != nil || m["A"] != "1" || m["B"] != "two=2" || len(m) != 2 {
		t.Fatalf("got %v %v", m, err)
	}
	if _, err := parseEnv("NOEQ"); err == nil {
		t.Fatal("expected error for missing =")
	}
}

func TestMCPEditRejectsInvalidServer(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	before := len(a.st.Servers)
	a.openMCPEdit(-1)
	a.st.EditMCP.Name = ""
	a.saveMCPEdit()
	if len(a.st.Servers) != before || a.lastNotice() == "" {
		t.Fatalf("invalid server saved: %+v", a.st.Servers)
	}
}

func TestHelpClosesOnQ(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openOverlay(OverHelp)
	a.draw()
	if !strings.Contains(screenText(scr), "modes & navigation") {
		t.Fatal("help rows missing")
	}
	a.onKey(key('q'))
	if a.st.Over != OverNone {
		t.Fatal("help did not close")
	}
}

func TestSkillAutocompleteCompletesOnTab(t *testing.T) {
	a, _ := newTestApp(t)
	defer a.scr.Fini()
	a.st.Skills = []config.Skill{{Name: "rate-limit", Desc: "add rl", Enabled: true}, {Name: "unit-tests", Desc: "tests", Enabled: true}}
	typeRunes(a, "/rat")
	if !a.acActive() || len(a.acMatches()) != 1 {
		t.Fatalf("autocomplete not active for /rat: %v", a.acMatches())
	}
	a.onKey(keyK(tcell.KeyTab))
	if got := string(a.st.Input); got != "/rate-limit " {
		t.Fatalf("completed to %q", got)
	}
}

func TestSkillAutocompleteEscLeavesInsert(t *testing.T) {
	a, _ := newTestApp(t)
	defer a.scr.Fini()
	a.st.Skills = []config.Skill{{Name: "rate-limit", Desc: "x", Enabled: true}}
	typeRunes(a, "/r")
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != ui.ModeNormal || a.acActive() || string(a.st.Input) != "/r" {
		t.Fatalf("esc: mode=%v input=%q", a.st.Mode, string(a.st.Input))
	}
}

func TestVisualModeShowsHintAndSelection(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "hi"})
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(key('v'))
	if a.st.Mode != ui.ModeVisual {
		t.Fatalf("mode %v", a.st.Mode)
	}
	a.draw()
	if !strings.Contains(screenText(scr), "y yank") {
		t.Fatal("visual hint missing")
	}
}

func TestCommandModeShowsListAndLine(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(key(':'))
	typeRunes(a, "mo")
	a.draw()
	out := screenText(scr)
	_, h := scr.Size()
	if !strings.HasPrefix(visualRow(scr, h-1), ":mo") {
		t.Fatalf("command line missing on the last row:\n%s", out)
	}
	if !strings.Contains(out, ":model") {
		t.Fatalf("command list missing:\n%s", out)
	}
}

func TestEmptyComposerShowsCursor(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.draw()
	if !cursorVisible(scr) {
		t.Fatal("cursor missing on empty composer")
	}
}

func TestCommandEnterRunsCommand(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.onKey(keyK(tcell.KeyEsc))
	a.onKey(key(':'))
	typeRunes(a, "help")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Over != OverHelp {
		t.Fatalf("command :help did not open help overlay (over=%v)", a.st.Over)
	}
	_ = scr
}
