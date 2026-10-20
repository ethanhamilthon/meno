package engine

import "context"

// bashExit reads the exit code of a finished bash call from its status line.
func (e *Engine) bashExit(id, output string) (int, bool) {
	s, err := e.load(context.Background(), id)
	if err != nil || s == nil {
		return 0, false
	}
	return *exitFromOutput(output), true
}
