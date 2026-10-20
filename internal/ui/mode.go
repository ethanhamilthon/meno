package ui

import "github.com/gdamore/tcell/v2"

type Mode int

const (
	ModeInput Mode = iota
	ModeVisual
	ModeCommand
	ModeModal
)

func (m Mode) Label() string {
	switch m {
	case ModeInput:
		return "INPUT"
	case ModeVisual:
		return "VISUAL"
	case ModeCommand:
		return "COMMAND"
	}
	return "MODAL"
}

func (m Mode) Color() tcell.Color {
	switch m {
	case ModeInput:
		return Green
	case ModeVisual:
		return Orange
	case ModeCommand:
		return Yellow
	}
	return Purple
}
