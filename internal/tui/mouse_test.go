package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

func mouseEv(x, y int, b tcell.ButtonMask) *tcell.EventMouse {
	return tcell.NewEventMouse(x, y, b, tcell.ModNone)
}

// cellOf finds the screen cell of the first rune of word in the timeline.
func cellOf(t *testing.T, a *App, word string) (x, y int) {
	t.Helper()
	w, _ := a.scr.Size()
	lines := a.transcriptWithSelection(w)
	logH := a.logHeight()
	start := max(0, len(lines)-logH-a.st.Scroll)
	for i := start; i < len(lines) && i-start < logH; i++ {
		txt := lines[i].Text()
		if at := strings.Index(txt, word); at >= 0 {
			return render.PadX + render.TextWidth(txt[:at]), i - start
		}
	}
	t.Fatalf("%q is not on screen", word)
	return
}

func TestMouseDragSelectsAndCopiesText(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	say(t, a, "alpha wonderful omega")
	x, y := cellOf(t, a, "wonderful")
	a.onMouse(mouseEv(x, y, tcell.Button1))
	a.onMouse(mouseEv(x+4, y, tcell.Button1))
	a.onMouse(mouseEv(x+8, y, tcell.Button1))
	a.onMouse(mouseEv(x+8, y, tcell.ButtonNone))
	if !a.st.Selecting {
		t.Fatal("no selection after a drag")
	}
	if a.st.Copied != "wonderful" {
		t.Fatalf("copied %q", a.st.Copied)
	}
	// Backwards selections work too.
	a.onKey(keyK(tcell.KeyEsc))
	a.onMouse(mouseEv(x+8, y, tcell.Button1))
	a.onMouse(mouseEv(x+5, y, tcell.Button1))
	a.onMouse(mouseEv(x+5, y, tcell.ButtonNone))
	if a.st.Copied != "rful" {
		t.Fatalf("backwards selection copied %q", a.st.Copied)
	}
}

func TestMouseDoubleClickSelectsWordTripleClickTheLine(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	say(t, a, "alpha wonderful omega")
	x, y := cellOf(t, a, "wonderful")
	click := func() {
		a.onMouse(mouseEv(x+2, y, tcell.Button1))
		a.onMouse(mouseEv(x+2, y, tcell.ButtonNone))
	}
	click()
	if a.st.Selecting {
		t.Fatal("a single click must not select")
	}
	click()
	if a.st.Copied != "wonderful" {
		t.Fatalf("double click copied %q", a.st.Copied)
	}
	click()
	if !a.st.SelLine || !strings.Contains(a.st.Copied, "alpha wonderful omega") {
		t.Fatalf("triple click copied %q (line=%v)", a.st.Copied, a.st.SelLine)
	}
}

func TestMouseOutsideTheTimelineAndInOverlaysIsIgnored(t *testing.T) {
	a, scr, _ := e2eApp(t, &recClient{})
	defer scr.Fini()
	say(t, a, "hello")
	_, h := a.scr.Size()
	a.onMouse(mouseEv(3, h-1, tcell.Button1)) // the status bar
	a.onMouse(mouseEv(6, h-1, tcell.Button1))
	a.onMouse(mouseEv(6, h-1, tcell.ButtonNone))
	if a.st.Selecting || a.st.Copied != "" {
		t.Fatal("selection started outside the timeline")
	}
	a.openOverlay(OverHelp)
	a.onMouse(mouseEv(3, 1, tcell.Button1))
	a.onMouse(mouseEv(9, 1, tcell.Button1))
	a.onMouse(mouseEv(9, 1, tcell.ButtonNone))
	if a.st.Selecting {
		t.Fatal("overlay lets the mouse through to the timeline")
	}
}
