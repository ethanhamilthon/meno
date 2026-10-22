package tui

// Composer helpers always target a runtime. Only active edits reset popup selection.
func (a *App) setInputFor(rt *Runtime, rs []rune) {
	rt.InLog = false
	rt.Input, rt.Cursor, rt.WantSet = rs, len(rs), false
	if rt == a.st.Runtime {
		a.st.Sel = 0
	}
}

func (rt *Runtime) clampCursor() {
	rt.Cursor = min(max(0, rt.Cursor), len(rt.Input))
}

func (a *App) insertAtCursorFor(rt *Runtime, rs []rune) {
	rt.clampCursor()
	c := rt.Cursor
	out := make([]rune, 0, len(rt.Input)+len(rs))
	out = append(out, rt.Input[:c]...)
	out = append(out, rs...)
	out = append(out, rt.Input[c:]...)
	rt.Input, rt.Cursor, rt.WantSet = out, c+len(rs), false
	if rt == a.st.Runtime {
		a.st.Sel = 0
	}
}

func (rt *Runtime) snapshot() snap {
	return snap{in: append([]rune(nil), rt.Input...), cur: rt.Cursor, imgs: append([]draftImage(nil), rt.Images...)}
}
