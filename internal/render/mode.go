package render

import "github.com/gdamore/tcell/v2"

type Mode int

const (
	ModeInput Mode = iota
	ModeVisual
	ModeModal
	ModeEdit
)

func (m Mode) Label() string {
	switch m {
	case ModeInput:
		return "INPUT"
	case ModeVisual:
		return "VISUAL"
	case ModeEdit:
		return "EDIT"
	}
	return "MODAL"
}

func (m Mode) Color() tcell.Color {
	switch m {
	case ModeInput:
		return Green
	case ModeVisual:
		return Orange
	case ModeEdit:
		return Blue
	}
	return Purple
}
