package app

// uiCall runs fn on the UI goroutine, which owns a.st.
type uiCall struct {
	fn   func()
	done chan struct{}
}

// onUI runs fn on the UI goroutine and waits for it. It reports false when the
// app is closing and fn did not run.
func (a *App) onUI(fn func()) bool {
	c := uiCall{fn, make(chan struct{})}
	a.postEvent(c)
	select {
	case <-c.done:
		return true
	case <-a.done:
		return false
	}
}
