package tui

// uiCall runs fn on the UI goroutine, which owns a.st.
type uiCall struct {
	fn   func()
	done chan struct{}
}
