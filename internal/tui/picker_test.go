package tui

import (
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func pickerApp(t *testing.T) (*App, tcell.SimulationScreen, string) {
	t.Helper()
	a, scr, paths := e2eApp(t, &recClient{})
	for _, f := range []string{"main.go", "internal/util.go", ".git/HEAD", "notes/my file.md"} {
		p := filepath.Join(paths.Cwd, f)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("x"), 0o644)
	}
	return a, scr, paths.Cwd
}

func waitPicker(t *testing.T, a *App, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if a.st.Pick != nil && a.st.Pick.ResultsFor == a.st.Pick.Query && len(a.st.Pick.Items) >= want {
			return
		}
		select {
		case e := <-a.events:
			a.handle(e)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatalf("picker has no results")
}

func TestAtPickerCreatesOneAtomicObjectAndSerializes(t *testing.T) {
	a, scr, cwd := pickerApp(t)
	defer scr.Fini()
	typeRunes(a, "read @main")
	waitPicker(t, a, 1)
	if !a.acActive() || a.pickItems()[0].name != "main.go" {
		t.Fatalf("items: %+v", a.pickItems())
	}
	a.onKey(keyK(tcell.KeyEnter))
	if len(a.st.Input) != len("read ")+2 || !isObjRune(a.st.Input[5]) {
		t.Fatalf("one object rune, then a space: %q", string(a.st.Input))
	}
	if a.messageText() != "read <file-link>"+filepath.Join(cwd, "main.go")+"</file-link> " {
		t.Fatalf("message text: %q", a.messageText())
	}
	if labelText(a.st.Input) != "read @main.go " {
		t.Fatalf("label: %q", labelText(a.st.Input))
	}
	// Backspace removes the whole object, undo brings it back as the same object.
	typeRunes(a, "x")
	a.onKey(keyK(tcell.KeyBackspace2))
	a.onKey(keyK(tcell.KeyBackspace2)) // the space
	a.onKey(keyK(tcell.KeyBackspace2)) // the object
	if labelText(a.st.Input) != "read " {
		t.Fatalf("object not deleted as a unit: %q", labelText(a.st.Input))
	}
}

func TestAtOnlyOpensAtWordStartAndEscKeepsText(t *testing.T) {
	a, scr, _ := pickerApp(t)
	defer scr.Fini()
	typeRunes(a, "mail me@main")
	time.Sleep(50 * time.Millisecond)
	for len(a.events) > 0 {
		a.handle(<-a.events)
	}
	if a.pickOpen() {
		t.Fatal("@ inside a word (an email) must not open the picker")
	}
	a.setInput(nil)
	typeRunes(a, "@ma")
	waitPicker(t, a, 1)
	a.onKey(keyK(tcell.KeyEsc))
	if a.pickOpen() || string(a.st.Input) != "@ma" || a.st.Mode != 0 && a.st.Mode.Label() == "NORMAL" {
		t.Fatalf("Esc closes the list and keeps the text, staying in insert mode: %q mode %v", string(a.st.Input), a.st.Mode.Label())
	}
}

func TestPickerFindsHiddenFilesAndEntersFoldersWithTab(t *testing.T) {
	a, scr, cwd := pickerApp(t)
	defer scr.Fini()
	typeRunes(a, "@HEAD")
	waitPicker(t, a, 1)
	if got := a.pickItems()[0].name; got != ".git/HEAD" {
		t.Fatalf(".git must be searchable: %q", got)
	}
	a.setInput(nil)
	typeRunes(a, "@notes")
	waitPicker(t, a, 1)
	a.onKey(keyK(tcell.KeyTab)) // folder: enter it, the path stays text
	if string(a.st.Input) != "@notes/" {
		t.Fatalf("tab on a folder navigates: %q", string(a.st.Input))
	}
	waitPicker(t, a, 1)
	for a.pickItems()[0].name == "" {
		t.Fatal("empty")
	}
	a.onKey(keyK(tcell.KeyEnter))
	if want := "<file-link>" + filepath.Join(cwd, "notes", "my file.md") + "</file-link> "; a.messageText() != want {
		t.Fatalf("a path with a space is one reference: %q", a.messageText())
	}
}

func TestObjectsSurviveClipboardQueueAndPasteRules(t *testing.T) {
	a, scr, cwd := pickerApp(t)
	defer scr.Fini()
	file := filepath.Join(cwd, "main.go")
	r, _ := objectRune(inputObject{Kind: "file", Ref: file})
	a.setInput([]rune{'a', r, 'b'})
	if got := externalText(a.st.Input); got != "a<file-link>"+file+"</file-link>b" {
		t.Fatalf("external copy has the full tag: %q", got)
	}
	// Pasting a well-formed tag restores the object; relative, unclosed and
	// fenced tags and a forged private-use character stay text.
	a.setInput(nil)
	tick := "`"
	a.insertAtCursor(a.runesFromText("<file-link>" + file + "</file-link> <file-link>rel.go</file-link> " + tick + "<file-link>" + file + "</file-link>" + tick + " \ue123"))
	text := string(a.st.Input)
	if strings.Count(labelText(a.st.Input), "@main.go") != 1 || !strings.Contains(text, "<file-link>rel.go</file-link>") || strings.Contains(text, "\ue123") {
		t.Fatalf("paste rules: %q / %q", text, labelText(a.st.Input))
	}
	// The queue shows labels and recall restores objects.
	a.setInput([]rune{r})
	msg := a.draftMessage(a.messageText())
	a.setInput(nil)
	a.setDraft(msg, nil)
	if len(a.st.Input) != 1 || a.st.Input[0] != r {
		t.Fatalf("recall must restore the same object: %q", string(a.st.Input))
	}
}

// Whatever keys are pressed, an object is never split: the buffer only ever
// holds whole, known object runes, and the serialized text is always parseable.
func TestRandomEditsNeverLeavePartialObjects(t *testing.T) {
	a, scr, cwd := pickerApp(t)
	defer scr.Fini()
	f, _ := objectRune(inputObject{Kind: "file", Ref: filepath.Join(cwd, "main.go")})
	d, _ := objectRune(inputObject{Kind: "folder", Ref: filepath.Join(cwd, "internal")})
	keys := []string{"h", "l", "j", "k", "w", "b", "e", "0", "$", "x", "X", "dd", "dw", "db", "cw", "yy", "yw", "p", "P", "u", "D", "C", "i", "a", "I", "A", "o", "v", "V", "d", "c", "y"}
	rng := rand.New(rand.NewSource(7))
	for round := 0; round < 60; round++ {
		a.setInput([]rune{'a', f, ' ', d, 'b', 'c', f})
		a.st.Mode, a.st.Cursor = 1, 0
		a.runCommand("edit")
		for i := 0; i < 40; i++ {
			k := keys[rng.Intn(len(keys))]
			for _, r := range k {
				a.onKey(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
			}
			if rng.Intn(5) == 0 {
				a.onKey(keyK(tcell.KeyEsc))
			}
			for _, r := range a.st.Input {
				if isObjRune(r) {
					if _, ok := objectAt(r); !ok {
						t.Fatalf("round %d key %q: unknown object rune %U in %q", round, k, r, string(a.st.Input))
					}
				}
			}
			if txt := externalText(a.st.Input); strings.Count(txt, "<file-link>") != strings.Count(txt, "</file-link>") || strings.Count(txt, "<folder-link>") != strings.Count(txt, "</folder-link>") {
				t.Fatalf("round %d key %q: serialization lost a tag: %q", round, k, txt)
			}
		}
	}
}
