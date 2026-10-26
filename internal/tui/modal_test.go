package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/config"
	"meno/internal/llm"
	"meno/internal/render"
	"meno/internal/version"
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
	for _, want := range []string{"thin and efficient AI coding harness", "@", "skills", "/help", version.Label(), "gpt-5.5"} {
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
	a.running.Wait()
	a.reloadSkills()
	a.running.Wait()
	pump(t, a, func() bool { return len(a.st.Skills) == 2 })
	if len(a.st.Skills) != 2 || !a.st.Skills[0].Enabled {
		t.Fatalf("skills not discovered: %+v", a.st.Skills)
	}
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "alpha") || !strings.Contains(text, "first skill") || !strings.Contains(text, "global") {
		t.Fatalf("skills not shown:\n%s", text)
	}
	a.onKey(tcell.NewEventKey(tcell.KeyTab, 0, 0))
	if !a.st.Skills[0].Auto {
		t.Fatal("tab did not mark the skill auto-load")
	}
	a.onKey(key(' '))
	if a.st.Skills[0].Enabled {
		t.Fatal("space did not toggle skill")
	}
	saved, lerr := a.storeForTest().LoadSkills()
	if lerr != nil || len(saved) != 2 {
		t.Fatalf("load: %v %v", saved, lerr)
	}
	if saved[0].Enabled || !saved[0].Auto || !saved[1].Enabled || saved[1].Auto {
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
	cl := &scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}}}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	a.openSessions()
	a.draw()
	if text := screenText(scr); strings.Contains(text, "new session") || strings.Contains(text, "current") {
		t.Fatalf("a session without a message is listed:\n%s", text)
	}
	a.closeOverlay()
	say(t, a, "hello overlay")
	a.openSessions()
	a.draw()
	if text := screenText(scr); !strings.Contains(text, "hello overlay") || !strings.Contains(text, "current") {
		t.Fatalf("sessions overlay not rendered:\n%s", text)
	}
	a.st.Query.Set("nothing like this")
	a.draw()
	if !strings.Contains(screenText(scr), "no matching sessions") {
		t.Fatal("filter")
	}
}

func TestMCPEditRejectsInvalidServer(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	before := len(a.st.Servers)
	a.openMCPEdit(-1)
	a.st.EditMCP.Name = ""
	a.saveMCPEdit()
	if len(a.st.Servers) != before || a.st.FormNote == "" {
		t.Fatalf("invalid server saved: %+v", a.st.Servers)
	}
}

func TestHelpClosesOnQ(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.openOverlay(OverHelp)
	a.draw()
	if !strings.Contains(screenText(scr), "keys") {
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
	typeRunes(a, "/skill:rat")
	if !a.acActive() || len(a.acMatches()) != 1 {
		t.Fatalf("autocomplete not active for /skill:rat: %v", a.acMatches())
	}
	a.onKey(keyK(tcell.KeyTab))
	if got := labelText(a.st.Input); got != "/skill:rate-limit " {
		t.Fatalf("completed to %q", got)
	}
}

func TestSkillAutocompleteEscLeavesInsert(t *testing.T) {
	a, _ := newTestApp(t)
	defer a.scr.Fini()
	a.st.Skills = []config.Skill{{Name: "rate-limit", Desc: "x", Enabled: true}}
	typeRunes(a, "/skill:r")
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Mode != render.ModeInput || a.acActive() || string(a.st.Input) != "/skill:r" {
		t.Fatalf("esc: mode=%v input=%q", a.st.Mode, string(a.st.Input))
	}
}

func TestInlineCommandShowsList(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	typeRunes(a, "fix /mo")
	a.draw()
	out := screenText(scr)
	_, h := scr.Size()
	if !strings.Contains(out, "fix /mo") || !strings.Contains(visualRow(scr, h-1), a.ctxLabel()) {
		t.Fatalf("draft missing:\n%s", out)
	}
	if !strings.Contains(out, "/model") {
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
	typeRunes(a, "/help")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.Over != OverHelp {
		t.Fatalf("command /help did not open help overlay (over=%v)", a.st.Over)
	}
	_ = scr
}
