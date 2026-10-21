package app

import (
	"fmt"
	"strings"

	"meno/internal/commentref"
	"meno/internal/objtag"
	"meno/internal/ui"
)

// commentSelection turns the VISUAL selection of the saved conversation into a
// Comment object at the end of the draft. It only quotes persisted assistant
// text, tool code, output and errors; anything else is refused with a reason
// and nothing changes. It never sends and shows no preview.
func (a *App) commentSelection(top int) {
	ref, err := a.selectionReference(top)
	if err != nil {
		a.notifyErr("comment: " + err.Error())
		return
	}
	enc, err := commentref.Encode(ref)
	if err != nil {
		a.notifyErr("comment: " + err.Error())
		return
	}
	r, ok := objectRune(inputObject{Kind: objtag.Comment, Ref: enc})
	if !ok {
		a.notifyErr("comment: too many distinct objects in this session")
		return
	}
	before := a.snapshot()
	a.exitVisual()
	a.st.Input = append(a.st.Input, r)
	a.pushUndo(before)
	a.st.Mode, a.st.InLog = ui.ModeInput, false
	a.st.Cursor, a.st.WantSet = len(a.st.Input), false
	a.notify("added " + ref.Label())
}

// selectionReference reads the selected rows and groups them by source.
func (a *App) selectionReference(top int) (commentref.Reference, error) {
	if !a.st.AnchorLog || !a.st.InLog {
		return commentref.Reference{}, fmt.Errorf("select text in the conversation, not the draft")
	}
	w, _ := a.scr.Size()
	lines := a.transcriptWithSelection(w)
	s, e := a.selOrder(top)
	if e.line >= len(lines) || s.line < 0 {
		return commentref.Reference{}, fmt.Errorf("the selection is outside the conversation")
	}
	sid := a.st.Runtime.ID()
	var ref commentref.Reference
	var cur *commentref.Segment
	var parts []string
	flush := func() {
		if cur != nil {
			cur.Quote = strings.Join(parts, "\n")
			ref.Segments = append(ref.Segments, *cur)
			cur, parts = nil, nil
		}
	}
	for l := s.line; l <= e.line; l++ {
		row := []rune(lines[l].Text())
		lo, hi, ok := a.selSpan(top, l, len(row))
		if !ok {
			continue
		}
		src := lines[l].Source
		if src == nil {
			if strings.TrimSpace(string(row)) == "" {
				continue // a blank band between blocks
			}
			return commentref.Reference{}, fmt.Errorf("only assistant text, code, output and errors can be quoted")
		}
		lo = max(lo, src.Skip)
		hi = min(hi, len(row))
		text := ""
		if lo < hi {
			text = string(row[lo:hi])
		}
		if cur == nil || cur.TurnID != src.TurnID || cur.CallID != src.CallID || cur.Part != src.Part {
			flush()
			cur = &commentref.Segment{SessionID: sid, TurnID: src.TurnID, CallID: src.CallID, Part: src.Part}
		}
		// Source/output spaces are content, including at a wrapped row boundary.
		if src.Part == commentref.PartAssistant {
			text = strings.TrimRight(text, " ")
		}
		parts = append(parts, text)
	}
	flush()
	var keep []commentref.Segment
	for _, sg := range ref.Segments {
		if strings.TrimSpace(sg.Quote) != "" {
			sg.Quote = strings.Trim(sg.Quote, "\n")
			keep = append(keep, sg)
		}
	}
	ref.Segments = keep
	if len(ref.Segments) == 0 {
		return commentref.Reference{}, fmt.Errorf("nothing to quote in the selection")
	}
	return ref, nil
}
