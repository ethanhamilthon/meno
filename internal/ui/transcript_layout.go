package ui

// TranscriptTextWidth is the width available after both outer gutters and the
// two-cell marker lane. Keep at least one cell for extremely narrow terminals.
func TranscriptTextWidth(width int) int {
	return max(1, width-2*Gutter-2)
}

// transcriptRows reserves the marker lane on every row, including wrapped rows.
func transcriptRows(text, marker string, width int) []Line {
	first := true
	return WrapLines(text, TranscriptTextWidth(width), func(s string) Line {
		prefix := "  "
		if first {
			prefix = marker + " "
			first = false
		}
		return Line{Segs: []Seg{{Text: prefix, FG: FgDim}, {Text: s, FG: FgDim}}, BG: BgTerm}
	})
}

// transcriptSummary adds the same marker lane as message and tool body rows.
// SummaryLines itself continues to accept an available text width.
func transcriptSummary(stats *RunStats, width int) []Line {
	rows := SummaryLines(stats, TranscriptTextWidth(width))
	for i := range rows {
		rows[i].Segs = append([]Seg{{Text: "  ", FG: FgDim}}, rows[i].Segs...)
	}
	return rows
}
