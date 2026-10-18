package ui

// withTitle puts the model's title of a js call in the right corner of the last
// row of the block. When the row has no room for it (or already carries the
// timing), the title gets a row of its own below the block.
func withTitle(rows []Line, title string, w int, bg Line) []Line {
	if title == "" {
		return rows
	}
	seg := []Seg{{Text: title, FG: FgDim, Italic: true}}
	if n := len(rows); n > 0 {
		last := &rows[n-1]
		if len(last.Aside) == 0 && last.Width()+TextWidth(title)+2*padX < w {
			last.Aside = seg
			return rows
		}
	}
	bg.Aside = seg
	return append(rows, bg)
}
