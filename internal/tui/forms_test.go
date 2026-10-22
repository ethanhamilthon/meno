package tui

import (
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/llm"
)

// fillField opens field i of the open form, types text and presses Enter.
func fillField(a *App, i int, text string) {
	a.st.Field = i
	a.onKey(keyK(tcell.KeyEnter))
	a.st.Edit.Set("")
	typeRunes(a, text)
	a.onKey(keyK(tcell.KeyEnter))
}

func openNewProvider(a *App) {
	a.st.EditProv = llm.Provider{Protocol: llm.ProtoChat}
	a.st.EditIdx = -1
	a.openOverlay(OverProviderEdit)
}

func TestProviderFormSavesWithoutSaveKey(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	before := len(a.st.Providers)
	openNewProvider(a)
	fillField(a, 0, "fresh")
	if len(a.st.Providers) != before || a.st.FormNote == "" {
		t.Fatalf("an incomplete provider must wait and say why: %q", a.st.FormNote)
	}
	fillField(a, 1, "http://127.0.0.1:1/v1")
	if len(a.st.Providers) != before+1 || a.st.FormNote != "" || a.st.EditIdx != before {
		t.Fatalf("provider not saved: n=%d note=%q idx=%d", len(a.st.Providers), a.st.FormNote, a.st.EditIdx)
	}
	fillField(a, 2, "secret")
	if len(a.st.Providers) != before+1 || a.st.Providers[before].APIKey != "secret" {
		t.Fatalf("a later change must be saved at once: %+v", a.st.Providers)
	}
	loaded, err := a.storeForTest().LoadProviders()
	if err != nil || len(loaded) != before+1 || loaded[before].APIKey == "" { // the daemon keeps the key and only says it has one
		t.Fatalf("not persisted: %v %+v", err, loaded)
	}
}

func TestProviderFormEscDropsTheField(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	openNewProvider(a)
	a.st.Field = 0
	a.onKey(keyK(tcell.KeyEnter))
	typeRunes(a, "abc")
	a.onKey(keyK(tcell.KeyEsc))
	if a.st.Editing || a.st.EditProv.Name != "" || a.st.Over != OverProviderEdit {
		t.Fatalf("esc must end the edit and keep the form: editing=%v name=%q", a.st.Editing, a.st.EditProv.Name)
	}
}

func TestFormFieldEditsAtTheCursor(t *testing.T) {
	a, scr := newTestApp(t)
	defer scr.Fini()
	openNewProvider(a)
	a.st.EditProv.Name = "ac"
	a.st.Field = 0
	a.onKey(keyK(tcell.KeyEnter))
	a.onKey(keyK(tcell.KeyLeft))
	typeRunes(a, "b")
	a.onKey(keyK(tcell.KeyEnter))
	if a.st.EditProv.Name != "abc" {
		t.Fatalf("got %q", a.st.EditProv.Name)
	}
}

func TestSecretFieldDrawsBullets(t *testing.T) {
	e := textEdit{Secret: true}
	e.Set("key")
	if string(e.shown()) != "•••" || e.String() != "key" {
		t.Fatalf("shown %q text %q", string(e.shown()), e.String())
	}
}
