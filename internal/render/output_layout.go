package render

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/uniseg"
	"strings"
)

// limitedOutput mirrors WrapLines, but hidden rows are counted, not allocated.
func limitedOutput(text string, width, limit int, fg tcell.Color) []Line {
	text = strings.TrimRight(text, "\n")
	if width <= 0 || limit <= 0 {
		return WrapLines(text, width, func(s string) Line { return Plain("  "+s, fg, BgTerm) })
	}
	var rows []Line
	var b strings.Builder
	count, col := 0, 0
	flush := func() {
		if count < limit {
			rows = append(rows, Plain("  "+b.String(), fg, BgTerm))
			b.Reset()
		}
		count++
		col = 0
	}
	for {
		para, rest, more := strings.Cut(text, "\n")
		ascii := true
		for i := 0; i < len(para); i++ {
			if para[i] >= 128 {
				ascii = false
				break
			}
		}
		if ascii {
			for i := 0; i < len(para); i++ {
				ch := para[i]
				cw := 1
				if ch < 32 || ch == 127 {
					cw = 0
				}
				if col+cw > width {
					flush()
				}
				if count < limit {
					b.WriteByte(ch)
				}
				col += cw
			}
		} else {
			g := uniseg.NewGraphemes(para)
			for g.Next() {
				gr := g.Str()
				cw := uniseg.StringWidth(gr)
				if col+cw > width {
					flush()
				}
				if count < limit {
					b.WriteString(gr)
				}
				col += cw
			}
		}
		flush()
		if !more {
			break
		}
		text = rest
	}
	if count > limit {
		rows = append(rows, Line{Segs: []Seg{{Text: "  … " + itoa(count-limit) + " more", FG: FgDim}}, BG: BgTerm})
	}
	return rows
}

func limitedTurnOutput(t Turn, w int) []Line {
	if t.Err != "" {
		return limitedOutput("error: "+t.Err, TranscriptTextWidth(w), OutputRows, Red)
	}
	if t.Output == "" {
		return nil
	}
	return limitedOutput(t.Output, TranscriptTextWidth(w), OutputRows, FgDim)
}
