package tui

import (
	"strings"

	"meno/internal/safe"
)

// dispatch handles one UI message. A panic in a handler is shown as an error
// and does not stop the application.
func (a *App) dispatch(msg any) {
	if err := safe.Call(func() { a.handle(msg) }); err != nil {
		a.notifyErr("internal error, meno keeps running: " + strings.SplitN(err.Error(), "\n", 2)[0])
	}
}

// drawSafe renders a frame. A rendering panic is reported, not fatal; the next
// frame is tried again, and the same error is not repeated in the transcript.
func (a *App) drawSafe() {
	err := safe.Call(a.draw)
	msg := ""
	if err != nil {
		msg = strings.SplitN(err.Error(), "\n", 2)[0]
	}
	if msg != a.drawErr {
		a.drawErr = msg
		if err != nil {
			a.notifyErr("render error, meno keeps running: " + msg)
		}
	}
}

// internalPanic carries a panic recovered in a background goroutine to the UI loop.
type internalPanic struct{ err error }

// reportPanic is safe to call from any goroutine.
func (a *App) reportPanic(err error) { a.postEvent(internalPanic{err: err}) }
