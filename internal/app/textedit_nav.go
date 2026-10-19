package app

import (
	"unicode"

	"github.com/rivo/uniseg"
)

// Cursor movement on a rune slice, shared by the composer and textEdit.

// wordBoundary is the start of the previous (dir<0) or the end of the next
// (dir>0) word.
func wordBoundary(in []rune, c, dir int) int {
	if dir < 0 {
		for c > 0 && unicode.IsSpace(in[c-1]) {
			c--
		}
		for c > 0 && !unicode.IsSpace(in[c-1]) {
			c--
		}
		return c
	}
	for c < len(in) && unicode.IsSpace(in[c]) {
		c++
	}
	for c < len(in) && !unicode.IsSpace(in[c]) {
		c++
	}
	return c
}

// lineEdge is the start (dir<0) or end (dir>0) of the logical line holding c.
func lineEdge(in []rune, c, dir int) int {
	if dir < 0 {
		for c > 0 && in[c-1] != '\n' {
			c--
		}
		return c
	}
	for c < len(in) && in[c] != '\n' {
		c++
	}
	return c
}

// vertTarget is the cursor one visual row up (d<0) or down (d>0) that keeps
// the column wantCol. Past the first or last row it jumps to the start or end.
func vertTarget(in []rune, rows []vrow, cursor, wantCol, d int) int {
	nr := cursorRow(rows, cursor) + d
	if nr < 0 {
		return 0
	}
	if nr >= len(rows) {
		return len(in)
	}
	row := rows[nr]
	idx, col := row.start, 0
	g := uniseg.NewGraphemes(string(in[row.start:row.end]))
	for g.Next() {
		gw := cellWidth(g.Str())
		if col+gw > wantCol {
			break
		}
		col += gw
		idx += len([]rune(g.Str()))
	}
	// A cursor at the end of a wrapped row would render at the next row's start.
	if row.soft && idx == row.end && idx > row.start {
		idx -= lastGraphemeLen(in[row.start:idx])
	}
	return idx
}
