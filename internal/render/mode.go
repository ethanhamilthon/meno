package render

import "github.com/gdamore/tcell/v2"

type Mode int

const (
	ModeInput Mode = iota
	ModeModal
)

func (m Mode) Label() string {
	switch m {
	case ModeInput:
		return "INPUT"
	}
	return "MODAL"
}

func (m Mode) Color() tcell.Color {
	switch m {
	case ModeInput:
		return Green
	}
	return Purple
}
