package app

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"

	"meno/internal/attach"
	"meno/internal/safe"
	"meno/internal/ui"
)

// attachAsync prepares an image in the background and inserts it when ready.
func (a *App) attachAsync(after bool, load func() (attach.Image, error)) {
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
		read = func() ([]byte, error) { return attach.Clipboard(runtime.GOOS) }
	}
	a.attachAsync(after, func() (attach.Image, error) {
		data, err := read()
		if err != nil {
			return attach.Image{}, err
		}
		return attach.Prepare(data)
	})
}

func (a *App) attachFile(path string, after bool) {
	a.attachAsync(after, func() (attach.Image, error) {
		st, err := os.Stat(path)
		if err != nil {
			return attach.Image{}, err
		}
		if !st.Mode().IsRegular() {
			return attach.Image{}, errors.New(path + " is not a file")
		}
		if st.Size() > attach.MaxBytes {
			return attach.Image{}, errors.New("image is larger than " + itoa(attach.MaxBytes>>20) + " MB")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return attach.Image{}, err
		}
		return attach.Prepare(data)
	})
}

func (a *App) onImageReady(m imageReady) {
	a.inRT(m.sid, func() {
		if m.err != nil {
			a.notifyErr(m.err.Error())
			return
		}
		a.insertImage(m.img, m.after)
	})
}

func userHome() string {
	h, _ := os.UserHomeDir()
	return h
}

// onPaste handles the start and end of a bracketed paste. The text between
// them arrives as key events and is collected, not typed.
func (a *App) onPaste(e *tcell.EventPaste) {
	if e.Start() {
		a.pasting = a.st.Over == OverNone && (a.st.Ask != nil || a.st.Mode == ui.ModeInput || a.st.Mode == ui.ModeCommand)
		a.pasteBuf = a.pasteBuf[:0]
		return
	}
	if !a.pasting {
		return
	}
	a.pasting = false
	text := strings.ReplaceAll(strings.ReplaceAll(string(a.pasteBuf), "\r\n", "\n"), "\r", "\n")
	a.pasteBuf = a.pasteBuf[:0]
	if a.st.Mode == ui.ModeCommand && a.st.Ask == nil {
		a.st.CmdUndo = append(a.st.CmdUndo, a.commandPoint())
		a.st.CmdRedo = nil
		c := min(a.st.CmdCursor, len(a.st.Cmd))
		a.st.Cmd = append(a.st.Cmd[:c], append([]rune(text), a.st.Cmd[c:]...)...)
		a.st.CmdCursor = c + len([]rune(text))
		return
	}
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
		if p, ok := attach.PathFromText(text, userHome()); ok && a.st.Ask == nil {
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
