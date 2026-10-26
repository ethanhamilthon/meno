package tui

import (
	"fmt"
	"meno/internal/api"
	"meno/internal/render"
	"reflect"
	"strings"
	"testing"
)

func TestLineCacheReplacementsAndCompletePalette(t *testing.T) {
	defer render.ApplyPalette(render.DefaultPalette())
	var c lineCache
	base := render.Turn{ID: "same", Role: render.RoleAssistant, Text: "old", Thinking: "one", Code: "true", Title: "old", Output: "one", Err: "bad", Run: &render.RunStats{Turns: 1}}
	variants := []render.Turn{base}
	for _, field := range []string{"text", "think", "code", "title", "output", "error", "run"} {
		v := base
		switch field {
		case "text":
			v.Text = "new"
		case "think":
			v.Thinking = "two"
		case "code":
			v.Code = "echo"
		case "title":
			v.Title = "new"
		case "output":
			v.Output = "two"
		case "error":
			v.Err = "err"
		case "run":
			v.Run = &render.RunStats{Turns: 2}
		}
		variants = append(variants, v)
	}
	for _, v := range variants {
		got := c.turnLines(v, 80, render.JSOpts{})
		want := render.BuildLines([]render.Turn{v}, 80, render.JSOpts{})
		if !reflect.DeepEqual(got, want) {
			t.Fatal("stale turn layout")
		}
	}
	p := render.CurrentPalette()
	p.FgDim = render.Red
	render.ApplyPalette(p)
	if !reflect.DeepEqual(c.turnLines(base, 80, render.JSOpts{}), render.BuildLines([]render.Turn{base}, 80, render.JSOpts{})) {
		t.Fatal("stale palette")
	}
}

func TestTranscriptCacheWarmAppendResetAndNotices(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	for i := 0; i < 6100; i++ {
		a.st.Session.upsert(api.Turn{ID: fmt.Sprint(i + 1), Role: "assistant", Content: "hello"})
	}
	first := a.transcriptWithSelection(96)
	version := a.st.transcript.version
	a.setInput([]rune("typing"))
	second := a.transcriptWithSelection(96)
	if &first[0] != &second[0] || a.st.transcript.version != version {
		t.Fatal("composer rebuilt history")
	}
	if allocs := testing.AllocsPerRun(10, func() { a.transcriptWithSelection(96) }); allocs != 0 {
		t.Fatalf("warm allocations %v", allocs)
	}
	a.st.Session.upsert(api.Turn{ID: "6101", Role: "user", Content: "next"})
	a.transcriptWithSelection(96)
	if len(a.st.lines.m) != 6101 {
		t.Fatal("cache cliff")
	}
	a.st.Session.upsert(api.Turn{ID: "1", Role: "assistant", Content: "world"})
	text := func() string {
		var b strings.Builder
		for _, l := range a.transcriptWithSelection(96) {
			b.WriteString(l.Text())
		}
		return b.String()
	}
	if !strings.Contains(text(), "world") {
		t.Fatal("same length replacement missed")
	}
	a.notify("notice")
	if !strings.Contains(text(), "notice") {
		t.Fatal("notice not invalidated")
	}
	a.st.Session.resetTurns([]api.Turn{{ID: "z", Role: "user", Content: "reset"}})
	if out := text(); strings.Contains(out, "world") || !strings.Contains(out, "reset") {
		t.Fatal("reset retained history")
	}
}

func TestStaticLiveLayoutAndDocumentAreReused(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.LiveText = strings.Repeat("**bold** and `code`\n\n", 50)
	a.st.LiveThink = "reasoning"
	first := a.transcriptWithSelection(96)
	stamp := a.st.transcript.stamp
	doc := a.logText()
	a.transcriptWithSelection(96)
	if a.st.transcript.stamp != stamp {
		t.Fatal("unchanged live response rebuilt")
	}
	if len(first) == 0 || &a.logText()[0][0] != &doc[0][0] {
		t.Fatal("document was rebuilt")
	}
	a.st.LiveText += "changed"
	a.transcriptWithSelection(96)
	if a.st.transcript.stamp == stamp {
		t.Fatal("live delta did not invalidate layout")
	}
}

func TestCompactStatusInvalidatesDocument(t *testing.T) {
	a, s := newTestApp(t)
	defer s.Fini()
	a.st.Settings.View = "compact"
	a.st.Running = true
	a.st.LiveThink = "thinking"
	a.logText()
	stamp := a.st.transcript.stamp
	a.st.LiveThink = ""
	a.transcriptWithSelection(96)
	if a.st.transcript.stamp == stamp {
		t.Fatal("compact status changed without invalidating document")
	}
}
