package app

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/attach"
	"meno/internal/llm"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// waitImage pumps events until the background image load has been applied.
func waitImage(t *testing.T, a *App) {
	t.Helper()
	pump(t, a, func() bool { return len(a.st.Images) > 0 || len(a.currentNotices()) > 0 })
}

func TestCtrlVAttachesClipboardImage(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.clipboard = func() ([]byte, error) { return testPNG(t, 8, 8), nil }
	typeRunes(a, "look ")
	a.onKey(keyK(tcell.KeyCtrlV))
	waitImage(t, a)
	if string(a.st.Input) != "look [Image #1] " || len(a.st.Images) != 1 || a.st.Images[0].W != 8 {
		t.Fatalf("input=%q images=%+v", string(a.st.Input), a.st.Images)
	}
	a.draw()
	if !strings.Contains(screenText(scr), "[Image #1]") {
		t.Fatal("token not drawn")
	}
}

func TestClipboardWithoutImageShowsNotice(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.clipboard = func() ([]byte, error) { return nil, attach.ErrNoImage }
	a.onKey(keyK(tcell.KeyCtrlV))
	waitImage(t, a)
	if len(a.st.Images) != 0 || len(a.st.Input) != 0 {
		t.Fatal("something was attached")
	}
	if n := a.currentNotices(); len(n) == 0 || !strings.Contains(n[len(n)-1].Text, "no image") {
		t.Fatalf("notices: %+v", n)
	}
}

func TestTokenIsDeletedAndSkippedAsOne(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.clipboard = func() ([]byte, error) { return testPNG(t, 4, 4), nil }
	a.onKey(keyK(tcell.KeyCtrlV))
	waitImage(t, a)
	typeRunes(a, "x")
	// "[Image #1] x": cursor at the end. Walk left over "x", " " and the token.
	a.onKey(keyK(tcell.KeyLeft))
	a.onKey(keyK(tcell.KeyLeft))
	if a.st.Cursor != len([]rune("[Image #1]")) {
		t.Fatalf("cursor %d", a.st.Cursor)
	}
	a.onKey(keyK(tcell.KeyLeft))
	if a.st.Cursor != 0 {
		t.Fatalf("cursor should jump over the token, got %d", a.st.Cursor)
	}
	a.onKey(keyK(tcell.KeyRight))
	if a.st.Cursor != len([]rune("[Image #1]")) {
		t.Fatalf("cursor %d", a.st.Cursor)
	}
	a.onKey(keyK(tcell.KeyBackspace))
	if string(a.st.Input) != " x" || len(a.st.Images) != 0 {
		t.Fatalf("input=%q images=%d", string(a.st.Input), len(a.st.Images))
	}
}

func TestSubmitSendsImageAndStoresFile(t *testing.T) {
	var seen []llm.Msg
	cl := &spyClient{scriptedClient: scriptedClient{turns: [][]llm.Event{{{Kind: llm.EvText, Text: "nice"}, {Kind: llm.EvDone}}}}, seen: &seen}
	a, scr, paths := e2eApp(t, cl)
	defer scr.Fini()
	a.clipboard = func() ([]byte, error) { return testPNG(t, 6, 5), nil }
	typeRunes(a, "what is ")
	a.onKey(keyK(tcell.KeyCtrlV))
	waitImage(t, a)
	a.onKey(keyK(tcell.KeyEnter))
	if len(a.st.Images) != 0 || len(a.st.Input) != 0 {
		t.Fatal("draft not cleared")
	}
	pump(t, a, func() bool { return runDone(a) })
	turns := a.st.Session.Snapshot()
	if len(turns[0].Attachments) != 1 || turns[0].Text != "what is [Image #1]" {
		t.Fatalf("turn: %+v", turns[0])
	}
	file := filepath.Join(paths.Home, "attachments", turns[0].Attachments[0].SHA+".png")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("image not stored: %v", err)
	}
	if len(seen) == 0 || len(seen[0].Images) != 1 || seen[0].Images[0].Path != file {
		t.Fatalf("request: %+v", seen)
	}
}

type spyClient struct {
	scriptedClient
	seen *[]llm.Msg
}

func (c *spyClient) Stream(ctx context.Context, req llm.Request, emit func(llm.Event)) error {
	*c.seen = append(*c.seen, req.Msgs...)
	return c.scriptedClient.Stream(ctx, req, emit)
}

func sendPaste(a *App, text string) {
	a.handle(tcell.NewEventPaste(true))
	for _, r := range text {
		if r == '\n' {
			a.handle(keyK(tcell.KeyEnter))
			continue
		}
		a.handle(tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone))
	}
	a.handle(tcell.NewEventPaste(false))
}

func TestPastedTextDoesNotSubmit(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	sendPaste(a, "line one\nline two")
	if a.st.Running || string(a.st.Input) != "line one\nline two" {
		t.Fatalf("running=%v input=%q", a.st.Running, string(a.st.Input))
	}
}

func TestPastedImagePathBecomesAttachment(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	p := filepath.Join(t.TempDir(), "my shot.png")
	if err := os.WriteFile(p, testPNG(t, 5, 5), 0o644); err != nil {
		t.Fatal(err)
	}
	sendPaste(a, strings.ReplaceAll(p, " ", "\\ "))
	waitImage(t, a)
	if string(a.st.Input) != "[Image #1] " || len(a.st.Images) != 1 {
		t.Fatalf("input=%q", string(a.st.Input))
	}
}

func TestEmptyPasteLooksAtClipboard(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	a.clipboard = func() ([]byte, error) { return testPNG(t, 3, 3), nil }
	sendPaste(a, "")
	waitImage(t, a)
	if len(a.st.Images) != 1 {
		t.Fatal("clipboard image not attached")
	}
}

func TestAtPickerAttachesImageAndNormalModeP(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	p := filepath.Join(a.st.Cwd, "pic.png")
	if err := os.WriteFile(p, testPNG(t, 5, 5), 0o644); err != nil {
		t.Fatal(err)
	}
	typeRunes(a, "@pic.png")
	pump(t, a, func() bool { return a.st.Pick != nil && a.st.Pick.ResultsFor == "pic.png" && len(a.st.Pick.Items) > 0 })
	a.onKey(keyK(tcell.KeyEnter))
	waitImage(t, a)
	if len(a.st.Images) != 1 || string(a.st.Input) != "[Image #1] " {
		t.Fatalf("input=%q", string(a.st.Input))
	}
	// Normal-mode p with an empty register takes the clipboard image.
	a.clipboard = func() ([]byte, error) { return testPNG(t, 7, 7), nil }
	a.onKey(keyK(tcell.KeyEsc))
	typeRunes(a, "p")
	pump(t, a, func() bool { return len(a.st.Images) == 2 })
	if !strings.Contains(string(a.st.Input), "[Image #2]") {
		t.Fatalf("input=%q", string(a.st.Input))
	}
}

func TestModelWithoutVisionRejectsImages(t *testing.T) {
	a, scr, _ := e2eApp(t, &scriptedClient{})
	defer scr.Fini()
	no := false
	a.st.Pricing = map[string]llm.ModelInfo{"custom:openai/gpt-5.5": {Vision: &no}}
	a.clipboard = func() ([]byte, error) { return testPNG(t, 3, 3), nil }
	a.onKey(keyK(tcell.KeyCtrlV))
	waitImage(t, a)
	if len(a.st.Images) != 0 || len(a.st.Input) != 0 {
		t.Fatal("image attached to a model without vision")
	}
	if n := a.currentNotices(); len(n) == 0 || !strings.Contains(n[len(n)-1].Text, "does not accept images") {
		t.Fatalf("notices %+v", n)
	}
}
