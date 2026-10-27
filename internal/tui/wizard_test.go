package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// answer types text into the input row of the wizard and presses Enter.
func answer(a *App, text string) {
	a.st.Edit.Set("")
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
}

func TestProviderWizardAsksOneFieldAtATime(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	before := len(a.st.Providers)
	a.startProviderWizard(llm.Provider{Protocol: llm.ProtoChat})
	if a.st.Over != OverWizard || a.wizField().label != "name" {
		t.Fatalf("wizard did not start on name: over=%v", a.st.Over)
	}
	answer(a, "fresh")
	answer(a, "http://127.0.0.1:1/v1")
	if len(a.st.Providers) != before {
		t.Fatal("saved before the last step")
	}
	answer(a, "secret")
	if a.wizField().label != "run" || !a.st.Edit.Empty() {
		t.Fatalf("expected an empty run step, got %q", a.wizField().label)
	}
	a.onKey(keyK(tcell.KeyEnter)) // skip run
	if len(a.st.Providers) != before+1 || a.st.Providers[before].APIKey != "secret" {
		t.Fatalf("provider not saved: %+v", a.st.Providers)
	}
	if a.st.Over != OverLogin {
		t.Fatalf("expected the login list, got %v", a.st.Over)
	}
}

func TestProviderWizardKeepsTheDefaultAndChecksTheName(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	draft := a.newAPIDraft(llm.ProtoResponses)
	a.startProviderWizard(draft)
	if a.st.Edit.String() != draft.Name {
		t.Fatalf("the default name must be in the input: %q", a.st.Edit.String())
	}
	answer(a, "")
	if a.wizField().label != "name" {
		t.Fatal("an empty name must stay on the step")
	}
	a.st.Providers = append(a.st.Providers, llm.Provider{Name: "taken"})
	answer(a, "taken")
	if a.wizField().label != "name" {
		t.Fatal("a duplicate name must stay on the step")
	}
}

func TestProviderWizardEscSavesNothing(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	before := len(a.st.Providers)
	a.startProviderWizard(llm.Provider{Protocol: llm.ProtoChat})
	answer(a, "fresh")
	answer(a, "http://127.0.0.1:1/v1")
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverLogin || len(a.st.Providers) != before {
		t.Fatalf("esc must cancel: over=%v n=%d", a.st.Over, len(a.st.Providers))
	}
}

func TestMCPWizardStepsFollowTheTransport(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	a.startMCPWizard()
	answer(a, "tools")
	if a.wizField().label != "transport" {
		t.Fatalf("expected transport, got %q", a.wizField().label)
	}
	a.onKey(keyK(tcell.KeyRight)) // stdio -> sse
	if got := a.wizSteps(); len(got) != 3 || got[2] != "url" {
		t.Fatalf("a remote server asks for a url: %v", got)
	}
	a.onKey(keyK(tcell.KeyLeft)) // back to stdio
	a.onKey(keyK(tcell.KeyEnter))
	if a.wizField().label != "command" {
		t.Fatalf("stdio asks for a command, got %q", a.wizField().label)
	}
	answer(a, "")
	if a.wizField().label != "args" {
		t.Fatal("an empty command still moves on; the form says what is missing")
	}
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Over != OverMCP {
		t.Fatalf("esc must return to the mcp list, got %v", a.st.Over)
	}
}

func TestWizardIsDrawn(t *testing.T) {
	a, _ := visualFixture(t, 90, 30)
	a.startProviderWizard(llm.Provider{Protocol: llm.ProtoChat})
	answer(a, "fresh")
	a.draw()
}
