package tui

import (
	"time"

	"github.com/gdamore/tcell/v2"

	"meno/internal/render"
)

// askHit is a clickable area of the form from the last draw.
type askHit struct {
	x, y, w int
	kind    int // hitRow, hitQuestion, hitButton
	idx     int
}

const (
	hitRow = iota
	hitQuestion
	hitButton
)

// askButtonLabels picks the widest label set that fits inner columns; the
// narrowest still names every action.
func askButtonLabels(inner int) (labels [focusCount]string, gap int) {
	sets := [][focusCount]string{
		{"", "[ ‹ Prev ]", "[ Next › ]", "[ Cancel ]"},
		{"", "[‹ Prev]", "[Next ›]", "[Cancel]"},
		{"", "[‹]", "[›]", "[Cancel]"},
		{"", "[‹]", "[›]", "[✕]"},
	}
	for _, set := range sets {
		for _, g := range []int{2, 1} {
			total := g * (focusCount - 2)
			for _, l := range set[focusPrev:] {
				total += render.TextWidth(l)
			}
			if total <= inner {
				return set, g
			}
		}
	}
	return sets[len(sets)-1], 1
}

// askBoxFor sizes the dialog: wider than the other dialogs, never wider than
// the terminal.
func askBoxFor(sw, sh, contentH int) box {
	bw := min(sw, max(20, min(100, sw-2)))
	oh := max(0, min(contentH+2, sh))
	oy := (sh - oh) / 2
	return box{x: (sw - bw) / 2, y: oy + 1, w: bw, h: max(0, oh-2), oy: oy, oh: oh}
}

// askLines is the content of the form as drawn rows, built from the state alone
// so a resize only reflows it.
type askRow struct {
	text string
	fg   tcell.Color
	bg   tcell.Color
	hit  *askHit
}

func (a *App) drawAskForm(c render.Canvas, w, h int) {
	st := a.st.Ask
	if st == nil || w < 12 || h < 6 {
		return
	}
	bw := min(w, max(20, min(100, w-2)))
	inner := max(1, bw-4)
	bg := render.BgPopup
	var rows []askRow
	add := func(text string, fg tcell.Color) { rows = append(rows, askRow{text: text, fg: fg, bg: bg}) }

	n := len(st.F.Questions)
	title := "agent asks · question " + itoa(st.Cur+1) + "/" + itoa(n) + " · " + itoa(st.answeredCount()) + " answered"
	add(title, render.Orange)
	compact := w < 64 || h < 18 || n == 1
	if n > 1 && !compact {
		window := min(n, 5)
		start := min(max(0, st.Cur-window/2), n-window)
		for i := start; i < start+window; i++ {
			mark, fg := "○", render.FgDim
			if st.answered(i) {
				mark, fg = "●", render.Green
			}
			if i == st.Cur {
				fg = render.FgWhite
			}
			rows = append(rows, askRow{text: " " + mark + " " + itoa(i+1) + ". " + render.Truncate(st.F.Questions[i].Text, max(1, inner-8)), fg: fg, bg: bg, hit: &askHit{kind: hitQuestion, idx: i}})
		}
		if start+window < n || start > 0 {
			add("   ("+itoa(n)+" questions)", render.FgDim)
		}
	} else if n > 1 {
		add("‹ "+itoa(st.Cur+1)+"/"+itoa(n)+" ›  "+render.Truncate(st.q().Text, max(1, inner-12)), render.FgSoft)
	}
	add("", render.FgDim)
	q := st.q()
	for _, l := range wordWrap(q.Text, inner) {
		add(l, render.FgWhite)
	}
	if q.Recommendation != "" {
		for _, l := range wordWrap("suggested: "+q.Recommendation, inner) {
			add(l, render.FgDim)
		}
	}
	add("", render.FgDim)
	kind := map[string]string{askSingle: "choose one", askMulti: "choose any", askText: "type an answer"}[q.Type]
	add(kind, render.FgDim)
	ans := st.Ans[st.Cur]
	for i, o := range q.Options {
		mark := "( )"
		if q.Type == askMulti {
			mark = "[ ]"
		}
		if ans.Picked[i] {
			mark = map[bool]string{true: "[x]", false: "(•)"}[q.Type == askMulti]
		}
		fg, rbg := render.FgSoft, bg
		if st.Focus == focusBody && st.Row == i && !st.Typing {
			fg, rbg = render.FgWhite, render.BgPopupSel
		}
		rows = append(rows, askRow{text: "  " + mark + " " + render.Truncate(o.Label, max(1, inner-8)), fg: fg, bg: rbg, hit: &askHit{kind: hitRow, idx: i}})
	}
	otherLabel := "Other: "
	if q.Type == askText {
		otherLabel = ""
	}
	val := string(ans.Other)
	if st.Typing {
		val += "█"
	} else if val == "" && q.Type != askText {
		val = "type your own answer…"
	} else if val == "" {
		val = "press enter to type…"
	}
	fg, rbg := render.FgSoft, bg
	if st.Focus == focusBody && st.onOther() {
		fg, rbg = render.FgWhite, render.BgPopupSel
	}
	if string(ans.Other) == "" && !st.Typing {
		fg = render.FgDim
	}
	otherRow := len(q.Options)
	if q.Type == askText {
		otherRow = 0
	}
	for i, l := range wordWrap(otherLabel+val, max(1, inner-2)) {
		rows = append(rows, askRow{text: "  " + l, fg: fg, bg: rbg, hit: &askHit{kind: hitRow, idx: otherRow + i*0}})
	}
	add("", render.FgDim)
	if st.Notice != "" {
		add(st.Notice, render.Red)
	}
	// Buttons.
	labels, gap := askButtonLabels(inner)
	rows = append(rows, askRow{text: "", fg: render.FgSoft, bg: bg})
	hint := "↑/↓ move · space mark · enter next · ←/shift+tab back · tab buttons · ctrl+o sessions"
	if st.Typing {
		hint = "typing · enter next · shift+tab back · esc buttons · ctrl+u clear"
	}
	if render.TextWidth(hint) > inner {
		hint = "tab · ↑↓ · space · enter"
	}
	add(hint, render.FgDim)
	if !st.Until.IsZero() {
		rows[0].text += "  ·  " + fmtWait(max(time.Until(st.Until), 0)) + " left"
	}

	// Keep the cursor row visible when the form is taller than the screen.
	maxRows := max(3, h-4)
	first := 0
	if len(rows) > maxRows {
		cursor := 0
		for i, r := range rows {
			if r.hit != nil && r.hit.kind == hitRow && r.hit.idx == st.Row && r.bg == render.BgPopupSel {
				cursor = i
			}
		}
		first = min(max(0, cursor-maxRows/2), len(rows)-maxRows)
		// The header, the buttons and the hint stay.
		tail := rows[len(rows)-2:]
		head := rows[:1]
		mid := rows[1 : len(rows)-2]
		room := maxRows - 3
		s := min(max(0, first-1), max(0, len(mid)-room))
		rows = append(append(head, mid[s:min(len(mid), s+room)]...), tail...)
	}
	b := askBoxFor(w, h, len(rows))
	b.frame(c)
	st.hits = st.hits[:0]
	btnY := -1
	for i, r := range rows {
		b.fill(c, i, r.bg)
		b.put(c, 2, i, render.Truncate(r.text, inner), r.fg, r.bg)
		if r.hit != nil {
			hit := *r.hit
			hit.x, hit.y, hit.w = b.x, b.y+i, b.w
			st.hits = append(st.hits, hit)
		}
		if i == len(rows)-2 {
			btnY = i
		}
	}
	// Redraw the buttons with focus highlighting and record their areas.
	if btnY >= 0 {
		x := 2
		for f := focusPrev; f < focusCount; f++ {
			label := labels[f]
			fg, rbg := render.FgSoft, render.BgPopup
			if st.Focus == f {
				fg, rbg = render.FgBlack, render.Blue
			}
			wd := b.put(c, x, btnY, label, fg, rbg)
			st.hits = append(st.hits, askHit{x: b.x + x, y: b.y + btnY, w: max(1, wd), kind: hitButton, idx: f})
			x += wd + gap
		}
	}
}

// askMouse handles a click on the form.
func (a *App) askMouse(e *tcell.EventMouse) {
	st := a.st.Ask
	if st == nil || e.Buttons()&tcell.Button1 == 0 {
		return
	}
	x, y := e.Position()
	for _, hit := range st.hits {
		if y != hit.y || x < hit.x || x >= hit.x+hit.w {
			continue
		}
		switch hit.kind {
		case hitQuestion:
			st.enter(hit.idx)
		case hitRow:
			st.Focus, st.Typing, st.Row = focusBody, false, hit.idx
			a.askPressRow()
		case hitButton:
			st.Focus, st.Typing = hit.idx, false
			a.askActivate(hit.idx)
		}
		return
	}
}
