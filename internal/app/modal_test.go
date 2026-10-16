package app

import (
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
	for _, want := range []string{":model", "#prompt", "███"} {
		if !strings.Contains(out, want) {
			t.Fatalf("welcome missing %q:\n%s", want, out)
		}
	}
}

func TestSkillsToggleAndSearch(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.st.Skills = config.DefaultSkills()
	a.openOverlay(OverSkills)
	before := a.st.Skills[0].Enabled
	a.onKey(key(' '))
	if a.st.Skills[0].Enabled == before {
		t.Fatal("space did not toggle skill")
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

func TestResumeOverlayNoSessions(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openResume()
	a.draw()
	if !strings.Contains(screenText(scr), "nothing here") {
		t.Fatal("resume overlay not rendered")
	}
}

func TestPromptSaveRejectsDuplicateAndBadName(t *testing.T) {
	a, _ := newTestApp(t)
	defer a.scr.Fini()
	a.st.Prompts = []config.Prompt{{Name: "rate-limit", Scope: "project", Template: "x"}}
	a.openPromptEdit(-1)
	a.st.EditPrompt.Name = "rate-limit"
	a.st.EditPrompt.Template = "y"
	a.savePromptEdit()
	if len(a.st.Prompts) != 1 || a.st.Toast == "" {
		t.Fatalf("duplicate accepted: %+v toast=%q", a.st.Prompts, a.st.Toast)
	}
	a.st.EditPrompt.Name = "Bad Name!"
	a.savePromptEdit()
	if len(a.st.Prompts) != 1 {
		t.Fatal("invalid name accepted")
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
	if len(a.st.Servers) != before || a.st.Toast == "" {
		t.Fatalf("invalid server saved: %+v", a.st.Servers)
	}
}

func TestHelpClosesOnQ(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openOverlay(OverHelp)
	a.draw()
	if !strings.Contains(screenText(scr), ":compact") {
		t.Fatal("help rows missing")
	}
	a.onKey(key('q'))
	if a.st.Over != OverNone {
		t.Fatal("help did not close")
	}
}

func TestPromptAutocompleteCompletesOnEnter(t *testing.T) {
	a, _ := newTestApp(t)
	defer a.scr.Fini()
	a.st.Prompts = []config.Prompt{
		{Name: "rate-limit", Scope: "project", Template: "add rl"},
		{Name: "unit-tests", Scope: "global", Template: "tests"},
	}
	typeRunes(a, "#rat")
	if !a.acActive() || len(a.acMatches()) != 1 {
		t.Fatalf("autocomplete not active for #rat: %v", a.acMatches())
	}
	a.onKey(keyK(tcell.KeyTab))
	if got := string(a.st.Input); got != "#rate-limit " {
		t.Fatalf("completed to %q", got)
	}
}

func TestPromptAutocompleteEscLeavesInsert(t *testing.T) {
	a, _ := newTestApp(t)
	defer a.scr.Fini()
	a.st.Prompts = []config.Prompt{{Name: "rate-limit", Scope: "project", Template: "x"}}
	typeRunes(a, "#r")
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != ui.ModeNormal || a.acActive() || string(a.st.Input) != "#r" {
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
	a.onKey(key(':'))
	typeRunes(a, "mo")
	a.draw()
	out := screenText(scr)
	if !strings.Contains(out, ":mo█") {
		t.Fatalf("command line missing:\n%s", out)
	}
	if !strings.Contains(out, ":model") {
		t.Fatalf("command list missing:\n%s", out)
	}
}

func TestEmptyComposerShowsCursor(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.draw()
	if !strings.Contains(screenText(scr), "▏") {
		t.Fatal("cursor missing on empty composer")
	}
}

func TestCommandEnterRunsCommand(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.onKey(key(':'))
	typeRunes(a, "help")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Over != OverHelp {
		t.Fatalf("command :help did not open help overlay (over=%v)", a.st.Over)
	}
	_ = scr
}
