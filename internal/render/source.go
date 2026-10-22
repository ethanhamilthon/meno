package render

import "meno/internal/content"

// Source says which saved turn a rendered row came from and what part of it,
// so a selection can be quoted exactly. Rows without a Source are decoration
// (headers, bands, collapse markers, live text) and cannot be quoted. Skip is
// the number of leading runes of the row that are layout, not content.
type Source struct {
	TurnID string
	CallID string
	Part   string
	Skip   int
}

// mark attaches the source of part to the rows of a saved turn. Turns without
// an ID (live or synthetic) stay unquotable.
func mark(rows []Line, t Turn, part string, skip int) []Line {
	if t.ID == "" {
		return rows
	}
	src := &Source{TurnID: t.ID, CallID: t.CallID, Part: part, Skip: skip}
	for i := range rows {
		rows[i].Source = src
	}
	// The first row of an error reads "error: text"; the prefix is layout.
	if part == content.PartError && len(rows) > 0 {
		first := *src
		first.Skip += len("error: ")
		rows[0].Source = &first
	}
	return rows
}

// unmarkCollapsed removes the source from a trailing "…" row that stands for
// code or output that was cut off: it is not text the turn contains.
func unmarkCollapsed(rows []Line) []Line {
	if n := len(rows); n > 0 && rows[n-1].Text() == "  …" {
		rows[n-1].Source = nil
	}
	return rows
}

func errorPart(t Turn) string {
	if t.Err != "" {
		return content.PartError
	}
	return content.PartOutput
}
