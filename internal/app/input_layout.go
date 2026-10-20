package app

import (
	"github.com/rivo/uniseg"
	"unicode"
)

// layoutInput keeps contiguous rune offsets and reserves one cursor cell.
func layoutInput(in []rune, innerW int) []vrow {
	width := max(1, innerW-1)
	var rows []vrow
	for start := 0; ; {
		end := start
		for end < len(in) && in[end] != '\n' {
			end++
		}
		row := start
		for row < end {
			col, off, boundary := 0, row, row
			g := uniseg.NewGraphemes(string(in[row:end]))
			for g.Next() {
				text := g.Str()
				n := len([]rune(text))
				gw := cellWidth(text)
				if col+gw > width && off > row {
					break
				}
				col += gw
				off += n
				rs := []rune(text)
				if len(rs) > 0 && unicode.IsSpace(rs[0]) {
					boundary = off
				}
			}
			if off < end && boundary > row {
				off = boundary
			}
			rows = append(rows, vrow{row, off, off < end})
			row = off
		}
		if start == end {
			rows = append(rows, vrow{start, end, false})
		}
		if end >= len(in) {
			return rows
		}
		start = end + 1
	}
}
