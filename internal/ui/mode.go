package ui

import "github.com/gdamore/tcell/v2"

type Mode int

const (
	ModeInsert Mode = iota
	ModeNormal
	ModeVisual
	ModeCommand
	ModeModal
)

func (m Mode) Label() string {
	switch m {
	case ModeInsert:
		return "INSERT"
	case ModeNormal:
		return "NORMAL"
	case ModeVisual:
		return "VISUAL"
	case ModeCommand:
		return "COMMAND"
	}
	return "MODAL"
}

func (m Mode) Color() tcell.Color {
	switch m {
	case ModeInsert:
		return Green
	case ModeNormal:
		return Blue
	case ModeVisual:
		return Orange
	case ModeCommand:
		return Yellow
	}
	return Purple
}
