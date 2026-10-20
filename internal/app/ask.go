package app

import (
	"encoding/json"
	"strings"
	"time"

	"meno/internal/proto"
	"meno/internal/ui"
)

// Focus targets of the question form.
const (
	focusBody = iota
	focusPrev
	focusNext
	focusCancel
	focusCount
)

// Question types of a form, as the agent names them.
const (
	askSingle = "single"
	askMulti  = "multi"
	askText   = "text"
)

type askOption struct{ ID, Label string }

type askQuestion struct {
	ID, Text, Type string
	Options        []askOption
	Recommendation string
}

type askForm struct{ Questions []askQuestion }

// askAnswer is the draft answer of one question, kept while the user moves
// around, resizes the terminal or comes back from another question.
type askAnswer struct {
	Picked []bool
	Other  []rune // custom text (the answer itself for a text question)
}

// askState is the form the agent waits on. It is shown as a dialog over the
// session; the composer's own text is put aside while it is open.
type askState struct {
	ID     string // the daemon's id of the form
	F      askForm
	Ans    []askAnswer
	Cur    int  // question on screen
	Focus  int  // focusBody...focusCancel
	Row    int  // cursor row in the body: options, then the custom-text row
	Typing bool // keys edit the custom text of the current question
	Notice string
	Until  time.Time
	draft  draft
	hits   []askHit // clickable areas of the last draw
}

// draft is the composer as it was before the form.
type draft struct {
	Input      []rune
	Cursor     int
	Mode       ui.Mode
	Undo, Redo []snap
	InsBase    snap
}

func newAskState(f proto.AskForm) *askState {
	st := &askState{ID: f.AskID}
	for _, q := range f.Questions {
		aq := askQuestion{ID: q.ID, Text: q.Prompt, Type: q.Type, Recommendation: q.Recommendation}
		if len(q.Choices) > 0 {
			for _, c := range q.Choices {
				aq.Options = append(aq.Options, askOption{c.ID, c.Label})
			}
		} else {
			for _, id := range q.Options {
				aq.Options = append(aq.Options, askOption{id, id})
			}
		}
		st.F.Questions = append(st.F.Questions, aq)
		st.Ans = append(st.Ans, askAnswer{Picked: make([]bool, len(aq.Options))})
	}
	st.enter(0)
	if f.Deadline != "" {
		if t, err := time.Parse(time.RFC3339Nano, f.Deadline); err == nil {
			st.Until = t
		}
	}
	return st
}

// enter shows question i with the cursor on its selected row (or first row).
func (st *askState) enter(i int) {
	st.Cur = min(max(i, 0), len(st.F.Questions)-1)
	st.Row, st.Focus, st.Notice = 0, focusBody, ""
	q := st.F.Questions[st.Cur]
	st.Typing = q.Type == askText
	if q.Type != askText {
		for idx, picked := range st.Ans[st.Cur].Picked {
			if picked {
				st.Row = idx
				break
			}
		}
		if q.Type == askSingle && strings.TrimSpace(string(st.Ans[st.Cur].Other)) != "" {
			st.Row = len(q.Options)
		}
	}
}

func (st *askState) q() askQuestion { return st.F.Questions[st.Cur] }

// rows is the number of body rows of the current question: the options and the
// custom-text row (a text question has only the text row).
func (st *askState) rows() int {
	if st.q().Type == askText {
		return 1
	}
	return len(st.q().Options) + 1
}

// onOther reports whether the body cursor is on the custom-text row.
func (st *askState) onOther() bool {
	return st.q().Type == askText || st.Row == len(st.q().Options)
}

// answered reports whether question i has a real answer. Empty custom text is
// not an answer.
func (st *askState) answered(i int) bool {
	a := st.Ans[i]
	if strings.TrimSpace(string(a.Other)) != "" {
		return true
	}
	if st.F.Questions[i].Type == askText {
		return false
	}
	for _, p := range a.Picked {
		if p {
			return true
		}
	}
	return false
}

func (st *askState) answeredCount() int {
	n := 0
	for i := range st.Ans {
		if st.answered(i) {
			n++
		}
	}
	return n
}

// choose selects option row r of the current question. A single question holds
// one option or custom text, never both; a multi question holds both.
func (st *askState) choose(r int) {
	q := st.q()
	a := &st.Ans[st.Cur]
	if r < 0 || r >= len(q.Options) {
		return
	}
	switch q.Type {
	case askSingle:
		for i := range a.Picked {
			a.Picked[i] = i == r
		}
		a.Other = nil
	case askMulti:
		a.Picked[r] = !a.Picked[r]
	}
}

// build turns the drafts into the answers the daemon takes: per question the
// chosen option ids, or the custom text. Text that belongs to an unselected
// alternative is left out.
func (st *askState) build() map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for i, q := range st.F.Questions {
		a := st.Ans[i]
		var picked []string
		for j, p := range a.Picked {
			if p {
				picked = append(picked, q.Options[j].ID)
			}
		}
		text := strings.TrimSpace(string(a.Other))
		if text != "" && q.Type == askSingle && len(picked) > 0 {
			text = ""
		}
		b, _ := json.Marshal(map[string]any{"selected": nonNil(picked), "text": text})
		out[q.ID] = b
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// askChoosing reports whether a form is open, so the composer stays out of it.
func (a *App) askChoosing() bool { return a.st.Ask != nil }

// askHeight: the form is a dialog, it takes no rows above the composer.
func (a *App) askHeight() int { return 0 }

// wordWrap wraps text to width columns at spaces, keeping line breaks; words
// longer than a row are cut.
func wordWrap(text string, width int) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			for ui.TextWidth(word) > width {
				if line != "" {
					out = append(out, line)
					line = ""
				}
				parts := ui.Wrap(word, width)
				out = append(out, parts[:len(parts)-1]...)
				word = parts[len(parts)-1]
			}
			switch {
			case line == "":
				line = word
			case ui.TextWidth(line)+1+ui.TextWidth(word) <= width:
				line += " " + word
			default:
				out = append(out, line)
				line = word
			}
		}
		out = append(out, line)
	}
	return out
}
