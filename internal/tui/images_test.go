package tui

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
	"meno/internal/llm"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }
func (s *syncBuf) Reset()                      { s.mu.Lock(); defer s.mu.Unlock(); s.b.Reset() }

func imageApp(t *testing.T, proto string) (*App, *syncBuf) {
	t.Helper()
	a, scr, _ := e2eApp(t, &recClient{})
	t.Cleanup(scr.Fini)
	scr.SetSize(100, 40)
	sink := &syncBuf{}
	a.imgs.out = sink
	a.st.Settings.ImagesProtocol = proto
	a.st.Settings.View = "full"
	img := image.NewNRGBA(image.Rect(0, 0, 160, 160))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	dir := a.attachDir()
	os.MkdirAll(dir, 0o755)
	f, _ := os.Create(filepath.Join(dir, "sha1.png"))
	png.Encode(f, img)
	f.Close()
	a.st.Session.Append(agent.Turn{Role: llm.RoleUser, Text: "look [Image #1]", Attachments: []agent.Attachment{{N: 1, Mime: "image/png", SHA: "sha1", W: 160, H: 160}}})
	return a, sink
}

func drawUntil(t *testing.T, a *App, sink *syncBuf, cond func(string) bool) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		a.draw()
		if s := sink.String(); cond(s) {
			return s
		}
		select {
		case e := <-a.events:
			a.handle(e)
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatalf("graphics output never matched; got %q", sink.String())
	return ""
}

func TestKittyImageIsPlacedOnceAfterAsyncDecodeAndClearedOnScrollAway(t *testing.T) {
	a, sink := imageApp(t, "kitty")
	got := drawUntil(t, a, sink, func(s string) bool { return strings.Contains(s, "a=T") })
	if strings.Count(got, "a=T") != 1 || !strings.Contains(got, "\x1b7\x1b[") || !strings.Contains(got, "c=20,r=10") {
		t.Fatalf("one placement of 20x10 cells: %q", got[:min(len(got), 120)])
	}
	before := sink.String()
	a.draw()
	a.draw()
	if sink.String() != before {
		t.Fatal("an unchanged frame must not resend the picture")
	}
	// New content pushes it out of the viewport (the view follows the tail): it is removed.
	a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: strings.Repeat("later text\n\n", 80)})
	a.draw()
	if !strings.Contains(strings.TrimPrefix(sink.String(), before), "a=d,d=A") {
		t.Fatalf("scrolled away: the placement must be deleted: %q", strings.TrimPrefix(sink.String(), before))
	}
	// Back at the top the picture returns; a dialog then removes it again.
	a.st.Scroll = 10000
	a.draw()
	if !strings.Contains(strings.TrimPrefix(sink.String(), before), "a=T") {
		t.Fatalf("the picture must return when it is visible again")
	}
	sink.Reset()
	a.openOverlay(OverHelp)
	a.draw()
	if !strings.Contains(sink.String(), "a=d,d=A") {
		t.Fatalf("a dialog must remove the pictures: %q", sink.String())
	}
}

func TestNoGraphicsWhenOffOrUnsupportedAndCaptionStays(t *testing.T) {
	a, sink := imageApp(t, "off")
	a.draw()
	time.Sleep(50 * time.Millisecond)
	a.draw()
	if sink.String() != "" {
		t.Fatalf("off writes nothing: %q", sink.String())
	}
	if !strings.Contains(screenTextOf(a), "▣ Image #1 · 160×160 · png · attachment") {
		t.Fatalf("the caption is the fallback:\n%s", screenTextOf(a))
	}
	// Auto in an unrecognized terminal is off too.
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TERM_PROGRAM", "")
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	a.st.Settings.ImagesProtocol = "auto"
	a.draw()
	if sink.String() != "" {
		t.Fatalf("auto without a recognized terminal writes nothing: %q", sink.String())
	}
}

func TestSixelAndITermUseTheirSequencesAndRepaintOnChange(t *testing.T) {
	for proto, marker := range map[string]string{"sixel": "\x1bPq", "iterm": "\x1b]1337;File=inline=1"} {
		a, sink := imageApp(t, proto)
		got := drawUntil(t, a, sink, func(s string) bool { return strings.Contains(s, marker) })
		if strings.Count(got, marker) != 1 {
			t.Fatalf("%s: %q", proto, got[:min(len(got), 80)])
		}
		a.st.Session.Append(agent.Turn{Role: llm.RoleAssistant, Text: strings.Repeat("later text\n\n", 80)})
		a.draw() // pixels are removed by repainting the cells: no new image bytes
		if strings.Count(sink.String(), marker) != 1 {
			t.Fatalf("%s: a scrolled-away picture must not be painted again", proto)
		}
	}
}

func TestMissingStoredBytesKeepTheCaption(t *testing.T) {
	a, sink := imageApp(t, "kitty")
	os.Remove(filepath.Join(a.attachDir(), "sha1.png"))
	for i := 0; i < 5; i++ {
		a.draw()
		time.Sleep(20 * time.Millisecond)
		for len(a.events) > 0 {
			a.handle(<-a.events)
		}
	}
	if strings.Contains(sink.String(), "a=T") || !strings.Contains(screenTextOf(a), "▣ Image #1") {
		t.Fatalf("missing bytes: caption only, no graphics: %q", sink.String())
	}
}

func screenTextOf(a *App) string {
	return screenText(a.scr.(tcell.SimulationScreen))
}

func TestImageVisibilityFollowsViewSwitches(t *testing.T) {
	for proto, marker := range map[string]string{"kitty": "a=T", "sixel": "\x1bPq", "iterm": "\x1b]1337;File=inline=1"} {
		t.Run(proto, func(t *testing.T) {
			a, sink := imageApp(t, proto)
			a.st.Settings.View = "compact"
			a.draw()
			if sink.String() != "" || len(a.imgs.cache) != 0 || strings.Contains(screenTextOf(a), "▣ Image") {
				t.Fatal("compact must not draw, decode, or caption images")
			}
			a.st.Settings.View = "full"
			drawUntil(t, a, sink, func(s string) bool { return strings.Contains(s, marker) })
			sink.Reset()
			a.st.Settings.View = "compact"
			a.draw()
			if strings.Contains(sink.String(), marker) || a.imgs.placed != "" || len(a.imgs.vis) != 0 || strings.Contains(screenTextOf(a), "▣ Image") {
				t.Fatal("switching to compact must remove images and captions")
			}
			if proto == "kitty" && !strings.Contains(sink.String(), "a=d,d=A") {
				t.Fatal("switching to compact must delete the Kitty placement")
			}
			// A late decode completion must not resurrect a hidden picture.
			a.handle(imageDecoded{})
			a.draw()
			if strings.Contains(sink.String(), marker) || !strings.Contains(screenTextOf(a), "look [Image #1]") {
				t.Fatal("compact must keep only ordinary message text")
			}
			a.st.Settings.View = "bash_only"
			drawUntil(t, a, sink, func(s string) bool { return strings.Contains(s, marker) })
			if !strings.Contains(screenTextOf(a), "▣ Image #1") {
				t.Fatal("bash-only must restore image captions")
			}
		})
	}
}
