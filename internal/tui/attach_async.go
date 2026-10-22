package tui

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/content"
	"meno/internal/render"
	"meno/internal/safe"
)

// attachAsync prepares an image in the background and inserts it when ready.
func (a *App) attachAsync(after bool, load func() (content.Image, error)) {
	sid := a.st.ID()
	go func() {
		defer safe.Recover(a.reportPanic)
		img, err := load()
		a.postEvent(imageReady{sid: sid, img: img, err: err, after: after})
	}()
}

func (a *App) attachFromClipboard(after bool) {
	read := a.clipboard
	if read == nil {
		read = func() ([]byte, error) { return content.Clipboard(runtime.GOOS) }
	}
	a.attachAsync(after, func() (content.Image, error) {
		data, err := read()
		if err != nil {
			return content.Image{}, err
		}
		return content.Prepare(data)
	})
}

func (a *App) attachFile(path string, after bool) {
	a.attachAsync(after, func() (content.Image, error) {
		st, err := os.Stat(path)
		if err != nil {
			return content.Image{}, err
		}
		if !st.Mode().IsRegular() {
			return content.Image{}, errors.New(path + " is not a file")
		}
		if st.Size() > content.MaxBytes {
			return content.Image{}, errors.New("image is larger than " + itoa(content.MaxBytes>>20) + " MB")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return content.Image{}, err
		}
		return content.Prepare(data)
	})
}

func (a *App) onImageReady(m imageReady) {
	rt := a.rts[m.sid]
	if rt == nil {
		return
	}
	if m.err != nil {
		a.imageError(rt, m.err.Error())
		return
	}
	a.insertImageFor(rt, m.img, m.after)
}

func userHome() string {
	h, _ := os.UserHomeDir()
	return h
}

// onPaste handles the start and end of a bracketed paste. The text between
// them arrives as key events and is collected, not typed.
func (a *App) onPaste(e *tcell.EventPaste) {
	if e.Start() {
		a.pasting = a.st.Over == OverNone && (a.st.Ask != nil || a.st.Mode == render.ModeInput)
		a.pasteBuf = a.pasteBuf[:0]
		return
	}
	if !a.pasting {
		return
	}
	a.pasting = false
	text := strings.ReplaceAll(strings.ReplaceAll(string(a.pasteBuf), "\r\n", "\n"), "\r", "\n")
	a.pasteBuf = a.pasteBuf[:0]
	if a.st.Ask != nil {
		if a.st.Ask.Typing || a.st.Ask.onOther() {
			a.st.Ask.Typing = true
			a.askInsert(text)
		}
		return
	}
	switch {
	case text == "":
		// Terminals send an empty paste when the clipboard holds no text.
		a.attachFromClipboard(false)
	default:
		if p, ok := content.PathFromText(text, userHome()); ok && a.st.Ask == nil {
			a.attachFile(p, false)
			return
		}
		a.clampCursor()
		a.insertAtCursor(a.runesFromText(text))
	}
}

func (a *App) collectPaste(k *tcell.EventKey) {
	switch k.Key() {
	case tcell.KeyRune:
		a.pasteBuf = append(a.pasteBuf, k.Rune())
	case tcell.KeyEnter, tcell.KeyCtrlJ:
		a.pasteBuf = append(a.pasteBuf, '\n')
	case tcell.KeyTab:
		a.pasteBuf = append(a.pasteBuf, '\t')
	}
}
