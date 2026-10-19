package app

import (
	"strings"
	"testing"

	"meno/internal/llm"
	"meno/internal/objtag"
)

const linkTag = "<file-link>/tmp/x.go</file-link>"

// sayWithFile types "#goal read <file>" as the composer holds it: the mode and
// the file are input objects.
func sayWithFile(t *testing.T, a *App) {
	t.Helper()
	mode, ok1 := objectRune(objtag.Object{Kind: objtag.Mode, Ref: "goal"})
	file, ok2 := objectRune(objtag.Object{Kind: objtag.File, Ref: "/tmp/x.go"})
	if !ok1 || !ok2 {
		t.Fatal("no object runes")
	}
	a.setInput([]rune{mode, ' ', 'r', 'e', 'a', 'd', ' ', file})
}

func TestMessageModesKeepsFileAndFolderLinks(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	text := "<mode>goal</mode> r " + linkTag + " <folder-link>/tmp</folder-link>"
	names, rest, tagged := a.messageModes(text)
	if !tagged || len(names) != 1 || names[0] != "goal" {
		t.Fatalf("names = %v, tagged = %v", names, tagged)
	}
	if rest != "r "+linkTag+" <folder-link>/tmp</folder-link>" {
		t.Fatalf("rest = %q", rest)
	}
	if _, rest, tagged = a.messageModes("<mode>goal</mode>"); !tagged || rest != "" {
		t.Fatalf("a message of only modes: rest = %q, tagged = %v", rest, tagged)
	}
}

func TestStartedTurnKeepsTheFileLinkNextToAMode(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "g1", Code: "goal.done('ok')"}},
		{{Kind: llm.EvText, Text: "done"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	sayWithFile(t, a)
	a.submit()
	pump(t, a, func() bool { return !a.st.Running && cl.left() == 0 })
	if got := a.st.Session.ToMsgs()[0].Text; !strings.Contains(got, linkTag) {
		t.Fatalf("the file link is lost from the sent message:\n%s", got)
	}
	if len(a.st.Session.Modes) != 1 || a.st.Session.Modes[0] != "goal" {
		t.Fatalf("modes: %v", a.st.Session.Modes)
	}
}

func TestQueuedMessageKeepsTheFileLinkNextToAMode(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.st.Running = true
	sayWithFile(t, a)
	a.submit()
	items := a.st.Queue.Items()
	if len(items) != 1 || !strings.Contains(items[0].Text, linkTag) || strings.Contains(items[0].Text, "<mode>") {
		t.Fatalf("queued: %+v", items)
	}
	if len(a.st.Session.Modes) != 1 {
		t.Fatalf("modes: %v", a.st.Session.Modes)
	}
}
