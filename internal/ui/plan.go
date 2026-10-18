package ui

import (
	"strconv"

	"github.com/gdamore/tcell/v2"

	"meno/internal/agent"
)

var itemGlyph = map[string]string{
	agent.ItemPending: "○", agent.ItemInProgress: "◐", agent.ItemBlocked: "⊘", agent.ItemDone: "●", agent.ItemCancelled: "✕",
}

var critGlyph = map[string]string{
	agent.CritNotChecked: "○", agent.CritPassed: "✓", agent.CritFailed: "✗", agent.CritDeferred: "↷",
}

// PlanSummary is the one-line state of a plan: steps done and criteria passed.
func PlanSummary(p *agent.Plan) string {
	done := 0
	for _, it := range p.Items {
		if it.Status == agent.ItemDone || it.Status == agent.ItemCancelled {
			done++
		}
	}
	passed := 0
	for _, c := range p.Criteria {
		if c.Status == agent.CritPassed {
			passed++
		}
	}
	s := "plan · steps " + strconv.Itoa(done) + "/" + strconv.Itoa(len(p.Items))
	if len(p.Criteria) > 0 {
		s += " · criteria " + strconv.Itoa(passed) + "/" + strconv.Itoa(len(p.Criteria))
	}
	if n := len(p.Unverified()); n > 0 {
		s += " · " + strconv.Itoa(n) + " unverified"
	}
	return s
}

// PlanLines renders the plan block of the timeline: read-only, steps and
// criteria apart. Collapsed it is a single line.
func PlanLines(p *agent.Plan, w int, collapsed bool) []Line {
	if p == nil || (len(p.Items) == 0 && len(p.Criteria) == 0) {
		return nil
	}
	w = max(1, w-PadX*2)
	mark := "▾ "
	if collapsed {
		mark = "▸ "
	}
	out := []Line{Plain("  "+Truncate(mark+PlanSummary(p), w), FgSoft, BgTerm)}
	if collapsed {
		return out
	}
	add := func(glyph, text string, fg tcell.Color) {
		out = append(out, WrapLines(glyph+" "+text, w-4, func(s string) Line { return Plain("    "+s, fg, BgTerm) })...)
	}
	for _, it := range p.Items {
		fg := FgWhite
		if it.Status == agent.ItemDone || it.Status == agent.ItemCancelled {
			fg = FgDim
		}
		add(itemGlyph[it.Status], it.Title, fg)
	}
	if len(p.Criteria) > 0 {
		out = append(out, Plain("    acceptance", FgDim, BgTerm))
	}
	for _, c := range p.Criteria {
		fg := FgWhite
		switch c.Status {
		case agent.CritPassed:
			fg = Green
		case agent.CritFailed:
			fg = Red
		case agent.CritDeferred:
			fg = Yellow
		}
		add(critGlyph[c.Status], c.Text, fg)
	}
	return out
}
