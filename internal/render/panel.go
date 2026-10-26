package render

// PanelBounds reserves chrome above the fixed composer, without covering history.
type PanelBounds struct{ X, Y, Width, Height int }

func ComposerPanel(w, h, composer, desired int) PanelBounds {
	bottom := max(0, h-1-min(composer, max(0, h-1)))
	height := min(max(0, desired), bottom)
	gutter := min(Gutter, max(0, (w-1)/2))
	return PanelBounds{gutter, bottom - height, max(0, w-2*gutter), height}
}

func DrawPanel(c Canvas, b PanelBounds) {
	c.Fill(b.X, b.Y, b.Width, b.Height, BgTerm)
	if b.Height > 0 {
		c.Text(b.X, b.Y, b.Width, repeatRule(b.Width), Style{FG: FgFaint, BG: BgTerm}.TCell())
	}
}

func repeatRule(w int) string {
	out := make([]rune, w)
	for i := range out {
		out[i] = '─'
	}
	return string(out)
}
