package ui

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	east "github.com/yuin/goldmark/extension/ast"
)

// wrapSegments cuts styled text into lines of at most width columns, keeping
// every space.
func wrapSegments(segs []Seg, width int) []Line {
	var out []Line
	for _, l := range preserveWords(sourceCells(segs), width) {
		out = append(out, Line{Segs: joinCells(l), BG: BgTerm})
	}
	return out
}

// minNumberedLines is the length from which a code block gets line numbers.
const minNumberedLines = 6

// codeBlock draws a fenced block on a panel as wide as the text: the language
// and file name on top, line numbers on long blocks, highlighted code, diffs
// with tinted added and removed lines. info is the text after the opening fence.
func codeBlock(info string, src []string, w int) []Line {
	fields := strings.Fields(info)
	lang, extra := "", ""
	if len(fields) > 0 {
		lang = strings.Trim(fields[0], "{}.")
		extra = strings.Join(fields[1:], " ")
	}
	for len(src) > 0 && strings.TrimSpace(src[len(src)-1]) == "" {
		src = src[:len(src)-1]
	}
	for i := range src {
		src[i] = strings.ReplaceAll(src[i], "\t", "    ")
	}
	panel := Seg{BG: BgPopup, FG: FgWhite}
	var out []Line
	row := func(segs []Seg, bg tcell.Color) {
		fill := Seg{BG: bg, FG: FgWhite}
		out = append(out, Line{Segs: fitRow(segs, w, fill), BG: BgTerm})
	}
	if lang != "" || extra != "" {
		head := []Seg{{Text: " ", BG: BgPopup}}
		if lang != "" {
			head = append(head, Seg{Text: lang, FG: Blue, Bold: true, BG: BgPopup})
		}
		if extra != "" {
			head = append(head, Seg{Text: "  " + extra, FG: FgDim, BG: BgPopup})
		}
		row(head, BgPopup)
	}
	diff := strings.EqualFold(lang, "diff") || strings.EqualFold(lang, "patch")
	numbered := !diff && len(src) >= minNumberedLines
	digits := len(strconv.Itoa(len(src)))
	lead := 1
	if numbered {
		lead = 1 + digits + 2
	}
	if w < lead+4 {
		lead = min(1, w/2)
		numbered = false
	}
	cw := max(1, w-lead-1)
	if w < lead+4 {
		cw = max(1, w-lead)
	}
	hl := &highlighter{lang: langOf(lang)}
	for i, line := range src {
		bg := BgPopup
		var segs []Seg
		if diff {
			bg, segs = diffLine(line)
		} else {
			segs = hl.line(line, BgPopup)
		}
		for j := range segs {
			segs[j].BG = bg
		}
		for k, part := range preserveWords(sourceCells(segs), cw) {
			margin := strings.Repeat(" ", lead)
			if numbered {
				n := ""
				if k == 0 {
					n = strconv.Itoa(i + 1)
				}
				margin = " " + strings.Repeat(" ", digits-len(n)) + n + "  "
			}
			gutter := Seg{Text: margin, FG: FgDim, BG: bg}
			row(append([]Seg{gutter}, joinCells(part)...), bg)
		}
	}
	if len(src) == 0 {
		row([]Seg{panel}, BgPopup)
	}
	return out
}

// diffLine colours one line of a unified diff and returns its background.
func diffLine(l string) (tcell.Color, []Seg) {
	seg := func(t string, fg tcell.Color, bold bool) Seg { return Seg{Text: t, FG: fg, Bold: bold} }
	switch {
	case strings.HasPrefix(l, "+++"), strings.HasPrefix(l, "---"), strings.HasPrefix(l, "diff "), strings.HasPrefix(l, "index "),
		strings.HasPrefix(l, "new file"), strings.HasPrefix(l, "deleted file"), strings.HasPrefix(l, "similarity "), strings.HasPrefix(l, "rename "):
		return BgPopup, []Seg{seg(" "+l, FgDim, true)}
	case strings.HasPrefix(l, "@@"):
		return BgPopup, []Seg{seg(" "+l, Blue, false)}
	case strings.HasPrefix(l, "+"):
		return BgJSDone, []Seg{seg("+", Green, true), seg(" "+l[1:], FgWhite, false)}
	case strings.HasPrefix(l, "-"):
		return BgJSErr, []Seg{seg("-", Red, true), seg(" "+l[1:], FgWhite, false)}
	case strings.HasPrefix(l, "\\"):
		return BgPopup, []Seg{seg(" "+l, FgDim, false)}
	}
	return BgPopup, []Seg{seg("  ", FgDim, false), seg(strings.TrimPrefix(l, " "), FgSoft, false)}
}

// ---- tables ----

func (r *mdr) tableRows(t *east.Table) (rows [][][]Seg, header bool) {
	for c := t.FirstChild(); c != nil; c = c.NextSibling() {
		st := mstyle{fg: FgWhite}
		if _, ok := c.(*east.TableHeader); ok {
			st.bold, header = true, true
		}
		var row [][]Seg
		for cell := c.FirstChild(); cell != nil; cell = cell.NextSibling() {
			row = append(row, r.inlines(cell, st))
		}
		rows = append(rows, row)
	}
	return rows, header
}

func (r *mdr) table(t *east.Table, w int) []Line {
	rows, header := r.tableRows(t)
	cols := len(t.Alignments)
	for _, row := range rows {
		cols = max(cols, len(row))
	}
	if cols == 0 || len(rows) == 0 {
		return nil
	}
	natural := make([]int, cols)
	word := make([]int, cols)               // the longest word of each column: what shrinking tries to keep whole
	cells := make([][][][]wcell, len(rows)) // row, column, line
	for i, row := range rows {
		cells[i] = make([][][]wcell, cols)
		for j := 0; j < cols; j++ {
			var segs []Seg
			if j < len(row) {
				segs = row[j]
			}
			wide := breakCells(cellsOf(segs), 1<<20)
			for _, l := range wide {
				n, run := 0, 0
				for _, c := range l {
					n += c.w
					if c.space || c.w > 1 {
						run = 0
						continue
					}
					run++
					word[j] = max(word[j], run)
				}
				natural[j] = max(natural[j], n)
			}
		}
	}
	for j := range natural {
		natural[j] = max(1, natural[j])
	}
	widths := append([]int(nil), natural...)
	total := 0
	for _, n := range widths {
		total += n
	}
	budget := w - (3*cols + 1)
	// First narrow the columns that have room around their longest word, then,
	// if that is not enough, cut words as well.
	for _, floor := range []func(j int) int{
		func(j int) int { return min(natural[j], max(word[j], 4)) },
		func(j int) int { return min(natural[j], 4) },
	} {
		for total > budget {
			widest := -1
			for j, n := range widths {
				if n > floor(j) && (widest < 0 || n > widths[widest]) {
					widest = j
				}
			}
			if widest < 0 {
				break
			}
			widths[widest]--
			total--
		}
	}
	if total > budget {
		return tableAsList(rows, header, w)
	}
	multi := false
	for i, row := range rows {
		for j := 0; j < cols; j++ {
			var segs []Seg
			if j < len(row) {
				segs = row[j]
			}
			cells[i][j] = breakCells(cellsOf(segs), widths[j])
			if len(cells[i][j]) > 1 {
				multi = true
			}
		}
	}
	edge := func(l, m, rr string) Line {
		parts := make([]string, cols)
		for j, n := range widths {
			parts[j] = strings.Repeat("─", n+2)
		}
		return Plain(l+strings.Join(parts, m)+rr, FgDim, BgTerm)
	}
	border := Seg{Text: "│", FG: FgDim}
	var out []Line
	out = append(out, edge("┌", "┬", "┐"))
	for i := range rows {
		height := 1
		for j := 0; j < cols; j++ {
			height = max(height, len(cells[i][j]))
		}
		for k := 0; k < height; k++ {
			segs := []Seg{border}
			for j := 0; j < cols; j++ {
				var part []Seg
				if k < len(cells[i][j]) {
					part = joinCells(cells[i][j][k])
				}
				pad := widths[j] - segsWidth(part)
				left := 0
				switch alignOf(t, j) {
				case east.AlignRight:
					left = pad
				case east.AlignCenter:
					left = pad / 2
				}
				segs = append(segs, Seg{Text: strings.Repeat(" ", 1+left)})
				segs = append(segs, part...)
				segs = append(segs, Seg{Text: strings.Repeat(" ", 1+pad-left)}, border)
			}
			out = append(out, Line{Segs: segs, BG: BgTerm})
		}
		switch {
		case header && i == 0:
			out = append(out, edge("├", "┼", "┤"))
		case multi && i < len(rows)-1:
			out = append(out, edge("├", "┼", "┤"))
		}
	}
	return append(out, edge("└", "┴", "┘"))
}

func alignOf(t *east.Table, j int) east.Alignment {
	if j < len(t.Alignments) {
		return t.Alignments[j]
	}
	return east.AlignNone
}

// tableAsList is the fallback when the columns cannot fit: one wrapped line per
// row, cells apart by a bar.
func tableAsList(rows [][][]Seg, header bool, w int) []Line {
	var out []Line
	for i, row := range rows {
		var segs []Seg
		for j, cell := range row {
			if j > 0 {
				segs = append(segs, Seg{Text: " │ ", FG: FgDim})
			}
			segs = append(segs, cell...)
		}
		out = append(out, wrapWords(segs, w)...)
		if header && i == 0 {
			out = append(out, Plain(strings.Repeat("─", max(1, min(w, 20))), FgDim, BgTerm))
		}
	}
	return out
}
