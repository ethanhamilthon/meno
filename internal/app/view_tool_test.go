package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
)

func TestViewQueuesImageForNextStep(t *testing.T) {
	var seen []llm.Msg
	cl := &spyClient{scriptedClient: scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: `view("pic.png")`}},
		{{Kind: llm.EvText, Text: "I see it"}, {Kind: llm.EvDone}},
	}}, seen: &seen}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	if err := os.WriteFile(filepath.Join(a.st.Cwd, "pic.png"), testPNG(t, 9, 9), 0o644); err != nil {
		t.Fatal(err)
	}
	typeRunes(a, "look at pic.png")
	a.onKey(keyK(tcell.KeyEnter)) // enter
	pump(t, a, func() bool { return !a.st.Running })

	turns := a.st.Session.Snapshot()
	var view *agent.Turn
	for i := range turns {
		if turns[i].Kind == agent.KindView {
			view = &turns[i]
		}
	}
	if view == nil || len(view.Attachments) != 1 || view.Text != `Image from view("pic.png"): [Image #1]` {
		t.Fatalf("turns: %+v", turns)
	}
	if !strings.Contains(turns[1].Output, "queued image #1") {
		t.Fatalf("js output: %q", turns[1].Output)
	}
	// Second request: user, assistant call, tool result, then the image message.
	var second []llm.Msg
	for i, m := range seen {
		if m.Role == llm.RoleTool {
			second = seen[i:]
			break
		}
	}
	if len(second) < 2 || second[1].Role != llm.RoleUser || len(second[1].Images) != 1 {
		t.Fatalf("request after the call: %+v", seen)
	}
	if _, err := os.Stat(filepath.Join(paths.Home, "attachments", view.Attachments[0].SHA+".png")); err != nil {
		t.Fatal(err)
	}
	a.draw()
	out := screenText(scr)
	if !strings.Contains(out, `Image from view("pic.png")`) {
		t.Fatalf("view turn not shown:\n%s", out)
	}
	if a.st.Queue.Viewed() != 0 {
		t.Fatal("viewed queue not drained")
	}
}

func TestViewErrors(t *testing.T) {
	cl := &scriptedClient{turns: [][]llm.Event{
		{{Kind: llm.EvTool, CallID: "c1", Code: `try { view("none.png") } catch (e) { console.log("E1 " + e.message) }; fs.write("t.txt","x"); try { view("t.txt") } catch (e) { console.log("E2 " + e.message) }`}},
		{{Kind: llm.EvText, Text: "ok"}, {Kind: llm.EvDone}},
	}}
	a, scr, _ := e2eApp(t, cl)
	defer scr.Fini()
	typeRunes(a, "go")
	a.onKey(keyK(tcell.KeyEnter))
	pump(t, a, func() bool { return !a.st.Running })
	out := a.st.Session.Snapshot()[1].Output
	if !strings.Contains(out, "E1 ") || !strings.Contains(out, "E2 ") || !strings.Contains(out, "not an image") {
		t.Fatalf("output: %q", out)
	}
	for _, tr := range a.st.Session.Snapshot() {
		if tr.Kind == agent.KindView {
			t.Fatal("failed view queued an image")
		}
	}
}
